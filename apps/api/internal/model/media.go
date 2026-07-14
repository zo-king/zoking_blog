package model

import "github.com/google/uuid"

type MediaAsset struct {
	Base
	Filename      string     `json:"filename"`
	OriginalName  string     `json:"original_name"`
	MimeType      string     `json:"mime_type"`
	SizeBytes     int64      `json:"size_bytes"`
	Width         int        `json:"width"`
	Height        int        `json:"height"`
	StorageDriver string     `json:"storage_driver"`
	StorageBucket string     `json:"storage_bucket"`
	StorageKey    string     `json:"storage_key"`
	PublicURL     string     `json:"public_url"`
	Checksum      string     `json:"checksum"`
	UploadedBy    *uuid.UUID `json:"uploaded_by" gorm:"type:uuid"`
	Status        string     `json:"status"`
	UsageCount    int64      `json:"usage_count" gorm:"-"`
}

func (MediaAsset) TableName() string {
	return "media_assets"
}

type MediaUsage struct {
	Base
	MediaID      uuid.UUID `json:"media_id" gorm:"type:uuid"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id" gorm:"type:uuid"`
	UsageType    string    `json:"usage_type"`
}

func (MediaUsage) TableName() string {
	return "media_usages"
}
