package model

import (
	"time"

	"github.com/google/uuid"
)

type Achievement struct {
	Base
	Kind         string      `json:"kind"`
	Title        string      `json:"title"`
	Organization string      `json:"organization"`
	Summary      string      `json:"summary"`
	OccurredAt   time.Time   `json:"occurred_at" gorm:"type:date"`
	EndedAt      *time.Time  `json:"ended_at" gorm:"type:date"`
	ExternalURL  string      `json:"external_url"`
	CredentialID string      `json:"credential_id"`
	ImageMediaID *uuid.UUID  `json:"image_media_id" gorm:"type:uuid"`
	ImageMedia   *MediaAsset `json:"image_media" gorm:"foreignKey:ImageMediaID"`
	SortOrder    int         `json:"sort_order"`
	Status       string      `json:"status"`
	PublishedAt  *time.Time  `json:"published_at"`
}

func (Achievement) TableName() string {
	return "achievements"
}
