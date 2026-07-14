package publisher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	publishPromotionLockID     int64 = 2026071001
	maxLogMessageBytes               = 12000
	publicationRecoveryTimeout       = 5 * time.Second
)

var errPublishJobCanceled = errors.New("publish job canceled")

var ErrPublishContentNotPublished = errors.New("publish job content is not published")

type PublishManifest struct {
	JobID            string    `json:"job_id"`
	Scope            string    `json:"scope"`
	PostID           string    `json:"post_id"`
	PageID           string    `json:"page_id"`
	Slug             string    `json:"slug"`
	ContentPath      string    `json:"content_path"`
	ContentHash      string    `json:"content_hash"`
	SettingsHash     string    `json:"settings_hash"`
	AchievementsHash string    `json:"achievements_hash,omitempty"`
	DataPaths        []string  `json:"data_paths,omitempty"`
	ReleaseKey       string    `json:"release_key"`
	OutputPath       string    `json:"output_path"`
	CurrentPath      string    `json:"current_path"`
	CreatedAt        time.Time `json:"created_at"`
	HugoCommand      string    `json:"hugo_command"`
}

type PublishLogEntry struct {
	At      time.Time         `json:"at"`
	Stage   string            `json:"stage"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ProcessResult struct {
	Job     model.PublishJob     `json:"job"`
	Release model.PublishRelease `json:"release"`
}

type publishBuildInput struct {
	Scope          string
	Slug           string
	Post           *model.Post
	Page           *model.Page
	PostID         *uuid.UUID
	PageID         *uuid.UUID
	ContentPath    string
	SnapshotKey    string
	ContentMD      string
	AbsentPath     string
	Finalize       func(tx *gorm.DB) error
	PublishContent bool
}

func ProcessPostJob(ctx context.Context, db *gorm.DB, cfg config.Config, job model.PublishJob, post model.Post) (result ProcessResult, resultErr error) {
	if err := beginPostPublication(ctx, db, job.ID, post.ID); err != nil {
		return ProcessResult{}, err
	}
	defer func() {
		if resultErr != nil {
			if restoreErr := restorePostPublication(db, post.ID); restoreErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("restore post publication state: %w", restoreErr))
			}
		}
	}()

	logs := parseJobLogs(job.LogJSON)
	startedAt := time.Now()
	if err := updateJobStage(ctx, db, job.ID, &logs, "queued", "snapshotting", "snapshot", "snapshotting post content", map[string]interface{}{
		"started_at": startedAt,
	}); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}

	if err := hydratePostCoverMedia(ctx, db, &post); err != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "snapshot", "COVER_MEDIA_LOAD_FAILED", err)
		return ProcessResult{}, err
	}

	writeResult, err := WritePost(cfg.HugoSiteDir, post)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "snapshot", "SNAPSHOT_FAILED", err)
		return ProcessResult{}, err
	}
	if err := mediaref.SyncPostCoverUsage(db.WithContext(ctx), post.ID, post.CoverMediaID); err != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "snapshot", "MEDIA_USAGE_SYNC_FAILED", err)
		return ProcessResult{}, err
	}

	return processHugoBuildJob(ctx, db, cfg, job, &logs, startedAt, publishBuildInput{
		Scope:          "post",
		Slug:           post.Slug,
		Post:           &post,
		PostID:         &post.ID,
		ContentPath:    writeResult.ContentPath,
		SnapshotKey:    filepath.ToSlash(filepath.Join("content", "post", post.Slug, "index.md")),
		ContentMD:      post.ContentMD,
		PublishContent: true,
	})
}

func ProcessPageJob(ctx context.Context, db *gorm.DB, cfg config.Config, job model.PublishJob, page model.Page) (result ProcessResult, resultErr error) {
	if err := beginPagePublication(ctx, db, job.ID, page.ID); err != nil {
		return ProcessResult{}, err
	}
	defer func() {
		if resultErr != nil {
			if restoreErr := restorePagePublication(db, page.ID); restoreErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("restore page publication state: %w", restoreErr))
			}
		}
	}()

	logs := parseJobLogs(job.LogJSON)
	startedAt := time.Now()
	if err := updateJobStage(ctx, db, job.ID, &logs, "queued", "snapshotting", "snapshot", "snapshotting page content", map[string]interface{}{
		"started_at": startedAt,
	}); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}

	writeResult, err := WritePage(cfg.HugoSiteDir, page)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "snapshot", "SNAPSHOT_FAILED", err)
		return ProcessResult{}, err
	}

	return processHugoBuildJob(ctx, db, cfg, job, &logs, startedAt, publishBuildInput{
		Scope:          "page",
		Slug:           page.Slug,
		Page:           &page,
		PageID:         &page.ID,
		ContentPath:    writeResult.ContentPath,
		SnapshotKey:    filepath.ToSlash(filepath.Join("content", "page", page.Slug, "index.md")),
		ContentMD:      page.ContentMD,
		PublishContent: true,
	})
}

func beginPostPublication(ctx context.Context, db *gorm.DB, jobID uuid.UUID, postID uuid.UUID) error {
	var post model.Post
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockQueuedPublicationJob(tx, jobID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&post, "id = ?", postID).Error; err != nil {
			return err
		}
		if post.Status != "published" {
			return fmt.Errorf("%w: post status is %q", ErrPublishContentNotPublished, post.Status)
		}
		updated := tx.Model(&model.Post{}).
			Where("id = ? and status = ?", postID, "published").
			Update("status", "publishing")
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("post publication state transition affected %d rows", updated.RowsAffected)
		}
		return nil
	})
}

func beginPagePublication(ctx context.Context, db *gorm.DB, jobID uuid.UUID, pageID uuid.UUID) error {
	var page model.Page
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockQueuedPublicationJob(tx, jobID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&page, "id = ?", pageID).Error; err != nil {
			return err
		}
		if page.Status != "published" {
			return fmt.Errorf("%w: page status is %q", ErrPublishContentNotPublished, page.Status)
		}
		updated := tx.Model(&model.Page{}).
			Where("id = ? and status = ?", pageID, "published").
			Update("status", "publishing")
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("page publication state transition affected %d rows", updated.RowsAffected)
		}
		return nil
	})
}

func lockQueuedPublicationJob(tx *gorm.DB, jobID uuid.UUID) error {
	var job model.PublishJob
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").First(&job, "id = ?", jobID).Error; err != nil {
		return err
	}
	if job.Status == "canceled" {
		return errPublishJobCanceled
	}
	if job.Status != "queued" {
		return fmt.Errorf("publish job cannot begin from status %q", job.Status)
	}
	return nil
}

func restorePostPublication(db *gorm.DB, postID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), publicationRecoveryTimeout)
	defer cancel()
	return restorePostPublicationTx(db.WithContext(ctx), postID)
}

func restorePagePublication(db *gorm.DB, pageID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), publicationRecoveryTimeout)
	defer cancel()
	return restorePagePublicationTx(db.WithContext(ctx), pageID)
}

func restorePostPublicationTx(tx *gorm.DB, postID uuid.UUID) error {
	return tx.Model(&model.Post{}).
		Where("id = ? and status = ?", postID, "publishing").
		Update("status", "published").Error
}

func restorePagePublicationTx(tx *gorm.DB, pageID uuid.UUID) error {
	return tx.Model(&model.Page{}).
		Where("id = ? and status = ?", pageID, "publishing").
		Update("status", "published").Error
}

func finalizePublicationStatus(tx *gorm.DB, input publishBuildInput) error {
	if !input.PublishContent {
		return nil
	}
	if input.PostID != nil {
		updated := tx.Model(&model.Post{}).Where("id = ? and status = ?", *input.PostID, "publishing").Update("status", "published")
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("post publication finalization affected %d rows", updated.RowsAffected)
		}
		return nil
	}
	if input.PageID != nil {
		updated := tx.Model(&model.Page{}).Where("id = ? and status = ?", *input.PageID, "publishing").Update("status", "published")
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("page publication finalization affected %d rows", updated.RowsAffected)
		}
		return nil
	}
	return fmt.Errorf("published content input has no post or page id")
}

func ProcessPostWithdrawalJob(ctx context.Context, db *gorm.DB, cfg config.Config, job model.PublishJob, post model.Post) (ProcessResult, error) {
	logs := parseJobLogs(job.LogJSON)
	startedAt := time.Now()
	if err := updateJobStage(ctx, db, job.ID, &logs, "queued", "snapshotting", "withdraw", "removing post snapshot", map[string]interface{}{
		"started_at": startedAt,
	}); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}

	removed, err := RemovePost(cfg.HugoSiteDir, post.Slug)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "withdraw", "SNAPSHOT_REMOVE_FAILED", err)
		return ProcessResult{}, err
	}
	result, err := processHugoBuildJob(ctx, db, cfg, job, &logs, startedAt, publishBuildInput{
		Scope:      "withdraw_post",
		Slug:       post.Slug,
		PostID:     &post.ID,
		AbsentPath: "/p/" + post.Slug + "/",
		Finalize: func(tx *gorm.DB) error {
			if err := tx.Exec("delete from post_categories where post_id = ?", post.ID).Error; err != nil {
				return err
			}
			if err := tx.Exec("delete from post_tags where post_id = ?", post.ID).Error; err != nil {
				return err
			}
			if err := tx.Where("resource_type = ? and resource_id = ?", mediaref.ResourcePost, post.ID).Delete(&model.MediaUsage{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&post).Update("status", "archived").Error; err != nil {
				return err
			}
			return tx.Delete(&post).Error
		},
	})
	if err == nil || !removed {
		return result, err
	}
	if _, restoreErr := WritePost(cfg.HugoSiteDir, post); restoreErr != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "restore", "SNAPSHOT_RESTORE_FAILED", restoreErr)
		return ProcessResult{}, fmt.Errorf("withdraw post failed: %w; snapshot restore failed: %v", err, restoreErr)
	}
	appendJobLog(ctx, db, job.ID, &logs, "restore", "info", "post snapshot restored after failed withdrawal", nil)
	return ProcessResult{}, err
}

func ProcessPageWithdrawalJob(ctx context.Context, db *gorm.DB, cfg config.Config, job model.PublishJob, page model.Page) (ProcessResult, error) {
	logs := parseJobLogs(job.LogJSON)
	startedAt := time.Now()
	if err := updateJobStage(ctx, db, job.ID, &logs, "queued", "snapshotting", "withdraw", "removing page snapshot", map[string]interface{}{
		"started_at": startedAt,
	}); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}

	removed, err := RemovePage(cfg.HugoSiteDir, page.Slug)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "withdraw", "SNAPSHOT_REMOVE_FAILED", err)
		return ProcessResult{}, err
	}
	result, err := processHugoBuildJob(ctx, db, cfg, job, &logs, startedAt, publishBuildInput{
		Scope:      "withdraw_page",
		Slug:       page.Slug,
		PageID:     &page.ID,
		AbsentPath: "/" + page.Slug + "/",
		Finalize: func(tx *gorm.DB) error {
			if err := tx.Where("resource_type = ? and resource_id = ?", mediaref.ResourcePage, page.ID).Delete(&model.MediaUsage{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&page).Update("status", "archived").Error; err != nil {
				return err
			}
			return tx.Delete(&page).Error
		},
	})
	if err == nil || !removed {
		return result, err
	}
	if _, restoreErr := WritePage(cfg.HugoSiteDir, page); restoreErr != nil {
		markJobFailedWithLogs(ctx, db, &job, &logs, "restore", "SNAPSHOT_RESTORE_FAILED", restoreErr)
		return ProcessResult{}, fmt.Errorf("withdraw page failed: %w; snapshot restore failed: %v", err, restoreErr)
	}
	appendJobLog(ctx, db, job.ID, &logs, "restore", "info", "page snapshot restored after failed withdrawal", nil)
	return ProcessResult{}, err
}

func ProcessSiteJob(ctx context.Context, db *gorm.DB, cfg config.Config, job model.PublishJob) (ProcessResult, error) {
	logs := parseJobLogs(job.LogJSON)
	startedAt := time.Now()
	if err := updateJobStage(ctx, db, job.ID, &logs, "queued", "snapshotting", "snapshot", "snapshotting site settings", map[string]interface{}{
		"started_at": startedAt,
	}); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}

	return processHugoBuildJob(ctx, db, cfg, job, &logs, startedAt, publishBuildInput{
		Scope: "site",
	})
}

func processHugoBuildJob(ctx context.Context, db *gorm.DB, cfg config.Config, job model.PublishJob, logs *[]PublishLogEntry, startedAt time.Time, input publishBuildInput) (result ProcessResult, resultErr error) {
	settings, settingsHash, err := LoadSiteSettings(ctx, db)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "settings", "SETTINGS_LOAD_FAILED", err)
		return ProcessResult{}, err
	}
	if err := ValidateReleasePublicURLs(cfg.AppEnv, settings, cfg.SiteBaseURL, cfg.PublicAPIBaseURL, cfg.MediaPublicBaseURL); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "settings", "PUBLIC_URL_INVALID", err)
		return ProcessResult{}, err
	}
	if err := ApplySiteSettings(cfg.HugoSiteDir, settings); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "settings", "SETTINGS_APPLY_FAILED", err)
		return ProcessResult{}, err
	}
	achievements, err := loadPublishedAchievements(ctx, db)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "snapshot", "ACHIEVEMENTS_LOAD_FAILED", err)
		return ProcessResult{}, err
	}
	achievementSnapshot, err := WriteAchievementsData(cfg.HugoSiteDir, achievements)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "snapshot", "ACHIEVEMENTS_SNAPSHOT_FAILED", err)
		return ProcessResult{}, err
	}

	releaseKey := "rel_" + startedAt.Format("20060102_150405") + "_" + shortID(job.ID)
	outputPath := filepath.Join(releaseRoot(cfg), releaseKey)
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "release_dir", "RELEASE_DIR_FAILED", err)
		return ProcessResult{}, err
	}
	releaseRegistered := false
	defer func() {
		if releaseRegistered {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		removed, cleanupErr := cleanupUnregisteredJobOutput(cleanupCtx, db, cfg, job.ID, outputPath)
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("clean orphan release output for job %s: %w", job.ID, cleanupErr)
			addLog(logs, "cleanup", "error", cleanupErr.Error(), map[string]string{
				"code":        "ORPHAN_RELEASE_CLEANUP_FAILED",
				"output_path": outputPath,
			})
			markJobFailedWithLogs(cleanupCtx, db, &job, logs, "cleanup", "ORPHAN_RELEASE_CLEANUP_FAILED", cleanupErr)
			resultErr = errors.Join(resultErr, cleanupErr)
			return
		}
		if removed {
			appendJobLog(cleanupCtx, db, job.ID, logs, "cleanup", "info", "removed unregistered release output", map[string]string{
				"output_path": outputPath,
			})
		}
	}()

	if err := updateJobStage(ctx, db, job.ID, logs, "snapshotting", "building", "build", "running Hugo build", map[string]interface{}{
		"content_path":  input.ContentPath,
		"release_key":   releaseKey,
		"output_path":   outputPath,
		"snapshot_key":  input.SnapshotKey,
		"settings_hash": settingsHash,
	}); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}

	hugoBin := cfg.HugoBin
	if _, err := os.Stat(hugoBin); err != nil {
		hugoBin = "hugo"
	}

	buildCtx := ctx
	var cancel context.CancelFunc
	if cfg.PublishJobTimeout > 0 {
		buildCtx, cancel = context.WithTimeout(ctx, cfg.PublishJobTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(buildCtx, hugoBin, "--source", cfg.HugoSiteDir, "--destination", outputPath, "--minify")
	output, err := cmd.CombinedOutput()
	if err != nil {
		code := "HUGO_BUILD_FAILED"
		if errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
			code = "PUBLISH_TIMEOUT"
		}
		message := fmt.Errorf("%w: %s", err, compactLogOutput(output))
		markJobFailedWithLogs(ctx, db, &job, logs, "build", code, message)
		return ProcessResult{}, err
	}
	if summary := compactLogOutput(output); summary != "" {
		appendJobLog(ctx, db, job.ID, logs, "build", "info", summary, nil)
	}
	if err := ValidateReleaseOutputPublicURLs(cfg.AppEnv, outputPath, cfg.SiteBaseURL, cfg.PublicAPIBaseURL); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "verify", "PUBLIC_URL_OUTPUT_INVALID", err)
		return ProcessResult{}, err
	}

	if err := updateJobStage(ctx, db, job.ID, logs, "building", "verifying", "verify", "verifying Hugo release artifacts", nil); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}

	contentHash := settingsHash
	if input.ContentPath != "" {
		contentBytes, err := os.ReadFile(input.ContentPath)
		if err != nil {
			markJobFailedWithLogs(ctx, db, &job, logs, "manifest", "MANIFEST_FAILED", err)
			return ProcessResult{}, err
		}
		hash := sha256.Sum256(contentBytes)
		contentHash = hex.EncodeToString(hash[:])
	}
	postID := ""
	if input.PostID != nil {
		postID = input.PostID.String()
	}
	pageID := ""
	if input.PageID != nil {
		pageID = input.PageID.String()
	}
	manifest := PublishManifest{
		JobID:            job.ID.String(),
		Scope:            input.Scope,
		PostID:           postID,
		PageID:           pageID,
		Slug:             input.Slug,
		ContentPath:      input.ContentPath,
		ContentHash:      contentHash,
		SettingsHash:     settingsHash,
		AchievementsHash: achievementSnapshot.SHA256,
		DataPaths:        []string{achievementSnapshot.RelativePath},
		ReleaseKey:       releaseKey,
		OutputPath:       outputPath,
		CurrentPath:      currentOutputDir(cfg),
		CreatedAt:        time.Now(),
		HugoCommand:      cmd.String(),
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "manifest", "MANIFEST_FAILED", err)
		return ProcessResult{}, err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "manifest.json"), manifestJSON, 0o644); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "manifest", "MANIFEST_FAILED", err)
		return ProcessResult{}, err
	}
	if err := verifyReleaseTarget(outputPath, input.Post, input.Page); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "verify", "VERIFY_FAILED", err)
		return ProcessResult{}, err
	}
	if input.AbsentPath != "" {
		if err := verifyReleasePathAbsent(outputPath, input.AbsentPath); err != nil {
			markJobFailedWithLogs(ctx, db, &job, logs, "verify", "WITHDRAW_VERIFY_FAILED", err)
			return ProcessResult{}, err
		}
	}

	if err := updateJobStage(ctx, db, job.ID, logs, "verifying", "promoting", "promote", "staging current site directory", nil); err != nil {
		return ProcessResult{}, err
	}
	if err := ensureJobNotCanceled(ctx, db, job.ID); err != nil {
		return ProcessResult{}, err
	}
	stagedCurrent, err := stageCurrentRelease(cfg, outputPath, releaseKey)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "CURRENT_STAGE_FAILED", err)
		return ProcessResult{}, err
	}
	defer os.RemoveAll(stagedCurrent)

	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "PROMOTE_FAILED", tx.Error)
		return ProcessResult{}, tx.Error
	}
	committed := false
	rollbackTx := func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}
	defer rollbackTx()

	if err := acquirePromotionLock(tx); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "PROMOTE_FAILED", err)
		return ProcessResult{}, err
	}
	if err := tx.Model(&model.PublishRelease{}).Where("is_active = true").Updates(map[string]interface{}{
		"is_active": false,
		"status":    "inactive",
	}).Error; err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "PROMOTE_FAILED", err)
		return ProcessResult{}, err
	}
	promotedAt := time.Now()
	release := model.PublishRelease{
		JobID:        job.ID,
		ReleaseKey:   releaseKey,
		Status:       "active",
		PostID:       input.PostID,
		PageID:       input.PageID,
		OutputPath:   outputPath,
		ManifestJSON: manifestJSON,
		IsActive:     true,
		PromotedAt:   &promotedAt,
	}
	if err := tx.Create(&release).Error; err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "PROMOTE_FAILED", err)
		return ProcessResult{}, err
	}
	if input.ContentMD != "" {
		if err := mediaref.SyncReleaseMarkdownUsages(tx, release.ID, input.ContentMD); err != nil {
			markJobFailedWithLogs(ctx, db, &job, logs, "promote", "MEDIA_USAGE_SYNC_FAILED", err)
			return ProcessResult{}, err
		}
	}
	if input.Post != nil {
		if err := mediaref.SyncReleaseCoverUsage(tx, release.ID, input.Post.CoverMediaID); err != nil {
			markJobFailedWithLogs(ctx, db, &job, logs, "promote", "MEDIA_USAGE_SYNC_FAILED", err)
			return ProcessResult{}, err
		}
	}
	if err := finalizePublicationStatus(tx, input); err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "finalize", "CONTENT_FINALIZE_FAILED", err)
		return ProcessResult{}, err
	}
	if input.Finalize != nil {
		if err := input.Finalize(tx); err != nil {
			markJobFailedWithLogs(ctx, db, &job, logs, "finalize", "WITHDRAW_FINALIZE_FAILED", err)
			return ProcessResult{}, err
		}
	}
	backupPath, err := switchStagedCurrent(cfg, stagedCurrent)
	if err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "CURRENT_SWITCH_FAILED", err)
		return ProcessResult{}, err
	}
	defer func() {
		if !committed {
			_ = restoreCurrentDirectory(cfg, backupPath)
		}
	}()
	addLog(logs, "promote", "info", "release promoted to current site directory", map[string]string{
		"release_key":  releaseKey,
		"current_path": currentOutputDir(cfg),
	})
	if err := tx.Model(&model.PublishJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"status":        "published",
		"finished_at":   promotedAt,
		"manifest_json": manifestJSON,
		"log_json":      marshalJobLogs(*logs),
		"release_key":   releaseKey,
		"output_path":   outputPath,
		"content_path":  input.ContentPath,
	}).Error; err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "PROMOTE_FAILED", err)
		return ProcessResult{}, err
	}
	if err := tx.Commit().Error; err != nil {
		markJobFailedWithLogs(ctx, db, &job, logs, "promote", "PROMOTE_FAILED", err)
		_ = restoreCurrentDirectory(cfg, backupPath)
		return ProcessResult{}, err
	}
	committed = true
	releaseRegistered = true
	if backupPath != "" {
		_ = os.RemoveAll(backupPath)
	}

	var refreshedJob model.PublishJob
	if err := db.WithContext(ctx).Preload("Post").Preload("Page").First(&refreshedJob, "id = ?", job.ID).Error; err != nil {
		return ProcessResult{}, err
	}
	var createdRelease model.PublishRelease
	if err := db.WithContext(ctx).First(&createdRelease, "release_key = ?", releaseKey).Error; err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{Job: refreshedJob, Release: createdRelease}, nil
}

func loadPublishedAchievements(ctx context.Context, db *gorm.DB) ([]model.Achievement, error) {
	var items []model.Achievement
	err := db.WithContext(ctx).
		Preload("ImageMedia").
		Where("status = ?", "published").
		Order("occurred_at desc, sort_order asc, id asc").
		Find(&items).Error
	return items, err
}

func cleanupUnregisteredJobOutput(ctx context.Context, db *gorm.DB, cfg config.Config, jobID uuid.UUID, outputPath string) (bool, error) {
	var removed bool
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := acquirePromotionLock(tx); err != nil {
			return err
		}
		var releases []model.PublishRelease
		if err := tx.Select("id", "job_id", "release_key", "output_path").Find(&releases).Error; err != nil {
			return fmt.Errorf("load registered releases before cleaning job %s: %w", jobID, err)
		}
		var err error
		removed, err = cleanupOrphanReleaseOutput(cfg, outputPath, releases)
		return err
	})
	return removed, err
}

func cleanupOrphanReleaseOutput(cfg config.Config, outputPath string, releases []model.PublishRelease) (bool, error) {
	root, err := filepath.Abs(releaseRoot(cfg))
	if err != nil {
		return false, fmt.Errorf("resolve release root: %w", err)
	}
	candidate, err := filepath.Abs(outputPath)
	if err != nil {
		return false, fmt.Errorf("resolve release output path: %w", err)
	}
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)

	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.Dir(rel) != "." {
		return false, fmt.Errorf("refusing to clean release output outside a direct child of release root")
	}
	base := filepath.Base(candidate)
	if strings.EqualFold(base, "current") || strings.EqualFold(base, "baseline") {
		return false, fmt.Errorf("refusing to clean protected release output %q", base)
	}
	current, err := filepath.Abs(currentOutputDir(cfg))
	if err != nil {
		return false, fmt.Errorf("resolve current output path: %w", err)
	}
	current = filepath.Clean(current)
	if samePath(candidate, current) || isChildPath(candidate, current) || isChildPath(current, candidate) {
		return false, fmt.Errorf("refusing to clean release output that overlaps current")
	}
	baseline := filepath.Join(root, "baseline")
	if samePath(candidate, baseline) || isChildPath(candidate, baseline) || isChildPath(baseline, candidate) {
		return false, fmt.Errorf("refusing to clean release output that overlaps baseline")
	}

	for _, release := range releases {
		if release.ReleaseKey != "" && samePath(base, release.ReleaseKey) {
			return false, nil
		}
		registeredPath := release.OutputPath
		if registeredPath == "" && release.ReleaseKey != "" {
			registeredPath = filepath.Join(root, release.ReleaseKey)
		}
		if registeredPath == "" {
			continue
		}
		registeredAbs, resolveErr := filepath.Abs(registeredPath)
		if resolveErr != nil {
			return false, fmt.Errorf("resolve registered release %s output path: %w", release.ID, resolveErr)
		}
		if samePath(candidate, filepath.Clean(registeredAbs)) {
			return false, nil
		}
	}

	info, err := os.Lstat(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect orphan release output: %w", err)
	}
	if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return false, fmt.Errorf("refusing to clean release output that is not a directory")
	}
	if err := os.RemoveAll(candidate); err != nil {
		return false, fmt.Errorf("remove orphan release output: %w", err)
	}
	return true, nil
}

func samePath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func PromoteRelease(ctx context.Context, db *gorm.DB, cfg config.Config, releaseID uuid.UUID) (model.PublishRelease, error) {
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return model.PublishRelease{}, tx.Error
	}
	committed := false
	rollbackTx := func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}
	defer rollbackTx()

	if err := acquirePromotionLock(tx); err != nil {
		return model.PublishRelease{}, err
	}

	var release model.PublishRelease
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Post").Preload("Page").Preload("Job").
		First(&release, "id = ?", releaseID).Error; err != nil {
		return model.PublishRelease{}, err
	}
	outputPath, err := verifyReleaseOutput(cfg, release, release.Post, release.Page)
	if err != nil {
		return model.PublishRelease{}, err
	}
	settings, _, err := LoadSiteSettings(ctx, tx)
	if err != nil {
		return model.PublishRelease{}, err
	}
	if err := ValidateReleasePublicURLs(cfg.AppEnv, settings, cfg.SiteBaseURL, cfg.PublicAPIBaseURL, cfg.MediaPublicBaseURL); err != nil {
		return model.PublishRelease{}, err
	}
	if err := ValidateReleaseOutputPublicURLs(cfg.AppEnv, outputPath, cfg.SiteBaseURL, cfg.PublicAPIBaseURL); err != nil {
		return model.PublishRelease{}, err
	}

	stagedCurrent, err := stageCurrentRelease(cfg, outputPath, release.ReleaseKey)
	if err != nil {
		return model.PublishRelease{}, err
	}
	defer os.RemoveAll(stagedCurrent)

	promotedAt := time.Now()
	if err := tx.Model(&model.PublishRelease{}).Where("is_active = true").Updates(map[string]interface{}{
		"is_active": false,
		"status":    "inactive",
	}).Error; err != nil {
		return model.PublishRelease{}, err
	}
	updated := tx.Model(&model.PublishRelease{}).Where("id = ?", release.ID).Updates(map[string]interface{}{
		"is_active":   true,
		"status":      "active",
		"promoted_at": promotedAt,
		"output_path": outputPath,
	})
	if updated.Error != nil {
		return model.PublishRelease{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return model.PublishRelease{}, fmt.Errorf("release promotion affected %d candidate rows", updated.RowsAffected)
	}
	backupPath, err := switchStagedCurrent(cfg, stagedCurrent)
	if err != nil {
		return model.PublishRelease{}, err
	}
	if err := tx.Commit().Error; err != nil {
		_ = restoreCurrentDirectory(cfg, backupPath)
		return model.PublishRelease{}, err
	}
	committed = true
	if backupPath != "" {
		_ = os.RemoveAll(backupPath)
	}

	if err := db.WithContext(ctx).Preload("Post").Preload("Page").Preload("Job").First(&release, "id = ?", releaseID).Error; err != nil {
		return model.PublishRelease{}, err
	}
	return release, nil
}

func RetryJob(ctx context.Context, db *gorm.DB, cfg config.Config, jobID uuid.UUID) (model.PublishJob, error) {
	var job model.PublishJob
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", jobID).Error; err != nil {
			return err
		}
		if job.Status != "failed" && job.Status != "canceled" {
			return fmt.Errorf("publish job is not retryable")
		}
		if cfg.PublishMaxRetries > 0 && job.RetryCount >= cfg.PublishMaxRetries {
			return fmt.Errorf("publish job retry limit reached")
		}
		if err := prepareJobContentForRetry(tx, job); err != nil {
			return err
		}
		if err := ValidateJobContent(ctx, tx, job); err != nil {
			return err
		}

		now := time.Now()
		logs := parseJobLogs(job.LogJSON)
		addLog(&logs, "retry", "info", "publish job requeued", map[string]string{
			"job_id":          job.ID.String(),
			"retry_count":     strconv.Itoa(job.RetryCount + 1),
			"previous_status": job.Status,
		})
		updates := map[string]interface{}{
			"status":        "requested",
			"run_at":        now,
			"started_at":    nil,
			"finished_at":   nil,
			"release_key":   "",
			"content_path":  "",
			"output_path":   "",
			"settings_hash": "",
			"manifest_json": nil,
			"log_json":      marshalJobLogs(logs),
			"error_code":    "",
			"error_message": "",
			"canceled_at":   nil,
			"retry_count":   gorm.Expr("retry_count + 1"),
		}
		updated := tx.Model(&model.PublishJob{}).
			Where("id = ? and status in ?", jobID, []string{"failed", "canceled"}).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("publish job retry transition affected %d rows", updated.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return model.PublishJob{}, err
	}
	if err := db.WithContext(ctx).Preload("Post").Preload("Page").First(&job, "id = ?", jobID).Error; err != nil {
		return model.PublishJob{}, err
	}
	return job, nil
}

func CancelJob(ctx context.Context, db *gorm.DB, jobID uuid.UUID) (model.PublishJob, error) {
	var job model.PublishJob
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", jobID).Error; err != nil {
			return err
		}
		if job.Status != "requested" && job.Status != "queued" {
			return fmt.Errorf("publish job is not cancelable")
		}
		if err := restoreJobPublicationStateTx(tx, job); err != nil {
			return err
		}

		now := time.Now()
		logs := parseJobLogs(job.LogJSON)
		addLog(&logs, "cancel", "info", "publish job canceled", map[string]string{
			"job_id": job.ID.String(),
		})
		updates := map[string]interface{}{
			"status":        "canceled",
			"finished_at":   now,
			"canceled_at":   now,
			"log_json":      marshalJobLogs(logs),
			"error_code":    "CANCELED",
			"error_message": "publish job canceled",
		}
		if job.Status == "requested" {
			updates["run_at"] = now
		}
		updated := tx.Model(&model.PublishJob{}).
			Where("id = ? and status in ?", jobID, []string{"requested", "queued"}).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("publish job cancel transition affected %d rows", updated.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return model.PublishJob{}, err
	}
	if err := db.WithContext(ctx).Preload("Post").Preload("Page").First(&job, "id = ?", jobID).Error; err != nil {
		return model.PublishJob{}, err
	}
	return job, nil
}

func prepareJobContentForRetry(tx *gorm.DB, job model.PublishJob) error {
	switch job.JobType {
	case "post":
		if job.PostID == nil {
			return fmt.Errorf("publish job has no post id")
		}
		var post model.Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").First(&post, "id = ?", *job.PostID).Error; err != nil {
			return err
		}
		if post.Status != "published" && post.Status != "publishing" && post.Status != "draft" {
			return fmt.Errorf("post status %q is not retryable", post.Status)
		}
		return tx.Model(&model.Post{}).Where("id = ?", post.ID).Update("status", "published").Error
	case "page":
		if job.PageID == nil {
			return fmt.Errorf("publish job has no page id")
		}
		var page model.Page
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").First(&page, "id = ?", *job.PageID).Error; err != nil {
			return err
		}
		if page.Status != "published" && page.Status != "publishing" && page.Status != "draft" {
			return fmt.Errorf("page status %q is not retryable", page.Status)
		}
		return tx.Model(&model.Page{}).Where("id = ?", page.ID).Update("status", "published").Error
	default:
		return nil
	}
}

func restoreJobPublicationStateTx(tx *gorm.DB, job model.PublishJob) error {
	if job.JobType == "post" && job.PostID != nil {
		return restorePostPublicationTx(tx, *job.PostID)
	}
	if job.JobType == "page" && job.PageID != nil {
		return restorePagePublicationTx(tx, *job.PageID)
	}
	return nil
}

func restoreJobPublicationState(db *gorm.DB, job model.PublishJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), publicationRecoveryTimeout)
	defer cancel()
	return restoreJobPublicationStateTx(db.WithContext(ctx), job)
}

func hydratePostCoverMedia(ctx context.Context, db *gorm.DB, post *model.Post) error {
	if post == nil || post.CoverMediaID == nil {
		return nil
	}
	if post.CoverMedia != nil && post.CoverMedia.ID == *post.CoverMediaID && strings.TrimSpace(post.CoverMedia.PublicURL) != "" {
		return nil
	}
	var asset model.MediaAsset
	err := db.WithContext(ctx).
		Where("id = ? and status <> ?", *post.CoverMediaID, "deleted").
		First(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	post.CoverMedia = &asset
	return nil
}

func ensureJobNotCanceled(ctx context.Context, db *gorm.DB, jobID uuid.UUID) error {
	var job model.PublishJob
	if err := db.WithContext(ctx).Select("status").First(&job, "id = ?", jobID).Error; err != nil {
		return err
	}
	if job.Status == "canceled" {
		return errPublishJobCanceled
	}
	return nil
}

func markJobFailed(ctx context.Context, db *gorm.DB, job *model.PublishJob, code string, err error) {
	logs := parseJobLogs(job.LogJSON)
	markJobFailedWithLogs(ctx, db, job, &logs, "failed", code, err)
}

func markJobFailedWithLogs(ctx context.Context, db *gorm.DB, job *model.PublishJob, logs *[]PublishLogEntry, stage string, code string, err error) {
	now := time.Now()
	addLog(logs, stage, "error", err.Error(), map[string]string{"code": code})
	recoveryCtx, cancel := context.WithTimeout(context.Background(), publicationRecoveryTimeout)
	defer cancel()
	_ = db.WithContext(recoveryCtx).Model(&model.PublishJob{}).
		Where("id = ? and status in ?", job.ID, []string{"queued", "snapshotting", "building", "verifying", "promoting", "failed"}).
		Updates(map[string]interface{}{
			"status":        "failed",
			"finished_at":   now,
			"error_code":    code,
			"error_message": err.Error(),
			"log_json":      marshalJobLogs(*logs),
		}).Error
}

// ReconcileCurrentRelease repairs file-system state after a process crash or an
// uncertain database commit. The database's unique active release is authoritative.
func ReconcileCurrentRelease(ctx context.Context, db *gorm.DB, cfg config.Config) error {
	var active model.PublishRelease
	err := db.WithContext(ctx).Where("is_active = true").First(&active).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	outputPath, err := validatedReleaseOutputPath(cfg, active)
	if err != nil {
		return err
	}
	wantManifest, err := os.ReadFile(filepath.Join(outputPath, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read active release manifest: %w", err)
	}
	gotManifest, err := os.ReadFile(filepath.Join(currentOutputDir(cfg), "manifest.json"))
	if err == nil && bytes.Equal(gotManifest, wantManifest) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read current release manifest: %w", err)
	}
	staged, err := stageCurrentRelease(cfg, outputPath, active.ReleaseKey)
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	backup, err := switchStagedCurrent(cfg, staged)
	if err != nil {
		return err
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove reconciled current backup: %w", err)
		}
	}
	return nil
}

func validatedReleaseOutputPath(cfg config.Config, release model.PublishRelease) (string, error) {
	root, err := filepath.Abs(releaseRoot(cfg))
	if err != nil {
		return "", fmt.Errorf("resolve release root: %w", err)
	}
	outputPath := strings.TrimSpace(release.OutputPath)
	if outputPath == "" {
		outputPath = filepath.Join(root, release.ReleaseKey)
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("resolve active release output: %w", err)
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(outputPath))
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.Dir(rel) != "." {
		return "", fmt.Errorf("active release output is outside the release root")
	}
	if filepath.Base(outputPath) != release.ReleaseKey {
		return "", fmt.Errorf("active release output does not match release key")
	}
	if err := verifyReleasePath(outputPath, nil); err != nil {
		return "", fmt.Errorf("verify active release output: %w", err)
	}
	return filepath.Clean(outputPath), nil
}

func updateJobStage(ctx context.Context, db *gorm.DB, jobID uuid.UUID, logs *[]PublishLogEntry, expectedStatus string, status string, stage string, message string, updates map[string]interface{}) error {
	addLog(logs, stage, "info", message, nil)
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["status"] = status
	updates["log_json"] = marshalJobLogs(*logs)
	tx := db.WithContext(ctx).Model(&model.PublishJob{}).Where("id = ? and status = ?", jobID, expectedStatus).Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		if err := ensureJobNotCanceled(ctx, db, jobID); err != nil {
			return err
		}
		return fmt.Errorf("publish job stage transition failed")
	}
	return nil
}

func appendJobLog(ctx context.Context, db *gorm.DB, jobID uuid.UUID, logs *[]PublishLogEntry, stage string, level string, message string, fields map[string]string) {
	addLog(logs, stage, level, message, fields)
	_ = db.WithContext(ctx).Model(&model.PublishJob{}).Where("id = ?", jobID).Update("log_json", marshalJobLogs(*logs)).Error
}

func addLog(logs *[]PublishLogEntry, stage string, level string, message string, fields map[string]string) {
	if len(message) > maxLogMessageBytes {
		message = message[:maxLogMessageBytes] + "\n... truncated ..."
	}
	*logs = append(*logs, PublishLogEntry{
		At:      time.Now(),
		Stage:   stage,
		Level:   level,
		Message: message,
		Fields:  fields,
	})
}

func parseJobLogs(raw []byte) []PublishLogEntry {
	if len(raw) == 0 {
		return []PublishLogEntry{}
	}
	var logs []PublishLogEntry
	if err := json.Unmarshal(raw, &logs); err != nil {
		return []PublishLogEntry{}
	}
	return logs
}

func marshalJobLogs(logs []PublishLogEntry) []byte {
	value, err := json.Marshal(logs)
	if err != nil {
		return []byte("[]")
	}
	return value
}

func compactLogOutput(output []byte) string {
	value := strings.TrimSpace(string(output))
	if len(value) <= maxLogMessageBytes {
		return value
	}
	return value[:maxLogMessageBytes] + "\n... truncated ..."
}

func shortID(id uuid.UUID) string {
	value := id.String()
	if len(value) < 8 {
		return value
	}
	return value[:8]
}

func verifyReleaseOutput(cfg config.Config, release model.PublishRelease, post *model.Post, page *model.Page) (string, error) {
	outputPath := release.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(releaseRoot(cfg), release.ReleaseKey)
	}
	outputPath = filepath.Clean(outputPath)

	if !isChildPath(releaseRoot(cfg), outputPath) {
		return "", fmt.Errorf("release output path is outside release root")
	}
	if err := verifyReleaseTarget(outputPath, post, page); err != nil {
		return "", err
	}
	var manifest PublishManifest
	if err := json.Unmarshal(release.ManifestJSON, &manifest); err != nil {
		return "", fmt.Errorf("parse release manifest: %w", err)
	}
	if manifest.Scope == "withdraw_post" {
		if err := verifyReleasePathAbsent(outputPath, "/p/"+manifest.Slug+"/"); err != nil {
			return "", err
		}
	}
	if manifest.Scope == "withdraw_page" {
		if err := verifyReleasePathAbsent(outputPath, "/"+manifest.Slug+"/"); err != nil {
			return "", err
		}
	}
	return outputPath, nil
}

func verifyReleaseTarget(outputPath string, post *model.Post, page *model.Page) error {
	if err := verifyReleasePath(outputPath, post); err != nil {
		return err
	}
	if page == nil || page.Slug == "" {
		return nil
	}

	pagePath := filepath.Join(outputPath, page.Slug, "index.html")
	if info, err := os.Stat(pagePath); err != nil || info.IsDir() {
		if err != nil {
			return fmt.Errorf("release page missing: %w", err)
		}
		return fmt.Errorf("release page path is not a file")
	}
	if ok, err := fileContainsAny(pagePath, page.Title, page.Slug); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("release page does not include page %q", page.Slug)
	}
	if ok, err := sitemapContainsPath(outputPath, "/"+page.Slug+"/"); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("release sitemap does not include page %q", page.Slug)
	}
	if page.ShowInMenu {
		if ok, err := fileContainsAny(filepath.Join(outputPath, "index.html"), page.Title, page.Slug); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("release home page menu does not include page %q", page.Slug)
		}
	}
	return nil
}

func verifyReleasePathAbsent(outputPath string, expectedPath string) error {
	expectedPath = normalizeURLPath(expectedPath)
	relative := strings.Trim(expectedPath, "/")
	if relative == "" {
		return fmt.Errorf("withdrawal path is empty")
	}
	target := filepath.Join(outputPath, filepath.FromSlash(relative), "index.html")
	if !isChildPath(outputPath, target) {
		return fmt.Errorf("withdrawal path escapes release output")
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("withdrawn path still exists in release: %s", expectedPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	present, err := sitemapContainsPath(outputPath, expectedPath)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("withdrawn path still exists in release sitemap: %s", expectedPath)
	}
	return nil
}

func verifyReleasePath(outputPath string, post *model.Post) error {
	if info, err := os.Stat(outputPath); err != nil || !info.IsDir() {
		if err != nil {
			return fmt.Errorf("release output directory missing: %w", err)
		}
		return fmt.Errorf("release output path is not a directory")
	}
	for _, required := range []string{"manifest.json", "index.html", "index.xml", "sitemap.xml"} {
		if info, err := os.Stat(filepath.Join(outputPath, required)); err != nil || info.IsDir() {
			if err != nil {
				return fmt.Errorf("release %s missing: %w", required, err)
			}
			return fmt.Errorf("release %s path is not a file", required)
		}
	}

	if post == nil || post.Slug == "" {
		return nil
	}

	postPath := filepath.Join(outputPath, "p", post.Slug, "index.html")
	if info, err := os.Stat(postPath); err != nil || info.IsDir() {
		if err != nil {
			return fmt.Errorf("release article page missing: %w", err)
		}
		return fmt.Errorf("release article page path is not a file")
	}

	if ok, err := fileContainsAny(filepath.Join(outputPath, "index.html"), post.Title, post.Slug); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("release home page does not include post %q", post.Slug)
	}
	if ok, err := fileContainsAny(filepath.Join(outputPath, "index.xml"), post.Title, post.Slug); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("release RSS does not include post %q", post.Slug)
	}
	if ok, err := sitemapContainsSlug(outputPath, post.Slug); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("release sitemap does not include post %q", post.Slug)
	}

	for _, category := range post.Categories {
		if category.Slug == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(outputPath, "categories", category.Slug, "index.html")); err != nil || info.IsDir() {
			if err != nil {
				return fmt.Errorf("release category page missing for %s: %w", category.Slug, err)
			}
			return fmt.Errorf("release category page path is not a file for %s", category.Slug)
		}
	}
	for _, tag := range post.Tags {
		if tag.Slug == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(outputPath, "tags", tag.Slug, "index.html")); err != nil || info.IsDir() {
			if err != nil {
				return fmt.Errorf("release tag page missing for %s: %w", tag.Slug, err)
			}
			return fmt.Errorf("release tag page path is not a file for %s", tag.Slug)
		}
	}
	return nil
}

func fileContainsAny(path string, values ...string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	content := string(raw)
	for _, value := range values {
		if value == "" {
			continue
		}
		if strings.Contains(content, value) || strings.Contains(content, url.PathEscape(value)) {
			return true, nil
		}
	}
	return false, nil
}

func sitemapContainsSlug(outputPath string, slug string) (bool, error) {
	return sitemapContainsPath(outputPath, "/p/"+slug+"/")
}

func sitemapContainsPath(outputPath string, expected string) (bool, error) {
	foundSitemap := false
	foundSlug := false
	expectedPath := normalizeURLPath(expected)
	err := filepath.WalkDir(outputPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "sitemap.xml" {
			return nil
		}
		foundSitemap = true
		ok, err := sitemapFileContainsPath(path, expectedPath)
		if err != nil {
			return err
		}
		if ok {
			foundSlug = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if !foundSitemap {
		return false, fmt.Errorf("release sitemap missing")
	}
	return foundSlug, nil
}

func sitemapFileContainsPath(path string, expectedPath string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, fmt.Errorf("parse sitemap %s: %w", path, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "loc" {
			continue
		}
		var loc string
		if err := decoder.DecodeElement(&loc, &start); err != nil {
			return false, fmt.Errorf("parse sitemap loc %s: %w", path, err)
		}
		if sitemapLocMatchesPath(loc, expectedPath) {
			return true, nil
		}
	}

	return false, nil
}

func sitemapLocMatchesPath(loc string, expectedPath string) bool {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return false
	}
	parsed, err := url.Parse(loc)
	if err != nil {
		return false
	}
	return normalizeURLPath(parsed.EscapedPath()) == expectedPath
}

func normalizeURLPath(value string) string {
	if value == "" {
		return "/"
	}
	unescaped, err := url.PathUnescape(value)
	if err == nil {
		value = unescaped
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if !strings.HasSuffix(value, "/") {
		value += "/"
	}
	return value
}

func releaseRoot(cfg config.Config) string {
	if cfg.PublishReleaseRoot != "" {
		return filepath.Clean(cfg.PublishReleaseRoot)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(cfg.HugoPublicDir), "releases"))
}

func currentOutputDir(cfg config.Config) string {
	if cfg.PublishCurrentDir != "" {
		return filepath.Clean(cfg.PublishCurrentDir)
	}
	return filepath.Clean(cfg.HugoPublicDir)
}

func stageCurrentRelease(cfg config.Config, outputPath string, releaseKey string) (string, error) {
	currentPath := currentOutputDir(cfg)
	outputPath = filepath.Clean(outputPath)
	if currentPath == outputPath || isChildPath(outputPath, currentPath) {
		return "", fmt.Errorf("current site path must not be inside the release output path")
	}
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o755); err != nil {
		return "", err
	}

	stagedPath := filepath.Join(filepath.Dir(currentPath), filepath.Base(currentPath)+".next-"+releaseKey)
	if err := os.RemoveAll(stagedPath); err != nil {
		return "", err
	}
	if err := copyDir(outputPath, stagedPath); err != nil {
		_ = os.RemoveAll(stagedPath)
		return "", err
	}
	if err := verifyReleasePath(stagedPath, nil); err != nil {
		_ = os.RemoveAll(stagedPath)
		return "", err
	}
	return stagedPath, nil
}

func switchStagedCurrent(cfg config.Config, stagedPath string) (string, error) {
	currentPath := currentOutputDir(cfg)
	if filepath.Clean(stagedPath) == currentPath {
		return "", fmt.Errorf("staged current path equals current path")
	}

	var backupPath string
	if info, err := os.Lstat(currentPath); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("current site path exists and is not a directory")
		}
		backupPath = filepath.Join(filepath.Dir(currentPath), filepath.Base(currentPath)+".previous-"+time.Now().Format("20060102150405"))
		if err := os.RemoveAll(backupPath); err != nil {
			return "", err
		}
		if err := os.Rename(currentPath, backupPath); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := os.Rename(stagedPath, currentPath); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, currentPath)
		}
		return "", err
	}
	return backupPath, nil
}

func restoreCurrentDirectory(cfg config.Config, backupPath string) error {
	if backupPath == "" {
		return os.RemoveAll(currentOutputDir(cfg))
	}
	currentPath := currentOutputDir(cfg)
	if err := os.RemoveAll(currentPath); err != nil {
		return err
	}
	if err := os.Rename(backupPath, currentPath); err != nil {
		return err
	}
	return nil
}

func copyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("release output contains unsupported symlink: %s", path)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src string, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return nil
}

func isChildPath(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func acquirePromotionLock(tx *gorm.DB) error {
	return tx.Exec("select pg_advisory_xact_lock(?)", publishPromotionLockID).Error
}
