package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/mediaref"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	achievementDateLayout = "2006-01-02"
)

var (
	errInvalidAchievementPayload = errors.New("invalid achievement payload")
	errPublishedAchievement      = errors.New("published achievement cannot be deleted")
	minimumAchievementDate       = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
)

type createAchievementRequest struct {
	Kind         string  `json:"kind" binding:"required"`
	Title        string  `json:"title" binding:"required"`
	Organization string  `json:"organization"`
	Summary      string  `json:"summary"`
	OccurredAt   string  `json:"occurred_at" binding:"required"`
	EndedAt      rawJSON `json:"ended_at"`
	ExternalURL  string  `json:"external_url"`
	CredentialID string  `json:"credential_id"`
	ImageMediaID rawJSON `json:"image_media_id"`
	SortOrder    *int    `json:"sort_order"`
}

type updateAchievementRequest struct {
	Kind         *string `json:"kind"`
	Title        *string `json:"title"`
	Organization *string `json:"organization"`
	Summary      *string `json:"summary"`
	OccurredAt   *string `json:"occurred_at"`
	EndedAt      rawJSON `json:"ended_at"`
	ExternalURL  *string `json:"external_url"`
	CredentialID *string `json:"credential_id"`
	ImageMediaID rawJSON `json:"image_media_id"`
	SortOrder    *int    `json:"sort_order"`
}

type updateAchievementStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func listAdminAchievements(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}
		if pagination.Status != "" && !validAchievementStatus(pagination.Status) {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid achievement status")
			return
		}

		order := "occurred_at desc, sort_order asc, id asc"
		if pagination.Sort != "" {
			parsedOrder, valid := adminListOrder(pagination.Sort, map[string]string{
				"created_at":   "created_at",
				"updated_at":   "updated_at",
				"kind":         "kind",
				"title":        "title",
				"organization": "organization",
				"occurred_at":  "occurred_at",
				"ended_at":     "ended_at",
				"sort_order":   "sort_order",
				"status":       "status",
				"published_at": "published_at",
			})
			if !valid {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
				return
			}
			order = parsedOrder + ", occurred_at desc, sort_order asc, id asc"
		}

		query := db.WithContext(c.Request.Context()).Model(&model.Achievement{})
		if pagination.Query != "" {
			pattern := "%" + pagination.Query + "%"
			query = query.Where(
				"title ILIKE ? OR organization ILIKE ? OR summary ILIKE ? OR credential_id ILIKE ?",
				pattern, pattern, pattern, pattern,
			)
		}
		if pagination.Status != "" {
			query = query.Where("status = ?", pagination.Status)
		}
		if rawYear := strings.TrimSpace(c.Query("year")); rawYear != "" {
			year, valid := parseAchievementYear(rawYear)
			if !valid {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid year")
				return
			}
			start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
			query = query.Where(
				"occurred_at >= ? and occurred_at < ?",
				start.Format(achievementDateLayout), start.AddDate(1, 0, 0).Format(achievementDateLayout),
			)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count achievements")
			return
		}
		if returnEmptyPageIfOutOfRange[model.Achievement](c, total, pagination) {
			return
		}

		var items []model.Achievement
		if err := query.Preload("ImageMedia").Order(order).
			Offset(pagination.Offset).Limit(pagination.PageSize).Find(&items).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list achievements")
			return
		}
		OKPaginated(c, items, total, pagination)
	}
}

func getAdminAchievement(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		item, err := loadAchievement(db.WithContext(c.Request.Context()), c.Param("id"))
		if err != nil {
			writeAchievementLoadError(c, err)
			return
		}
		OK(c, item)
	}
}

func createAchievement(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createAchievementRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid achievement payload")
			return
		}

		item, err := achievementFromCreateRequest(req)
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			provided, mediaID, resolveErr := resolveAchievementImage(tx, req.ImageMediaID)
			if resolveErr != nil {
				return resolveErr
			}
			if provided {
				item.ImageMediaID = mediaID
			}
			if createErr := tx.Create(&item).Error; createErr != nil {
				return createErr
			}
			return mediaref.SyncAchievementImageUsage(tx, item.ID, item.ImageMediaID)
		})
		if err != nil {
			writeAchievementMutationError(c, err, "create")
			return
		}

		item, err = loadAchievement(db.WithContext(c.Request.Context()), item.ID.String())
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load created achievement")
			return
		}
		setAuditSnapshot(c, auditAfterKey, item)
		Created(c, item)
	}
}

func updateAchievement(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateAchievementRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid achievement payload")
			return
		}

		var before model.Achievement
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&before, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			updates, imageProvided, imageMediaID, err := achievementUpdatesFromRequest(tx, before, req)
			if err != nil {
				return err
			}
			if len(updates) > 0 {
				if err := tx.Model(&before).Updates(updates).Error; err != nil {
					return err
				}
			}
			if imageProvided {
				return mediaref.SyncAchievementImageUsage(tx, before.ID, imageMediaID)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeAchievementLoadError(c, err)
				return
			}
			writeAchievementMutationError(c, err, "update")
			return
		}

		item, err := loadAchievement(db.WithContext(c.Request.Context()), before.ID.String())
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load updated achievement")
			return
		}
		setAuditSnapshot(c, auditBeforeKey, before)
		setAuditSnapshot(c, auditAfterKey, item)
		OK(c, item)
	}
}

func updateAchievementStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateAchievementStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid achievement status payload")
			return
		}
		status := strings.ToLower(strings.TrimSpace(req.Status))
		if !validAchievementStatus(status) {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid achievement status")
			return
		}

		var before model.Achievement
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&before, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			updates := map[string]interface{}{"status": status}
			if status == "published" && before.Status != "published" {
				updates["published_at"] = time.Now().UTC()
			} else if status == "draft" {
				updates["published_at"] = nil
			}
			return tx.Model(&before).Updates(updates).Error
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeAchievementLoadError(c, err)
				return
			}
			writeAchievementMutationError(c, err, "update status of")
			return
		}

		item, err := loadAchievement(db.WithContext(c.Request.Context()), before.ID.String())
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load updated achievement")
			return
		}
		setAuditSnapshot(c, auditBeforeKey, before)
		setAuditSnapshot(c, auditAfterKey, item)
		OK(c, item)
	}
}

func deleteAchievement(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var before model.Achievement
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&before, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			if before.Status == "published" {
				return errPublishedAchievement
			}
			if err := mediaref.SyncAchievementImageUsage(tx, before.ID, nil); err != nil {
				return err
			}
			return tx.Delete(&before).Error
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeAchievementLoadError(c, err)
			return
		}
		if errors.Is(err, errPublishedAchievement) {
			Fail(c, http.StatusConflict, "ACHIEVEMENT_PUBLISHED", "published achievement cannot be deleted")
			return
		}
		if err != nil {
			writeAchievementMutationError(c, err, "delete")
			return
		}

		setAuditSnapshot(c, auditBeforeKey, before)
		setAuditSnapshot(c, auditAfterKey, gin.H{"id": before.ID, "deleted": true})
		OK(c, gin.H{"deleted": true})
	}
}

func publishAchievements(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := publisher.ValidateSiteContent(c.Request.Context(), db); err != nil {
			if errors.Is(err, publisher.ErrContentQualityBlocked) {
				Fail(c, http.StatusUnprocessableEntity, "CONTENT_QUALITY_BLOCKED", "published content did not pass the quality check")
				return
			}
			Fail(c, http.StatusInternalServerError, "CONTENT_QUALITY_CHECK_FAILED", "could not check published content")
			return
		}
		job := model.PublishJob{
			JobType: "site", Status: "requested", TriggerSource: "achievement",
			RequestedBy: parseOptionalUUID(c.GetString("user_id")), RunAt: time.Now(),
		}
		if err := db.WithContext(c.Request.Context()).Create(&job).Error; err != nil {
			Fail(c, http.StatusConflict, "ACHIEVEMENT_PUBLISH_CONFLICT", "could not create achievement publish job")
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"job": job}, "request_id": requestID(c)})
	}
}

func achievementFromCreateRequest(req createAchievementRequest) (model.Achievement, error) {
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	title := strings.TrimSpace(req.Title)
	occurredAt, err := parseAchievementDate(req.OccurredAt)
	if !validAchievementKind(kind) || title == "" || err != nil {
		return model.Achievement{}, errInvalidAchievementPayload
	}
	endedProvided, endedAt, err := parseOptionalAchievementDate(req.EndedAt)
	if err != nil || (endedProvided && endedAt != nil && endedAt.Before(occurredAt)) {
		return model.Achievement{}, errInvalidAchievementPayload
	}
	externalURL := strings.TrimSpace(req.ExternalURL)
	if !validAchievementURL(externalURL) || (req.SortOrder != nil && *req.SortOrder < 0) {
		return model.Achievement{}, errInvalidAchievementPayload
	}

	item := model.Achievement{
		Kind:         kind,
		Title:        title,
		Organization: strings.TrimSpace(req.Organization),
		Summary:      strings.TrimSpace(req.Summary),
		OccurredAt:   occurredAt,
		EndedAt:      endedAt,
		ExternalURL:  externalURL,
		CredentialID: strings.TrimSpace(req.CredentialID),
		Status:       "draft",
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}
	return item, nil
}

func achievementUpdatesFromRequest(tx *gorm.DB, current model.Achievement, req updateAchievementRequest) (map[string]interface{}, bool, *uuid.UUID, error) {
	updates := map[string]interface{}{}
	if req.Kind != nil {
		kind := strings.ToLower(strings.TrimSpace(*req.Kind))
		if !validAchievementKind(kind) {
			return nil, false, nil, errInvalidAchievementPayload
		}
		updates["kind"] = kind
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, false, nil, errInvalidAchievementPayload
		}
		updates["title"] = title
	}
	if req.Organization != nil {
		updates["organization"] = strings.TrimSpace(*req.Organization)
	}
	if req.Summary != nil {
		updates["summary"] = strings.TrimSpace(*req.Summary)
	}

	occurredAt := current.OccurredAt
	if req.OccurredAt != nil {
		parsed, err := parseAchievementDate(*req.OccurredAt)
		if err != nil {
			return nil, false, nil, errInvalidAchievementPayload
		}
		occurredAt = parsed
		updates["occurred_at"] = parsed
	}
	endedAt := current.EndedAt
	endedProvided, parsedEndedAt, err := parseOptionalAchievementDate(req.EndedAt)
	if err != nil {
		return nil, false, nil, errInvalidAchievementPayload
	}
	if endedProvided {
		endedAt = parsedEndedAt
		updates["ended_at"] = parsedEndedAt
	}
	if endedAt != nil && endedAt.Before(occurredAt) {
		return nil, false, nil, errInvalidAchievementPayload
	}

	if req.ExternalURL != nil {
		externalURL := strings.TrimSpace(*req.ExternalURL)
		if !validAchievementURL(externalURL) {
			return nil, false, nil, errInvalidAchievementPayload
		}
		updates["external_url"] = externalURL
	}
	if req.CredentialID != nil {
		updates["credential_id"] = strings.TrimSpace(*req.CredentialID)
	}
	if req.SortOrder != nil {
		if *req.SortOrder < 0 {
			return nil, false, nil, errInvalidAchievementPayload
		}
		updates["sort_order"] = *req.SortOrder
	}

	imageProvided, imageMediaID, err := resolveAchievementImage(tx, req.ImageMediaID)
	if err != nil {
		return nil, false, nil, err
	}
	if imageProvided {
		updates["image_media_id"] = imageMediaID
	}
	return updates, imageProvided, imageMediaID, nil
}

func parseAchievementDate(value string) (time.Time, error) {
	parsed, err := time.Parse(achievementDateLayout, strings.TrimSpace(value))
	if err != nil || parsed.Before(minimumAchievementDate) {
		return time.Time{}, errInvalidAchievementPayload
	}
	return parsed, nil
}

func parseOptionalAchievementDate(raw rawJSON) (bool, *time.Time, error) {
	if len(raw) == 0 {
		return false, nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, nil, errInvalidAchievementPayload
	}
	parsed, err := parseAchievementDate(value)
	if err != nil {
		return false, nil, err
	}
	return true, &parsed, nil
}

func parseAchievementYear(value string) (int, bool) {
	if len(value) != 4 {
		return 0, false
	}
	year, err := strconv.Atoi(value)
	if err != nil || year < minimumAchievementDate.Year() || year > 9999 {
		return 0, false
	}
	return year, true
}

func validAchievementKind(value string) bool {
	switch value {
	case "award", "certificate", "project":
		return true
	default:
		return false
	}
}

func validAchievementStatus(value string) bool {
	switch value {
	case "draft", "published", "archived":
		return true
	default:
		return false
	}
}

func validAchievementURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func resolveAchievementImage(db *gorm.DB, raw rawJSON) (bool, *uuid.UUID, error) {
	if len(raw) == 0 {
		return false, nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return false, nil, errInvalidAchievementPayload
	}
	asset, err := mediaref.ResolveReadyMedia(db, value)
	if err != nil {
		return false, nil, err
	}
	return true, &asset.ID, nil
}

func loadAchievement(db *gorm.DB, id string) (model.Achievement, error) {
	var item model.Achievement
	err := db.Preload("ImageMedia").First(&item, "id = ?", id).Error
	return item, err
}

func writeAchievementLoadError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		Fail(c, http.StatusNotFound, "ACHIEVEMENT_NOT_FOUND", "achievement not found")
		return
	}
	Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load achievement")
}

func writeAchievementMutationError(c *gin.Context, err error, operation string) {
	if errors.Is(err, errInvalidAchievementPayload) || strings.HasPrefix(postgresConstraint(err), "achievements_") {
		Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid achievement payload")
		return
	}
	if errors.Is(err, mediaref.ErrMediaNotFound) {
		Fail(c, http.StatusNotFound, "MEDIA_NOT_FOUND", "achievement image media not found")
		return
	}
	Fail(c, http.StatusConflict, "CONFLICT", "could not "+operation+" achievement")
}
