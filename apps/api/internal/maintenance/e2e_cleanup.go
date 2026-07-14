package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
)

type E2EIDRef struct {
	ID uuid.UUID `json:"id"`
}

type E2ESlugRef struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	RemoveSnapshot *bool     `json:"remove_snapshot,omitempty"`
}

type E2EKeyRef struct {
	ID  uuid.UUID `json:"id"`
	Key string    `json:"key"`
}

type E2ERunManifest struct {
	SchemaVersion     int                             `json:"schema_version"`
	RunID             uuid.UUID                       `json:"run_id"`
	BaselineReleaseID *uuid.UUID                      `json:"baseline_release_id"`
	SettingsBefore    *publisher.SiteSettingsSnapshot `json:"settings_before"`
	Posts             []E2ESlugRef                    `json:"posts"`
	Pages             []E2ESlugRef                    `json:"pages"`
	Categories        []E2ESlugRef                    `json:"categories"`
	Tags              []E2ESlugRef                    `json:"tags"`
	Comments          []E2EIDRef                      `json:"comments"`
	Media             []E2EKeyRef                     `json:"media"`
	Previews          []E2EKeyRef                     `json:"previews"`
	Jobs              []E2EIDRef                      `json:"jobs"`
	Releases          []E2EKeyRef                     `json:"releases"`
}

type E2ECleanupResult struct {
	RunID      uuid.UUID      `json:"run_id"`
	DryRun     bool           `json:"dry_run"`
	Candidates map[string]int `json:"candidates"`
	Deleted    map[string]int `json:"deleted"`
	Skipped    []string       `json:"skipped"`
}

type E2EValidationError struct{ Message string }

func (e *E2EValidationError) Error() string { return e.Message }

type e2eCleanupPlan struct {
	manifest            E2ERunManifest
	baseline            *model.PublishRelease
	posts               []model.Post
	pages               []model.Page
	categories          []model.Category
	tags                []model.Tag
	comments            []model.Comment
	media               []model.MediaAsset
	previews            []model.PublishPreview
	jobs                []model.PublishJob
	releases            []model.PublishRelease
	removePostSnapshots map[uuid.UUID]bool
	removePageSnapshots map[uuid.UUID]bool
}

func CleanupE2ERun(ctx context.Context, db *gorm.DB, cfg config.Config, manifest E2ERunManifest, dryRun bool) (E2ECleanupResult, error) {
	result := E2ECleanupResult{RunID: manifest.RunID, DryRun: dryRun, Candidates: map[string]int{}, Deleted: map[string]int{}, Skipped: []string{}}
	acquired, releaseLock, err := publisher.AcquirePublishWorkerLock(ctx, db)
	if err != nil {
		return result, err
	}
	if !acquired {
		return result, validationError("publish worker is busy")
	}
	defer releaseLock()

	plan, err := buildE2ECleanupPlan(ctx, db, cfg, manifest)
	if err != nil {
		return result, err
	}
	result.Candidates = plan.counts()
	if dryRun {
		return result, nil
	}

	if manifest.SettingsBefore != nil {
		if err := restoreE2ESettings(ctx, db, *manifest.SettingsBefore); err != nil {
			return result, err
		}
		if err := publisher.ApplySiteSettings(cfg.HugoSiteDir, *manifest.SettingsBefore); err != nil {
			return result, fmt.Errorf("restore E2E Hugo settings: %w", err)
		}
	}
	if plan.baseline != nil {
		if _, err := publisher.PromoteRelease(ctx, db, cfg, plan.baseline.ID); err != nil {
			return result, fmt.Errorf("restore baseline release: %w", err)
		}
	}
	if err := verifyE2EReleaseState(ctx, db, plan); err != nil {
		return result, err
	}

	staging, err := stageE2EFiles(cfg, plan)
	if err != nil {
		return result, err
	}
	deleted, err := deleteE2ERecords(ctx, db, plan)
	if err != nil {
		if rollbackErr := staging.rollback(); rollbackErr != nil {
			return result, fmt.Errorf("delete E2E records: %w; restore staged files: %v", err, rollbackErr)
		}
		return result, err
	}
	result.Deleted = deleted
	if err := staging.purge(); err != nil {
		return result, fmt.Errorf("purge staged E2E files: %w", err)
	}
	return result, nil
}

func buildE2ECleanupPlan(ctx context.Context, db *gorm.DB, cfg config.Config, manifest E2ERunManifest) (e2eCleanupPlan, error) {
	plan := e2eCleanupPlan{manifest: manifest}
	if manifest.SchemaVersion != 1 || manifest.RunID == uuid.Nil {
		return plan, validationError("manifest schema_version or run_id is invalid")
	}
	if err := validateUniqueManifestRefs(manifest); err != nil {
		return plan, err
	}
	if manifest.SettingsBefore != nil {
		if err := validateE2ESettings(*manifest.SettingsBefore); err != nil {
			return plan, err
		}
		var count int64
		if err := db.WithContext(ctx).Model(&model.SiteSetting{}).Where("key in ?", e2eSettingKeys()).Count(&count).Error; err != nil {
			return plan, err
		}
		if count != int64(len(e2eSettingKeys())) {
			return plan, validationError("settings_before cannot be restored because required setting rows are missing")
		}
	}
	var err error
	if plan.posts, err = loadSlugRefs[model.Post](ctx, db, manifest.Posts, "posts", manifest.RunID); err != nil {
		return plan, err
	}
	if plan.pages, err = loadSlugRefs[model.Page](ctx, db, manifest.Pages, "pages", manifest.RunID); err != nil {
		return plan, err
	}
	if plan.categories, err = loadSlugRefs[model.Category](ctx, db, manifest.Categories, "categories", manifest.RunID); err != nil {
		return plan, err
	}
	if plan.tags, err = loadSlugRefs[model.Tag](ctx, db, manifest.Tags, "tags", manifest.RunID); err != nil {
		return plan, err
	}
	plan.removePostSnapshots = snapshotRemovalPlan(manifest.Posts)
	plan.removePageSnapshots = snapshotRemovalPlan(manifest.Pages)
	if plan.comments, err = loadIDRefs[model.Comment](ctx, db, manifest.Comments); err != nil {
		return plan, err
	}
	if plan.jobs, err = loadIDRefs[model.PublishJob](ctx, db, manifest.Jobs); err != nil {
		return plan, err
	}
	if plan.media, err = loadKeyRefs[model.MediaAsset](ctx, db, manifest.Media, "storage_key"); err != nil {
		return plan, err
	}
	if plan.previews, err = loadKeyRefs[model.PublishPreview](ctx, db, manifest.Previews, "preview_key"); err != nil {
		return plan, err
	}
	if plan.releases, err = loadKeyRefs[model.PublishRelease](ctx, db, manifest.Releases, "release_key"); err != nil {
		return plan, err
	}

	if manifest.BaselineReleaseID != nil {
		releaseIDs := collectModelIDs(plan.releases, func(item model.PublishRelease) uuid.UUID { return item.ID })
		if containsUUID(releaseIDs, *manifest.BaselineReleaseID) {
			return plan, validationError("baseline release is included in cleanup releases")
		}
		var baseline model.PublishRelease
		if err := db.WithContext(ctx).Unscoped().First(&baseline, "id = ?", *manifest.BaselineReleaseID).Error; err != nil {
			return plan, validationError("baseline release does not exist")
		}
		if baseline.DeletedAt.Valid {
			return plan, validationError("baseline release is deleted")
		}
		plan.baseline = &baseline
		expected, ok := safeE2EPath(cfg.PublishReleaseRoot, baseline.ReleaseKey)
		if !ok || filepath.Clean(baseline.OutputPath) != expected {
			return plan, validationError("baseline release path is unsafe")
		}
		if info, err := os.Stat(expected); err != nil || !info.IsDir() {
			return plan, validationError("baseline release output is missing")
		}
		if _, err := os.Stat(filepath.Join(expected, "manifest.json")); err != nil {
			return plan, validationError("baseline release manifest is missing")
		}
	}
	if err := validateE2ERelations(ctx, db, cfg, plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func validateE2ERelations(ctx context.Context, db *gorm.DB, cfg config.Config, plan e2eCleanupPlan) error {
	postIDs := collectModelIDs(plan.posts, func(item model.Post) uuid.UUID { return item.ID })
	pageIDs := collectModelIDs(plan.pages, func(item model.Page) uuid.UUID { return item.ID })
	categoryIDs := collectModelIDs(plan.categories, func(item model.Category) uuid.UUID { return item.ID })
	tagIDs := collectModelIDs(plan.tags, func(item model.Tag) uuid.UUID { return item.ID })
	commentIDs := collectModelIDs(plan.comments, func(item model.Comment) uuid.UUID { return item.ID })
	mediaIDs := collectModelIDs(plan.media, func(item model.MediaAsset) uuid.UUID { return item.ID })
	jobIDs := collectModelIDs(plan.jobs, func(item model.PublishJob) uuid.UUID { return item.ID })
	releaseIDs := collectModelIDs(plan.releases, func(item model.PublishRelease) uuid.UUID { return item.ID })
	for _, comment := range plan.comments {
		if !containsUUID(postIDs, comment.PostID) {
			return validationError("comment is not attached to a manifest post")
		}
	}
	for _, preview := range plan.previews {
		if preview.PostID != nil && !containsUUID(postIDs, *preview.PostID) {
			return validationError("preview references a non-manifest post")
		}
		if preview.PageID != nil && !containsUUID(pageIDs, *preview.PageID) {
			return validationError("preview references a non-manifest page")
		}
		expected, ok := safeE2EPath(cfg.PublishPreviewRoot, preview.PreviewKey)
		if !ok || (preview.OutputPath != "" && filepath.Clean(preview.OutputPath) != expected) {
			return validationError("preview path is unsafe")
		}
	}
	for _, release := range plan.releases {
		if !containsUUID(jobIDs, release.JobID) {
			return validationError("release references a non-manifest job")
		}
		expected, ok := safeE2EPath(cfg.PublishReleaseRoot, release.ReleaseKey)
		if !ok || filepath.Clean(release.OutputPath) != expected || expected == filepath.Clean(cfg.PublishCurrentDir) {
			return validationError("release path is unsafe")
		}
	}
	for _, job := range plan.jobs {
		if job.PostID != nil && !containsUUID(postIDs, *job.PostID) {
			return validationError("job references a non-manifest post")
		}
		if job.PageID != nil && !containsUUID(pageIDs, *job.PageID) {
			return validationError("job references a non-manifest page")
		}
		if containsString([]string{"requested", "queued", "snapshotting", "building", "verifying", "promoting"}, job.Status) {
			return validationError("manifest contains a non-terminal publish job")
		}
	}
	if anyActive(plan.releases) && plan.baseline == nil {
		return validationError("active test release requires baseline_release_id")
	}
	if err := ensureTaxonomyOnlyUsedBy(ctx, db, "post_categories", "category_id", categoryIDs, postIDs); err != nil {
		return err
	}
	if err := ensureTaxonomyOnlyUsedBy(ctx, db, "post_tags", "tag_id", tagIDs, postIDs); err != nil {
		return err
	}
	if err := ensureNoExternalCategoryChildren(ctx, db, categoryIDs); err != nil {
		return err
	}
	if err := ensureJobsOnlyOwnManifestReleases(ctx, db, jobIDs, releaseIDs); err != nil {
		return err
	}
	if err := ensureMediaOnlyUsedBy(ctx, db, plan.media, postIDs, pageIDs, releaseIDs); err != nil {
		return err
	}
	if err := ensureMediaNotCoveringExternalPosts(ctx, db, mediaIDs, postIDs); err != nil {
		return err
	}
	if err := ensureCommentsOnlyBelongToManifest(ctx, db, postIDs, commentIDs); err != nil {
		return err
	}
	for _, post := range plan.posts {
		if _, ok := safeContentPath(cfg.HugoSiteDir, "post", post.Slug); !ok {
			return validationError("post snapshot path is unsafe")
		}
	}
	for _, page := range plan.pages {
		if _, ok := safeContentPath(cfg.HugoSiteDir, "page", page.Slug); !ok {
			return validationError("page snapshot path is unsafe")
		}
	}
	for _, category := range plan.categories {
		if _, ok := safeContentPath(cfg.HugoSiteDir, "categories", category.Slug); !ok {
			return validationError("category snapshot path is unsafe")
		}
	}
	for _, tag := range plan.tags {
		if _, ok := safeContentPath(cfg.HugoSiteDir, "tags", tag.Slug); !ok {
			return validationError("tag snapshot path is unsafe")
		}
	}
	for _, asset := range plan.media {
		if asset.StorageDriver != "local" {
			return validationError("manifest media does not use local storage")
		}
		if _, ok := safeE2EPath(cfg.MediaLocalDir, asset.StorageKey); !ok {
			return validationError("media path is unsafe")
		}
		if !mediaOriginalNameMatchesRunID(asset.OriginalName, plan.manifest.RunID) {
			return validationError("media original_name does not match manifest run_id")
		}
	}
	return nil
}

type stagedE2EPath struct {
	original   string
	quarantine string
}

type e2eFileStaging struct {
	paths []stagedE2EPath
}

func stageE2EFiles(cfg config.Config, plan e2eCleanupPlan) (*e2eFileStaging, error) {
	staging := &e2eFileStaging{}
	stage := func(root, key string, directChild bool) error {
		var (
			original string
			ok       bool
		)
		if directChild {
			original, ok = safeDirectChildPath(root, key)
		} else {
			original, ok = safeE2EPath(root, key)
		}
		if !ok {
			return validationError("unsafe cleanup path")
		}
		quarantine := original + ".qa-cleanup-" + plan.manifest.RunID.String()
		if !pathInsideRoot(root, quarantine) {
			return validationError("unsafe cleanup quarantine path")
		}
		if _, err := os.Lstat(original); errors.Is(err, os.ErrNotExist) {
			if _, quarantineErr := os.Lstat(quarantine); quarantineErr == nil {
				staging.paths = append(staging.paths, stagedE2EPath{original: original, quarantine: quarantine})
				return nil
			} else if !errors.Is(quarantineErr, os.ErrNotExist) {
				return quarantineErr
			}
			return nil
		} else if err != nil {
			return err
		}
		if _, err := os.Lstat(quarantine); err == nil {
			return validationError("cleanup quarantine already exists beside a live path")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(original, quarantine); err != nil {
			return err
		}
		staging.paths = append(staging.paths, stagedE2EPath{original: original, quarantine: quarantine})
		return nil
	}
	rollbackOnError := func(err error) (*e2eFileStaging, error) {
		if rollbackErr := staging.rollback(); rollbackErr != nil {
			return staging, fmt.Errorf("stage E2E files: %w; rollback: %v", err, rollbackErr)
		}
		return staging, err
	}
	for _, preview := range plan.previews {
		if err := stage(cfg.PublishPreviewRoot, preview.PreviewKey, false); err != nil {
			return rollbackOnError(err)
		}
	}
	for _, release := range plan.releases {
		if err := stage(cfg.PublishReleaseRoot, release.ReleaseKey, false); err != nil {
			return rollbackOnError(err)
		}
	}
	for _, post := range plan.posts {
		if !shouldRemoveSnapshot(plan.removePostSnapshots, post.ID) {
			continue
		}
		if err := stage(filepath.Join(cfg.HugoSiteDir, "content", "post"), post.Slug, true); err != nil {
			return rollbackOnError(err)
		}
	}
	for _, page := range plan.pages {
		if !shouldRemoveSnapshot(plan.removePageSnapshots, page.ID) {
			continue
		}
		if err := stage(filepath.Join(cfg.HugoSiteDir, "content", "page"), page.Slug, true); err != nil {
			return rollbackOnError(err)
		}
	}
	for _, category := range plan.categories {
		if err := stage(filepath.Join(cfg.HugoSiteDir, "content", "categories"), category.Slug, true); err != nil {
			return rollbackOnError(err)
		}
	}
	for _, tag := range plan.tags {
		if err := stage(filepath.Join(cfg.HugoSiteDir, "content", "tags"), tag.Slug, true); err != nil {
			return rollbackOnError(err)
		}
	}
	for _, asset := range plan.media {
		if err := stage(cfg.MediaLocalDir, asset.StorageKey, false); err != nil {
			return rollbackOnError(err)
		}
	}
	return staging, nil
}

func (s *e2eFileStaging) rollback() error {
	var firstErr error
	for index := len(s.paths) - 1; index >= 0; index-- {
		entry := s.paths[index]
		if _, err := os.Lstat(entry.quarantine); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if _, err := os.Lstat(entry.original); err == nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("cannot restore staged path because original exists: %s", entry.original)
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.Rename(entry.quarantine, entry.original); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *e2eFileStaging) purge() error {
	var firstErr error
	for _, entry := range s.paths {
		if err := os.RemoveAll(entry.quarantine); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func deleteE2ERecords(ctx context.Context, db *gorm.DB, plan e2eCleanupPlan) (map[string]int, error) {
	deleted := emptyE2ECounts()
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("select pg_advisory_xact_lock(hashtext(?))", plan.manifest.RunID.String()).Error; err != nil {
			return err
		}
		postIDs := collectModelIDs(plan.posts, func(item model.Post) uuid.UUID { return item.ID })
		pageIDs := collectModelIDs(plan.pages, func(item model.Page) uuid.UUID { return item.ID })
		categoryIDs := collectModelIDs(plan.categories, func(item model.Category) uuid.UUID { return item.ID })
		tagIDs := collectModelIDs(plan.tags, func(item model.Tag) uuid.UUID { return item.ID })
		commentIDs := collectModelIDs(plan.comments, func(item model.Comment) uuid.UUID { return item.ID })
		mediaIDs := collectModelIDs(plan.media, func(item model.MediaAsset) uuid.UUID { return item.ID })
		previewIDs := collectModelIDs(plan.previews, func(item model.PublishPreview) uuid.UUID { return item.ID })
		jobIDs := collectModelIDs(plan.jobs, func(item model.PublishJob) uuid.UUID { return item.ID })
		releaseIDs := collectModelIDs(plan.releases, func(item model.PublishRelease) uuid.UUID { return item.ID })
		if len(releaseIDs) > 0 {
			var activeTargets int64
			if err := tx.Model(&model.PublishRelease{}).Where("id in ? and is_active = true", releaseIDs).Count(&activeTargets).Error; err != nil {
				return err
			}
			if activeTargets > 0 {
				return validationError("a manifest release became active during cleanup")
			}
		}
		if plan.baseline != nil {
			var activeBaseline int64
			if err := tx.Model(&model.PublishRelease{}).Where("id = ? and is_active = true", plan.baseline.ID).Count(&activeBaseline).Error; err != nil {
				return err
			}
			if activeBaseline != 1 {
				return validationError("baseline release changed during cleanup")
			}
		}
		if len(releaseIDs) > 0 {
			if err := tx.Unscoped().Where("resource_type = ? and resource_id in ?", "release", releaseIDs).Delete(&model.MediaUsage{}).Error; err != nil {
				return err
			}
		}
		count, err := hardDeleteIDs[model.PublishPreview](tx, "previews", previewIDs)
		if err != nil {
			return err
		}
		deleted["previews"] = count
		count, err = hardDeleteIDs[model.PublishRelease](tx, "releases", releaseIDs)
		if err != nil {
			return err
		}
		deleted["releases"] = count
		count, err = hardDeleteIDs[model.PublishJob](tx, "jobs", jobIDs)
		if err != nil {
			return err
		}
		deleted["jobs"] = count
		count, err = hardDeleteIDs[model.Comment](tx, "comments", commentIDs)
		if err != nil {
			return err
		}
		deleted["comments"] = count
		if len(postIDs) > 0 {
			if err := tx.Unscoped().Where("resource_type = ? and resource_id in ?", "post", postIDs).Delete(&model.MediaUsage{}).Error; err != nil {
				return err
			}
		}
		if len(pageIDs) > 0 {
			if err := tx.Unscoped().Where("resource_type = ? and resource_id in ?", "page", pageIDs).Delete(&model.MediaUsage{}).Error; err != nil {
				return err
			}
		}
		if len(postIDs) > 0 {
			if err := tx.Exec("delete from post_categories where post_id in ?", postIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("delete from post_tags where post_id in ?", postIDs).Error; err != nil {
				return err
			}
		}
		count, err = hardDeleteIDs[model.Post](tx, "posts", postIDs)
		if err != nil {
			return err
		}
		deleted["posts"] = count
		count, err = hardDeleteIDs[model.Page](tx, "pages", pageIDs)
		if err != nil {
			return err
		}
		deleted["pages"] = count
		count, err = hardDeleteIDs[model.Category](tx, "categories", categoryIDs)
		if err != nil {
			return err
		}
		deleted["categories"] = count
		count, err = hardDeleteIDs[model.Tag](tx, "tags", tagIDs)
		if err != nil {
			return err
		}
		deleted["tags"] = count
		count, err = hardDeleteIDs[model.MediaAsset](tx, "media", mediaIDs)
		if err != nil {
			return err
		}
		deleted["media"] = count
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func restoreE2ESettings(ctx context.Context, db *gorm.DB, settings publisher.SiteSettingsSnapshot) error {
	values := map[string]interface{}{
		"site.title": settings.Site.Title, "site.base_url": settings.Site.BaseURL,
		"sidebar.subtitle": settings.Sidebar.Subtitle, "sidebar.emoji": settings.Sidebar.Emoji,
		"comments.enabled": settings.Comments.Enabled, "comments.api_base": settings.Comments.APIBase,
		"footer.since": settings.Footer.Since, "pagination.pager_size": settings.Pagination.PagerSize,
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range values {
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			result := tx.Model(&model.SiteSetting{}).Where("key = ?", key).Update("value_json", raw)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return validationError("required site setting disappeared during cleanup")
			}
		}
		return nil
	})
}

func verifyE2EReleaseState(ctx context.Context, db *gorm.DB, plan e2eCleanupPlan) error {
	if plan.baseline == nil {
		return nil
	}
	var active []model.PublishRelease
	if err := db.WithContext(ctx).Where("is_active = true").Find(&active).Error; err != nil {
		return err
	}
	if len(active) != 1 || active[0].ID != plan.baseline.ID {
		return validationError("baseline release is not the only active release")
	}
	return nil
}

func (p e2eCleanupPlan) counts() map[string]int {
	counts := emptyE2ECounts()
	counts["posts"] = len(p.posts)
	counts["pages"] = len(p.pages)
	counts["categories"] = len(p.categories)
	counts["tags"] = len(p.tags)
	counts["comments"] = len(p.comments)
	counts["media"] = len(p.media)
	counts["previews"] = len(p.previews)
	counts["jobs"] = len(p.jobs)
	counts["releases"] = len(p.releases)
	return counts
}

func emptyE2ECounts() map[string]int {
	return map[string]int{"posts": 0, "pages": 0, "categories": 0, "tags": 0, "comments": 0, "media": 0, "previews": 0, "jobs": 0, "releases": 0}
}

func loadSlugRefs[T any](ctx context.Context, db *gorm.DB, refs []E2ESlugRef, table string, runID uuid.UUID) ([]T, error) {
	items := make([]T, 0, len(refs))
	for _, ref := range refs {
		if ref.ID == uuid.Nil || !allowedE2ECleanupSlug(table, ref.Slug, runID) {
			return nil, validationError(table + " manifest reference is invalid")
		}
		var item T
		err := db.WithContext(ctx).Unscoped().First(&item, "id = ?", ref.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, validationError(table + " manifest id does not exist")
		}
		if err != nil {
			return nil, err
		}
		if fieldString(item, "Slug") != ref.Slug {
			return nil, validationError(table + " id exists but slug does not match manifest")
		}
		items = append(items, item)
	}
	return items, nil
}

func allowedE2ECleanupSlug(table, slug string, runID uuid.UUID) bool {
	prefix, ok := map[string]string{
		"posts":      "e2e-smoke-",
		"pages":      "e2e-page-",
		"categories": "e2e-category-",
		"tags":       "e2e-tag-",
	}[table]
	return ok && runID != uuid.Nil && canonicalDirectChildName(slug) && slug == prefix+runID.String()
}

func loadIDRefs[T any](ctx context.Context, db *gorm.DB, refs []E2EIDRef) ([]T, error) {
	items := make([]T, 0, len(refs))
	for _, ref := range refs {
		if ref.ID == uuid.Nil {
			return nil, validationError("manifest contains an empty id")
		}
		var item T
		err := db.WithContext(ctx).Unscoped().First(&item, "id = ?", ref.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, validationError("manifest id does not exist")
		}
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func loadKeyRefs[T any](ctx context.Context, db *gorm.DB, refs []E2EKeyRef, column string) ([]T, error) {
	items := make([]T, 0, len(refs))
	for _, ref := range refs {
		if ref.ID == uuid.Nil || strings.TrimSpace(ref.Key) == "" {
			return nil, validationError("manifest key reference is invalid")
		}
		var item T
		err := db.WithContext(ctx).Unscoped().First(&item, "id = ?", ref.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, validationError(column + " manifest id does not exist")
		}
		if err != nil {
			return nil, err
		}
		field := map[string]string{"storage_key": "StorageKey", "preview_key": "PreviewKey", "release_key": "ReleaseKey"}[column]
		if field == "" || fieldString(item, field) != ref.Key {
			return nil, validationError(column + " id exists but key does not match manifest")
		}
		items = append(items, item)
	}
	return items, nil
}

func validateUniqueManifestRefs(m E2ERunManifest) error {
	seen := map[uuid.UUID]bool{}
	add := func(id uuid.UUID) error {
		if id == uuid.Nil || seen[id] {
			return validationError("manifest contains an empty or duplicate id")
		}
		seen[id] = true
		return nil
	}
	for _, refs := range [][]uuid.UUID{slugRefIDs(m.Posts), slugRefIDs(m.Pages), slugRefIDs(m.Categories), slugRefIDs(m.Tags), idRefIDs(m.Comments), keyRefIDs(m.Media), keyRefIDs(m.Previews), idRefIDs(m.Jobs), keyRefIDs(m.Releases)} {
		for _, id := range refs {
			if err := add(id); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureTaxonomyOnlyUsedBy(ctx context.Context, db *gorm.DB, table, column string, taxonomyIDs, postIDs []uuid.UUID) error {
	if len(taxonomyIDs) == 0 {
		return nil
	}
	var count int64
	q := db.WithContext(ctx).Table(table).Where(column+" in ?", taxonomyIDs)
	if len(postIDs) > 0 {
		q = q.Where("post_id not in ?", postIDs)
	}
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return validationError("taxonomy is referenced outside the manifest")
	}
	return nil
}

func ensureNoExternalCategoryChildren(ctx context.Context, db *gorm.DB, categoryIDs []uuid.UUID) error {
	if len(categoryIDs) == 0 {
		return nil
	}
	var count int64
	if err := db.WithContext(ctx).Unscoped().Model(&model.Category{}).Where("parent_id in ? and id not in ?", categoryIDs, categoryIDs).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return validationError("manifest category has children outside the manifest")
	}
	return nil
}

func ensureJobsOnlyOwnManifestReleases(ctx context.Context, db *gorm.DB, jobIDs, releaseIDs []uuid.UUID) error {
	if len(jobIDs) == 0 {
		return nil
	}
	var count int64
	query := db.WithContext(ctx).Unscoped().Model(&model.PublishRelease{}).Where("job_id in ?", jobIDs)
	if len(releaseIDs) > 0 {
		query = query.Where("id not in ?", releaseIDs)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return validationError("manifest job owns releases outside the manifest")
	}
	return nil
}
func ensureMediaOnlyUsedBy(ctx context.Context, db *gorm.DB, media []model.MediaAsset, posts, pages, releases []uuid.UUID) error {
	allowed := map[string]map[uuid.UUID]bool{"post": toSet(posts), "page": toSet(pages), "release": toSet(releases)}
	for _, a := range media {
		var uses []model.MediaUsage
		if err := db.WithContext(ctx).Where("media_id = ?", a.ID).Find(&uses).Error; err != nil {
			return err
		}
		for _, u := range uses {
			if !allowed[u.ResourceType][u.ResourceID] {
				return validationError("media is referenced outside the manifest")
			}
		}
	}
	return nil
}

func ensureMediaNotCoveringExternalPosts(ctx context.Context, db *gorm.DB, mediaIDs, postIDs []uuid.UUID) error {
	if len(mediaIDs) == 0 {
		return nil
	}
	var count int64
	query := db.WithContext(ctx).Unscoped().Model(&model.Post{}).Where("cover_media_id in ?", mediaIDs)
	if len(postIDs) > 0 {
		query = query.Where("id not in ?", postIDs)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return validationError("manifest media is used as a cover outside the manifest")
	}
	return nil
}

func ensureCommentsOnlyBelongToManifest(ctx context.Context, db *gorm.DB, postIDs, commentIDs []uuid.UUID) error {
	if len(postIDs) == 0 {
		return nil
	}
	var count int64
	query := db.WithContext(ctx).Unscoped().Model(&model.Comment{}).Where("post_id in ?", postIDs)
	if len(commentIDs) > 0 {
		query = query.Where("id not in ?", commentIDs)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return validationError("manifest post has comments outside the manifest")
	}
	return nil
}

func validateE2ESettings(settings publisher.SiteSettingsSnapshot) error {
	if strings.TrimSpace(settings.Site.Title) == "" || strings.TrimSpace(settings.Site.BaseURL) == "" || strings.TrimSpace(settings.Sidebar.Subtitle) == "" || strings.TrimSpace(settings.Sidebar.Emoji) == "" {
		return validationError("settings_before is incomplete")
	}
	if settings.Footer.Since < 1900 || settings.Pagination.PagerSize < 1 || settings.Pagination.PagerSize > 50 {
		return validationError("settings_before contains invalid numeric values")
	}
	return nil
}

func e2eSettingKeys() []string {
	return []string{"site.title", "site.base_url", "sidebar.subtitle", "sidebar.emoji", "comments.enabled", "comments.api_base", "footer.since", "pagination.pager_size"}
}

func fieldString(item interface{}, name string) string {
	value := reflect.ValueOf(item)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	field := value.FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func hardDeleteIDs[T any](tx *gorm.DB, candidate string, ids []uuid.UUID) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var item T
	result := tx.Unscoped().Where("id in ?", ids).Delete(&item)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != int64(len(ids)) {
		return 0, validationError(fmt.Sprintf("%s changed during cleanup: expected to delete %d rows, deleted %d", candidate, len(ids), result.RowsAffected))
	}
	return int(result.RowsAffected), nil
}

func safeE2EPath(root, key string) (string, bool) {
	keyPath := filepath.FromSlash(key)
	if strings.TrimSpace(root) == "" || strings.TrimSpace(key) == "" || filepath.IsAbs(keyPath) || filepath.VolumeName(keyPath) != "" {
		return "", false
	}
	root = filepath.Clean(root)
	target := filepath.Clean(filepath.Join(root, keyPath))
	rel, err := filepath.Rel(root, target)
	return target, err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func safeDirectChildPath(root, name string) (string, bool) {
	if strings.TrimSpace(root) == "" || !canonicalDirectChildName(name) {
		return "", false
	}
	root = filepath.Clean(root)
	target := filepath.Join(root, name)
	return target, pathInsideRoot(root, target)
}

func canonicalDirectChildName(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && name != "." && name != ".." &&
		!strings.ContainsAny(name, `/\`) && filepath.Clean(name) == name && filepath.Base(name) == name && filepath.VolumeName(name) == ""
}

func pathInsideRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func safeContentPath(site, section, slug string) (string, bool) {
	return safeDirectChildPath(filepath.Join(site, "content", section), slug)
}
func removeValidatedPath(root, key string) error {
	path, ok := safeE2EPath(root, key)
	if !ok {
		return validationError("unsafe cleanup directory")
	}
	return os.RemoveAll(path)
}
func removeValidatedFile(root, key string) error {
	path, ok := safeE2EPath(root, key)
	if !ok {
		return validationError("unsafe cleanup file")
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func snapshotRemovalPlan(refs []E2ESlugRef) map[uuid.UUID]bool {
	result := make(map[uuid.UUID]bool, len(refs))
	for _, ref := range refs {
		remove := true
		if ref.RemoveSnapshot != nil {
			remove = *ref.RemoveSnapshot
		}
		result[ref.ID] = remove
	}
	return result
}

func shouldRemoveSnapshot(removals map[uuid.UUID]bool, id uuid.UUID) bool {
	remove, ok := removals[id]
	return !ok || remove
}

func mediaOriginalNameMatchesRunID(originalName string, runID uuid.UUID) bool {
	return runID != uuid.Nil && strings.Contains(originalName, runID.String())
}

func collectModelIDs[T any](items []T, getID func(T) uuid.UUID) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, getID(item))
	}
	return ids
}

func validationError(message string) error { return &E2EValidationError{Message: message} }
func slugRefIDs(v []E2ESlugRef) []uuid.UUID {
	r := make([]uuid.UUID, 0, len(v))
	for _, x := range v {
		r = append(r, x.ID)
	}
	return r
}
func idRefIDs(v []E2EIDRef) []uuid.UUID {
	r := make([]uuid.UUID, 0, len(v))
	for _, x := range v {
		r = append(r, x.ID)
	}
	return r
}
func keyRefIDs(v []E2EKeyRef) []uuid.UUID {
	r := make([]uuid.UUID, 0, len(v))
	for _, x := range v {
		r = append(r, x.ID)
	}
	return r
}
func containsUUID(v []uuid.UUID, id uuid.UUID) bool {
	for _, x := range v {
		if x == id {
			return true
		}
	}
	return false
}
func containsString(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}
func anyActive(v []model.PublishRelease) bool {
	for _, x := range v {
		if x.IsActive {
			return true
		}
	}
	return false
}
func toSet(v []uuid.UUID) map[uuid.UUID]bool {
	r := map[uuid.UUID]bool{}
	for _, x := range v {
		r[x] = true
	}
	return r
}
