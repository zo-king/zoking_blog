package publisher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSitemapContainsSlugScansNestedLanguageSitemaps(t *testing.T) {
	outputPath := t.TempDir()
	writeTestFile(t, filepath.Join(outputPath, "sitemap.xml"), `<?xml version="1.0" encoding="utf-8"?><sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>http://example.test/en/sitemap.xml</loc></sitemap></sitemapindex>`)
	writeTestFile(t, filepath.Join(outputPath, "en", "sitemap.xml"), `<?xml version="1.0" encoding="utf-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>http://example.test/p/e2e-smoke/</loc></url></urlset>`)

	ok, err := sitemapContainsSlug(outputPath, "e2e-smoke")
	if err != nil {
		t.Fatalf("sitemapContainsSlug returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected nested sitemap slug match")
	}
}

func TestSitemapContainsSlugRequiresLocPath(t *testing.T) {
	outputPath := t.TempDir()
	writeTestFile(t, filepath.Join(outputPath, "sitemap.xml"), `<?xml version="1.0" encoding="utf-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><!-- /p/e2e-smoke/ --><url><loc>http://example.test/p/other/</loc></url></urlset>`)

	ok, err := sitemapContainsSlug(outputPath, "e2e-smoke")
	if err != nil {
		t.Fatalf("sitemapContainsSlug returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected sitemap slug miss when only a comment contains the path")
	}
}

func TestApplySiteSettingsUpdatesDefaultLanguageOverrides(t *testing.T) {
	siteDir := t.TempDir()
	configDir := filepath.Join(siteDir, "config", "_default")
	writeTestFile(t, filepath.Join(configDir, "hugo.toml"), `baseURL = "http://localhost:1313/"
title = "Old Global"
defaultContentLanguage = "en"

[pagination]
    pagerSize = 3
`)
	writeTestFile(t, filepath.Join(configDir, "params.toml"), `[sidebar]
    emoji = "O"
    subtitle = "Old Global Subtitle"

[footer]
    since = 2020

[comments]
    enabled = false

    [comments.public]
        apiBase = "http://localhost:18080"
`)
	writeTestFile(t, filepath.Join(configDir, "languages.toml"), `[en]
    label = "English"
    title = "Old English"

    [en.params.sidebar]
        subtitle = "Old English Subtitle"

[zh]
    label = "简体中文"
    title = "Old Chinese"

    [zh.params.sidebar]
        subtitle = "Old Chinese Subtitle"
`)

	settings := defaultSiteSettings()
	settings.Site.Title = "New Site"
	settings.Sidebar.Subtitle = "New Subtitle"
	settings.Sidebar.Emoji = "N"
	settings.Comments.Enabled = true
	settings.Comments.APIBase = "http://localhost:18080"
	settings.Footer.Since = 2026
	settings.Pagination.PagerSize = 5
	if err := ApplySiteSettings(siteDir, settings); err != nil {
		t.Fatalf("ApplySiteSettings returned error: %v", err)
	}

	hugo := readTestFile(t, filepath.Join(configDir, "hugo.toml"))
	params := readTestFile(t, filepath.Join(configDir, "params.toml"))
	languages := readTestFile(t, filepath.Join(configDir, "languages.toml"))
	if !strings.Contains(hugo, `title = "New Site"`) {
		t.Fatalf("global title not updated:\n%s", hugo)
	}
	if !strings.Contains(params, `subtitle = "New Subtitle"`) {
		t.Fatalf("global sidebar subtitle not updated:\n%s", params)
	}
	if !strings.Contains(languages, `title = "New Site"`) {
		t.Fatalf("default language title not updated:\n%s", languages)
	}
	if !strings.Contains(languages, `subtitle = "New Subtitle"`) {
		t.Fatalf("default language sidebar subtitle not updated:\n%s", languages)
	}
	if strings.Contains(languages, `[zh]`+"\n"+`    label = "简体中文"`+"\n"+`    title = "New Site"`) {
		t.Fatalf("non-default language title should not be overwritten:\n%s", languages)
	}
}

func TestApplySiteSettingsRejectsMissingCommentsAPI(t *testing.T) {
	settings := defaultSiteSettings()
	settings.Comments.Enabled = true
	settings.Comments.APIBase = "  "

	err := ApplySiteSettings(t.TempDir(), settings)
	if err == nil || !strings.Contains(err.Error(), "comments API base is required") {
		t.Fatalf("expected missing comments API error, got %v", err)
	}
}

func TestBuildPostMarkdownIncludesCoverImage(t *testing.T) {
	post := model.Post{
		Title:     "Cover Post",
		Slug:      "cover-post",
		ContentMD: "Body",
		CoverMedia: &model.MediaAsset{
			PublicURL: " /media-files/2026/07/cover.png ",
		},
	}

	markdown := buildPostMarkdown(post)
	if !strings.Contains(markdown, `image: "/media-files/2026/07/cover.png"`) {
		t.Fatalf("cover image front matter missing:\n%s", markdown)
	}
}

func TestBuildPostMarkdownOmitsEmptyCoverImage(t *testing.T) {
	cases := []model.Post{
		{
			Title:     "No Cover",
			Slug:      "no-cover",
			ContentMD: "Body",
		},
		{
			Title:     "Blank Cover",
			Slug:      "blank-cover",
			ContentMD: "Body",
			CoverMedia: &model.MediaAsset{
				PublicURL: "   ",
			},
		},
	}

	for _, post := range cases {
		markdown := buildPostMarkdown(post)
		if strings.Contains(markdown, "\nimage:") {
			t.Fatalf("unexpected cover image front matter for %s:\n%s", post.Slug, markdown)
		}
	}
}

func TestBuildPostMarkdownUsesTaxonomySlugs(t *testing.T) {
	post := model.Post{
		Title:     "Taxonomy Post",
		Slug:      "taxonomy-post",
		ContentMD: "Body",
		Categories: []model.Category{
			{Name: "生活", Slug: "life"},
		},
		Tags: []model.Tag{
			{Name: "工程实践", Slug: "engineering-practice"},
		},
	}

	markdown := buildPostMarkdown(post)
	if !strings.Contains(markdown, "categories:\n  - \"life\"") {
		t.Fatalf("category slug missing from front matter:\n%s", markdown)
	}
	if !strings.Contains(markdown, "tags:\n  - \"engineering-practice\"") {
		t.Fatalf("tag slug missing from front matter:\n%s", markdown)
	}
	if strings.Contains(markdown, "  - \"生活\"") || strings.Contains(markdown, "  - \"工程实践\"") {
		t.Fatalf("taxonomy display names must not become URL keys:\n%s", markdown)
	}
}

func TestBuildPostMarkdownIncludesStructuredSeries(t *testing.T) {
	seriesID := uuid.New()
	seriesOrder := 3
	post := model.Post{
		Title:       "Series Post",
		Slug:        "series-post",
		ContentMD:   "Body",
		SeriesID:    &seriesID,
		SeriesOrder: &seriesOrder,
		Series:      &model.Series{Base: model.Base{ID: seriesID}, Name: "Go 工程实践", Slug: "go-engineering"},
	}

	markdown := buildPostMarkdown(post)
	for _, expected := range []string{"series:\n", `  slug: "go-engineering"`, `  name: "Go 工程实践"`, "  order: 3"} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("series front matter %q missing:\n%s", expected, markdown)
		}
	}
}

func TestWritePostRejectsIncompleteSeriesMetadata(t *testing.T) {
	seriesID := uuid.New()
	post := model.Post{Title: "Broken Series", Slug: "broken-series", ContentMD: "Body", SeriesID: &seriesID}
	if _, err := WritePost(t.TempDir(), post); err == nil || !strings.Contains(err.Error(), "series metadata is incomplete") {
		t.Fatalf("WritePost error = %v, want incomplete series metadata", err)
	}
}

func TestWritePostCreatesLocalizedTaxonomySnapshots(t *testing.T) {
	siteDir := t.TempDir()
	post := model.Post{
		Title:     "Taxonomy Post",
		Slug:      "taxonomy-post",
		ContentMD: "Body",
		Categories: []model.Category{
			{Name: "生活", Slug: "life", Description: "生活记录"},
		},
		Tags: []model.Tag{
			{Name: "工程实践", Slug: "engineering-practice", Description: "工程方法"},
		},
	}

	if _, err := WritePost(siteDir, post); err != nil {
		t.Fatalf("WritePost returned error: %v", err)
	}
	category := readTestFile(t, filepath.Join(siteDir, "content", "categories", "life", "_index.md"))
	if !strings.Contains(category, `title: "生活"`) || !strings.Contains(category, `slug: "life"`) {
		t.Fatalf("localized category snapshot is invalid:\n%s", category)
	}
	tag := readTestFile(t, filepath.Join(siteDir, "content", "tags", "engineering-practice", "_index.md"))
	if !strings.Contains(tag, `title: "工程实践"`) || !strings.Contains(tag, `slug: "engineering-practice"`) {
		t.Fatalf("localized tag snapshot is invalid:\n%s", tag)
	}
}

func TestSyncReleaseCoverUsageSupportsSetAndClear(t *testing.T) {
	db, err := gorm.Open(postgres.Open("postgres://test:test@localhost/test"), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	releaseID := uuid.New()
	mediaID := uuid.New()
	if err := mediaref.SyncReleaseCoverUsage(db, releaseID, &mediaID); err != nil {
		t.Fatalf("set release cover usage: %v", err)
	}
	if err := mediaref.SyncReleaseCoverUsage(db, releaseID, nil); err != nil {
		t.Fatalf("clear release cover usage: %v", err)
	}
}

func TestRemoveContentSnapshots(t *testing.T) {
	siteDir := t.TempDir()
	postDir := filepath.Join(siteDir, "content", "post", "example-post")
	pageDir := filepath.Join(siteDir, "content", "page", "example-page")
	for _, dir := range []string{postDir, pageDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := RemovePost(siteDir, "example-post")
	if err != nil || !removed {
		t.Fatalf("remove post: removed=%v err=%v", removed, err)
	}
	removed, err = RemovePage(siteDir, "example-page")
	if err != nil || !removed {
		t.Fatalf("remove page: removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(postDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("post snapshot still exists: %v", err)
	}
	if _, err := os.Stat(pageDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("page snapshot still exists: %v", err)
	}
	removed, err = RemovePost(siteDir, "example-post")
	if err != nil || removed {
		t.Fatalf("idempotent remove: removed=%v err=%v", removed, err)
	}
	if _, err := RemovePost(siteDir, "../outside"); err == nil {
		t.Fatal("expected invalid slug to be rejected")
	}
}

func TestVerifyReleasePathAbsent(t *testing.T) {
	outputPath := t.TempDir()
	writeTestFile(t, filepath.Join(outputPath, "sitemap.xml"), `<?xml version="1.0" encoding="utf-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>http://example.test/p/other/</loc></url></urlset>`)
	if err := verifyReleasePathAbsent(outputPath, "/p/withdrawn/"); err != nil {
		t.Fatalf("expected withdrawn path to be absent: %v", err)
	}

	withdrawnPath := filepath.Join(outputPath, "p", "withdrawn", "index.html")
	writeTestFile(t, withdrawnPath, "stale")
	if err := verifyReleasePathAbsent(outputPath, "/p/withdrawn/"); err == nil {
		t.Fatal("expected stale release file to fail withdrawal verification")
	}
	if err := os.Remove(withdrawnPath); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(outputPath, "sitemap.xml"), `<?xml version="1.0" encoding="utf-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>http://example.test/p/withdrawn/</loc></url></urlset>`)
	if err := verifyReleasePathAbsent(outputPath, "/p/withdrawn/"); err == nil {
		t.Fatal("expected stale sitemap entry to fail withdrawal verification")
	}
}

func TestCleanupOrphanReleaseOutputRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(t.TempDir(), "current")
	cfg := config.Config{PublishReleaseRoot: root, PublishCurrentDir: current}

	cases := map[string]string{
		"release root":         root,
		"outside root":         filepath.Join(t.TempDir(), "rel_outside"),
		"nested path":          filepath.Join(root, "nested", "rel_nested"),
		"baseline":             filepath.Join(root, "baseline"),
		"current direct child": filepath.Join(root, "current"),
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			writeTestFile(t, filepath.Join(path, "sentinel.txt"), "keep")
			removed, err := cleanupOrphanReleaseOutput(cfg, path, nil)
			if err == nil {
				t.Fatalf("expected unsafe path to be rejected, removed=%v", removed)
			}
			if _, statErr := os.Stat(filepath.Join(path, "sentinel.txt")); statErr != nil {
				t.Fatalf("unsafe path was touched: %v", statErr)
			}
		})
	}

	overlappingCurrent := filepath.Join(root, "rel_current_overlap")
	cfg.PublishCurrentDir = filepath.Join(overlappingCurrent, "site")
	writeTestFile(t, filepath.Join(overlappingCurrent, "sentinel.txt"), "keep")
	if removed, err := cleanupOrphanReleaseOutput(cfg, overlappingCurrent, nil); err == nil {
		t.Fatalf("expected current overlap to be rejected, removed=%v", removed)
	}
	if _, err := os.Stat(filepath.Join(overlappingCurrent, "sentinel.txt")); err != nil {
		t.Fatalf("current overlap was touched: %v", err)
	}
}

func TestCleanupOrphanReleaseOutputRemovesSafeUnregisteredDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rel_orphan")
	writeTestFile(t, filepath.Join(path, "index.html"), "orphan")
	cfg := config.Config{PublishReleaseRoot: root, PublishCurrentDir: filepath.Join(t.TempDir(), "current")}

	removed, err := cleanupOrphanReleaseOutput(cfg, path, nil)
	if err != nil || !removed {
		t.Fatalf("clean safe orphan: removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan release output still exists: %v", err)
	}
}

func TestCleanupOrphanReleaseOutputPreservesRegisteredRelease(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rel_success")
	writeTestFile(t, filepath.Join(path, "index.html"), "published")
	cfg := config.Config{PublishReleaseRoot: root, PublishCurrentDir: filepath.Join(t.TempDir(), "current")}
	releases := []model.PublishRelease{{
		ReleaseKey: "rel_success",
		OutputPath: filepath.Join(root, "legacy-location"),
	}}

	removed, err := cleanupOrphanReleaseOutput(cfg, path, releases)
	if err != nil {
		t.Fatalf("check registered release: %v", err)
	}
	if removed {
		t.Fatal("registered release must not be removed")
	}
	if got := readTestFile(t, filepath.Join(path, "index.html")); got != "published" {
		t.Fatalf("registered release changed: %q", got)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(raw)
}
