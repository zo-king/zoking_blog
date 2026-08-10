package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestPatchAdminSiteSettingsEnforcesProductionPublicURLsBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		AppEnv:             "production",
		SiteBaseURL:        "https://zoking.tech/",
		PublicAPIBaseURL:   "https://api.zoking.tech",
		MediaPublicBaseURL: "/media-files",
	}
	tests := []struct {
		name      string
		payload   string
		wantField string
	}{
		{name: "reject HTTP site", payload: `{"site":{"base_url":"http://zoking.tech/"}}`, wantField: "site.base_url"},
		{name: "reject localhost site", payload: `{"site":{"base_url":"https://localhost/"}}`, wantField: "site.base_url"},
		{name: "reject site mismatch", payload: `{"site":{"base_url":"https://other.example.org/"}}`, wantField: "site.base_url"},
		{name: "reject HTTP comments API", payload: `{"comments":{"api_base":"http://api.zoking.tech"}}`, wantField: "comments.api_base"},
		{name: "reject comments API mismatch", payload: `{"comments":{"api_base":"https://api.example.org"}}`, wantField: "comments.api_base"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock := newSettingsMockDB(t)
			expectSiteSettingsLoad(mock, validProductionSettingsRows())

			response := serveSettingsPatch(db, cfg, test.payload)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
			}
			body := response.Body.String()
			if !strings.Contains(body, `"code":"PUBLIC_URL_INVALID"`) || !strings.Contains(body, `"field":"`+test.wantField+`"`) {
				t.Fatalf("unexpected error body: %s", body)
			}
			assertPreviewMockExpectations(t, mock)
		})
	}
}

func TestPatchAdminSiteSettingsAllowsValidProductionURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newSettingsMockDB(t)
	expectSiteSettingsLoad(mock, validProductionSettingsRows())
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "site_settings".*RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()
	expectSiteSettingsLoad(mock, validProductionSettingsRows())

	cfg := config.Config{
		AppEnv:             "production",
		SiteBaseURL:        "https://zoking.tech/",
		PublicAPIBaseURL:   "https://api.zoking.tech",
		MediaPublicBaseURL: "/media-files",
	}
	response := serveSettingsPatch(db, cfg, `{"site":{"base_url":"https://zoking.tech"}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	assertPreviewMockExpectations(t, mock)
}

func serveSettingsPatch(db *gorm.DB, cfg config.Config, payload string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/settings", bytes.NewBufferString(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	patchAdminSiteSettings(db, cfg)(context)
	return response
}

func expectSiteSettingsLoad(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT .* FROM "site_settings" WHERE is_public = \$1`).
		WithArgs(true).
		WillReturnRows(rows)
}

func validProductionSettingsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"key", "value_json", "is_public"}).
		AddRow("site.base_url", []byte(`"https://zoking.tech/"`), true).
		AddRow("comments.enabled", []byte(`true`), true).
		AddRow("comments.api_base", []byte(`"https://api.zoking.tech"`), true)
}

func newSettingsMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open mock GORM database: %v", err)
	}
	return db, mock
}
