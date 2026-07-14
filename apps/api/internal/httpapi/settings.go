package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type siteSettingsPatchRequest struct {
	Site *struct {
		Title   *string `json:"title"`
		BaseURL *string `json:"base_url"`
	} `json:"site"`
	Sidebar *struct {
		Emoji    *string `json:"emoji"`
		Subtitle *string `json:"subtitle"`
	} `json:"sidebar"`
	Comments *struct {
		Enabled *bool   `json:"enabled"`
		APIBase *string `json:"api_base"`
	} `json:"comments"`
	Footer *struct {
		Since *int `json:"since"`
	} `json:"footer"`
	Pagination *struct {
		PagerSize *int `json:"pager_size"`
	} `json:"pagination"`
}

type siteSettingDefinition struct {
	Description string
	Public      bool
}

var siteSettingDefinitions = map[string]siteSettingDefinition{
	"site.title":            {Description: "Public site title", Public: true},
	"site.base_url":         {Description: "Public site canonical base URL", Public: true},
	"sidebar.subtitle":      {Description: "Public sidebar subtitle", Public: true},
	"sidebar.emoji":         {Description: "Public sidebar emoji mark", Public: true},
	"comments.enabled":      {Description: "Public comments switch", Public: true},
	"comments.api_base":     {Description: "Public comments API base URL", Public: true},
	"footer.since":          {Description: "Public footer start year", Public: true},
	"pagination.pager_size": {Description: "Public home/list pager size", Public: true},
}

func getPublicSiteSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, hash, err := publisher.LoadSiteSettings(c.Request.Context(), db)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "SETTINGS_LOAD_FAILED", "could not load site settings")
			return
		}
		OK(c, gin.H{
			"settings": settings,
			"hash":     hash,
		})
	}
}

func getAdminSiteSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, hash, err := publisher.LoadSiteSettings(c.Request.Context(), db)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "SETTINGS_LOAD_FAILED", "could not load site settings")
			return
		}
		OK(c, gin.H{
			"settings": settings,
			"hash":     hash,
			"keys":     allowedSiteSettingKeys(),
		})
	}
}

func patchAdminSiteSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req siteSettingsPatchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid settings payload")
			return
		}

		settings, _, err := publisher.LoadSiteSettings(c.Request.Context(), db)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "SETTINGS_LOAD_FAILED", "could not load site settings")
			return
		}

		updates, err := applySettingsPatch(&settings, req)
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}
		if err := validateSiteSettings(settings); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error())
			return
		}

		if len(updates) > 0 {
			if err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
				for key, value := range updates {
					if err := upsertSiteSetting(tx, key, value); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				Fail(c, http.StatusConflict, "SETTINGS_UPDATE_FAILED", "could not update site settings")
				return
			}
		}

		settings, hash, err := publisher.LoadSiteSettings(c.Request.Context(), db)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "SETTINGS_LOAD_FAILED", "could not reload site settings")
			return
		}
		OK(c, gin.H{
			"settings": settings,
			"hash":     hash,
			"updated":  len(updates),
		})
	}
}

func publishAdminSiteSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := publisher.ValidateSiteContent(c.Request.Context(), db); err != nil {
			if errors.Is(err, publisher.ErrContentQualityBlocked) {
				Fail(c, http.StatusUnprocessableEntity, "CONTENT_QUALITY_BLOCKED", "published content did not pass the quality check")
				return
			}
			Fail(c, http.StatusInternalServerError, "CONTENT_QUALITY_CHECK_FAILED", "could not check published content")
			return
		}
		requestedBy := parseOptionalUUID(c.GetString("user_id"))
		job := model.PublishJob{
			JobType:       "site",
			Status:        "requested",
			TriggerSource: "admin",
			RequestedBy:   requestedBy,
			RunAt:         time.Now(),
		}
		if err := db.WithContext(c.Request.Context()).Create(&job).Error; err != nil {
			Fail(c, http.StatusConflict, "PUBLISH_JOB_CONFLICT", "could not create publish job")
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"data": gin.H{
				"job": job,
			},
			"request_id": requestID(c),
		})
	}
}

func applySettingsPatch(settings *publisher.SiteSettingsSnapshot, req siteSettingsPatchRequest) (map[string]interface{}, error) {
	updates := map[string]interface{}{}
	if req.Site != nil {
		if req.Site.Title != nil {
			value := strings.TrimSpace(*req.Site.Title)
			if value == "" {
				return nil, fmt.Errorf("site title is required")
			}
			settings.Site.Title = value
			updates["site.title"] = value
		}
		if req.Site.BaseURL != nil {
			value, err := normalizeHTTPURL(*req.Site.BaseURL, true)
			if err != nil {
				return nil, fmt.Errorf("site base_url must be a valid http(s) URL")
			}
			settings.Site.BaseURL = value
			updates["site.base_url"] = value
		}
	}
	if req.Sidebar != nil {
		if req.Sidebar.Emoji != nil {
			value := strings.TrimSpace(*req.Sidebar.Emoji)
			if value == "" {
				return nil, fmt.Errorf("sidebar emoji is required")
			}
			settings.Sidebar.Emoji = value
			updates["sidebar.emoji"] = value
		}
		if req.Sidebar.Subtitle != nil {
			value := strings.TrimSpace(*req.Sidebar.Subtitle)
			if value == "" {
				return nil, fmt.Errorf("sidebar subtitle is required")
			}
			settings.Sidebar.Subtitle = value
			updates["sidebar.subtitle"] = value
		}
	}
	if req.Comments != nil {
		if req.Comments.Enabled != nil {
			settings.Comments.Enabled = *req.Comments.Enabled
			updates["comments.enabled"] = *req.Comments.Enabled
		}
		if req.Comments.APIBase != nil {
			value, err := normalizeHTTPURL(*req.Comments.APIBase, false)
			if err != nil {
				return nil, fmt.Errorf("comments api_base must be a valid http(s) URL")
			}
			settings.Comments.APIBase = value
			updates["comments.api_base"] = value
		}
	}
	if req.Footer != nil && req.Footer.Since != nil {
		if *req.Footer.Since < 1900 || *req.Footer.Since > time.Now().Year()+1 {
			return nil, fmt.Errorf("footer since year is out of range")
		}
		settings.Footer.Since = *req.Footer.Since
		updates["footer.since"] = *req.Footer.Since
	}
	if req.Pagination != nil && req.Pagination.PagerSize != nil {
		if *req.Pagination.PagerSize < 1 || *req.Pagination.PagerSize > 50 {
			return nil, fmt.Errorf("pagination pager_size must be between 1 and 50")
		}
		settings.Pagination.PagerSize = *req.Pagination.PagerSize
		updates["pagination.pager_size"] = *req.Pagination.PagerSize
	}
	return updates, nil
}

func validateSiteSettings(settings publisher.SiteSettingsSnapshot) error {
	if strings.TrimSpace(settings.Site.Title) == "" {
		return fmt.Errorf("site title is required")
	}
	if _, err := normalizeHTTPURL(settings.Site.BaseURL, true); err != nil {
		return fmt.Errorf("site base_url must be a valid http(s) URL")
	}
	if strings.TrimSpace(settings.Sidebar.Subtitle) == "" {
		return fmt.Errorf("sidebar subtitle is required")
	}
	if strings.TrimSpace(settings.Sidebar.Emoji) == "" {
		return fmt.Errorf("sidebar emoji is required")
	}
	if settings.Comments.Enabled {
		if _, err := normalizeHTTPURL(settings.Comments.APIBase, false); err != nil {
			return fmt.Errorf("comments api_base must be a valid http(s) URL")
		}
	}
	if settings.Footer.Since < 1900 || settings.Footer.Since > time.Now().Year()+1 {
		return fmt.Errorf("footer since year is out of range")
	}
	if settings.Pagination.PagerSize < 1 || settings.Pagination.PagerSize > 50 {
		return fmt.Errorf("pagination pager_size must be between 1 and 50")
	}
	return nil
}

func normalizeHTTPURL(value string, forceTrailingSlash bool) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("empty url")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	normalized := parsed.String()
	if forceTrailingSlash && !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}
	if !forceTrailingSlash {
		normalized = strings.TrimRight(normalized, "/")
	}
	return normalized, nil
}

func upsertSiteSetting(tx *gorm.DB, key string, value interface{}) error {
	definition, ok := siteSettingDefinitions[key]
	if !ok {
		return fmt.Errorf("unsupported setting key: %s", key)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	setting := model.SiteSetting{
		Key:         key,
		ValueJSON:   raw,
		Description: definition.Description,
		IsPublic:    definition.Public,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"value_json":  raw,
			"description": definition.Description,
			"is_public":   definition.Public,
			"updated_at":  time.Now(),
		}),
	}).Create(&setting).Error
}

func allowedSiteSettingKeys() []string {
	return []string{
		"site.title",
		"site.base_url",
		"sidebar.subtitle",
		"sidebar.emoji",
		"comments.enabled",
		"comments.api_base",
		"footer.since",
		"pagination.pager_size",
	}
}
