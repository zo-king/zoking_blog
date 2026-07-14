package httpapi

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestResolvePreviewFileAccessBoundary(t *testing.T) {
	root := t.TempDir()
	previewDir := filepath.Join(root, "prev-safe")
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatalf("create preview fixture: %v", err)
	}
	indexPath := filepath.Join(previewDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("preview"), 0o644); err != nil {
		t.Fatalf("write index fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(previewDir, "manifest.json"), []byte(`{"output_path":"private"}`), 0o644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}

	for _, key := range []string{"prev-safe", "prev-safe/", "prev-safe/index.html"} {
		resolved, ok := resolvePreviewFile(root, key)
		if !ok || filepath.Clean(resolved) != filepath.Clean(indexPath) {
			t.Fatalf("key %q resolved to %q, ok=%v", key, resolved, ok)
		}
	}
	for _, key := range []string{"", "../outside.txt", "prev-safe/../outside.txt", `prev-safe\index.html`, "prev-safe/manifest.json"} {
		if resolved, ok := resolvePreviewFile(root, key); ok {
			t.Fatalf("unsafe key %q resolved to %q", key, resolved)
		}
	}
}

func TestProductionPreviewRequiresDedicatedHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{AppEnv: "production", PublishPreviewPublicBaseURL: "https://preview.zoking.tech/preview-files"}
	for _, test := range []struct {
		host string
		want bool
	}{
		{"preview.zoking.tech", true},
		{"api.zoking.tech", false},
	} {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest("GET", "https://"+test.host+"/preview-files/example/", nil)
		if got := previewRequestHostAllowed(context, cfg); got != test.want {
			t.Fatalf("host %q allowed=%v, want %v", test.host, got, test.want)
		}
	}
}

func TestResolvePreviewFileRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	previewDir := filepath.Join(root, "prev-safe")
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatalf("create preview fixture: %v", err)
	}
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	linkPath := filepath.Join(previewDir, "escape.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if resolved, ok := resolvePreviewFile(root, "prev-safe/escape.txt"); ok {
		t.Fatalf("symlink escape resolved to %q", resolved)
	}
}
