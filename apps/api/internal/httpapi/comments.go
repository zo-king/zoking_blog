package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

type publicCommentDTO struct {
	ID            uuid.UUID  `json:"id"`
	ParentID      *uuid.UUID `json:"parent_id"`
	AuthorName    string     `json:"author_name"`
	AuthorWebsite string     `json:"author_website"`
	Content       string     `json:"content"`
	CreatedAt     time.Time  `json:"created_at"`
}

func newPublicCommentDTO(comment model.Comment) publicCommentDTO {
	return publicCommentDTO{
		ID:            comment.ID,
		ParentID:      comment.ParentID,
		AuthorName:    comment.AuthorName,
		AuthorWebsite: comment.AuthorWebsite,
		Content:       comment.Content,
		CreatedAt:     comment.CreatedAt,
	}
}

func listPublicComments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePublicPagination(c, publicCommentLimit)
		if !ok {
			return
		}
		var post model.Post
		if err := db.WithContext(c.Request.Context()).
			Where("slug = ? and status = ? and visibility = ?", c.Param("slug"), "published", "public").
			First(&post).Error; err != nil {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}

		query := db.WithContext(c.Request.Context()).
			Model(&model.Comment{}).
			Where("post_id = ? and status = ?", post.ID, "approved")
		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count comments")
			return
		}
		var comments []publicCommentDTO
		if err := query.
			Select("id", "parent_id", "author_name", "author_website", "content", "created_at").
			Order("created_at asc, id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&comments).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list comments")
			return
		}
		setPublicPaginationHeaders(c, total, pagination)
		OK(c, comments)
	}
}

func submitPublicComment(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		ParentID      *uuid.UUID `json:"parent_id"`
		AuthorName    string     `json:"author_name" binding:"required"`
		AuthorEmail   string     `json:"author_email"`
		AuthorWebsite string     `json:"author_website"`
		Content       string     `json:"content" binding:"required"`
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid comment payload")
			return
		}

		authorName := strings.TrimSpace(req.AuthorName)
		content := normalizeCommentContent(req.Content)
		if authorName == "" || content == "" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid comment payload")
			return
		}
		if len([]rune(authorName)) > 80 || len([]rune(content)) > 2000 {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "comment payload is too long")
			return
		}

		var post model.Post
		if err := db.WithContext(c.Request.Context()).
			Where("slug = ? and status = ? and visibility = ?", c.Param("slug"), "published", "public").
			First(&post).Error; err != nil {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}
		if !post.AllowComment {
			Fail(c, http.StatusConflict, "CONFLICT", "comments are disabled for this post")
			return
		}
		if req.ParentID != nil {
			var parent model.Comment
			if err := db.WithContext(c.Request.Context()).
				Where("post_id = ?", post.ID).
				First(&parent, "id = ?", *req.ParentID).Error; err != nil {
				Fail(c, http.StatusNotFound, "NOT_FOUND", "parent comment not found")
				return
			}
		}

		comment := model.Comment{
			PostID:          post.ID,
			ParentID:        req.ParentID,
			AuthorName:      authorName,
			AuthorEmailHash: hashString(strings.ToLower(strings.TrimSpace(req.AuthorEmail))),
			AuthorWebsite:   strings.TrimSpace(req.AuthorWebsite),
			Content:         content,
			Status:          "pending",
			IPHash:          hashString(c.ClientIP()),
			UserAgent:       trimTo(c.GetHeader("User-Agent"), 512),
		}
		if err := db.WithContext(c.Request.Context()).Create(&comment).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not create comment")
			return
		}
		Created(c, newPublicCommentDTO(comment))
	}
}

func listAdminComments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}
		order, ok := adminListOrder(pagination.Sort, map[string]string{
			"created_at":  "comments.created_at",
			"updated_at":  "comments.updated_at",
			"author_name": "comments.author_name",
			"status":      "comments.status",
		})
		if !ok {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
			return
		}

		query := db.WithContext(c.Request.Context()).
			Model(&model.Comment{})
		if pagination.Query != "" {
			pattern := "%" + pagination.Query + "%"
			query = query.Where(
				"comments.author_name ILIKE ? OR comments.author_email_hash = ? OR comments.content ILIKE ?",
				pattern,
				hashString(strings.ToLower(pagination.Query)),
				pattern,
			)
		}
		if pagination.Status != "" {
			query = query.Where("comments.status = ?", pagination.Status)
		}
		if postID := strings.TrimSpace(c.Query("post_id")); postID != "" {
			parsedPostID, err := uuid.Parse(postID)
			if err != nil {
				Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid post_id")
				return
			}
			query = query.Where("comments.post_id = ?", parsedPostID)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count comments")
			return
		}
		if returnEmptyPageIfOutOfRange[model.Comment](c, total, pagination) {
			return
		}
		var comments []model.Comment
		if err := query.
			Preload("Post", func(db *gorm.DB) *gorm.DB { return db.Select("id", "title", "slug") }).
			Order(order + ", comments.id asc").
			Offset(pagination.Offset).
			Limit(pagination.PageSize).
			Find(&comments).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list comments")
			return
		}
		OKPaginated(c, comments, total, pagination)
	}
}

func moderateComment(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Status     string `json:"status" binding:"required"`
		SpamReason string `json:"spam_reason"`
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid moderation payload")
			return
		}
		status := strings.TrimSpace(req.Status)
		if status != "approved" && status != "rejected" && status != "spam" && status != "pending" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid comment status")
			return
		}

		var comment model.Comment
		if err := db.WithContext(c.Request.Context()).First(&comment, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "comment not found")
			return
		}

		now := time.Now()
		reviewer := parseOptionalUUID(c.GetString("user_id"))
		updates := map[string]interface{}{
			"status":      status,
			"reviewed_at": now,
			"reviewed_by": reviewer,
			"spam_reason": strings.TrimSpace(req.SpamReason),
		}
		if status == "pending" {
			updates["reviewed_at"] = nil
			updates["reviewed_by"] = nil
		}

		if err := db.WithContext(c.Request.Context()).Model(&comment).Updates(updates).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not moderate comment")
			return
		}
		if err := db.WithContext(c.Request.Context()).Preload("Post").First(&comment, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "comment not found")
			return
		}
		OK(c, comment)
	}
}

func replyComment(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Content string `json:"content" binding:"required"`
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid reply payload")
			return
		}
		content := normalizeCommentContent(req.Content)
		if content == "" || len([]rune(content)) > 2000 {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid reply payload")
			return
		}

		var parent model.Comment
		if err := db.WithContext(c.Request.Context()).First(&parent, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "comment not found")
			return
		}

		reply := model.Comment{
			PostID:     parent.PostID,
			ParentID:   &parent.ID,
			AuthorName: "Zoking Admin",
			Content:    content,
			Status:     "approved",
			UserAgent:  "admin-reply",
		}
		now := time.Now()
		reply.ReviewedAt = &now
		reply.ReviewedBy = parseOptionalUUID(c.GetString("user_id"))
		if err := db.WithContext(c.Request.Context()).Create(&reply).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not create reply")
			return
		}
		Created(c, reply)
	}
}

func deleteComment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var comment model.Comment
		if err := db.WithContext(c.Request.Context()).First(&comment, "id = ?", c.Param("id")).Error; err != nil {
			Fail(c, http.StatusNotFound, "NOT_FOUND", "comment not found")
			return
		}
		if err := db.WithContext(c.Request.Context()).
			Model(&comment).
			Update("status", "deleted").Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not delete comment")
			return
		}
		if err := db.WithContext(c.Request.Context()).Delete(&comment).Error; err != nil {
			Fail(c, http.StatusConflict, "CONFLICT", "could not delete comment")
			return
		}
		OK(c, gin.H{"deleted": true})
	}
}

func normalizeCommentContent(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSpace(value)
	return trimTo(value, 2000)
}

func trimTo(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func hashString(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
