package hugoimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type apiClient struct {
	base      string
	csrfToken string
	http      *http.Client
}

type envelope[T any] struct {
	Data T `json:"data"`
}

type loginResponse struct {
	CSRFToken string `json:"csrf_token"`
}

type mediaRecord struct {
	ID        string `json:"id"`
	Checksum  string `json:"checksum"`
	PublicURL string `json:"public_url"`
}

type taxonomyRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type postRecord struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	Slug           string           `json:"slug"`
	Summary        string           `json:"summary"`
	ContentMD      string           `json:"content_md"`
	Status         string           `json:"status"`
	Visibility     string           `json:"visibility"`
	AllowComment   bool             `json:"allow_comment"`
	PublishedAt    *time.Time       `json:"published_at"`
	CoverMediaID   *string          `json:"cover_media_id"`
	SEOTitle       string           `json:"seo_title"`
	SEODescription string           `json:"seo_description"`
	Categories     []taxonomyRecord `json:"categories"`
	Tags           []taxonomyRecord `json:"tags"`
}

type jobRecord struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code"`
}

func newAPIClient(base string) *apiClient {
	jar, _ := cookiejar.New(nil)
	return &apiClient{base: strings.TrimRight(base, "/"), http: &http.Client{Jar: jar}}
}

func (c *apiClient) login(ctx context.Context, email, password string) error {
	var result envelope[loginResponse]
	if err := c.json(ctx, http.MethodPost, "/api/v1/admin/auth/login", map[string]string{"email": email, "password": password}, &result); err != nil {
		return err
	}
	if result.Data.CSRFToken == "" {
		return fmt.Errorf("login response did not contain a CSRF token")
	}
	c.csrfToken = result.Data.CSRFToken
	return nil
}

func (c *apiClient) findMedia(ctx context.Context, checksum string) (*mediaRecord, error) {
	var result envelope[[]mediaRecord]
	if err := c.json(ctx, http.MethodGet, "/api/v1/admin/media?checksum="+url.QueryEscape(checksum), nil, &result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, nil
	}
	if len(result.Data) != 1 || !strings.EqualFold(result.Data[0].Checksum, checksum) {
		return nil, fmt.Errorf("media checksum query returned a non-exact result for %s", checksum)
	}
	return &result.Data[0], nil
}

func (c *apiClient) findMediaByPublicURL(ctx context.Context, publicURL string) (*mediaRecord, error) {
	var result envelope[[]mediaRecord]
	if err := c.json(ctx, http.MethodGet, "/api/v1/admin/media?public_url="+url.QueryEscape(publicURL), nil, &result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, nil
	}
	if len(result.Data) != 1 || result.Data[0].PublicURL != publicURL || strings.TrimSpace(result.Data[0].Checksum) == "" {
		return nil, fmt.Errorf("media public URL query returned a non-exact result for %s", publicURL)
	}
	return &result.Data[0], nil
}

func (c *apiClient) uploadMedia(ctx context.Context, asset Asset) (mediaRecord, error) {
	file, err := os.Open(asset.Path)
	if err != nil {
		return mediaRecord{}, err
	}
	defer file.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(asset.Path))
	if err != nil {
		return mediaRecord{}, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return mediaRecord{}, err
	}
	if err := writer.Close(); err != nil {
		return mediaRecord{}, err
	}
	req, err := c.request(ctx, http.MethodPost, "/api/v1/admin/media", &body)
	if err != nil {
		return mediaRecord{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var result envelope[mediaRecord]
	if err := c.do(req, &result); err != nil {
		return mediaRecord{}, err
	}
	if !strings.EqualFold(result.Data.Checksum, asset.Checksum) {
		return mediaRecord{}, fmt.Errorf("uploaded media checksum mismatch for %s", asset.Path)
	}
	return result.Data, nil
}

func (c *apiClient) listTaxonomy(ctx context.Context, kind string) ([]taxonomyRecord, error) {
	var result envelope[[]taxonomyRecord]
	if err := c.json(ctx, http.MethodGet, "/api/v1/admin/"+kind, nil, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *apiClient) createTaxonomy(ctx context.Context, kind, name, slug string) (taxonomyRecord, error) {
	var result envelope[taxonomyRecord]
	if err := c.json(ctx, http.MethodPost, "/api/v1/admin/"+kind, map[string]any{"name": name, "slug": slug}, &result); err != nil {
		return taxonomyRecord{}, err
	}
	return result.Data, nil
}

func (c *apiClient) findPost(ctx context.Context, slug string) (*postRecord, error) {
	var result envelope[[]postRecord]
	if err := c.json(ctx, http.MethodGet, "/api/v1/admin/posts?slug="+url.QueryEscape(slug), nil, &result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, nil
	}
	if len(result.Data) != 1 || result.Data[0].Slug != slug {
		return nil, fmt.Errorf("post slug query returned a non-exact result for %q", slug)
	}
	return &result.Data[0], nil
}

func (c *apiClient) createPost(ctx context.Context, payload map[string]any) (postRecord, error) {
	var result envelope[postRecord]
	if err := c.json(ctx, http.MethodPost, "/api/v1/admin/posts", payload, &result); err != nil {
		return postRecord{}, err
	}
	return result.Data, nil
}

func (c *apiClient) updatePost(ctx context.Context, id string, payload map[string]any) (postRecord, error) {
	var result envelope[postRecord]
	if err := c.json(ctx, http.MethodPatch, "/api/v1/admin/posts/"+url.PathEscape(id), payload, &result); err != nil {
		return postRecord{}, err
	}
	return result.Data, nil
}

func (c *apiClient) publishPost(ctx context.Context, id string) (jobRecord, error) {
	var result envelope[struct {
		Job jobRecord `json:"job"`
	}]
	if err := c.json(ctx, http.MethodPost, "/api/v1/admin/posts/"+url.PathEscape(id)+"/publish", map[string]any{}, &result); err != nil {
		return jobRecord{}, err
	}
	return result.Data.Job, nil
}

func (c *apiClient) getJob(ctx context.Context, id string) (jobRecord, error) {
	var result envelope[jobRecord]
	if err := c.json(ctx, http.MethodGet, "/api/v1/admin/publish/jobs/"+url.PathEscape(id), nil, &result); err != nil {
		return jobRecord{}, err
	}
	return result.Data, nil
}

func (c *apiClient) json(ctx context.Context, method, path string, payload any, output any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := c.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, output)
}

func (c *apiClient) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if c.csrfToken != "" && method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		req.Header.Set("X-CSRF-Token", c.csrfToken)
	}
	return req, nil
}

func (c *apiClient) do(req *http.Request, output any) error {
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s returned %d: %s", req.Method, req.URL.Path, response.StatusCode, strings.TrimSpace(string(payload)))
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(payload, output); err != nil {
		return fmt.Errorf("decode %s %s response: %w", req.Method, req.URL.Path, err)
	}
	return nil
}
