package model

import "github.com/google/uuid"

type Series struct {
	Base
	Name         string      `json:"name"`
	Slug         string      `json:"slug"`
	Description  string      `json:"description"`
	CoverMediaID *uuid.UUID  `json:"cover_media_id" gorm:"type:uuid"`
	CoverMedia   *MediaAsset `json:"cover_media" gorm:"foreignKey:CoverMediaID"`
	SortOrder    int         `json:"sort_order"`
	Enabled      bool        `json:"enabled"`
	PostCount    int64       `json:"post_count" gorm:"-"`
	Posts        []Post      `json:"posts,omitempty" gorm:"foreignKey:SeriesID"`
}

func (Series) TableName() string {
	return "series"
}
