package model

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	Base
	Title          string      `json:"title"`
	Slug           string      `json:"slug"`
	Summary        string      `json:"summary"`
	ContentMD      string      `json:"content_md" gorm:"column:content_md"`
	Status         string      `json:"status"`
	Visibility     string      `json:"visibility"`
	AllowComment   bool        `json:"allow_comment"`
	PublishedAt    *time.Time  `json:"published_at"`
	AuthorID       *uuid.UUID  `json:"author_id" gorm:"type:uuid"`
	CoverMediaID   *uuid.UUID  `json:"cover_media_id" gorm:"type:uuid"`
	CoverMedia     *MediaAsset `json:"cover_media" gorm:"foreignKey:CoverMediaID"`
	SeriesID       *uuid.UUID  `json:"series_id" gorm:"type:uuid"`
	SeriesOrder    *int        `json:"series_order"`
	Series         *Series     `json:"series" gorm:"foreignKey:SeriesID"`
	SEOTitle       string      `json:"seo_title" gorm:"column:seo_title"`
	SEODescription string      `json:"seo_description" gorm:"column:seo_description"`
	Categories     []Category  `json:"categories,omitempty" gorm:"many2many:post_categories;constraint:OnDelete:CASCADE;"`
	Tags           []Tag       `json:"tags,omitempty" gorm:"many2many:post_tags;constraint:OnDelete:CASCADE;"`
}

func (Post) TableName() string {
	return "posts"
}
