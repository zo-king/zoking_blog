package model

import (
	"time"

	"github.com/google/uuid"
)

type Page struct {
	Base
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Summary        string     `json:"summary"`
	ContentMD      string     `json:"content_md" gorm:"column:content_md"`
	Status         string     `json:"status"`
	Visibility     string     `json:"visibility"`
	ShowInMenu     bool       `json:"show_in_menu"`
	MenuWeight     int        `json:"menu_weight"`
	MenuIcon       string     `json:"menu_icon"`
	AllowComment   bool       `json:"allow_comment"`
	PublishedAt    *time.Time `json:"published_at"`
	AuthorID       *uuid.UUID `json:"author_id" gorm:"type:uuid"`
	SEOTitle       string     `json:"seo_title" gorm:"column:seo_title"`
	SEODescription string     `json:"seo_description" gorm:"column:seo_description"`
}

func (Page) TableName() string {
	return "pages"
}
