package hugoimport

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var markdownImagePattern = regexp.MustCompile(`!\[[^\]]*\]\(([^\s)]+)(?:\s+[^)]*)?\)`)

func Preflight(siteDir, taxonomyMapPath, mediaBase string) ([]Article, TaxonomyMap, error) {
	mapping, err := loadTaxonomyMap(taxonomyMapPath)
	if err != nil {
		return nil, TaxonomyMap{}, err
	}
	managedMedia, err := parseManagedMediaBase(mediaBase)
	if err != nil {
		return nil, TaxonomyMap{}, err
	}
	postRoot := filepath.Join(siteDir, "content", "post")
	entries, err := os.ReadDir(postRoot)
	if err != nil {
		return nil, TaxonomyMap{}, fmt.Errorf("read post directory: %w", err)
	}
	var articles []Article
	var problems []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		article, err := parseArticle(siteDir, filepath.Join(postRoot, entry.Name(), "index.md"), entry.Name(), mapping, managedMedia)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		articles = append(articles, article)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, TaxonomyMap{}, fmt.Errorf("preflight failed:\n- %s", strings.Join(problems, "\n- "))
	}
	sort.Slice(articles, func(i, j int) bool { return articles[i].Slug < articles[j].Slug })
	return articles, mapping, nil
}

func loadTaxonomyMap(path string) (TaxonomyMap, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return TaxonomyMap{}, fmt.Errorf("read taxonomy map: %w", err)
	}
	var mapping TaxonomyMap
	if err := yaml.Unmarshal(payload, &mapping); err != nil {
		return TaxonomyMap{}, fmt.Errorf("parse taxonomy map: %w", err)
	}
	if mapping.Categories == nil || mapping.Tags == nil {
		return TaxonomyMap{}, fmt.Errorf("taxonomy map must define categories and tags")
	}
	mapping.Categories, err = canonicalTaxonomyMap(mapping.Categories, "category")
	if err != nil {
		return TaxonomyMap{}, err
	}
	mapping.Tags, err = canonicalTaxonomyMap(mapping.Tags, "tag")
	if err != nil {
		return TaxonomyMap{}, err
	}
	return mapping, nil
}

func parseArticle(siteDir, path, directoryName string, mapping TaxonomyMap, managedMedia managedMediaBase) (Article, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Article{}, fmt.Errorf("%s: read article: %w", path, err)
	}
	frontMatter, body, err := splitFrontMatter(string(payload))
	if err != nil {
		return Article{}, fmt.Errorf("%s: %w", path, err)
	}
	var fm FrontMatter
	if err := yaml.Unmarshal([]byte(frontMatter), &fm); err != nil {
		return Article{}, fmt.Errorf("%s: parse YAML front matter: %w", path, err)
	}
	fm.Title = strings.TrimSpace(fm.Title)
	fm.Slug = strings.TrimSpace(fm.Slug)
	fm.Description = strings.TrimSpace(fm.Description)
	fm.Image = strings.TrimSpace(fm.Image)
	if fm.Title == "" || fm.Slug == "" || strings.TrimSpace(fm.Date) == "" {
		return Article{}, fmt.Errorf("%s: title, slug, and date are required", path)
	}
	if directoryName != fm.Slug {
		return Article{}, fmt.Errorf("%s: directory basename %q must equal slug %q", path, directoryName, fm.Slug)
	}
	publishedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(fm.Date))
	if err != nil {
		return Article{}, fmt.Errorf("%s: invalid RFC3339 date: %w", path, err)
	}
	fm.Categories, err = normalizeTaxonomyNames(fm.Categories, mapping.Categories, "category")
	if err != nil {
		return Article{}, fmt.Errorf("%s: %w", path, err)
	}
	fm.Tags, err = normalizeTaxonomyNames(fm.Tags, mapping.Tags, "tag")
	if err != nil {
		return Article{}, fmt.Errorf("%s: %w", path, err)
	}
	urls := imageURLs(body)
	if fm.Image != "" {
		urls = append(urls, fm.Image)
	}
	assets, err := resolveAssets(siteDir, urls, managedMedia)
	if err != nil {
		return Article{}, fmt.Errorf("%s: %w", path, err)
	}
	return Article{FrontMatter: fm, PublishedAt: publishedAt, Body: strings.TrimSpace(body), SourcePath: path, Assets: assets}, nil
}

func splitFrontMatter(content string) (string, string, error) {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("YAML front matter must start with ---")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", fmt.Errorf("YAML front matter is not closed")
}

func imageURLs(body string) []string {
	matches := markdownImagePattern.FindAllStringSubmatch(body, -1)
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		result = append(result, match[1])
	}
	return result
}

type managedMediaBase struct {
	origin       string
	path         string
	publicPrefix string
}

func parseManagedMediaBase(value string) (managedMediaBase, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return managedMediaBase{}, fmt.Errorf("managed media base is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return managedMediaBase{}, fmt.Errorf("managed media base %q is invalid", value)
	}
	if parsed.IsAbs() && ((parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "") {
		return managedMediaBase{}, fmt.Errorf("managed media base %q is invalid", value)
	}
	if !parsed.IsAbs() && (parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/")) {
		return managedMediaBase{}, fmt.Errorf("managed media base %q must be an HTTP(S) URL or root-relative path", value)
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	if path == "" || hasParentPathSegment(parsed.Path) {
		return managedMediaBase{}, fmt.Errorf("managed media base %q has an invalid path", value)
	}
	origin := ""
	if parsed.IsAbs() {
		origin = strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
	}
	return managedMediaBase{
		origin:       origin,
		path:         path,
		publicPrefix: strings.TrimRight(value, "/"),
	}, nil
}

func (base managedMediaBase) normalize(value string) (string, bool, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return "", false, fmt.Errorf("image URL %q is not a valid managed media URL", value)
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", false, fmt.Errorf("image URL %q is not a valid managed media URL", value)
		}
		origin := strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
		if base.origin == "" || origin != base.origin {
			return "", false, nil
		}
	} else if parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return "", false, nil
	}
	path := parsed.EscapedPath()
	if hasParentPathSegment(parsed.Path) || (path != base.path && !strings.HasPrefix(path, base.path+"/")) {
		return "", false, nil
	}
	return base.publicPrefix + strings.TrimPrefix(path, base.path), true, nil
}

func hasParentPathSegment(path string) bool {
	for _, segment := range strings.Split(strings.ReplaceAll(path, "\\", "/"), "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func resolveAssets(siteDir string, urls []string, managedMedia managedMediaBase) ([]Asset, error) {
	seen := map[string]bool{}
	assets := make([]Asset, 0, len(urls))
	for _, staticURL := range urls {
		staticURL = strings.TrimSpace(staticURL)
		if staticURL == "" || seen[staticURL] {
			continue
		}
		managedURL, managed, err := managedMedia.normalize(staticURL)
		if err != nil {
			return nil, err
		}
		if managed {
			assets = append(assets, Asset{StaticURL: staticURL, ManagedURL: managedURL, Filename: filepath.Base(managedURL)})
			seen[staticURL] = true
			continue
		}
		if parsed, err := url.Parse(staticURL); err != nil || parsed.IsAbs() {
			return nil, fmt.Errorf("image URL %q is outside the managed media base", staticURL)
		}
		if !strings.HasPrefix(staticURL, "/") || strings.Contains(staticURL, "..") {
			return nil, fmt.Errorf("image URL %q must be a root-relative static path or managed media URL", staticURL)
		}
		path := filepath.Join(siteDir, "static", filepath.FromSlash(strings.TrimPrefix(staticURL, "/")))
		root := filepath.Clean(filepath.Join(siteDir, "static")) + string(filepath.Separator)
		if !strings.HasPrefix(filepath.Clean(path)+string(filepath.Separator), root) {
			return nil, fmt.Errorf("image URL %q escapes the static directory", staticURL)
		}
		payload, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("image %q does not exist under the Hugo static directory", staticURL)
		}
		if err != nil {
			return nil, fmt.Errorf("read image %q: %w", staticURL, err)
		}
		sum := sha256.Sum256(payload)
		assets = append(assets, Asset{StaticURL: staticURL, Path: path, Checksum: hex.EncodeToString(sum[:]), Filename: filepath.Base(path)})
		seen[staticURL] = true
	}
	return assets, nil
}

func normalizeTaxonomyNames(values []string, mapping map[string]string, kind string) ([]string, error) {
	lookup := make(map[string]string, len(mapping)*2)
	for name, slug := range mapping {
		lookup[name] = name
		lookup[slug] = name
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if name := lookup[value]; name != "" {
			if seen[name] {
				continue
			}
			normalized = append(normalized, name)
			seen[name] = true
			continue
		}
		return nil, fmt.Errorf("%s %q is missing from taxonomy map", kind, value)
	}
	return normalized, nil
}

func canonicalTaxonomyMap(mapping map[string]string, kind string) (map[string]string, error) {
	canonical := make(map[string]string, len(mapping))
	lookup := make(map[string]string, len(mapping)*2)
	for rawName, rawSlug := range mapping {
		name, slug := strings.TrimSpace(rawName), strings.TrimSpace(rawSlug)
		if name == "" || slug == "" {
			return nil, fmt.Errorf("taxonomy map contains an empty %s name or slug", kind)
		}
		if previous, ok := canonical[name]; ok && previous != slug {
			return nil, fmt.Errorf("taxonomy map assigns %s name %q more than once", kind, name)
		}
		for _, token := range []string{name, slug} {
			if owner, ok := lookup[token]; ok && owner != name {
				return nil, fmt.Errorf("taxonomy map %s token %q is ambiguous between %q and %q", kind, token, owner, name)
			}
			lookup[token] = name
		}
		canonical[name] = slug
	}
	return canonical, nil
}
