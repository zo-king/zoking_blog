package hugoimport

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type Importer struct{}

func (Importer) Run(ctx context.Context, options Options) (Result, error) {
	if strings.TrimSpace(options.MediaBase) == "" {
		options.MediaBase = strings.TrimRight(strings.TrimSpace(options.APIBase), "/") + "/media-files"
	}
	articles, mapping, err := Preflight(options.SiteDir, options.TaxonomyMap, options.MediaBase)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(options.APIBase) == "" || strings.TrimSpace(options.Email) == "" || options.Password == "" {
		return Result{}, fmt.Errorf("api-base, email, and password are required")
	}
	if options.PollEvery <= 0 {
		options.PollEvery = time.Second
	}
	if options.JobTimeout <= 0 {
		options.JobTimeout = 3 * time.Minute
	}
	if options.Out == nil {
		options.Out = io.Discard
	}

	client := newAPIClient(options.APIBase)
	if err := client.login(ctx, options.Email, options.Password); err != nil {
		return Result{}, fmt.Errorf("login: %w", err)
	}

	// Complete remote discovery before the first mutating API request.
	existingCategories, err := client.listTaxonomy(ctx, "categories")
	if err != nil {
		return Result{}, fmt.Errorf("list categories: %w", err)
	}
	existingTags, err := client.listTaxonomy(ctx, "tags")
	if err != nil {
		return Result{}, fmt.Errorf("list tags: %w", err)
	}
	mediaByChecksum := map[string]mediaRecord{}
	mediaByManagedURL := map[string]mediaRecord{}
	missingAssets := map[string]Asset{}
	result := Result{}
	for articleIndex := range articles {
		for assetIndex := range articles[articleIndex].Assets {
			asset := &articles[articleIndex].Assets[assetIndex]
			if asset.Checksum == "" {
				media, ok := mediaByManagedURL[asset.ManagedURL]
				if !ok {
					found, err := client.findMediaByPublicURL(ctx, asset.ManagedURL)
					if err != nil {
						return result, fmt.Errorf("query managed media %s: %w", asset.ManagedURL, err)
					}
					if found == nil {
						return result, fmt.Errorf("managed media URL %s does not exist in the Admin API", asset.ManagedURL)
					}
					media = *found
					mediaByManagedURL[asset.ManagedURL] = media
					result.Reused++
				}
				asset.Checksum = media.Checksum
				mediaByChecksum[media.Checksum] = media
			}
			if _, ok := mediaByChecksum[asset.Checksum]; ok || missingAssets[asset.Checksum].Checksum != "" {
				continue
			}
			media, err := client.findMedia(ctx, asset.Checksum)
			if err != nil {
				return result, fmt.Errorf("query media %s: %w", asset.StaticURL, err)
			}
			if media == nil {
				missingAssets[asset.Checksum] = *asset
			} else {
				result.Reused++
				mediaByChecksum[asset.Checksum] = *media
			}
		}
	}
	existingPosts := make(map[string]*postRecord, len(articles))
	for _, article := range articles {
		post, err := client.findPost(ctx, article.Slug)
		if err != nil {
			return result, fmt.Errorf("query post %s: %w", article.Slug, err)
		}
		existingPosts[article.Slug] = post
	}

	categoryIDs, err := ensureTaxonomies(ctx, client, "categories", mapping.Categories, articles, true, existingCategories)
	if err != nil {
		return Result{}, err
	}
	tagIDs, err := ensureTaxonomies(ctx, client, "tags", mapping.Tags, articles, false, existingTags)
	if err != nil {
		return Result{}, err
	}
	checksums := make([]string, 0, len(missingAssets))
	for checksum := range missingAssets {
		checksums = append(checksums, checksum)
	}
	sort.Strings(checksums)
	for _, checksum := range checksums {
		asset := missingAssets[checksum]
		uploaded, err := client.uploadMedia(ctx, asset)
		if err != nil {
			return result, fmt.Errorf("upload media %s: %w", asset.StaticURL, err)
		}
		mediaByChecksum[checksum] = uploaded
		result.Uploaded++
	}

	for _, article := range articles {
		existing := existingPosts[article.Slug]
		status := "draft"
		if existing != nil && strings.TrimSpace(existing.Status) != "" {
			status = existing.Status
		}
		payload, err := articlePayload(article, categoryIDs, tagIDs, mediaByChecksum, status)
		if err != nil {
			return result, err
		}
		var post postRecord
		changed := existing == nil || !postMatchesPayload(*existing, payload)
		if existing == nil {
			post, err = client.createPost(ctx, payload)
			if err == nil {
				result.Created++
			}
		} else if changed {
			post, err = client.updatePost(ctx, existing.ID, payload)
			if err == nil {
				result.Updated++
			}
		} else {
			post = *existing
			result.Unchanged++
		}
		if err != nil {
			return result, fmt.Errorf("upsert post %s: %w", article.Slug, err)
		}
		if changed {
			fmt.Fprintf(options.Out, "upserted %s (%s)\n", article.Slug, post.ID)
		} else {
			fmt.Fprintf(options.Out, "unchanged %s (%s)\n", article.Slug, post.ID)
		}
		shouldPublish := options.Publish && (existing == nil || changed || existing.Status != "published")
		if shouldPublish {
			job, err := client.publishPost(ctx, post.ID)
			if err != nil {
				return result, fmt.Errorf("publish post %s: %w", article.Slug, err)
			}
			if err := waitForJob(ctx, client, job.ID, options.PollEvery, options.JobTimeout); err != nil {
				return result, fmt.Errorf("publish post %s: %w", article.Slug, err)
			}
			result.Published++
		}
	}
	return result, nil
}

func ensureTaxonomies(ctx context.Context, client *apiClient, kind string, mapping map[string]string, articles []Article, categories bool, existing []taxonomyRecord) (map[string]string, error) {
	bySlug := make(map[string]taxonomyRecord, len(existing))
	for _, item := range existing {
		bySlug[item.Slug] = item
	}
	names := map[string]bool{}
	for _, article := range articles {
		values := article.Tags
		if categories {
			values = article.Categories
		}
		for _, name := range values {
			names[name] = true
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	result := make(map[string]string, len(ordered))
	for _, name := range ordered {
		slug := mapping[name]
		item, ok := bySlug[slug]
		if !ok {
			var err error
			item, err = client.createTaxonomy(ctx, kind, name, slug)
			if err != nil {
				return nil, fmt.Errorf("create %s %q: %w", kind, name, err)
			}
			bySlug[slug] = item
		}
		if item.Name != name {
			return nil, fmt.Errorf("%s slug %q belongs to %q, expected %q", kind, slug, item.Name, name)
		}
		result[name] = item.ID
	}
	return result, nil
}

func articlePayload(article Article, categoryIDs, tagIDs map[string]string, media map[string]mediaRecord, status string) (map[string]any, error) {
	replacements := map[string]string{}
	assetByURL := map[string]Asset{}
	for _, asset := range article.Assets {
		record, ok := media[asset.Checksum]
		if !ok {
			return nil, fmt.Errorf("media was not prepared for %s", asset.StaticURL)
		}
		replacements[asset.StaticURL] = record.PublicURL
		assetByURL[asset.StaticURL] = asset
	}
	body := rewriteStaticURLs(article.Body, replacements)
	categories := make([]string, 0, len(article.Categories))
	for _, name := range article.Categories {
		categories = append(categories, categoryIDs[name])
	}
	tags := make([]string, 0, len(article.Tags))
	for _, name := range article.Tags {
		tags = append(tags, tagIDs[name])
	}
	payload := map[string]any{
		"title": article.Title, "slug": article.Slug, "summary": article.Description,
		"content_md": body, "status": status, "visibility": "public", "allow_comment": true,
		"seo_title": article.Title, "seo_description": article.Description,
		"category_ids": categories, "tag_ids": tags, "published_at": article.PublishedAt.Format(time.RFC3339),
	}
	if article.Image == "" {
		payload["cover_media_id"] = ""
	} else {
		asset, ok := assetByURL[article.Image]
		if !ok {
			return nil, fmt.Errorf("cover image was not prepared for %s", article.Slug)
		}
		payload["cover_media_id"] = media[asset.Checksum].ID
	}
	return payload, nil
}

func postMatchesPayload(post postRecord, payload map[string]any) bool {
	if post.Title != payload["title"] || post.Slug != payload["slug"] || post.Summary != payload["summary"] ||
		post.ContentMD != payload["content_md"] || post.Status != payload["status"] || post.Visibility != payload["visibility"] ||
		post.AllowComment != payload["allow_comment"] || post.SEOTitle != payload["seo_title"] || post.SEODescription != payload["seo_description"] {
		return false
	}
	if !sameOptionalString(post.CoverMediaID, payload["cover_media_id"].(string)) {
		return false
	}
	publishedAt, err := time.Parse(time.RFC3339, payload["published_at"].(string))
	if err != nil || post.PublishedAt == nil || !post.PublishedAt.Equal(publishedAt) {
		return false
	}
	return sameTaxonomyIDs(post.Categories, payload["category_ids"].([]string)) && sameTaxonomyIDs(post.Tags, payload["tag_ids"].([]string))
}

func sameOptionalString(value *string, expected string) bool {
	if expected == "" {
		return value == nil || strings.TrimSpace(*value) == ""
	}
	return value != nil && *value == expected
}

func sameTaxonomyIDs(records []taxonomyRecord, expected []string) bool {
	if len(records) != len(expected) {
		return false
	}
	actual := make([]string, 0, len(records))
	for _, record := range records {
		actual = append(actual, record.ID)
	}
	expected = append([]string(nil), expected...)
	sort.Strings(actual)
	sort.Strings(expected)
	for i := range actual {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func rewriteStaticURLs(body string, replacements map[string]string) string {
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, key := range keys {
		body = strings.ReplaceAll(body, key, replacements[key])
	}
	return body
}

func waitForJob(ctx context.Context, client *apiClient, id string, interval, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		job, err := client.getJob(ctx, id)
		if err != nil {
			return err
		}
		switch job.Status {
		case "published":
			return nil
		case "failed", "canceled":
			return fmt.Errorf("job %s ended as %s (%s)", id, job.Status, job.ErrorCode)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("job %s did not finish within %s", id, timeout)
		case <-ticker.C:
		}
	}
}
