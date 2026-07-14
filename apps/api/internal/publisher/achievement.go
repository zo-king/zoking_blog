package publisher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

const achievementsDataRelativePath = "data/achievements.json"

// AchievementSnapshotResult describes the generated Hugo data snapshot.
type AchievementSnapshotResult struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
	Count        int    `json:"count"`
}

type achievementSnapshot struct {
	Version int                       `json:"version"`
	Items   []achievementSnapshotItem `json:"items"`
}

type achievementSnapshotItem struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	Organization string  `json:"organization"`
	Summary      string  `json:"summary"`
	OccurredAt   string  `json:"occurred_at"`
	EndedAt      *string `json:"ended_at"`
	ExternalURL  string  `json:"external_url"`
	CredentialID string  `json:"credential_id"`
	ImageURL     string  `json:"image_url"`
	SortOrder    int     `json:"sort_order"`
}

// WriteAchievementsData writes the published achievement data consumed by Hugo.
func WriteAchievementsData(siteDir string, items []model.Achievement) (AchievementSnapshotResult, error) {
	targetPath, err := achievementSnapshotPath(siteDir)
	if err != nil {
		return AchievementSnapshotResult{}, err
	}

	published := make([]model.Achievement, 0, len(items))
	for _, item := range items {
		if item.Status == "published" {
			published = append(published, item)
		}
	}
	sort.SliceStable(published, func(i, j int) bool {
		leftDate := published[i].OccurredAt.Format("2006-01-02")
		rightDate := published[j].OccurredAt.Format("2006-01-02")
		if leftDate != rightDate {
			return leftDate > rightDate
		}
		if published[i].SortOrder != published[j].SortOrder {
			return published[i].SortOrder < published[j].SortOrder
		}
		return published[i].ID.String() < published[j].ID.String()
	})

	snapshotItems := make([]achievementSnapshotItem, 0, len(published))
	for _, item := range published {
		var endedAt *string
		if item.EndedAt != nil {
			formatted := item.EndedAt.Format("2006-01-02")
			endedAt = &formatted
		}
		imageURL := ""
		if item.ImageMedia != nil {
			imageURL = strings.TrimSpace(item.ImageMedia.PublicURL)
		}
		snapshotItems = append(snapshotItems, achievementSnapshotItem{
			ID:           item.ID.String(),
			Kind:         item.Kind,
			Title:        item.Title,
			Organization: item.Organization,
			Summary:      item.Summary,
			OccurredAt:   item.OccurredAt.Format("2006-01-02"),
			EndedAt:      endedAt,
			ExternalURL:  item.ExternalURL,
			CredentialID: item.CredentialID,
			ImageURL:     imageURL,
			SortOrder:    item.SortOrder,
		})
	}

	payload, err := json.Marshal(achievementSnapshot{Version: 1, Items: snapshotItems})
	if err != nil {
		return AchievementSnapshotResult{}, fmt.Errorf("marshal achievements snapshot: %w", err)
	}
	if err := writeAchievementSnapshotAtomically(targetPath, payload); err != nil {
		return AchievementSnapshotResult{}, err
	}

	sum := sha256.Sum256(payload)
	return AchievementSnapshotResult{
		RelativePath: achievementsDataRelativePath,
		SHA256:       hex.EncodeToString(sum[:]),
		Count:        len(snapshotItems),
	}, nil
}

func achievementSnapshotPath(siteDir string) (string, error) {
	if strings.TrimSpace(siteDir) == "" {
		return "", fmt.Errorf("achievement snapshot site directory is required")
	}
	absSiteDir, err := filepath.Abs(siteDir)
	if err != nil {
		return "", fmt.Errorf("resolve achievement snapshot site directory: %w", err)
	}
	absSiteDir = filepath.Clean(absSiteDir)
	if filepath.Dir(absSiteDir) == absSiteDir {
		return "", fmt.Errorf("achievement snapshot site directory cannot be a filesystem root")
	}
	info, err := os.Stat(absSiteDir)
	if err != nil {
		return "", fmt.Errorf("inspect achievement snapshot site directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("achievement snapshot site path is not a directory")
	}

	resolvedSiteDir, err := filepath.EvalSymlinks(absSiteDir)
	if err != nil {
		return "", fmt.Errorf("resolve achievement snapshot site directory links: %w", err)
	}
	resolvedSiteDir = filepath.Clean(resolvedSiteDir)
	if filepath.Dir(resolvedSiteDir) == resolvedSiteDir {
		return "", fmt.Errorf("achievement snapshot site directory cannot resolve to a filesystem root")
	}
	dataDir := filepath.Join(resolvedSiteDir, "data")
	if dataInfo, statErr := os.Lstat(dataDir); statErr == nil {
		if dataInfo.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("achievement snapshot data directory cannot be a symbolic link")
		}
		if !dataInfo.IsDir() {
			return "", fmt.Errorf("achievement snapshot data path is not a directory")
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect achievement snapshot data directory: %w", statErr)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create achievement snapshot data directory: %w", err)
	}
	dataInfo, err := os.Lstat(dataDir)
	if err != nil {
		return "", fmt.Errorf("inspect created achievement snapshot data directory: %w", err)
	}
	if dataInfo.Mode()&os.ModeSymlink != 0 || !dataInfo.IsDir() {
		return "", fmt.Errorf("achievement snapshot data directory is unsafe")
	}

	targetPath := filepath.Join(dataDir, "achievements.json")
	relative, err := filepath.Rel(resolvedSiteDir, targetPath)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("achievement snapshot path escapes site directory")
	}
	return targetPath, nil
}

func writeAchievementSnapshotAtomically(targetPath string, payload []byte) (err error) {
	temporary, err := os.CreateTemp(filepath.Dir(targetPath), ".achievements-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temporary achievement snapshot: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if temporaryPath != "" {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("set temporary achievement snapshot permissions: %w", err)
	}
	if _, err := temporary.Write(payload); err != nil {
		return fmt.Errorf("write temporary achievement snapshot: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary achievement snapshot: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary achievement snapshot: %w", err)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("replace achievement snapshot: %w", err)
	}
	temporaryPath = ""
	return nil
}
