package publisher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestNewPreviewKeyUsesFullUUIDEntropy(t *testing.T) {
	key := NewPreviewKey(time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC))
	const prefix = "prev-20260713-090000-"
	if !strings.HasPrefix(key, prefix) {
		t.Fatalf("unexpected preview key prefix: %s", key)
	}
	randomPart := strings.TrimPrefix(key, prefix)
	if len(randomPart) != 32 {
		t.Fatalf("preview random part length = %d, want 32", len(randomPart))
	}
	if err := ValidateSlug(key); err != nil {
		t.Fatalf("preview key must remain a valid slug: %v", err)
	}
}

func TestBuildSitePreviewRejectsProtectedRootOverlapBeforeDelete(t *testing.T) {
	parent := t.TempDir()
	siteDir := filepath.Join(parent, "site")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatalf("create site fixture: %v", err)
	}
	sentinel := filepath.Join(siteDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("must survive"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	settings := defaultSiteSettings()
	cfg := config.Config{
		HugoSiteDir:        siteDir,
		PublishPreviewRoot: parent,
		PublishCurrentDir:  filepath.Join(parent, "current"),
		PublishReleaseRoot: filepath.Join(parent, "releases"),
		MediaLocalDir:      filepath.Join(parent, "media"),
	}

	_, err := BuildSitePreview(context.Background(), nil, cfg, PreviewOptions{
		PreviewKey: "site",
		BaseURL:    "http://localhost:18080/preview-files/site/",
		Settings:   &settings,
	})
	if err == nil || !strings.Contains(err.Error(), "overlaps protected path") {
		t.Fatalf("expected protected-root error, got %v", err)
	}
	if content, readErr := os.ReadFile(sentinel); readErr != nil || string(content) != "must survive" {
		t.Fatalf("protected sentinel changed: content=%q error=%v", content, readErr)
	}
}
