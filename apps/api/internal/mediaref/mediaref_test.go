package mediaref

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestResolveReadyMediaRejectsInvalidUUID(t *testing.T) {
	asset, err := ResolveReadyMedia(nil, "not-a-uuid")
	if asset != nil || !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("expected ErrMediaNotFound, got asset=%v err=%v", asset, err)
	}
}

func TestMediaUsageConstants(t *testing.T) {
	if ResourcePost != "post" || UsageCover != "cover" {
		t.Fatalf("unexpected cover usage contract: %s/%s", ResourcePost, UsageCover)
	}
}

func TestSyncPostCoverUsageSupportsSetAndClear(t *testing.T) {
	db, err := gorm.Open(postgres.Open("postgres://test:test@localhost/test"), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	postID := uuid.New()
	mediaID := uuid.New()
	if err := SyncPostCoverUsage(db, postID, &mediaID); err != nil {
		t.Fatalf("set cover usage: %v", err)
	}
	if err := SyncPostCoverUsage(db, postID, nil); err != nil {
		t.Fatalf("clear cover usage: %v", err)
	}
}

func TestReferencedByMarkdownMatchesPublicURLAndStorageKey(t *testing.T) {
	asset := model.MediaAsset{
		Base: model.Base{
			ID: uuid.New(),
		},
		StorageKey: "2026/07/example.png",
		PublicURL:  "/media-files/2026/07/example.png",
	}

	cases := []string{
		"![image](/media-files/2026/07/example.png)",
		"![image](http://localhost:18080/media-files/2026/07/example.png)",
		"![image](2026/07/example.png)",
		`<img src="/media-files/2026/07/example.png?width=800">`,
	}
	for _, content := range cases {
		if !ReferencedByMarkdown(content, asset) {
			t.Fatalf("expected media reference for %q", content)
		}
	}
}

func TestReferencedByMarkdownIgnoresUnrelatedMedia(t *testing.T) {
	asset := model.MediaAsset{
		Base: model.Base{
			ID: uuid.New(),
		},
		StorageKey: "2026/07/example.png",
		PublicURL:  "/media-files/2026/07/example.png",
	}

	if ReferencedByMarkdown("![image](/media-files/2026/07/other.png)", asset) {
		t.Fatalf("unexpected media reference")
	}
}
