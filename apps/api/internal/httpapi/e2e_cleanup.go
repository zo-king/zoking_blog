package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/maintenance"
	"gorm.io/gorm"
)

type e2eCleanupRequest struct {
	DryRun   *bool                      `json:"dry_run"`
	Manifest maintenance.E2ERunManifest `json:"manifest" binding:"required"`
}

func e2eCleanupEnabled(cfg config.Config) bool {
	return cfg.AppEnv == "test" && cfg.QAE2ECleanupEnabled
}

func cleanupE2ERun(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		runID, err := uuid.Parse(c.Param("run_id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid E2E run id")
			return
		}
		var req e2eCleanupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid E2E cleanup manifest")
			return
		}
		if req.Manifest.RunID != runID {
			Fail(c, http.StatusConflict, "E2E_MANIFEST_CONFLICT", "manifest run_id does not match route")
			return
		}
		dryRun := true
		if req.DryRun != nil {
			dryRun = *req.DryRun
		}
		result, err := maintenance.CleanupE2ERun(c.Request.Context(), db, cfg, req.Manifest, dryRun)
		if err != nil {
			var validationErr *maintenance.E2EValidationError
			if errors.As(err, &validationErr) {
				Fail(c, http.StatusConflict, "E2E_MANIFEST_CONFLICT", validationErr.Error())
				return
			}
			Fail(c, http.StatusConflict, "E2E_CLEANUP_FAILED", err.Error())
			return
		}
		OK(c, result)
	}
}
