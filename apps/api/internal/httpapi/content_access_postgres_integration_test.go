package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

type contentAccessListResponse[T any] struct {
	Data       []T            `json:"data"`
	Pagination paginationMeta `json:"pagination"`
}

func TestContentAccessPostgresOwnerIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openHTTPAPIPostgresTestSchema(t, "content_owner")
	migrateContentAccessIntegrationSchema(t, db)

	ownerA := uuid.MustParse("81000000-0000-0000-0000-000000000001")
	ownerB := uuid.MustParse("81000000-0000-0000-0000-000000000002")
	now := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
	postA := contentAccessPost("82000000-0000-0000-0000-000000000001", "Owner A draft", "owner-a-draft", "draft", &ownerA, now)
	postAPublished := contentAccessPost("82000000-0000-0000-0000-000000000002", "Owner A published", "owner-a-published", "published", &ownerA, now.Add(time.Minute))
	postB := contentAccessPost("82000000-0000-0000-0000-000000000003", "Owner B draft", "owner-b-draft", "draft", &ownerB, now.Add(2*time.Minute))
	postNull := contentAccessPost("82000000-0000-0000-0000-000000000004", "No owner", "no-owner", "draft", nil, now.Add(3*time.Minute))
	pageA := contentAccessPage("83000000-0000-0000-0000-000000000001", "Owner A page", "owner-a-page", &ownerA, now)
	pageB := contentAccessPage("83000000-0000-0000-0000-000000000002", "Owner B page", "owner-b-page", &ownerB, now.Add(time.Minute))
	pageNull := contentAccessPage("83000000-0000-0000-0000-000000000003", "No owner page", "no-owner-page", nil, now.Add(2*time.Minute))
	contentAccessCreate(t, db, &[]model.Post{postA, postAPublished, postB, postNull})
	contentAccessCreate(t, db, &[]model.Page{pageA, pageB, pageNull})

	t.Run("owner and global list scopes include matching totals", func(t *testing.T) {
		postsRecorder := contentAccessServe(t, db, http.MethodGet, "/posts", "/posts?page=1&page_size=20", "", ownerA, []string{"author"}, nil, listAdminPosts(db))
		var posts contentAccessListResponse[model.Post]
		contentAccessDecode(t, postsRecorder, http.StatusOK, &posts)
		contentAccessAssertIDs(t, posts.Data, []uuid.UUID{postAPublished.ID, postA.ID}, func(item model.Post) uuid.UUID { return item.ID })
		if posts.Pagination.Total != 2 {
			t.Fatalf("owner post total = %d, want 2", posts.Pagination.Total)
		}

		pagesRecorder := contentAccessServe(t, db, http.MethodGet, "/pages", "/pages?page=1&page_size=20", "", ownerA, []string{"author"}, nil, listAdminPages(db))
		var pages contentAccessListResponse[model.Page]
		contentAccessDecode(t, pagesRecorder, http.StatusOK, &pages)
		contentAccessAssertIDs(t, pages.Data, []uuid.UUID{pageA.ID}, func(item model.Page) uuid.UUID { return item.ID })
		if pages.Pagination.Total != 1 {
			t.Fatalf("owner page total = %d, want 1", pages.Pagination.Total)
		}

		globalRecorder := contentAccessServe(t, db, http.MethodGet, "/posts", "/posts?page=1&page_size=20", "", ownerA, nil, []string{"content:read_all"}, listAdminPosts(db))
		var global contentAccessListResponse[model.Post]
		contentAccessDecode(t, globalRecorder, http.StatusOK, &global)
		if global.Pagination.Total != 4 || len(global.Data) != 4 {
			t.Fatalf("global posts = %d total=%d, want 4", len(global.Data), global.Pagination.Total)
		}
	})

	t.Run("create forces authenticated owner", func(t *testing.T) {
		postBody := `{"title":"Created by A","slug":"created-by-a","summary":"","content_md":"","status":"draft","author_id":"` + ownerB.String() + `"}`
		postRecorder := contentAccessServe(t, db, http.MethodPost, "/posts", "/posts", postBody, ownerA, []string{"author"}, []string{"post:create"}, createPost(db))
		var postResponse struct {
			Data model.Post `json:"data"`
		}
		contentAccessDecode(t, postRecorder, http.StatusCreated, &postResponse)
		if postResponse.Data.AuthorID == nil || *postResponse.Data.AuthorID != ownerA {
			t.Fatalf("created post author = %v, want %s", postResponse.Data.AuthorID, ownerA)
		}

		pageBody := `{"title":"Created page by A","slug":"created-page-by-a","content_md":"","status":"draft","author_id":"` + ownerB.String() + `"}`
		pageRecorder := contentAccessServe(t, db, http.MethodPost, "/pages", "/pages", pageBody, ownerA, nil, []string{"page:create"}, createPage(db))
		var pageResponse struct {
			Data model.Page `json:"data"`
		}
		contentAccessDecode(t, pageRecorder, http.StatusCreated, &pageResponse)
		if pageResponse.Data.AuthorID == nil || *pageResponse.Data.AuthorID != ownerA {
			t.Fatalf("created page author = %v, want %s", pageResponse.Data.AuthorID, ownerA)
		}
	})

	t.Run("foreign content operations return 404 without side effects", func(t *testing.T) {
		cases := []struct {
			name    string
			method  string
			pattern string
			path    string
			body    string
			handler gin.HandlerFunc
		}{
			{name: "get post", method: http.MethodGet, pattern: "/posts/:id", path: "/posts/" + postB.ID.String(), handler: getAdminPost(db)},
			{name: "update post", method: http.MethodPatch, pattern: "/posts/:id", path: "/posts/" + postB.ID.String(), body: `{"title":"stolen"}`, handler: updatePost(db)},
			{name: "delete post", method: http.MethodDelete, pattern: "/posts/:id", path: "/posts/" + postB.ID.String(), handler: deletePost(db)},
			{name: "preview post", method: http.MethodPost, pattern: "/posts/:id/preview", path: "/posts/" + postB.ID.String() + "/preview", handler: previewPost(db, config.Config{})},
			{name: "publish post", method: http.MethodPost, pattern: "/posts/:id/publish", path: "/posts/" + postB.ID.String() + "/publish", handler: publishPost(db, config.Config{})},
			{name: "get page", method: http.MethodGet, pattern: "/pages/:id", path: "/pages/" + pageB.ID.String(), handler: getAdminPage(db)},
			{name: "update page", method: http.MethodPatch, pattern: "/pages/:id", path: "/pages/" + pageB.ID.String(), body: `{"title":"stolen page"}`, handler: updatePage(db)},
			{name: "delete page", method: http.MethodDelete, pattern: "/pages/:id", path: "/pages/" + pageB.ID.String(), handler: deletePage(db)},
			{name: "preview page", method: http.MethodPost, pattern: "/pages/:id/preview", path: "/pages/" + pageB.ID.String() + "/preview", handler: previewPage(db, config.Config{})},
			{name: "publish page", method: http.MethodPost, pattern: "/pages/:id/publish", path: "/pages/" + pageB.ID.String() + "/publish", handler: publishPage(db)},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				recorder := contentAccessServe(t, db, test.method, test.pattern, test.path, test.body, ownerA, []string{"author"}, []string{"post:update", "post:delete", "post:publish", "page:update", "page:delete", "page:publish"}, test.handler)
				if recorder.Code != http.StatusNotFound {
					t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
				}
			})
		}

		var refreshedPost model.Post
		if err := db.First(&refreshedPost, "id = ?", postB.ID).Error; err != nil || refreshedPost.Title != postB.Title {
			t.Fatalf("foreign post changed: title=%q err=%v", refreshedPost.Title, err)
		}
		var refreshedPage model.Page
		if err := db.First(&refreshedPage, "id = ?", pageB.ID).Error; err != nil || refreshedPage.Title != pageB.Title {
			t.Fatalf("foreign page changed: title=%q err=%v", refreshedPage.Title, err)
		}
		var jobCount, previewCount int64
		if err := db.Model(&model.PublishJob{}).Count(&jobCount).Error; err != nil || jobCount != 0 {
			t.Fatalf("unexpected publish jobs = %d err=%v", jobCount, err)
		}
		if err := db.Model(&model.PublishPreview{}).Count(&previewCount).Error; err != nil || previewCount != 0 {
			t.Fatalf("unexpected publish previews = %d err=%v", previewCount, err)
		}
	})

	t.Run("publish state requires explicit publish endpoint", func(t *testing.T) {
		createRecorder := contentAccessServe(t, db, http.MethodPost, "/posts", "/posts", `{"title":"Bypass","slug":"publish-bypass","status":"published"}`, ownerA, []string{"author"}, []string{"post:create"}, createPost(db))
		if createRecorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("published create status = %d, want 422; body=%s", createRecorder.Code, createRecorder.Body.String())
		}
		transitionRecorder := contentAccessServe(t, db, http.MethodPatch, "/posts/:id", "/posts/"+postA.ID.String(), `{"status":"published"}`, ownerA, []string{"author"}, []string{"post:update"}, updatePost(db))
		if transitionRecorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("post publish transition status = %d, want 422; body=%s", transitionRecorder.Code, transitionRecorder.Body.String())
		}
		updateRecorder := contentAccessServe(t, db, http.MethodPatch, "/posts/:id", "/posts/"+postAPublished.ID.String(), `{"title":"Changed live"}`, ownerA, []string{"author"}, []string{"post:update"}, updatePost(db))
		if updateRecorder.Code != http.StatusForbidden {
			t.Fatalf("published update status = %d, want 403; body=%s", updateRecorder.Code, updateRecorder.Body.String())
		}

		pageAPublished := contentAccessPage("83000000-0000-0000-0000-000000000004", "Owner A published page", "owner-a-published-page", &ownerA, now.Add(4*time.Minute))
		pageAPublished.Status = "published"
		pageAPublished.PublishedAt = &pageAPublished.CreatedAt
		contentAccessCreate(t, db, &pageAPublished)
		createPageRecorder := contentAccessServe(t, db, http.MethodPost, "/pages", "/pages", `{"title":"Page bypass","slug":"page-publish-bypass","status":"published"}`, ownerA, []string{"author"}, []string{"page:create"}, createPage(db))
		if createPageRecorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("published page create status = %d, want 422; body=%s", createPageRecorder.Code, createPageRecorder.Body.String())
		}
		pageTransitionRecorder := contentAccessServe(t, db, http.MethodPatch, "/pages/:id", "/pages/"+pageA.ID.String(), `{"status":"published"}`, ownerA, []string{"author"}, []string{"page:update"}, updatePage(db))
		if pageTransitionRecorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("page publish transition status = %d, want 422; body=%s", pageTransitionRecorder.Code, pageTransitionRecorder.Body.String())
		}
		updatePageRecorder := contentAccessServe(t, db, http.MethodPatch, "/pages/:id", "/pages/"+pageAPublished.ID.String(), `{"title":"Changed live page"}`, ownerA, []string{"author"}, []string{"page:update"}, updatePage(db))
		if updatePageRecorder.Code != http.StatusForbidden {
			t.Fatalf("published page update status = %d, want 403; body=%s", updatePageRecorder.Code, updatePageRecorder.Body.String())
		}

		var refreshed model.Post
		if err := db.First(&refreshed, "id = ?", postAPublished.ID).Error; err != nil || refreshed.Title != postAPublished.Title {
			t.Fatalf("published post changed: title=%q err=%v", refreshed.Title, err)
		}
		var refreshedDraft model.Post
		if err := db.First(&refreshedDraft, "id = ?", postA.ID).Error; err != nil || refreshedDraft.Status != "draft" {
			t.Fatalf("draft post status changed: status=%q err=%v", refreshedDraft.Status, err)
		}
		var refreshedPublishedPage model.Page
		if err := db.First(&refreshedPublishedPage, "id = ?", pageAPublished.ID).Error; err != nil || refreshedPublishedPage.Title != pageAPublished.Title {
			t.Fatalf("published page changed: title=%q err=%v", refreshedPublishedPage.Title, err)
		}
		var refreshedDraftPage model.Page
		if err := db.First(&refreshedDraftPage, "id = ?", pageA.ID).Error; err != nil || refreshedDraftPage.Status != "draft" {
			t.Fatalf("draft page status changed: status=%q err=%v", refreshedDraftPage.Status, err)
		}
	})

	t.Run("publish records follow related content owners", func(t *testing.T) {
		jobs := []model.PublishJob{
			contentAccessJob("84000000-0000-0000-0000-000000000001", &postA.ID, nil, now),
			contentAccessJob("84000000-0000-0000-0000-000000000002", nil, &pageA.ID, now.Add(time.Minute)),
			contentAccessJob("84000000-0000-0000-0000-000000000003", &postB.ID, nil, now.Add(2*time.Minute)),
			contentAccessJob("84000000-0000-0000-0000-000000000004", nil, nil, now.Add(3*time.Minute)),
		}
		contentAccessCreate(t, db, &jobs)
		releases := []model.PublishRelease{
			contentAccessRelease("85000000-0000-0000-0000-000000000001", jobs[0].ID, &postA.ID, nil, now),
			contentAccessRelease("85000000-0000-0000-0000-000000000002", jobs[2].ID, &postB.ID, nil, now.Add(time.Minute)),
			contentAccessRelease("85000000-0000-0000-0000-000000000003", jobs[3].ID, nil, nil, now.Add(2*time.Minute)),
		}
		contentAccessCreate(t, db, &releases)
		previews := []model.PublishPreview{
			contentAccessPreview("86000000-0000-0000-0000-000000000001", "preview-owner-a", &postA.ID, nil, ownerA, now),
			contentAccessPreview("86000000-0000-0000-0000-000000000002", "preview-owner-b", &postB.ID, nil, ownerB, now.Add(time.Minute)),
			contentAccessPreview("86000000-0000-0000-0000-000000000003", "preview-site", nil, nil, ownerA, now.Add(2*time.Minute)),
		}
		contentAccessCreate(t, db, &previews)

		jobRecorder := contentAccessServe(t, db, http.MethodGet, "/publish/jobs", "/publish/jobs?page=1&page_size=20", "", ownerA, []string{"author"}, []string{"publish:read"}, listPublishJobs(db))
		var jobResponse contentAccessListResponse[model.PublishJob]
		contentAccessDecode(t, jobRecorder, http.StatusOK, &jobResponse)
		if jobResponse.Pagination.Total != 2 {
			t.Fatalf("owner job total = %d, want 2", jobResponse.Pagination.Total)
		}

		releaseRecorder := contentAccessServe(t, db, http.MethodGet, "/publish/releases", "/publish/releases?page=1&page_size=20", "", ownerA, []string{"author"}, []string{"publish:read"}, listPublishReleases(db))
		var releaseResponse contentAccessListResponse[model.PublishRelease]
		contentAccessDecode(t, releaseRecorder, http.StatusOK, &releaseResponse)
		if releaseResponse.Pagination.Total != 1 || releaseResponse.Data[0].ID != releases[0].ID {
			t.Fatalf("owner releases = %#v total=%d", releaseResponse.Data, releaseResponse.Pagination.Total)
		}

		previewRecorder := contentAccessServe(t, db, http.MethodGet, "/publish/previews", "/publish/previews?page=1&page_size=20", "", ownerA, []string{"author"}, []string{"publish:read"}, listPublishPreviews(db))
		var previewResponse contentAccessListResponse[model.PublishPreview]
		contentAccessDecode(t, previewRecorder, http.StatusOK, &previewResponse)
		if previewResponse.Pagination.Total != 1 || previewResponse.Data[0].ID != previews[0].ID {
			t.Fatalf("owner previews = %#v total=%d", previewResponse.Data, previewResponse.Pagination.Total)
		}

		foreignJobRecorder := contentAccessServe(t, db, http.MethodGet, "/publish/jobs/:id", "/publish/jobs/"+jobs[2].ID.String(), "", ownerA, []string{"author"}, []string{"publish:read"}, getPublishJob(db))
		if foreignJobRecorder.Code != http.StatusNotFound {
			t.Fatalf("foreign job status = %d, want 404; body=%s", foreignJobRecorder.Code, foreignJobRecorder.Body.String())
		}

		retryRecorder := contentAccessServe(t, db, http.MethodPost, "/publish/jobs/:id/retry", "/publish/jobs/"+jobs[0].ID.String()+"/retry", "", ownerA, []string{"author"}, []string{"publish:create"}, retryPublishJob(db, config.Config{}))
		if retryRecorder.Code != http.StatusOK {
			t.Fatalf("owner retry status = %d, want 200; body=%s", retryRecorder.Code, retryRecorder.Body.String())
		}
		cancelRecorder := contentAccessServe(t, db, http.MethodPost, "/publish/jobs/:id/cancel", "/publish/jobs/"+jobs[0].ID.String()+"/cancel", "", ownerA, []string{"author"}, []string{"publish:create"}, cancelPublishJob(db))
		if cancelRecorder.Code != http.StatusOK {
			t.Fatalf("owner cancel status = %d, want 200; body=%s", cancelRecorder.Code, cancelRecorder.Body.String())
		}
		var ownerJob model.PublishJob
		if err := db.First(&ownerJob, "id = ?", jobs[0].ID).Error; err != nil || ownerJob.Status != "canceled" || ownerJob.RetryCount != 1 {
			t.Fatalf("owner job after retry/cancel = status %q retries %d err=%v", ownerJob.Status, ownerJob.RetryCount, err)
		}

		foreignActions := []struct {
			name    string
			pattern string
			path    string
			handler gin.HandlerFunc
		}{
			{name: "retry", pattern: "/publish/jobs/:id/retry", path: "/publish/jobs/" + jobs[2].ID.String() + "/retry", handler: retryPublishJob(db, config.Config{})},
			{name: "cancel", pattern: "/publish/jobs/:id/cancel", path: "/publish/jobs/" + jobs[2].ID.String() + "/cancel", handler: cancelPublishJob(db)},
		}
		for _, action := range foreignActions {
			t.Run("foreign job "+action.name, func(t *testing.T) {
				recorder := contentAccessServe(t, db, http.MethodPost, action.pattern, action.path, "", ownerA, []string{"author"}, []string{"publish:create"}, action.handler)
				if recorder.Code != http.StatusNotFound {
					t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
				}
			})
		}
		var foreignJob model.PublishJob
		if err := db.First(&foreignJob, "id = ?", jobs[2].ID).Error; err != nil || foreignJob.Status != "failed" || foreignJob.RetryCount != 0 {
			t.Fatalf("foreign job changed: status=%q retries=%d err=%v", foreignJob.Status, foreignJob.RetryCount, err)
		}

		promoteRecorder := contentAccessServe(t, db, http.MethodPost, "/publish/releases/:id/promote", "/publish/releases/"+releases[0].ID.String()+"/promote", "", ownerA, []string{"author"}, []string{"publish:rollback"}, promotePublishRelease(db, config.Config{}))
		if promoteRecorder.Code != http.StatusForbidden {
			t.Fatalf("owner-scoped promote status = %d, want 403; body=%s", promoteRecorder.Code, promoteRecorder.Body.String())
		}

		globalRecorder := contentAccessServe(t, db, http.MethodGet, "/publish/jobs", "/publish/jobs?page=1&page_size=20", "", ownerA, nil, []string{"publish:read", "content:read_all"}, listPublishJobs(db))
		var global contentAccessListResponse[model.PublishJob]
		contentAccessDecode(t, globalRecorder, http.StatusOK, &global)
		if global.Pagination.Total != 4 {
			t.Fatalf("global job total = %d, want 4", global.Pagination.Total)
		}
	})
}

func migrateContentAccessIntegrationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&model.MediaAsset{},
		&model.MediaUsage{},
		&model.Category{},
		&model.Tag{},
		&model.Post{},
		&model.Page{},
		&model.PublishJob{},
		&model.PublishRelease{},
		&model.PublishPreview{},
	); err != nil {
		t.Fatalf("migrate content access integration schema: %v", err)
	}
}

func contentAccessServe(t *testing.T, _ *gorm.DB, method, pattern, path, body string, userID uuid.UUID, roles, permissions []string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID.String())
		c.Set("roles", roles)
		c.Set("permissions", permissions)
		c.Next()
	})
	router.Handle(method, pattern, handler)
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func contentAccessDecode(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, target interface{}) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
}

func contentAccessCreate(t *testing.T, db *gorm.DB, value interface{}) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("create content access fixture: %v", err)
	}
}

func contentAccessAssertIDs[T any](t *testing.T, items []T, want []uuid.UUID, id func(T) uuid.UUID) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("item count = %d, want %d", len(items), len(want))
	}
	for index := range want {
		if got := id(items[index]); got != want[index] {
			t.Fatalf("item %d id = %s, want %s", index, got, want[index])
		}
	}
}

func contentAccessPost(id, title, slug, status string, authorID *uuid.UUID, createdAt time.Time) model.Post {
	post := model.Post{
		Base:         model.Base{ID: uuid.MustParse(id), CreatedAt: createdAt, UpdatedAt: createdAt},
		Title:        title,
		Slug:         slug,
		ContentMD:    "body",
		Status:       status,
		Visibility:   "public",
		AllowComment: true,
		AuthorID:     authorID,
	}
	if status == "published" {
		post.PublishedAt = &createdAt
	}
	return post
}

func contentAccessPage(id, title, slug string, authorID *uuid.UUID, createdAt time.Time) model.Page {
	return model.Page{
		Base:       model.Base{ID: uuid.MustParse(id), CreatedAt: createdAt, UpdatedAt: createdAt},
		Title:      title,
		Slug:       slug,
		ContentMD:  "body",
		Status:     "draft",
		Visibility: "public",
		AuthorID:   authorID,
	}
}

func contentAccessJob(id string, postID, pageID *uuid.UUID, createdAt time.Time) model.PublishJob {
	return model.PublishJob{
		Base:          model.Base{ID: uuid.MustParse(id), CreatedAt: createdAt, UpdatedAt: createdAt},
		PostID:        postID,
		PageID:        pageID,
		JobType:       "test",
		Status:        "failed",
		TriggerSource: "test",
		RunAt:         createdAt,
		ManifestJSON:  []byte(`{}`),
		LogJSON:       []byte(`[]`),
	}
}

func contentAccessRelease(id string, jobID uuid.UUID, postID, pageID *uuid.UUID, createdAt time.Time) model.PublishRelease {
	return model.PublishRelease{
		Base:         model.Base{ID: uuid.MustParse(id), CreatedAt: createdAt, UpdatedAt: createdAt},
		JobID:        jobID,
		ReleaseKey:   "release-" + id[len(id)-4:],
		Status:       "published",
		PostID:       postID,
		PageID:       pageID,
		OutputPath:   "output/" + id,
		ManifestJSON: []byte(`{}`),
	}
}

func contentAccessPreview(id, key string, postID, pageID *uuid.UUID, requestedBy uuid.UUID, createdAt time.Time) model.PublishPreview {
	return model.PublishPreview{
		Base:         model.Base{ID: uuid.MustParse(id), CreatedAt: createdAt, UpdatedAt: createdAt},
		PreviewKey:   key,
		Scope:        "test",
		Status:       "ready",
		PostID:       postID,
		PageID:       pageID,
		RequestedBy:  &requestedBy,
		ManifestJSON: []byte(`{}`),
		LogJSON:      []byte(`[]`),
	}
}
