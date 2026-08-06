package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

// 后台工作台概览统计(仪表盘图表数据源)。
// 权限:system:read(见 rbac.go 的 stats 路由映射)。

type statusCountDTO struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type dayViewsDTO struct {
	Day   string `json:"day"`
	Views int64  `json:"views"`
}

type topPostDTO struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Views int64  `json:"views"`
	Likes int64  `json:"likes"`
}

type dayCountDTO struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type statsOverviewDTO struct {
	Posts        []statusCountDTO `json:"posts"`
	Pages        []statusCountDTO `json:"pages"`
	Comments     []statusCountDTO `json:"comments"`
	PublishJobs  []statusCountDTO `json:"publish_jobs"`
	MediaCount   int64            `json:"media_count"`
	Achievements int64            `json:"achievement_count"`
	TotalViews   int64            `json:"total_views"`
	TotalLikes   int64            `json:"total_likes"`
	ViewsByDay   []dayViewsDTO    `json:"views_by_day"`
	LikesByDay   []dayCountDTO    `json:"likes_by_day"`
	TopPosts     []topPostDTO     `json:"top_posts"`
}

func countByStatus(db *gorm.DB, c *gin.Context, modelValue interface{}) ([]statusCountDTO, error) {
	rows := []statusCountDTO{}
	err := db.WithContext(c.Request.Context()).Model(modelValue).
		Select("status, count(*) as count").
		Group("status").Order("status").
		Scan(&rows).Error
	return rows, err
}

func getStatsOverview(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		out := statsOverviewDTO{ViewsByDay: []dayViewsDTO{}, LikesByDay: []dayCountDTO{}, TopPosts: []topPostDTO{}}
		var err error

		if out.Posts, err = countByStatus(db, c, &model.Post{}); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}
		if out.Pages, err = countByStatus(db, c, &model.Page{}); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}
		if out.Comments, err = countByStatus(db, c, &model.Comment{}); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}
		if out.PublishJobs, err = countByStatus(db, c, &model.PublishJob{}); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}
		if err = db.WithContext(ctx).Model(&model.MediaAsset{}).Count(&out.MediaCount).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}
		if err = db.WithContext(ctx).Model(&model.Achievement{}).Count(&out.Achievements).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}

		row := db.WithContext(ctx).Raw(
			`select coalesce(sum(views), 0), coalesce(sum(likes), 0) from post_metrics`,
		).Row()
		if err = row.Scan(&out.TotalViews, &out.TotalLikes); err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}

		// 近 14 天(UTC)每日浏览:post_view_marks 一行即一次去重后的浏览。
		since := time.Now().UTC().AddDate(0, 0, -13).Format("2006-01-02")
		if err = db.WithContext(ctx).Raw(
			`select to_char(view_day, 'YYYY-MM-DD') as day, count(*) as views
			 from post_view_marks where view_day >= ?::date
			 group by view_day order by view_day`,
			since,
		).Scan(&out.ViewsByDay).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}

		// 近 14 天(UTC)每日新增点赞;取消点赞会删行,曲线反映的是"当前仍有效的点赞"。
		if err = db.WithContext(ctx).Raw(
			`select to_char((created_at at time zone 'utc')::date, 'YYYY-MM-DD') as day, count(*) as count
			 from post_likes where created_at >= (now() at time zone 'utc')::date - interval '13 days'
			 group by 1 order by 1`,
		).Scan(&out.LikesByDay).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}

		if err = db.WithContext(ctx).Raw(
			`select p.id::text as id, p.title, m.views, m.likes
			 from post_metrics m join posts p on p.id = m.post_id
			 order by m.views desc, m.likes desc limit 5`,
		).Scan(&out.TopPosts).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load stats")
			return
		}

		OK(c, out)
	}
}
