package httpapi

import (
	"testing"
	"time"

	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

func TestAchievementFromCreateRequest(t *testing.T) {
	sortOrder := 3
	item, err := achievementFromCreateRequest(createAchievementRequest{
		Kind:         " Certificate ",
		Title:        " PostgreSQL Associate ",
		Organization: " PostgreSQL Global Development Group ",
		Summary:      " Passed the assessment ",
		OccurredAt:   "2026-07-13",
		EndedAt:      rawJSON(`"2027-07-13"`),
		ExternalURL:  " https://example.test/credentials/123 ",
		CredentialID: " CERT-123 ",
		SortOrder:    &sortOrder,
	})
	if err != nil {
		t.Fatalf("achievementFromCreateRequest() error = %v", err)
	}
	if item.Kind != "certificate" || item.Title != "PostgreSQL Associate" || item.Status != "draft" {
		t.Fatalf("normalized item = %#v", item)
	}
	if item.EndedAt == nil || item.EndedAt.Format(achievementDateLayout) != "2027-07-13" {
		t.Fatalf("ended_at = %v", item.EndedAt)
	}
	if item.SortOrder != 3 || item.CredentialID != "CERT-123" {
		t.Fatalf("sort_order/credential_id = %d/%q", item.SortOrder, item.CredentialID)
	}
}

func TestAchievementCreateValidation(t *testing.T) {
	negative := -1
	tests := []struct {
		name string
		req  createAchievementRequest
	}{
		{name: "invalid kind", req: createAchievementRequest{Kind: "other", Title: "Title", OccurredAt: "2026-01-01"}},
		{name: "blank title", req: createAchievementRequest{Kind: "award", Title: " ", OccurredAt: "2026-01-01"}},
		{name: "date before minimum", req: createAchievementRequest{Kind: "award", Title: "Title", OccurredAt: "2023-12-31"}},
		{name: "invalid date", req: createAchievementRequest{Kind: "award", Title: "Title", OccurredAt: "2026-02-30"}},
		{name: "ended before occurred", req: createAchievementRequest{Kind: "award", Title: "Title", OccurredAt: "2026-02-01", EndedAt: rawJSON(`"2026-01-31"`)}},
		{name: "invalid URL", req: createAchievementRequest{Kind: "award", Title: "Title", OccurredAt: "2026-01-01", ExternalURL: "javascript:alert(1)"}},
		{name: "negative sort", req: createAchievementRequest{Kind: "award", Title: "Title", OccurredAt: "2026-01-01", SortOrder: &negative}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := achievementFromCreateRequest(test.req); err == nil {
				t.Fatal("achievementFromCreateRequest() error = nil")
			}
		})
	}
}

func TestAchievementUpdatesValidateEffectiveDateRange(t *testing.T) {
	occurredAt := mustAchievementTestDate(t, "2026-01-01")
	endedAt := mustAchievementTestDate(t, "2026-06-01")
	current := model.Achievement{OccurredAt: occurredAt, EndedAt: &endedAt}

	later := "2026-07-01"
	if _, _, _, err := achievementUpdatesFromRequest(nil, current, updateAchievementRequest{OccurredAt: &later}); err == nil {
		t.Fatal("moving occurred_at after retained ended_at should fail")
	}

	updates, provided, imageID, err := achievementUpdatesFromRequest(nil, current, updateAchievementRequest{EndedAt: rawJSON(`null`)})
	if err != nil {
		t.Fatalf("clearing ended_at error = %v", err)
	}
	if provided || imageID != nil {
		t.Fatalf("image change = %v/%v, want omitted", provided, imageID)
	}
	value, exists := updates["ended_at"]
	if !exists || value != (*time.Time)(nil) {
		t.Fatalf("ended_at update = %#v, exists=%v", value, exists)
	}
}

func TestParseAchievementYear(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
		ok    bool
	}{
		{value: "2024", want: 2024, ok: true},
		{value: "9999", want: 9999, ok: true},
		{value: "2023"},
		{value: "02024"},
		{value: "abcd"},
	} {
		got, ok := parseAchievementYear(test.value)
		if got != test.want || ok != test.ok {
			t.Fatalf("parseAchievementYear(%q) = %d/%v, want %d/%v", test.value, got, ok, test.want, test.ok)
		}
	}
}

func mustAchievementTestDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(achievementDateLayout, value)
	if err != nil {
		t.Fatalf("parse test date %q: %v", value, err)
	}
	return parsed
}
