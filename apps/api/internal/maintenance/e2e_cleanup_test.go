package maintenance

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSafeE2EPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if _, ok := safeE2EPath(root, "prev-123"); !ok {
		t.Fatal("expected direct child to be safe")
	}
	if _, ok := safeE2EPath(root, "2026/07/media.png"); !ok {
		t.Fatal("expected nested storage key to be safe")
	}
	for _, key := range []string{"", ".", "..", "../outside", filepath.Join(root, "absolute")} {
		if _, ok := safeE2EPath(root, key); ok {
			t.Fatalf("expected %q to be unsafe", key)
		}
	}
}

func TestSafeContentPathRequiresCanonicalDirectChild(t *testing.T) {
	siteDir := t.TempDir()
	if _, ok := safeContentPath(siteDir, "post", "e2e-smoke-safe"); !ok {
		t.Fatal("expected canonical direct child slug to be safe")
	}
	for _, slug := range []string{"", ".", "..", "nested/slug", `nested\slug`, " slug", "slug ", "a/../slug"} {
		if _, ok := safeContentPath(siteDir, "post", slug); ok {
			t.Fatalf("expected %q to be rejected as a content slug", slug)
		}
	}
}

func TestFieldStringSupportsExactManifestMatching(t *testing.T) {
	post := model.Post{Slug: "e2e-smoke-exact"}
	if got := fieldString(post, "Slug"); got != post.Slug {
		t.Fatalf("expected %q, got %q", post.Slug, got)
	}
	if got := fieldString(post, "ReleaseKey"); got != "" {
		t.Fatalf("expected missing field to be empty, got %q", got)
	}
}

func TestValidateE2ESettingsRejectsIncompleteSnapshot(t *testing.T) {
	if err := validateE2ESettings(publisher.SiteSettingsSnapshot{}); err == nil {
		t.Fatal("expected incomplete settings snapshot to fail")
	}
}

func TestValidateUniqueManifestRefs(t *testing.T) {
	id := uuid.New()
	err := validateUniqueManifestRefs(E2ERunManifest{
		Posts: []E2ESlugRef{{ID: id, Slug: "e2e-smoke-one"}},
		Pages: []E2ESlugRef{{ID: id, Slug: "e2e-page-one"}},
	})
	if err == nil {
		t.Fatal("expected duplicate id to fail validation")
	}
}

func TestManifestIDs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	got := keyRefIDs([]E2EKeyRef{{ID: a, Key: "a"}, {ID: b, Key: "b"}})
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("unexpected IDs: %v", got)
	}
}

func TestAllowedE2ECleanupSlugIsExactlyBoundToRunID(t *testing.T) {
	runID := uuid.New()
	for table, slug := range map[string]string{
		"posts":      "e2e-smoke-" + runID.String(),
		"pages":      "e2e-page-" + runID.String(),
		"categories": "e2e-category-" + runID.String(),
		"tags":       "e2e-tag-" + runID.String(),
	} {
		if !allowedE2ECleanupSlug(table, slug, runID) {
			t.Fatalf("expected %s/%s to match run_id", table, slug)
		}
	}

	otherRunID := uuid.New()
	for _, test := range []struct {
		table string
		slug  string
	}{
		{"posts", "hello-from-postgresql"},
		{"pages", "about"},
		{"categories", "development"},
		{"tags", "go"},
		{"posts", "e2e-smoke-" + otherRunID.String()},
		{"posts", "e2e-smoke-" + runID.String() + "-extra"},
		{"posts", "e2e-smoke-" + runID.String() + "/nested"},
		{"unknown", "e2e-smoke-" + runID.String()},
	} {
		if allowedE2ECleanupSlug(test.table, test.slug, runID) {
			t.Fatalf("expected %s/%s to be rejected", test.table, test.slug)
		}
	}
}

func TestMediaOriginalNameMustContainRunID(t *testing.T) {
	runID := uuid.New()
	for _, name := range []string{
		"zoking-e2e-" + runID.String() + "-referenced.png",
		"zoking-e2e-" + runID.String() + "-orphan.png",
	} {
		if !mediaOriginalNameMatchesRunID(name, runID) {
			t.Fatalf("expected %q to match run_id", name)
		}
	}
	if mediaOriginalNameMatchesRunID("zoking-e2e-"+uuid.New().String()+"-orphan.png", runID) {
		t.Fatal("media from another run must be rejected")
	}
}

func TestManifestLoadersRejectMissingIDs(t *testing.T) {
	db := newCleanupTestDB(t, &cleanupTestDBState{})
	runID := uuid.New()

	tests := []struct {
		name string
		load func() error
	}{
		{
			name: "slug reference",
			load: func() error {
				_, err := loadSlugRefs[model.Post](context.Background(), db, []E2ESlugRef{{ID: uuid.New(), Slug: "e2e-smoke-" + runID.String()}}, "posts", runID)
				return err
			},
		},
		{
			name: "id reference",
			load: func() error {
				_, err := loadIDRefs[model.Comment](context.Background(), db, []E2EIDRef{{ID: uuid.New()}})
				return err
			},
		},
		{
			name: "key reference",
			load: func() error {
				_, err := loadKeyRefs[model.MediaAsset](context.Background(), db, []E2EKeyRef{{ID: uuid.New(), Key: "2026/07/missing.png"}}, "storage_key")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.load()
			var conflict *E2EValidationError
			if !errors.As(err, &conflict) {
				t.Fatalf("expected manifest conflict, got %v", err)
			}
		})
	}
}

func TestStageE2EFilesCanRollbackAndPurge(t *testing.T) {
	siteDir := t.TempDir()
	runID := uuid.New()
	postID := uuid.New()
	slug := "e2e-smoke-" + runID.String()
	original := filepath.Join(siteDir, "content", "post", slug)
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "index.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestSlug := "e2e-smoke-" + uuid.New().String()
	manifestPath := filepath.Join(siteDir, "content", "post", manifestSlug)
	if err := os.MkdirAll(manifestPath, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := e2eCleanupPlan{
		manifest:            E2ERunManifest{RunID: runID, Posts: []E2ESlugRef{{ID: postID, Slug: manifestSlug}}},
		posts:               []model.Post{{Base: model.Base{ID: postID}, Slug: slug}},
		removePostSnapshots: map[uuid.UUID]bool{postID: true},
	}
	staging, err := stageE2EFiles(config.Config{HugoSiteDir: siteDir}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Fatalf("original path was not staged: %v", err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("raw manifest slug affected staging: %v", err)
	}
	if err := staging.rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(original, "index.md")); err != nil {
		t.Fatalf("rollback did not restore content: %v", err)
	}

	staging, err = stageE2EFiles(config.Config{HugoSiteDir: siteDir}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := staging.purge(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Fatalf("purge unexpectedly restored original: %v", err)
	}
}

func TestStageE2EFilesCanPreserveConflictingSnapshot(t *testing.T) {
	siteDir := t.TempDir()
	runID := uuid.New()
	pageID := uuid.New()
	slug := "e2e-page-" + runID.String()
	original := filepath.Join(siteDir, "content", "page", slug)
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := e2eCleanupPlan{
		manifest:            E2ERunManifest{RunID: runID},
		pages:               []model.Page{{Base: model.Base{ID: pageID}, Slug: slug}},
		removePageSnapshots: map[uuid.UUID]bool{pageID: false},
	}
	staging, err := stageE2EFiles(config.Config{HugoSiteDir: siteDir}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(staging.paths) != 0 {
		t.Fatalf("expected no staged paths, got %d", len(staging.paths))
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("preserved snapshot disappeared: %v", err)
	}
}

func TestStageE2EFilesStagesTaxonomySnapshots(t *testing.T) {
	siteDir := t.TempDir()
	runID := uuid.New()
	categoryID := uuid.New()
	tagID := uuid.New()
	categorySlug := "e2e-category-" + runID.String()
	tagSlug := "e2e-tag-" + runID.String()
	paths := []string{
		filepath.Join(siteDir, "content", "categories", categorySlug),
		filepath.Join(siteDir, "content", "tags", tagSlug),
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "_index.md"), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plan := e2eCleanupPlan{
		manifest:   E2ERunManifest{RunID: runID},
		categories: []model.Category{{Base: model.Base{ID: categoryID}, Slug: categorySlug}},
		tags:       []model.Tag{{Base: model.Base{ID: tagID}, Slug: tagSlug}},
	}
	staging, err := stageE2EFiles(config.Config{HugoSiteDir: siteDir}, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("taxonomy path was not staged: %s: %v", path, err)
		}
	}
	if err := staging.rollback(); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(path, "_index.md")); err != nil {
			t.Fatalf("taxonomy rollback did not restore %s: %v", path, err)
		}
	}
}

func TestStageE2EFilesUsesLoadedKeyPlans(t *testing.T) {
	root := t.TempDir()
	previewRoot := filepath.Join(root, "previews")
	releaseRoot := filepath.Join(root, "releases")
	mediaRoot := filepath.Join(root, "media")
	runID := uuid.New()
	previewID := uuid.New()
	releaseID := uuid.New()
	mediaID := uuid.New()

	previewKey := "preview-" + runID.String()
	releaseKey := "release-" + runID.String()
	mediaKey := "2026/07/zoking-e2e-" + runID.String() + "-referenced.png"
	manifestPreviewKey := "manifest-preview"
	manifestReleaseKey := "manifest-release"
	manifestMediaKey := "2026/07/manifest-media.png"

	loadedPaths := []string{
		filepath.Join(previewRoot, previewKey),
		filepath.Join(releaseRoot, releaseKey),
		filepath.Join(mediaRoot, filepath.FromSlash(mediaKey)),
	}
	manifestPaths := []string{
		filepath.Join(previewRoot, manifestPreviewKey),
		filepath.Join(releaseRoot, manifestReleaseKey),
		filepath.Join(mediaRoot, filepath.FromSlash(manifestMediaKey)),
	}
	for index, path := range append(loadedPaths, manifestPaths...) {
		if index == 2 || index == 5 {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	plan := e2eCleanupPlan{
		manifest: E2ERunManifest{
			RunID:    runID,
			Previews: []E2EKeyRef{{ID: previewID, Key: manifestPreviewKey}},
			Releases: []E2EKeyRef{{ID: releaseID, Key: manifestReleaseKey}},
			Media:    []E2EKeyRef{{ID: mediaID, Key: manifestMediaKey}},
		},
		previews: []model.PublishPreview{{Base: model.Base{ID: previewID}, PreviewKey: previewKey}},
		releases: []model.PublishRelease{{Base: model.Base{ID: releaseID}, ReleaseKey: releaseKey}},
		media:    []model.MediaAsset{{Base: model.Base{ID: mediaID}, StorageKey: mediaKey}},
	}
	staging, err := stageE2EFiles(config.Config{
		PublishPreviewRoot: previewRoot,
		PublishReleaseRoot: releaseRoot,
		MediaLocalDir:      mediaRoot,
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range loadedPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("loaded plan path was not staged: %s: %v", path, err)
		}
	}
	for _, path := range manifestPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("raw manifest path affected staging: %s: %v", path, err)
		}
	}
	if err := staging.rollback(); err != nil {
		t.Fatal(err)
	}
	for _, path := range loadedPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("rollback did not restore loaded plan path: %s: %v", path, err)
		}
	}
}

func TestDeleteE2ERecordsReturnsRowsAffectedCounts(t *testing.T) {
	state := &cleanupTestDBState{deleteRows: map[string]int64{"posts": 1}}
	db := newCleanupTestDB(t, state)
	postID := uuid.New()
	deleted, err := deleteE2ERecords(context.Background(), db, e2eCleanupPlan{
		manifest: E2ERunManifest{RunID: uuid.New()},
		posts:    []model.Post{{Base: model.Base{ID: postID}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted["posts"] != 1 {
		t.Fatalf("expected one actually deleted post, got %d", deleted["posts"])
	}
	for _, candidate := range []string{"pages", "categories", "tags", "comments", "media", "previews", "jobs", "releases"} {
		if deleted[candidate] != 0 {
			t.Fatalf("expected zero deleted %s, got %d", candidate, deleted[candidate])
		}
	}
	commits, rollbacks := state.transactionOutcomes()
	if commits != 1 || rollbacks != 0 {
		t.Fatalf("expected committed delete transaction, commits=%d rollbacks=%d", commits, rollbacks)
	}
}

func TestDeleteE2ERecordsRollsBackOnRowsAffectedMismatch(t *testing.T) {
	state := &cleanupTestDBState{deleteRows: map[string]int64{"posts": 0}}
	db := newCleanupTestDB(t, state)
	postID := uuid.New()
	deleted, err := deleteE2ERecords(context.Background(), db, e2eCleanupPlan{
		manifest: E2ERunManifest{RunID: uuid.New()},
		posts:    []model.Post{{Base: model.Base{ID: postID}}},
	})
	var conflict *E2EValidationError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected RowsAffected mismatch conflict, got deleted=%v err=%v", deleted, err)
	}
	if deleted != nil {
		t.Fatalf("failed transaction must not report deleted rows: %v", deleted)
	}
	commits, rollbacks := state.transactionOutcomes()
	if commits != 0 || rollbacks != 1 {
		t.Fatalf("expected rolled back delete transaction, commits=%d rollbacks=%d", commits, rollbacks)
	}
}

type cleanupTestDBState struct {
	mu         sync.Mutex
	deleteRows map[string]int64
	commits    int
	rollbacks  int
}

func (s *cleanupTestDBState) rowsAffected(query string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	lowerQuery := strings.ToLower(query)
	for table, count := range s.deleteRows {
		if strings.Contains(lowerQuery, `delete from "`+table+`"`) {
			return count
		}
	}
	return 0
}

func (s *cleanupTestDBState) recordCommit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commits++
}

func (s *cleanupTestDBState) recordRollback() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollbacks++
}

func (s *cleanupTestDBState) transactionOutcomes() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commits, s.rollbacks
}

func newCleanupTestDB(t *testing.T, state *cleanupTestDBState) *gorm.DB {
	t.Helper()
	sqlDB := sql.OpenDB(cleanupTestConnector{state: state})
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, WithoutReturning: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open cleanup test database: %v", err)
	}
	return db
}

type cleanupTestConnector struct {
	state *cleanupTestDBState
}

func (c cleanupTestConnector) Connect(context.Context) (driver.Conn, error) {
	return &cleanupTestConn{state: c.state}, nil
}

func (c cleanupTestConnector) Driver() driver.Driver {
	return cleanupTestDriver{}
}

type cleanupTestDriver struct{}

func (cleanupTestDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("cleanup test driver requires connector")
}

type cleanupTestConn struct {
	state *cleanupTestDBState
}

func (c *cleanupTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *cleanupTestConn) Close() error {
	return nil
}

func (c *cleanupTestConn) Begin() (driver.Tx, error) {
	return &cleanupTestTx{state: c.state}, nil
}

func (c *cleanupTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &cleanupTestTx{state: c.state}, nil
}

func (c *cleanupTestConn) Ping(context.Context) error {
	return nil
}

func (c *cleanupTestConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(c.state.rowsAffected(query)), nil
}

func (c *cleanupTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return cleanupTestRows{}, nil
}

type cleanupTestTx struct {
	state *cleanupTestDBState
}

func (tx *cleanupTestTx) Commit() error {
	tx.state.recordCommit()
	return nil
}

func (tx *cleanupTestTx) Rollback() error {
	tx.state.recordRollback()
	return nil
}

type cleanupTestRows struct{}

func (cleanupTestRows) Columns() []string {
	return []string{"id"}
}

func (cleanupTestRows) Close() error {
	return nil
}

func (cleanupTestRows) Next([]driver.Value) error {
	return io.EOF
}
