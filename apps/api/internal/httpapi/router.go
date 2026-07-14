package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/auth"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/contentquality"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	adminAccessCookieName = "zoking_admin_access"
	adminCSRFCookieName   = "zoking_admin_csrf"
	adminCookiePath       = "/api/v1/admin"
)

func NewRouter(db *gorm.DB, cfg config.Config) *gin.Engine {
	if cfg.AppEnv != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	trustedProxies := splitCSV(cfg.TrustedProxies)
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		panic("invalid TRUSTED_PROXIES: " + err.Error())
	}
	router.Use(requestIDMiddleware(), recoverMiddleware(), securityHeadersMiddleware(), requestBodyLimitMiddleware(cfg.RequestMaxBytes), corsMiddleware(cfg))
	router.GET(publicRoutePath(cfg.MediaPublicBaseURL)+"/*filepath", mediaFileServer(cfg))
	router.GET(publicRoutePath(cfg.PublishPreviewPublicBaseURL)+"/*filepath", previewFileServer(db, cfg))

	router.GET("/healthz", func(c *gin.Context) {
		OK(c, gin.H{"status": "ok"})
	})

	router.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			Fail(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database handle unavailable")
			return
		}
		if err := sqlDB.PingContext(c.Request.Context()); err != nil {
			Fail(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database ping failed")
			return
		}
		OK(c, gin.H{"status": "ready"})
	})

	api := router.Group("/api/v1")
	public := api.Group("/public")
	public.GET("/posts", listPublicPosts(db))
	public.GET("/posts/:slug", getPublicPost(db))
	public.GET("/posts/:slug/comments", listPublicComments(db))
	public.POST("/posts/:slug/comments", commentRateLimitMiddleware(cfg), submitPublicComment(db))
	public.GET("/pages", listPublicPages(db))
	public.GET("/pages/:slug", getPublicPage(db))
	public.GET("/categories", listCategories(db))
	public.GET("/tags", listTags(db))
	public.GET("/series", listPublicSeries(db))
	public.GET("/series/:slug", getPublicSeries(db))
	public.GET("/site/public-settings", getPublicSiteSettings(db))

	admin := api.Group("/admin")
	admin.Use(adminOriginMiddleware(cfg))
	admin.POST("/auth/login", login(db, cfg, newAdminLoginRateLimiter(cfg)))
	admin.POST("/auth/session", authMiddleware(cfg), resumeAdminSession(cfg))

	authed := admin.Group("")
	authed.Use(authMiddleware(cfg))
	authed.Use(csrfMiddleware(cfg))
	authed.Use(auditMiddleware(db, cfg))
	authed.Use(authorizationMiddleware(db))
	authed.POST("/auth/logout", logout(cfg))
	authed.GET("/auth/me", func(c *gin.Context) {
		OK(c, gin.H{
			"id":          c.GetString("user_id"),
			"email":       c.GetString("user_email"),
			"roles":       c.MustGet("roles"),
			"permissions": c.MustGet("permissions"),
		})
	})
	authed.GET("/posts", listAdminPosts(db))
	authed.POST("/posts/quality-check", qualityCheckNewPost())
	authed.POST("/posts", createPost(db))
	authed.GET("/posts/:id", getAdminPost(db))
	authed.POST("/posts/:id/quality-check", qualityCheckPost(db))
	authed.PATCH("/posts/:id", updatePost(db))
	authed.DELETE("/posts/:id", deletePost(db))
	authed.POST("/posts/:id/preview", previewPost(db, cfg))
	authed.POST("/posts/:id/publish", publishPost(db, cfg))
	authed.GET("/pages", listAdminPages(db))
	authed.POST("/pages/quality-check", qualityCheckNewPage())
	authed.POST("/pages", createPage(db))
	authed.GET("/pages/:id", getAdminPage(db))
	authed.POST("/pages/:id/quality-check", qualityCheckPage(db))
	authed.PATCH("/pages/:id", updatePage(db))
	authed.POST("/pages/:id/preview", previewPage(db, cfg))
	authed.POST("/pages/:id/publish", publishPage(db))
	authed.DELETE("/pages/:id", deletePage(db))
	authed.GET("/categories", listAdminCategories(db))
	authed.POST("/categories", createCategory(db))
	authed.PATCH("/categories/:id", updateCategory(db))
	authed.DELETE("/categories/:id", deleteCategory(db))
	authed.GET("/tags", listAdminTags(db))
	authed.POST("/tags", createTag(db))
	authed.PATCH("/tags/:id", updateTag(db))
	authed.DELETE("/tags/:id", deleteTag(db))
	authed.GET("/series", listAdminSeries(db))
	authed.POST("/series", createSeries(db))
	authed.GET("/series/:id", getAdminSeries(db))
	authed.PATCH("/series/:id", updateSeries(db))
	authed.DELETE("/series/:id", deleteSeries(db))
	authed.GET("/achievements", listAdminAchievements(db))
	authed.POST("/achievements", createAchievement(db))
	authed.GET("/achievements/:id", getAdminAchievement(db))
	authed.PATCH("/achievements/:id", updateAchievement(db))
	authed.PATCH("/achievements/:id/status", updateAchievementStatus(db))
	authed.DELETE("/achievements/:id", deleteAchievement(db))
	authed.POST("/achievements/publish", publishAchievements(db))
	authed.GET("/media", listAdminMedia(db))
	authed.POST("/media", uploadMedia(db, cfg))
	authed.POST("/media/cleanup", cleanupOrphanMedia(db, cfg))
	authed.GET("/media/:id", getAdminMedia(db))
	authed.DELETE("/media/:id", deleteMedia(db, cfg))
	authed.GET("/comments", listAdminComments(db))
	authed.PATCH("/comments/:id/moderation", moderateComment(db))
	authed.POST("/comments/:id/reply", replyComment(db))
	authed.DELETE("/comments/:id", deleteComment(db))
	authed.GET("/publish/previews", listPublishPreviews(db))
	authed.POST("/publish/previews/cleanup", cleanupPublishPreviews(db, cfg))
	authed.GET("/audit-logs", listAuditLogs(db))
	authed.GET("/users", listAdminUsers(db))
	authed.POST("/users", createAdminUser(db))
	authed.PATCH("/users/:id/status", updateAdminUserStatus(db))
	authed.PATCH("/users/:id/roles", updateAdminUserRoles(db))
	authed.POST("/users/:id/reset-password", resetAdminUserPassword(db))
	authed.GET("/roles", listAdminRoles(db))
	authed.POST("/roles", createAdminRole(db))
	authed.PATCH("/roles/:id", updateAdminRole(db))
	authed.PATCH("/roles/:id/permissions", updateAdminRolePermissions(db))
	authed.DELETE("/roles/:id", deleteAdminRole(db))
	authed.GET("/permissions", listAdminPermissions(db))
	authed.GET("/publish/jobs", listPublishJobs(db))
	authed.GET("/publish/jobs/:id", getPublishJob(db))
	authed.GET("/publish/releases", listPublishReleases(db))
	authed.POST("/publish/releases/cleanup", cleanupPublishReleases(db, cfg))
	authed.POST("/publish/releases/:id/promote", promotePublishRelease(db, cfg))
	authed.POST("/publish/jobs/:id/retry", retryPublishJob(db, cfg))
	authed.POST("/publish/jobs/:id/cancel", cancelPublishJob(db))
	authed.GET("/settings", getAdminSiteSettings(db))
	authed.PATCH("/settings", patchAdminSiteSettings(db))
	authed.POST("/settings/preview", previewSiteSettings(db, cfg))
	authed.POST("/settings/publish", publishAdminSiteSettings(db))
	if e2eCleanupEnabled(cfg) {
		authed.POST("/qa/e2e-runs/:run_id/cleanup", cleanupE2ERun(db, cfg))
	}

	return router
}

func publicRoutePath(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	value = strings.TrimRight(value, "/")
	if value == "" {
		return "/"
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func login(db *gorm.DB, cfg config.Config, limiter *adminLoginRateLimiter) gin.HandlerFunc {
	type request struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid login payload")
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))
		if !limiter.allow(c.ClientIP(), email) {
			c.Header("Retry-After", "60")
			Fail(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many login attempts")
			return
		}

		var user model.User
		if err := db.WithContext(c.Request.Context()).Where("email = ?", email).First(&user).Error; err != nil {
			Fail(c, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "invalid credentials")
			return
		}
		if user.Status != "active" || !auth.CheckPassword(user.PasswordHash, req.Password) {
			Fail(c, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "invalid credentials")
			return
		}

		token, err := auth.GenerateAccessToken(cfg.JWTSecret, user.ID.String(), user.Email, cfg.AccessTokenTTL)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create token")
			return
		}
		csrfToken, err := newCSRFToken()
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create session")
			return
		}
		now := time.Now()
		_ = db.WithContext(c.Request.Context()).Model(&user).Update("last_login_at", now).Error

		secureCookie := secureAdminCookie(cfg)
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     adminAccessCookieName,
			Value:    token,
			Path:     adminCookiePath,
			MaxAge:   int(cfg.AccessTokenTTL.Seconds()),
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteStrictMode,
		})
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     adminCSRFCookieName,
			Value:    csrfToken,
			Path:     adminCookiePath,
			MaxAge:   int(cfg.AccessTokenTTL.Seconds()),
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteStrictMode,
		})
		OK(c, gin.H{
			"csrf_token": csrfToken,
			"token_type": "Cookie",
			"expires_in": int(cfg.AccessTokenTTL.Seconds()),
			"user": gin.H{
				"id":           user.ID,
				"email":        user.Email,
				"display_name": user.DisplayName,
			},
		})
	}
}

func resumeAdminSession(cfg config.Config) gin.HandlerFunc {
	allowedOrigins := originAllowlist(cfg.AdminAllowedOrigins)
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" || !allowedOrigins[origin] || c.GetString("auth_transport") != "cookie" {
			Fail(c, http.StatusForbidden, "SESSION_RESUME_FORBIDDEN", "admin session cannot be resumed")
			return
		}
		csrfToken, err := newCSRFToken()
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not resume session")
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     adminCSRFCookieName,
			Value:    csrfToken,
			Path:     adminCookiePath,
			MaxAge:   int(cfg.AccessTokenTTL.Seconds()),
			HttpOnly: true,
			Secure:   secureAdminCookie(cfg),
			SameSite: http.SameSiteStrictMode,
		})
		OK(c, gin.H{
			"csrf_token": csrfToken,
			"token_type": "Cookie",
			"expires_in": int(cfg.AccessTokenTTL.Seconds()),
		})
	}
}

func logout(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, name := range []string{adminAccessCookieName, adminCSRFCookieName} {
			http.SetCookie(c.Writer, &http.Cookie{
				Name:     name,
				Value:    "",
				Path:     adminCookiePath,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   secureAdminCookie(cfg),
				SameSite: http.SameSiteStrictMode,
			})
		}
		OK(c, gin.H{"logged_out": true})
	}
}

func secureAdminCookie(cfg config.Config) bool {
	return cfg.AppEnv != "development" && cfg.AppEnv != "dev" && cfg.AppEnv != "test"
}

func newCSRFToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func listPublicPosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePublicPagination(c, publicListLimit)
		if !ok {
			return
		}
		baseQuery := db.WithContext(c.Request.Context()).Model(&model.Post{}).
			Where("status = ? and visibility = ?", "published", "public")
		var total int64
		if err := baseQuery.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list posts")
			return
		}
		var posts []model.Post
		err := preloadPostTaxonomy(db.WithContext(c.Request.Context())).
			Preload("CoverMedia").
			Where("status = ? and visibility = ?", "published", "public").
			Order("published_at desc nulls last, id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&posts).Error
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list posts")
			return
		}
		items := make([]publicPostDTO, 0, len(posts))
		for _, post := range posts {
			items = append(items, newPublicPostSummaryDTO(post))
		}
		setPublicPaginationHeaders(c, total, pagination)
		OK(c, items)
	}
}

func getPublicPost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var post model.Post
		err := preloadPostTaxonomy(db.WithContext(c.Request.Context())).
			Preload("CoverMedia").
			Where("slug = ? and status = ? and visibility = ?", c.Param("slug"), "published", "public").
			First(&post).Error
		if err != nil {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}
		OK(c, newPublicPostDTO(post))
	}
}

func listAdminPosts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}

		order, ok := map[string]string{
			"":             "posts.created_at desc",
			"-created_at":  "posts.created_at desc",
			"created_at":   "posts.created_at asc",
			"published_at": "posts.published_at asc",
			"title":        "posts.title asc",
		}[pagination.Sort]
		if !ok {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
			return
		}

		var posts []model.Post
		query := preloadPostTaxonomy(db.WithContext(c.Request.Context())).
			Model(&model.Post{}).
			Preload("CoverMedia").
			Distinct("posts.*")
		query, ok = scopeContentQuery(c, query, "posts", contentAccessRead)
		if !ok {
			return
		}

		if pagination.Status != "" {
			query = query.Where("posts.status = ?", pagination.Status)
		}
		if pagination.Query != "" {
			like := "%" + pagination.Query + "%"
			query = query.Where("(posts.title ILIKE ? OR posts.summary ILIKE ? OR posts.slug ILIKE ?)", like, like, like)
		}
		if slug := strings.TrimSpace(c.Query("slug")); slug != "" {
			if err := publisher.ValidateSlug(slug); err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post slug")
				return
			}
			query = query.Where("slug = ?", slug)
		}
		if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("title ILIKE ? OR summary ILIKE ? OR content_md ILIKE ?", like, like, like)
		}
		if categoryID := strings.TrimSpace(c.Query("category_id")); categoryID != "" {
			parsedCategoryID, err := uuid.Parse(categoryID)
			if err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category_id")
				return
			}
			query = query.Joins("JOIN post_categories pc ON pc.post_id = posts.id").Where("pc.category_id = ?", parsedCategoryID)
		}
		if tagID := strings.TrimSpace(c.Query("tag_id")); tagID != "" {
			parsedTagID, err := uuid.Parse(tagID)
			if err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid tag_id")
				return
			}
			query = query.Joins("JOIN post_tags pt ON pt.post_id = posts.id").Where("pt.tag_id = ?", parsedTagID)
		}
		if seriesID := strings.TrimSpace(c.Query("series_id")); seriesID != "" {
			parsedSeriesID, err := uuid.Parse(seriesID)
			if err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series_id")
				return
			}
			query = query.Where("posts.series_id = ?", parsedSeriesID)
		}

		var total int64
		if err := query.Session(&gorm.Session{}).Distinct("posts.id").Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list posts")
			return
		}
		if returnEmptyPageIfOutOfRange[model.Post](c, total, pagination) {
			return
		}
		if err := query.
			Order(order + ", posts.id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&posts).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list posts")
			return
		}
		OKPaginated(c, posts, total, pagination)
	}
}

func getAdminPost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var post model.Post
		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "posts", contentAccessRead)
		if !ok {
			return
		}
		if err := preloadPostTaxonomy(query).
			Preload("CoverMedia").
			First(&post, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}
		OK(c, post)
	}
}

var errInvalidPostStatus = errors.New("invalid post status")

func editablePostStatus(value string, defaultDraft bool) (string, error) {
	status := strings.TrimSpace(value)
	if status == "" && defaultDraft {
		return "draft", nil
	}
	switch status {
	case "draft", "offline", "archived":
		return status, nil
	case "published":
		return "", errPublishEndpointRequired
	default:
		return "", errInvalidPostStatus
	}
}

func createPost(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Title          string       `json:"title" binding:"required"`
		Slug           string       `json:"slug" binding:"required"`
		Summary        string       `json:"summary"`
		ContentMD      string       `json:"content_md"`
		Status         string       `json:"status"`
		Visibility     string       `json:"visibility"`
		AllowComment   *bool        `json:"allow_comment"`
		SEOTitle       string       `json:"seo_title"`
		SEODescription string       `json:"seo_description"`
		CategoryIDs    *[]uuid.UUID `json:"category_ids"`
		TagIDs         *[]uuid.UUID `json:"tag_ids"`
		CoverMediaID   *string      `json:"cover_media_id"`
		PublishedAt    *time.Time   `json:"published_at"`
		SeriesID       rawJSON      `json:"series_id"`
		SeriesOrder    rawJSON      `json:"series_order"`
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post payload")
			return
		}
		title := strings.TrimSpace(req.Title)
		slug := strings.TrimSpace(req.Slug)
		if title == "" || slug == "" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post payload")
			return
		}
		if err := publisher.ValidateSlug(slug); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post slug")
			return
		}
		_, seriesID, seriesOrder, err := parsePostSeriesFields(req.SeriesID, req.SeriesOrder)
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}

		status, err := editablePostStatus(req.Status, true)
		if errors.Is(err, errPublishEndpointRequired) {
			Fail(c, http.StatusUnprocessableEntity, "PUBLISH_ENDPOINT_REQUIRED", "use the post publish endpoint to publish content")
			return
		}
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post status")
			return
		}

		visibility := strings.TrimSpace(req.Visibility)
		if visibility == "" {
			visibility = "public"
		}
		allowComment := true
		if req.AllowComment != nil {
			allowComment = *req.AllowComment
		}
		seoTitle := strings.TrimSpace(req.SEOTitle)
		if seoTitle == "" {
			seoTitle = title
		}
		seoDescription := strings.TrimSpace(req.SEODescription)
		if seoDescription == "" {
			seoDescription = strings.TrimSpace(req.Summary)
		}
		authorID, ok := currentContentUserID(c)
		if !ok {
			return
		}

		var created model.Post
		if err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := ensureSeriesExists(tx, seriesID); err != nil {
				return err
			}
			var coverMediaID *uuid.UUID
			if req.CoverMediaID != nil && strings.TrimSpace(*req.CoverMediaID) != "" {
				asset, err := mediaref.ResolveReadyMedia(tx, *req.CoverMediaID)
				if err != nil {
					return err
				}
				coverMediaID = &asset.ID
			}
			post := model.Post{
				Title:          title,
				Slug:           slug,
				Summary:        strings.TrimSpace(req.Summary),
				ContentMD:      req.ContentMD,
				Status:         status,
				Visibility:     visibility,
				AllowComment:   allowComment,
				SEOTitle:       seoTitle,
				SEODescription: seoDescription,
				AuthorID:       &authorID,
				CoverMediaID:   coverMediaID,
				PublishedAt:    req.PublishedAt,
				SeriesID:       seriesID,
				SeriesOrder:    seriesOrder,
			}
			if err := tx.Create(&post).Error; err != nil {
				return err
			}
			if err := syncPostTaxonomy(tx, &post, req.CategoryIDs, req.TagIDs); err != nil {
				return err
			}
			if err := mediaref.SyncPostMarkdownUsages(tx, post.ID, post.ContentMD); err != nil {
				return err
			}
			if err := mediaref.SyncPostCoverUsage(tx, post.ID, post.CoverMediaID); err != nil {
				return err
			}
			created = post
			return nil
		}); err != nil {
			if postgresConstraint(err) == "posts_series_order_active_idx" {
				Fail(c, http.StatusConflict, "SERIES_ORDER_CONFLICT", "series order is already assigned")
				return
			}
			if postgresConstraint(err) == "posts_slug_active_idx" {
				Fail(c, http.StatusConflict, "POST_SLUG_CONFLICT", "could not create post")
				return
			}
			if errors.Is(err, ErrSeriesNotFound) {
				Fail(c, http.StatusNotFound, "SERIES_NOT_FOUND", "series not found")
				return
			}
			if errors.Is(err, ErrCategoriesNotFound) || errors.Is(err, ErrTagsNotFound) {
				Fail(c, http.StatusNotFound, "NOT_FOUND", err.Error())
				return
			}
			if errors.Is(err, mediaref.ErrMediaNotFound) {
				Fail(c, http.StatusNotFound, "MEDIA_NOT_FOUND", "cover media not found")
				return
			}
			Fail(c, http.StatusConflict, "CONFLICT", "could not create post")
			return
		}

		post, err := loadPostWithCoverAndTaxonomy(db.WithContext(c.Request.Context()), "id = ?", created.ID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load created post")
			return
		}
		Created(c, post)
	}
}

func updatePost(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Title          *string      `json:"title"`
		Slug           *string      `json:"slug"`
		Summary        *string      `json:"summary"`
		ContentMD      *string      `json:"content_md"`
		Status         *string      `json:"status"`
		Visibility     *string      `json:"visibility"`
		AllowComment   *bool        `json:"allow_comment"`
		SEOTitle       *string      `json:"seo_title"`
		SEODescription *string      `json:"seo_description"`
		CategoryIDs    *[]uuid.UUID `json:"category_ids"`
		TagIDs         *[]uuid.UUID `json:"tag_ids"`
		CoverMediaID   *string      `json:"cover_media_id"`
		PublishedAt    *time.Time   `json:"published_at"`
		SeriesID       rawJSON      `json:"series_id"`
		SeriesOrder    rawJSON      `json:"series_order"`
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post payload")
			return
		}

		if req.Slug != nil {
			slug := strings.TrimSpace(*req.Slug)
			if err := publisher.ValidateSlug(slug); err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post slug")
				return
			}
		}
		var requestedStatus *string
		if req.Status != nil {
			status, err := editablePostStatus(*req.Status, false)
			if errors.Is(err, errPublishEndpointRequired) {
				Fail(c, http.StatusUnprocessableEntity, "PUBLISH_ENDPOINT_REQUIRED", "use the post publish endpoint to publish content")
				return
			}
			if err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post status")
				return
			}
			requestedStatus = &status
		}
		seriesChanged, seriesID, seriesOrder, err := parsePostSeriesFields(req.SeriesID, req.SeriesOrder)
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}

		var post model.Post
		accessOK := true
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			query, ok := scopeContentQuery(c, tx, "posts", contentAccessManage)
			if !ok {
				accessOK = false
				return nil
			}
			if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).First(&post, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			if post.Status == "published" && !containsPermission(c.GetStringSlice("permissions"), "post:publish") {
				return errPublishedContentPermission
			}
			if requestedStatus != nil {
				status := *requestedStatus
				if status != post.Status && post.Status == "published" && !containsPermission(c.GetStringSlice("permissions"), "post:publish") {
					return errPublishedContentPermission
				}
			}
			inProgress, err := contentPublishInProgress(tx, "post_id", post.ID)
			if err != nil {
				return err
			}
			if inProgress {
				return errContentPublishInProgress
			}
			updates := map[string]interface{}{}
			if seriesChanged {
				if err := ensureSeriesExists(tx, seriesID); err != nil {
					return err
				}
				updates["series_id"] = seriesID
				updates["series_order"] = seriesOrder
			}
			var coverMediaID *uuid.UUID
			coverMediaChanged := req.CoverMediaID != nil
			if coverMediaChanged && strings.TrimSpace(*req.CoverMediaID) != "" {
				asset, err := mediaref.ResolveReadyMedia(tx, *req.CoverMediaID)
				if err != nil {
					return err
				}
				coverMediaID = &asset.ID
			}
			if coverMediaChanged {
				updates["cover_media_id"] = coverMediaID
			}
			if req.Title != nil {
				title := strings.TrimSpace(*req.Title)
				updates["title"] = title
				if req.SEOTitle == nil {
					updates["seo_title"] = title
				}
			}
			if req.Slug != nil {
				updates["slug"] = strings.TrimSpace(*req.Slug)
			}
			if req.Summary != nil {
				updates["summary"] = strings.TrimSpace(*req.Summary)
			}
			if req.ContentMD != nil {
				updates["content_md"] = *req.ContentMD
			}
			if requestedStatus != nil {
				updates["status"] = *requestedStatus
			}
			if req.PublishedAt != nil {
				updates["published_at"] = *req.PublishedAt
			}
			if req.Visibility != nil {
				updates["visibility"] = strings.TrimSpace(*req.Visibility)
			}
			if req.AllowComment != nil {
				updates["allow_comment"] = *req.AllowComment
			}
			if req.SEOTitle != nil {
				updates["seo_title"] = strings.TrimSpace(*req.SEOTitle)
			}
			if req.SEODescription != nil {
				updates["seo_description"] = strings.TrimSpace(*req.SEODescription)
			}

			if len(updates) > 0 {
				if err := tx.Model(&post).Updates(updates).Error; err != nil {
					return err
				}
			}
			if err := syncPostTaxonomy(tx, &post, req.CategoryIDs, req.TagIDs); err != nil {
				return err
			}
			var refreshed model.Post
			if err := tx.First(&refreshed, "id = ?", post.ID).Error; err != nil {
				return err
			}
			if err := mediaref.SyncPostMarkdownUsages(tx, post.ID, refreshed.ContentMD); err != nil {
				return err
			}
			if coverMediaChanged {
				if err := mediaref.SyncPostCoverUsage(tx, post.ID, refreshed.CoverMediaID); err != nil {
					return err
				}
			}
			return nil
		})
		if !accessOK {
			return
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
				return
			}
			if errors.Is(err, errContentPublishInProgress) {
				Fail(c, http.StatusConflict, "CONTENT_PUBLISH_IN_PROGRESS", "post has a publish job in progress")
				return
			}
			if errors.Is(err, errPublishEndpointRequired) {
				Fail(c, http.StatusUnprocessableEntity, "PUBLISH_ENDPOINT_REQUIRED", "use the post publish endpoint to publish content")
				return
			}
			if errors.Is(err, errPublishedContentPermission) {
				Fail(c, http.StatusForbidden, "FORBIDDEN", "permission required: post:publish")
				return
			}
			if postgresConstraint(err) == "posts_series_order_active_idx" {
				Fail(c, http.StatusConflict, "SERIES_ORDER_CONFLICT", "series order is already assigned")
				return
			}
			if postgresConstraint(err) == "posts_slug_active_idx" {
				Fail(c, http.StatusConflict, "POST_SLUG_CONFLICT", "could not update post")
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "invalid slug") {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post slug")
				return
			}
			if errors.Is(err, ErrCategoriesNotFound) || errors.Is(err, ErrTagsNotFound) {
				Fail(c, http.StatusNotFound, "NOT_FOUND", err.Error())
				return
			}
			if errors.Is(err, ErrSeriesNotFound) {
				Fail(c, http.StatusNotFound, "SERIES_NOT_FOUND", "series not found")
				return
			}
			if errors.Is(err, mediaref.ErrMediaNotFound) {
				Fail(c, http.StatusNotFound, "MEDIA_NOT_FOUND", "cover media not found")
				return
			}
			Fail(c, http.StatusConflict, "POST_INVALID_STATUS", "could not update post")
			return
		}

		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "posts", contentAccessManage)
		if !ok {
			return
		}
		updated, err := loadPostWithCoverAndTaxonomy(query, "id = ?", post.ID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load updated post")
			return
		}
		OK(c, updated)
	}
}

func loadPostWithCoverAndTaxonomy(db *gorm.DB, query string, args ...interface{}) (model.Post, error) {
	var post model.Post
	conds := append([]interface{}{query}, args...)
	err := preloadPostTaxonomy(db).Preload("CoverMedia").First(&post, conds...).Error
	return post, err
}

func publishPost(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var post model.Post
		var job model.PublishJob
		var blockedReport *contentquality.Report
		accessOK := true
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			query, ok := scopeContentQuery(c, tx, "posts", contentAccessManage)
			if !ok {
				accessOK = false
				return nil
			}
			var err error
			post, err = loadPostWithCoverAndTaxonomy(query.Clauses(clause.Locking{Strength: "UPDATE"}), "id = ?", c.Param("id"))
			if err != nil {
				return err
			}
			inProgress, err := contentPublishInProgress(tx, "post_id", post.ID)
			if err != nil {
				return err
			}
			if inProgress {
				return errContentPublishInProgress
			}
			report := evaluatePostWithSlugError(post)
			if !report.Ready {
				blockedReport = &report
				return errContentQualityBlocked
			}

			publishedAt := time.Now()
			if post.PublishedAt != nil {
				publishedAt = *post.PublishedAt
			}
			if err := tx.Model(&post).Updates(map[string]interface{}{
				"status": "published", "published_at": publishedAt,
			}).Error; err != nil {
				return err
			}
			post.Status = "published"
			post.PublishedAt = &publishedAt
			if err := mediaref.SyncPostMarkdownUsages(tx, post.ID, post.ContentMD); err != nil {
				return err
			}
			if err := mediaref.SyncPostCoverUsage(tx, post.ID, post.CoverMediaID); err != nil {
				return err
			}
			job = model.PublishJob{
				PostID: &post.ID, JobType: "post", Status: "requested", TriggerSource: "admin",
				RequestedBy: parseOptionalUUID(c.GetString("user_id")), RunAt: time.Now(),
			}
			return tx.Create(&job).Error
		})
		if !accessOK {
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}
		if errors.Is(err, errContentPublishInProgress) {
			Fail(c, http.StatusConflict, "CONTENT_PUBLISH_IN_PROGRESS", "post has a publish job in progress")
			return
		}
		if errors.Is(err, errContentQualityBlocked) && blockedReport != nil {
			failContentQuality(c, *blockedReport)
			return
		}
		if err != nil {
			Fail(c, http.StatusConflict, "PUBLISH_JOB_CONFLICT", "could not create publish job")
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"data": gin.H{
				"post": post,
				"job":  job,
			},
			"request_id": requestID(c),
		})
	}
}

func listPublishJobs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}

		order, ok := map[string]string{
			"":            "publish_jobs.created_at desc",
			"-created_at": "publish_jobs.created_at desc",
			"created_at":  "publish_jobs.created_at asc",
		}[pagination.Sort]
		if !ok {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
			return
		}

		var jobs []model.PublishJob
		query := db.WithContext(c.Request.Context()).
			Model(&model.PublishJob{}).
			Preload("Post", func(db *gorm.DB) *gorm.DB { return db.Select("id", "title", "slug") }).
			Preload("Page", func(db *gorm.DB) *gorm.DB { return db.Select("id", "title", "slug") })
		query, ok = scopePublishQuery(c, query, "publish_jobs", contentAccessRead)
		if !ok {
			return
		}
		if pagination.Status != "" {
			query = query.Where("publish_jobs.status = ?", pagination.Status)
		}
		if pagination.Query != "" {
			like := "%" + pagination.Query + "%"
			query = query.Where("(publish_jobs.release_key ILIKE ? OR publish_jobs.error_code ILIKE ? OR publish_jobs.error_message ILIKE ?)", like, like, like)
		}

		var total int64
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list publish jobs")
			return
		}
		if returnEmptyPageIfOutOfRange[model.PublishJob](c, total, pagination) {
			return
		}
		if err := query.
			Order(order + ", publish_jobs.id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&jobs).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list publish jobs")
			return
		}
		OKPaginated(c, jobs, total, pagination)
	}
}

func getPublishJob(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid publish job id")
			return
		}
		var job model.PublishJob
		query, ok := scopePublishQuery(c, db.WithContext(c.Request.Context()), "publish_jobs", contentAccessRead)
		if !ok {
			return
		}
		if err := query.
			Preload("Post").
			Preload("Page").
			First(&job, "id = ?", jobID).Error; err != nil {
			Fail(c, http.StatusNotFound, "PUBLISH_JOB_NOT_FOUND", "publish job not found")
			return
		}
		OK(c, job)
	}
}

func listPublishReleases(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}

		order, ok := map[string]string{
			"":            "publish_releases.created_at desc",
			"-created_at": "publish_releases.created_at desc",
			"created_at":  "publish_releases.created_at asc",
		}[pagination.Sort]
		if !ok {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
			return
		}

		var releases []model.PublishRelease
		query := db.WithContext(c.Request.Context()).
			Model(&model.PublishRelease{}).
			Preload("Post", func(db *gorm.DB) *gorm.DB { return db.Select("id", "title", "slug") }).
			Preload("Page", func(db *gorm.DB) *gorm.DB { return db.Select("id", "title", "slug") })
		query, ok = scopePublishQuery(c, query, "publish_releases", contentAccessRead)
		if !ok {
			return
		}
		if pagination.Status != "" {
			query = query.Where("publish_releases.status = ?", pagination.Status)
		}
		if pagination.Query != "" {
			like := "%" + pagination.Query + "%"
			query = query.Where("(publish_releases.release_key ILIKE ? OR publish_releases.output_path ILIKE ?)", like, like)
		}

		var total int64
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list publish releases")
			return
		}
		if returnEmptyPageIfOutOfRange[model.PublishRelease](c, total, pagination) {
			return
		}
		if err := query.
			Order(order + ", publish_releases.id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&releases).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list publish releases")
			return
		}
		OKPaginated(c, releases, total, pagination)
	}
}

func promotePublishRelease(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !hasGlobalContentAccess(c, contentAccessManage) {
			Fail(c, http.StatusForbidden, "FORBIDDEN", "global content access required")
			return
		}
		releaseID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid release id")
			return
		}

		release, err := publisher.PromoteRelease(c.Request.Context(), db, cfg, releaseID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Fail(c, http.StatusNotFound, "RELEASE_NOT_FOUND", "release not found")
				return
			}
			Fail(c, http.StatusConflict, "RELEASE_OUTPUT_INVALID", err.Error())
			return
		}
		OK(c, release)
	}
}

func retryPublishJob(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid job id")
			return
		}
		query, ok := scopePublishQuery(c, db.WithContext(c.Request.Context()), "publish_jobs", contentAccessManage)
		if !ok {
			return
		}
		var accessible model.PublishJob
		if err := query.First(&accessible, "publish_jobs.id = ?", jobID).Error; err != nil {
			Fail(c, http.StatusNotFound, "PUBLISH_JOB_NOT_FOUND", "publish job not found")
			return
		}

		job, err := publisher.RetryJob(c.Request.Context(), db, cfg, jobID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Fail(c, http.StatusNotFound, "PUBLISH_JOB_NOT_FOUND", "publish job not found")
				return
			}
			Fail(c, http.StatusConflict, "PUBLISH_JOB_NOT_RETRYABLE", err.Error())
			return
		}
		OK(c, job)
	}
}

func cancelPublishJob(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid job id")
			return
		}
		query, ok := scopePublishQuery(c, db.WithContext(c.Request.Context()), "publish_jobs", contentAccessManage)
		if !ok {
			return
		}
		var accessible model.PublishJob
		if err := query.First(&accessible, "publish_jobs.id = ?", jobID).Error; err != nil {
			Fail(c, http.StatusNotFound, "PUBLISH_JOB_NOT_FOUND", "publish job not found")
			return
		}

		job, err := publisher.CancelJob(c.Request.Context(), db, jobID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Fail(c, http.StatusNotFound, "PUBLISH_JOB_NOT_FOUND", "publish job not found")
				return
			}
			Fail(c, http.StatusConflict, "PUBLISH_JOB_NOT_CANCELABLE", err.Error())
			return
		}
		OK(c, job)
	}
}
