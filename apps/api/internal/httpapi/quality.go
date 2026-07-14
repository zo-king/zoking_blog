package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/contentquality"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
)

var (
	errContentQualityBlocked      = errors.New("content quality check failed")
	errPublishEndpointRequired    = errors.New("published state requires the publish endpoint")
	errPublishedContentPermission = errors.New("published content requires publish permission")
)

type postQualityRequest struct {
	Title          *string      `json:"title"`
	Slug           *string      `json:"slug"`
	Summary        *string      `json:"summary"`
	ContentMD      *string      `json:"content_md"`
	Visibility     *string      `json:"visibility"`
	SEOTitle       *string      `json:"seo_title"`
	SEODescription *string      `json:"seo_description"`
	CategoryIDs    *[]uuid.UUID `json:"category_ids"`
	TagIDs         *[]uuid.UUID `json:"tag_ids"`
	CoverMediaID   *string      `json:"cover_media_id"`
	SeriesID       *string      `json:"series_id"`
	SeriesOrder    *int         `json:"series_order"`
}

type pageQualityRequest struct {
	Title          *string `json:"title"`
	Slug           *string `json:"slug"`
	Summary        *string `json:"summary"`
	ContentMD      *string `json:"content_md"`
	Visibility     *string `json:"visibility"`
	ShowInMenu     *bool   `json:"show_in_menu"`
	MenuIcon       *string `json:"menu_icon"`
	SEOTitle       *string `json:"seo_title"`
	SEODescription *string `json:"seo_description"`
}

func qualityCheckNewPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req postQualityRequest
		if err := bindOptionalQualityRequest(c, &req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post quality payload")
			return
		}
		post := model.Post{Visibility: "public"}
		applyPostQualityRequest(&post, req, true)
		OK(c, evaluatePostWithSlugError(post))
	}
}

func qualityCheckPost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req postQualityRequest
		if err := bindOptionalQualityRequest(c, &req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post quality payload")
			return
		}
		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "posts", contentAccessManage)
		if !ok {
			return
		}
		post, err := loadPostWithCoverAndTaxonomy(query, "id = ?", c.Param("id"))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load post")
			return
		}
		applyPostQualityRequest(&post, req, false)
		OK(c, evaluatePostWithSlugError(post))
	}
}

func qualityCheckNewPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pageQualityRequest
		if err := bindOptionalQualityRequest(c, &req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid page quality payload")
			return
		}
		page := model.Page{Visibility: "public"}
		applyPageQualityRequest(&page, req, true)
		OK(c, evaluatePageWithSlugError(page))
	}
}

func qualityCheckPage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pageQualityRequest
		if err := bindOptionalQualityRequest(c, &req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid page quality payload")
			return
		}
		var page model.Page
		query, ok := scopeContentQuery(c, db.WithContext(c.Request.Context()), "pages", contentAccessManage)
		if !ok {
			return
		}
		if err := query.First(&page, "id = ?", c.Param("id")).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
			return
		} else if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load page")
			return
		}
		applyPageQualityRequest(&page, req, false)
		OK(c, evaluatePageWithSlugError(page))
	}
}

func bindOptionalQualityRequest(c *gin.Context, target interface{}) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	return c.ShouldBindJSON(target)
}

func applyPostQualityRequest(post *model.Post, req postQualityRequest, creating bool) {
	if req.Title != nil {
		post.Title = strings.TrimSpace(*req.Title)
		if !creating && req.SEOTitle == nil {
			post.SEOTitle = post.Title
		}
	}
	if req.Slug != nil {
		post.Slug = strings.TrimSpace(*req.Slug)
	}
	if req.Summary != nil {
		post.Summary = strings.TrimSpace(*req.Summary)
	}
	if req.ContentMD != nil {
		post.ContentMD = *req.ContentMD
	}
	if req.Visibility != nil {
		post.Visibility = strings.TrimSpace(*req.Visibility)
	}
	if req.SEOTitle != nil {
		post.SEOTitle = strings.TrimSpace(*req.SEOTitle)
	}
	if req.SEODescription != nil {
		post.SEODescription = strings.TrimSpace(*req.SEODescription)
	}
	if req.CategoryIDs != nil {
		post.Categories = make([]model.Category, len(uniqueUUIDs(*req.CategoryIDs)))
	}
	if req.TagIDs != nil {
		post.Tags = make([]model.Tag, len(uniqueUUIDs(*req.TagIDs)))
	}
	if req.CoverMediaID != nil {
		post.CoverMediaID = nil
		if id, err := uuid.Parse(strings.TrimSpace(*req.CoverMediaID)); err == nil {
			post.CoverMediaID = &id
		}
	}
	if req.SeriesID != nil {
		post.SeriesID = nil
		post.Series = nil
		seriesID := strings.TrimSpace(*req.SeriesID)
		if seriesID == "" {
			post.SeriesOrder = nil
		} else if id, err := uuid.Parse(seriesID); err == nil {
			post.SeriesID = &id
		}
	}
	if req.SeriesOrder != nil {
		order := *req.SeriesOrder
		post.SeriesOrder = &order
	}
	if creating {
		if post.SEOTitle == "" {
			post.SEOTitle = post.Title
		}
		if post.SEODescription == "" {
			post.SEODescription = post.Summary
		}
	}
}

func applyPageQualityRequest(page *model.Page, req pageQualityRequest, creating bool) {
	if req.Title != nil {
		page.Title = strings.TrimSpace(*req.Title)
		if !creating && req.SEOTitle == nil {
			page.SEOTitle = page.Title
		}
	}
	if req.Slug != nil {
		page.Slug = strings.TrimSpace(*req.Slug)
	}
	if req.Summary != nil {
		page.Summary = strings.TrimSpace(*req.Summary)
	}
	if req.ContentMD != nil {
		page.ContentMD = *req.ContentMD
	}
	if req.Visibility != nil {
		page.Visibility = strings.TrimSpace(*req.Visibility)
	}
	if req.ShowInMenu != nil {
		page.ShowInMenu = *req.ShowInMenu
	}
	if req.MenuIcon != nil {
		page.MenuIcon = strings.TrimSpace(*req.MenuIcon)
	}
	if req.SEOTitle != nil {
		page.SEOTitle = strings.TrimSpace(*req.SEOTitle)
	}
	if req.SEODescription != nil {
		page.SEODescription = strings.TrimSpace(*req.SEODescription)
	}
	if creating {
		if page.SEOTitle == "" {
			page.SEOTitle = page.Title
		}
		if page.SEODescription == "" {
			page.SEODescription = page.Summary
		}
	}
}

func evaluatePostWithSlugError(post model.Post) contentquality.Report {
	document := postQualityDocument(post)
	if post.Slug != "" {
		if err := publisher.ValidateSlug(post.Slug); err != nil {
			document.SlugError = "Slug 格式不正确"
		}
	}
	return contentquality.Evaluate(document)
}

func evaluatePageWithSlugError(page model.Page) contentquality.Report {
	document := pageQualityDocument(page)
	if page.Slug != "" {
		if err := publisher.ValidatePageSlug(page.Slug); err != nil {
			document.SlugError = "Slug 格式不正确或使用了保留路径"
		}
	}
	return contentquality.Evaluate(document)
}

func postQualityDocument(post model.Post) contentquality.Document {
	return contentquality.Document{
		Kind: "post", Title: post.Title, Slug: post.Slug, Summary: post.Summary, ContentMD: post.ContentMD,
		Visibility: post.Visibility, SEOTitle: post.SEOTitle, SEODescription: post.SEODescription,
		HasCover: post.CoverMediaID != nil, CategoryCount: len(post.Categories), TagCount: len(post.Tags),
		SeriesSelected: post.SeriesID != nil, SeriesOrderSet: post.SeriesOrder != nil, SeriesOrder: qualityIntValue(post.SeriesOrder),
	}
}

func qualityIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func pageQualityDocument(page model.Page) contentquality.Document {
	return contentquality.Document{
		Kind: "page", Title: page.Title, Slug: page.Slug, Summary: page.Summary, ContentMD: page.ContentMD,
		Visibility: page.Visibility, SEOTitle: page.SEOTitle, SEODescription: page.SEODescription,
		ShowInMenu: page.ShowInMenu, MenuIcon: page.MenuIcon,
	}
}

func failContentQuality(c *gin.Context, report contentquality.Report) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{
		"error":      ErrorBody{Code: "CONTENT_QUALITY_BLOCKED", Message: "内容未通过发布检查", Details: report},
		"request_id": requestID(c),
	})
}
