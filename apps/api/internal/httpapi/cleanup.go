package httpapi

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/maintenance"
	"gorm.io/gorm"
)

type cleanupRequest struct {
	DryRun             *bool `json:"dry_run"`
	OrphanGraceSeconds *int  `json:"orphan_grace_seconds"`
	BatchSize          *int  `json:"batch_size"`
	KeepLatest         *int  `json:"keep_latest"`
	KeepDays           *int  `json:"keep_days"`
	PreviewBatchSize   *int  `json:"preview_batch_size"`
}

func cleanupPublishPreviews(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		req, ok := bindCleanupRequest(c)
		if !ok {
			return
		}
		dryRun := true
		if req.DryRun != nil {
			dryRun = *req.DryRun
		}
		batchSize := cfg.PublishPreviewCleanupBatchSize
		if req.PreviewBatchSize != nil {
			if *req.PreviewBatchSize < 1 || *req.PreviewBatchSize > 500 {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "preview_batch_size must be between 1 and 500")
				return
			}
			batchSize = *req.PreviewBatchSize
		}
		result, err := maintenance.CleanupExpiredPreviews(c.Request.Context(), db, cfg, maintenance.PreviewCleanupOptions{DryRun: dryRun, BatchSize: batchSize})
		if err != nil {
			Fail(c, http.StatusInternalServerError, "PREVIEW_CLEANUP_FAILED", "preview cleanup failed")
			return
		}
		OK(c, result)
	}
}

func cleanupOrphanMedia(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		req, ok := bindCleanupRequest(c)
		if !ok {
			return
		}
		dryRun := true
		if req.DryRun != nil {
			dryRun = *req.DryRun
		}
		options := maintenance.MediaCleanupOptions{
			DryRun:                dryRun,
			UseDefaultGracePeriod: req.OrphanGraceSeconds == nil,
			BatchSize:             cfg.MediaCleanupBatchSize,
		}
		if req.OrphanGraceSeconds != nil {
			if *req.OrphanGraceSeconds < 0 {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "orphan_grace_seconds must be greater than or equal to 0")
				return
			}
			options.GracePeriod = time.Duration(*req.OrphanGraceSeconds) * time.Second
		}
		if req.BatchSize != nil {
			if *req.BatchSize < 1 || *req.BatchSize > 500 {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "batch_size must be between 1 and 500")
				return
			}
			options.BatchSize = *req.BatchSize
		}

		result, err := maintenance.CleanupOrphanMedia(c.Request.Context(), db, cfg, options)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "MEDIA_CLEANUP_FAILED", "media cleanup failed")
			return
		}
		OK(c, result)
	}
}

func cleanupPublishReleases(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		req, ok := bindCleanupRequest(c)
		if !ok {
			return
		}
		dryRun := true
		if req.DryRun != nil {
			dryRun = *req.DryRun
		}
		options := maintenance.ReleaseCleanupOptions{
			DryRun:               dryRun,
			UseDefaultKeepLatest: req.KeepLatest == nil,
			UseDefaultKeepDays:   req.KeepDays == nil,
		}
		if req.KeepLatest != nil {
			if *req.KeepLatest < 1 || *req.KeepLatest > 200 {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "keep_latest must be between 1 and 200")
				return
			}
			options.KeepLatest = *req.KeepLatest
		}
		if req.KeepDays != nil {
			if *req.KeepDays < 0 || *req.KeepDays > 3650 {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "keep_days must be between 0 and 3650")
				return
			}
			options.KeepDays = *req.KeepDays
		}

		result, err := maintenance.CleanupOldReleases(c.Request.Context(), db, cfg, options)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "RELEASE_CLEANUP_FAILED", "release cleanup failed")
			return
		}
		OK(c, result)
	}
}

func bindCleanupRequest(c *gin.Context) (cleanupRequest, bool) {
	var req cleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid cleanup payload")
		return req, false
	}
	return req, true
}
