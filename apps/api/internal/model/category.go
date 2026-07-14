package model

import "github.com/google/uuid"

type Category struct {
	Base
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	ParentID    *uuid.UUID `json:"parent_id" gorm:"type:uuid"`
	SortOrder   int        `json:"sort_order"`
	Enabled     bool       `json:"enabled"`
}

func (Category) TableName() string {
	return "categories"
}
