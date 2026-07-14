package publisher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const publishWorkerLockID int64 = 2026071101

type Worker struct {
	db           *gorm.DB
	cfg          config.Config
	pollInterval time.Duration
	logger       *log.Logger
}

func NewWorker(db *gorm.DB, cfg config.Config, logger *log.Logger) *Worker {
	pollInterval := cfg.PublishWorkerPollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if logger == nil {
		logger = log.Default()
	}

	return &Worker{
		db:           db,
		cfg:          cfg,
		pollInterval: pollInterval,
		logger:       logger,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Printf("publish worker started poll_interval=%s", w.pollInterval)
	if acquired, releaseLock, err := AcquirePublishWorkerLock(ctx, w.db); err != nil {
		return err
	} else if acquired {
		if err := RecoverStaleJobs(ctx, w.db, w.cfg); err != nil {
			releaseLock()
			return err
		}
		releaseLock()
	}
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		processed, err := w.ProcessOne(ctx)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		if err != nil {
			w.logger.Printf("publish worker process error: %v", err)
		}
		if processed {
			continue
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func RecoverStaleJobs(ctx context.Context, db *gorm.DB, cfg config.Config) error {
	if err := recoverTerminalPublicationStates(db); err != nil {
		return err
	}
	staleAfter := 10 * time.Minute
	if cfg.PublishJobTimeout > 0 && 2*cfg.PublishJobTimeout > staleAfter {
		staleAfter = 2 * cfg.PublishJobTimeout
	}
	activeStatuses := []string{"queued", "snapshotting", "building", "verifying", "promoting"}
	var jobs []model.PublishJob
	if err := db.WithContext(ctx).
		Where("status in ? and updated_at < ?", activeStatuses, time.Now().Add(-staleAfter)).
		Order("created_at asc").
		Find(&jobs).Error; err != nil {
		return err
	}
	var recoveryErrors []error
	for index := range jobs {
		job := &jobs[index]
		logs := parseJobLogs(job.LogJSON)
		failure := errors.New("publish worker was interrupted before the job reached a terminal state")
		code := "WORKER_INTERRUPTED"
		if err := restoreJobPublicationState(db, *job); err != nil {
			restoreErr := fmt.Errorf("restore interrupted publication content state: %w", err)
			failure = errors.Join(failure, restoreErr)
			code = "CONTENT_STATE_RESTORE_FAILED"
			recoveryErrors = append(recoveryErrors, restoreErr)
		}
		if job.JobType == "withdraw_post" && job.PostID != nil {
			var post model.Post
			if err := db.WithContext(ctx).
				Preload("Categories", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc, name asc") }).
				Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("name asc") }).
				Preload("CoverMedia").
				Preload("Series.CoverMedia").
				First(&post, "id = ?", *job.PostID).Error; err == nil {
				if _, err := WritePost(cfg.HugoSiteDir, post); err != nil {
					failure = fmt.Errorf("restore interrupted post withdrawal: %w", err)
					code = "SNAPSHOT_RESTORE_FAILED"
				}
			} else {
				failure = fmt.Errorf("load interrupted post withdrawal: %w", err)
				code = "SNAPSHOT_RESTORE_FAILED"
			}
		}
		if job.JobType == "withdraw_page" && job.PageID != nil {
			var page model.Page
			if err := db.WithContext(ctx).First(&page, "id = ?", *job.PageID).Error; err == nil {
				if _, err := WritePage(cfg.HugoSiteDir, page); err != nil {
					failure = fmt.Errorf("restore interrupted page withdrawal: %w", err)
					code = "SNAPSHOT_RESTORE_FAILED"
				}
			} else {
				failure = fmt.Errorf("load interrupted page withdrawal: %w", err)
				code = "SNAPSHOT_RESTORE_FAILED"
			}
		}
		if job.OutputPath != "" {
			removed, cleanupErr := cleanupUnregisteredJobOutput(ctx, db, cfg, job.ID, job.OutputPath)
			if cleanupErr != nil {
				cleanupErr = fmt.Errorf("recover stale job %s orphan release output: %w", job.ID, cleanupErr)
				addLog(&logs, "cleanup", "error", cleanupErr.Error(), map[string]string{
					"code":        "ORPHAN_RELEASE_CLEANUP_FAILED",
					"output_path": job.OutputPath,
				})
				failure = errors.Join(failure, cleanupErr)
				code = "ORPHAN_RELEASE_CLEANUP_FAILED"
				recoveryErrors = append(recoveryErrors, cleanupErr)
			} else if removed {
				addLog(&logs, "cleanup", "info", "removed stale unregistered release output", map[string]string{
					"output_path": job.OutputPath,
				})
			}
		}
		markJobFailedWithLogs(ctx, db, job, &logs, "failed", code, failure)
	}
	return errors.Join(recoveryErrors...)
}

func recoverTerminalPublicationStates(db *gorm.DB) error {
	queryCtx, cancel := context.WithTimeout(context.Background(), publicationRecoveryTimeout)
	defer cancel()
	var jobs []model.PublishJob
	err := db.WithContext(queryCtx).
		Where("status in ?", []string{"failed", "canceled"}).
		Where(`
			(job_type = 'post' and exists (
				select 1 from posts
				where posts.id = publish_jobs.post_id
				  and posts.status = 'publishing'
				  and posts.deleted_at is null
			))
			or
			(job_type = 'page' and exists (
				select 1 from pages
				where pages.id = publish_jobs.page_id
				  and pages.status = 'publishing'
				  and pages.deleted_at is null
			))`).
		Order("created_at asc").
		Find(&jobs).Error
	if err != nil {
		return fmt.Errorf("find terminal publish jobs with stuck content: %w", err)
	}
	var recoveryErrors []error
	for _, job := range jobs {
		if err := restoreJobPublicationState(db, job); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("restore terminal publish job %s content state: %w", job.ID, err))
		}
	}
	return errors.Join(recoveryErrors...)
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	acquired, releaseLock, err := AcquirePublishWorkerLock(ctx, w.db)
	if err != nil || !acquired {
		return false, err
	}
	defer releaseLock()
	if err := ReconcileCurrentRelease(ctx, w.db, w.cfg); err != nil {
		w.logger.Printf("publish worker current release reconciliation failed: %v", err)
	}
	if err := RecoverStaleJobs(ctx, w.db, w.cfg); err != nil {
		w.logger.Printf("publish worker stale job recovery failed: %v", err)
	}

	job, err := ClaimNextJob(ctx, w.db)
	if err != nil {
		return false, err
	}
	if job == nil {
		return false, nil
	}

	switch job.JobType {
	case "post":
		if job.PostID == nil {
			markJobFailed(ctx, w.db, job, "POST_NOT_FOUND", errors.New("publish job has no post id"))
			return true, nil
		}

		var post model.Post
		if err := w.db.WithContext(ctx).
			Preload("Categories", func(db *gorm.DB) *gorm.DB {
				return db.Order("sort_order asc, name asc")
			}).
			Preload("Tags", func(db *gorm.DB) *gorm.DB {
				return db.Order("name asc")
			}).
			Preload("CoverMedia").
			Preload("Series.CoverMedia").
			First(&post, "id = ?", *job.PostID).Error; err != nil {
			markJobFailed(ctx, w.db, job, "POST_NOT_FOUND", err)
			return true, nil
		}
		if err := ValidatePostContent(post); err != nil {
			markJobFailed(ctx, w.db, job, "CONTENT_QUALITY_BLOCKED", err)
			return true, nil
		}

		if _, err := ProcessPostJob(ctx, w.db, w.cfg, *job, post); err != nil {
			return true, err
		}
	case "page":
		if job.PageID == nil {
			markJobFailed(ctx, w.db, job, "PAGE_NOT_FOUND", errors.New("publish job has no page id"))
			return true, nil
		}
		var page model.Page
		if err := w.db.WithContext(ctx).First(&page, "id = ?", *job.PageID).Error; err != nil {
			markJobFailed(ctx, w.db, job, "PAGE_NOT_FOUND", err)
			return true, nil
		}
		if err := ValidatePageContent(page); err != nil {
			markJobFailed(ctx, w.db, job, "CONTENT_QUALITY_BLOCKED", err)
			return true, nil
		}
		if _, err := ProcessPageJob(ctx, w.db, w.cfg, *job, page); err != nil {
			return true, err
		}
	case "site":
		if err := ValidateSiteContent(ctx, w.db); err != nil {
			markJobFailed(ctx, w.db, job, "CONTENT_QUALITY_BLOCKED", err)
			return true, nil
		}
		if _, err := ProcessSiteJob(ctx, w.db, w.cfg, *job); err != nil {
			return true, err
		}
	case "withdraw_post":
		if job.PostID == nil {
			markJobFailed(ctx, w.db, job, "POST_NOT_FOUND", errors.New("withdrawal job has no post id"))
			return true, nil
		}
		var post model.Post
		if err := w.db.WithContext(ctx).
			Preload("Categories", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc, name asc") }).
			Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("name asc") }).
			Preload("CoverMedia").
			Preload("Series.CoverMedia").
			First(&post, "id = ?", *job.PostID).Error; err != nil {
			markJobFailed(ctx, w.db, job, "POST_NOT_FOUND", err)
			return true, nil
		}
		if _, err := ProcessPostWithdrawalJob(ctx, w.db, w.cfg, *job, post); err != nil {
			return true, err
		}
	case "withdraw_page":
		if job.PageID == nil {
			markJobFailed(ctx, w.db, job, "PAGE_NOT_FOUND", errors.New("withdrawal job has no page id"))
			return true, nil
		}
		var page model.Page
		if err := w.db.WithContext(ctx).First(&page, "id = ?", *job.PageID).Error; err != nil {
			markJobFailed(ctx, w.db, job, "PAGE_NOT_FOUND", err)
			return true, nil
		}
		if _, err := ProcessPageWithdrawalJob(ctx, w.db, w.cfg, *job, page); err != nil {
			return true, err
		}
	default:
		markJobFailed(ctx, w.db, job, "UNSUPPORTED_JOB_TYPE", errors.New("unsupported publish job type"))
		return true, nil
	}
	return true, nil
}

// AcquirePublishWorkerLock serializes operations that mutate Hugo source or current releases.
func AcquirePublishWorkerLock(ctx context.Context, db *gorm.DB) (bool, func(), error) {
	sqlDB, err := db.DB()
	if err != nil {
		return false, nil, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return false, nil, err
	}
	var acquired bool
	if err := conn.QueryRowContext(ctx, "select pg_try_advisory_lock($1)", publishWorkerLockID).Scan(&acquired); err != nil {
		_ = conn.Close()
		return false, nil, err
	}
	if !acquired {
		_ = conn.Close()
		return false, nil, nil
	}
	release := func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, "select pg_advisory_unlock($1)", publishWorkerLockID)
		_ = conn.Close()
	}
	return true, release, nil
}

func ClaimNextJob(ctx context.Context, db *gorm.DB) (*model.PublishJob, error) {
	var claimed model.PublishJob
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? and run_at <= ?", "requested", time.Now()).
			Order("run_at asc, created_at asc").
			First(&claimed).Error
		if err != nil {
			return err
		}
		return tx.Model(&claimed).Update("status", "queued").Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &claimed, nil
}
