package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCategoriesNotFound = errors.New("some categories were not found")
	ErrTagsNotFound       = errors.New("some tags were not found")
	ErrSeriesNotFound     = errors.New("series not found")
)

type rawJSON = json.RawMessage

func preloadPostTaxonomy(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Series.CoverMedia").
		Preload("Categories", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order asc, name asc")
		}).
		Preload("Tags", func(db *gorm.DB) *gorm.DB {
			return db.Order("name asc")
		})
}

func parsePostSeriesFields(seriesIDRaw, seriesOrderRaw rawJSON) (bool, *uuid.UUID, *int, error) {
	seriesIDProvided := len(seriesIDRaw) > 0
	seriesOrderProvided := len(seriesOrderRaw) > 0
	if !seriesIDProvided && !seriesOrderProvided {
		return false, nil, nil, nil
	}
	if seriesIDProvided != seriesOrderProvided {
		return false, nil, nil, errors.New("series_id and series_order must be provided together")
	}

	seriesIDNull := bytes.Equal(bytes.TrimSpace(seriesIDRaw), []byte("null"))
	seriesOrderNull := bytes.Equal(bytes.TrimSpace(seriesOrderRaw), []byte("null"))
	var rawID string
	if !seriesIDNull {
		if err := json.Unmarshal(seriesIDRaw, &rawID); err != nil {
			return false, nil, nil, errors.New("series_id must be a UUID string or null")
		}
		seriesIDNull = strings.TrimSpace(rawID) == ""
	}
	if seriesIDNull || seriesOrderNull {
		if seriesIDNull && seriesOrderNull {
			return true, nil, nil, nil
		}
		return false, nil, nil, errors.New("series_id and series_order must both be null or both be set")
	}

	seriesID, err := uuid.Parse(rawID)
	if err != nil || seriesID == uuid.Nil {
		return false, nil, nil, errors.New("series_id must be a valid UUID")
	}
	var seriesOrder int
	if err := json.Unmarshal(seriesOrderRaw, &seriesOrder); err != nil || seriesOrder <= 0 {
		return false, nil, nil, errors.New("series_order must be a positive integer")
	}
	return true, &seriesID, &seriesOrder, nil
}

func ensureSeriesExists(db *gorm.DB, seriesID *uuid.UUID) error {
	if seriesID == nil {
		return nil
	}
	var series model.Series
	if err := db.Clauses(clause.Locking{Strength: "KEY SHARE"}).First(&series, "id = ?", *seriesID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSeriesNotFound
		}
		return err
	}
	return nil
}

func postgresConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

func loadPostWithTaxonomy(db *gorm.DB, query string, args ...interface{}) (model.Post, error) {
	var post model.Post
	conds := append([]interface{}{query}, args...)
	err := preloadPostTaxonomy(db).First(&post, conds...).Error
	return post, err
}

func syncPostTaxonomy(tx *gorm.DB, post *model.Post, categoryIDs *[]uuid.UUID, tagIDs *[]uuid.UUID) error {
	if categoryIDs != nil {
		categories, err := loadCategoriesByIDs(tx, *categoryIDs)
		if err != nil {
			return err
		}
		if err := tx.Model(post).Association("Categories").Replace(categories); err != nil {
			return err
		}
	}
	if tagIDs != nil {
		tags, err := loadTagsByIDs(tx, *tagIDs)
		if err != nil {
			return err
		}
		if err := tx.Model(post).Association("Tags").Replace(tags); err != nil {
			return err
		}
	}
	return nil
}

func loadCategoriesByIDs(db *gorm.DB, ids []uuid.UUID) ([]model.Category, error) {
	ids = uniqueUUIDs(ids)
	if len(ids) == 0 {
		return []model.Category{}, nil
	}

	var categories []model.Category
	if err := db.Where("id IN ?", ids).Find(&categories).Error; err != nil {
		return nil, err
	}
	if len(categories) != len(ids) {
		return nil, ErrCategoriesNotFound
	}
	return categories, nil
}

func loadTagsByIDs(db *gorm.DB, ids []uuid.UUID) ([]model.Tag, error) {
	ids = uniqueUUIDs(ids)
	if len(ids) == 0 {
		return []model.Tag{}, nil
	}

	var tags []model.Tag
	if err := db.Where("id IN ?", ids).Find(&tags).Error; err != nil {
		return nil, err
	}
	if len(tags) != len(ids) {
		return nil, ErrTagsNotFound
	}
	return tags, nil
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
