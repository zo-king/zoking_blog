package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type contentAccessMode int

const (
	contentAccessRead contentAccessMode = iota
	contentAccessManage
)

func currentContentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err == nil && userID != uuid.Nil {
		return userID, true
	}

	Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "valid user identity required")
	c.Abort()
	return uuid.Nil, false
}

func hasGlobalContentAccess(c *gin.Context, mode contentAccessMode) bool {
	if !validContentAccessMode(mode) {
		return false
	}

	for _, permission := range contentContextStrings(c, "permissions") {
		if permission == "*" || permission == "content:manage_all" {
			return true
		}
		if mode == contentAccessRead && permission == "content:read_all" {
			return true
		}
	}

	for _, role := range contentContextStrings(c, "roles") {
		switch role {
		case "super_admin", "admin", "editor":
			return true
		case "viewer":
			return mode == contentAccessRead
		}
	}

	return false
}

func scopeContentQuery(c *gin.Context, query *gorm.DB, table string, mode contentAccessMode) (*gorm.DB, bool) {
	if query == nil {
		failContentScope(c)
		return nil, false
	}

	var ownerPredicate string
	switch table {
	case "posts":
		ownerPredicate = "posts.author_id = ?"
	case "pages":
		ownerPredicate = "pages.author_id = ?"
	default:
		return failClosedContentQuery(c, query)
	}

	return scopeOwnedContentQuery(c, query, ownerPredicate, 1, mode)
}

func scopePublishQuery(c *gin.Context, query *gorm.DB, table string, mode contentAccessMode) (*gorm.DB, bool) {
	if query == nil {
		failContentScope(c)
		return nil, false
	}

	var ownerPredicate string
	switch table {
	case "publish_jobs":
		ownerPredicate = "(EXISTS (SELECT 1 FROM posts WHERE posts.id = publish_jobs.post_id AND posts.author_id = ?) OR EXISTS (SELECT 1 FROM pages WHERE pages.id = publish_jobs.page_id AND pages.author_id = ?))"
	case "publish_releases":
		ownerPredicate = "(EXISTS (SELECT 1 FROM posts WHERE posts.id = publish_releases.post_id AND posts.author_id = ?) OR EXISTS (SELECT 1 FROM pages WHERE pages.id = publish_releases.page_id AND pages.author_id = ?))"
	case "publish_previews":
		ownerPredicate = "(EXISTS (SELECT 1 FROM posts WHERE posts.id = publish_previews.post_id AND posts.author_id = ?) OR EXISTS (SELECT 1 FROM pages WHERE pages.id = publish_previews.page_id AND pages.author_id = ?))"
	default:
		return failClosedContentQuery(c, query)
	}

	return scopeOwnedContentQuery(c, query, ownerPredicate, 2, mode)
}

func scopeOwnedContentQuery(c *gin.Context, query *gorm.DB, ownerPredicate string, ownerBindings int, mode contentAccessMode) (*gorm.DB, bool) {
	if !validContentAccessMode(mode) {
		return failClosedContentQuery(c, query)
	}

	userID, ok := currentContentUserID(c)
	if !ok {
		return query.Where("1 = 0"), false
	}
	if hasGlobalContentAccess(c, mode) {
		return query, true
	}

	switch ownerBindings {
	case 1:
		return query.Where(ownerPredicate, userID), true
	case 2:
		return query.Where(ownerPredicate, userID, userID), true
	default:
		return failClosedContentQuery(c, query)
	}
}

func failClosedContentQuery(c *gin.Context, query *gorm.DB) (*gorm.DB, bool) {
	failContentScope(c)
	return query.Where("1 = 0"), false
}

func failContentScope(c *gin.Context) {
	Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "invalid content access scope")
	c.Abort()
}

func validContentAccessMode(mode contentAccessMode) bool {
	return mode == contentAccessRead || mode == contentAccessManage
}

func contentContextStrings(c *gin.Context, key string) []string {
	value, ok := c.Get(key)
	if !ok {
		return nil
	}
	values, _ := value.([]string)
	return values
}
