package maintenance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNormalizeMediaOptionsAllowsExplicitImmediateGrace(t *testing.T) {
	cfg := config.Config{
		MediaOrphanGracePeriod: 7 * 24 * time.Hour,
		MediaCleanupBatchSize:  100,
	}

	options := normalizeMediaOptions(cfg, MediaCleanupOptions{})
	if options.GracePeriod != 0 {
		t.Fatalf("explicit zero grace should mean immediate cleanup, got %s", options.GracePeriod)
	}

	options = normalizeMediaOptions(cfg, MediaCleanupOptions{UseDefaultGracePeriod: true})
	if options.GracePeriod != cfg.MediaOrphanGracePeriod {
		t.Fatalf("default grace not applied: got %s", options.GracePeriod)
	}
}

func TestNormalizeReleaseOptionsKeepsAtLeastOneInactiveRelease(t *testing.T) {
	cfg := config.Config{
		PublishReleaseKeepLatest: 20,
		PublishReleaseKeepDays:   30,
	}

	options := normalizeReleaseOptions(cfg, ReleaseCleanupOptions{KeepLatest: 0, KeepDays: 0})
	if options.KeepLatest != 1 {
		t.Fatalf("expected explicit zero keep_latest to clamp to 1, got %d", options.KeepLatest)
	}
	if options.KeepDays != 0 {
		t.Fatalf("expected explicit zero keep_days to be preserved, got %d", options.KeepDays)
	}

	options = normalizeReleaseOptions(cfg, ReleaseCleanupOptions{UseDefaultKeepLatest: true, UseDefaultKeepDays: true})
	if options.KeepLatest != cfg.PublishReleaseKeepLatest || options.KeepDays != cfg.PublishReleaseKeepDays {
		t.Fatalf("default release policy not applied: %+v", options)
	}
}

func TestSafeChildPathRejectsPathTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "media")
	if _, ok := safeChildPath(root, "2026/07/example.png"); !ok {
		t.Fatalf("expected normal storage key to be accepted")
	}
	if _, ok := safeChildPath(root, "../outside.png"); ok {
		t.Fatalf("expected path traversal storage key to be rejected")
	}
	if _, ok := safeChildPath(root, "2026/../outside.png"); ok {
		t.Fatalf("expected non-canonical storage key to be rejected")
	}
}

func TestCleanupOrphanMediaRestoresFileAfterDatabaseFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		failAt string
	}{
		{name: "status update", failAt: "update"},
		{name: "soft delete", failAt: "delete"},
		{name: "commit", failAt: "commit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "media")
			key := "2026/07/example.png"
			target := filepath.Join(root, filepath.FromSlash(key))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}

			id := uuid.New()
			createdAt := time.Now().Add(-time.Hour)
			db, mock := newOrphanCleanupMockDB(t)
			expectOrphanCleanupQueries(mock, id, key, createdAt)
			switch test.failAt {
			case "update":
				mock.ExpectExec(`UPDATE "media_assets"`).WillReturnError(errors.New("update failed"))
				mock.ExpectRollback()
			case "delete":
				mock.ExpectExec(`UPDATE "media_assets"`).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE "media_assets"`).WillReturnError(errors.New("soft delete failed"))
				mock.ExpectRollback()
			case "commit":
				mock.ExpectExec(`UPDATE "media_assets"`).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE "media_assets"`).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
			}

			result, err := runOrphanCleanup(db, root)
			if err == nil {
				t.Fatalf("CleanupOrphanMedia() returned nil after %s failure", test.failAt)
			}
			if result.DeletedCount != 0 {
				t.Fatalf("DeletedCount = %d after rollback", result.DeletedCount)
			}
			payload, readErr := os.ReadFile(target)
			if readErr != nil || string(payload) != "original" {
				t.Fatalf("orphan media was not restored after %s failure: payload=%q err=%v", test.failAt, payload, readErr)
			}
			assertCleanupDirectoryEmpty(t, filepath.Join(root, ".zoking-private", "quarantine"))
			if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
				t.Fatalf("unmet SQL expectations: %v", mockErr)
			}
		})
	}
}

func TestCleanupOrphanMediaKeepsOriginalURLRevokedWhenQuarantineCleanupFails(t *testing.T) {
	root := filepath.Join(t.TempDir(), "media")
	key := "2026/07/not-a-file.png"
	target := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	createdAt := time.Now().Add(-time.Hour)
	db, mock := newOrphanCleanupMockDB(t)
	expectOrphanCleanupQueries(mock, id, key, createdAt)
	mock.ExpectExec(`UPDATE "media_assets"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "media_assets"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := runOrphanCleanup(db, root)
	if err == nil || !strings.Contains(err.Error(), "quarantine cleanup failed") {
		t.Fatalf("CleanupOrphanMedia() result=%+v err=%v, want explicit quarantine cleanup error", result, err)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("original orphan path became accessible after cleanup failure: %v", statErr)
	}
	entries, readErr := os.ReadDir(filepath.Join(root, ".zoking-private", "quarantine"))
	if readErr != nil || len(entries) != 1 {
		t.Fatalf("quarantine entries = %v, err=%v, want retained private item", entries, readErr)
	}
	if mockErr := mock.ExpectationsWereMet(); mockErr != nil {
		t.Fatalf("unmet SQL expectations: %v", mockErr)
	}
}

func expectOrphanCleanupQueries(mock sqlmock.Sqlmock, id uuid.UUID, key string, createdAt time.Time) {
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "created_at", "storage_driver", "storage_key", "status"}).
			AddRow(id, createdAt, "local", key, "ready")
	}
	mock.ExpectQuery(`SELECT .*FROM "media_assets"`).WillReturnRows(rows())
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*FROM "media_assets"`).WillReturnRows(rows())
	mock.ExpectQuery(`SELECT count\(\*\) FROM "media_usages"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
}

func runOrphanCleanup(db *gorm.DB, root string) (CleanupResult, error) {
	return CleanupOrphanMedia(context.Background(), db, config.Config{MediaLocalDir: root}, MediaCleanupOptions{
		GracePeriod: 0,
		BatchSize:   1,
		Now:         time.Now(),
	})
}

func assertCleanupDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("directory %s contains restored quarantine artifacts: %v", path, entries)
	}
}

func newOrphanCleanupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestDangerousPreviewRootRejectsProtectedPaths(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "previews")
	for _, protected := range []string{
		root,
		filepath.Join(root, "current"),
		parent,
	} {
		cfg := config.Config{PublishPreviewRoot: root, PublishCurrentDir: protected}
		if !dangerousPreviewRoot(root, cfg) {
			t.Fatalf("expected preview root overlap with %q to be rejected", protected)
		}
	}
	cfg := config.Config{PublishPreviewRoot: root, PublishCurrentDir: filepath.Join(parent, "current")}
	if dangerousPreviewRoot(root, cfg) {
		t.Fatal("expected sibling preview and current roots to be accepted")
	}
}
