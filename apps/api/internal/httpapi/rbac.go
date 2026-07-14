package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errInactiveUser = errors.New("user is inactive or deleted")

const denyPermission = "__deny__"

type accessProfile struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

func authorizationMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		required := permissionForRoute(c.Request.Method, c.FullPath())
		profile, err := loadAccessProfile(c, db)
		if errors.Is(err, errInactiveUser) {
			Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "user is inactive or deleted")
			c.Abort()
			return
		}
		if err != nil {
			Fail(c, http.StatusServiceUnavailable, "AUTHORIZATION_UNAVAILABLE", "could not load permissions")
			c.Abort()
			return
		}
		c.Set("roles", profile.Roles)
		c.Set("permissions", profile.Permissions)
		if required == "" || containsPermission(profile.Permissions, required) {
			c.Next()
			return
		}
		Fail(c, http.StatusForbidden, "FORBIDDEN", "permission required: "+required)
		c.Abort()
	}
}

func loadAccessProfile(c *gin.Context, db *gorm.DB) (accessProfile, error) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return accessProfile{}, err
	}
	var activeUsers int64
	if err := db.WithContext(c.Request.Context()).Table("users").
		Where("id = ? and status = 'active' and deleted_at is null", userID).
		Count(&activeUsers).Error; err != nil {
		return accessProfile{}, err
	}
	if activeUsers != 1 {
		return accessProfile{}, errInactiveUser
	}
	var roles []string
	if err := db.WithContext(c.Request.Context()).Table("roles r").
		Select("distinct r.code").
		Joins("join user_roles ur on ur.role_id = r.id").
		Where("ur.user_id = ?", userID).
		Pluck("r.code", &roles).Error; err != nil {
		return accessProfile{}, err
	}
	var permissions []string
	if err := db.WithContext(c.Request.Context()).Table("permissions p").
		Select("distinct p.code").
		Joins("join role_permissions rp on rp.permission_id = p.id").
		Joins("join user_roles ur on ur.role_id = rp.role_id").
		Where("ur.user_id = ?", userID).
		Pluck("p.code", &permissions).Error; err != nil {
		return accessProfile{}, err
	}
	sort.Strings(roles)
	sort.Strings(permissions)
	return accessProfile{Roles: roles, Permissions: permissions}, nil
}

func containsPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if permission == required || permission == "*" {
			return true
		}
	}
	return false
}

func permissionForRoute(method string, route string) string {
	path := strings.TrimPrefix(route, "/api/v1/admin/")
	if path == "auth/me" || path == "auth/logout" || path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	resource := parts[0]
	switch resource {
	case "posts":
		return contentPermission("post", method, parts)
	case "pages":
		return contentPermission("page", method, parts)
	case "categories", "tags", "series":
		if method == http.MethodGet {
			return "taxonomy:read"
		}
		return "taxonomy:manage"
	case "achievements":
		if strings.Contains(path, "/publish") || strings.HasSuffix(path, "/status") {
			return "achievement:publish"
		}
		if method == http.MethodGet {
			return "achievement:read"
		}
		if method == http.MethodPost && len(parts) == 1 {
			return "achievement:create"
		}
		if method == http.MethodDelete {
			return "achievement:delete"
		}
		return "achievement:update"
	case "media":
		if method == http.MethodGet {
			return "media:read"
		}
		if method == http.MethodPost && len(parts) == 1 {
			return "media:upload"
		}
		return "media:delete"
	case "comments":
		if method == http.MethodGet {
			return "comment:read"
		}
		return "comment:moderate"
	case "publish":
		if method == http.MethodGet {
			return "publish:read"
		}
		if strings.Contains(path, "/cleanup") {
			return "publish:cleanup"
		}
		if strings.Contains(path, "/promote") {
			return "publish:rollback"
		}
		return "publish:create"
	case "settings":
		if method == http.MethodGet {
			return "setting:read"
		}
		return "setting:update"
	case "audit-logs":
		return "audit:read"
	case "users":
		if method == http.MethodGet {
			return "user:read"
		}
		return "user:manage"
	case "roles":
		if method == http.MethodGet {
			return "role:read"
		}
		return "role:manage"
	case "permissions":
		return "role:read"
	case "qa":
		return "qa:cleanup"
	default:
		return denyPermission
	}
}

func contentPermission(resource string, method string, parts []string) string {
	if method == http.MethodGet {
		return resource + ":read"
	}
	if method == http.MethodPost && parts[len(parts)-1] == "quality-check" {
		if len(parts) == 2 {
			return resource + ":create"
		}
		return resource + ":update"
	}
	if len(parts) > 2 && parts[len(parts)-1] == "preview" {
		return resource + ":update"
	}
	if len(parts) > 2 && parts[len(parts)-1] == "publish" {
		return resource + ":publish"
	}
	if method == http.MethodPost && len(parts) == 1 {
		return resource + ":create"
	}
	if method == http.MethodDelete {
		return resource + ":delete"
	}
	return resource + ":update"
}
