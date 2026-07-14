package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParsePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		query    string
		wantOK   bool
		wantPage int
		wantSize int
	}{
		{name: "defaults", wantOK: true, wantPage: 1, wantSize: 20},
		{name: "explicit", query: "?page=3&page_size=50&q=hello", wantOK: true, wantPage: 3, wantSize: 50},
		{name: "legacy limit", query: "?limit=40", wantOK: true, wantPage: 1, wantSize: 40},
		{name: "invalid page", query: "?page=0", wantOK: false},
		{name: "oversized page size", query: "?page_size=101", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest("GET", "/items"+tt.query, nil)

			got, ok := parsePagination(context)
			if ok != tt.wantOK {
				t.Fatalf("parsePagination() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				if recorder.Code != 422 {
					t.Fatalf("invalid query status = %d, want 422", recorder.Code)
				}
				return
			}
			if got.Page != tt.wantPage || got.PageSize != tt.wantSize {
				t.Fatalf("parsePagination() = page %d size %d, want page %d size %d", got.Page, got.PageSize, tt.wantPage, tt.wantSize)
			}
		})
	}
}

func TestOKPaginatedIncludesTotalPages(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/items", nil)

	OKPaginated(context, []string{"item"}, 41, paginationQuery{Page: 2, PageSize: 20})

	var response struct {
		Pagination paginationMeta `json:"pagination"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Pagination.TotalPages != 3 {
		t.Fatalf("total_pages = %d, want 3", response.Pagination.TotalPages)
	}
}

func TestReturnEmptyPageIfOutOfRange(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/items?page=1000000", nil)
	query := paginationQuery{Page: 1_000_000, PageSize: 100, Offset: 99_999_900}

	if !returnEmptyPageIfOutOfRange[string](context, 3, query) {
		t.Fatal("expected out-of-range page to be handled")
	}
	var response struct {
		Data       []string       `json:"data"`
		Pagination paginationMeta `json:"pagination"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 0 || response.Pagination.Total != 3 {
		t.Fatalf("unexpected empty-page response: %#v", response)
	}
}
