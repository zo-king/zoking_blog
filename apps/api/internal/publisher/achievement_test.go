package publisher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

func TestWriteAchievementsDataEmpty(t *testing.T) {
	siteDir := t.TempDir()
	result, err := WriteAchievementsData(siteDir, nil)
	if err != nil {
		t.Fatalf("WriteAchievementsData() error = %v", err)
	}
	if result.RelativePath != "data/achievements.json" || result.Count != 0 {
		t.Fatalf("result = %#v", result)
	}

	raw := readAchievementSnapshot(t, siteDir)
	if string(raw) != `{"version":1,"items":[]}` {
		t.Fatalf("snapshot = %s", raw)
	}
	sum := sha256.Sum256(raw)
	if result.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("SHA256 = %q, want %q", result.SHA256, hex.EncodeToString(sum[:]))
	}
}

func TestWriteAchievementsDataFiltersSortsAndProjectsMedia(t *testing.T) {
	siteDir := t.TempDir()
	date := func(value string) time.Time {
		t.Helper()
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	id3 := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	endedAt := date("2026-06-30")
	items := []model.Achievement{
		{Base: model.Base{ID: id3}, Title: "older", OccurredAt: date("2025-01-01"), SortOrder: 0, Status: "published"},
		{Base: model.Base{ID: id2}, Title: "second-id", OccurredAt: date("2026-07-01"), SortOrder: 4, Status: "published"},
		{Base: model.Base{ID: uuid.New()}, Title: "draft", OccurredAt: date("2027-01-01"), Status: "draft"},
		{Base: model.Base{ID: id1}, Kind: "project", Title: "first-id", Organization: "Zoking", Summary: "summary", OccurredAt: date("2026-07-01"), EndedAt: &endedAt, ExternalURL: "https://example.test/result", CredentialID: "credential", ImageMedia: &model.MediaAsset{PublicURL: " /media-files/result.png "}, SortOrder: 4, Status: "published"},
	}

	result, err := WriteAchievementsData(siteDir, items)
	if err != nil {
		t.Fatalf("WriteAchievementsData() error = %v", err)
	}
	if result.Count != 3 {
		t.Fatalf("Count = %d, want 3", result.Count)
	}
	if items[0].ID != id3 {
		t.Fatal("WriteAchievementsData mutated the input order")
	}

	var snapshot achievementSnapshot
	if err := json.Unmarshal(readAchievementSnapshot(t, siteDir), &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snapshot.Version != 1 || len(snapshot.Items) != 3 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.Items[0].ID != id1.String() || snapshot.Items[1].ID != id2.String() || snapshot.Items[2].ID != id3.String() {
		t.Fatalf("item order = %q, %q, %q", snapshot.Items[0].ID, snapshot.Items[1].ID, snapshot.Items[2].ID)
	}
	first := snapshot.Items[0]
	if first.OccurredAt != "2026-07-01" || first.EndedAt == nil || *first.EndedAt != "2026-06-30" {
		t.Fatalf("snapshot dates = %q, %#v", first.OccurredAt, first.EndedAt)
	}
	if first.ImageURL != "/media-files/result.png" {
		t.Fatalf("image_url = %q", first.ImageURL)
	}
	if strings.Contains(string(readAchievementSnapshot(t, siteDir)), "image_media") {
		t.Fatal("snapshot leaked the internal media relation")
	}
}

func TestWriteAchievementsDataProducesUTF8EscapedJSON(t *testing.T) {
	siteDir := t.TempDir()
	item := model.Achievement{
		Base:         model.Base{ID: uuid.New()},
		Kind:         "award",
		Title:        "中文 \"成果\"\n第二行",
		Organization: "研发\\平台",
		Summary:      "<script>& JSON",
		OccurredAt:   time.Date(2026, time.July, 13, 23, 59, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
		Status:       "published",
	}
	if _, err := WriteAchievementsData(siteDir, []model.Achievement{item}); err != nil {
		t.Fatalf("WriteAchievementsData() error = %v", err)
	}
	raw := readAchievementSnapshot(t, siteDir)
	if !strings.Contains(string(raw), "中文") || !strings.Contains(string(raw), `\"成果\"\n第二行`) {
		t.Fatalf("snapshot is not expected UTF-8 JSON: %s", raw)
	}
	var snapshot achievementSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snapshot.Items[0].Title != item.Title || snapshot.Items[0].Organization != item.Organization || snapshot.Items[0].Summary != item.Summary {
		t.Fatalf("escaped fields did not round trip: %#v", snapshot.Items[0])
	}
}

func TestWriteAchievementsDataRejectsInvalidPaths(t *testing.T) {
	if _, err := WriteAchievementsData("", nil); err == nil {
		t.Fatal("empty site directory accepted")
	}

	notDirectory := filepath.Join(t.TempDir(), "site-file")
	if err := os.WriteFile(notDirectory, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteAchievementsData(notDirectory, nil); err == nil {
		t.Fatal("file site path accepted")
	}

	dataIsFile := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataIsFile, "data"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteAchievementsData(dataIsFile, nil); err == nil {
		t.Fatal("file data path accepted")
	}
}

func TestWriteAchievementsDataAtomicallyReplacesSnapshot(t *testing.T) {
	siteDir := t.TempDir()
	first := model.Achievement{Base: model.Base{ID: uuid.New()}, Title: "first", OccurredAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Status: "published"}
	firstResult, err := WriteAchievementsData(siteDir, []model.Achievement{first})
	if err != nil {
		t.Fatalf("first WriteAchievementsData() error = %v", err)
	}
	second := model.Achievement{Base: model.Base{ID: uuid.New()}, Title: "second", OccurredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Status: "published"}
	secondResult, err := WriteAchievementsData(siteDir, []model.Achievement{second})
	if err != nil {
		t.Fatalf("second WriteAchievementsData() error = %v", err)
	}
	if firstResult.SHA256 == secondResult.SHA256 {
		t.Fatal("replacement did not change the snapshot hash")
	}

	raw := readAchievementSnapshot(t, siteDir)
	var snapshot achievementSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("replacement left partial JSON: %v", err)
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].Title != "second" {
		t.Fatalf("replacement snapshot = %#v", snapshot)
	}
	temporaryFiles, err := filepath.Glob(filepath.Join(siteDir, "data", ".achievements-*.json.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("temporary files left behind: %v", temporaryFiles)
	}
}

func readAchievementSnapshot(t *testing.T, siteDir string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(siteDir, filepath.FromSlash(achievementsDataRelativePath)))
	if err != nil {
		t.Fatalf("read achievement snapshot: %v", err)
	}
	return raw
}
