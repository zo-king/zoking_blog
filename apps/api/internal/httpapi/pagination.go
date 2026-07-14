package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 20
	maximumPageSize = 100
)

type paginationQuery struct {
	Page     int
	PageSize int
	Offset   int
	Query    string
	Status   string
	Sort     string
}

type paginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

func parsePagination(c *gin.Context) (paginationQuery, bool) {
	page, ok := parsePositiveQueryInt(c, "page", 1, 1_000_000)
	if !ok {
		return paginationQuery{}, false
	}

	pageSizeKey := "page_size"
	if c.Query(pageSizeKey) == "" && c.Query("limit") != "" {
		pageSizeKey = "limit"
	}
	pageSize, ok := parsePositiveQueryInt(c, pageSizeKey, defaultPageSize, maximumPageSize)
	if !ok {
		return paginationQuery{}, false
	}

	query := strings.TrimSpace(c.Query("q"))
	status := strings.TrimSpace(c.Query("status"))
	sort := strings.TrimSpace(c.Query("sort"))
	if len(query) > 200 || len(status) > 64 || len(sort) > 64 {
		Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid list query")
		return paginationQuery{}, false
	}

	return paginationQuery{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
		Query:    query,
		Status:   status,
		Sort:     sort,
	}, true
}

func parsePositiveQueryInt(c *gin.Context, key string, fallback, maximum int) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximum {
		Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid "+key)
		return 0, false
	}
	return value, true
}

func OKPaginated(c *gin.Context, data interface{}, total int64, query paginationQuery) {
	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(query.PageSize) - 1) / int64(query.PageSize)
	}
	c.JSON(http.StatusOK, gin.H{
		"data": data,
		"pagination": paginationMeta{
			Page:       query.Page,
			PageSize:   query.PageSize,
			Total:      total,
			TotalPages: totalPages,
		},
		"request_id": requestID(c),
	})
}

func returnEmptyPageIfOutOfRange[T any](c *gin.Context, total int64, query paginationQuery) bool {
	if int64(query.Offset) < total {
		return false
	}
	OKPaginated(c, []T{}, total, query)
	return true
}
