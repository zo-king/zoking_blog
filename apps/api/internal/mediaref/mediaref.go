package mediaref

import (
	"errors"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ResourcePost        = "post"
	ResourcePage        = "page"
	ResourceRelease     = "release"
	ResourceSeries      = "series"
	ResourceAchievement = "achievement"
	UsageMarkdown       = "markdown"
	UsageCover          = "cover"
	UsageImage          = "image"
)

var ErrMediaNotFound = errors.New("media not found")

func ResolveReadyMedia(tx *gorm.DB, value string) (*model.MediaAsset, error) {
	mediaID, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, ErrMediaNotFound
	}
	var asset model.MediaAsset
	if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).
		Where("id = ? and status = ? and deleted_at is null", mediaID, "ready").First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMediaNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func SyncPostCoverUsage(tx *gorm.DB, postID uuid.UUID, coverMediaID *uuid.UUID) error {
	return withReferenceTransaction(tx, func(referenceTx *gorm.DB) error {
		return syncCoverUsage(referenceTx, ResourcePost, postID, coverMediaID)
	})
}

func SyncSeriesCoverUsage(tx *gorm.DB, seriesID uuid.UUID, coverMediaID *uuid.UUID) error {
	return withReferenceTransaction(tx, func(referenceTx *gorm.DB) error {
		return syncCoverUsage(referenceTx, ResourceSeries, seriesID, coverMediaID)
	})
}

func SyncReleaseCoverUsage(tx *gorm.DB, releaseID uuid.UUID, coverMediaID *uuid.UUID) error {
	return withReferenceTransaction(tx, func(referenceTx *gorm.DB) error {
		return syncCoverUsage(referenceTx, ResourceRelease, releaseID, coverMediaID)
	})
}

func SyncAchievementImageUsage(tx *gorm.DB, achievementID uuid.UUID, imageMediaID *uuid.UUID) error {
	return withReferenceTransaction(tx, func(referenceTx *gorm.DB) error {
		return syncUsage(referenceTx, ResourceAchievement, achievementID, UsageImage, imageMediaID)
	})
}

func syncCoverUsage(tx *gorm.DB, resourceType string, resourceID uuid.UUID, coverMediaID *uuid.UUID) error {
	return syncUsage(tx, resourceType, resourceID, UsageCover, coverMediaID)
}

func syncUsage(tx *gorm.DB, resourceType string, resourceID uuid.UUID, usageType string, mediaID *uuid.UUID) error {
	if err := tx.Where("resource_type = ? and resource_id = ? and usage_type = ?", resourceType, resourceID, usageType).
		Delete(&model.MediaUsage{}).Error; err != nil {
		return err
	}
	if mediaID == nil {
		return nil
	}
	if _, err := ResolveReadyMedia(tx, mediaID.String()); err != nil {
		return err
	}
	return tx.Create(&model.MediaUsage{
		MediaID:      *mediaID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		UsageType:    usageType,
	}).Error
}

func SyncPostMarkdownUsages(tx *gorm.DB, postID uuid.UUID, contentMD string) error {
	return withReferenceTransaction(tx, func(referenceTx *gorm.DB) error {
		return syncMarkdownUsages(referenceTx, ResourcePost, postID, UsageMarkdown, contentMD)
	})
}

func SyncPageMarkdownUsages(tx *gorm.DB, pageID uuid.UUID, contentMD string) error {
	return withReferenceTransaction(tx, func(referenceTx *gorm.DB) error {
		return syncMarkdownUsages(referenceTx, ResourcePage, pageID, UsageMarkdown, contentMD)
	})
}

func SyncReleaseMarkdownUsages(tx *gorm.DB, releaseID uuid.UUID, contentMD string) error {
	return withReferenceTransaction(tx, func(referenceTx *gorm.DB) error {
		return syncMarkdownUsages(referenceTx, ResourceRelease, releaseID, UsageMarkdown, contentMD)
	})
}

func withReferenceTransaction(db *gorm.DB, operation func(*gorm.DB) error) error {
	if db.DryRun {
		return operation(db)
	}
	return db.Transaction(operation)
}

func syncMarkdownUsages(tx *gorm.DB, resourceType string, resourceID uuid.UUID, usageType string, contentMD string) error {
	var assets []model.MediaAsset
	if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).
		Where("status = ? and deleted_at is null", "ready").
		Find(&assets).Error; err != nil {
		return err
	}

	referenced := make([]model.MediaUsage, 0)
	seen := make(map[uuid.UUID]struct{})
	for _, asset := range assets {
		if !ReferencedByMarkdown(contentMD, asset) {
			continue
		}
		if _, ok := seen[asset.ID]; ok {
			continue
		}
		seen[asset.ID] = struct{}{}
		referenced = append(referenced, model.MediaUsage{
			MediaID:      asset.ID,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			UsageType:    usageType,
		})
	}

	if err := tx.
		Where("resource_type = ? and resource_id = ? and usage_type = ?", resourceType, resourceID, usageType).
		Delete(&model.MediaUsage{}).Error; err != nil {
		return err
	}
	if len(referenced) == 0 {
		return nil
	}
	return tx.Create(&referenced).Error
}

func ReferencedByMarkdown(content string, asset model.MediaAsset) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}

	candidates := []string{
		strings.TrimSpace(asset.PublicURL),
		strings.TrimSpace(asset.StorageKey),
	}
	if asset.PublicURL != "" {
		if parsed, err := url.Parse(asset.PublicURL); err == nil {
			candidates = append(candidates, parsed.Path)
		}
	}
	if asset.StorageKey != "" {
		slashed := "/" + strings.TrimLeft(strings.ReplaceAll(asset.StorageKey, "\\", "/"), "/")
		candidates = append(candidates, slashed, url.PathEscape(asset.StorageKey))
	}

	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.Contains(content, candidate) {
			return true
		}
	}
	return false
}
