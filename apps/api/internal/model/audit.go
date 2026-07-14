package model

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ActorID       *uuid.UUID `json:"actor_id" gorm:"type:uuid"`
	ActorEmail    string     `json:"actor_email"`
	Action        string     `json:"action"`
	ResourceType  string     `json:"resource_type"`
	ResourceID    *uuid.UUID `json:"resource_id" gorm:"type:uuid"`
	BeforeJSON    []byte     `json:"before_json" gorm:"type:jsonb"`
	AfterJSON     []byte     `json:"after_json" gorm:"type:jsonb"`
	Route         string     `json:"route"`
	Method        string     `json:"method"`
	Result        string     `json:"result"`
	StatusCode    int        `json:"status_code"`
	ErrorCode     string     `json:"error_code"`
	RequestID     string     `json:"request_id"`
	IPHash        string     `json:"ip_hash"`
	IPHashVersion int16      `json:"ip_hash_version"`
	UserAgent     string     `json:"user_agent"`
	DetailsJSON   []byte     `json:"details_json" gorm:"type:jsonb"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
