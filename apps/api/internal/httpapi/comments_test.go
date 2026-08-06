package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

func TestHashStringUsesConfiguredHMACSecret(t *testing.T) {
	value, secret := "reader@example.com", "0123456789abcdef0123456789abcdef"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	want := hex.EncodeToString(mac.Sum(nil))
	if got := hashString(value, secret); got != want {
		t.Fatalf("hashString() = %q, want keyed digest %q", got, want)
	}
	if got := hashString(value, "different-secret"); got == want {
		t.Fatal("hashString() did not change when the secret changed")
	}
}

func TestPublicCommentDTOExposesOnlyPublicFields(t *testing.T) {
	parentID := uuid.New()
	comment := model.Comment{
		Base: model.Base{
			ID:        uuid.New(),
			CreatedAt: time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC),
		},
		PostID:          uuid.New(),
		ParentID:        &parentID,
		AuthorName:      "Reader",
		AuthorEmailHash: "private-email-hash",
		AuthorWebsite:   "https://reader.example",
		Content:         "Hello",
		Status:          "approved",
		IPHash:          "private-ip-hash",
		UserAgent:       "private-user-agent",
		ReviewedBy:      ptrUUID(uuid.New()),
		ReviewedAt:      ptrTime(time.Now()),
		SpamReason:      "private-spam-reason",
	}

	payload, err := json.Marshal(newPublicCommentDTO(comment))
	if err != nil {
		t.Fatalf("marshal public comment: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("decode public comment: %v", err)
	}

	wantFields := []string{"id", "parent_id", "author_name", "author_website", "content", "created_at"}
	if len(fields) != len(wantFields) {
		t.Fatalf("public fields = %v, want exactly %v", fields, wantFields)
	}
	for _, field := range wantFields {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing public field %q in %s", field, payload)
		}
	}
	for _, field := range []string{"post_id", "status", "author_email_hash", "ip_hash", "user_agent", "reviewed_by", "reviewed_at", "spam_reason", "deleted_at", "updated_at"} {
		if _, ok := fields[field]; ok {
			t.Fatalf("sensitive/internal field %q leaked in %s", field, payload)
		}
	}
}

func ptrUUID(value uuid.UUID) *uuid.UUID {
	return &value
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
