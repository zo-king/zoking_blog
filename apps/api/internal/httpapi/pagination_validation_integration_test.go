package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

func TestAdminPaginationUUIDFiltersRejectMalformedValues(t *testing.T) {
	db := openHTTPAPIPostgresTestSchema(t, "pagination_validation")
	if err := db.AutoMigrate(
		&model.MediaAsset{},
		&model.Category{},
		&model.Tag{},
		&model.Post{},
		&model.Comment{},
	); err != nil {
		t.Fatalf("migrate pagination validation schema: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		handler gin.HandlerFunc
	}{
		{name: "post category", path: "/posts?category_id=not-a-uuid", handler: listAdminPosts(db)},
		{name: "post tag", path: "/posts?tag_id=not-a-uuid", handler: listAdminPosts(db)},
		{name: "comment post", path: "/comments?post_id=not-a-uuid", handler: listAdminComments(db)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)
			context.Set("user_id", "90000000-0000-0000-0000-000000000001")
			context.Set("permissions", []string{"content:read_all"})

			tt.handler(context)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Error ErrorBody `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if response.Error.Code != "VALIDATION_FAILED" {
				t.Fatalf("error code = %q, want VALIDATION_FAILED", response.Error.Code)
			}
		})
	}
}

func TestAuditActorIDFilterUsesParsedUUID(t *testing.T) {
	db := openHTTPAPIPostgresTestSchema(t, "pagination_audit_uuid")
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate audit schema: %v", err)
	}

	actorID := uuid.MustParse("60000000-0000-0000-0000-000000000001")
	entry := model.AuditLog{
		ID:            uuid.MustParse("60000000-0000-0000-0000-000000000002"),
		ActorID:       &actorID,
		ActorEmail:    "audit-uuid@example.test",
		Action:        "posts.update",
		ResourceType:  "posts",
		Route:         "/api/v1/admin/posts/:id",
		Method:        http.MethodPatch,
		Result:        "success",
		StatusCode:    http.StatusOK,
		RequestID:     "audit-uuid-filter",
		IPHashVersion: 1,
		CreatedAt:     time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("seed audit entry: %v", err)
	}

	router := gin.New()
	router.GET("/audit-logs", listAuditLogs(db))
	requestPath := "/audit-logs?actor_id=" + url.QueryEscape("urn:uuid:"+actorID.String())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data       []model.AuditLog `json:"data"`
		Pagination paginationMeta   `json:"pagination"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != entry.ID || response.Pagination.Total != 1 {
		t.Fatalf("unexpected audit response: %#v", response)
	}
}

func TestMediaExactFiltersRejectEmptyAndPreservePublicURLBytes(t *testing.T) {
	db := openHTTPAPIPostgresTestSchema(t, "pagination_media_exact")
	if err := db.AutoMigrate(&model.MediaAsset{}, &model.MediaUsage{}); err != nil {
		t.Fatalf("migrate media schema: %v", err)
	}
	media := model.MediaAsset{
		Base:          model.Base{ID: uuid.MustParse("70000000-0000-0000-0000-000000000001")},
		Filename:      "exact.png",
		OriginalName:  "exact.png",
		MimeType:      "image/png",
		StorageDriver: "local",
		StorageKey:    "exact.png",
		PublicURL:     "https://cdn.example.test/exact.png",
		Checksum:      strings.Repeat("a", 64),
		Status:        "ready",
	}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("seed media: %v", err)
	}

	router := gin.New()
	router.GET("/media", listAdminMedia(db))
	for _, path := range []string{
		"/media?checksum=",
		"/media?checksum=%20",
		"/media?public_url=",
		"/media?public_url=%20",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}

	query := url.Values{"public_url": {" " + media.PublicURL + " "}}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/media?"+query.Encode(), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []model.MediaAsset `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode media response: %v", err)
	}
	if len(response.Data) != 0 {
		t.Fatalf("whitespace-altered public_url matched media: %#v", response.Data)
	}
}
