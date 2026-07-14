package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

type adminPostsIntegrationResponse struct {
	Data       []model.Post   `json:"data"`
	Pagination paginationMeta `json:"pagination"`
}

func TestListAdminPostsPostgresIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("combines filters and preloads complete taxonomy without inflating total", func(t *testing.T) {
		db := openHTTPAPIPostgresTestSchema(t, "admin_posts_filters")
		migrateAdminPostsIntegrationSchema(t, db)

		categories := []model.Category{
			{
				Base:      model.Base{ID: adminPostsIntegrationUUID(t, "10000000-0000-0000-0000-000000000001")},
				Name:      "Target Category",
				Slug:      "target-category",
				SortOrder: 20,
				Enabled:   true,
			},
			{
				Base:      model.Base{ID: adminPostsIntegrationUUID(t, "10000000-0000-0000-0000-000000000002")},
				Name:      "Alpha Category",
				Slug:      "alpha-category",
				SortOrder: 10,
				Enabled:   true,
			},
			{
				Base:      model.Base{ID: adminPostsIntegrationUUID(t, "10000000-0000-0000-0000-000000000003")},
				Name:      "Beta Category",
				Slug:      "beta-category",
				SortOrder: 20,
				Enabled:   true,
			},
			{
				Base:      model.Base{ID: adminPostsIntegrationUUID(t, "10000000-0000-0000-0000-000000000004")},
				Name:      "Other Category",
				Slug:      "other-category",
				SortOrder: 30,
				Enabled:   true,
			},
		}
		tags := []model.Tag{
			{
				Base: model.Base{ID: adminPostsIntegrationUUID(t, "20000000-0000-0000-0000-000000000001")},
				Name: "Target Tag",
				Slug: "target-tag",
			},
			{
				Base: model.Base{ID: adminPostsIntegrationUUID(t, "20000000-0000-0000-0000-000000000002")},
				Name: "Alpha Tag",
				Slug: "alpha-tag",
			},
			{
				Base: model.Base{ID: adminPostsIntegrationUUID(t, "20000000-0000-0000-0000-000000000003")},
				Name: "Zulu Tag",
				Slug: "zulu-tag",
			},
			{
				Base: model.Base{ID: adminPostsIntegrationUUID(t, "20000000-0000-0000-0000-000000000004")},
				Name: "Other Tag",
				Slug: "other-tag",
			},
		}
		adminPostsIntegrationCreate(t, db, &categories)
		adminPostsIntegrationCreate(t, db, &tags)

		createdAt := time.Date(2026, time.July, 13, 9, 0, 0, 0, time.UTC)
		matchingTaxonomy := adminPostsIntegrationTaxonomy{
			categories: categories[:3],
			tags:       tags[:3],
		}
		posts := []struct {
			post     model.Post
			taxonomy adminPostsIntegrationTaxonomy
		}{
			{
				post:     adminPostsIntegrationPost(t, "30000000-0000-0000-0000-000000000001", "Alpha matched", "needle-q-alpha", "summary", "body needle-keyword", "published", createdAt),
				taxonomy: matchingTaxonomy,
			},
			{
				post:     adminPostsIntegrationPost(t, "30000000-0000-0000-0000-000000000002", "Beta matched", "needle-q-beta", "summary", "needle-keyword body", "published", createdAt),
				taxonomy: matchingTaxonomy,
			},
			{
				post:     adminPostsIntegrationPost(t, "30000000-0000-0000-0000-000000000003", "Wrong category", "needle-q-wrong-category", "summary", "needle-keyword", "published", createdAt),
				taxonomy: adminPostsIntegrationTaxonomy{categories: categories[3:], tags: tags[:1]},
			},
			{
				post:     adminPostsIntegrationPost(t, "30000000-0000-0000-0000-000000000004", "Wrong tag", "needle-q-wrong-tag", "summary", "needle-keyword", "published", createdAt),
				taxonomy: adminPostsIntegrationTaxonomy{categories: categories[:1], tags: tags[3:]},
			},
			{
				post:     adminPostsIntegrationPost(t, "30000000-0000-0000-0000-000000000005", "Wrong status", "needle-q-wrong-status", "summary", "needle-keyword", "draft", createdAt),
				taxonomy: adminPostsIntegrationTaxonomy{categories: categories[:1], tags: tags[:1]},
			},
			{
				post:     adminPostsIntegrationPost(t, "30000000-0000-0000-0000-000000000006", "Missing q", "q-does-not-match", "summary", "needle-keyword", "published", createdAt),
				taxonomy: adminPostsIntegrationTaxonomy{categories: categories[:1], tags: tags[:1]},
			},
			{
				post:     adminPostsIntegrationPost(t, "30000000-0000-0000-0000-000000000007", "Missing keyword", "needle-q-missing-keyword", "summary", "keyword-does-not-match", "published", createdAt),
				taxonomy: adminPostsIntegrationTaxonomy{categories: categories[:1], tags: tags[:1]},
			},
		}
		for i := range posts {
			adminPostsIntegrationCreatePost(t, db, &posts[i].post, posts[i].taxonomy)
		}

		response := adminPostsIntegrationList(t, db, url.Values{
			"category_id": {categories[0].ID.String()},
			"tag_id":      {tags[0].ID.String()},
			"status":      {"published"},
			"q":           {"needle-q"},
			"keyword":     {"needle-keyword"},
			"sort":        {"title"},
			"page_size":   {"100"},
		})

		if response.Pagination.Total != 2 {
			t.Fatalf("total = %d, want 2; data=%#v", response.Pagination.Total, response.Data)
		}
		if response.Pagination.TotalPages != 1 {
			t.Fatalf("total_pages = %d, want 1", response.Pagination.TotalPages)
		}
		adminPostsIntegrationAssertPostIDs(t, response.Data, posts[0].post.ID, posts[1].post.ID)
		for _, post := range response.Data {
			adminPostsIntegrationAssertCategoryNames(t, post.Categories, "Alpha Category", "Beta Category", "Target Category")
			adminPostsIntegrationAssertTagNames(t, post.Tags, "Alpha Tag", "Target Tag", "Zulu Tag")
		}
	})

	t.Run("uses post id as a stable cross-page tie breaker", func(t *testing.T) {
		db := openHTTPAPIPostgresTestSchema(t, "admin_posts_stable")
		migrateAdminPostsIntegrationSchema(t, db)

		createdAt := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
		posts := []model.Post{
			adminPostsIntegrationPost(t, "40000000-0000-0000-0000-000000000003", "Same title", "same-title-3", "", "", "draft", createdAt),
			adminPostsIntegrationPost(t, "40000000-0000-0000-0000-000000000001", "Same title", "same-title-1", "", "", "draft", createdAt),
			adminPostsIntegrationPost(t, "40000000-0000-0000-0000-000000000002", "Same title", "same-title-2", "", "", "draft", createdAt),
		}
		adminPostsIntegrationCreate(t, db, &posts)

		wantIDs := []uuid.UUID{posts[1].ID, posts[2].ID, posts[0].ID}
		for page, wantID := range wantIDs {
			response := adminPostsIntegrationList(t, db, url.Values{
				"page":      {adminPostsIntegrationPageNumber(page + 1)},
				"page_size": {"1"},
				"sort":      {"title"},
			})
			if response.Pagination.Total != 3 || response.Pagination.TotalPages != 3 {
				t.Fatalf("page %d pagination = %#v, want total=3 total_pages=3", page+1, response.Pagination)
			}
			adminPostsIntegrationAssertPostIDs(t, response.Data, wantID)
		}
	})

	t.Run("returns an empty huge out-of-range page with complete pagination metadata", func(t *testing.T) {
		db := openHTTPAPIPostgresTestSchema(t, "admin_posts_range")
		migrateAdminPostsIntegrationSchema(t, db)

		createdAt := time.Date(2026, time.July, 13, 11, 0, 0, 0, time.UTC)
		posts := []model.Post{
			adminPostsIntegrationPost(t, "50000000-0000-0000-0000-000000000001", "First", "range-first", "", "", "draft", createdAt),
			adminPostsIntegrationPost(t, "50000000-0000-0000-0000-000000000002", "Second", "range-second", "", "", "draft", createdAt),
			adminPostsIntegrationPost(t, "50000000-0000-0000-0000-000000000003", "Third", "range-third", "", "", "draft", createdAt),
		}
		adminPostsIntegrationCreate(t, db, &posts)

		response := adminPostsIntegrationList(t, db, url.Values{
			"page":      {"1000000"},
			"page_size": {"2"},
		})

		if len(response.Data) != 0 {
			t.Fatalf("data length = %d, want 0: %#v", len(response.Data), response.Data)
		}
		if response.Pagination.Page != 1_000_000 || response.Pagination.PageSize != 2 {
			t.Fatalf("pagination page metadata = %#v, want page=1000000 page_size=2", response.Pagination)
		}
		if response.Pagination.Total != 3 || response.Pagination.TotalPages != 2 {
			t.Fatalf("pagination totals = %#v, want total=3 total_pages=2", response.Pagination)
		}
	})
}

type adminPostsIntegrationTaxonomy struct {
	categories []model.Category
	tags       []model.Tag
}

func migrateAdminPostsIntegrationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&model.MediaAsset{},
		&model.Category{},
		&model.Tag{},
		&model.Post{},
	); err != nil {
		t.Fatalf("migrate admin posts integration schema: %v", err)
	}
}

func adminPostsIntegrationCreate(t *testing.T, db *gorm.DB, value interface{}) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("create admin posts integration fixture: %v", err)
	}
}

func adminPostsIntegrationCreatePost(t *testing.T, db *gorm.DB, post *model.Post, taxonomy adminPostsIntegrationTaxonomy) {
	t.Helper()
	adminPostsIntegrationCreate(t, db, post)
	if err := db.Model(post).Association("Categories").Append(&taxonomy.categories); err != nil {
		t.Fatalf("associate post categories: %v", err)
	}
	if err := db.Model(post).Association("Tags").Append(&taxonomy.tags); err != nil {
		t.Fatalf("associate post tags: %v", err)
	}
}

func adminPostsIntegrationPost(t *testing.T, id, title, slug, summary, content, status string, createdAt time.Time) model.Post {
	t.Helper()
	return model.Post{
		Base: model.Base{
			ID:        adminPostsIntegrationUUID(t, id),
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		Title:        title,
		Slug:         slug,
		Summary:      summary,
		ContentMD:    content,
		Status:       status,
		Visibility:   "public",
		AllowComment: true,
	}
}

func adminPostsIntegrationList(t *testing.T, db *gorm.DB, query url.Values) adminPostsIntegrationResponse {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "90000000-0000-0000-0000-000000000001")
		c.Set("permissions", []string{"content:read_all"})
		c.Next()
	})
	router.GET("/posts", listAdminPosts(db))

	request := httptest.NewRequest(http.MethodGet, "/posts?"+query.Encode(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list admin posts status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var response adminPostsIntegrationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list admin posts response: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func adminPostsIntegrationUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse fixture UUID %q: %v", value, err)
	}
	return id
}

func adminPostsIntegrationPageNumber(page int) string {
	return strconv.Itoa(page)
}

func adminPostsIntegrationAssertPostIDs(t *testing.T, posts []model.Post, want ...uuid.UUID) {
	t.Helper()
	if len(posts) != len(want) {
		t.Fatalf("post count = %d, want %d: %#v", len(posts), len(want), posts)
	}
	for i := range want {
		if posts[i].ID != want[i] {
			t.Fatalf("post[%d].id = %s, want %s", i, posts[i].ID, want[i])
		}
	}
}

func adminPostsIntegrationAssertCategoryNames(t *testing.T, categories []model.Category, want ...string) {
	t.Helper()
	if len(categories) != len(want) {
		t.Fatalf("category count = %d, want %d: %#v", len(categories), len(want), categories)
	}
	for i := range want {
		if categories[i].Name != want[i] {
			t.Fatalf("category[%d].name = %q, want %q", i, categories[i].Name, want[i])
		}
	}
}

func adminPostsIntegrationAssertTagNames(t *testing.T, tags []model.Tag, want ...string) {
	t.Helper()
	if len(tags) != len(want) {
		t.Fatalf("tag count = %d, want %d: %#v", len(tags), len(want), tags)
	}
	for i := range want {
		if tags[i].Name != want[i] {
			t.Fatalf("tag[%d].name = %q, want %q", i, tags[i].Name, want[i])
		}
	}
}
