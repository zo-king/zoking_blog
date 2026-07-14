package httpapi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

func TestMediaReferenceKeySharePreventsConcurrentDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openHTTPAPIPostgresTestSchema(t, "media_lock")
	createMediaLockIntegrationTables(t, db)

	mediaID := uuid.New()
	storageKey := "2026/07/concurrent-reference.png"
	if err := db.Exec(`
		insert into media_assets (id, storage_driver, storage_key, status)
		values (?, 'local', ?, 'ready')
	`, mediaID, storageKey).Error; err != nil {
		t.Fatalf("create media fixture: %v", err)
	}

	mediaRoot := filepath.Join(t.TempDir(), "media")
	mediaPath := filepath.Join(mediaRoot, filepath.FromSlash(storageKey))
	if err := os.MkdirAll(filepath.Dir(mediaPath), 0o755); err != nil {
		t.Fatalf("create media fixture directory: %v", err)
	}
	const payload = "media that must remain available"
	if err := os.WriteFile(mediaPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("create media fixture file: %v", err)
	}

	referenceTx := db.Begin()
	if referenceTx.Error != nil {
		t.Fatalf("begin reference transaction: %v", referenceTx.Error)
	}
	referenceFinished := false
	t.Cleanup(func() {
		if !referenceFinished {
			_ = referenceTx.Rollback().Error
		}
	})

	asset, err := mediaref.ResolveReadyMedia(referenceTx, mediaID.String())
	if err != nil {
		t.Fatalf("resolve ready media with key-share lock: %v", err)
	}
	if asset.ID != mediaID {
		t.Fatalf("resolved media ID = %s, want %s", asset.ID, mediaID)
	}

	deleteResult := make(chan deleteMediaIntegrationResult, 1)
	go func() {
		response := performDeleteMediaRequest(t, db, mediaRoot, mediaID)
		deleteResult <- deleteMediaIntegrationResult{
			status: response.Code,
			body:   response.Body.String(),
		}
	}()

	select {
	case result := <-deleteResult:
		t.Fatalf("delete completed before reference transaction committed: status=%d body=%s", result.status, result.body)
	case <-time.After(300 * time.Millisecond):
		// FOR UPDATE must wait for the reference transaction's FOR KEY SHARE lock.
	}

	usage := model.MediaUsage{
		Base:         model.Base{ID: uuid.New()},
		MediaID:      mediaID,
		ResourceType: mediaref.ResourcePost,
		ResourceID:   uuid.New(),
		UsageType:    mediaref.UsageCover,
	}
	if err := referenceTx.Create(&usage).Error; err != nil {
		t.Fatalf("create media usage while holding key-share lock: %v", err)
	}
	if err := referenceTx.Commit().Error; err != nil {
		t.Fatalf("commit reference transaction: %v", err)
	}
	referenceFinished = true

	var result deleteMediaIntegrationResult
	select {
	case result = <-deleteResult:
	case <-time.After(5 * time.Second):
		t.Fatal("delete did not resume after reference transaction committed")
	}
	if result.status != http.StatusConflict || !strings.Contains(result.body, "media is still referenced") {
		t.Fatalf("delete status/body = %d/%s, want 409 media-is-referenced conflict", result.status, result.body)
	}

	var persisted model.MediaAsset
	if err := db.First(&persisted, "id = ?", mediaID).Error; err != nil {
		t.Fatalf("media record was deleted after concurrent reference: %v", err)
	}
	if persisted.Status != "ready" || persisted.DeletedAt.Valid {
		t.Fatalf("media record status/deleted_at = %q/%v, want ready/not deleted", persisted.Status, persisted.DeletedAt.Valid)
	}

	var usageCount int64
	if err := db.Model(&model.MediaUsage{}).Where("media_id = ?", mediaID).Count(&usageCount).Error; err != nil {
		t.Fatalf("count committed media usages: %v", err)
	}
	if usageCount != 1 {
		t.Fatalf("committed media usage count = %d, want 1", usageCount)
	}

	gotPayload, err := os.ReadFile(mediaPath)
	if err != nil {
		t.Fatalf("media file was removed after concurrent reference: %v", err)
	}
	if string(gotPayload) != payload {
		t.Fatalf("media file payload = %q, want %q", gotPayload, payload)
	}
	quarantineDir := filepath.Join(mediaRoot, mediaPrivateDirName, mediaQuarantineDirName)
	entries, err := os.ReadDir(quarantineDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read media quarantine: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("media quarantine contains artifacts after rejected delete: %v", entries)
	}
}

type deleteMediaIntegrationResult struct {
	status int
	body   string
}

func createMediaLockIntegrationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		create table media_assets (
			id uuid primary key,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			deleted_at timestamptz,
			storage_driver text not null,
			storage_key text not null,
			status text not null
		)
	`).Error; err != nil {
		t.Fatalf("create media_assets table: %v", err)
	}
	if err := db.Exec(`
		create table media_usages (
			id uuid primary key,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			deleted_at timestamptz,
			media_id uuid not null references media_assets(id),
			resource_type text not null,
			resource_id uuid not null,
			usage_type text not null
		)
	`).Error; err != nil {
		t.Fatalf("create media_usages table: %v", err)
	}
}
