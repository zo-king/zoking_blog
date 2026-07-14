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
	"gorm.io/gorm"
)

func TestListAdminMediaPostgresPaginationAndExactQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openHTTPAPIPostgresTestSchema(t, "pagination_media")
	if err := db.AutoMigrate(&model.MediaAsset{}, &model.MediaUsage{}); err != nil {
		t.Fatalf("auto-migrate media tables: %v", err)
	}

	createdAt := time.Date(2026, time.July, 13, 8, 0, 0, 0, time.UTC)
	checksum := strings.Repeat("a", 64)
	media := []model.MediaAsset{
		newPaginationTestMedia("00000000-0000-0000-0000-000000000001", "campaign-one.jpg", "Campaign One.jpg", "https://cdn.example.test/campaign-one.jpg", checksum, "ready", createdAt),
		newPaginationTestMedia("00000000-0000-0000-0000-000000000002", "campaign-two.jpg", "Campaign Two.jpg", "https://cdn.example.test/campaign-two.jpg", strings.Repeat("b", 64), "ready", createdAt),
		newPaginationTestMedia("00000000-0000-0000-0000-000000000003", "campaign-three.jpg", "Campaign Three.jpg", "https://cdn.example.test/campaign-three.jpg", strings.Repeat("c", 64), "ready", createdAt),
		newPaginationTestMedia("00000000-0000-0000-0000-000000000004", "campaign-processing.jpg", "Campaign Processing.jpg", "https://cdn.example.test/campaign-processing.jpg", strings.Repeat("d", 64), "processing", createdAt),
		newPaginationTestMedia("00000000-0000-0000-0000-000000000005", "other.jpg", "Other.jpg", "https://cdn.example.test/other.jpg", strings.Repeat("e", 64), "ready", createdAt),
		newPaginationTestMedia("00000000-0000-0000-0000-000000000006", "deleted-copy.jpg", "Deleted Campaign.jpg", "https://cdn.example.test/deleted-copy.jpg", checksum, "deleted", createdAt.Add(-time.Hour)),
	}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("seed media: %v", err)
	}

	deletedAt := createdAt.Add(time.Minute)
	usages := []model.MediaUsage{
		newPaginationTestUsage("10000000-0000-0000-0000-000000000001", media[0].ID, "20000000-0000-0000-0000-000000000001", "cover", nil),
		newPaginationTestUsage("10000000-0000-0000-0000-000000000002", media[0].ID, "20000000-0000-0000-0000-000000000002", "body", nil),
		newPaginationTestUsage("10000000-0000-0000-0000-000000000003", media[0].ID, "20000000-0000-0000-0000-000000000003", "body", &deletedAt),
	}
	if err := db.Create(&usages).Error; err != nil {
		t.Fatalf("seed media usages: %v", err)
	}

	router := gin.New()
	router.GET("/media", listAdminMedia(db))

	t.Run("checksum exact query preserves array envelope and usage count", func(t *testing.T) {
		response := performPaginationIntegrationRequest(t, router, "/media?checksum="+strings.ToUpper(checksum))
		assertHTTPStatus(t, response, http.StatusOK)
		items := decodeLegacyMediaEnvelope(t, response)
		if len(items) != 1 || items[0].ID != media[0].ID {
			t.Fatalf("checksum data = %#v, want media %s", items, media[0].ID)
		}
		if items[0].UsageCount != 2 {
			t.Fatalf("checksum usage_count = %d, want 2", items[0].UsageCount)
		}
	})

	t.Run("public URL exact query preserves array envelope", func(t *testing.T) {
		query := url.Values{"public_url": []string{media[1].PublicURL}}
		response := performPaginationIntegrationRequest(t, router, "/media?"+query.Encode())
		assertHTTPStatus(t, response, http.StatusOK)
		items := decodeLegacyMediaEnvelope(t, response)
		if len(items) != 1 || items[0].ID != media[1].ID {
			t.Fatalf("public_url data = %#v, want media %s", items, media[1].ID)
		}
	})

	t.Run("exact query validation", func(t *testing.T) {
		queries := []struct {
			name string
			path string
		}{
			{
				name: "checksum and public URL",
				path: "/media?checksum=" + checksum + "&public_url=" + url.QueryEscape(media[0].PublicURL),
			},
			{name: "invalid checksum", path: "/media?checksum=not-a-sha256"},
		}
		for _, test := range queries {
			t.Run(test.name, func(t *testing.T) {
				response := performPaginationIntegrationRequest(t, router, test.path)
				assertHTTPStatus(t, response, http.StatusUnprocessableEntity)
				assertErrorCode(t, response, "VALIDATION_FAILED")
			})
		}
	})

	t.Run("q and status pagination uses id as stable fallback", func(t *testing.T) {
		first := performPaginationIntegrationRequest(t, router, "/media?q=campaign&status=ready&page=1&page_size=2&sort=created_at")
		assertHTTPStatus(t, first, http.StatusOK)
		firstPage := decodePaginatedMediaEnvelope(t, first)
		assertPaginationMeta(t, firstPage.Pagination, 1, 2, 3, 2)
		assertMediaIDs(t, firstPage.Data, media[0].ID, media[1].ID)
		if firstPage.Data[0].UsageCount != 2 {
			t.Fatalf("paginated usage_count = %d, want 2", firstPage.Data[0].UsageCount)
		}

		second := performPaginationIntegrationRequest(t, router, "/media?q=campaign&status=ready&page=2&page_size=2&sort=created_at")
		assertHTTPStatus(t, second, http.StatusOK)
		secondPage := decodePaginatedMediaEnvelope(t, second)
		assertPaginationMeta(t, secondPage.Pagination, 2, 2, 3, 2)
		assertMediaIDs(t, secondPage.Data, media[2].ID)
	})
}

func TestListAuditLogsPostgresPaginationAndFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openHTTPAPIPostgresTestSchema(t, "pagination_audit")
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("auto-migrate audit logs: %v", err)
	}

	actorID := uuid.MustParse("30000000-0000-0000-0000-000000000001")
	otherActorID := uuid.MustParse("30000000-0000-0000-0000-000000000002")
	createdAt := time.Date(2026, time.July, 13, 9, 0, 0, 0, time.UTC)
	logs := []model.AuditLog{
		newPaginationTestAudit("40000000-0000-0000-0000-000000000001", actorID, "posts", "success", "/api/v1/admin/posts/needle-one", createdAt),
		newPaginationTestAudit("40000000-0000-0000-0000-000000000002", actorID, "posts", "success", "/api/v1/admin/posts/needle-two", createdAt),
		newPaginationTestAudit("40000000-0000-0000-0000-000000000003", actorID, "posts", "success", "/api/v1/admin/posts/needle-three", createdAt),
		newPaginationTestAudit("40000000-0000-0000-0000-000000000004", otherActorID, "posts", "success", "/api/v1/admin/posts/needle-other-actor", createdAt),
		newPaginationTestAudit("40000000-0000-0000-0000-000000000005", actorID, "posts", "failure", "/api/v1/admin/posts/needle-failure", createdAt),
		newPaginationTestAudit("40000000-0000-0000-0000-000000000006", actorID, "pages", "success", "/api/v1/admin/pages/needle-page", createdAt),
		newPaginationTestAudit("40000000-0000-0000-0000-000000000007", actorID, "posts", "success", "/api/v1/admin/posts/no-match", createdAt),
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("seed audit logs: %v", err)
	}

	router := gin.New()
	router.GET("/audit-logs", listAuditLogs(db))
	baseQuery := url.Values{
		"actor_id":      []string{actorID.String()},
		"limit":         []string{"2"},
		"q":             []string{"needle"},
		"resource_type": []string{"posts"},
		"result":        []string{"success"},
		"sort":          []string{"-created_at"},
	}

	first := performPaginationIntegrationRequest(t, router, "/audit-logs?"+baseQuery.Encode())
	assertHTTPStatus(t, first, http.StatusOK)
	firstPage := decodePaginatedAuditEnvelope(t, first)
	assertPaginationMeta(t, firstPage.Pagination, 1, 2, 3, 2)
	assertAuditIDs(t, firstPage.Data, logs[0].ID, logs[1].ID)

	secondQuery := clonePaginationTestValues(baseQuery)
	secondQuery.Set("page", "2")
	second := performPaginationIntegrationRequest(t, router, "/audit-logs?"+secondQuery.Encode())
	assertHTTPStatus(t, second, http.StatusOK)
	secondPage := decodePaginatedAuditEnvelope(t, second)
	assertPaginationMeta(t, secondPage.Pagination, 2, 2, 3, 2)
	assertAuditIDs(t, secondPage.Data, logs[2].ID)

	outOfRangeQuery := clonePaginationTestValues(baseQuery)
	outOfRangeQuery.Set("page", "3")
	outOfRange := performPaginationIntegrationRequest(t, router, "/audit-logs?"+outOfRangeQuery.Encode())
	assertHTTPStatus(t, outOfRange, http.StatusOK)
	outOfRangePage := decodePaginatedAuditEnvelope(t, outOfRange)
	assertPaginationMeta(t, outOfRangePage.Pagination, 3, 2, 3, 2)
	assertAuditIDs(t, outOfRangePage.Data)
}

type paginationMediaEnvelope struct {
	Data       []model.MediaAsset `json:"data"`
	Pagination paginationMeta     `json:"pagination"`
}

type paginationAuditEnvelope struct {
	Data       []model.AuditLog `json:"data"`
	Pagination paginationMeta   `json:"pagination"`
}

func newPaginationTestMedia(id, filename, originalName, publicURL, checksum, status string, createdAt time.Time) model.MediaAsset {
	return model.MediaAsset{
		Base: model.Base{
			ID:        uuid.MustParse(id),
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		Filename:      filename,
		OriginalName:  originalName,
		MimeType:      "image/jpeg",
		SizeBytes:     1024,
		StorageDriver: "local",
		StorageKey:    filename,
		PublicURL:     publicURL,
		Checksum:      checksum,
		Status:        status,
	}
}

func newPaginationTestUsage(id string, mediaID uuid.UUID, resourceID string, usageType string, deletedAt *time.Time) model.MediaUsage {
	usage := model.MediaUsage{
		Base: model.Base{
			ID: uuid.MustParse(id),
		},
		MediaID:      mediaID,
		ResourceType: "post",
		ResourceID:   uuid.MustParse(resourceID),
		UsageType:    usageType,
	}
	if deletedAt != nil {
		usage.DeletedAt = gorm.DeletedAt{Time: *deletedAt, Valid: true}
	}
	return usage
}

func newPaginationTestAudit(id string, actorID uuid.UUID, resourceType, result, route string, createdAt time.Time) model.AuditLog {
	return model.AuditLog{
		ID:            uuid.MustParse(id),
		ActorID:       &actorID,
		ActorEmail:    "qa@example.test",
		Action:        resourceType + ".update",
		ResourceType:  resourceType,
		Route:         route,
		Method:        http.MethodPatch,
		Result:        result,
		StatusCode:    http.StatusOK,
		RequestID:     "request-" + id,
		IPHashVersion: 1,
		CreatedAt:     createdAt,
	}
}

func performPaginationIntegrationRequest(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

func assertHTTPStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, want, response.Body.String())
	}
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var envelope struct {
		Error ErrorBody `json:"error"`
	}
	decodePaginationTestJSON(t, response, &envelope)
	if envelope.Error.Code != want {
		t.Fatalf("error code = %q, want %q; body: %s", envelope.Error.Code, want, response.Body.String())
	}
}

func decodeLegacyMediaEnvelope(t *testing.T, response *httptest.ResponseRecorder) []model.MediaAsset {
	t.Helper()
	var raw map[string]json.RawMessage
	decodePaginationTestJSON(t, response, &raw)
	if _, exists := raw["pagination"]; exists {
		t.Fatalf("exact media response unexpectedly contains pagination: %s", response.Body.String())
	}
	var media []model.MediaAsset
	if err := json.Unmarshal(raw["data"], &media); err != nil {
		t.Fatalf("decode exact media data: %v; body: %s", err, response.Body.String())
	}
	return media
}

func decodePaginatedMediaEnvelope(t *testing.T, response *httptest.ResponseRecorder) paginationMediaEnvelope {
	t.Helper()
	var envelope paginationMediaEnvelope
	decodePaginationTestJSON(t, response, &envelope)
	return envelope
}

func decodePaginatedAuditEnvelope(t *testing.T, response *httptest.ResponseRecorder) paginationAuditEnvelope {
	t.Helper()
	var envelope paginationAuditEnvelope
	decodePaginationTestJSON(t, response, &envelope)
	return envelope
}

func decodePaginationTestJSON(t *testing.T, response *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, response.Body.String())
	}
}

func assertPaginationMeta(t *testing.T, got paginationMeta, page, pageSize int, total, totalPages int64) {
	t.Helper()
	want := paginationMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
	if got != want {
		t.Fatalf("pagination = %#v, want %#v", got, want)
	}
}

func assertMediaIDs(t *testing.T, media []model.MediaAsset, want ...uuid.UUID) {
	t.Helper()
	got := make([]uuid.UUID, len(media))
	for i := range media {
		got[i] = media[i].ID
	}
	assertPaginationTestIDs(t, got, want)
}

func assertAuditIDs(t *testing.T, logs []model.AuditLog, want ...uuid.UUID) {
	t.Helper()
	got := make([]uuid.UUID, len(logs))
	for i := range logs {
		got[i] = logs[i].ID
	}
	assertPaginationTestIDs(t, got, want)
}

func assertPaginationTestIDs(t *testing.T, got, want []uuid.UUID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("IDs = %v, want %v", got, want)
		}
	}
}

func clonePaginationTestValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}
