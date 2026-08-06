package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

// postMetricsDTO is the public view/like payload for a post.
type postMetricsDTO struct {
	Views int64 `json:"views"`
	Likes int64 `json:"likes"`
	Liked bool  `json:"liked"`
}

// lookupPublishedPost loads a public, published post by :slug or writes a 404.
func lookupPublishedPost(db *gorm.DB, c *gin.Context) (model.Post, bool) {
	var post model.Post
	if err := db.WithContext(c.Request.Context()).
		Where("slug = ? and status = ? and visibility = ?", c.Param("slug"), "published", "public").
		First(&post).Error; err != nil {
		Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
		return model.Post{}, false
	}
	return post, true
}

// visitorHash derives a stable, non-reversible visitor key from the client IP.
// Reuses the same hashing the comment pipeline uses so no raw IPs are stored.
func visitorHash(c *gin.Context, secret string) string {
	if h := hashString(c.ClientIP(), secret); h != "" {
		return h
	}
	// Fallback keeps dedupe working even when the IP is unavailable.
	return hashString("anon:"+trimTo(c.GetHeader("User-Agent"), 256), secret)
}

// readPostMetrics returns the current counters plus whether this visitor liked the post.
// The query always returns exactly one row (0/0/false when no metrics exist yet).
func readPostMetrics(db *gorm.DB, c *gin.Context, post model.Post, visitor string) (postMetricsDTO, error) {
	var out postMetricsDTO
	row := db.WithContext(c.Request.Context()).Raw(
		`select
			coalesce((select views from post_metrics where post_id = ?), 0),
			coalesce((select likes from post_metrics where post_id = ?), 0),
			exists(select 1 from post_likes where post_id = ? and visitor_hash = ?)`,
		post.ID, post.ID, post.ID, visitor,
	).Row()
	if err := row.Scan(&out.Views, &out.Likes, &out.Liked); err != nil {
		return postMetricsDTO{}, err
	}
	return out, nil
}

// getPublicPostMetrics returns view/like counts for a post without mutating anything.
func getPublicPostMetrics(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		post, ok := lookupPublishedPost(db, c)
		if !ok {
			return
		}
		metrics, err := readPostMetrics(db, c, post, visitorHash(c, cfg.PrivacyHashSecret))
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not read metrics")
			return
		}
		OK(c, metrics)
	}
}

// recordPostView increments the view counter at most once per visitor per UTC day.
func recordPostView(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		post, ok := lookupPublishedPost(db, c)
		if !ok {
			return
		}
		visitor := visitorHash(c, cfg.PrivacyHashSecret)
		ctx := c.Request.Context()
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`insert into post_metrics (post_id) values (?) on conflict (post_id) do nothing`, post.ID).Error; err != nil {
				return err
			}
			res := tx.Exec(`insert into post_view_marks (post_id, visitor_hash) values (?, ?) on conflict do nothing`, post.ID, visitor)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 1 {
				if err := tx.Exec(`update post_metrics set views = views + 1, updated_at = now() where post_id = ?`, post.ID).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not record view")
			return
		}
		metrics, err := readPostMetrics(db, c, post, visitor)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not read metrics")
			return
		}
		OK(c, metrics)
	}
}

// togglePostLike flips this visitor's like state for the post and returns fresh counts.
func togglePostLike(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		post, ok := lookupPublishedPost(db, c)
		if !ok {
			return
		}
		visitor := visitorHash(c, cfg.PrivacyHashSecret)
		ctx := c.Request.Context()
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`insert into post_metrics (post_id) values (?) on conflict (post_id) do nothing`, post.ID).Error; err != nil {
				return err
			}
			var liked bool
			if err := tx.Raw(`select exists(select 1 from post_likes where post_id = ? and visitor_hash = ?)`, post.ID, visitor).Row().Scan(&liked); err != nil {
				return err
			}
			// 计数增减必须以 RowsAffected 为条件:并发下 insert 可能冲突、delete 可能落空,
			// 无条件更新会让计数与 post_likes 实际行数漂移。
			if liked {
				res := tx.Exec(`delete from post_likes where post_id = ? and visitor_hash = ?`, post.ID, visitor)
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					return nil
				}
				return tx.Exec(`update post_metrics set likes = greatest(likes - 1, 0), updated_at = now() where post_id = ?`, post.ID).Error
			}
			res := tx.Exec(`insert into post_likes (post_id, visitor_hash) values (?, ?) on conflict do nothing`, post.ID, visitor)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return nil
			}
			return tx.Exec(`update post_metrics set likes = likes + 1, updated_at = now() where post_id = ?`, post.ID).Error
		}); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not toggle like")
			return
		}
		metrics, err := readPostMetrics(db, c, post, visitor)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not read metrics")
			return
		}
		OK(c, metrics)
	}
}
