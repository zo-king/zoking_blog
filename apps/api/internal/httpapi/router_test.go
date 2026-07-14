package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/auth"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestSplitCSVTrimsAndDropsEmptyEntries(t *testing.T) {
	got := splitCSV(" 127.0.0.1, ,::1,, 10.0.0.0/8 ")
	want := []string{"127.0.0.1", "::1", "10.0.0.0/8"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoginUsesScopedCookieSessionWithoutBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newPreviewMockDB(t)
	passwordHash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	mock.ExpectQuery(`SELECT .*FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "display_name", "status"}).
			AddRow(userID, "owner@zoking.tech", passwordHash, "Owner", "active"))
	mock.ExpectExec(`UPDATE "users"`).WillReturnResult(sqlmock.NewResult(0, 1))

	cfg := config.Config{
		AppEnv:                       "production",
		JWTSecret:                    strings.Repeat("s", 32),
		AccessTokenTTL:               30 * time.Minute,
		AdminLoginRateLimitPerMinute: 10,
		AdminLoginRateLimitBurst:     5,
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", bytes.NewBufferString(`{"email":"owner@zoking.tech","password":"correct-password"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	login(db, cfg, newAdminLoginRateLimiter(cfg))(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	data, _ := payload["data"].(map[string]any)
	if _, leaked := data["access_token"]; leaked {
		t.Fatalf("login response leaked access_token: %s", recorder.Body.String())
	}
	if strings.TrimSpace(data["csrf_token"].(string)) == "" {
		t.Fatal("login response omitted CSRF token")
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("login cookies = %d, want 2", len(cookies))
	}
	for _, cookie := range cookies {
		if cookie.Path != adminCookiePath || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("unsafe cookie attributes: %#v", cookie)
		}
	}
	assertPreviewMockExpectations(t, mock)
}

func TestNewCSRFTokenIsStrongAndUnique(t *testing.T) {
	first, err := newCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 40 || first == second {
		t.Fatalf("unexpected CSRF tokens: %q %q", first, second)
	}
}

func TestResumeAdminSessionRequiresTrustedBrowserOriginAndCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		AppEnv:              "production",
		JWTSecret:           strings.Repeat("r", 32),
		AccessTokenTTL:      30 * time.Minute,
		AdminAllowedOrigins: "https://admin.zoking.tech",
	}
	accessToken, err := auth.GenerateAccessToken(cfg.JWTSecret, uuid.NewString(), "owner@zoking.tech", cfg.AccessTokenTTL)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/api/v1/admin/auth/session", authMiddleware(cfg), resumeAdminSession(cfg))

	tests := []struct {
		name       string
		origin     string
		bearer     bool
		cookie     bool
		wantStatus int
	}{
		{"trusted cookie session", "https://admin.zoking.tech", false, true, http.StatusOK},
		{"missing origin", "", false, true, http.StatusForbidden},
		{"bearer cannot bootstrap browser session", "https://admin.zoking.tech", true, false, http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/session", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: adminAccessCookieName, Value: accessToken})
			}
			if test.bearer {
				request.Header.Set("Authorization", "Bearer "+accessToken)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus == http.StatusOK {
				cookies := response.Result().Cookies()
				if len(cookies) != 1 || cookies[0].Name != adminCSRFCookieName || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].Path != adminCookiePath {
					t.Fatalf("unexpected resumed session cookie: %#v", cookies)
				}
				if !strings.Contains(response.Body.String(), "csrf_token") || strings.Contains(response.Body.String(), "access_token") {
					t.Fatalf("unexpected resume response: %s", response.Body.String())
				}
			}
		})
	}
}

func TestEditablePostStatus(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultDraft bool
		want         string
		wantErr      error
	}{
		{name: "default draft", defaultDraft: true, want: "draft"},
		{name: "draft", value: " draft ", want: "draft"},
		{name: "offline", value: "offline", want: "offline"},
		{name: "archived", value: "archived", want: "archived"},
		{name: "published requires endpoint", value: "published", wantErr: errPublishEndpointRequired},
		{name: "empty update", value: "", wantErr: errInvalidPostStatus},
		{name: "unknown", value: "publsihed", wantErr: errInvalidPostStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := editablePostStatus(tt.value, tt.defaultDraft)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewRouterPanicsForInvalidTrustedProxiesBeforeDBUse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		value := recover()
		if value == nil || !strings.Contains(value.(string), "invalid TRUSTED_PROXIES") {
			t.Fatalf("panic = %v, want invalid TRUSTED_PROXIES", value)
		}
	}()

	NewRouter(nil, config.Config{TrustedProxies: "not-an-ip"})
}
