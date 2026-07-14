package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errContentPublishInProgress = errors.New("content publish is already in progress")
	activePublishStatuses       = []string{"requested", "queued", "snapshotting", "building", "verifying", "promoting"}
)

func deletePost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var post model.Post
		var job model.PublishJob
		accessOK := true
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			query, ok := scopeContentQuery(c, tx, "posts", contentAccessManage)
			if !ok {
				accessOK = false
				return nil
			}
			var err error
			post, err = loadPostWithCoverAndTaxonomy(query.Clauses(clause.Locking{Strength: "UPDATE"}), "id = ?", c.Param("id"))
			if err != nil {
				return err
			}
			inProgress, err := contentPublishInProgress(tx, "post_id", post.ID)
			if err != nil {
				return err
			}
			if inProgress {
				return errContentPublishInProgress
			}

			job = newWithdrawalJob(c, "withdraw_post")
			job.PostID = &post.ID
			return tx.Create(&job).Error
		})
		if !accessOK {
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
			return
		}
		if errors.Is(err, errContentPublishInProgress) {
			Fail(c, http.StatusConflict, "CONTENT_PUBLISH_IN_PROGRESS", "post has a publish job in progress")
			return
		}
		if err != nil {
			Fail(c, http.StatusConflict, "CONTENT_WITHDRAW_REQUEST_FAILED", "could not request post withdrawal")
			return
		}

		setAuditSnapshot(c, auditBeforeKey, gin.H{
			"id": post.ID, "title": post.Title, "slug": post.Slug, "status": post.Status,
			"visibility": post.Visibility, "published_at": post.PublishedAt,
		})
		setAuditSnapshot(c, auditAfterKey, gin.H{
			"state": "withdrawal_requested", "job_id": job.ID, "job_type": job.JobType,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"data":       gin.H{"deleted": false, "withdrawal_requested": true, "job": job},
			"request_id": requestID(c),
		})
	}
}

func deletePage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var page model.Page
		var job model.PublishJob
		accessOK := true
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			query, ok := scopeContentQuery(c, tx, "pages", contentAccessManage)
			if !ok {
				accessOK = false
				return nil
			}
			if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).First(&page, "id = ?", c.Param("id")).Error; err != nil {
				return err
			}
			inProgress, err := contentPublishInProgress(tx, "page_id", page.ID)
			if err != nil {
				return err
			}
			if inProgress {
				return errContentPublishInProgress
			}

			job = newWithdrawalJob(c, "withdraw_page")
			job.PageID = &page.ID
			return tx.Create(&job).Error
		})
		if !accessOK {
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(c, http.StatusNotFound, "PAGE_NOT_FOUND", "page not found")
			return
		}
		if errors.Is(err, errContentPublishInProgress) {
			Fail(c, http.StatusConflict, "CONTENT_PUBLISH_IN_PROGRESS", "page has a publish job in progress")
			return
		}
		if err != nil {
			Fail(c, http.StatusConflict, "CONTENT_WITHDRAW_REQUEST_FAILED", "could not request page withdrawal")
			return
		}

		setAuditSnapshot(c, auditBeforeKey, gin.H{
			"id": page.ID, "title": page.Title, "slug": page.Slug, "status": page.Status,
			"visibility": page.Visibility, "published_at": page.PublishedAt,
		})
		setAuditSnapshot(c, auditAfterKey, gin.H{
			"state": "withdrawal_requested", "job_id": job.ID, "job_type": job.JobType,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"data":       gin.H{"deleted": false, "withdrawal_requested": true, "job": job},
			"request_id": requestID(c),
		})
	}
}

func contentPublishInProgress(db *gorm.DB, foreignKey string, resourceID interface{}) (bool, error) {
	var count int64
	if err := db.Model(&model.PublishJob{}).
		Where(foreignKey+" = ? and status in ?", resourceID, activePublishStatuses).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func newWithdrawalJob(c *gin.Context, jobType string) model.PublishJob {
	return model.PublishJob{
		JobType:       jobType,
		Status:        "requested",
		TriggerSource: "admin",
		RequestedBy:   parseOptionalUUID(c.GetString("user_id")),
		RunAt:         time.Now(),
	}
}
