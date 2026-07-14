package hugoimport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRewriteStaticURLs(t *testing.T) {
	body := "![cover](/img/cover.jpg) and /img/other.jpg"
	got := rewriteStaticURLs(body, map[string]string{
		"/img/cover.jpg": "http://api/media/cover.jpg",
		"/img/other.jpg": "http://api/media/other.jpg",
	})
	if strings.Contains(got, "/img/") || !strings.Contains(got, "http://api/media/cover.jpg") {
		t.Fatalf("URLs were not rewritten: %s", got)
	}
}

func TestImporterSecondRunIsUnchangedAndNonPublishingUpdatePreservesStatus(t *testing.T) {
	root := t.TempDir()
	articlePath := filepath.Join(root, "content", "post", "hello", "index.md")
	writeImportArticle := func(title string) {
		writeTestFile(t, articlePath, "---\ntitle: "+title+"\ndescription: 简介\ndate: 2026-07-10T09:30:00+08:00\nslug: hello\ncategories: [技术]\ntags: [Go]\n---\n正文")
	}
	writeImportArticle("你好")
	mapPath := filepath.Join(root, "taxonomy.yaml")
	writeTestFile(t, mapPath, "categories:\n  技术: technology\ntags:\n  Go: go\n")

	var stored *postRecord
	patches, publishes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/admin/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "zoking_admin_access", Value: "session", Path: "/api/v1/admin"})
			writeEnvelope(t, w, loginResponse{CSRFToken: "csrf-token"})
		case r.URL.Path == "/api/v1/admin/categories":
			writeEnvelope(t, w, []taxonomyRecord{{ID: "category-id", Name: "技术", Slug: "technology"}})
		case r.URL.Path == "/api/v1/admin/tags":
			writeEnvelope(t, w, []taxonomyRecord{{ID: "tag-id", Name: "Go", Slug: "go"}})
		case r.URL.Path == "/api/v1/admin/posts" && r.Method == http.MethodGet:
			if stored == nil {
				writeEnvelope(t, w, []postRecord{})
			} else {
				writeEnvelope(t, w, []postRecord{*stored})
			}
		case r.URL.Path == "/api/v1/admin/posts" && r.Method == http.MethodPost:
			payload := decodePayload(t, r)
			post := postFromPayload(t, payload)
			stored = &post
			writeEnvelope(t, w, post)
		case r.URL.Path == "/api/v1/admin/posts/post-id" && r.Method == http.MethodPatch:
			patches++
			payload := decodePayload(t, r)
			post := postFromPayload(t, payload)
			stored = &post
			writeEnvelope(t, w, post)
		case r.URL.Path == "/api/v1/admin/posts/post-id/publish":
			publishes++
			stored.Status = "published"
			writeEnvelope(t, w, map[string]any{"job": jobRecord{ID: "job-id", Status: "requested"}})
		case r.URL.Path == "/api/v1/admin/publish/jobs/job-id":
			writeEnvelope(t, w, jobRecord{ID: "job-id", Status: "published"})
		default:
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()

	options := Options{SiteDir: root, TaxonomyMap: mapPath, APIBase: server.URL, MediaBase: server.URL + "/media-files", Email: "admin@example.test", Password: "password", Publish: true, PollEvery: time.Millisecond, JobTimeout: time.Second}
	first, err := (Importer{}).Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Created != 1 || first.Published != 1 || stored.Status != "published" {
		t.Fatalf("unexpected first result=%#v post=%#v", first, stored)
	}
	second, err := (Importer{}).Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if second.Unchanged != 1 || second.Updated != 0 || second.Published != 0 || patches != 0 || publishes != 1 {
		t.Fatalf("second run was not unchanged: result=%#v patches=%d publishes=%d", second, patches, publishes)
	}

	writeImportArticle("更新后的标题")
	options.Publish = false
	third, err := (Importer{}).Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if third.Updated != 1 || third.Published != 0 || stored.Status != "published" || patches != 1 || publishes != 1 {
		t.Fatalf("non-publishing update changed status: result=%#v post=%#v patches=%d publishes=%d", third, stored, patches, publishes)
	}
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
		t.Fatal(err)
	}
}

func decodePayload(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func postFromPayload(t *testing.T, payload map[string]any) postRecord {
	t.Helper()
	publishedAt, err := time.Parse(time.RFC3339, payload["published_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	post := postRecord{
		ID: "post-id", Title: payload["title"].(string), Slug: payload["slug"].(string), Summary: payload["summary"].(string),
		ContentMD: payload["content_md"].(string), Status: payload["status"].(string), Visibility: payload["visibility"].(string),
		AllowComment: payload["allow_comment"].(bool), PublishedAt: &publishedAt, SEOTitle: payload["seo_title"].(string), SEODescription: payload["seo_description"].(string),
	}
	for _, id := range payload["category_ids"].([]any) {
		post.Categories = append(post.Categories, taxonomyRecord{ID: id.(string), Name: "技术", Slug: "technology"})
	}
	for _, id := range payload["tag_ids"].([]any) {
		post.Tags = append(post.Tags, taxonomyRecord{ID: id.(string), Name: "Go", Slug: "go"})
	}
	if cover, _ := payload["cover_media_id"].(string); cover != "" {
		post.CoverMediaID = &cover
	}
	return post
}

func TestArticlePayloadKeepsArticleWithoutFrontMatterImageCoverless(t *testing.T) {
	article := Article{
		FrontMatter: FrontMatter{Title: "系统设计", Slug: "system-design", Description: "desc", Categories: []string{"技术"}, Tags: []string{"Go"}},
		PublishedAt: time.Date(2026, 7, 5, 20, 15, 0, 0, time.FixedZone("CST", 8*60*60)),
		Body:        "![diagram](/img/architecture.jpg)",
		Assets:      []Asset{{StaticURL: "/img/architecture.jpg", Checksum: "sum"}},
	}
	payload, err := articlePayload(article, map[string]string{"技术": "category-id"}, map[string]string{"Go": "tag-id"}, map[string]mediaRecord{"sum": {ID: "media-id", PublicURL: "/media/file.jpg"}}, "draft")
	if err != nil {
		t.Fatal(err)
	}
	if payload["cover_media_id"] != "" {
		t.Fatalf("expected no cover, got %#v", payload["cover_media_id"])
	}
	if payload["published_at"] != "2026-07-05T20:15:00+08:00" {
		t.Fatalf("unexpected published_at: %v", payload["published_at"])
	}
	if !strings.Contains(payload["content_md"].(string), "/media/file.jpg") {
		t.Fatalf("body image was not rewritten: %s", payload["content_md"])
	}
}
