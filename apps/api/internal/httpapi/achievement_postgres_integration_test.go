package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

func TestAchievementPostgresContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openHTTPAPIPostgresTestSchema(t, "achievement_contract")
	applyAchievementIntegrationMigration(t, db)

	readyMedia := achievementTestMedia("ready")
	processingMedia := achievementTestMedia("processing")
	if err := db.Create(&[]model.MediaAsset{readyMedia, processingMedia}).Error; err != nil {
		t.Fatalf("seed media: %v", err)
	}

	router := gin.New()
	router.GET("/achievements", listAdminAchievements(db))
	router.GET("/achievements/:id", getAdminAchievement(db))
	router.POST("/achievements", createAchievement(db))
	router.PATCH("/achievements/:id", updateAchievement(db))
	router.PATCH("/achievements/:id/status", updateAchievementStatus(db))
	router.DELETE("/achievements/:id", deleteAchievement(db))

	created := achievementRequest(t, router, http.MethodPost, "/achievements", `{
		"kind":"certificate",
		"title":"PostgreSQL Associate",
		"organization":"Example Foundation",
		"summary":"Database credential",
		"occurred_at":"2026-07-13",
		"ended_at":"2027-07-13",
		"external_url":"https://example.test/credentials/pg",
		"credential_id":"PG-123",
		"image_media_id":"`+readyMedia.ID.String()+`",
		"sort_order":2,
		"status":"published"
	}`, http.StatusCreated)
	if created.Status != "draft" || created.PublishedAt != nil {
		t.Fatalf("create status/published_at = %q/%v, want draft/nil", created.Status, created.PublishedAt)
	}
	if created.ImageMedia == nil || created.ImageMedia.ID != readyMedia.ID {
		t.Fatalf("created image_media = %#v", created.ImageMedia)
	}
	assertAchievementUsageCount(t, db, created.ID, readyMedia.ID, 1)

	t.Run("detail preloads media", func(t *testing.T) {
		item := achievementRequest(t, router, http.MethodGet, "/achievements/"+created.ID.String(), "", http.StatusOK)
		if item.ImageMedia == nil || item.ImageMedia.ID != readyMedia.ID {
			t.Fatalf("detail image_media = %#v", item.ImageMedia)
		}
	})

	t.Run("content patch clears nullable fields and media usage", func(t *testing.T) {
		item := achievementRequest(t, router, http.MethodPatch, "/achievements/"+created.ID.String(), `{
			"title":"PostgreSQL Professional",
			"ended_at":null,
			"image_media_id":null,
			"sort_order":0
		}`, http.StatusOK)
		if item.Title != "PostgreSQL Professional" || item.EndedAt != nil || item.ImageMediaID != nil || item.SortOrder != 0 {
			t.Fatalf("patched item = %#v", item)
		}
		assertAchievementUsageCount(t, db, created.ID, readyMedia.ID, 0)
	})

	t.Run("only ready media can be selected", func(t *testing.T) {
		response := achievementServe(router, http.MethodPatch, "/achievements/"+created.ID.String(),
			`{"image_media_id":"`+processingMedia.ID.String()+`"}`)
		assertAchievementError(t, response, http.StatusNotFound, "MEDIA_NOT_FOUND")
	})

	t.Run("effective date range is validated", func(t *testing.T) {
		achievementRequest(t, router, http.MethodPatch, "/achievements/"+created.ID.String(),
			`{"ended_at":"2026-08-01"}`, http.StatusOK)
		response := achievementServe(router, http.MethodPatch, "/achievements/"+created.ID.String(),
			`{"occurred_at":"2026-09-01"}`)
		assertAchievementError(t, response, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	})

	other := createAchievementFixture(t, db, "00000000-0000-0000-0000-000000000010", "Earlier project", "project", "2025-03-01", 0)
	later := createAchievementFixture(t, db, "00000000-0000-0000-0000-000000000011", "Later award", "award", "2026-12-01", 4)
	sameDayFirst := createAchievementFixture(t, db, "00000000-0000-0000-0000-000000000012", "Same day first", "award", "2026-12-01", 1)

	t.Run("list filters paginate and use stable timeline order", func(t *testing.T) {
		response := achievementServe(router, http.MethodGet, "/achievements?year=2026&status=draft&page=1&page_size=10", "")
		if response.Code != http.StatusOK {
			t.Fatalf("list status = %d body=%s", response.Code, response.Body.String())
		}
		var envelope achievementListEnvelope
		decodeAchievementResponse(t, response, &envelope)
		if envelope.Pagination.Total != 3 {
			t.Fatalf("list total = %d, want 3", envelope.Pagination.Total)
		}
		assertAchievementIDs(t, envelope.Data, sameDayFirst.ID, later.ID, created.ID)

		queryResponse := achievementServe(router, http.MethodGet, "/achievements?q=Earlier&year=2025", "")
		decodeAchievementResponse(t, queryResponse, &envelope)
		assertAchievementIDs(t, envelope.Data, other.ID)
	})

	t.Run("invalid list parameters are rejected", func(t *testing.T) {
		for _, path := range []string{
			"/achievements?year=2023",
			"/achievements?status=deleted",
			"/achievements?sort=unknown",
		} {
			response := achievementServe(router, http.MethodGet, path, "")
			assertAchievementError(t, response, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
		}
	})

	t.Run("status controls published timestamp and deletion", func(t *testing.T) {
		published := achievementRequest(t, router, http.MethodPatch, "/achievements/"+created.ID.String()+"/status",
			`{"status":"published"}`, http.StatusOK)
		if published.PublishedAt == nil {
			t.Fatal("published_at = nil after publishing")
		}

		response := achievementServe(router, http.MethodDelete, "/achievements/"+created.ID.String(), "")
		assertAchievementError(t, response, http.StatusConflict, "ACHIEVEMENT_PUBLISHED")

		archived := achievementRequest(t, router, http.MethodPatch, "/achievements/"+created.ID.String()+"/status",
			`{"status":"archived"}`, http.StatusOK)
		if archived.PublishedAt == nil {
			t.Fatal("archiving should retain published_at")
		}
		deleted := achievementServe(router, http.MethodDelete, "/achievements/"+created.ID.String(), "")
		if deleted.Code != http.StatusOK {
			t.Fatalf("delete archived status = %d body=%s", deleted.Code, deleted.Body.String())
		}
		missing := achievementServe(router, http.MethodGet, "/achievements/"+created.ID.String(), "")
		assertAchievementError(t, missing, http.StatusNotFound, "ACHIEVEMENT_NOT_FOUND")
	})

	t.Run("migration constraints reject invalid direct writes", func(t *testing.T) {
		invalidKind := model.Achievement{Kind: "other", Title: "Invalid", OccurredAt: mustAchievementTestDate(t, "2026-01-01"), Status: "draft"}
		if err := db.Create(&invalidKind).Error; postgresConstraint(err) != "achievements_kind_allowed" {
			t.Fatalf("invalid kind constraint = %q err=%v", postgresConstraint(err), err)
		}
		invalidDate := model.Achievement{Kind: "award", Title: "Too old", OccurredAt: mustAchievementTestDate(t, "2023-12-31"), Status: "draft"}
		if err := db.Create(&invalidDate).Error; postgresConstraint(err) != "achievements_occurred_at_minimum" {
			t.Fatalf("old date constraint = %q err=%v", postgresConstraint(err), err)
		}
	})
}

type achievementListEnvelope struct {
	Data       []model.Achievement `json:"data"`
	Pagination paginationMeta      `json:"pagination"`
}

func applyAchievementIntegrationMigration(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&model.MediaAsset{}, &model.MediaUsage{}); err != nil {
		t.Fatalf("auto-migrate achievement dependencies: %v", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve achievement integration test path")
	}
	migrationPath := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../db/migrations/20260713000200_create_achievements.sql"))
	raw, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read achievement migration: %v", err)
	}
	upSQL := strings.SplitN(string(raw), "-- +goose Down", 2)[0]
	if err := db.Exec(upSQL).Error; err != nil {
		t.Fatalf("apply achievement migration: %v", err)
	}
}

func achievementTestMedia(status string) model.MediaAsset {
	id := uuid.New()
	return model.MediaAsset{
		Base:          model.Base{ID: id},
		Filename:      status + "-achievement.png",
		OriginalName:  status + "-achievement.png",
		MimeType:      "image/png",
		StorageDriver: "local",
		StorageKey:    "achievements/" + id.String() + ".png",
		PublicURL:     "https://cdn.example.test/" + id.String() + ".png",
		Checksum:      id.String(),
		Status:        status,
	}
}

func createAchievementFixture(t *testing.T, db *gorm.DB, id, title, kind, occurredAt string, sortOrder int) model.Achievement {
	t.Helper()
	item := model.Achievement{
		Base:       model.Base{ID: uuid.MustParse(id)},
		Kind:       kind,
		Title:      title,
		OccurredAt: mustAchievementTestDate(t, occurredAt),
		SortOrder:  sortOrder,
		Status:     "draft",
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create achievement fixture %q: %v", title, err)
	}
	return item
}

func achievementRequest(t *testing.T, router http.Handler, method, target, body string, wantStatus int) model.Achievement {
	t.Helper()
	response := achievementServe(router, method, target, body)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, target, response.Code, wantStatus, response.Body.String())
	}
	var envelope struct {
		Data model.Achievement `json:"data"`
	}
	decodeAchievementResponse(t, response, &envelope)
	return envelope.Data
}

func achievementServe(router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func assertAchievementError(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, wantStatus, response.Body.String())
	}
	var envelope struct {
		Error ErrorBody `json:"error"`
	}
	decodeAchievementResponse(t, response, &envelope)
	if envelope.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q; body=%s", envelope.Error.Code, wantCode, response.Body.String())
	}
}

func assertAchievementUsageCount(t *testing.T, db *gorm.DB, achievementID, mediaID uuid.UUID, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.MediaUsage{}).
		Where("resource_type = ? and resource_id = ? and usage_type = ? and media_id = ?",
			mediaref.ResourceAchievement, achievementID, mediaref.UsageImage, mediaID).
		Count(&count).Error; err != nil {
		t.Fatalf("count achievement media usages: %v", err)
	}
	if count != want {
		t.Fatalf("achievement media usage count = %d, want %d", count, want)
	}
}

func assertAchievementIDs(t *testing.T, items []model.Achievement, want ...uuid.UUID) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("achievement count = %d, want %d; items=%#v", len(items), len(want), items)
	}
	for i := range want {
		if items[i].ID != want[i] {
			t.Fatalf("achievement[%d].id = %s, want %s", i, items[i].ID, want[i])
		}
	}
}

func decodeAchievementResponse(t *testing.T, response *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}

func TestAchievementStatusAndKindValidation(t *testing.T) {
	for _, value := range []string{"award", "certificate", "project"} {
		if !validAchievementKind(value) {
			t.Fatalf("kind %q should be valid", value)
		}
	}
	for _, value := range []string{"draft", "published", "archived"} {
		if !validAchievementStatus(value) {
			t.Fatalf("status %q should be valid", value)
		}
	}
	if validAchievementKind("badge") || validAchievementStatus("deleted") {
		t.Fatal("unknown kind/status should be invalid")
	}
}

func TestAchievementDateJSONUsesExistingTimeContract(t *testing.T) {
	item := model.Achievement{OccurredAt: time.Date(2026, time.July, 13, 0, 0, 0, 0, time.UTC)}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal achievement: %v", err)
	}
	if !strings.Contains(string(raw), `"occurred_at":"2026-07-13T00:00:00Z"`) {
		t.Fatalf("achievement date JSON = %s", raw)
	}
}
