package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

type Result struct {
	ContentPath string `json:"content_path"`
}

var safeSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)
var reservedPageSlugs = map[string]struct{}{
	"admin":       {},
	"api":         {},
	"archives":    {},
	"assets":      {},
	"categories":  {},
	"css":         {},
	"en":          {},
	"img":         {},
	"index":       {},
	"ja":          {},
	"js":          {},
	"media-files": {},
	"p":           {},
	"page":        {},
	"post":        {},
	"search":      {},
	"sitemap":     {},
	"static":      {},
	"tags":        {},
	"uploads":     {},
	"zh":          {},
	"zh-hant-tw":  {},
}

func WritePost(siteDir string, post model.Post) (Result, error) {
	if err := ValidateSlug(post.Slug); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(post.ContentMD) == "" {
		return Result{}, fmt.Errorf("post content is required")
	}
	if err := validatePostSeries(post); err != nil {
		return Result{}, err
	}

	absSiteDir, err := filepath.Abs(siteDir)
	if err != nil {
		return Result{}, err
	}
	if err := writePostTaxonomySnapshots(absSiteDir, post); err != nil {
		return Result{}, err
	}

	contentDir := filepath.Join(absSiteDir, "content", "post", post.Slug)
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return Result{}, err
	}

	contentPath := filepath.Join(contentDir, "index.md")
	body := buildPostMarkdown(post)
	if err := os.WriteFile(contentPath, []byte(body), 0o644); err != nil {
		return Result{}, err
	}

	return Result{ContentPath: contentPath}, nil
}

func WritePage(siteDir string, page model.Page) (Result, error) {
	if err := ValidatePageSlug(page.Slug); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(page.ContentMD) == "" {
		return Result{}, fmt.Errorf("page content is required")
	}

	absSiteDir, err := filepath.Abs(siteDir)
	if err != nil {
		return Result{}, err
	}

	contentDir := filepath.Join(absSiteDir, "content", "page", page.Slug)
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return Result{}, err
	}

	contentPath := filepath.Join(contentDir, "index.md")
	body := buildPageMarkdown(page)
	if err := os.WriteFile(contentPath, []byte(body), 0o644); err != nil {
		return Result{}, err
	}

	return Result{ContentPath: contentPath}, nil
}

func RemovePost(siteDir string, slug string) (bool, error) {
	return removeContentSnapshot(siteDir, "post", slug)
}

func RemovePage(siteDir string, slug string) (bool, error) {
	return removeContentSnapshot(siteDir, "page", slug)
}

func removeContentSnapshot(siteDir string, section string, slug string) (bool, error) {
	if err := ValidateSlug(slug); err != nil {
		return false, err
	}
	absSiteDir, err := filepath.Abs(siteDir)
	if err != nil {
		return false, err
	}
	contentRoot := filepath.Join(absSiteDir, "content", section)
	target := filepath.Join(contentRoot, slug)
	relative, err := filepath.Rel(contentRoot, target)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("content snapshot path escapes root")
	}
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(target); err != nil {
		return false, err
	}
	return true, nil
}

func buildPostMarkdown(post model.Post) string {
	publishedAt := post.CreatedAt
	if post.PublishedAt != nil {
		publishedAt = *post.PublishedAt
	}

	description := post.SEODescription
	if description == "" {
		description = post.Summary
	}

	body := strings.TrimSpace(post.ContentMD)

	categorySlugs := make([]string, 0, len(post.Categories))
	for _, category := range post.Categories {
		if trimmed := strings.TrimSpace(category.Slug); trimmed != "" {
			categorySlugs = append(categorySlugs, trimmed)
		}
	}

	tagSlugs := make([]string, 0, len(post.Tags))
	for _, tag := range post.Tags {
		if trimmed := strings.TrimSpace(tag.Slug); trimmed != "" {
			tagSlugs = append(tagSlugs, trimmed)
		}
	}

	lines := []string{
		"---",
		"title: " + yamlString(post.Title),
		"description: " + yamlString(description),
		"slug: " + yamlString(post.Slug),
		"date: " + publishedAt.Format(time.RFC3339),
		"lastmod: " + post.UpdatedAt.Format(time.RFC3339),
		"draft: false",
		"comments: " + boolString(post.AllowComment),
		"toc: true",
		"readingTime: true",
	}
	if imageURL := postCoverImageURL(post); imageURL != "" {
		lines = append(lines, "image: "+yamlString(imageURL))
	}
	if post.SeriesID != nil && post.SeriesOrder != nil && post.Series != nil {
		lines = append(lines,
			"series:",
			"  slug: "+yamlString(post.Series.Slug),
			"  name: "+yamlString(post.Series.Name),
			"  order: "+strconv.Itoa(*post.SeriesOrder),
		)
	}
	lines = append(lines, yamlList("categories", categorySlugs)...)
	lines = append(lines, yamlList("tags", tagSlugs)...)
	lines = append(lines, "---", "", body, "")

	return strings.Join(lines, "\n")
}

func validatePostSeries(post model.Post) error {
	hasID := post.SeriesID != nil
	hasOrder := post.SeriesOrder != nil
	hasSeries := post.Series != nil
	if !hasID && !hasOrder && !hasSeries {
		return nil
	}
	if !hasID || !hasOrder || !hasSeries {
		return fmt.Errorf("post series metadata is incomplete")
	}
	if *post.SeriesOrder <= 0 {
		return fmt.Errorf("post series order must be positive")
	}
	if post.Series.ID != *post.SeriesID {
		return fmt.Errorf("post series relation does not match series_id")
	}
	if strings.TrimSpace(post.Series.Name) == "" {
		return fmt.Errorf("post series name is required")
	}
	if err := ValidateSlug(post.Series.Slug); err != nil {
		return fmt.Errorf("invalid post series slug: %w", err)
	}
	return nil
}

func writePostTaxonomySnapshots(siteDir string, post model.Post) error {
	for _, category := range post.Categories {
		if err := writeTaxonomyTermSnapshot(siteDir, "categories", category.Slug, category.Name, category.Description); err != nil {
			return fmt.Errorf("write category snapshot %q: %w", category.Slug, err)
		}
	}
	for _, tag := range post.Tags {
		if err := writeTaxonomyTermSnapshot(siteDir, "tags", tag.Slug, tag.Name, tag.Description); err != nil {
			return fmt.Errorf("write tag snapshot %q: %w", tag.Slug, err)
		}
	}
	return nil
}

func writeTaxonomyTermSnapshot(siteDir string, taxonomy string, slug string, name string, description string) error {
	slug = strings.TrimSpace(slug)
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if taxonomy != "categories" && taxonomy != "tags" {
		return fmt.Errorf("invalid taxonomy")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("taxonomy name is required")
	}

	termDir := filepath.Join(siteDir, "content", taxonomy, slug)
	if err := os.MkdirAll(termDir, 0o755); err != nil {
		return err
	}
	lines := []string{
		"---",
		"title: " + yamlString(name),
		"slug: " + yamlString(slug),
		"description: " + yamlString(strings.TrimSpace(description)),
		"zokingManaged: true",
		"---",
		"",
	}
	return os.WriteFile(filepath.Join(termDir, "_index.md"), []byte(strings.Join(lines, "\n")), 0o644)
}

func postCoverImageURL(post model.Post) string {
	if post.CoverMedia == nil {
		return ""
	}
	return strings.TrimSpace(post.CoverMedia.PublicURL)
}

func ValidateSlug(slug string) error {
	if strings.TrimSpace(slug) != slug || slug == "" {
		return fmt.Errorf("invalid slug")
	}
	if strings.Contains(slug, "..") || strings.ContainsAny(slug, `/\:. `) {
		return fmt.Errorf("invalid slug")
	}
	if !safeSlugPattern.MatchString(slug) {
		return fmt.Errorf("invalid slug")
	}
	switch strings.ToUpper(slug) {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return fmt.Errorf("invalid slug")
	default:
		return nil
	}
}

func ValidatePageSlug(slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if _, ok := reservedPageSlugs[strings.ToLower(slug)]; ok {
		return fmt.Errorf("reserved page slug")
	}
	return nil
}

type SiteSettingsSnapshot struct {
	Site struct {
		Title   string `json:"title"`
		BaseURL string `json:"base_url"`
	} `json:"site"`
	Sidebar struct {
		Emoji    string `json:"emoji"`
		Subtitle string `json:"subtitle"`
	} `json:"sidebar"`
	Comments struct {
		Enabled bool   `json:"enabled"`
		APIBase string `json:"api_base"`
	} `json:"comments"`
	Footer struct {
		Since int `json:"since"`
	} `json:"footer"`
	Pagination struct {
		PagerSize int `json:"pager_size"`
	} `json:"pagination"`
}

func LoadSiteSettings(ctx context.Context, db *gorm.DB) (SiteSettingsSnapshot, string, error) {
	settings := defaultSiteSettings()
	var rows []model.SiteSetting
	if err := db.WithContext(ctx).Where("is_public = ?", true).Find(&rows).Error; err != nil {
		return settings, "", err
	}

	values := map[string]json.RawMessage{}
	for _, row := range rows {
		values[row.Key] = json.RawMessage(row.ValueJSON)
	}
	applyStringSetting(values, "site.title", &settings.Site.Title)
	applyStringSetting(values, "site.base_url", &settings.Site.BaseURL)
	applyStringSetting(values, "site.subtitle", &settings.Sidebar.Subtitle)
	applyStringSetting(values, "sidebar.subtitle", &settings.Sidebar.Subtitle)
	applyStringSetting(values, "sidebar.emoji", &settings.Sidebar.Emoji)
	applyBoolSetting(values, "comments.enabled", &settings.Comments.Enabled)
	applyStringSetting(values, "comments.api_base", &settings.Comments.APIBase)
	applyIntSetting(values, "footer.since", &settings.Footer.Since)
	applyIntSetting(values, "pagination.pager_size", &settings.Pagination.PagerSize)

	hash, err := hashSiteSettings(settings)
	if err != nil {
		return settings, "", err
	}
	return settings, hash, nil
}

func hashSiteSettings(settings SiteSettingsSnapshot) (string, error) {
	raw, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func ApplySiteSettings(siteDir string, settings SiteSettingsSnapshot) error {
	if settings.Comments.Enabled && strings.TrimSpace(settings.Comments.APIBase) == "" {
		return fmt.Errorf("comments API base is required when comments are enabled")
	}
	absSiteDir, err := filepath.Abs(siteDir)
	if err != nil {
		return err
	}
	hugoPath := filepath.Join(absSiteDir, "config", "_default", "hugo.toml")
	paramsPath := filepath.Join(absSiteDir, "config", "_default", "params.toml")
	languagesPath := filepath.Join(absSiteDir, "config", "_default", "languages.toml")

	if err := setTomlValue(hugoPath, "", "title", tomlString(settings.Site.Title)); err != nil {
		return err
	}
	if err := setTomlValue(hugoPath, "", "baseURL", tomlString(settings.Site.BaseURL)); err != nil {
		return err
	}
	if err := setTomlValue(hugoPath, "pagination", "pagerSize", strconv.Itoa(settings.Pagination.PagerSize)); err != nil {
		return err
	}
	defaultLanguage := readTomlStringValue(hugoPath, "", "defaultContentLanguage", "en")
	if fileExists(languagesPath) {
		if err := setTomlValue(languagesPath, defaultLanguage, "title", tomlString(settings.Site.Title)); err != nil {
			return err
		}
		if err := setTomlValue(languagesPath, defaultLanguage+".params.sidebar", "subtitle", tomlString(settings.Sidebar.Subtitle)); err != nil {
			return err
		}
	}
	if err := setTomlValue(paramsPath, "sidebar", "emoji", tomlString(settings.Sidebar.Emoji)); err != nil {
		return err
	}
	if err := setTomlValue(paramsPath, "sidebar", "subtitle", tomlString(settings.Sidebar.Subtitle)); err != nil {
		return err
	}
	if err := setTomlValue(paramsPath, "footer", "since", strconv.Itoa(settings.Footer.Since)); err != nil {
		return err
	}
	if err := setTomlValue(paramsPath, "comments", "enabled", boolString(settings.Comments.Enabled)); err != nil {
		return err
	}
	return setTomlValue(paramsPath, "comments.public", "apiBase", tomlString(settings.Comments.APIBase))
}

func yamlList(key string, values []string) []string {
	if len(values) == 0 {
		return []string{key + ": []"}
	}

	lines := []string{key + ":"}
	for _, value := range values {
		lines = append(lines, "  - "+yamlString(value))
	}
	return lines
}

func buildPageMarkdown(page model.Page) string {
	publishedAt := page.CreatedAt
	if page.PublishedAt != nil {
		publishedAt = *page.PublishedAt
	}

	description := page.SEODescription
	if description == "" {
		description = page.Summary
	}

	lines := []string{
		"---",
		"title: " + yamlString(page.Title),
		"description: " + yamlString(description),
		"slug: " + yamlString(page.Slug),
		"date: " + publishedAt.Format(time.RFC3339),
		"lastmod: " + page.UpdatedAt.Format(time.RFC3339),
		"draft: false",
		"comments: " + boolString(page.AllowComment),
		"toc: true",
	}
	if page.ShowInMenu {
		lines = append(lines,
			"menu:",
			"  main:",
			"    weight: "+strconv.Itoa(page.MenuWeight),
		)
		if strings.TrimSpace(page.MenuIcon) != "" {
			lines = append(lines,
				"    params:",
				"      icon: "+yamlString(strings.TrimSpace(page.MenuIcon)),
			)
		}
	}
	lines = append(lines, "---", "", strings.TrimSpace(page.ContentMD), "")
	return strings.Join(lines, "\n")
}

func defaultSiteSettings() SiteSettingsSnapshot {
	var settings SiteSettingsSnapshot
	settings.Site.Title = "Zoking Blog"
	settings.Site.BaseURL = "http://localhost:1313/"
	settings.Sidebar.Emoji = "✏️"
	settings.Sidebar.Subtitle = "Card-style full-stack blog powered by Stack."
	settings.Comments.Enabled = true
	settings.Comments.APIBase = "http://localhost:18080"
	settings.Footer.Since = 2020
	settings.Pagination.PagerSize = 3
	return settings
}

func applyStringSetting(values map[string]json.RawMessage, key string, target *string) {
	if raw, ok := values[key]; ok {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil && strings.TrimSpace(value) != "" {
			*target = value
		}
	}
}

func applyBoolSetting(values map[string]json.RawMessage, key string, target *bool) {
	if raw, ok := values[key]; ok {
		var value bool
		if err := json.Unmarshal(raw, &value); err == nil {
			*target = value
		}
	}
}

func applyIntSetting(values map[string]json.RawMessage, key string, target *int) {
	if raw, ok := values[key]; ok {
		var value int
		if err := json.Unmarshal(raw, &value); err == nil && value > 0 {
			*target = value
		}
	}
}

func setTomlValue(path string, section string, key string, value string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	currentSection := ""
	targetSectionFound := section == ""
	valueSet := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = strings.Trim(trimmed, "[]")
			if currentSection == section {
				targetSectionFound = true
			}
			continue
		}
		if currentSection == section && strings.HasPrefix(trimmed, key) {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
			if strings.HasPrefix(rest, "=") {
				prefix := line[:strings.Index(line, key)]
				lines[i] = prefix + key + " = " + value
				valueSet = true
				break
			}
		}
		if section == "" && currentSection != "" && targetSectionFound && !valueSet {
			break
		}
	}

	if !valueSet {
		if !targetSectionFound && section != "" {
			lines = append(lines, "", "["+section+"]")
		}
		lines = append(lines, key+" = "+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func readTomlStringValue(path string, section string, key string, fallback string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	lines := strings.Split(string(raw), "\n")
	currentSection := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = strings.Trim(trimmed, "[]")
			continue
		}
		if currentSection != section || !strings.HasPrefix(trimmed, key) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(rest, "="))
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return fallback
		}
		if strings.TrimSpace(unquoted) == "" {
			return fallback
		}
		return unquoted
	}
	return fallback
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func tomlString(value string) string {
	return strconv.Quote(value)
}

func yamlString(value string) string {
	return strconv.Quote(value)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
