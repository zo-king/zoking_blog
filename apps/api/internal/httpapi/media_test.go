package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

func TestExactMediaIdentityCardinality(t *testing.T) {
	cases := []struct {
		name      string
		media     []model.MediaAsset
		ambiguous bool
	}{
		{name: "none"},
		{name: "one", media: []model.MediaAsset{{Checksum: "abc"}}},
		{name: "multiple", media: []model.MediaAsset{{Checksum: "abc"}, {Checksum: "abc"}}, ambiguous: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := exactMediaIdentity(test.media)
			if errors.Is(err, errMediaIdentityAmbiguous) != test.ambiguous {
				t.Fatalf("unexpected ambiguity result: %v", err)
			}
		})
	}
}

func TestStageMediaUploadStreamsAndCleansOversizedFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "media")
	reader := &countingReader{reader: strings.NewReader(strings.Repeat("x", 1024))}
	_, err := stageMediaUpload(config.Config{MediaLocalDir: root, MediaMaxBytes: 32}, reader)
	if !errors.Is(err, errMediaTooLarge) {
		t.Fatalf("stageMediaUpload() error = %v, want errMediaTooLarge", err)
	}
	if reader.read != 33 {
		t.Fatalf("source bytes read = %d, want bounded read of 33", reader.read)
	}
	stagingDir, err := privateMediaStagingDir(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("oversized upload left staging files: %v", entries)
	}
}

func TestStageMediaUploadCalculatesMetadataWhileStreaming(t *testing.T) {
	var payload bytes.Buffer
	if err := png.Encode(&payload, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	upload, err := stageMediaUpload(config.Config{
		MediaLocalDir: filepath.Join(t.TempDir(), "media"),
		MediaMaxBytes: int64(payload.Len() + 1),
	}, bytes.NewReader(payload.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(upload.path)
	wantSum := sha256.Sum256(payload.Bytes())
	if upload.mimeType != "image/png" || upload.width != 2 || upload.height != 3 {
		t.Fatalf("metadata = type %q dimensions %dx%d", upload.mimeType, upload.width, upload.height)
	}
	if upload.size != int64(payload.Len()) || upload.checksum != hex.EncodeToString(wantSum[:]) {
		t.Fatalf("size/checksum = %d/%s", upload.size, upload.checksum)
	}
}

func TestPrivateMediaDirectoriesStayInsideMediaRootAndAreNotServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := filepath.Join(t.TempDir(), "media")
	stagingDir, err := privateMediaStagingDir(root)
	if err != nil {
		t.Fatal(err)
	}
	quarantineDir, err := privateMediaQuarantineDir(root)
	if err != nil {
		t.Fatal(err)
	}
	privateRoot := filepath.Join(root, mediaPrivateDirName)
	for _, dir := range []string{stagingDir, quarantineDir} {
		if !isSafeMediaChild(root, dir) || !isSameOrSafeMediaPath(privateRoot, dir) {
			t.Fatalf("private media directory escaped media root: %s", dir)
		}
	}

	secretPath := filepath.Join(quarantineDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/media-files/*filepath", mediaFileServer(config.Config{MediaLocalDir: root}))
	request := httptest.NewRequest(http.MethodGet, "/media-files/"+mediaPrivateDirName+"/"+mediaQuarantineDirName+"/secret.txt", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("private media response = %d/%q, want 404", response.Code, response.Body.String())
	}
}

func TestUploadMediaLimitsConcurrentBodyReads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		MediaStorageDriver:        "local",
		MediaLocalDir:             filepath.Join(t.TempDir(), "media"),
		MediaMaxBytes:             1024,
		MediaUploadMaxConcurrency: 1,
	}
	handler := uploadMedia(nil, cfg)

	firstReader, firstWriter := io.Pipe()
	firstMultipart := multipart.NewWriter(firstWriter)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	go func() {
		part, _ := firstMultipart.CreateFormFile("file", "first.bin")
		_, _ = part.Write([]byte("x"))
		close(firstStarted)
		<-releaseFirst
		_ = firstMultipart.Close()
		_ = firstWriter.Close()
	}()
	firstDone := make(chan struct{})
	go func() {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/media", firstReader)
		c.Request.Header.Set("Content-Type", firstMultipart.FormDataContentType())
		handler(c)
		close(firstDone)
	}()
	<-firstStarted

	secondBody, secondContentType := testMultipartBody(t, []byte("x"))
	secondRead := make(chan struct{})
	secondDone := make(chan struct{})
	go func() {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/media", &notifyingReader{reader: bytes.NewReader(secondBody), firstRead: secondRead})
		c.Request.Header.Set("Content-Type", secondContentType)
		handler(c)
		close(secondDone)
	}()

	select {
	case <-secondRead:
		t.Fatal("second upload body was read before the first upload released its slot")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	select {
	case <-secondRead:
	case <-time.After(2 * time.Second):
		t.Fatal("second upload did not acquire the released slot")
	}
	<-firstDone
	<-secondDone
}

func TestRemoveLocalMediaFileRejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "media")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "secret.png")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if err := removeLocalMediaFile(root, "escape/secret.png"); !errors.Is(err, errUnsafeMediaPath) {
		t.Fatalf("removeLocalMediaFile() error = %v, want unsafe path", err)
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("outside file was affected: %v", err)
	}
}

func TestDeleteMediaRestoresFileAfterDatabaseFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
			db, mock := newPreviewMockDB(t)
			expectDeleteMediaQueries(mock, id, key)
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

			recorder := performDeleteMediaRequest(t, db, root, id)
			if recorder.Code == http.StatusOK {
				t.Fatalf("delete returned success after %s failure: %s", test.failAt, recorder.Body.String())
			}
			payload, err := os.ReadFile(target)
			if err != nil || string(payload) != "original" {
				t.Fatalf("media was not restored after %s failure: payload=%q err=%v", test.failAt, payload, err)
			}
			assertDirectoryEmpty(t, filepath.Join(root, mediaPrivateDirName, mediaQuarantineDirName))
			assertPreviewMockExpectations(t, mock)
		})
	}
}

func TestDeleteMediaKeepsOriginalURLRevokedWhenQuarantineCleanupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
	db, mock := newPreviewMockDB(t)
	expectDeleteMediaQueries(mock, id, key)
	mock.ExpectExec(`UPDATE "media_assets"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "media_assets"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	recorder := performDeleteMediaRequest(t, db, root, id)
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "MEDIA_QUARANTINE_CLEANUP_FAILED") {
		t.Fatalf("status/body = %d/%s, want explicit quarantine cleanup error", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original media path became accessible after cleanup failure: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, mediaPrivateDirName, mediaQuarantineDirName))
	if err != nil || len(entries) != 1 {
		t.Fatalf("quarantine entries = %v, err=%v, want retained private item", entries, err)
	}
	assertPreviewMockExpectations(t, mock)
}

func expectDeleteMediaQueries(mock sqlmock.Sqlmock, id uuid.UUID, key string) {
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "storage_driver", "storage_key", "status"}).AddRow(id, "local", key, "ready")
	}
	mock.ExpectQuery(`SELECT .*FROM "media_assets"`).WillReturnRows(rows())
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*FROM "media_assets"`).WillReturnRows(rows())
	mock.ExpectQuery(`SELECT count\(\*\) FROM "media_usages"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
}

func performDeleteMediaRequest(t *testing.T, db *gorm.DB, root string, id uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/media/"+id.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}
	deleteMedia(db, config.Config{MediaLocalDir: root})(c)
	return recorder

}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("directory %s contains restored quarantine artifacts: %v", path, entries)
	}
}

type countingReader struct {
	reader io.Reader
	read   int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += n
	return n, err
}

type notifyingReader struct {
	reader    io.Reader
	firstRead chan struct{}
	notified  bool
}

func (r *notifyingReader) Read(p []byte) (int, error) {
	if !r.notified {
		r.notified = true
		close(r.firstRead)
	}
	return r.reader.Read(p)
}

func testMultipartBody(t *testing.T, payload []byte) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "test.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}
