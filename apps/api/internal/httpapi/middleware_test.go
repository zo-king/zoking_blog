package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestCORSMiddlewareUsesConfiguredAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware(config.Config{
		AllowedOrigins:      "http://localhost:1313",
		AdminAllowedOrigins: "http://localhost:5173",
	}))
	router.Any("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name        string
		method      string
		origin      string
		wantStatus  int
		wantAllowed bool
	}{
		{"reader request", http.MethodGet, "http://localhost:1313", http.StatusOK, true},
		{"reader preflight", http.MethodOptions, "http://localhost:1313", http.StatusNoContent, true},
		{"unknown request", http.MethodGet, "https://untrusted.example", http.StatusOK, false},
		{"unknown preflight", http.MethodOptions, "https://untrusted.example", http.StatusNoContent, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/test", nil)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, response.Code)
			}
			allowedOrigin := response.Header().Get("Access-Control-Allow-Origin")
			if test.wantAllowed && allowedOrigin != test.origin {
				t.Fatalf("expected allowed origin %q, got %q", test.origin, allowedOrigin)
			}
			if !test.wantAllowed && allowedOrigin != "" {
				t.Fatalf("expected no CORS allow header, got %q", allowedOrigin)
			}
			vary := response.Header().Get("Vary")
			if test.origin != "" && (!strings.Contains(vary, "Origin") || !strings.Contains(vary, "Access-Control-Request-Method") || !strings.Contains(vary, "Access-Control-Request-Headers")) {
				t.Fatalf("incomplete CORS Vary header: %q", vary)
			}
		})
	}
}

func TestCORSMiddlewareSeparatesPublicAndAdminCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware(config.Config{
		AllowedOrigins:      "https://zoking.tech",
		AdminAllowedOrigins: "https://admin.zoking.tech",
	}))
	router.Any("/api/v1/public/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.Any("/api/v1/admin/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	tests := []struct {
		name            string
		path            string
		origin          string
		wantOrigin      string
		wantCredentials string
	}{
		{"public origin has no credentials", "/api/v1/public/test", "https://zoking.tech", "https://zoking.tech", ""},
		{"admin origin has credentials", "/api/v1/admin/test", "https://admin.zoking.tech", "https://admin.zoking.tech", "true"},
		{"public site cannot call admin", "/api/v1/admin/test", "https://zoking.tech", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.Header.Set("Origin", tt.origin)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
				t.Fatalf("allow origin = %q, want %q", got, tt.wantOrigin)
			}
			if got := response.Header().Get("Access-Control-Allow-Credentials"); got != tt.wantCredentials {
				t.Fatalf("allow credentials = %q, want %q", got, tt.wantCredentials)
			}
		})
	}
}

func TestCSRFMiddlewareProtectsCookieWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		origin     string
		cookie     string
		header     string
		transport  string
		wantStatus int
	}{
		{"valid cookie write", "https://admin.zoking.tech", "csrf", "csrf", "cookie", http.StatusOK},
		{"missing token", "https://admin.zoking.tech", "csrf", "", "cookie", http.StatusForbidden},
		{"untrusted same-site origin", "https://zoking.tech", "csrf", "csrf", "cookie", http.StatusForbidden},
		{"bearer client bypasses browser CSRF", "", "", "", "bearer", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("auth_transport", tt.transport); c.Next() })
			router.Use(csrfMiddleware(config.Config{AdminAllowedOrigins: "https://admin.zoking.tech"}))
			router.POST("/api/v1/admin/test", func(c *gin.Context) { c.Status(http.StatusOK) })
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/test", nil)
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			if tt.header != "" {
				request.Header.Set("X-CSRF-Token", tt.header)
			}
			if tt.cookie != "" {
				request.AddCookie(&http.Cookie{Name: adminCSRFCookieName, Value: tt.cookie})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tt.wantStatus, response.Body.String())
			}
		})
	}
}

func TestAdminOriginMiddlewareRejectsUntrustedBrowserRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(adminOriginMiddleware(config.Config{AdminAllowedOrigins: "https://admin.zoking.tech"}))
	router.POST("/api/v1/admin/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	for _, tt := range []struct {
		name       string
		origin     string
		wantStatus int
	}{
		{"trusted browser", "https://admin.zoking.tech", http.StatusOK},
		{"untrusted browser", "https://zoking.tech", http.StatusForbidden},
		{"non-browser client", "", http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", nil)
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
		})
	}
}

func TestPublicRoutePathAcceptsRelativeAndAbsolutePublicURLs(t *testing.T) {
	for input, expected := range map[string]string{
		"/media-files":                        "/media-files",
		"/media-files/":                       "/media-files",
		"http://localhost:18080/media-files/": "/media-files",
		"preview-files":                       "/preview-files",
	} {
		if actual := publicRoutePath(input); actual != expected {
			t.Fatalf("publicRoutePath(%q): expected %q, got %q", input, expected, actual)
		}
	}
}
