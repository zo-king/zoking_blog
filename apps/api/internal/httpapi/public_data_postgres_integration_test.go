package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

func TestPublicDataPaginationAndStableOrderingPostgres(t *testing.T) {
	db := openSeriesPostgresTestSchema(t)
	applySeriesIntegrationMigrations(t, db)

	author := model.User{
		Email:        "public-data-test@example.com",
		Username:     "public-data-test",
		PasswordHash: "not-used",
		DisplayName:  "Public Data Test",
		Status:       "active",
	}
	createSeriesFixture(t, db, &author)

	t.Run("series detail keeps the total above the default page size", func(t *testing.T) {
		series := model.Series{Name: "Long Series", Slug: "long-series", Enabled: true}
		createSeriesFixture(t, db, &series)
		publishedAt := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
		for order := 1; order <= 101; order++ {
			postOrder := order
			post := model.Post{
				Title:        fmt.Sprintf("Series Post %03d", order),
				Slug:         fmt.Sprintf("series-post-%03d", order),
				Status:       "published",
				Visibility:   "public",
				AllowComment: true,
				PublishedAt:  &publishedAt,
				AuthorID:     &author.ID,
				SeriesID:     &series.ID,
				SeriesOrder:  &postOrder,
			}
			createSeriesFixture(t, db, &post)
		}

		first := serveSeriesHandler(t, author.ID, http.MethodGet, "/series/:slug", "/series/long-series", "", getPublicSeries(db))
		if first.Code != http.StatusOK {
			t.Fatalf("first series page status = %d; body=%s", first.Code, first.Body.String())
		}
		var firstEnvelope struct {
			Data publicSeriesDTO `json:"data"`
		}
		decodeSeriesResponse(t, first, &firstEnvelope)
		if firstEnvelope.Data.PostCount != 101 || len(firstEnvelope.Data.Posts) != 100 {
			t.Fatalf("first series page count/posts = %d/%d, want 101/100", firstEnvelope.Data.PostCount, len(firstEnvelope.Data.Posts))
		}
		if first.Header().Get("X-Has-More") != "true" || first.Header().Get("X-Total-Count") != "101" {
			t.Fatalf("first series page headers = %#v", first.Header())
		}

		second := serveSeriesHandler(t, author.ID, http.MethodGet, "/series/:slug", "/series/long-series?page=2", "", getPublicSeries(db))
		if second.Code != http.StatusOK {
			t.Fatalf("second series page status = %d; body=%s", second.Code, second.Body.String())
		}
		var secondEnvelope struct {
			Data publicSeriesDTO `json:"data"`
		}
		decodeSeriesResponse(t, second, &secondEnvelope)
		if secondEnvelope.Data.PostCount != 101 || len(secondEnvelope.Data.Posts) != 1 {
			t.Fatalf("second series page count/posts = %d/%d, want 101/1", secondEnvelope.Data.PostCount, len(secondEnvelope.Data.Posts))
		}
		if secondEnvelope.Data.Posts[0].Title != "Series Post 101" {
			t.Fatalf("second series page post = %q, want Series Post 101", secondEnvelope.Data.Posts[0].Title)
		}
	})

	t.Run("page category tag and comment ties use id order", func(t *testing.T) {
		lowID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
		highID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
		publishedAt := time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC)

		for _, page := range []model.Page{
			{Base: model.Base{ID: highID}, Title: "Tied Page", Slug: "tied-page-high", Status: "published", Visibility: "public", ShowInMenu: true, MenuWeight: 1, PublishedAt: &publishedAt, AuthorID: &author.ID},
			{Base: model.Base{ID: lowID}, Title: "Tied Page", Slug: "tied-page-low", Status: "published", Visibility: "public", ShowInMenu: true, MenuWeight: 1, PublishedAt: &publishedAt, AuthorID: &author.ID},
		} {
			createSeriesFixture(t, db, &page)
		}
		assertPublicPageIDs(t, author.ID, "/pages?page_size=1", listPublicPages(db), lowID, "2")
		assertPublicPageIDs(t, author.ID, "/pages?page=2&page_size=1", listPublicPages(db), highID, "2")

		for _, category := range []model.Category{
			{Base: model.Base{ID: highID}, Name: "Tied Category", Slug: "tied-category-high", SortOrder: 1, Enabled: true},
			{Base: model.Base{ID: lowID}, Name: "Tied Category", Slug: "tied-category-low", SortOrder: 1, Enabled: true},
		} {
			createSeriesFixture(t, db, &category)
		}
		assertPublicTaxonomyID(t, author.ID, "/categories?page_size=1", listCategories(db), lowID, "2")
		assertPublicTaxonomyID(t, author.ID, "/categories?page=2&page_size=1", listCategories(db), highID, "2")

		for _, tag := range []model.Tag{
			{Base: model.Base{ID: highID}, Name: "Tied Tag", Slug: "tied-tag-high"},
			{Base: model.Base{ID: lowID}, Name: "Tied Tag", Slug: "tied-tag-low"},
		} {
			createSeriesFixture(t, db, &tag)
		}
		assertPublicTaxonomyID(t, author.ID, "/tags?page_size=1", listTags(db), lowID, "2")
		assertPublicTaxonomyID(t, author.ID, "/tags?page=2&page_size=1", listTags(db), highID, "2")

		post := model.Post{Title: "Comment Post", Slug: "comment-post", Status: "published", Visibility: "public", AllowComment: true, PublishedAt: &publishedAt, AuthorID: &author.ID}
		createSeriesFixture(t, db, &post)
		createdAt := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
		for _, comment := range []model.Comment{
			{Base: model.Base{ID: highID, CreatedAt: createdAt}, PostID: post.ID, AuthorName: "High", Content: "High", Status: "approved"},
			{Base: model.Base{ID: lowID, CreatedAt: createdAt}, PostID: post.ID, AuthorName: "Low", Content: "Low", Status: "approved"},
		} {
			createSeriesFixture(t, db, &comment)
		}
		assertPublicCommentID(t, author.ID, "/posts/:slug/comments", "/posts/comment-post/comments?page_size=1", listPublicComments(db), lowID, "2")
		assertPublicCommentID(t, author.ID, "/posts/:slug/comments", "/posts/comment-post/comments?page=2&page_size=1", listPublicComments(db), highID, "2")
	})
}

func assertPublicPageIDs(t *testing.T, userID uuid.UUID, target string, handler gin.HandlerFunc, wantID uuid.UUID, wantTotal string) {
	t.Helper()
	response := serveSeriesHandler(t, userID, http.MethodGet, "/pages", target, "", handler)
	if response.Code != http.StatusOK {
		t.Fatalf("pages status = %d; body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data []publicPageDTO `json:"data"`
	}
	decodeSeriesResponse(t, response, &envelope)
	if len(envelope.Data) != 1 || envelope.Data[0].ID != wantID {
		t.Fatalf("page IDs = %#v, want %s", envelope.Data, wantID)
	}
	if response.Header().Get("X-Total-Count") != wantTotal {
		t.Fatalf("page total = %q, want %s", response.Header().Get("X-Total-Count"), wantTotal)
	}
}

func assertPublicTaxonomyID(t *testing.T, userID uuid.UUID, target string, handler gin.HandlerFunc, wantID uuid.UUID, wantTotal string) {
	t.Helper()
	route := strings.SplitN(target, "?", 2)[0]
	response := serveSeriesHandler(t, userID, http.MethodGet, route, target, "", handler)
	if response.Code != http.StatusOK {
		t.Fatalf("taxonomy status = %d; body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data []publicTaxonomyDTO `json:"data"`
	}
	decodeSeriesResponse(t, response, &envelope)
	if len(envelope.Data) != 1 || envelope.Data[0].ID != wantID {
		t.Fatalf("taxonomy IDs = %#v, want %s", envelope.Data, wantID)
	}
	if response.Header().Get("X-Total-Count") != wantTotal {
		t.Fatalf("taxonomy total = %q, want %s", response.Header().Get("X-Total-Count"), wantTotal)
	}
}

func assertPublicCommentID(t *testing.T, userID uuid.UUID, route, target string, handler gin.HandlerFunc, wantID uuid.UUID, wantTotal string) {
	t.Helper()
	response := serveSeriesHandler(t, userID, http.MethodGet, route, target, "", handler)
	if response.Code != http.StatusOK {
		t.Fatalf("comments status = %d; body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data []publicCommentDTO `json:"data"`
	}
	decodeSeriesResponse(t, response, &envelope)
	if len(envelope.Data) != 1 || envelope.Data[0].ID != wantID {
		t.Fatalf("comment IDs = %#v, want %s", envelope.Data, wantID)
	}
	if response.Header().Get("X-Total-Count") != wantTotal {
		t.Fatalf("comment total = %q, want %s", response.Header().Get("X-Total-Count"), wantTotal)
	}
}
