package publisher

import (
	"context"
	"errors"
	"fmt"

	"github.com/zo-king/zoking_blog/apps/api/internal/contentquality"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

var ErrContentQualityBlocked = errors.New("content quality check failed")

type ContentQualityError struct {
	Kind   string
	ID     string
	Report contentquality.Report
}

func (err *ContentQualityError) Error() string {
	return fmt.Sprintf("%s %s failed content quality check with %d error(s)", err.Kind, err.ID, err.Report.ErrorCount)
}

func (err *ContentQualityError) Unwrap() error {
	return ErrContentQualityBlocked
}

func ValidatePostContent(post model.Post) error {
	if post.Status != "published" {
		return fmt.Errorf("%w: post %s status is %s", ErrContentQualityBlocked, post.ID, post.Status)
	}
	if err := validatePostSeries(post); err != nil {
		return fmt.Errorf("%w: post %s has invalid series metadata: %v", ErrContentQualityBlocked, post.ID, err)
	}
	return reportQualityError("post", post.ID.String(), contentquality.EvaluatePost(post))
}

func ValidatePageContent(page model.Page) error {
	if page.Status != "published" {
		return fmt.Errorf("%w: page %s status is %s", ErrContentQualityBlocked, page.ID, page.Status)
	}
	return reportQualityError("page", page.ID.String(), contentquality.EvaluatePage(page))
}

func ValidateSiteContent(ctx context.Context, db *gorm.DB) error {
	var posts []model.Post
	if err := db.WithContext(ctx).
		Where("status = ?", "published").
		Preload("Categories").
		Preload("Tags").
		Preload("CoverMedia").
		Preload("Series.CoverMedia").
		Order("id asc").
		Find(&posts).Error; err != nil {
		return err
	}
	for _, post := range posts {
		if err := ValidatePostContent(post); err != nil {
			return err
		}
	}

	var pages []model.Page
	if err := db.WithContext(ctx).
		Where("status = ?", "published").
		Order("id asc").
		Find(&pages).Error; err != nil {
		return err
	}
	for _, page := range pages {
		if err := ValidatePageContent(page); err != nil {
			return err
		}
	}
	return nil
}

func ValidateJobContent(ctx context.Context, db *gorm.DB, job model.PublishJob) error {
	switch job.JobType {
	case "post":
		if job.PostID == nil {
			return fmt.Errorf("%w: publish job has no post id", ErrContentQualityBlocked)
		}
		var post model.Post
		if err := db.WithContext(ctx).
			Preload("Categories").
			Preload("Tags").
			Preload("CoverMedia").
			Preload("Series.CoverMedia").
			First(&post, "id = ?", *job.PostID).Error; err != nil {
			return err
		}
		return ValidatePostContent(post)
	case "page":
		if job.PageID == nil {
			return fmt.Errorf("%w: publish job has no page id", ErrContentQualityBlocked)
		}
		var page model.Page
		if err := db.WithContext(ctx).First(&page, "id = ?", *job.PageID).Error; err != nil {
			return err
		}
		return ValidatePageContent(page)
	case "site":
		return ValidateSiteContent(ctx, db)
	default:
		return nil
	}
}

func reportQualityError(kind, id string, report contentquality.Report) error {
	if report.Ready {
		return nil
	}
	return &ContentQualityError{Kind: kind, ID: id, Report: report}
}
