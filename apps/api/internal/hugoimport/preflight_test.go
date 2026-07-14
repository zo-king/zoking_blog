package hugoimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreflightParsesArticleAndAssets(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "content", "post", "hello", "index.md"), `---
title: 你好
description: 简介
date: 2026-07-10T09:30:00+08:00
slug: hello
image: /img/cover.jpg
categories: [技术]
tags: [Go]
---

正文 ![图片](/img/body.jpg)
`)
	writeTestFile(t, filepath.Join(root, "static", "img", "cover.jpg"), "cover")
	writeTestFile(t, filepath.Join(root, "static", "img", "body.jpg"), "body")
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories:\n  技术: technology\ntags:\n  Go: go\n")

	articles, _, err := Preflight(root, mapPath, "http://localhost:18080/media-files")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].Slug != "hello" || len(articles[0].Assets) != 2 {
		t.Fatalf("unexpected articles: %#v", articles)
	}
	if got := articles[0].PublishedAt.Format(time.RFC3339); got != "2026-07-10T09:30:00+08:00" {
		t.Fatalf("published date changed: %s", got)
	}
}

func TestPreflightRejectsDirectorySlugMismatchBeforeReturningArticles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "content", "post", "中文目录", "index.md"), `---
title: 你好
date: 2026-07-10T09:30:00+08:00
slug: english-slug
categories: []
tags: []
---
body`)
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories: {}\ntags: {}\n")

	articles, _, err := Preflight(root, mapPath, "http://localhost:18080/media-files")
	if err == nil || articles != nil || !strings.Contains(err.Error(), `directory basename "中文目录" must equal slug "english-slug"`) {
		t.Fatalf("expected directory mismatch, got articles=%v err=%v", articles, err)
	}
}

func TestPreflightReportsAllArticleErrors(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"a", "b"} {
		writeTestFile(t, filepath.Join(root, "content", "post", slug, "index.md"), "missing front matter")
	}
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories: {}\ntags: {}\n")
	_, _, err := Preflight(root, mapPath, "http://localhost:18080/media-files")
	if err == nil || strings.Count(err.Error(), "YAML front matter") != 2 {
		t.Fatalf("expected both errors, got %v", err)
	}
}

func TestPreflightAcceptsPublisherManagedMediaAndTaxonomySlugs(t *testing.T) {
	root := t.TempDir()
	managedURL := "http://localhost:18080/media-files/2026/07/managed.jpg"
	writeTestFile(t, filepath.Join(root, "content", "post", "hello", "index.md"), `---
title: 你好
date: 2026-07-10T09:30:00+08:00
slug: hello
image: "`+managedURL+`"
categories: [technology]
tags: [go]
---

正文 ![图片](`+managedURL+`)
`)
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories:\n  技术: technology\ntags:\n  Go: go\n")

	articles, _, err := Preflight(root, mapPath, "http://localhost:18080/media-files")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || len(articles[0].Assets) != 1 {
		t.Fatalf("unexpected articles: %#v", articles)
	}
	article := articles[0]
	if article.Categories[0] != "技术" || article.Tags[0] != "Go" {
		t.Fatalf("taxonomy slugs were not normalized: %#v %#v", article.Categories, article.Tags)
	}
	if article.Assets[0].ManagedURL != managedURL || article.Assets[0].Checksum != "" || article.Assets[0].Path != "" {
		t.Fatalf("managed media was not classified correctly: %#v", article.Assets[0])
	}
}

func TestPreflightCanonicalizesManagedMediaToConfiguredBase(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "content", "post", "canonical-managed", "index.md"), `---
title: "Canonical managed media"
slug: "canonical-managed"
date: "2026-07-10T10:00:00+08:00"
categories: ["技术"]
tags: ["Go"]
---
正文 ![图片](/media-files/2026/07/managed.jpg)
`)
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories:\n  技术: technology\ntags:\n  Go: go\n")

	articles, _, err := Preflight(root, mapPath, "https://cdn.example.test/media-files")
	if err != nil {
		t.Fatal(err)
	}
	if got := articles[0].Assets[0].ManagedURL; got != "https://cdn.example.test/media-files/2026/07/managed.jpg" {
		t.Fatalf("unexpected canonical managed URL: %q", got)
	}
}

func TestPreflightRejectsURLsOutsideManagedMediaBase(t *testing.T) {
	for _, imageURL := range []string{
		"https://evil.example/media-files/tracker.png",
		"http://user@localhost:18080/media-files/private.png",
		"http://localhost:18080/media-files/private.png#fragment",
		"http://localhost:18080/media-files/private.png?token=secret",
	} {
		t.Run(imageURL, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "content", "post", "hello", "index.md"), "---\ntitle: Hello\ndate: 2026-07-10T09:30:00+08:00\nslug: hello\ncategories: []\ntags: []\n---\n![x]("+imageURL+")")
			mapPath := filepath.Join(root, "taxonomy.yaml")
			writeTestFile(t, mapPath, "categories: {}\ntags: {}\n")
			if _, _, err := Preflight(root, mapPath, "http://localhost:18080/media-files"); err == nil {
				t.Fatalf("expected %q to be rejected", imageURL)
			}
		})
	}
}

func TestPreflightRejectsMissingOrdinaryStaticAsset(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "content", "post", "hello", "index.md"), "---\ntitle: Hello\ndate: 2026-07-10T09:30:00+08:00\nslug: hello\ncategories: []\ntags: []\n---\n![x](/img/missing.png)")
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories: {}\ntags: {}\n")
	_, _, err := Preflight(root, mapPath, "http://localhost:18080/media-files")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing static asset error, got %v", err)
	}
}

func TestPreflightCanonicalizesAndDeduplicatesTaxonomyNamesAndSlugs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "content", "post", "hello", "index.md"), "---\ntitle: Hello\ndate: 2026-07-10T09:30:00+08:00\nslug: hello\ncategories: [技术, technology, 技术]\ntags: [Go, go]\n---\nbody")
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories:\n  ' 技术 ': ' technology '\ntags:\n  ' Go ': ' go '\n")
	articles, mapping, err := Preflight(root, mapPath, "http://localhost:18080/media-files")
	if err != nil {
		t.Fatal(err)
	}
	if got := articles[0].Categories; len(got) != 1 || got[0] != "技术" {
		t.Fatalf("categories not canonicalized: %#v", got)
	}
	if got := articles[0].Tags; len(got) != 1 || got[0] != "Go" {
		t.Fatalf("tags not canonicalized: %#v", got)
	}
	if mapping.Categories["技术"] != "technology" || mapping.Tags["Go"] != "go" {
		t.Fatalf("mapping not trimmed: %#v", mapping)
	}
}

func TestPreflightRejectsAmbiguousTaxonomyTokens(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "content", "post", "hello", "index.md"), "---\ntitle: Hello\ndate: 2026-07-10T09:30:00+08:00\nslug: hello\ncategories: []\ntags: []\n---\nbody")
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories:\n  技术: engineering\n  engineering: other\ntags: {}\n")
	if _, _, err := Preflight(root, mapPath, "http://localhost:18080/media-files"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous taxonomy error, got %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
