package maintenance

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

type PreviewCleanupOptions struct {
	DryRun    bool
	BatchSize int
	Now       time.Time
}

func CleanupExpiredPreviews(ctx context.Context, db *gorm.DB, cfg config.Config, options PreviewCleanupOptions) (CleanupResult, error) {
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	if options.BatchSize <= 0 {
		options.BatchSize = cfg.PublishPreviewCleanupBatchSize
	}
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}
	if options.BatchSize > 500 {
		options.BatchSize = 500
	}
	result := CleanupResult{DryRun: options.DryRun, Cutoff: options.Now, Items: []CleanupItem{}}
	var previews []model.PublishPreview
	if err := db.WithContext(ctx).
		Where("status in ? and expires_at is not null and expires_at <= ?", []string{"ready", "failed"}, options.Now).
		Order("expires_at asc").
		Limit(options.BatchSize).
		Find(&previews).Error; err != nil {
		return result, err
	}

	root := filepath.Clean(cfg.PublishPreviewRoot)
	if dangerousPreviewRoot(root, cfg) {
		return result, os.ErrInvalid
	}
	for _, preview := range previews {
		path, ok := safeChildPath(root, preview.PreviewKey)
		item := CleanupItem{ID: preview.ID, Key: preview.PreviewKey, Path: path, Reason: "preview_ttl_expired", Action: "candidate", CreatedAt: preview.CreatedAt}
		result.CandidateCount++
		if !ok || filepath.Clean(path) == root {
			item.Action = "skipped"
			item.Reason = "unsafe_preview_path"
			result.SkippedCount++
			result.Items = append(result.Items, item)
			continue
		}
		if options.DryRun {
			result.Items = append(result.Items, item)
			continue
		}
		claimResult := db.WithContext(ctx).Model(&model.PublishPreview{}).
			Where("id = ? and status in ? and expires_at is not null and expires_at <= ?", preview.ID, []string{"ready", "failed"}, options.Now).
			Update("status", "cleaning")
		if claimResult.Error != nil {
			return result, claimResult.Error
		}
		if claimResult.RowsAffected != 1 {
			item.Action = "skipped"
			item.Reason = "preview_changed"
			result.SkippedCount++
			result.Items = append(result.Items, item)
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			_ = db.WithContext(ctx).Model(&model.PublishPreview{}).Where("id = ? and status = ?", preview.ID, "cleaning").Updates(map[string]interface{}{"status": "cleanup_failed", "error_code": "PREVIEW_DELETE_FAILED", "error_message": err.Error()}).Error
			return result, err
		}
		if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
			if err == nil {
				err = os.ErrExist
			}
			_ = db.WithContext(ctx).Model(&model.PublishPreview{}).Where("id = ? and status = ?", preview.ID, "cleaning").Updates(map[string]interface{}{"status": "cleanup_failed", "error_code": "PREVIEW_DELETE_FAILED", "error_message": err.Error()}).Error
			return result, err
		}
		if err := db.WithContext(ctx).Model(&model.PublishPreview{}).Where("id = ? and status = ?", preview.ID, "cleaning").Updates(map[string]interface{}{"status": "expired", "output_path": "", "error_code": "", "error_message": ""}).Error; err != nil {
			return result, err
		}
		item.Action = "deleted"
		result.DeletedCount++
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func dangerousPreviewRoot(root string, cfg config.Config) bool {
	if root == "" || filepath.Clean(filepath.VolumeName(root)+string(filepath.Separator)) == root {
		return true
	}
	protected := []string{cfg.PublishReleaseRoot, cfg.PublishCurrentDir, cfg.MediaLocalDir, cfg.HugoSiteDir, cfg.HugoPublicDir}
	for _, value := range protected {
		if previewPathsOverlap(root, value) {
			return true
		}
	}
	return false
}

func previewPathsOverlap(left string, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return previewPathWithinOrEqual(left, right) || previewPathWithinOrEqual(right, left)
}

func previewPathWithinOrEqual(root string, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func RunPreviewCleanup(ctx context.Context, db *gorm.DB, cfg config.Config, logger *log.Logger) {
	interval := cfg.PublishPreviewCleanupInterval
	if interval <= 0 {
		return
	}
	run := func() {
		result, err := CleanupExpiredPreviews(ctx, db, cfg, PreviewCleanupOptions{DryRun: false})
		if err != nil {
			logger.Printf("preview cleanup failed: %v", err)
			return
		}
		if result.DeletedCount > 0 || result.SkippedCount > 0 {
			logger.Printf("preview cleanup completed: deleted=%d skipped=%d", result.DeletedCount, result.SkippedCount)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
