package model

import (
	"time"

	"github.com/google/uuid"
)

type PublishJob struct {
	Base
	PostID        *uuid.UUID `json:"post_id" gorm:"type:uuid"`
	PageID        *uuid.UUID `json:"page_id" gorm:"type:uuid"`
	JobType       string     `json:"job_type"`
	Status        string     `json:"status"`
	TriggerSource string     `json:"trigger_source"`
	RequestedBy   *uuid.UUID `json:"requested_by" gorm:"type:uuid"`
	RunAt         time.Time  `json:"run_at"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	SnapshotKey   string     `json:"snapshot_key"`
	SettingsHash  string     `json:"settings_hash"`
	ReleaseKey    string     `json:"release_key"`
	ContentPath   string     `json:"content_path"`
	OutputPath    string     `json:"output_path"`
	ManifestJSON  []byte     `json:"manifest_json" gorm:"type:jsonb"`
	LogJSON       []byte     `json:"log_json" gorm:"type:jsonb"`
	ErrorCode     string     `json:"error_code"`
	ErrorMessage  string     `json:"error_message"`
	RetryCount    int        `json:"retry_count"`
	CanceledAt    *time.Time `json:"canceled_at"`
	Post          *Post      `json:"post,omitempty" gorm:"foreignKey:PostID"`
	Page          *Page      `json:"page,omitempty" gorm:"foreignKey:PageID"`
}

func (PublishJob) TableName() string {
	return "publish_jobs"
}

type PublishRelease struct {
	Base
	JobID        uuid.UUID   `json:"job_id" gorm:"type:uuid"`
	ReleaseKey   string      `json:"release_key"`
	Status       string      `json:"status"`
	PostID       *uuid.UUID  `json:"post_id" gorm:"type:uuid"`
	PageID       *uuid.UUID  `json:"page_id" gorm:"type:uuid"`
	OutputPath   string      `json:"output_path"`
	ManifestJSON []byte      `json:"manifest_json" gorm:"type:jsonb"`
	IsActive     bool        `json:"is_active"`
	PromotedAt   *time.Time  `json:"promoted_at"`
	Job          *PublishJob `json:"job,omitempty" gorm:"foreignKey:JobID"`
	Post         *Post       `json:"post,omitempty" gorm:"foreignKey:PostID"`
	Page         *Page       `json:"page,omitempty" gorm:"foreignKey:PageID"`
}

func (PublishRelease) TableName() string {
	return "publish_releases"
}

type PublishPreview struct {
	Base
	PreviewKey   string     `json:"preview_key"`
	Scope        string     `json:"scope"`
	Status       string     `json:"status"`
	PostID       *uuid.UUID `json:"post_id" gorm:"type:uuid"`
	PageID       *uuid.UUID `json:"page_id" gorm:"type:uuid"`
	RequestedBy  *uuid.UUID `json:"requested_by" gorm:"type:uuid"`
	OutputPath   string     `json:"output_path"`
	EntryPath    string     `json:"entry_path"`
	URL          string     `json:"url"`
	TargetURL    string     `json:"target_url"`
	SettingsHash string     `json:"settings_hash"`
	ContentHash  string     `json:"content_hash"`
	ManifestJSON []byte     `json:"manifest_json" gorm:"type:jsonb"`
	LogJSON      []byte     `json:"log_json" gorm:"type:jsonb"`
	ErrorCode    string     `json:"error_code"`
	ErrorMessage string     `json:"error_message"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
	Post         *Post      `json:"post,omitempty" gorm:"foreignKey:PostID"`
	Page         *Page      `json:"page,omitempty" gorm:"foreignKey:PageID"`
}

func (PublishPreview) TableName() string {
	return "publish_previews"
}
