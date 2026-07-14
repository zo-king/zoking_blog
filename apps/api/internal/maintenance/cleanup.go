package maintenance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MediaCleanupOptions struct {
	DryRun                bool
	UseDefaultGracePeriod bool
	GracePeriod           time.Duration
	BatchSize             int
	Now                   time.Time
}

type ReleaseCleanupOptions struct {
	DryRun               bool
	UseDefaultKeepLatest bool
	UseDefaultKeepDays   bool
	KeepLatest           int
	KeepDays             int
	Now                  time.Time
}

type CleanupItem struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`
	Path      string    `json:"path,omitempty"`
	Reason    string    `json:"reason"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

type CleanupResult struct {
	DryRun         bool          `json:"dry_run"`
	Cutoff         time.Time     `json:"cutoff"`
	CandidateCount int           `json:"candidate_count"`
	DeletedCount   int           `json:"deleted_count"`
	SkippedCount   int           `json:"skipped_count"`
	Items          []CleanupItem `json:"items"`
}

var errMediaStillReferenced = errors.New("media is still referenced")

type quarantinedOrphanMedia struct {
	root           string
	originalPath   string
	quarantinePath string
	moved          bool
}

const publishPromotionLockID int64 = 2026071001

func CleanupOrphanMedia(ctx context.Context, db *gorm.DB, cfg config.Config, options MediaCleanupOptions) (CleanupResult, error) {
	options = normalizeMediaOptions(cfg, options)
	cutoff := options.Now.Add(-options.GracePeriod)
	result := CleanupResult{
		DryRun: options.DryRun,
		Cutoff: cutoff,
		Items:  []CleanupItem{},
	}

	var media []model.MediaAsset
	if err := db.WithContext(ctx).
		Where("status <> ?", "deleted").
		Where("created_at <= ?", cutoff).
		Where("not exists (select 1 from media_usages where media_usages.media_id = media_assets.id and media_usages.deleted_at is null)").
		Order("created_at asc").
		Limit(options.BatchSize).
		Find(&media).Error; err != nil {
		return result, err
	}

	for _, asset := range media {
		item := CleanupItem{
			ID:        asset.ID,
			Key:       asset.StorageKey,
			Reason:    "orphan_media",
			Action:    "candidate",
			CreatedAt: asset.CreatedAt,
		}
		result.CandidateCount++

		if asset.StorageDriver != "local" {
			item.Action = "skipped"
			item.Reason = "unsupported_storage_driver"
			result.SkippedCount++
			result.Items = append(result.Items, item)
			continue
		}

		absPath, ok := safeChildPath(cfg.MediaLocalDir, asset.StorageKey)
		item.Path = absPath
		if !ok {
			item.Action = "skipped"
			item.Reason = "unsafe_storage_path"
			result.SkippedCount++
			result.Items = append(result.Items, item)
			continue
		}
		if options.DryRun {
			result.Items = append(result.Items, item)
			continue
		}

		var quarantined *quarantinedOrphanMedia
		transactionErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var locked model.MediaAsset
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("status <> ?", "deleted").First(&locked, "id = ?", asset.ID).Error; err != nil {
				return err
			}
			var usageCount int64
			if err := tx.Model(&model.MediaUsage{}).Where("media_id = ?", locked.ID).Count(&usageCount).Error; err != nil {
				return err
			}
			if usageCount > 0 {
				return errMediaStillReferenced
			}
			if locked.StorageDriver != "local" {
				return fmt.Errorf("unsupported media storage driver %q", locked.StorageDriver)
			}
			var err error
			quarantined, err = quarantineOrphanMediaFile(cfg.MediaLocalDir, locked.StorageKey)
			if err != nil {
				return fmt.Errorf("quarantine orphan media %s: %w", locked.ID, err)
			}
			if err := tx.Model(&locked).Updates(map[string]interface{}{"status": "deleted"}).Error; err != nil {
				return err
			}
			return tx.Delete(&locked).Error
		})
		if transactionErr != nil {
			if restoreErr := quarantined.restore(); restoreErr != nil {
				return result, errors.Join(transactionErr, fmt.Errorf("restore orphan media after database rollback: %w", restoreErr))
			}
			if errors.Is(transactionErr, errMediaStillReferenced) || errors.Is(transactionErr, gorm.ErrRecordNotFound) {
				item.Action = "skipped"
				item.Reason = "media_changed_or_referenced"
				result.SkippedCount++
				result.Items = append(result.Items, item)
				continue
			}
			return result, transactionErr
		}
		if err := quarantined.discard(); err != nil {
			return result, fmt.Errorf("orphan media was revoked but quarantine cleanup failed: %w", err)
		}
		item.Action = "deleted"
		result.DeletedCount++
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func CleanupOldReleases(ctx context.Context, db *gorm.DB, cfg config.Config, options ReleaseCleanupOptions) (CleanupResult, error) {
	options = normalizeReleaseOptions(cfg, options)
	cutoff := options.Now.AddDate(0, 0, -options.KeepDays)
	result := CleanupResult{
		DryRun: options.DryRun,
		Cutoff: cutoff,
		Items:  []CleanupItem{},
	}

	var releases []model.PublishRelease
	if err := db.WithContext(ctx).
		Order("created_at desc").
		Find(&releases).Error; err != nil {
		return result, err
	}

	inactiveSeen := 0
	releaseRoot := filepath.Clean(cfg.PublishReleaseRoot)
	currentDir := filepath.Clean(cfg.PublishCurrentDir)
	for _, release := range releases {
		if release.IsActive {
			continue
		}
		inactiveSeen++
		if inactiveSeen <= options.KeepLatest {
			continue
		}
		if release.CreatedAt.After(cutoff) {
			continue
		}

		outputPath := release.OutputPath
		if outputPath == "" {
			outputPath = filepath.Join(releaseRoot, release.ReleaseKey)
		}
		outputPath = filepath.Clean(outputPath)
		item := CleanupItem{
			ID:        release.ID,
			Key:       release.ReleaseKey,
			Path:      outputPath,
			Reason:    "inactive_release_retention",
			Action:    "candidate",
			CreatedAt: release.CreatedAt,
		}
		result.CandidateCount++

		if outputPath == currentDir || !isChildPath(releaseRoot, outputPath) {
			item.Action = "skipped"
			item.Reason = "unsafe_release_path"
			result.SkippedCount++
			result.Items = append(result.Items, item)
			continue
		}
		if options.DryRun {
			result.Items = append(result.Items, item)
			continue
		}

		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("select pg_advisory_xact_lock(?)", publishPromotionLockID).Error; err != nil {
				return err
			}
			var locked model.PublishRelease
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", release.ID).Error; err != nil {
				return err
			}
			if locked.IsActive {
				return gorm.ErrInvalidData
			}
			if err := tx.Where("resource_type = ? and resource_id = ?", mediaref.ResourceRelease, release.ID).Delete(&model.MediaUsage{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&locked).Updates(map[string]interface{}{"status": "pruned", "is_active": false}).Error; err != nil {
				return err
			}
			return tx.Delete(&locked).Error
		}); err != nil {
			if errors.Is(err, gorm.ErrInvalidData) || errors.Is(err, gorm.ErrRecordNotFound) {
				item.Action = "skipped"
				item.Reason = "release_changed"
				result.SkippedCount++
				result.Items = append(result.Items, item)
				continue
			}
			return result, err
		}
		if err := os.RemoveAll(outputPath); err != nil {
			return result, err
		}
		item.Action = "deleted"
		result.DeletedCount++
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func normalizeMediaOptions(cfg config.Config, options MediaCleanupOptions) MediaCleanupOptions {
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	if options.UseDefaultGracePeriod {
		options.GracePeriod = cfg.MediaOrphanGracePeriod
	}
	if options.GracePeriod < 0 {
		options.GracePeriod = 0
	}
	if options.BatchSize <= 0 {
		options.BatchSize = cfg.MediaCleanupBatchSize
	}
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}
	if options.BatchSize > 500 {
		options.BatchSize = 500
	}
	return options
}

func normalizeReleaseOptions(cfg config.Config, options ReleaseCleanupOptions) ReleaseCleanupOptions {
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	if options.UseDefaultKeepLatest {
		options.KeepLatest = cfg.PublishReleaseKeepLatest
	}
	if options.KeepLatest < 0 {
		options.KeepLatest = 0
	}
	if options.KeepLatest < 1 {
		options.KeepLatest = 1
	}
	if options.UseDefaultKeepDays {
		options.KeepDays = cfg.PublishReleaseKeepDays
	}
	if options.KeepDays < 0 {
		options.KeepDays = 0
	}
	return options
}

func safeChildPath(root string, storageKey string) (string, bool) {
	if root == "" || storageKey == "" || strings.ContainsAny(storageKey, `\:`) || strings.HasPrefix(storageKey, "/") {
		return "", false
	}
	root = filepath.Clean(root)
	converted := filepath.FromSlash(storageKey)
	if filepath.IsAbs(converted) || filepath.ToSlash(filepath.Clean(converted)) != storageKey {
		return "", false
	}
	target := filepath.Clean(filepath.Join(root, converted))
	if !isChildPath(root, target) {
		return "", false
	}
	return target, true
}

func quarantineOrphanMediaFile(root, storageKey string) (*quarantinedOrphanMedia, error) {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	target, ok := safeChildPath(root, storageKey)
	if !ok {
		return nil, errors.New("unsafe media storage path")
	}
	quarantine := &quarantinedOrphanMedia{root: root, originalPath: target}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return quarantine, nil
		}
		return nil, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return quarantine, nil
		}
		return nil, err
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil || !isSameOrChildPath(resolvedRoot, resolvedParent) {
		return nil, errors.New("media storage path escapes configured root")
	}
	resolvedTarget, resolveErr := filepath.EvalSymlinks(target)
	if resolveErr == nil {
		if !isChildPath(resolvedRoot, resolvedTarget) {
			return nil, errors.New("media storage path escapes configured root")
		}
	} else if info.Mode()&os.ModeSymlink == 0 || !errors.Is(resolveErr, os.ErrNotExist) {
		return nil, resolveErr
	}
	quarantineDir, err := privateOrphanQuarantineDir(resolvedRoot)
	if err != nil {
		return nil, err
	}
	quarantine.quarantinePath = filepath.Join(quarantineDir, uuid.NewString())
	if err := os.Rename(target, quarantine.quarantinePath); err != nil {
		return nil, err
	}
	quarantine.moved = true
	return quarantine, nil
}

func privateOrphanQuarantineDir(resolvedRoot string) (string, error) {
	privateRoot := filepath.Join(resolvedRoot, ".zoking-private")
	if err := ensurePrivateOrphanMediaDir(privateRoot); err != nil {
		return "", err
	}
	dir := filepath.Join(privateRoot, "quarantine")
	if err := ensurePrivateOrphanMediaDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func ensurePrivateOrphanMediaDir(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe media quarantine directory")
	}
	return os.Chmod(dir, 0o700)
}

func (file *quarantinedOrphanMedia) restore() error {
	if file == nil || !file.moved {
		return nil
	}
	resolvedRoot, err := filepath.EvalSymlinks(file.root)
	if err != nil {
		return err
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(file.originalPath))
	if err != nil || !isSameOrChildPath(resolvedRoot, resolvedParent) {
		return errors.New("unsafe media restore path")
	}
	if _, err := os.Lstat(file.originalPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return errors.New("media restore destination already exists")
		}
		return err
	}
	if err := os.Rename(file.quarantinePath, file.originalPath); err != nil {
		return err
	}
	file.moved = false
	return nil
}

func (file *quarantinedOrphanMedia) discard() error {
	if file == nil || !file.moved {
		return nil
	}
	if err := os.Remove(file.quarantinePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file.moved = false
	return nil
}

func isSameOrChildPath(root, candidate string) bool {
	return filepath.Clean(root) == filepath.Clean(candidate) || isChildPath(root, candidate)
}

func isChildPath(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." {
		return false
	}
	return rel != "" && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
