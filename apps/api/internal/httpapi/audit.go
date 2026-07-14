package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

const (
	auditBeforeKey = "audit_before"
	auditAfterKey  = "audit_after"
)

func auditMiddleware(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		c.Next()
		if c.Request.Method == http.MethodGet && c.Writer.Status() != http.StatusForbidden {
			return
		}

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		resourceType, resourceID := auditResource(c, route)
		details, _ := json.Marshal(map[string]interface{}{
			"query_keys":  queryKeys(c),
			"error_count": len(c.Errors),
		})
		status := c.Writer.Status()
		result := "success"
		if status == http.StatusForbidden {
			result = "denied"
		} else if status >= http.StatusBadRequest {
			result = "failure"
		}
		userAgent := c.GetHeader("User-Agent")
		if len(userAgent) > 512 {
			userAgent = userAgent[:512]
		}
		entry := model.AuditLog{
			ActorID:       parseOptionalUUID(c.GetString("user_id")),
			ActorEmail:    c.GetString("user_email"),
			Action:        auditAction(c.Request.Method, route, resourceType),
			ResourceType:  resourceType,
			ResourceID:    resourceID,
			BeforeJSON:    auditSnapshot(c, auditBeforeKey),
			AfterJSON:     auditSnapshot(c, auditAfterKey),
			Route:         route,
			Method:        c.Request.Method,
			Result:        result,
			StatusCode:    status,
			RequestID:     requestID(c),
			IPHash:        hashAuditIP(c.ClientIP(), cfg.JWTSecret),
			IPHashVersion: 1,
			UserAgent:     userAgent,
			DetailsJSON:   details,
		}
		_ = db.WithContext(c.Request.Context()).Create(&entry).Error
	}
}

func setAuditSnapshot(c *gin.Context, key string, value interface{}) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	c.Set(key, raw)
}

func auditSnapshot(c *gin.Context, key string) []byte {
	value, ok := c.Get(key)
	if !ok {
		return []byte(`{}`)
	}
	raw, ok := value.([]byte)
	if !ok || len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func auditAction(method string, route string, resourceType string) string {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(route, "/api/v1/admin/"), "/"), "/")
	operation := "update"
	switch method {
	case http.MethodDelete:
		operation = "delete"
	case http.MethodPost:
		operation = "create"
	}
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if last != ":id" {
			operation = strings.ReplaceAll(last, "-", "_")
		}
	}
	return resourceType + "." + operation
}

func listAuditLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}
		order, ok := parseListOrder(c, pagination.Sort, map[string]string{
			"created_at":    "created_at",
			"actor_email":   "actor_email",
			"action":        "action",
			"resource_type": "resource_type",
			"result":        "result",
			"status_code":   "status_code",
			"request_id":    "request_id",
			"route":         "route",
		})
		if !ok {
			return
		}

		query := db.WithContext(c.Request.Context()).Model(&model.AuditLog{})
		if pagination.Query != "" {
			pattern := "%" + pagination.Query + "%"
			query = query.Where("actor_email ILIKE ? OR action ILIKE ? OR resource_type ILIKE ? OR request_id ILIKE ? OR route ILIKE ?", pattern, pattern, pattern, pattern, pattern)
		}
		if pagination.Status != "" {
			query = query.Where("result = ?", pagination.Status)
		}
		if value := strings.TrimSpace(c.Query("resource_type")); value != "" {
			query = query.Where("resource_type = ?", value)
		}
		if value := strings.TrimSpace(c.Query("result")); value != "" {
			query = query.Where("result = ?", value)
		}
		if value := strings.TrimSpace(c.Query("actor_id")); value != "" {
			parsedActorID, err := uuid.Parse(value)
			if err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid actor_id")
				return
			}
			query = query.Where("actor_id = ?", parsedActorID)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count audit logs")
			return
		}
		if returnEmptyPageIfOutOfRange[model.AuditLog](c, total, pagination) {
			return
		}

		var logs []model.AuditLog
		if err := query.Order(order + ", id ASC").Offset(pagination.Offset).Limit(pagination.PageSize).Find(&logs).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list audit logs")
			return
		}
		OKPaginated(c, logs, total, pagination)
	}
}

func auditResource(c *gin.Context, route string) (string, *uuid.UUID) {
	path := strings.TrimPrefix(route, "/api/v1/admin/")
	parts := strings.Split(path, "/")
	resourceType := "admin"
	if len(parts) > 0 && parts[0] != "" {
		resourceType = parts[0]
	}
	resourceID := parseOptionalUUID(c.Param("id"))
	return resourceType, resourceID
}

func queryKeys(c *gin.Context) []string {
	keys := make([]string, 0, len(c.Request.URL.Query()))
	for key := range c.Request.URL.Query() {
		keys = append(keys, key)
	}
	return keys
}

func hashAuditIP(value string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(mac.Sum(nil))
}
