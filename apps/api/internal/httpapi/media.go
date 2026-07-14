package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var allowedMediaTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var errMediaIdentityAmbiguous = errors.New("media identity is ambiguous")

var (
	errMediaTooLarge           = errors.New("media file is too large")
	errUnsafeMediaPath         = errors.New("unsafe media storage path")
	errUnsupportedMediaStorage = errors.New("unsupported media storage driver")
)

const (
	defaultMediaUploadConcurrency = 4
	mediaCopyBufferSize           = 32 * 1024
	mediaPrivateDirName           = ".zoking-private"
	mediaStagingDirName           = "uploads"
	mediaQuarantineDirName        = "quarantine"
)

type stagedMediaUpload struct {
	path     string
	size     int64
	mimeType string
	checksum string
	width    int
	height   int
}

type quarantinedMediaFile struct {
	root           string
	originalPath   string
	quarantinePath string
	moved          bool
}

func listAdminMedia(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		queryValues := c.Request.URL.Query()
		checksumValues, checksumProvided := queryValues["checksum"]
		publicURLValues, publicURLProvided := queryValues["public_url"]
		if checksumProvided && publicURLProvided {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "media query accepts only one exact filter")
			return
		}
		if checksumProvided || publicURLProvided {
			var media []model.MediaAsset
			query := db.WithContext(c.Request.Context()).Where("status <> ?", "deleted")
			if checksumProvided {
				checksum := ""
				if len(checksumValues) > 0 {
					checksum = strings.ToLower(strings.TrimSpace(checksumValues[0]))
				}
				decoded, err := hex.DecodeString(checksum)
				if err != nil || len(decoded) != sha256.Size {
					Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid media checksum")
					return
				}
				query = query.Where("lower(checksum) = ?", checksum).Order("created_at asc").Limit(2)
			} else {
				publicURL := ""
				if len(publicURLValues) > 0 {
					publicURL = publicURLValues[0]
				}
				if strings.TrimSpace(publicURL) == "" || len(publicURL) > 2048 || strings.ContainsAny(publicURL, "\r\n") {
					Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid media public URL")
					return
				}
				query = query.Where("public_url = ?", publicURL).Order("created_at asc").Limit(2)
			}
			if err := query.Find(&media).Error; err != nil {
				Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list media")
				return
			}
			if _, err := exactMediaIdentity(media); errors.Is(err, errMediaIdentityAmbiguous) {
				Fail(c, http.StatusConflict, "MEDIA_IDENTITY_AMBIGUOUS", "multiple active media records match the exact identity")
				return
			}
			if err := attachMediaUsageCounts(db.WithContext(c.Request.Context()), media); err != nil {
				Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count media usages")
				return
			}
			OK(c, media)
			return
		}

		pagination, ok := parsePagination(c)
		if !ok {
			return
		}
		order, ok := adminListOrder(pagination.Sort, map[string]string{
			"created_at":    "created_at",
			"updated_at":    "updated_at",
			"original_name": "original_name",
			"filename":      "filename",
			"mime_type":     "mime_type",
			"size_bytes":    "size_bytes",
			"status":        "status",
		})
		if !ok {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
			return
		}
		query := db.WithContext(c.Request.Context()).Model(&model.MediaAsset{}).Where("status <> ?", "deleted")
		if pagination.Query != "" {
			pattern := "%" + pagination.Query + "%"
			query = query.Where("original_name ILIKE ? OR filename ILIKE ? OR mime_type ILIKE ?", pattern, pattern, pattern)
		}
		if pagination.Status != "" {
			query = query.Where("status = ?", pagination.Status)
		}
		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count media")
			return
		}
		if returnEmptyPageIfOutOfRange[model.MediaAsset](c, total, pagination) {
			return
		}
		var media []model.MediaAsset
		if err := query.Order(order + ", id asc").Offset(pagination.Offset).Limit(pagination.PageSize).Find(&media).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list media")
			return
		}
		if err := attachMediaUsageCounts(db.WithContext(c.Request.Context()), media); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count media usages")
			return
		}
		OKPaginated(c, media, total, pagination)
	}
}

func getAdminMedia(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var media model.MediaAsset
		if err := db.WithContext(c.Request.Context()).
			Where("status <> ?", "deleted").
			First(&media, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "media not found")
			return
		}
		mediaList := []model.MediaAsset{media}
		if err := attachMediaUsageCounts(db.WithContext(c.Request.Context()), mediaList); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count media usages")
			return
		}
		OK(c, mediaList[0])
	}
}

func uploadMedia(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	concurrency := cfg.MediaUploadMaxConcurrency
	if concurrency <= 0 {
		concurrency = defaultMediaUploadConcurrency
	}
	uploadSlots := make(chan struct{}, concurrency)

	return func(c *gin.Context) {
		if cfg.MediaStorageDriver != "local" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "unsupported media storage driver")
			return
		}
		if cfg.MediaMaxBytes <= 0 {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "media upload limit is not configured")
			return
		}
		select {
		case uploadSlots <- struct{}{}:
			defer func() { <-uploadSlots }()
		case <-c.Request.Context().Done():
			return
		}

		multipartReader, err := c.Request.MultipartReader()
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "missing media file")
			return
		}

		var originalName string
		var upload stagedMediaUpload
		for {
			part, nextErr := multipartReader.NextPart()
			if errors.Is(nextErr, io.EOF) {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "missing media file")
				return
			}
			if nextErr != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "could not read media upload")
				return
			}
			if part.FormName() != "file" || part.FileName() == "" {
				_ = part.Close()
				continue
			}
			originalName = part.FileName()
			upload, err = stageMediaUpload(cfg, part)
			_ = part.Close()
			break
		}
		if errors.Is(err, errMediaTooLarge) {
			Fail(c, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "media file is too large")
			return
		}
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "could not read media file")
			return
		}
		defer func() {
			if upload.path != "" {
				_ = os.Remove(upload.path)
			}
		}()

		ext, ok := allowedMediaTypes[upload.mimeType]
		if !ok {
			Fail(c, http.StatusUnsupportedMediaType, "MEDIA_TYPE_NOT_ALLOWED", "media type is not allowed")
			return
		}

		existing, err := findActiveMediaByChecksum(c.Request.Context(), db, upload.checksum)
		if errors.Is(err, errMediaIdentityAmbiguous) {
			Fail(c, http.StatusConflict, "MEDIA_IDENTITY_AMBIGUOUS", "multiple active media records have the uploaded checksum")
			return
		}
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not check existing media")
			return
		}
		if existing != nil {
			if err := removeStagedMediaFile(upload.path); err != nil {
				Fail(c, http.StatusInternalServerError, "MEDIA_CLEANUP_FAILED", "could not clean staged media upload")
				return
			}
			upload.path = ""
			OK(c, existing)
			return
		}

		now := time.Now()
		storageKey := filepath.ToSlash(filepath.Join(now.Format("2006"), now.Format("01"), uuid.NewString()+ext))
		absPath, err := prepareMediaDestination(cfg.MediaLocalDir, storageKey)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create media directory")
			return
		}
		if err := os.Chmod(upload.path, 0o644); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not prepare media file")
			return
		}
		if err := os.Rename(upload.path, absPath); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store media file")
			return
		}
		upload.path = ""

		publicURL := strings.TrimRight(cfg.MediaPublicBaseURL, "/") + "/" + storageKey
		userID := parseOptionalUUID(c.GetString("user_id"))
		media := model.MediaAsset{
			Filename:      filepath.Base(storageKey),
			OriginalName:  originalName,
			MimeType:      upload.mimeType,
			SizeBytes:     upload.size,
			Width:         upload.width,
			Height:        upload.height,
			StorageDriver: cfg.MediaStorageDriver,
			StorageKey:    storageKey,
			PublicURL:     publicURL,
			Checksum:      upload.checksum,
			UploadedBy:    userID,
			Status:        "ready",
		}

		if err := db.WithContext(c.Request.Context()).Create(&media).Error; err != nil {
			if removeErr := removeLocalMediaFile(cfg.MediaLocalDir, storageKey); removeErr != nil {
				Fail(c, http.StatusInternalServerError, "MEDIA_ROLLBACK_FAILED", "could not revoke failed media upload")
				return
			}
			existing, lookupErr := findActiveMediaByChecksum(c.Request.Context(), db, upload.checksum)
			if lookupErr == nil && existing != nil {
				OK(c, existing)
				return
			}
			if errors.Is(lookupErr, errMediaIdentityAmbiguous) {
				Fail(c, http.StatusConflict, "MEDIA_IDENTITY_AMBIGUOUS", "multiple active media records have the uploaded checksum")
				return
			}
			Fail(c, http.StatusConflict, "CONFLICT", "could not create media record")
			return
		}

		Created(c, media)
	}
}

func removeStagedMediaFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove staged media file: %w", err)
	}
	return nil
}

func stageMediaUpload(cfg config.Config, source io.Reader) (stagedMediaUpload, error) {
	var upload stagedMediaUpload
	stagingDir, err := privateMediaStagingDir(cfg.MediaLocalDir)
	if err != nil {
		return upload, err
	}
	file, err := os.CreateTemp(stagingDir, "upload-*")
	if err != nil {
		return upload, err
	}
	upload.path = file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(upload.path)
		}
	}()

	hasher := sha256.New()
	buffer := make([]byte, mediaCopyBufferSize)
	upload.size, err = io.CopyBuffer(io.MultiWriter(file, hasher), io.LimitReader(source, cfg.MediaMaxBytes+1), buffer)
	if err != nil {
		return upload, err
	}
	if upload.size > cfg.MediaMaxBytes {
		return upload, errMediaTooLarge
	}
	if upload.size == 0 {
		return upload, errors.New("empty media file")
	}
	if err := file.Sync(); err != nil {
		return upload, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return upload, err
	}
	sniff := make([]byte, 512)
	n, readErr := io.ReadFull(file, sniff)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return upload, readErr
	}
	upload.mimeType = http.DetectContentType(sniff[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return upload, err
	}
	if imageConfig, _, decodeErr := image.DecodeConfig(file); decodeErr == nil {
		upload.width = imageConfig.Width
		upload.height = imageConfig.Height
	}
	upload.checksum = hex.EncodeToString(hasher.Sum(nil))
	if err := file.Close(); err != nil {
		return upload, err
	}
	keep = true
	return upload, nil
}

func privateMediaStagingDir(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errUnsafeMediaPath
	}
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	return privateMediaSubdir(resolvedRoot, mediaStagingDirName)
}

func prepareMediaDestination(root, storageKey string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	target, err := safeMediaStoragePath(root, storageKey)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	relativeDir, err := filepath.Rel(filepath.Clean(root), filepath.Dir(target))
	if err != nil {
		return "", err
	}
	current := filepath.Clean(root)
	for _, component := range strings.Split(relativeDir, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil || !isSafeMediaChild(resolvedRoot, resolved) {
			return "", errUnsafeMediaPath
		}
		info, err := os.Stat(current)
		if err != nil || !info.IsDir() {
			return "", errUnsafeMediaPath
		}
	}
	if _, err := os.Lstat(target); err == nil || !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return "", os.ErrExist
		}
		return "", err
	}
	return target, nil
}

func safeMediaStoragePath(root, storageKey string) (string, error) {
	if strings.TrimSpace(root) == "" || storageKey == "" || strings.ContainsAny(storageKey, `\:`) || strings.HasPrefix(storageKey, "/") {
		return "", errUnsafeMediaPath
	}
	converted := filepath.FromSlash(storageKey)
	if filepath.IsAbs(converted) || filepath.ToSlash(filepath.Clean(converted)) != storageKey {
		return "", errUnsafeMediaPath
	}
	var err error
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(root, converted))
	if !isSafeMediaChild(root, target) {
		return "", errUnsafeMediaPath
	}
	return target, nil
}

func removeLocalMediaFile(root, storageKey string) error {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return err
	}
	target, err := safeMediaStoragePath(root, storageKey)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !isSafeMediaChild(resolvedRoot, resolvedTarget) {
		return errUnsafeMediaPath
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove media file: %w", err)
	}
	return nil
}

func quarantineLocalMediaFile(root, storageKey string) (*quarantinedMediaFile, error) {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	target, err := safeMediaStoragePath(root, storageKey)
	if err != nil {
		return nil, err
	}
	quarantine := &quarantinedMediaFile{root: root, originalPath: target}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return quarantine, nil
		}
		return nil, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return quarantine, nil
		}
		return nil, err
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil || !isSameOrSafeMediaPath(resolvedRoot, resolvedParent) {
		return nil, errUnsafeMediaPath
	}
	resolvedTarget, resolveErr := filepath.EvalSymlinks(target)
	if resolveErr == nil {
		if !isSafeMediaChild(resolvedRoot, resolvedTarget) {
			return nil, errUnsafeMediaPath
		}
	} else if info.Mode()&os.ModeSymlink == 0 || !errors.Is(resolveErr, os.ErrNotExist) {
		return nil, resolveErr
	}

	quarantineDir, err := privateMediaQuarantineDir(resolvedRoot)
	if err != nil {
		return nil, err
	}
	quarantine.quarantinePath = filepath.Join(quarantineDir, uuid.NewString())
	if err := os.Rename(target, quarantine.quarantinePath); err != nil {
		return nil, fmt.Errorf("quarantine media file: %w", err)
	}
	quarantine.moved = true
	return quarantine, nil
}

func privateMediaQuarantineDir(resolvedRoot string) (string, error) {
	return privateMediaSubdir(resolvedRoot, mediaQuarantineDirName)
}

func privateMediaSubdir(resolvedRoot, name string) (string, error) {
	privateRoot := filepath.Join(resolvedRoot, mediaPrivateDirName)
	if err := ensurePrivateMediaDir(privateRoot); err != nil {
		return "", err
	}
	dir := filepath.Join(privateRoot, name)
	if err := ensurePrivateMediaDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func ensurePrivateMediaDir(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errUnsafeMediaPath
	}
	return os.Chmod(dir, 0o700)
}

func (file *quarantinedMediaFile) restore() error {
	if file == nil || !file.moved {
		return nil
	}
	resolvedRoot, err := filepath.EvalSymlinks(file.root)
	if err != nil {
		return fmt.Errorf("resolve media root for restore: %w", err)
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(file.originalPath))
	if err != nil || !isSameOrSafeMediaPath(resolvedRoot, resolvedParent) {
		return errUnsafeMediaPath
	}
	if _, err := os.Lstat(file.originalPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("restore media file: destination already exists")
		}
		return err
	}
	if err := os.Rename(file.quarantinePath, file.originalPath); err != nil {
		return fmt.Errorf("restore media file: %w", err)
	}
	file.moved = false
	return nil
}

func (file *quarantinedMediaFile) discard() error {
	if file == nil || !file.moved {
		return nil
	}
	if err := os.Remove(file.quarantinePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove quarantined media file: %w", err)
	}
	file.moved = false
	return nil
}

func isSameOrSafeMediaPath(root, candidate string) bool {
	return filepath.Clean(root) == filepath.Clean(candidate) || isSafeMediaChild(root, candidate)
}

func findActiveMediaByChecksum(ctx context.Context, db *gorm.DB, checksum string) (*model.MediaAsset, error) {
	var media []model.MediaAsset
	if err := db.WithContext(ctx).
		Where("status <> ? and lower(checksum) = ?", "deleted", strings.ToLower(strings.TrimSpace(checksum))).
		Order("created_at asc").
		Limit(2).
		Find(&media).Error; err != nil {
		return nil, err
	}
	return exactMediaIdentity(media)
}

func exactMediaIdentity(media []model.MediaAsset) (*model.MediaAsset, error) {
	if len(media) > 1 {
		return nil, errMediaIdentityAmbiguous
	}
	if len(media) == 0 {
		return nil, nil
	}
	return &media[0], nil
}

func deleteMedia(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var media model.MediaAsset
		if err := db.WithContext(c.Request.Context()).
			Where("status <> ?", "deleted").
			First(&media, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "media not found")
			return
		}

		var quarantined *quarantinedMediaFile
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var locked model.MediaAsset
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("status <> ?", "deleted").First(&locked, "id = ?", media.ID).Error; err != nil {
				return err
			}
			var usageCount int64
			if err := tx.Model(&model.MediaUsage{}).
				Where("media_id = ?", locked.ID).
				Count(&usageCount).Error; err != nil {
				return err
			}
			if usageCount > 0 {
				return gorm.ErrInvalidData
			}
			if locked.StorageDriver != "local" || cfg.MediaLocalDir == "" {
				return errUnsupportedMediaStorage
			}
			var err error
			quarantined, err = quarantineLocalMediaFile(cfg.MediaLocalDir, locked.StorageKey)
			if err != nil {
				return err
			}
			if err := tx.Model(&locked).Updates(map[string]interface{}{"status": "deleted"}).Error; err != nil {
				return err
			}
			media = locked
			return tx.Delete(&locked).Error
		})
		if err != nil {
			if restoreErr := quarantined.restore(); restoreErr != nil {
				Fail(c, http.StatusInternalServerError, "MEDIA_RESTORE_FAILED", "could not restore media after database rollback")
				return
			}
			if errors.Is(err, gorm.ErrInvalidData) {
				Fail(c, http.StatusConflict, "CONFLICT", "media is still referenced")
				return
			}
			Fail(c, http.StatusConflict, "CONFLICT", "could not delete media")
			return
		}
		if err := quarantined.discard(); err != nil {
			Fail(c, http.StatusInternalServerError, "MEDIA_QUARANTINE_CLEANUP_FAILED", "media was revoked but quarantine cleanup failed")
			return
		}
		OK(c, gin.H{"deleted": true})
	}
}

func attachMediaUsageCounts(db *gorm.DB, media []model.MediaAsset) error {
	if len(media) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(media))
	index := make(map[uuid.UUID]int, len(media))
	for i := range media {
		ids = append(ids, media[i].ID)
		index[media[i].ID] = i
	}

	type row struct {
		MediaID uuid.UUID
		Count   int64
	}
	var rows []row
	if err := db.Model(&model.MediaUsage{}).
		Select("media_id, count(*) as count").
		Where("media_id in ?", ids).
		Group("media_id").
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, item := range rows {
		if i, ok := index[item.MediaID]; ok {
			media[i].UsageCount = item.Count
		}
	}
	return nil
}

func parseOptionalUUID(value string) *uuid.UUID {
	if value == "" {
		return nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func mediaFileServer(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimPrefix(c.Param("filepath"), "/")
		firstSegment := strings.SplitN(filepath.ToSlash(key), "/", 2)[0]
		if key == "" || strings.EqualFold(firstSegment, mediaPrivateDirName) || strings.Contains(key, "..") || strings.ContainsAny(key, `\:`) {
			c.Status(http.StatusNotFound)
			return
		}
		root := filepath.Clean(cfg.MediaLocalDir)
		candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(key)))
		if !isSafeMediaChild(root, candidate) {
			c.Status(http.StatusNotFound)
			return
		}
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil || !isSafeMediaChild(root, resolved) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.File(resolved)
	}
}

func isSafeMediaChild(root, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
