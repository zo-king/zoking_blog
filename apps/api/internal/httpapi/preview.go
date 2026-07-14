package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
)

var errPreviewTerminalTransition = errors.New("preview terminal state transition failed")

func previewPost(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		postID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post id")
			return
		}
		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "posts", contentAccessManage)
		if !ok {
			return
		}
		post, err := loadPostWithCoverAndTaxonomy(query, "id = ?", postID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load post")
			return
		}

		options := newPreviewOptions(c, cfg, "post", post.Slug)
		if _, err := startPreviewRecord(c, db, cfg, options, "post", publisher.PreviewTargetPath("post", post.Slug), &post.ID, nil); err != nil {
			Fail(c, http.StatusConflict, "PREVIEW_RECORD_FAILED", "could not create preview record")
			return
		}
		result, err := publisher.BuildPostPreview(c.Request.Context(), db, cfg, post, options)
		if err != nil {
			_ = failPreviewRecord(c, db, options.PreviewKey, "PREVIEW_BUILD_FAILED", err)
			Fail(c, http.StatusConflict, "PREVIEW_BUILD_FAILED", err.Error())
			return
		}
		if err := finishPreviewRecord(c, db, options.PreviewKey, result); err != nil {
			Fail(c, http.StatusConflict, "PREVIEW_RECORD_FAILED", "could not update preview record")
			return
		}
		OK(c, result)
	}
}

func previewPage(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid page id")
			return
		}
		var page model.Page
		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "pages", contentAccessManage)
		if !ok {
			return
		}
		if err := query.First(&page, "id = ?", pageID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
			return
		} else if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load page")
			return
		}

		options := newPreviewOptions(c, cfg, "page", page.Slug)
		if _, err := startPreviewRecord(c, db, cfg, options, "page", publisher.PreviewTargetPath("page", page.Slug), nil, &page.ID); err != nil {
			Fail(c, http.StatusConflict, "PREVIEW_RECORD_FAILED", "could not create preview record")
			return
		}
		result, err := publisher.BuildPagePreview(c.Request.Context(), db, cfg, page, options)
		if err != nil {
			_ = failPreviewRecord(c, db, options.PreviewKey, "PREVIEW_BUILD_FAILED", err)
			Fail(c, http.StatusConflict, "PREVIEW_BUILD_FAILED", err.Error())
			return
		}
		if err := finishPreviewRecord(c, db, options.PreviewKey, result); err != nil {
			Fail(c, http.StatusConflict, "PREVIEW_RECORD_FAILED", "could not update preview record")
			return
		}
		OK(c, result)
	}
}

func previewSiteSettings(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, _, err := publisher.LoadSiteSettings(c.Request.Context(), db)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "SETTINGS_LOAD_FAILED", "could not load site settings")
			return
		}
		if c.Request.ContentLength != 0 {
			var req siteSettingsPatchRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid settings payload")
				return
			}
			if _, err := applySettingsPatch(&settings, req); err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
				return
			}
			if err := validateSiteSettings(settings); err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
				return
			}
		}

		options := newPreviewOptions(c, cfg, "site", "")
		options.Settings = &settings
		if _, err := startPreviewRecord(c, db, cfg, options, "site", publisher.PreviewTargetPath("site", ""), nil, nil); err != nil {
			Fail(c, http.StatusConflict, "PREVIEW_RECORD_FAILED", "could not create preview record")
			return
		}
		result, err := publisher.BuildSitePreview(c.Request.Context(), db, cfg, options)
		if err != nil {
			_ = failPreviewRecord(c, db, options.PreviewKey, "PREVIEW_BUILD_FAILED", err)
			Fail(c, http.StatusConflict, "PREVIEW_BUILD_FAILED", err.Error())
			return
		}
		if err := finishPreviewRecord(c, db, options.PreviewKey, result); err != nil {
			Fail(c, http.StatusConflict, "PREVIEW_RECORD_FAILED", "could not update preview record")
			return
		}
		OK(c, result)
	}
}

func listPublishPreviews(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}
		order, ok := parseListOrder(c, pagination.Sort, map[string]string{
			"created_at":  "created_at",
			"preview_key": "preview_key",
			"scope":       "scope",
			"status":      "status",
			"finished_at": "finished_at",
			"expires_at":  "expires_at",
		})
		if !ok {
			return
		}

		query := db.WithContext(c.Request.Context()).Model(&model.PublishPreview{})
		query, ok = scopePublishQuery(c, query, "publish_previews", contentAccessRead)
		if !ok {
			return
		}
		if pagination.Query != "" {
			pattern := "%" + pagination.Query + "%"
			query = query.Where("preview_key ILIKE ? OR error_code ILIKE ? OR error_message ILIKE ?", pattern, pattern, pattern)
		}
		if pagination.Status != "" {
			query = query.Where("status = ?", pagination.Status)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count publish previews")
			return
		}
		if returnEmptyPageIfOutOfRange[model.PublishPreview](c, total, pagination) {
			return
		}

		var previews []model.PublishPreview
		if err := query.
			Preload("Post", func(db *gorm.DB) *gorm.DB { return db.Select("id", "title", "slug") }).
			Preload("Page", func(db *gorm.DB) *gorm.DB { return db.Select("id", "title", "slug") }).
			Order(order + ", id ASC").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&previews).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list publish previews")
			return
		}
		OKPaginated(c, previews, total, pagination)
	}
}

func newPreviewOptions(c *gin.Context, cfg config.Config, scope string, slug string) publisher.PreviewOptions {
	key := publisher.NewPreviewKey(time.Now())
	baseURL := previewBaseURL(c, cfg, key)
	targetPath := publisher.PreviewTargetPath(scope, slug)
	targetURL := baseURL
	if targetPath != "/" {
		targetURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimPrefix(targetPath, "/")
	}
	return publisher.PreviewOptions{
		PreviewKey: key,
		BaseURL:    baseURL,
		URL:        baseURL,
		TargetURL:  targetURL,
	}
}

func previewBaseURL(c *gin.Context, cfg config.Config, key string) string {
	publicBase := strings.TrimSpace(cfg.PublishPreviewPublicBaseURL)
	if publicBase == "" {
		publicBase = "/preview-files"
	}
	if strings.HasPrefix(publicBase, "http://") || strings.HasPrefix(publicBase, "https://") {
		return strings.TrimRight(publicBase, "/") + "/" + key + "/"
	}
	publicBase = path.Clean("/" + strings.Trim(publicBase, "/"))
	if publicBase == "/" {
		return requestOrigin(c, cfg) + "/" + key + "/"
	}
	return requestOrigin(c, cfg) + publicBase + "/" + key + "/"
}

func requestOrigin(c *gin.Context, cfg config.Config) string {
	proto := "http"
	if c.Request.TLS != nil {
		proto = "https"
	}
	host := c.Request.Host
	if immediatePeerTrusted(c.Request.RemoteAddr, cfg.TrustedProxies) {
		if forwardedProto := strings.ToLower(firstForwardedValue(c.GetHeader("X-Forwarded-Proto"))); forwardedProto == "http" || forwardedProto == "https" {
			proto = forwardedProto
		}
		if forwardedHost := firstForwardedValue(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
			host = forwardedHost
		}
	}
	return proto + "://" + host
}

func firstForwardedValue(value string) string {
	value, _, _ = strings.Cut(value, ",")
	return strings.TrimSpace(value)
}

func immediatePeerTrusted(remoteAddr string, trustedProxies string) bool {
	peerIP := immediatePeerIP(remoteAddr)
	if peerIP == nil {
		return false
	}
	for _, raw := range strings.Split(trustedProxies, ",") {
		trusted := strings.TrimSpace(raw)
		if trusted == "" {
			continue
		}
		if trustedIP := net.ParseIP(trusted); trustedIP != nil {
			if trustedIP.Equal(peerIP) {
				return true
			}
			continue
		}
		_, network, err := net.ParseCIDR(trusted)
		if err == nil && network.Contains(peerIP) {
			return true
		}
	}
	return false
}

func immediatePeerIP(remoteAddr string) net.IP {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = strings.Trim(remoteAddr, "[]")
	}
	if zoneIndex := strings.LastIndex(host, "%"); zoneIndex >= 0 {
		host = host[:zoneIndex]
	}
	return net.ParseIP(host)
}

func startPreviewRecord(c *gin.Context, db *gorm.DB, cfg config.Config, options publisher.PreviewOptions, scope string, entryPath string, postID *uuid.UUID, pageID *uuid.UUID) (model.PublishPreview, error) {
	now := time.Now()
	var expiresAt *time.Time
	if cfg.PublishPreviewTTL > 0 {
		value := now.Add(cfg.PublishPreviewTTL)
		expiresAt = &value
	}
	preview := model.PublishPreview{
		PreviewKey:   options.PreviewKey,
		Scope:        scope,
		Status:       "building",
		PostID:       postID,
		PageID:       pageID,
		RequestedBy:  parseOptionalUUID(c.GetString("user_id")),
		EntryPath:    entryPath,
		URL:          options.URL,
		TargetURL:    options.TargetURL,
		ManifestJSON: []byte(`{}`),
		LogJSON:      []byte(`[]`),
		StartedAt:    &now,
		ExpiresAt:    expiresAt,
	}
	err := db.WithContext(c.Request.Context()).Create(&preview).Error
	return preview, err
}

func finishPreviewRecord(c *gin.Context, db *gorm.DB, previewKey string, result publisher.PreviewResult) error {
	now := time.Now()
	manifestJSON, err := json.Marshal(result.Manifest)
	if err != nil {
		return err
	}
	update := db.WithContext(c.Request.Context()).
		Model(&model.PublishPreview{}).
		Where("preview_key = ? and status = ?", previewKey, "building").
		Updates(map[string]interface{}{
			"status":        "ready",
			"output_path":   result.OutputPath,
			"entry_path":    result.TargetPath,
			"url":           result.URL,
			"target_url":    result.TargetURL,
			"settings_hash": result.Manifest.SettingsHash,
			"content_hash":  result.Manifest.ContentHash,
			"manifest_json": manifestJSON,
			"log_json":      []byte(`[{"stage":"preview","level":"info","message":"preview build ready"}]`),
			"finished_at":   now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return errPreviewTerminalTransition
	}
	return nil
}

func failPreviewRecord(c *gin.Context, db *gorm.DB, previewKey string, code string, err error) error {
	now := time.Now()
	update := db.WithContext(c.Request.Context()).
		Model(&model.PublishPreview{}).
		Where("preview_key = ? and status = ?", previewKey, "building").
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_code":    code,
			"error_message": err.Error(),
			"log_json":      []byte(`[{"stage":"preview","level":"error","message":"preview build failed"}]`),
			"finished_at":   now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return errPreviewTerminalTransition
	}
	return nil
}

func previewFileServer(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !previewRequestHostAllowed(c, cfg) {
			c.Status(http.StatusNotFound)
			return
		}
		key := strings.TrimPrefix(c.Param("filepath"), "/")
		if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `\:`) {
			c.Status(http.StatusNotFound)
			return
		}
		previewKey := strings.SplitN(key, "/", 2)[0]
		var preview model.PublishPreview
		if err := db.WithContext(c.Request.Context()).Where("preview_key = ? and status = ? and (expires_at is null or expires_at > ?)", previewKey, "ready", time.Now()).First(&preview).Error; err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		absPath, ok := resolvePreviewFile(cfg.PublishPreviewRoot, key)
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("X-Robots-Tag", "noindex, nofollow")
		c.Header("Cache-Control", "private, no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; script-src 'none'; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; sandbox allow-same-origin")
		c.File(absPath)
	}
}

func previewRequestHostAllowed(c *gin.Context, cfg config.Config) bool {
	if cfg.AppEnv == "development" || cfg.AppEnv == "dev" || cfg.AppEnv == "test" {
		return true
	}
	previewURL, err := url.Parse(strings.TrimSpace(cfg.PublishPreviewPublicBaseURL))
	if err != nil || previewURL.Host == "" {
		return false
	}
	requestURL, err := url.Parse(requestOrigin(c, cfg))
	return err == nil && strings.EqualFold(requestURL.Host, previewURL.Host)
}

func resolvePreviewFile(root string, key string) (string, bool) {
	if strings.ContainsAny(key, `\:`) {
		return "", false
	}
	key = strings.TrimPrefix(filepath.ToSlash(key), "/")
	if strings.TrimSpace(root) == "" || key == "" || strings.Contains(key, "..") {
		return "", false
	}
	parts := strings.SplitN(key, "/", 2)
	previewKey := parts[0]
	if publisher.ValidateSlug(previewKey) != nil {
		return "", false
	}
	if len(parts) == 2 && strings.EqualFold(strings.Trim(parts[1], "/"), "manifest.json") {
		return "", false
	}

	root = filepath.Clean(root)
	previewDir := filepath.Join(root, previewKey)
	if !isSafePreviewChild(root, previewDir) {
		return "", false
	}
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(key)))
	if candidate != previewDir && !isSafePreviewChild(previewDir, candidate) {
		return "", false
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		candidate = filepath.Join(candidate, "index.html")
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil || !isSafePreviewChild(previewDir, resolved) {
		return "", false
	}
	info, err = os.Stat(resolved)
	if err != nil || info.IsDir() {
		return "", false
	}
	return resolved, true
}

func isSafePreviewChild(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	return rel != "" && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
