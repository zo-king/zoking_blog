package model

import (
	"time"

	"github.com/google/uuid"
)

type SiteSetting struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Key         string    `json:"key"`
	ValueJSON   []byte    `json:"value_json" gorm:"type:jsonb"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SiteSetting) TableName() string {
	return "site_settings"
}
