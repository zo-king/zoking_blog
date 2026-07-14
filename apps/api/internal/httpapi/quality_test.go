package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/contentquality"
)

func TestQualityCheckNewPostReportsBlockingIssues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/posts/quality-check", qualityCheckNewPost())
	request := httptest.NewRequest(http.MethodPost, "/posts/quality-check", strings.NewReader(`{
		"title":"", "slug":"bad slug", "content_md":"[运行](javascript:alert(1))", "visibility":"private"
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data contentquality.Report `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, code := range []string{"TITLE_REQUIRED", "SLUG_INVALID", "VISIBILITY_NOT_PUBLIC", "UNSAFE_URL"} {
		if !hasQualityIssue(response.Data, code) {
			t.Fatalf("missing issue %s: %#v", code, response.Data.Issues)
		}
	}
}

func TestQualityCheckNewPageUsesCreateDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/pages/quality-check", qualityCheckNewPage())
	request := httptest.NewRequest(http.MethodPost, "/pages/quality-check", strings.NewReader(`{
		"title":"关于", "slug":"about", "summary":"页面摘要页面摘要页面摘要页面摘要页面摘要",
		"content_md":"这是完整的页面正文。", "show_in_menu":true, "menu_icon":"user"
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response struct {
		Data contentquality.Report `json:"data"`
	}
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &response) != nil {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !response.Data.Ready || hasQualityIssue(response.Data, "VISIBILITY_NOT_PUBLIC") {
		t.Fatalf("create defaults should be publishable: %#v", response.Data)
	}
}

func TestQualityCheckNewPostBlocksIncompleteSeries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/posts/quality-check", qualityCheckNewPost())
	request := httptest.NewRequest(http.MethodPost, "/posts/quality-check", strings.NewReader(`{
		"title":"系列文章", "slug":"series-post", "content_md":"完整正文", "visibility":"public",
		"series_id":"10000000-0000-0000-0000-000000000001"
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response struct {
		Data contentquality.Report `json:"data"`
	}
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &response) != nil {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if response.Data.Ready || !hasQualityIssue(response.Data, "SERIES_ASSIGNMENT_INCOMPLETE") {
		t.Fatalf("incomplete series should be blocked: %#v", response.Data)
	}
}

func hasQualityIssue(report contentquality.Report, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
