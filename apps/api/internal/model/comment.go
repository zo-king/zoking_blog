package model

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	Base
	PostID          uuid.UUID  `json:"post_id" gorm:"type:uuid"`
	ParentID        *uuid.UUID `json:"parent_id" gorm:"type:uuid"`
	AuthorName      string     `json:"author_name"`
	AuthorEmailHash string     `json:"author_email_hash"`
	AuthorWebsite   string     `json:"author_website"`
	Content         string     `json:"content"`
	Status          string     `json:"status"`
	IPHash          string     `json:"-"`
	UserAgent       string     `json:"-"`
	ReviewedBy      *uuid.UUID `json:"reviewed_by" gorm:"type:uuid"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	SpamReason      string     `json:"spam_reason"`
	Post            *Post      `json:"post,omitempty" gorm:"foreignKey:PostID"`
}

func (Comment) TableName() string {
	return "comments"
}
