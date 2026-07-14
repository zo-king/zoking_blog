package httpapi

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/auth"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
)

func TestContentQualityRouteAuthenticationPostgres(t *testing.T) {
	db := openHTTPAPIPostgresTestSchema(t, "quality_auth")
	migrateContentAccessIntegrationSchema(t, db)
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate auth quality schema: %v", err)
	}
	for _, statement := range []string{
		`create table roles (id uuid primary key, code text not null unique, name text not null, description text not null default '', is_system boolean not null default false, created_at timestamptz not null default now(), updated_at timestamptz not null default now())`,
		`create table permissions (id uuid primary key, code text not null unique, name text not null, resource text not null, action text not null, created_at timestamptz not null default now())`,
		`create table user_roles (user_id uuid not null references users(id) on delete cascade, role_id uuid not null references roles(id) on delete cascade, created_at timestamptz not null default now(), primary key (user_id, role_id))`,
		`create table role_permissions (role_id uuid not null references roles(id) on delete cascade, permission_id uuid not null references permissions(id) on delete cascade, created_at timestamptz not null default now(), primary key (role_id, permission_id))`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create auth quality table: %v", err)
		}
	}
	user := model.User{Email: "viewer@quality.test", Username: "quality-viewer", PasswordHash: "unused", Status: "active"}
	contentAccessCreate(t, db, &user)
	roleID := uuid.New()
	if err := db.Exec(`insert into roles (id, code, name) values (?, 'viewer', 'Viewer')`, roleID).Error; err != nil {
		t.Fatalf("create viewer role: %v", err)
	}
	if err := db.Exec(`insert into user_roles (user_id, role_id) values (?, ?)`, user.ID, roleID).Error; err != nil {
		t.Fatalf("assign viewer role: %v", err)
	}

	cfg := config.Config{
		AppEnv: "test", JWTSecret: "quality-auth-secret", AccessTokenTTL: time.Hour,
		MediaPublicBaseURL: "/media-files", PublishPreviewPublicBaseURL: "/preview-files",
	}
	router := NewRouter(db, cfg)
	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts/quality-check", strings.NewReader(`{}`)))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("no-token quality status=%d body=%s", unauthenticated.Code, unauthenticated.Body.String())
	}

	token, err := auth.GenerateAccessToken(cfg.JWTSecret, user.ID.String(), user.Email, time.Hour)
	if err != nil {
		t.Fatalf("generate viewer token: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts/quality-check", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	forbidden := httptest.NewRecorder()
	router.ServeHTTP(forbidden, request)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("viewer quality status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestContentQualityPublishGatePostgres(t *testing.T) {
	db := openHTTPAPIPostgresTestSchema(t, "quality_gate")
	migrateContentAccessIntegrationSchema(t, db)
	owner := uuid.New()
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)

	invalid := contentAccessPost(uuid.NewString(), "待完善文章", "invalid-post", "draft", &owner, now)
	invalid.ContentMD = "<!-- no visible content -->"
	invalid.Visibility = "private"
	contentAccessCreate(t, db, &invalid)

	recorder := contentAccessServe(t, db, http.MethodPost, "/posts/:id/publish", "/posts/"+invalid.ID.String()+"/publish", "", owner, []string{"author"}, []string{"post:publish"}, publishPost(db, config.Config{}))
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), `"code":"CONTENT_QUALITY_BLOCKED"`) {
		t.Fatalf("invalid publish status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var refreshed model.Post
	if err := db.First(&refreshed, "id = ?", invalid.ID).Error; err != nil {
		t.Fatalf("reload invalid post: %v", err)
	}
	if refreshed.Status != "draft" || refreshed.Visibility != "private" || refreshed.PublishedAt != nil {
		t.Fatalf("blocked publish mutated post: %#v", refreshed)
	}
	assertQualityCounts(t, db, 0, 0)

	valid := contentAccessPost(uuid.NewString(), "可发布文章", "valid-post", "draft", &owner, now.Add(time.Minute))
	valid.ContentMD = strings.Repeat("这是经过检查的工程正文。", 30)
	valid.Summary = strings.Repeat("文章摘要", 8)
	valid.SEODescription = strings.Repeat("搜索描述", 12)
	contentAccessCreate(t, db, &valid)

	var wait sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			r := contentAccessServe(t, db, http.MethodPost, "/posts/:id/publish", "/posts/"+valid.ID.String()+"/publish", "", owner, []string{"author"}, []string{"post:publish"}, publishPost(db, config.Config{}))
			codes <- r.Code
		}()
	}
	wait.Wait()
	close(codes)
	gotCodes := make([]int, 0, 2)
	for code := range codes {
		gotCodes = append(gotCodes, code)
	}
	sort.Ints(gotCodes)
	wantCodes := []int{http.StatusAccepted, http.StatusConflict}
	if len(gotCodes) != 2 || gotCodes[0] != wantCodes[0] || gotCodes[1] != wantCodes[1] {
		t.Fatalf("concurrent publish statuses = %v, want %v", gotCodes, wantCodes)
	}

	patch := contentAccessServe(t, db, http.MethodPatch, "/posts/:id", "/posts/"+valid.ID.String(), `{"title":"不应写入"}`, owner, []string{"author"}, []string{"post:update", "post:publish"}, updatePost(db))
	if patch.Code != http.StatusConflict || !strings.Contains(patch.Body.String(), "CONTENT_PUBLISH_IN_PROGRESS") {
		t.Fatalf("active-job patch status=%d body=%s", patch.Code, patch.Body.String())
	}
	var validRefreshed model.Post
	if err := db.First(&validRefreshed, "id = ?", valid.ID).Error; err != nil || validRefreshed.Title != valid.Title {
		t.Fatalf("active-job patch mutated post: title=%q err=%v", validRefreshed.Title, err)
	}
	assertQualityCounts(t, db, 1, 0)

	bypass := contentAccessServe(t, db, http.MethodPost, "/posts", "/posts", `{"title":"绕过","slug":"bypass","status":"published"}`, owner, []string{"admin"}, []string{"post:create", "post:publish"}, createPost(db))
	if bypass.Code != http.StatusUnprocessableEntity || !strings.Contains(bypass.Body.String(), "PUBLISH_ENDPOINT_REQUIRED") {
		t.Fatalf("privileged create bypass status=%d body=%s", bypass.Code, bypass.Body.String())
	}
}

func TestContentQualityPageAndSiteGatesPostgres(t *testing.T) {
	db := openHTTPAPIPostgresTestSchema(t, "quality_site")
	migrateContentAccessIntegrationSchema(t, db)
	owner := uuid.New()
	now := time.Now().UTC()
	page := contentAccessPage(uuid.NewString(), "空页面", "empty-page", &owner, now)
	page.ContentMD = ""
	contentAccessCreate(t, db, &page)

	recorder := contentAccessServe(t, db, http.MethodPost, "/pages/:id/publish", "/pages/"+page.ID.String()+"/publish", "", owner, []string{"author"}, []string{"page:publish"}, publishPage(db))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid page publish status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var refreshed model.Page
	if err := db.First(&refreshed, "id = ?", page.ID).Error; err != nil || refreshed.Status != "draft" || refreshed.PublishedAt != nil {
		t.Fatalf("blocked page publish mutated page: %#v err=%v", refreshed, err)
	}

	historical := contentAccessPost(uuid.NewString(), "历史坏文章", "historical-invalid", "published", &owner, now.Add(time.Minute))
	historical.ContentMD = ""
	contentAccessCreate(t, db, &historical)
	settingsPublish := contentAccessServe(t, db, http.MethodPost, "/settings/publish", "/settings/publish", "", owner, []string{"admin"}, []string{"setting:update"}, publishAdminSiteSettings(db))
	if settingsPublish.Code != http.StatusUnprocessableEntity || !strings.Contains(settingsPublish.Body.String(), "CONTENT_QUALITY_BLOCKED") {
		t.Fatalf("site quality gate status=%d body=%s", settingsPublish.Code, settingsPublish.Body.String())
	}
	assertQualityCounts(t, db, 0, 0)
}

func TestWorkerAndRetryRejectInvalidContentPostgres(t *testing.T) {
	db := openHTTPAPIPostgresTestSchema(t, "quality_worker")
	migrateContentAccessIntegrationSchema(t, db)
	owner := uuid.New()
	post := contentAccessPost(uuid.NewString(), "坏文章", "worker-invalid", "published", &owner, time.Now().UTC())
	post.ContentMD = ""
	contentAccessCreate(t, db, &post)
	job := model.PublishJob{PostID: &post.ID, JobType: "post", Status: "requested", TriggerSource: "test", RunAt: time.Now().Add(-time.Minute)}
	contentAccessCreate(t, db, &job)

	worker := publisher.NewWorker(db, config.Config{}, log.New(io.Discard, "", 0))
	processed, err := worker.ProcessOne(context.Background())
	if err != nil || !processed {
		t.Fatalf("worker processed=%v err=%v", processed, err)
	}
	if err := db.First(&job, "id = ?", job.ID).Error; err != nil {
		t.Fatalf("reload worker job: %v", err)
	}
	if job.Status != "failed" || job.ErrorCode != "CONTENT_QUALITY_BLOCKED" {
		t.Fatalf("worker job = status %q code %q", job.Status, job.ErrorCode)
	}

	_, err = publisher.RetryJob(context.Background(), db, config.Config{PublishMaxRetries: 3}, job.ID)
	if !errors.Is(err, publisher.ErrContentQualityBlocked) {
		t.Fatalf("retry error = %v, want content quality block", err)
	}
	var afterRetry model.PublishJob
	if err := db.First(&afterRetry, "id = ?", job.ID).Error; err != nil || afterRetry.Status != "failed" || afterRetry.RetryCount != 0 {
		t.Fatalf("blocked retry mutated job: %#v err=%v", afterRetry, err)
	}
}

func assertQualityCounts(t *testing.T, db interface {
	Model(value interface{}) *gorm.DB
}, wantJobs, wantUsages int64) {
	t.Helper()
	var jobs, usages int64
	if err := db.Model(&model.PublishJob{}).Count(&jobs).Error; err != nil {
		t.Fatalf("count publish jobs: %v", err)
	}
	if err := db.Model(&model.MediaUsage{}).Count(&usages).Error; err != nil {
		t.Fatalf("count media usages: %v", err)
	}
	if jobs != wantJobs || usages != wantUsages {
		t.Fatalf("jobs/usages = %d/%d, want %d/%d", jobs, usages, wantJobs, wantUsages)
	}
}
