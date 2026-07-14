package publisher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const publisherFakeHugoModeEnv = "PUBLISHER_TEST_FAKE_HUGO_MODE"

func TestMain(m *testing.M) {
	if os.Getenv(publisherFakeHugoModeEnv) == "invalid-public-url" {
		os.Exit(runPublisherFakeHugo(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func TestProcessSiteJobRejectsInvalidProductionOutputWithoutReleaseOrCurrentSwap(t *testing.T) {
	db := openPublisherJobTestDB(t)
	seedPublisherPublicSettings(t, db)

	root := t.TempDir()
	releaseRoot := filepath.Join(root, "releases")
	currentPath := filepath.Join(root, "current")
	previousOutput := filepath.Join(releaseRoot, "rel_previous")
	writeValidSiteRelease(t, previousOutput, "rel_previous", uuid.New())
	writeTestFile(t, filepath.Join(currentPath, "index.html"), "previous current artifact")

	previousJob := createPublisherJob(t, db, "published")
	previousRelease := createPublisherRelease(t, db, previousJob.ID, "rel_previous", previousOutput, true)
	failingJob := createPublisherJob(t, db, "queued")

	siteDir := filepath.Join(root, "site")
	writePublisherTestSiteConfig(t, siteDir)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	t.Setenv(publisherFakeHugoModeEnv, "invalid-public-url")

	cfg := publisherProductionTestConfig(siteDir, executable, releaseRoot, currentPath)
	_, err = ProcessSiteJob(context.Background(), db, cfg, failingJob)
	if err == nil || !strings.Contains(err.Error(), "non-public URL") {
		t.Fatalf("expected generated public URL validation failure, got %v", err)
	}

	var refreshedJob model.PublishJob
	if err := db.First(&refreshedJob, "id = ?", failingJob.ID).Error; err != nil {
		t.Fatalf("reload failed job: %v", err)
	}
	if refreshedJob.Status != "failed" || refreshedJob.ErrorCode != "PUBLIC_URL_OUTPUT_INVALID" {
		t.Fatalf("failed job state = status %q code %q", refreshedJob.Status, refreshedJob.ErrorCode)
	}
	if refreshedJob.OutputPath == "" {
		t.Fatal("failed job did not retain its attempted output path")
	}
	if _, err := os.Stat(refreshedJob.OutputPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid release output remains usable: %v", err)
	}

	var failedJobReleaseCount int64
	if err := db.Model(&model.PublishRelease{}).Where("job_id = ?", failingJob.ID).Count(&failedJobReleaseCount).Error; err != nil {
		t.Fatalf("count failed job releases: %v", err)
	}
	if failedJobReleaseCount != 0 {
		t.Fatalf("invalid output created %d release rows", failedJobReleaseCount)
	}
	assertPublisherActiveRelease(t, db, previousRelease.ID)
	if got := readTestFile(t, filepath.Join(currentPath, "index.html")); got != "previous current artifact" {
		t.Fatalf("current artifact changed: %q", got)
	}
}

func TestPromoteReleaseRevalidatesHistoricalOutputBeforeActiveOrCurrentSwap(t *testing.T) {
	db := openPublisherJobTestDB(t)
	seedPublisherPublicSettings(t, db)

	root := t.TempDir()
	releaseRoot := filepath.Join(root, "releases")
	currentPath := filepath.Join(root, "current")
	previousOutput := filepath.Join(releaseRoot, "rel_previous")
	candidateOutput := filepath.Join(releaseRoot, "rel_candidate")
	writeValidSiteRelease(t, previousOutput, "rel_previous", uuid.New())
	writeInvalidPublicURLSiteRelease(t, candidateOutput, "rel_candidate", uuid.New())
	writeTestFile(t, filepath.Join(currentPath, "index.html"), "previous current artifact")

	previousJob := createPublisherJob(t, db, "published")
	previousRelease := createPublisherRelease(t, db, previousJob.ID, "rel_previous", previousOutput, true)
	candidateJob := createPublisherJob(t, db, "published")
	candidateRelease := createPublisherRelease(t, db, candidateJob.ID, "rel_candidate", candidateOutput, false)

	cfg := publisherProductionTestConfig("", "", releaseRoot, currentPath)
	_, err := PromoteRelease(context.Background(), db, cfg, candidateRelease.ID)
	if err == nil || !strings.Contains(err.Error(), "non-public URL") {
		t.Fatalf("expected historical output public URL validation failure, got %v", err)
	}

	assertPublisherActiveRelease(t, db, previousRelease.ID)
	var refreshedCandidate model.PublishRelease
	if err := db.First(&refreshedCandidate, "id = ?", candidateRelease.ID).Error; err != nil {
		t.Fatalf("reload candidate release: %v", err)
	}
	if refreshedCandidate.IsActive || refreshedCandidate.Status != "inactive" || refreshedCandidate.PromotedAt != nil {
		t.Fatalf("candidate release was mutated: active=%v status=%q promoted_at=%v", refreshedCandidate.IsActive, refreshedCandidate.Status, refreshedCandidate.PromotedAt)
	}
	if got := readTestFile(t, filepath.Join(currentPath, "index.html")); got != "previous current artifact" {
		t.Fatalf("current artifact changed: %q", got)
	}
	stagedPath := filepath.Join(filepath.Dir(currentPath), filepath.Base(currentPath)+".next-"+candidateRelease.ReleaseKey)
	if _, err := os.Stat(stagedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("historical output was staged before validation completed: %v", err)
	}
	if got := readTestFile(t, filepath.Join(candidateOutput, "index.html")); !strings.Contains(got, "127.0.0.1") {
		t.Fatalf("candidate output was unexpectedly changed: %q", got)
	}
}

func TestRetryJobRestoresContentToPublishedBeforeRequeue(t *testing.T) {
	for _, initialStatus := range []string{"draft", "publishing"} {
		t.Run(initialStatus, func(t *testing.T) {
			db := openPublisherJobTestDB(t)
			page := createPublisherPage(t, db, initialStatus)
			job := createPublisherContentJob(t, db, "page", "failed", nil, &page.ID)

			retried, err := RetryJob(context.Background(), db, config.Config{PublishMaxRetries: 3}, job.ID)
			if err != nil {
				t.Fatalf("retry publish job: %v", err)
			}
			if retried.Status != "requested" || retried.RetryCount != 1 {
				t.Fatalf("retried job status/count = %q/%d", retried.Status, retried.RetryCount)
			}
			var refreshedPage model.Page
			if err := db.First(&refreshedPage, "id = ?", page.ID).Error; err != nil {
				t.Fatalf("reload page: %v", err)
			}
			if refreshedPage.Status != "published" {
				t.Fatalf("page status = %q, want published", refreshedPage.Status)
			}
		})
	}
}

func TestRecoverStaleJobsRestoresPublishingContent(t *testing.T) {
	db := openPublisherJobTestDB(t)
	page := createPublisherPage(t, db, "publishing")
	job := createPublisherContentJob(t, db, "page", "building", nil, &page.ID)
	if err := db.Model(&model.PublishJob{}).Where("id = ?", job.ID).
		Update("updated_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("age publish job: %v", err)
	}

	if err := RecoverStaleJobs(context.Background(), db, config.Config{}); err != nil {
		t.Fatalf("recover stale jobs: %v", err)
	}
	var refreshedPage model.Page
	if err := db.First(&refreshedPage, "id = ?", page.ID).Error; err != nil {
		t.Fatalf("reload page: %v", err)
	}
	if refreshedPage.Status != "published" {
		t.Fatalf("page status = %q, want published", refreshedPage.Status)
	}
	var refreshedJob model.PublishJob
	if err := db.First(&refreshedJob, "id = ?", job.ID).Error; err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if refreshedJob.Status != "failed" || refreshedJob.ErrorCode != "WORKER_INTERRUPTED" {
		t.Fatalf("job status/code = %q/%q", refreshedJob.Status, refreshedJob.ErrorCode)
	}
}

func TestRecoverStaleJobsRepairsTerminalJobPublishingContent(t *testing.T) {
	db := openPublisherJobTestDB(t)
	page := createPublisherPage(t, db, "publishing")
	job := createPublisherContentJob(t, db, "page", "failed", nil, &page.ID)

	if err := RecoverStaleJobs(context.Background(), db, config.Config{}); err != nil {
		t.Fatalf("recover terminal publication state: %v", err)
	}
	var refreshedPage model.Page
	if err := db.First(&refreshedPage, "id = ?", page.ID).Error; err != nil {
		t.Fatalf("reload page: %v", err)
	}
	if refreshedPage.Status != "published" {
		t.Fatalf("page status = %q, want published", refreshedPage.Status)
	}
	var refreshedJob model.PublishJob
	if err := db.First(&refreshedJob, "id = ?", job.ID).Error; err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if refreshedJob.Status != "failed" {
		t.Fatalf("terminal job status changed to %q", refreshedJob.Status)
	}
}

func TestMarkJobFailedDoesNotOverwriteConfirmedTerminalOrRetriedState(t *testing.T) {
	for _, status := range []string{"canceled", "requested", "published"} {
		t.Run(status, func(t *testing.T) {
			db := openPublisherJobTestDB(t)
			job := createPublisherJob(t, db, status)
			markJobFailed(context.Background(), db, &job, "LATE_FAILURE", errors.New("late worker failure"))

			var refreshed model.PublishJob
			if err := db.First(&refreshed, "id = ?", job.ID).Error; err != nil {
				t.Fatalf("reload job: %v", err)
			}
			if refreshed.Status != status || refreshed.ErrorCode != "" {
				t.Fatalf("job status/code = %q/%q, want %q/empty", refreshed.Status, refreshed.ErrorCode, status)
			}
		})
	}
}

func TestReconcileCurrentReleaseRepairsCurrentFromDatabaseActiveRelease(t *testing.T) {
	db := openPublisherJobTestDB(t)
	root := t.TempDir()
	releaseRoot := filepath.Join(root, "releases")
	currentPath := filepath.Join(root, "current")
	outputPath := filepath.Join(releaseRoot, "rel_active")
	job := createPublisherJob(t, db, "published")
	writeValidSiteRelease(t, outputPath, "rel_active", job.ID)
	writeTestFile(t, filepath.Join(currentPath, "manifest.json"), `{"release_key":"rel_stale"}`)
	writeTestFile(t, filepath.Join(currentPath, "index.html"), "stale current")
	createPublisherRelease(t, db, job.ID, "rel_active", outputPath, true)

	cfg := publisherProductionTestConfig("", "", releaseRoot, currentPath)
	if err := ReconcileCurrentRelease(context.Background(), db, cfg); err != nil {
		t.Fatalf("reconcile current release: %v", err)
	}
	wantManifest := readTestFile(t, filepath.Join(outputPath, "manifest.json"))
	gotManifest := readTestFile(t, filepath.Join(currentPath, "manifest.json"))
	if gotManifest != wantManifest {
		t.Fatalf("current manifest was not repaired: got %q want %q", gotManifest, wantManifest)
	}
}

func TestCancelJobCannotOverwritePublishedTerminalState(t *testing.T) {
	db := openPublisherJobTestDB(t)
	job := createPublisherJob(t, db, "queued")

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin lock transaction: %v", tx.Error)
	}
	var locked model.PublishJob
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", job.ID).Error; err != nil {
		_ = tx.Rollback().Error
		t.Fatalf("lock publish job: %v", err)
	}

	type cancelResult struct {
		job model.PublishJob
		err error
	}
	resultCh := make(chan cancelResult, 1)
	go func() {
		canceled, err := CancelJob(context.Background(), db, job.ID)
		resultCh <- cancelResult{job: canceled, err: err}
	}()

	select {
	case result := <-resultCh:
		_ = tx.Rollback().Error
		t.Fatalf("cancel did not wait for the locked job row: job=%+v err=%v", result.job, result.err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := tx.Model(&model.PublishJob{}).Where("id = ?", job.ID).Update("status", "published").Error; err != nil {
		_ = tx.Rollback().Error
		t.Fatalf("publish locked job: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit published terminal state: %v", err)
	}

	select {
	case result := <-resultCh:
		if result.err == nil || !strings.Contains(result.err.Error(), "not cancelable") {
			t.Fatalf("cancel result = job=%+v err=%v", result.job, result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel remained blocked after terminal state committed")
	}
	var refreshed model.PublishJob
	if err := db.First(&refreshed, "id = ?", job.ID).Error; err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if refreshed.Status != "published" {
		t.Fatalf("job status = %q, want published", refreshed.Status)
	}
}

func TestPromoteReleaseLocksBeforeReadingOrStagingCandidate(t *testing.T) {
	db := openPublisherJobTestDB(t)
	seedPublisherPublicSettings(t, db)

	root := t.TempDir()
	releaseRoot := filepath.Join(root, "releases")
	currentPath := filepath.Join(root, "current")
	previousOutput := filepath.Join(releaseRoot, "rel_previous")
	candidateOutput := filepath.Join(releaseRoot, "rel_candidate")
	writeValidSiteRelease(t, previousOutput, "rel_previous", uuid.New())
	writeValidSiteRelease(t, candidateOutput, "rel_candidate", uuid.New())
	writeTestFile(t, filepath.Join(currentPath, "index.html"), "previous current artifact")

	previousJob := createPublisherJob(t, db, "published")
	previousRelease := createPublisherRelease(t, db, previousJob.ID, "rel_previous", previousOutput, true)
	candidateJob := createPublisherJob(t, db, "published")
	candidateRelease := createPublisherRelease(t, db, candidateJob.ID, "rel_candidate", candidateOutput, false)

	cleanupTx := db.Begin()
	if cleanupTx.Error != nil {
		t.Fatalf("begin cleanup transaction: %v", cleanupTx.Error)
	}
	if err := acquirePromotionLock(cleanupTx); err != nil {
		_ = cleanupTx.Rollback().Error
		t.Fatalf("acquire cleanup promotion lock: %v", err)
	}

	type promoteResult struct {
		release model.PublishRelease
		err     error
	}
	resultCh := make(chan promoteResult, 1)
	cfg := publisherProductionTestConfig("", "", releaseRoot, currentPath)
	go func() {
		release, err := PromoteRelease(context.Background(), db, cfg, candidateRelease.ID)
		resultCh <- promoteResult{release: release, err: err}
	}()

	stagedPath := filepath.Join(filepath.Dir(currentPath), filepath.Base(currentPath)+".next-"+candidateRelease.ReleaseKey)
	select {
	case result := <-resultCh:
		_ = cleanupTx.Rollback().Error
		t.Fatalf("promotion did not wait for cleanup lock: release=%+v err=%v", result.release, result.err)
	case <-time.After(200 * time.Millisecond):
	}
	if _, err := os.Stat(stagedPath); !errors.Is(err, os.ErrNotExist) {
		_ = cleanupTx.Rollback().Error
		t.Fatalf("candidate was staged before advisory lock acquisition: %v", err)
	}
	if err := cleanupTx.Delete(&model.PublishRelease{}, "id = ?", candidateRelease.ID).Error; err != nil {
		_ = cleanupTx.Rollback().Error
		t.Fatalf("soft delete candidate under cleanup lock: %v", err)
	}
	if err := cleanupTx.Commit().Error; err != nil {
		t.Fatalf("commit cleanup transaction: %v", err)
	}

	select {
	case result := <-resultCh:
		if !errors.Is(result.err, gorm.ErrRecordNotFound) {
			t.Fatalf("promotion result = release=%+v err=%v, want record not found", result.release, result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("promotion remained blocked after cleanup committed")
	}
	assertPublisherActiveRelease(t, db, previousRelease.ID)
	if got := readTestFile(t, filepath.Join(currentPath, "index.html")); got != "previous current artifact" {
		t.Fatalf("current artifact changed: %q", got)
	}
}

func runPublisherFakeHugo(args []string) int {
	destination := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--destination" {
			destination = args[i+1]
			break
		}
	}
	if destination == "" {
		return 2
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return 3
	}
	content := `<html><head><link rel="canonical" href="https://blog.zoking.tech/"></head><body><a href="http://127.0.0.1/private">private</a></body></html>`
	if err := os.WriteFile(filepath.Join(destination, "index.html"), []byte(content), 0o644); err != nil {
		return 4
	}
	return 0
}

func openPublisherJobTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for PostgreSQL publisher job integration tests")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	databaseName := strings.Trim(parsed.Path, "/")
	if !strings.HasSuffix(strings.ToLower(databaseName), "_test") {
		t.Skipf("publisher job integration tests require a *_test database, got %q", databaseName)
	}

	adminDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	adminDB.SetMaxOpenConns(1)
	if err := adminDB.PingContext(context.Background()); err != nil {
		_ = adminDB.Close()
		t.Fatalf("ping PostgreSQL test database: %v", err)
	}
	schema := "publisher_job_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminDB.ExecContext(context.Background(), `create schema "`+schema+`"`); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create PostgreSQL test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), `drop schema if exists "`+schema+`" cascade`)
		_ = adminDB.Close()
	})

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open isolated publisher job database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve isolated publisher job database pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	statements := []string{
		`create table posts (
			id uuid primary key,
			title text not null default '',
			slug text not null default '',
			summary text not null default '',
			content_md text not null default '',
			status text not null default 'draft' check (status in ('draft', 'offline', 'archived', 'published', 'publishing')),
			visibility text not null default 'public',
			allow_comment boolean not null default false,
			published_at timestamptz null,
			author_id uuid null,
			cover_media_id uuid null,
			series_id uuid null,
			series_order integer null,
			seo_title text not null default '',
			seo_description text not null default '',
			created_at timestamptz not null,
			updated_at timestamptz not null,
			deleted_at timestamptz null
		)`,
		`create table pages (
			id uuid primary key,
			title text not null default '',
			slug text not null default '',
			summary text not null default '',
			content_md text not null default '',
			status text not null default 'draft',
			visibility text not null default 'public',
			show_in_menu boolean not null default false,
			menu_weight integer not null default 0,
			menu_icon text not null default '',
			allow_comment boolean not null default false,
			published_at timestamptz null,
			author_id uuid null,
			seo_title text not null default '',
			seo_description text not null default '',
			created_at timestamptz not null,
			updated_at timestamptz not null,
			deleted_at timestamptz null
		)`,
		`create table achievements (
			id uuid primary key,
			kind text not null default 'award',
			title text not null default '',
			organization text not null default '',
			summary text not null default '',
			occurred_at date not null,
			ended_at date null,
			external_url text not null default '',
			credential_id text not null default '',
			image_media_id uuid null,
			sort_order integer not null default 0,
			status text not null default 'draft',
			created_at timestamptz not null,
			updated_at timestamptz not null,
			deleted_at timestamptz null
		)`,
		`create table site_settings (
			id uuid primary key,
			key text not null unique,
			value_json jsonb not null,
			description text not null default '',
			is_public boolean not null default false,
			created_at timestamptz not null,
			updated_at timestamptz not null
		)`,
		`create table publish_jobs (
			id uuid primary key,
			post_id uuid null,
			page_id uuid null,
			job_type text not null default 'site',
			status text not null default 'requested',
			trigger_source text not null default 'test',
			requested_by uuid null,
			run_at timestamptz not null,
			started_at timestamptz null,
			finished_at timestamptz null,
			snapshot_key text not null default '',
			settings_hash text not null default '',
			release_key text not null default '',
			content_path text not null default '',
			output_path text not null default '',
			manifest_json jsonb null,
			log_json jsonb null,
			error_code text not null default '',
			error_message text not null default '',
			retry_count integer not null default 0,
			canceled_at timestamptz null,
			created_at timestamptz not null,
			updated_at timestamptz not null,
			deleted_at timestamptz null
		)`,
		`create table publish_releases (
			id uuid primary key,
			job_id uuid not null references publish_jobs(id) on delete cascade,
			release_key text not null unique,
			status text not null default 'active',
			post_id uuid null,
			page_id uuid null,
			output_path text not null,
			manifest_json jsonb not null,
			is_active boolean not null default true,
			promoted_at timestamptz null,
			created_at timestamptz not null,
			updated_at timestamptz not null,
			deleted_at timestamptz null
		)`,
		`create unique index publish_releases_active_unique_idx on publish_releases(is_active) where is_active = true and deleted_at is null`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create publisher job test table: %v", err)
		}
	}
	return db
}

func seedPublisherPublicSettings(t *testing.T, db *gorm.DB) {
	t.Helper()
	settings := map[string]interface{}{
		"site.base_url":     "https://blog.zoking.tech/",
		"comments.enabled":  true,
		"comments.api_base": "https://api.zoking.tech",
	}
	for key, value := range settings {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal setting %s: %v", key, err)
		}
		row := model.SiteSetting{
			ID:        uuid.New(),
			Key:       key,
			ValueJSON: raw,
			IsPublic:  true,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create setting %s: %v", key, err)
		}
	}
}

func createPublisherJob(t *testing.T, db *gorm.DB, status string) model.PublishJob {
	t.Helper()
	job := model.PublishJob{
		Base: model.Base{
			ID: uuid.New(),
		},
		JobType:       "site",
		Status:        status,
		TriggerSource: "test",
		RunAt:         time.Now(),
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("create publisher job: %v", err)
	}
	return job
}

func createPublisherContentJob(t *testing.T, db *gorm.DB, jobType string, status string, postID *uuid.UUID, pageID *uuid.UUID) model.PublishJob {
	t.Helper()
	job := model.PublishJob{
		Base:          model.Base{ID: uuid.New()},
		PostID:        postID,
		PageID:        pageID,
		JobType:       jobType,
		Status:        status,
		TriggerSource: "test",
		RunAt:         time.Now(),
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("create publisher content job: %v", err)
	}
	return job
}

func createPublisherPage(t *testing.T, db *gorm.DB, status string) model.Page {
	t.Helper()
	page := model.Page{
		Base:       model.Base{ID: uuid.New()},
		Title:      "Publisher state machine page",
		Slug:       "publisher-state-machine-" + uuid.NewString(),
		ContentMD:  "This page has valid publication content.",
		Status:     status,
		Visibility: "public",
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatalf("create publisher page: %v", err)
	}
	return page
}

func createPublisherRelease(t *testing.T, db *gorm.DB, jobID uuid.UUID, releaseKey string, outputPath string, active bool) model.PublishRelease {
	t.Helper()
	manifest := PublishManifest{
		JobID:       jobID.String(),
		Scope:       "site",
		ReleaseKey:  releaseKey,
		OutputPath:  outputPath,
		CurrentPath: filepath.Join(filepath.Dir(outputPath), "current"),
		CreatedAt:   time.Now(),
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal release manifest: %v", err)
	}
	status := "inactive"
	var promotedAt *time.Time
	if active {
		status = "active"
		now := time.Now()
		promotedAt = &now
	}
	release := model.PublishRelease{
		Base: model.Base{
			ID: uuid.New(),
		},
		JobID:        jobID,
		ReleaseKey:   releaseKey,
		Status:       status,
		OutputPath:   outputPath,
		ManifestJSON: manifestJSON,
		IsActive:     active,
		PromotedAt:   promotedAt,
	}
	if err := db.Create(&release).Error; err != nil {
		t.Fatalf("create publisher release: %v", err)
	}
	return release
}

func assertPublisherActiveRelease(t *testing.T, db *gorm.DB, expectedID uuid.UUID) {
	t.Helper()
	var releases []model.PublishRelease
	if err := db.Where("is_active = true").Find(&releases).Error; err != nil {
		t.Fatalf("load active releases: %v", err)
	}
	if len(releases) != 1 || releases[0].ID != expectedID || releases[0].Status != "active" {
		t.Fatalf("active releases = %+v, expected only %s", releases, expectedID)
	}
}

func publisherProductionTestConfig(siteDir string, hugoBin string, releaseRoot string, currentPath string) config.Config {
	return config.Config{
		AppEnv:             "production",
		SiteBaseURL:        "https://blog.zoking.tech/",
		PublicAPIBaseURL:   "https://api.zoking.tech",
		MediaPublicBaseURL: "/media-files",
		HugoSiteDir:        siteDir,
		HugoBin:            hugoBin,
		PublishReleaseRoot: releaseRoot,
		PublishCurrentDir:  currentPath,
		PublishJobTimeout:  10 * time.Second,
	}
}

func writePublisherTestSiteConfig(t *testing.T, siteDir string) {
	t.Helper()
	configDir := filepath.Join(siteDir, "config", "_default")
	writeTestFile(t, filepath.Join(configDir, "hugo.toml"), `baseURL = "http://localhost:1313/"
title = "Test"
defaultContentLanguage = "en"

[pagination]
pagerSize = 3
`)
	writeTestFile(t, filepath.Join(configDir, "params.toml"), `[sidebar]
emoji = "T"
subtitle = "Test"

[footer]
since = 2020

[comments]
enabled = true

[comments.public]
apiBase = "http://localhost:18080"
`)
	writeTestFile(t, filepath.Join(configDir, "languages.toml"), `[en]
title = "Test"

[en.params.sidebar]
subtitle = "Test"
`)
}

func writeValidSiteRelease(t *testing.T, outputPath string, releaseKey string, jobID uuid.UUID) {
	t.Helper()
	writeSiteRelease(t, outputPath, releaseKey, jobID, `<html><head><link rel="canonical" href="https://blog.zoking.tech/"></head><body>valid</body></html>`)
}

func writeInvalidPublicURLSiteRelease(t *testing.T, outputPath string, releaseKey string, jobID uuid.UUID) {
	t.Helper()
	writeSiteRelease(t, outputPath, releaseKey, jobID, `<html><head><link rel="canonical" href="https://blog.zoking.tech/"></head><body><a href="http://127.0.0.1/private">private</a></body></html>`)
}

func writeSiteRelease(t *testing.T, outputPath string, releaseKey string, jobID uuid.UUID, indexHTML string) {
	t.Helper()
	manifest := PublishManifest{
		JobID:      jobID.String(),
		Scope:      "site",
		ReleaseKey: releaseKey,
		OutputPath: outputPath,
		CreatedAt:  time.Now(),
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal release file manifest: %v", err)
	}
	writeTestFile(t, filepath.Join(outputPath, "manifest.json"), string(manifestJSON))
	writeTestFile(t, filepath.Join(outputPath, "index.html"), indexHTML)
	writeTestFile(t, filepath.Join(outputPath, "index.xml"), `<?xml version="1.0"?><rss></rss>`)
	writeTestFile(t, filepath.Join(outputPath, "sitemap.xml"), `<?xml version="1.0"?><urlset></urlset>`)
}
