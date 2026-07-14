package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

func TestPublicMediaDTOExposesOnlyRenderingFields(t *testing.T) {
	media := model.MediaAsset{
		Base:          model.Base{ID: uuid.New()},
		Filename:      "generated-name.jpg",
		OriginalName:  "private-original-name.jpg",
		MimeType:      "image/jpeg",
		SizeBytes:     123456,
		Width:         1280,
		Height:        720,
		StorageDriver: "s3",
		StorageBucket: "private-bucket",
		StorageKey:    "private/storage/key",
		PublicURL:     "/media/public.jpg",
		Checksum:      "private-checksum",
		UploadedBy:    ptrUUID(uuid.New()),
		Status:        "ready",
	}

	payload, err := json.Marshal(newPublicMediaDTO(&media))
	if err != nil {
		t.Fatalf("marshal public media: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("decode public media: %v", err)
	}

	wantFields := []string{"id", "mime_type", "width", "height", "public_url"}
	if len(fields) != len(wantFields) {
		t.Fatalf("public media fields = %v, want exactly %v", fields, wantFields)
	}
	for _, field := range wantFields {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing public media field %q in %s", field, payload)
		}
	}
	for _, field := range []string{"filename", "original_name", "size_bytes", "storage_driver", "storage_bucket", "storage_key", "checksum", "uploaded_by", "status", "created_at", "updated_at"} {
		if _, ok := fields[field]; ok {
			t.Fatalf("internal media field %q leaked in %s", field, payload)
		}
	}
}

func TestPublicPostDTOStabilizesEmbeddedTaxonomy(t *testing.T) {
	lowID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	highID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	post := model.Post{
		Categories: []model.Category{
			{Base: model.Base{ID: highID}, Name: "Same", SortOrder: 1},
			{Base: model.Base{ID: lowID}, Name: "Same", SortOrder: 1},
		},
		Tags: []model.Tag{
			{Base: model.Base{ID: highID}, Name: "Same"},
			{Base: model.Base{ID: lowID}, Name: "Same"},
		},
	}

	dto := newPublicPostDTO(post)
	if dto.Categories[0].ID != lowID || dto.Categories[1].ID != highID {
		t.Fatalf("category IDs = %s, %s; want low then high", dto.Categories[0].ID, dto.Categories[1].ID)
	}
	if dto.Tags[0].ID != lowID || dto.Tags[1].ID != highID {
		t.Fatalf("tag IDs = %s, %s; want low then high", dto.Tags[0].ID, dto.Tags[1].ID)
	}
}

func TestPublicPostSummaryOmitsContent(t *testing.T) {
	payload, err := json.Marshal(newPublicPostSummaryDTO(model.Post{Title: "Summary", ContentMD: "private list payload"}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "content_md") || strings.Contains(string(payload), "private list payload") {
		t.Fatalf("summary DTO leaked full content: %s", payload)
	}
	if detail := newPublicPostDTO(model.Post{ContentMD: "detail content"}); detail.ContentMD != "detail content" {
		t.Fatalf("detail DTO lost content: %#v", detail)
	}
}

func TestPublicPaginationDefaultsAndDiscoveryHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/pages?kind=menu", nil)

	query, ok := parsePublicPagination(context, publicListLimit)
	if !ok {
		t.Fatal("default public pagination was rejected")
	}
	if query.Page != 1 || query.PageSize != 100 || query.Offset != 0 {
		t.Fatalf("default pagination = %#v, want page 1, size 100, offset 0", query)
	}

	setPublicPaginationHeaders(context, 101, query)
	for name, want := range map[string]string{
		"X-Total-Count":                 "101",
		"X-Page":                        "1",
		"X-Page-Size":                   "100",
		"X-Total-Pages":                 "2",
		"X-Has-More":                    "true",
		"Access-Control-Expose-Headers": "Link, X-Total-Count, X-Page, X-Page-Size, X-Total-Pages, X-Has-More",
	} {
		if got := recorder.Header().Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	if got := recorder.Header().Get("Link"); got != `</api/v1/public/pages?kind=menu&page=2&page_size=100>; rel="next"` {
		t.Fatalf("Link = %q, want next-page discovery link", got)
	}
}

func TestPublicPaginationPreservesArrayContractAcrossPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/tags?page=2&page_size=25", nil)

	query, ok := parsePublicPagination(context, publicListLimit)
	if !ok {
		t.Fatal("valid public pagination was rejected")
	}
	if query.Page != 2 || query.PageSize != 25 || query.Offset != 25 {
		t.Fatalf("pagination = %#v, want page 2, size 25, offset 25", query)
	}
	setPublicPaginationHeaders(context, 61, query)
	link := recorder.Header().Get("Link")
	for _, want := range []string{
		`</api/v1/public/tags?page=1&page_size=25>; rel="prev"`,
		`</api/v1/public/tags?page=3&page_size=25>; rel="next"`,
	} {
		if !strings.Contains(link, want) {
			t.Fatalf("Link = %q, missing %q", link, want)
		}
	}

	OK(context, []string{"tag"})
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode compatible public response: %v", err)
	}
	if _, ok := envelope["pagination"]; ok {
		t.Fatalf("public response body unexpectedly changed shape: %s", recorder.Body.String())
	}
	if _, ok := envelope["data"]; !ok || len(envelope) != 2 {
		t.Fatalf("public response body = %s, want legacy data/request_id envelope", recorder.Body.String())
	}
}

func TestPublicPaginationRejectsInvalidBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, target := range []string{
		"/public/pages?page=0",
		"/public/pages?page=1000001",
		"/public/pages?page_size=0",
		"/public/pages?page_size=101",
		"/public/pages?page_size=invalid",
	} {
		t.Run(target, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, target, nil)
			if _, ok := parsePublicPagination(context, publicListLimit); ok {
				t.Fatalf("invalid pagination %q was accepted", target)
			}
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
			}
		})
	}
}
