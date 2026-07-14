package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidRequestIDRejectsUnsafeOrOversizedValues(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("a", 129), "bad\r\nheader", "非ASCII"} {
		if validRequestID(value) {
			t.Fatalf("validRequestID(%q) = true", value)
		}
	}
	if !validRequestID("request-123") {
		t.Fatal("expected printable request ID to be accepted")
	}
}

func TestRequestBodyLimitRejectsKnownOversizedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(requestBodyLimitMiddleware(4))
	router.POST("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("12345"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", response.Code)
	}
}
