package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type createSeriesRequest struct {
	Name         string  `json:"name" binding:"required"`
	Slug         string  `json:"slug" binding:"required"`
	Description  string  `json:"description"`
	CoverMediaID rawJSON `json:"cover_media_id"`
	SortOrder    *int    `json:"sort_order"`
	Enabled      *bool   `json:"enabled"`
}

type updateSeriesRequest struct {
	Name         *string `json:"name"`
	Slug         *string `json:"slug"`
	Description  *string `json:"description"`
	CoverMediaID rawJSON `json:"cover_media_id"`
	SortOrder    *int    `json:"sort_order"`
	Enabled      *bool   `json:"enabled"`
}

func listPublicSeries(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePublicPagination(c, publicListLimit)
		if !ok {
			return
		}
		query := db.WithContext(c.Request.Context()).Model(&model.Series{}).Where("enabled = true")
		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count series")
			return
		}
		var items []model.Series
		if err := query.
			Preload("CoverMedia").
			Order("sort_order asc, name asc, id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&items).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list series")
			return
		}
		if err := populateSeriesPostCounts(db.WithContext(c.Request.Context()), items, true); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count series posts")
			return
		}
		result := make([]publicSeriesDTO, 0, len(items))
		for _, item := range items {
			result = append(result, newPublicSeriesDTO(item))
		}
		setPublicPaginationHeaders(c, total, pagination)
		OK(c, result)
	}
}

func getPublicSeries(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePublicPagination(c, publicSeriesLimit)
		if !ok {
			return
		}
		var item model.Series
		if err := db.WithContext(c.Request.Context()).
			Preload("CoverMedia").
			Where("slug = ? and enabled = true", c.Param("slug")).
			First(&item).Error; err != nil {
			Fail(c, http.StatusNotFound, "SERIES_NOT_FOUND", "series not found")
			return
		}

		postQuery := db.WithContext(c.Request.Context()).Model(&model.Post{}).
			Where("series_id = ? and status = ? and visibility = ?", item.ID, "published", "public")
		if err := postQuery.Count(&item.PostCount).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count series posts")
			return
		}
		if err := preloadPostTaxonomy(postQuery).
			Preload("CoverMedia").
			Order("series_order asc, id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&item.Posts).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load series posts")
			return
		}
		setPublicPaginationHeaders(c, item.PostCount, pagination)
		OK(c, newPublicSeriesDTO(item))
	}
}

func listAdminSeries(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var items []model.Series
		if err := db.WithContext(c.Request.Context()).
			Preload("CoverMedia").
			Order("sort_order asc, name asc, id asc").
			Find(&items).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list series")
			return
		}
		if err := populateSeriesPostCounts(db.WithContext(c.Request.Context()), items, false); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count series posts")
			return
		}
		OK(c, items)
	}
}

func getAdminSeries(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var item model.Series
		if err := db.WithContext(c.Request.Context()).
			Preload("CoverMedia").
			First(&item, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "SERIES_NOT_FOUND", "series not found")
			return
		}
		items := []model.Series{item}
		if err := populateSeriesPostCounts(db.WithContext(c.Request.Context()), items, false); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count series posts")
			return
		}
		item.PostCount = items[0].PostCount
		OK(c, item)
	}
}

type seriesPostCount struct {
	SeriesID  uuid.UUID `gorm:"column:series_id"`
	PostCount int64     `gorm:"column:post_count"`
}

func populateSeriesPostCounts(db *gorm.DB, items []model.Series, publicOnly bool) error {
	if len(items) == 0 {
		return nil
	}
	seriesIDs := make([]uuid.UUID, 0, len(items))
	for i := range items {
		seriesIDs = append(seriesIDs, items[i].ID)
	}

	var counts []seriesPostCount
	query := db.Model(&model.Post{}).
		Select("series_id, count(*) as post_count").
		Where("series_id in ?", seriesIDs)
	if publicOnly {
		query = query.Where("status = ? and visibility = ?", "published", "public")
	}
	if err := query.Group("series_id").Scan(&counts).Error; err != nil {
		return err
	}
	bySeriesID := make(map[uuid.UUID]int64, len(counts))
	for _, count := range counts {
		bySeriesID[count.SeriesID] = count.PostCount
	}
	for i := range items {
		items[i].PostCount = bySeriesID[items[i].ID]
	}
	return nil
}

func createSeries(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createSeriesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series payload")
			return
		}
		name := strings.TrimSpace(req.Name)
		slug := strings.TrimSpace(req.Slug)
		if name == "" || slug == "" || (req.SortOrder != nil && *req.SortOrder < 0) {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series payload")
			return
		}
		if err := publisher.ValidateSlug(slug); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series slug")
			return
		}

		item := model.Series{
			Name:        name,
			Slug:        slug,
			Description: strings.TrimSpace(req.Description),
			Enabled:     true,
		}
		if req.SortOrder != nil {
			item.SortOrder = *req.SortOrder
		}
		if req.Enabled != nil {
			item.Enabled = *req.Enabled
		}

		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			provided, coverMediaID, err := resolveSeriesCoverMedia(tx, req.CoverMediaID)
			if err != nil {
				return err
			}
			if provided {
				item.CoverMediaID = coverMediaID
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			return mediaref.SyncSeriesCoverUsage(tx, item.ID, item.CoverMediaID)
		})
		if err != nil {
			writeSeriesMutationError(c, err, "create")
			return
		}
		if err := db.WithContext(c.Request.Context()).Preload("CoverMedia").First(&item, "id = ?", item.ID).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load created series")
			return
		}
		Created(c, item)
	}
}

func updateSeries(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateSeriesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series payload")
			return
		}
		updates := map[string]interface{}{}
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series name")
				return
			}
			updates["name"] = name
		}
		if req.Slug != nil {
			slug := strings.TrimSpace(*req.Slug)
			if publisher.ValidateSlug(slug) != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series slug")
				return
			}
			updates["slug"] = slug
		}
		if req.Description != nil {
			updates["description"] = strings.TrimSpace(*req.Description)
		}
		if req.SortOrder != nil {
			if *req.SortOrder < 0 {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid series sort_order")
				return
			}
			updates["sort_order"] = *req.SortOrder
		}
		if req.Enabled != nil {
			updates["enabled"] = *req.Enabled
		}

		var item model.Series
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			provided, coverMediaID, err := resolveSeriesCoverMedia(tx, req.CoverMediaID)
			if err != nil {
				return err
			}
			if provided {
				updates["cover_media_id"] = coverMediaID
			}
			if len(updates) > 0 {
				if err := tx.Model(&item).Updates(updates).Error; err != nil {
					return err
				}
			}
			if provided {
				return mediaref.SyncSeriesCoverUsage(tx, item.ID, coverMediaID)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				Fail(c, http.StatusNotFound, "SERIES_NOT_FOUND", "series not found")
				return
			}
			writeSeriesMutationError(c, err, "update")
			return
		}
		if err := db.WithContext(c.Request.Context()).Preload("CoverMedia").First(&item, "id = ?", item.ID).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load updated series")
			return
		}
		OK(c, item)
	}
}

func deleteSeries(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var item model.Series
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			var references int64
			if err := tx.Model(&model.Post{}).Where("series_id = ?", item.ID).Count(&references).Error; err != nil {
				return err
			}
			if references > 0 {
				return errSeriesInUse
			}
			if err := mediaref.SyncSeriesCoverUsage(tx, item.ID, nil); err != nil {
				return err
			}
			return tx.Delete(&item).Error
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "SERIES_NOT_FOUND", "series not found")
			return
		}
		if errors.Is(err, errSeriesInUse) {
			Fail(c, http.StatusConflict, "SERIES_IN_USE", "series is referenced by posts")
			return
		}
		if err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not delete series")
			return
		}
		OK(c, gin.H{"deleted": true})
	}
}

var errSeriesInUse = errors.New("series is in use")

func resolveSeriesCoverMedia(db *gorm.DB, raw rawJSON) (bool, *uuid.UUID, error) {
	if len(raw) == 0 {
		return false, nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return false, nil, errors.New("invalid cover_media_id")
	}
	asset, err := mediaref.ResolveReadyMedia(db, value)
	if err != nil {
		return false, nil, err
	}
	return true, &asset.ID, nil
}

func writeSeriesMutationError(c *gin.Context, err error, operation string) {
	if postgresConstraint(err) == "series_slug_active_idx" {
		Fail(c, http.StatusConflict, "SERIES_SLUG_CONFLICT", "series slug is already in use")
		return
	}
	if errors.Is(err, mediaref.ErrMediaNotFound) {
		Fail(c, http.StatusNotFound, "MEDIA_NOT_FOUND", "cover media not found")
		return
	}
	if strings.Contains(err.Error(), "cover_media_id") {
		Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid cover_media_id")
		return
	}
	Fail(c, http.StatusConflict, "CONFLICT", "could not "+operation+" series")
}
