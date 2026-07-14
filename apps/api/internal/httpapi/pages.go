package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/contentquality"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type pageCreateRequest struct {
	Title          string `json:"title" binding:"required"`
	Slug           string `json:"slug" binding:"required"`
	Summary        string `json:"summary"`
	ContentMD      string `json:"content_md"`
	Status         string `json:"status"`
	Visibility     string `json:"visibility"`
	ShowInMenu     *bool  `json:"show_in_menu"`
	MenuWeight     *int   `json:"menu_weight"`
	MenuIcon       string `json:"menu_icon"`
	AllowComment   *bool  `json:"allow_comment"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
}

type pageUpdateRequest struct {
	Title          *string `json:"title"`
	Slug           *string `json:"slug"`
	Summary        *string `json:"summary"`
	ContentMD      *string `json:"content_md"`
	Status         *string `json:"status"`
	Visibility     *string `json:"visibility"`
	ShowInMenu     *bool   `json:"show_in_menu"`
	MenuWeight     *int    `json:"menu_weight"`
	MenuIcon       *string `json:"menu_icon"`
	AllowComment   *bool   `json:"allow_comment"`
	SEOTitle       *string `json:"seo_title"`
	SEODescription *string `json:"seo_description"`
}

func listPublicPages(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePublicPagination(c, publicListLimit)
		if !ok {
			return
		}
		query := db.WithContext(c.Request.Context()).Model(&model.Page{}).
			Where("status = ? and visibility = ?", "published", "public")
		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count pages")
			return
		}
		var pages []model.Page
		if err := query.
			Order("show_in_menu desc, menu_weight asc, title asc, id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&pages).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list pages")
			return
		}
		items := make([]publicPageDTO, 0, len(pages))
		for _, page := range pages {
			items = append(items, newPublicPageDTO(page))
		}
		setPublicPaginationHeaders(c, total, pagination)
		OK(c, items)
	}
}

func getPublicPage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var page model.Page
		if err := db.WithContext(c.Request.Context()).
			Where("slug = ? and status = ? and visibility = ?", c.Param("slug"), "published", "public").
			First(&page).Error; err != nil {
			Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
			return
		}
		OK(c, newPublicPageDTO(page))
	}
}

func listAdminPages(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}
		order, ok := adminListOrder(pagination.Sort, map[string]string{
			"created_at":   "created_at",
			"updated_at":   "updated_at",
			"title":        "title",
			"slug":         "slug",
			"status":       "status",
			"published_at": "published_at",
			"menu_weight":  "menu_weight",
		})
		if !ok {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
			return
		}

		query := db.WithContext(c.Request.Context()).Model(&model.Page{})
		query, ok = scopeContentQuery(c, query, "pages", contentAccessRead)
		if !ok {
			return
		}
		if pagination.Query != "" {
			pattern := "%" + pagination.Query + "%"
			query = query.Where("title ILIKE ? OR summary ILIKE ? OR slug ILIKE ?", pattern, pattern, pattern)
		}
		if pagination.Status != "" {
			query = query.Where("status = ?", pagination.Status)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count pages")
			return
		}
		if returnEmptyPageIfOutOfRange[model.Page](c, total, pagination) {
			return
		}
		var pages []model.Page
		if err := query.Order(order + ", id asc").Offset(pagination.Offset).Limit(pagination.PageSize).Find(&pages).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list pages")
			return
		}
		OKPaginated(c, pages, total, pagination)
	}
}

func adminListOrder(sort string, allowed map[string]string) (string, bool) {
	if sort == "" {
		sort = "-created_at"
	}
	parts := strings.Split(sort, ",")
	orders := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return "", false
		}
		direction := "asc"
		field := part
		if strings.HasPrefix(part, "-") {
			direction = "desc"
			field = strings.TrimPrefix(part, "-")
		}
		column, ok := allowed[field]
		if !ok {
			return "", false
		}
		orders = append(orders, column+" "+direction)
	}
	return strings.Join(orders, ", "), true
}

func getAdminPage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var page model.Page
		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "pages", contentAccessRead)
		if !ok {
			return
		}
		if err := query.First(&page, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
			return
		}
		OK(c, page)
	}
}

func createPage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pageCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid page payload")
			return
		}

		page, err := pageFromCreateRequest(req)
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		if page.Status == "published" {
			Fail(c, http.StatusUnprocessableEntity, "PUBLISH_ENDPOINT_REQUIRED", "use the page publish endpoint to publish content")
			return
		}
		authorID, ok := currentContentUserID(c)
		if !ok {
			return
		}
		page.AuthorID = &authorID
		if err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&page).Error; err != nil {
				return err
			}
			return mediaref.SyncPageMarkdownUsages(tx, page.ID, page.ContentMD)
		}); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
				Fail(c, http.StatusConflict, "PAGE_SLUG_CONFLICT", "could not create page")
				return
			}
			Fail(c, http.StatusConflict, "CONFLICT", "could not create page")
			return
		}
		Created(c, page)
	}
}

func updatePage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pageUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid page payload")
			return
		}
		var page model.Page
		accessOK := true
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			query, ok := scopeContentQuery(c, tx, "pages", contentAccessManage)
			if !ok {
				accessOK = false
				return nil
			}
			if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).First(&page, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			if page.Status == "published" && !containsPermission(c.GetStringSlice("permissions"), "page:publish") {
				return errPublishedContentPermission
			}
			if req.Status != nil {
				status := normalizePageStatus(*req.Status)
				if status == "published" && status != page.Status {
					return errPublishEndpointRequired
				}
				if status != "" && status != page.Status && page.Status == "published" && !containsPermission(c.GetStringSlice("permissions"), "page:publish") {
					return errPublishedContentPermission
				}
			}
			inProgress, err := contentPublishInProgress(tx, "page_id", page.ID)
			if err != nil {
				return err
			}
			if inProgress {
				return errContentPublishInProgress
			}
			updates, err := pageUpdatesFromRequest(req)
			if err != nil {
				return err
			}
			if len(updates) > 0 {
				if err := tx.Model(&page).Updates(updates).Error; err != nil {
					return err
				}
			}
			var refreshed model.Page
			if err := tx.First(&refreshed, "id = ?", page.ID).Error; err != nil {
				return err
			}
			return mediaref.SyncPageMarkdownUsages(tx, page.ID, refreshed.ContentMD)
		})
		if !accessOK {
			return
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
				return
			}
			if errors.Is(err, errContentPublishInProgress) {
				Fail(c, http.StatusConflict, "CONTENT_PUBLISH_IN_PROGRESS", "page has a publish job in progress")
				return
			}
			if errors.Is(err, errPublishEndpointRequired) {
				Fail(c, http.StatusUnprocessableEntity, "PUBLISH_ENDPOINT_REQUIRED", "use the page publish endpoint to publish content")
				return
			}
			if errors.Is(err, errPublishedContentPermission) {
				Fail(c, http.StatusForbidden, "FORBIDDEN", "permission required: page:publish")
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
				Fail(c, http.StatusConflict, "PAGE_SLUG_CONFLICT", "could not update page")
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "invalid") || strings.Contains(strings.ToLower(err.Error()), "reserved") {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
				return
			}
			Fail(c, http.StatusConflict, "CONFLICT", "could not update page")
			return
		}

		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "pages", contentAccessManage)
		if !ok {
			return
		}
		if err := query.First(&page, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
			return
		}
		OK(c, page)
	}
}

func publishPage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var page model.Page
		var job model.PublishJob
		var blockedReport *contentquality.Report
		accessOK := true
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			query, ok := scopeContentQuery(c, tx, "pages", contentAccessManage)
			if !ok {
				accessOK = false
				return nil
			}
			if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).First(&page, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			inProgress, err := contentPublishInProgress(tx, "page_id", page.ID)
			if err != nil {
				return err
			}
			if inProgress {
				return errContentPublishInProgress
			}
			report := evaluatePageWithSlugError(page)
			if !report.Ready {
				blockedReport = &report
				return errContentQualityBlocked
			}

			publishedAt := time.Now()
			if page.PublishedAt != nil {
				publishedAt = *page.PublishedAt
			}
			if err := tx.Model(&page).Updates(map[string]interface{}{
				"status": "published", "published_at": publishedAt,
			}).Error; err != nil {
				return err
			}
			page.Status = "published"
			page.PublishedAt = &publishedAt
			if err := mediaref.SyncPageMarkdownUsages(tx, page.ID, page.ContentMD); err != nil {
				return err
			}
			job = model.PublishJob{
				PageID: &page.ID, JobType: "page", Status: "requested", TriggerSource: "admin",
				RequestedBy: parseOptionalUUID(c.GetString("user_id")), RunAt: time.Now(),
			}
			return tx.Create(&job).Error
		})
		if !accessOK {
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
			return
		}
		if errors.Is(err, errContentPublishInProgress) {
			Fail(c, http.StatusConflict, "CONTENT_PUBLISH_IN_PROGRESS", "page has a publish job in progress")
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
				"page": page,
				"job":  job,
			},
			"request_id": requestID(c),
		})
	}
}

func pageFromCreateRequest(req pageCreateRequest) (model.Page, error) {
	title := strings.TrimSpace(req.Title)
	slug := strings.TrimSpace(req.Slug)
	if title == "" || slug == "" {
		return model.Page{}, errors.New("invalid page payload")
	}
	if err := publisher.ValidatePageSlug(slug); err != nil {
		return model.Page{}, err
	}
	status := normalizePageStatus(req.Status)
	if status == "" {
		return model.Page{}, errors.New("invalid page status")
	}
	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = "public"
	}
	showInMenu := false
	if req.ShowInMenu != nil {
		showInMenu = *req.ShowInMenu
	}
	menuWeight := 0
	if req.MenuWeight != nil {
		menuWeight = *req.MenuWeight
	}
	allowComment := false
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
	page := model.Page{
		Title:          title,
		Slug:           slug,
		Summary:        strings.TrimSpace(req.Summary),
		ContentMD:      req.ContentMD,
		Status:         status,
		Visibility:     visibility,
		ShowInMenu:     showInMenu,
		MenuWeight:     menuWeight,
		MenuIcon:       strings.TrimSpace(req.MenuIcon),
		AllowComment:   allowComment,
		SEOTitle:       seoTitle,
		SEODescription: seoDescription,
	}
	if status == "published" {
		now := time.Now()
		page.PublishedAt = &now
	}
	return page, nil
}

func pageUpdatesFromRequest(value pageUpdateRequest) (map[string]interface{}, error) {
	updates := map[string]interface{}{}
	if value.Title != nil {
		title := strings.TrimSpace(*value.Title)
		if title == "" {
			return nil, errors.New("invalid page title")
		}
		updates["title"] = title
		if value.SEOTitle == nil {
			updates["seo_title"] = title
		}
	}
	if value.Slug != nil {
		slug := strings.TrimSpace(*value.Slug)
		if err := publisher.ValidatePageSlug(slug); err != nil {
			return nil, err
		}
		updates["slug"] = slug
	}
	if value.Summary != nil {
		updates["summary"] = strings.TrimSpace(*value.Summary)
	}
	if value.ContentMD != nil {
		updates["content_md"] = *value.ContentMD
	}
	if value.Status != nil {
		status := normalizePageStatus(*value.Status)
		if status == "" {
			return nil, errors.New("invalid page status")
		}
		updates["status"] = status
		if status == "published" {
			now := time.Now()
			updates["published_at"] = now
		}
	}
	if value.Visibility != nil {
		updates["visibility"] = strings.TrimSpace(*value.Visibility)
	}
	if value.ShowInMenu != nil {
		updates["show_in_menu"] = *value.ShowInMenu
	}
	if value.MenuWeight != nil {
		updates["menu_weight"] = *value.MenuWeight
	}
	if value.MenuIcon != nil {
		updates["menu_icon"] = strings.TrimSpace(*value.MenuIcon)
	}
	if value.AllowComment != nil {
		updates["allow_comment"] = *value.AllowComment
	}
	if value.SEOTitle != nil {
		updates["seo_title"] = strings.TrimSpace(*value.SEOTitle)
	}
	if value.SEODescription != nil {
		updates["seo_description"] = strings.TrimSpace(*value.SEODescription)
	}
	return updates, nil
}

func normalizePageStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "":
		return "draft"
	case "draft", "published", "offline", "archived":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}
