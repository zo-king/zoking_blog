package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

type PreviewResult struct {
	PreviewKey string          `json:"preview_key"`
	Scope      string          `json:"scope"`
	Slug       string          `json:"slug"`
	OutputPath string          `json:"output_path"`
	URL        string          `json:"url"`
	TargetURL  string          `json:"target_url"`
	TargetPath string          `json:"target_path"`
	CreatedAt  time.Time       `json:"created_at"`
	Manifest   PublishManifest `json:"manifest"`
}

type PreviewOptions struct {
	PreviewKey string
	BaseURL    string
	URL        string
	TargetURL  string
	Settings   *SiteSettingsSnapshot
}

func NewPreviewKey(now time.Time) string {
	return "prev-" + now.Format("20060102-150405") + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func PreviewTargetPath(scope string, slug string) string {
	switch scope {
	case "post":
		return "/p/" + slug + "/"
	case "page":
		return "/" + slug + "/"
	default:
		return "/"
	}
}

func BuildPostPreview(ctx context.Context, db *gorm.DB, cfg config.Config, post model.Post, options PreviewOptions) (PreviewResult, error) {
	writeInput := publishBuildInput{
		Scope:     "post",
		Slug:      post.Slug,
		Post:      &post,
		PostID:    &post.ID,
		ContentMD: post.ContentMD,
	}
	return buildPreview(ctx, db, cfg, writeInput, options)
}

func BuildPagePreview(ctx context.Context, db *gorm.DB, cfg config.Config, page model.Page, options PreviewOptions) (PreviewResult, error) {
	writeInput := publishBuildInput{
		Scope:     "page",
		Slug:      page.Slug,
		Page:      &page,
		PageID:    &page.ID,
		ContentMD: page.ContentMD,
	}
	return buildPreview(ctx, db, cfg, writeInput, options)
}

func BuildSitePreview(ctx context.Context, db *gorm.DB, cfg config.Config, options PreviewOptions) (PreviewResult, error) {
	return buildPreview(ctx, db, cfg, publishBuildInput{Scope: "site"}, options)
}

func buildPreview(ctx context.Context, db *gorm.DB, cfg config.Config, input publishBuildInput, options PreviewOptions) (PreviewResult, error) {
	createdAt := time.Now()
	previewKey := strings.TrimSpace(options.PreviewKey)
	if previewKey == "" {
		previewKey = NewPreviewKey(createdAt)
	}
	if err := ValidateSlug(previewKey); err != nil {
		return PreviewResult{}, fmt.Errorf("invalid preview key: %w", err)
	}
	if err := validatePreviewFilesystemLayout(cfg, previewKey); err != nil {
		return PreviewResult{}, err
	}

	previewBaseURL := ensureTrailingSlash(strings.TrimSpace(options.BaseURL))
	if previewBaseURL == "" {
		return PreviewResult{}, fmt.Errorf("preview base URL is required")
	}

	settings, settingsHash, err := previewSettings(ctx, db, options.Settings, previewBaseURL)
	if err != nil {
		return PreviewResult{}, err
	}

	workDir := filepath.Join(filepath.Dir(cfg.HugoSiteDir), "."+previewKey)
	outputPath := filepath.Join(previewRoot(cfg), previewKey)
	if err := os.RemoveAll(workDir); err != nil {
		return PreviewResult{}, err
	}
	if err := os.RemoveAll(outputPath); err != nil {
		return PreviewResult{}, err
	}
	if err := copySiteForPreview(cfg.HugoSiteDir, workDir); err != nil {
		return PreviewResult{}, err
	}
	defer os.RemoveAll(workDir)

	if err := ApplySiteSettings(workDir, settings); err != nil {
		return PreviewResult{}, err
	}
	achievements, err := loadPublishedAchievements(ctx, db)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("load published achievements: %w", err)
	}
	achievementSnapshot, err := WriteAchievementsData(workDir, achievements)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("write achievements preview snapshot: %w", err)
	}

	var writeResult Result
	switch input.Scope {
	case "post":
		writeResult, err = WritePost(workDir, *input.Post)
	case "page":
		writeResult, err = WritePage(workDir, *input.Page)
	case "site":
		writeResult = Result{}
	default:
		err = fmt.Errorf("unsupported preview scope")
	}
	if err != nil {
		return PreviewResult{}, err
	}
	input.ContentPath = writeResult.ContentPath
	if input.Scope == "post" {
		input.SnapshotKey = filepath.ToSlash(filepath.Join("content", "post", input.Slug, "index.md"))
	}
	if input.Scope == "page" {
		input.SnapshotKey = filepath.ToSlash(filepath.Join("content", "page", input.Slug, "index.md"))
	}

	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return PreviewResult{}, err
	}
	hugoBin := cfg.HugoBin
	if _, err := os.Stat(hugoBin); err != nil {
		hugoBin = "hugo"
	}
	buildCtx := ctx
	var cancel context.CancelFunc
	if cfg.PublishJobTimeout > 0 {
		buildCtx, cancel = context.WithTimeout(ctx, cfg.PublishJobTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(buildCtx, hugoBin, "--source", workDir, "--destination", outputPath, "--baseURL", previewBaseURL, "--minify")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PreviewResult{}, fmt.Errorf("preview Hugo build failed: %w: %s", err, compactLogOutput(output))
	}

	contentHash := settingsHash
	if input.ContentPath != "" {
		contentBytes, err := os.ReadFile(input.ContentPath)
		if err != nil {
			return PreviewResult{}, err
		}
		hash := sha256.Sum256(contentBytes)
		contentHash = hex.EncodeToString(hash[:])
	}
	postID := ""
	if input.PostID != nil {
		postID = input.PostID.String()
	}
	pageID := ""
	if input.PageID != nil {
		pageID = input.PageID.String()
	}
	manifest := PublishManifest{
		Scope:            input.Scope,
		PostID:           postID,
		PageID:           pageID,
		Slug:             input.Slug,
		ContentPath:      input.ContentPath,
		ContentHash:      contentHash,
		SettingsHash:     settingsHash,
		AchievementsHash: achievementSnapshot.SHA256,
		DataPaths:        []string{achievementSnapshot.RelativePath},
		ReleaseKey:       previewKey,
		OutputPath:       outputPath,
		CreatedAt:        createdAt,
		HugoCommand:      cmd.String(),
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return PreviewResult{}, err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "manifest.json"), manifestJSON, 0o644); err != nil {
		return PreviewResult{}, err
	}
	if err := verifyReleaseTarget(outputPath, nil, nil); err != nil {
		return PreviewResult{}, err
	}

	targetPath := PreviewTargetPath(input.Scope, input.Slug)
	targetFile := filepath.Join(outputPath, filepath.FromSlash(strings.Trim(targetPath, "/")), "index.html")
	if targetPath == "/" {
		targetFile = filepath.Join(outputPath, "index.html")
	}
	if _, err := os.Stat(targetFile); err != nil {
		return PreviewResult{}, fmt.Errorf("preview target is missing: %w", err)
	}
	previewURL := options.URL
	if previewURL == "" {
		previewURL = previewBaseURL
	}
	targetURL := options.TargetURL
	if targetURL == "" {
		targetURL = previewBaseURL + strings.TrimPrefix(targetPath, "/")
	}
	return PreviewResult{
		PreviewKey: previewKey,
		Scope:      input.Scope,
		Slug:       input.Slug,
		OutputPath: outputPath,
		URL:        previewURL,
		TargetURL:  targetURL,
		TargetPath: targetPath,
		CreatedAt:  createdAt,
		Manifest:   manifest,
	}, nil
}

func previewSettings(ctx context.Context, db *gorm.DB, override *SiteSettingsSnapshot, previewBaseURL string) (SiteSettingsSnapshot, string, error) {
	var settings SiteSettingsSnapshot
	var err error
	if override != nil {
		settings = *override
	} else {
		settings, _, err = LoadSiteSettings(ctx, db)
		if err != nil {
			return settings, "", err
		}
	}
	settings.Site.BaseURL = previewBaseURL
	hash, err := hashSiteSettings(settings)
	if err != nil {
		return settings, "", err
	}
	return settings, hash, nil
}

func copySiteForPreview(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		name := entry.Name()
		if entry.IsDir() && (name == "public" || name == "resources") {
			return filepath.SkipDir
		}
		if name == ".hugo_build.lock" {
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("site source contains unsupported symlink: %s", path)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func previewRoot(cfg config.Config) string {
	if cfg.PublishPreviewRoot != "" {
		return filepath.Clean(cfg.PublishPreviewRoot)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentOutputDir(cfg)), "previews"))
}

func validatePreviewFilesystemLayout(cfg config.Config, previewKey string) error {
	root := previewRoot(cfg)
	if strings.TrimSpace(root) == "" || filepath.Clean(filepath.VolumeName(root)+string(filepath.Separator)) == root {
		return fmt.Errorf("preview root must be a dedicated non-root directory")
	}
	protected := []string{cfg.PublishReleaseRoot, cfg.PublishCurrentDir, cfg.MediaLocalDir, cfg.HugoSiteDir, cfg.HugoPublicDir}
	for _, candidate := range protected {
		if pathsOverlap(root, candidate) {
			return fmt.Errorf("preview root overlaps protected path %s", filepath.Clean(candidate))
		}
	}
	outputPath := filepath.Join(root, previewKey)
	if !pathWithinOrEqual(root, outputPath) || filepath.Clean(outputPath) == root {
		return fmt.Errorf("preview output path escapes preview root")
	}
	return nil
}

func pathsOverlap(left string, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	return pathWithinOrEqual(left, right) || pathWithinOrEqual(right, left)
}

func pathWithinOrEqual(root string, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func ensureTrailingSlash(value string) string {
	if value == "" || strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}
