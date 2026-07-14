package httpapi

import (
	"testing"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestPermissionForRoute(t *testing.T) {
	tests := []struct {
		method string
		route  string
		want   string
	}{
		{"GET", "/api/v1/admin/posts", "post:read"},
		{"POST", "/api/v1/admin/posts", "post:create"},
		{"POST", "/api/v1/admin/posts/quality-check", "post:create"},
		{"POST", "/api/v1/admin/posts/:id/quality-check", "post:update"},
		{"POST", "/api/v1/admin/posts/:id/preview", "post:update"},
		{"POST", "/api/v1/admin/posts/:id/publish", "post:publish"},
		{"DELETE", "/api/v1/admin/posts/:id", "post:delete"},
		{"DELETE", "/api/v1/admin/pages/:id", "page:delete"},
		{"POST", "/api/v1/admin/pages/quality-check", "page:create"},
		{"POST", "/api/v1/admin/pages/:id/quality-check", "page:update"},
		{"GET", "/api/v1/admin/achievements", "achievement:read"},
		{"POST", "/api/v1/admin/achievements", "achievement:create"},
		{"PATCH", "/api/v1/admin/achievements/:id", "achievement:update"},
		{"PATCH", "/api/v1/admin/achievements/:id/status", "achievement:publish"},
		{"DELETE", "/api/v1/admin/achievements/:id", "achievement:delete"},
		{"POST", "/api/v1/admin/achievements/publish", "achievement:publish"},
		{"POST", "/api/v1/admin/media/cleanup", "media:delete"},
		{"POST", "/api/v1/admin/publish/releases/:id/promote", "publish:rollback"},
		{"POST", "/api/v1/admin/publish/previews/cleanup", "publish:cleanup"},
		{"GET", "/api/v1/admin/audit-logs", "audit:read"},
		{"POST", "/api/v1/admin/qa/e2e-runs/:run_id/cleanup", "qa:cleanup"},
		{"GET", "/api/v1/admin/unregistered-feature", denyPermission},
	}
	for _, test := range tests {
		if got := permissionForRoute(test.method, test.route); got != test.want {
			t.Fatalf("%s %s: expected %s, got %s", test.method, test.route, test.want, got)
		}
	}
}

func TestE2ECleanupEnabled(t *testing.T) {
	for _, test := range []struct {
		env     string
		enabled bool
		want    bool
	}{{"test", true, true}, {"development", true, false}, {"production", true, false}, {"test", false, false}, {"staging", true, false}} {
		if got := e2eCleanupEnabled(config.Config{AppEnv: test.env, QAE2ECleanupEnabled: test.enabled}); got != test.want {
			t.Fatalf("env=%s enabled=%v: expected %v, got %v", test.env, test.enabled, test.want, got)
		}
	}
}
