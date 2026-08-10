package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const adminOrigin = "https://admin.restore.invalid"

type probe struct {
	client  *http.Client
	apiURL  string
	siteURL string
}

type loginResponse struct {
	Data struct {
		CSRFToken string `json:"csrf_token"`
		User      struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"data"`
}

type publishResponse struct {
	Data struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	} `json:"data"`
}

type jobResponse struct {
	Data struct {
		Status    string `json:"status"`
		ErrorCode string `json:"error_code"`
	} `json:"data"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	apiURL := requiredEnv("PROBE_API_URL")
	siteURL := requiredEnv("PROBE_SITE_URL")
	account := requiredEnv("PROBE_ADMIN_ACCOUNT")
	mediaPath := strings.TrimSpace(os.Getenv("PROBE_MEDIA_PATH"))
	skipPublish := strings.EqualFold(strings.TrimSpace(os.Getenv("PROBE_SKIP_PUBLISH")), "true")

	password, err := io.ReadAll(io.LimitReader(os.Stdin, 4097))
	if err != nil {
		log.Fatalf("read administrator password: %v", err)
	}
	if len(password) == 0 || len(password) > 4096 {
		log.Fatal("administrator password input is empty or too large")
	}
	defer clear(password)

	p := probe{
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		apiURL:  strings.TrimRight(apiURL, "/"),
		siteURL: strings.TrimRight(siteURL, "/"),
	}

	if err := p.waitForStatus(ctx, p.apiURL+"/readyz", http.StatusOK, time.Minute); err != nil {
		log.Fatal(err)
	}
	log.Print("[zoking-probe] API readiness passed")

	for _, path := range []string{
		"/api/v1/public/posts?page=1&page_size=1",
		"/api/v1/public/site/public-settings",
	} {
		if _, _, err := p.request(ctx, http.MethodGet, p.apiURL+path, nil, ""); err != nil {
			log.Fatal(err)
		}
	}
	if err := p.waitForStatus(ctx, p.siteURL+"/", http.StatusOK, 30*time.Second); err != nil {
		log.Fatal(err)
	}

	mediaURL := p.siteURL + "/media-files/__restore-probe-missing__"
	mediaStatus := http.StatusNotFound
	if mediaPath != "" {
		mediaURL = p.siteURL + "/media-files/" + strings.TrimLeft(mediaPath, "/")
		mediaStatus = http.StatusOK
	}
	if _, _, err := p.requestExpect(ctx, http.MethodGet, mediaURL, nil, "", mediaStatus); err != nil {
		log.Fatal(err)
	}
	log.Print("[zoking-probe] public API, site, and media reads passed")

	loginPayload, err := json.Marshal(map[string]string{
		"account":  account,
		"password": string(password),
	})
	if err != nil {
		log.Fatalf("encode login request: %v", err)
	}
	clear(password)

	body, response, err := p.requestExpect(ctx, http.MethodPost, p.apiURL+"/api/v1/admin/auth/login", loginPayload, "", http.StatusOK)
	clear(loginPayload)
	if err != nil {
		log.Fatal(err)
	}
	var login loginResponse
	if err := json.Unmarshal(body, &login); err != nil || login.Data.User.ID == "" || login.Data.CSRFToken == "" {
		log.Fatal("administrator login response is invalid")
	}
	accessToken := ""
	for _, cookie := range response.Cookies() {
		if cookie.Name == "zoking_admin_access" {
			accessToken = cookie.Value
			break
		}
	}
	if accessToken == "" {
		log.Fatal("administrator access token cookie is missing")
	}
	defer func() { accessToken = "" }()

	if _, _, err := p.request(ctx, http.MethodGet, p.apiURL+"/api/v1/admin/auth/me", nil, accessToken); err != nil {
		log.Fatal(err)
	}
	log.Print("[zoking-probe] restored administrator login passed")

	if !skipPublish {
		body, _, err = p.requestExpect(ctx, http.MethodPost, p.apiURL+"/api/v1/admin/settings/publish", nil, accessToken, http.StatusAccepted)
		if err != nil {
			log.Fatal(err)
		}
		var published publishResponse
		if err := json.Unmarshal(body, &published); err != nil || published.Data.Job.ID == "" {
			log.Fatal("isolated publish response is invalid")
		}
		if err := p.waitForPublish(ctx, published.Data.Job.ID, accessToken); err != nil {
			log.Fatal(err)
		}
		if _, _, err := p.request(ctx, http.MethodGet, p.siteURL+"/", nil, ""); err != nil {
			log.Fatalf("site read after isolated publish: %v", err)
		}
	}

	log.Printf("[zoking-probe] service checks passed publish_exercised=%t", !skipPublish)
}

func (p probe) waitForStatus(ctx context.Context, url string, expected int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		_, _, lastErr = p.requestExpect(ctx, http.MethodGet, url, nil, "", expected)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("timed out waiting for %s: %w", url, lastErr)
}

func (p probe) waitForPublish(ctx context.Context, jobID, token string) error {
	deadline := time.Now().Add(3 * time.Minute)
	lastStatus := ""
	for time.Now().Before(deadline) {
		body, _, err := p.request(ctx, http.MethodGet, p.apiURL+"/api/v1/admin/publish/jobs/"+jobID, nil, token)
		if err != nil {
			return err
		}
		var job jobResponse
		if err := json.Unmarshal(body, &job); err != nil {
			return fmt.Errorf("decode isolated publish status: %w", err)
		}
		if job.Data.Status != lastStatus {
			log.Printf("[zoking-probe] isolated publish status=%s", job.Data.Status)
			lastStatus = job.Data.Status
		}
		switch job.Data.Status {
		case "published":
			return nil
		case "failed", "canceled":
			return fmt.Errorf("isolated publish ended with status=%s error_code=%s", job.Data.Status, job.Data.ErrorCode)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New("isolated publish did not finish within 180 seconds")
}

func (p probe) request(ctx context.Context, method, url string, body []byte, token string) ([]byte, *http.Response, error) {
	return p.requestExpect(ctx, method, url, body, token, http.StatusOK)
}

func (p probe) requestExpect(ctx context.Context, method, url string, body []byte, token string, expected int) ([]byte, *http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create %s request: %w", url, err)
	}
	if strings.HasPrefix(url, p.apiURL+"/api/v1/admin/") {
		request.Header.Set("Origin", adminOrigin)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("request %s: %w", url, err)
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	response.Body.Close()
	if readErr != nil {
		return nil, response, fmt.Errorf("read %s response: %w", url, readErr)
	}
	if response.StatusCode != expected {
		return nil, response, fmt.Errorf("%s returned HTTP %d, expected %d", url, response.StatusCode, expected)
	}
	return responseBody, response, nil
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}
