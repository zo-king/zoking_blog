package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
)

type taxonomyRequest struct {
	Name        string     `json:"name" binding:"required"`
	Slug        string     `json:"slug" binding:"required"`
	Description string     `json:"description"`
	ParentID    *uuid.UUID `json:"parent_id"`
	SortOrder   *int       `json:"sort_order"`
	Enabled     *bool      `json:"enabled"`
	Color       string     `json:"color"`
}

func listCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePublicPagination(c, publicListLimit)
		if !ok {
			return
		}
		query := db.WithContext(c.Request.Context()).Model(&model.Category{}).Where("enabled = true")
		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count categories")
			return
		}
		var categories []model.Category
		if err := query.
			Order("sort_order asc, name asc, id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&categories).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list categories")
			return
		}
		items := make([]publicTaxonomyDTO, 0, len(categories))
		for _, item := range categories {
			items = append(items, newPublicCategoryDTO(item))
		}
		setPublicPaginationHeaders(c, total, pagination)
		OK(c, items)
	}
}

func listAdminCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var categories []model.Category
		if err := db.WithContext(c.Request.Context()).
			Order("sort_order asc, name asc").
			Find(&categories).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list categories")
			return
		}
		OK(c, categories)
	}
}

func createCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req taxonomyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category payload")
			return
		}
		name := strings.TrimSpace(req.Name)
		slug := strings.TrimSpace(req.Slug)
		if name == "" || slug == "" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category payload")
			return
		}
		if err := publisher.ValidateSlug(slug); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category slug")
			return
		}

		category := model.Category{
			Name:        name,
			Slug:        slug,
			Description: strings.TrimSpace(req.Description),
			ParentID:    req.ParentID,
			SortOrder:   0,
			Enabled:     true,
		}
		if req.SortOrder != nil {
			category.SortOrder = *req.SortOrder
		}
		if req.Enabled != nil {
			category.Enabled = *req.Enabled
		}

		if err := db.WithContext(c.Request.Context()).Create(&category).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not create category")
			return
		}
		Created(c, category)
	}
}

func updateCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req taxonomyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category payload")
			return
		}
		name := strings.TrimSpace(req.Name)
		slug := strings.TrimSpace(req.Slug)
		if name == "" || slug == "" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category payload")
			return
		}
		if err := publisher.ValidateSlug(slug); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid category slug")
			return
		}

		var category model.Category
		if err := db.WithContext(c.Request.Context()).First(&category, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "category not found")
			return
		}

		updates := map[string]interface{}{
			"name":        name,
			"slug":        slug,
			"description": strings.TrimSpace(req.Description),
		}
		if req.ParentID != nil {
			updates["parent_id"] = req.ParentID
		}
		if req.SortOrder != nil {
			updates["sort_order"] = *req.SortOrder
		}
		if req.Enabled != nil {
			updates["enabled"] = *req.Enabled
		}

		if err := db.WithContext(c.Request.Context()).Model(&category).Updates(updates).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not update category")
			return
		}
		if err := db.WithContext(c.Request.Context()).First(&category, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "category not found")
			return
		}
		OK(c, category)
	}
}

func deleteCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var category model.Category
		if err := db.WithContext(c.Request.Context()).First(&category, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "category not found")
			return
		}
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("delete from post_categories where category_id = ?", category.ID).Error; err != nil {
				return err
			}
			return tx.Delete(&category).Error
		})
		if err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not delete category")
			return
		}
		OK(c, gin.H{"deleted": true})
	}
}

func listTags(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePublicPagination(c, publicListLimit)
		if !ok {
			return
		}
		query := db.WithContext(c.Request.Context()).Model(&model.Tag{})
		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count tags")
			return
		}
		var tags []model.Tag
		if err := query.
			Order("name asc, id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&tags).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list tags")
			return
		}
		items := make([]publicTaxonomyDTO, 0, len(tags))
		for _, item := range tags {
			items = append(items, newPublicTagDTO(item))
		}
		setPublicPaginationHeaders(c, total, pagination)
		OK(c, items)
	}
}

func listAdminTags(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tags []model.Tag
		if err := db.WithContext(c.Request.Context()).
			Order("name asc").
			Find(&tags).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list tags")
			return
		}
		OK(c, tags)
	}
}

func createTag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req taxonomyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid tag payload")
			return
		}
		name := strings.TrimSpace(req.Name)
		slug := strings.TrimSpace(req.Slug)
		if name == "" || slug == "" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid tag payload")
			return
		}
		if err := publisher.ValidateSlug(slug); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid tag slug")
			return
		}

		tag := model.Tag{
			Name:        name,
			Slug:        slug,
			Description: strings.TrimSpace(req.Description),
			Color:       strings.TrimSpace(req.Color),
		}

		if err := db.WithContext(c.Request.Context()).Create(&tag).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not create tag")
			return
		}
		Created(c, tag)
	}
}

func updateTag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req taxonomyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid tag payload")
			return
		}
		name := strings.TrimSpace(req.Name)
		slug := strings.TrimSpace(req.Slug)
		if name == "" || slug == "" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid tag payload")
			return
		}
		if err := publisher.ValidateSlug(slug); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid tag slug")
			return
		}

		var tag model.Tag
		if err := db.WithContext(c.Request.Context()).First(&tag, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "tag not found")
			return
		}

		updates := map[string]interface{}{
			"name":        name,
			"slug":        slug,
			"description": strings.TrimSpace(req.Description),
			"color":       strings.TrimSpace(req.Color),
		}

		if err := db.WithContext(c.Request.Context()).Model(&tag).Updates(updates).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not update tag")
			return
		}
		if err := db.WithContext(c.Request.Context()).First(&tag, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "tag not found")
			return
		}
		OK(c, tag)
	}
}

func deleteTag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tag model.Tag
		if err := db.WithContext(c.Request.Context()).First(&tag, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "tag not found")
			return
		}
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("delete from post_tags where tag_id = ?", tag.ID).Error; err != nil {
				return err
			}
			return tx.Delete(&tag).Error
		})
		if err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not delete tag")
			return
		}
		OK(c, gin.H{"deleted": true})
	}
}
