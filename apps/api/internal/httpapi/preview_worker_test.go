package httpapi

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPreviewTerminalUpdatesRequireBuildingRow(t *testing.T) {
	updatePattern := `UPDATE "publish_previews" SET .* WHERE .*preview_key = \$[0-9]+ and status = \$[0-9]+.*`

	t.Run("finish", func(t *testing.T) {
		db, mock := newPreviewMockDB(t)
		mock.ExpectExec(updatePattern).WillReturnResult(sqlmock.NewResult(0, 0))

		err := finishPreviewRecord(newPreviewContext(), db, "prev-finished", publisher.PreviewResult{})
		if !errors.Is(err, errPreviewTerminalTransition) {
			t.Fatalf("finishPreviewRecord error = %v", err)
		}
		assertPreviewMockExpectations(t, mock)
	})

	t.Run("fail", func(t *testing.T) {
		db, mock := newPreviewMockDB(t)
		mock.ExpectExec(updatePattern).WillReturnResult(sqlmock.NewResult(0, 0))

		err := failPreviewRecord(newPreviewContext(), db, "prev-finished", "PREVIEW_BUILD_FAILED", errors.New("build failed"))
		if !errors.Is(err, errPreviewTerminalTransition) {
			t.Fatalf("failPreviewRecord error = %v", err)
		}
		assertPreviewMockExpectations(t, mock)
	})
}

func TestPreviewFinishFailRaceHasSingleWinner(t *testing.T) {
	db, mock := newPreviewMockDB(t)
	mock.MatchExpectationsInOrder(false)
	updatePattern := `UPDATE "publish_previews" SET .* WHERE .*preview_key = \$[0-9]+ and status = \$[0-9]+.*`
	mock.ExpectExec(updatePattern).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(updatePattern).WillReturnResult(sqlmock.NewResult(0, 0))

	start := make(chan struct{})
	results := make(chan error, 2)
	finishContext := newPreviewContext()
	failContext := newPreviewContext()
	go func() {
		<-start
		results <- finishPreviewRecord(finishContext, db, "prev-race", publisher.PreviewResult{})
	}()
	go func() {
		<-start
		results <- failPreviewRecord(failContext, db, "prev-race", "PREVIEW_BUILD_FAILED", errors.New("build failed"))
	}()
	close(start)

	winners := 0
	losers := 0
	for range 2 {
		err := <-results
		if err == nil {
			winners++
		} else if errors.Is(err, errPreviewTerminalTransition) {
			losers++
		} else {
			t.Fatalf("unexpected terminal update error: %v", err)
		}
	}
	if winners != 1 || losers != 1 {
		t.Fatalf("terminal update results: winners=%d losers=%d, want 1 each", winners, losers)
	}
	assertPreviewMockExpectations(t, mock)
}

func TestPreviewFinishFailRacePostgresIntegration(t *testing.T) {
	adminDB, parsedDatabaseURL := openPreviewPostgresTestAdmin(t)
	const rounds = 12

	for round := 1; round <= rounds; round++ {
		t.Run(fmt.Sprintf("round-%02d", round), func(t *testing.T) {
			db := openPreviewPostgresRaceSchema(t, adminDB, parsedDatabaseURL)
			previewKey := fmt.Sprintf("prev-postgres-race-%02d", round)
			initialEntryPath := "/p/initial/"
			initialURL := "https://preview.example.com/initial/"
			initialTargetURL := initialURL + "p/initial/"
			if err := db.Exec(`insert into publish_previews
				(preview_key, status, entry_path, url, target_url)
				values (?, 'building', ?, ?, ?)`, previewKey, initialEntryPath, initialURL, initialTargetURL).Error; err != nil {
				t.Fatalf("seed building preview: %v", err)
			}

			result := publisher.PreviewResult{
				OutputPath: "/tmp/preview-ready",
				TargetPath: "/p/ready/",
				URL:        "https://preview.example.com/ready/",
				TargetURL:  "https://preview.example.com/ready/p/ready/",
				Manifest: publisher.PublishManifest{
					SettingsHash: fmt.Sprintf("settings-%02d", round),
					ContentHash:  fmt.Sprintf("content-%02d", round),
				},
			}
			buildErr := errors.New("postgres integration build failed")
			finishContext := newPreviewContext()
			failContext := newPreviewContext()

			start := make(chan struct{})
			var ready sync.WaitGroup
			var complete sync.WaitGroup
			ready.Add(2)
			complete.Add(2)
			var finishErr error
			var failErr error
			go func() {
				defer complete.Done()
				ready.Done()
				<-start
				finishErr = finishPreviewRecord(finishContext, db, previewKey, result)
			}()
			go func() {
				defer complete.Done()
				ready.Done()
				<-start
				failErr = failPreviewRecord(failContext, db, previewKey, "PREVIEW_BUILD_FAILED", buildErr)
			}()
			ready.Wait()
			close(start)
			complete.Wait()

			assertPreviewRaceErrors(t, finishErr, failErr)
			state := loadPreviewTerminalState(t, db, previewKey)
			switch state.Status {
			case "ready":
				if finishErr != nil || !errors.Is(failErr, errPreviewTerminalTransition) {
					t.Fatalf("ready state has inconsistent errors: finish=%v fail=%v", finishErr, failErr)
				}
				if state.OutputPath != result.OutputPath || state.EntryPath != result.TargetPath || state.URL != result.URL || state.TargetURL != result.TargetURL {
					t.Fatalf("ready URL/output fields are inconsistent: %+v", state)
				}
				if state.SettingsHash != result.Manifest.SettingsHash || state.ContentHash != result.Manifest.ContentHash {
					t.Fatalf("ready hashes are inconsistent: %+v", state)
				}
				if state.ErrorCode != "" || state.ErrorMessage != "" {
					t.Fatalf("ready state contains failure fields: %+v", state)
				}
				assertPreviewJSONEqual(t, state.ManifestJSON, result.Manifest)
				assertPreviewJSONEqual(t, state.LogJSON, []map[string]string{{
					"stage": "preview", "level": "info", "message": "preview build ready",
				}})
			case "failed":
				if failErr != nil || !errors.Is(finishErr, errPreviewTerminalTransition) {
					t.Fatalf("failed state has inconsistent errors: finish=%v fail=%v", finishErr, failErr)
				}
				if state.OutputPath != "" || state.SettingsHash != "" || state.ContentHash != "" {
					t.Fatalf("failed state contains ready output fields: %+v", state)
				}
				if state.EntryPath != initialEntryPath || state.URL != initialURL || state.TargetURL != initialTargetURL {
					t.Fatalf("failed state changed building URL fields: %+v", state)
				}
				if state.ErrorCode != "PREVIEW_BUILD_FAILED" || state.ErrorMessage != buildErr.Error() {
					t.Fatalf("failed error fields are inconsistent: %+v", state)
				}
				assertPreviewJSONEqual(t, state.ManifestJSON, map[string]interface{}{})
				assertPreviewJSONEqual(t, state.LogJSON, []map[string]string{{
					"stage": "preview", "level": "error", "message": "preview build failed",
				}})
			default:
				t.Fatalf("terminal status = %q, want ready or failed", state.Status)
			}
			if !state.FinishedAt.Valid {
				t.Fatal("terminal preview has null finished_at")
			}
		})
	}
}

func TestPreviewBaseURLNormalizesRelativePublicBase(t *testing.T) {
	c := newPreviewContext()
	c.Request.Host = "admin.internal:18080"
	c.Request.RemoteAddr = "203.0.113.9:4321"

	got := previewBaseURL(c, config.Config{PublishPreviewPublicBaseURL: " preview-files//nested/../ "}, "prev-key")
	want := "http://admin.internal:18080/preview-files/prev-key/"
	if got != want {
		t.Fatalf("previewBaseURL() = %q, want %q", got, want)
	}
}

func TestRequestOriginTrustsOnlyImmediateConfiguredProxy(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		trustedProxies string
		tls            bool
		want           string
	}{
		{
			name:           "trusted CIDR",
			remoteAddr:     "10.20.30.40:54321",
			trustedProxies: "10.0.0.0/8",
			want:           "https://preview.example.com",
		},
		{
			name:           "trusted exact IPv6",
			remoteAddr:     "[2001:db8::10]:54321",
			trustedProxies: "2001:db8::10",
			want:           "https://preview.example.com",
		},
		{
			name:           "untrusted peer uses request TLS and host",
			remoteAddr:     "203.0.113.9:54321",
			trustedProxies: "10.0.0.0/8",
			tls:            true,
			want:           "https://admin.internal:18080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newPreviewContext()
			c.Request.Host = "admin.internal:18080"
			c.Request.RemoteAddr = tt.remoteAddr
			c.Request.Header.Set("X-Forwarded-Proto", "https")
			c.Request.Header.Set("X-Forwarded-Host", "preview.example.com")
			if tt.tls {
				c.Request.TLS = &tls.ConnectionState{}
			}

			got := requestOrigin(c, config.Config{TrustedProxies: tt.trustedProxies})
			if got != tt.want {
				t.Fatalf("requestOrigin() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreviewPostLookupErrorMapping(t *testing.T) {
	testPreviewLookupErrorMapping(t, "posts", "POST_NOT_FOUND", previewPost)
}

func TestPreviewPageLookupErrorMapping(t *testing.T) {
	testPreviewLookupErrorMapping(t, "pages", "PAGE_NOT_FOUND", previewPage)
}

func testPreviewLookupErrorMapping(t *testing.T, table string, notFoundCode string, handler func(*gorm.DB, config.Config) gin.HandlerFunc) {
	t.Helper()
	tests := []struct {
		name       string
		id         string
		queryError error
		wantStatus int
		wantCode   string
	}{
		{name: "malformed UUID", id: "not-a-uuid", wantStatus: http.StatusUnprocessableEntity, wantCode: "VALIDATION_FAILED"},
		{name: "missing record", id: uuid.NewString(), wantStatus: http.StatusNotFound, wantCode: notFoundCode},
		{name: "database failure", id: uuid.NewString(), queryError: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var db *gorm.DB
			var mock sqlmock.Sqlmock
			if tt.wantStatus != http.StatusUnprocessableEntity {
				db, mock = newPreviewMockDB(t)
				expectation := mock.ExpectQuery(`SELECT .*FROM "` + table + `".*`)
				if tt.queryError != nil {
					expectation.WillReturnError(tt.queryError)
				} else {
					expectation.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				}
			}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/preview", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.id}}
			c.Set("user_id", uuid.NewString())
			c.Set("permissions", []string{"content:manage_all"})
			handler(db, config.Config{})(c)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if tt.wantCode != "" && !strings.Contains(recorder.Body.String(), `"code":"`+tt.wantCode+`"`) {
				t.Fatalf("body = %s, want error code %s", recorder.Body.String(), tt.wantCode)
			}
			if mock != nil {
				assertPreviewMockExpectations(t, mock)
			}
		})
	}
}

func newPreviewMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, WithoutReturning: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open mock GORM database: %v", err)
	}
	return db, mock
}

func newPreviewContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/preview", nil)
	return c
}

func assertPreviewMockExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL mock expectations: %v", err)
	}
}

type previewTerminalState struct {
	Status       string
	OutputPath   string
	EntryPath    string
	URL          string
	TargetURL    string
	SettingsHash string
	ContentHash  string
	ManifestJSON []byte
	LogJSON      []byte
	ErrorCode    string
	ErrorMessage string
	FinishedAt   sql.NullTime
}

func openPreviewPostgresTestAdmin(t *testing.T) (*sql.DB, *url.URL) {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for PostgreSQL preview integration tests")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		t.Skipf("preview integration tests require a PostgreSQL DATABASE_URL, got scheme %q", parsed.Scheme)
	}
	databaseName := strings.Trim(parsed.Path, "/")
	if !strings.HasSuffix(strings.ToLower(databaseName), "_test") {
		t.Skipf("preview integration tests require a *_test database, got %q", databaseName)
	}

	adminDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL preview test database: %v", err)
	}
	adminDB.SetMaxOpenConns(1)
	pingContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adminDB.PingContext(pingContext); err != nil {
		_ = adminDB.Close()
		t.Fatalf("ping PostgreSQL preview test database: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })
	return adminDB, parsed
}

func openPreviewPostgresRaceSchema(t *testing.T, adminDB *sql.DB, parsedDatabaseURL *url.URL) *gorm.DB {
	t.Helper()
	schema := "preview_race_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminDB.ExecContext(context.Background(), `create schema "`+schema+`"`); err != nil {
		t.Fatalf("create PostgreSQL preview test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), `drop schema if exists "`+schema+`" cascade`)
	})

	scopedDatabaseURL := *parsedDatabaseURL
	query := scopedDatabaseURL.Query()
	query.Set("search_path", schema)
	scopedDatabaseURL.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(scopedDatabaseURL.String()), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL preview database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve isolated PostgreSQL preview pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })

	statements := []string{
		`create table publish_previews (
			preview_key text primary key,
			status text not null,
			output_path text not null default '',
			entry_path text not null default '',
			url text not null default '',
			target_url text not null default '',
			settings_hash text not null default '',
			content_hash text not null default '',
			manifest_json jsonb not null default '{}'::jsonb,
			log_json jsonb not null default '[]'::jsonb,
			error_code text not null default '',
			error_message text not null default '',
			finished_at timestamptz null,
			updated_at timestamptz not null default now(),
			deleted_at timestamptz null
		)`,
		`create function preview_terminal_delay() returns trigger language plpgsql as $$
		begin
			perform pg_sleep(0.04);
			return new;
		end
		$$`,
		`create trigger preview_terminal_delay
			before update on publish_previews
			for each row when (old.status = 'building')
			execute function preview_terminal_delay()`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create PostgreSQL preview test fixture: %v", err)
		}
	}
	return db
}

func assertPreviewRaceErrors(t *testing.T, finishErr error, failErr error) {
	t.Helper()
	nilCount := 0
	transitionCount := 0
	for _, err := range []error{finishErr, failErr} {
		switch {
		case err == nil:
			nilCount++
		case errors.Is(err, errPreviewTerminalTransition):
			transitionCount++
		default:
			t.Fatalf("unexpected terminal race error: %v", err)
		}
	}
	if nilCount != 1 || transitionCount != 1 {
		t.Fatalf("terminal race errors: finish=%v fail=%v", finishErr, failErr)
	}
}

func loadPreviewTerminalState(t *testing.T, db *gorm.DB, previewKey string) previewTerminalState {
	t.Helper()
	var state previewTerminalState
	err := db.Raw(`select status, output_path, entry_path, url, target_url,
		settings_hash, content_hash, manifest_json::text as manifest_json,
		log_json::text as log_json, error_code, error_message, finished_at
		from publish_previews where preview_key = ?`, previewKey).Scan(&state).Error
	if err != nil {
		t.Fatalf("load terminal preview state: %v", err)
	}
	return state
}

func assertPreviewJSONEqual(t *testing.T, raw []byte, want interface{}) {
	t.Helper()
	var gotValue interface{}
	if err := json.Unmarshal(raw, &gotValue); err != nil {
		t.Fatalf("decode preview JSON %q: %v", raw, err)
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("encode expected preview JSON: %v", err)
	}
	var wantValue interface{}
	if err := json.Unmarshal(wantRaw, &wantValue); err != nil {
		t.Fatalf("decode expected preview JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("preview JSON = %s, want %s", raw, wantRaw)
	}
}
