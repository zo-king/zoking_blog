package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var seriesMigrationSchemaPattern = regexp.MustCompile(`^series_contract_[0-9a-f]{32}$`)

func TestSeriesPostgresContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openSeriesPostgresTestSchema(t)
	applySeriesIntegrationMigrations(t, db)

	author := model.User{
		Email:        "series-test@example.com",
		Username:     "series-test",
		PasswordHash: "not-used",
		DisplayName:  "Series Test",
		Status:       "active",
	}
	createSeriesFixture(t, db, &author)

	t.Run("slug conflict has a stable code", func(t *testing.T) {
		createSeriesFixture(t, db, &model.Series{Name: "Existing", Slug: "existing", Enabled: true})
		response := serveSeriesHandler(t, author.ID, http.MethodPost, "/series", "/series",
			`{"name":"Duplicate","slug":"existing"}`, createSeries(db))
		assertSeriesError(t, response, http.StatusConflict, "SERIES_SLUG_CONFLICT")
	})

	t.Run("post series fields require a valid pair and existing series", func(t *testing.T) {
		missingOrder := serveSeriesHandler(t, author.ID, http.MethodPost, "/posts", "/posts",
			`{"title":"Missing order","slug":"missing-order","series_id":"10000000-0000-0000-0000-000000000001"}`, createPost(db))
		assertSeriesError(t, missingOrder, http.StatusUnprocessableEntity, "VALIDATION_FAILED")

		missingSeries := serveSeriesHandler(t, author.ID, http.MethodPost, "/posts", "/posts",
			`{"title":"Missing series","slug":"missing-series","series_id":"10000000-0000-0000-0000-000000000099","series_order":1}`, createPost(db))
		assertSeriesError(t, missingSeries, http.StatusNotFound, "SERIES_NOT_FOUND")
	})

	t.Run("order conflict has a stable code", func(t *testing.T) {
		item := model.Series{Name: "Ordered", Slug: "ordered", Enabled: true}
		createSeriesFixture(t, db, &item)
		order := 1
		createSeriesFixture(t, db, &model.Post{
			Title: "First", Slug: "ordered-first", Status: "draft", Visibility: "public",
			AllowComment: true, AuthorID: &author.ID, SeriesID: &item.ID, SeriesOrder: &order,
		})

		response := serveSeriesHandler(t, author.ID, http.MethodPost, "/posts", "/posts",
			`{"title":"Duplicate order","slug":"duplicate-order","series_id":"`+item.ID.String()+`","series_order":1}`, createPost(db))
		assertSeriesError(t, response, http.StatusConflict, "SERIES_ORDER_CONFLICT")
	})

	t.Run("empty series id and null order clear the association", func(t *testing.T) {
		item := model.Series{Name: "Clearable", Slug: "clearable", Enabled: true}
		createSeriesFixture(t, db, &item)
		order := 7
		post := model.Post{
			Title: "Clear me", Slug: "clear-me", Status: "draft", Visibility: "public",
			AllowComment: true, AuthorID: &author.ID, SeriesID: &item.ID, SeriesOrder: &order,
		}
		createSeriesFixture(t, db, &post)

		response := serveSeriesHandler(t, author.ID, http.MethodPatch, "/posts/:id", "/posts/"+post.ID.String(),
			`{"series_id":"","series_order":null}`, updatePost(db))
		if response.Code != http.StatusOK {
			t.Fatalf("clear series status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var refreshed model.Post
		if err := db.First(&refreshed, "id = ?", post.ID).Error; err != nil {
			t.Fatalf("reload cleared post: %v", err)
		}
		if refreshed.SeriesID != nil || refreshed.SeriesOrder != nil {
			t.Fatalf("cleared series fields = %v/%v, want nil/nil", refreshed.SeriesID, refreshed.SeriesOrder)
		}
	})

	t.Run("referenced series cannot be deleted", func(t *testing.T) {
		item := model.Series{Name: "In use", Slug: "in-use", Enabled: true}
		createSeriesFixture(t, db, &item)
		order := 1
		post := model.Post{
			Title: "Reference", Slug: "series-reference", Status: "draft", Visibility: "public",
			AllowComment: true, AuthorID: &author.ID, SeriesID: &item.ID, SeriesOrder: &order,
		}
		createSeriesFixture(t, db, &post)
		deletedOrder := 2
		deletedPost := model.Post{
			Title: "Deleted reference", Slug: "deleted-series-reference", Status: "draft", Visibility: "public",
			AllowComment: true, AuthorID: &author.ID, SeriesID: &item.ID, SeriesOrder: &deletedOrder,
		}
		createSeriesFixture(t, db, &deletedPost)
		if err := db.Delete(&deletedPost).Error; err != nil {
			t.Fatalf("soft-delete series post fixture: %v", err)
		}

		response := serveSeriesHandler(t, author.ID, http.MethodDelete, "/series/:id", "/series/"+item.ID.String(), "", deleteSeries(db))
		assertSeriesError(t, response, http.StatusConflict, "SERIES_IN_USE")
		var count int64
		if err := db.Model(&model.Series{}).Where("id = ?", item.ID).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("active referenced series count = %d, err=%v; want 1", count, err)
		}
		var linked model.Post
		if err := db.First(&linked, "id = ?", post.ID).Error; err != nil || linked.SeriesID == nil || *linked.SeriesID != item.ID {
			t.Fatalf("post link changed after rejected delete: series_id=%v err=%v", linked.SeriesID, err)
		}
		listResponse := serveSeriesHandler(t, author.ID, http.MethodGet, "/series", "/series", "", listAdminSeries(db))
		if listResponse.Code != http.StatusOK {
			t.Fatalf("admin series list status = %d, want 200; body=%s", listResponse.Code, listResponse.Body.String())
		}
		var listEnvelope struct {
			Data []model.Series `json:"data"`
		}
		decodeSeriesResponse(t, listResponse, &listEnvelope)
		var found bool
		for _, listed := range listEnvelope.Data {
			if listed.ID == item.ID {
				found = true
				if listed.PostCount != 1 {
					t.Fatalf("admin referenced series post_count = %d, want 1", listed.PostCount)
				}
			}
		}
		if !found {
			t.Fatalf("admin series list did not include %s", item.ID)
		}
	})

	t.Run("create key-share lock serializes with series deletion", func(t *testing.T) {
		item := model.Series{Name: "Locking", Slug: "locking", Enabled: true}
		createSeriesFixture(t, db, &item)
		createTx := db.Begin()
		if createTx.Error != nil {
			t.Fatalf("begin create transaction: %v", createTx.Error)
		}
		defer createTx.Rollback()
		if err := ensureSeriesExists(createTx, &item.ID); err != nil {
			t.Fatalf("lock active series for create: %v", err)
		}

		deleteRouter := gin.New()
		deleteRouter.DELETE("/series/:id", deleteSeries(db))
		deleteDone := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			request := httptest.NewRequest(http.MethodDelete, "/series/"+item.ID.String(), nil)
			response := httptest.NewRecorder()
			deleteRouter.ServeHTTP(response, request)
			deleteDone <- response
		}()

		select {
		case response := <-deleteDone:
			t.Fatalf("delete completed while create held KEY SHARE lock: status=%d body=%s", response.Code, response.Body.String())
		case <-time.After(100 * time.Millisecond):
		}

		order := 1
		post := model.Post{
			Title: "Wins race", Slug: "wins-series-race", Status: "draft", Visibility: "public",
			AllowComment: true, AuthorID: &author.ID, SeriesID: &item.ID, SeriesOrder: &order,
		}
		if err := createTx.Create(&post).Error; err != nil {
			t.Fatalf("insert post while holding series lock: %v", err)
		}
		if err := createTx.Commit().Error; err != nil {
			t.Fatalf("commit post create: %v", err)
		}

		select {
		case response := <-deleteDone:
			assertSeriesError(t, response, http.StatusConflict, "SERIES_IN_USE")
		case <-time.After(2 * time.Second):
			t.Fatal("delete did not resume after create committed")
		}
	})

	t.Run("delete committed first makes create return series not found", func(t *testing.T) {
		item := model.Series{Name: "Delete first", Slug: "delete-first", Enabled: true}
		createSeriesFixture(t, db, &item)
		deleted := serveSeriesHandler(t, author.ID, http.MethodDelete, "/series/:id", "/series/"+item.ID.String(), "", deleteSeries(db))
		if deleted.Code != http.StatusOK {
			t.Fatalf("delete-first status = %d, want 200; body=%s", deleted.Code, deleted.Body.String())
		}
		response := serveSeriesHandler(t, author.ID, http.MethodPost, "/posts", "/posts",
			`{"title":"Too late","slug":"too-late","series_id":"`+item.ID.String()+`","series_order":1}`, createPost(db))
		assertSeriesError(t, response, http.StatusNotFound, "SERIES_NOT_FOUND")
	})

	t.Run("public detail sorts and filters posts and preloads series cover", func(t *testing.T) {
		asset := model.MediaAsset{
			Filename: "series-cover.jpg", OriginalName: "series-cover.jpg", MimeType: "image/jpeg",
			StorageDriver: "local", StorageKey: "series/cover.jpg", PublicURL: "/media/series-cover.jpg",
			Checksum: "series-cover", Status: "ready",
		}
		createSeriesFixture(t, db, &asset)
		item := model.Series{Name: "Public", Slug: "public-series", Enabled: true, CoverMediaID: &asset.ID}
		createSeriesFixture(t, db, &item)
		for _, fixture := range []struct {
			title      string
			slug       string
			order      int
			status     string
			visibility string
		}{
			{"Third", "public-third", 3, "published", "public"},
			{"First", "public-first", 1, "published", "public"},
			{"Second", "public-second", 2, "published", "public"},
			{"Draft", "public-draft", 4, "draft", "public"},
			{"Private", "public-private", 5, "published", "private"},
		} {
			publishedAt := time.Now()
			post := model.Post{
				Title: fixture.title, Slug: fixture.slug, Status: fixture.status, Visibility: fixture.visibility,
				AllowComment: true, PublishedAt: &publishedAt, AuthorID: &author.ID,
				SeriesID: &item.ID, SeriesOrder: &fixture.order,
			}
			createSeriesFixture(t, db, &post)
		}

		response := serveSeriesHandler(t, author.ID, http.MethodGet, "/series/:slug", "/series/"+item.Slug, "", getPublicSeries(db))
		if response.Code != http.StatusOK {
			t.Fatalf("public detail status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var envelope struct {
			Data model.Series `json:"data"`
		}
		decodeSeriesResponse(t, response, &envelope)
		if len(envelope.Data.Posts) != 3 {
			t.Fatalf("public posts count = %d, want 3", len(envelope.Data.Posts))
		}
		for index, want := range []string{"First", "Second", "Third"} {
			post := envelope.Data.Posts[index]
			if post.Title != want {
				t.Fatalf("public post[%d] = %q, want %q", index, post.Title, want)
			}
			if post.Series == nil || post.Series.CoverMedia == nil || post.Series.CoverMedia.ID != asset.ID {
				t.Fatalf("public post[%d] did not preload Series.CoverMedia: %#v", index, post.Series)
			}
		}
		listResponse := serveSeriesHandler(t, author.ID, http.MethodGet, "/series", "/series", "", listPublicSeries(db))
		if listResponse.Code != http.StatusOK {
			t.Fatalf("public series list status = %d, want 200; body=%s", listResponse.Code, listResponse.Body.String())
		}
		var listEnvelope struct {
			Data []model.Series `json:"data"`
		}
		decodeSeriesResponse(t, listResponse, &listEnvelope)
		for _, listed := range listEnvelope.Data {
			if listed.ID == item.ID && listed.PostCount != 3 {
				t.Fatalf("public series post_count = %d, want 3", listed.PostCount)
			}
		}
	})
}

func openSeriesPostgresTestSchema(t *testing.T) *gorm.DB {
	t.Helper()
	isolatedDB := openHTTPAPIPostgresTestSchema(t, "series_contract")
	var schema string
	if err := isolatedDB.Raw("select current_schema()").Scan(&schema).Error; err != nil || schema == "" {
		t.Fatalf("resolve isolated schema: schema=%q err=%v", schema, err)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse test DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("reopen isolated schema with public extensions visible: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve series integration pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func applySeriesIntegrationMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	migrationsDir := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../db/migrations"))
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve isolated database: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	var schema string
	if err := db.Raw("select current_schema()").Scan(&schema).Error; err != nil || !seriesMigrationSchemaPattern.MatchString(schema) {
		t.Fatalf("resolve safe series migration schema: schema=%q err=%v", schema, err)
	}
	goose.SetTableName(schema + ".goose_db_version")
	t.Cleanup(func() { goose.SetTableName("goose_db_version") })
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("apply migrations in isolated schema: %v", err)
	}
}

func createSeriesFixture(t *testing.T, db *gorm.DB, value interface{}) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("create series fixture: %v", err)
	}
}

func serveSeriesHandler(t *testing.T, userID uuid.UUID, method, route, target, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID.String())
		c.Set("permissions", []string{"*"})
		c.Next()
	})
	switch method {
	case http.MethodGet:
		router.GET(route, handler)
	case http.MethodPost:
		router.POST(route, handler)
	case http.MethodPatch:
		router.PATCH(route, handler)
	case http.MethodDelete:
		router.DELETE(route, handler)
	default:
		t.Fatalf("unsupported method %s", method)
	}
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func assertSeriesError(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, wantStatus, response.Body.String())
	}
	var envelope struct {
		Error ErrorBody `json:"error"`
	}
	decodeSeriesResponse(t, response, &envelope)
	if envelope.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q; body=%s", envelope.Error.Code, wantCode, response.Body.String())
	}
}

func decodeSeriesResponse(t *testing.T, response *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
}
