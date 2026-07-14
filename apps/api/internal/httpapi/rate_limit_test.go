package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestCommentRateLimitMiddlewareReturnsRetryContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/comments", commentRateLimitMiddleware(config.Config{
		CommentRateLimitPerMinute: 1,
		CommentRateLimitBurst:     1,
	}), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := httptest.NewRequest(http.MethodPost, "/comments", nil)
	router.ServeHTTP(httptest.NewRecorder(), first)
	second := httptest.NewRequest(http.MethodPost, "/comments", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, second)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", response.Code)
	}
	if response.Header().Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q, want 60", response.Header().Get("Retry-After"))
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "RATE_LIMITED" {
		t.Fatalf("error code = %q, want RATE_LIMITED", payload.Error.Code)
	}
}

func TestCommentRateLimitMiddlewareConcurrentBurstAndIPIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := router.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}
	router.POST("/comments", commentRateLimitMiddleware(config.Config{
		CommentRateLimitPerMinute: 1,
		CommentRateLimitBurst:     4,
	}), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	statuses := make(chan int, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request := httptest.NewRequest(http.MethodPost, "/comments", nil)
			request.RemoteAddr = "127.0.0.1:10001"
			request.Header.Set("X-Forwarded-For", "203.0.113.10")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			statuses <- response.Code
		}()
	}
	wg.Wait()
	close(statuses)

	allowed, limited := 0, 0
	for status := range statuses {
		switch status {
		case http.StatusNoContent:
			allowed++
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatalf("unexpected status %d", status)
		}
	}
	if allowed != 4 || limited != 4 {
		t.Fatalf("same-IP burst results = allowed %d, limited %d; want 4/4", allowed, limited)
	}

	other := httptest.NewRequest(http.MethodPost, "/comments", nil)
	other.RemoteAddr = "127.0.0.1:10002"
	other.Header.Set("X-Forwarded-For", "203.0.113.11")
	otherResponse := httptest.NewRecorder()
	router.ServeHTTP(otherResponse, other)
	if otherResponse.Code != http.StatusNoContent {
		t.Fatalf("different IP status = %d, want 204", otherResponse.Code)
	}
}

func TestCommentRateLimitMiddlewareDisabledConfigurationAllowsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, cfg := range []config.Config{
		{AppEnv: "development", CommentRateLimitPerMinute: 0, CommentRateLimitBurst: 1},
		{AppEnv: "test", CommentRateLimitPerMinute: 1, CommentRateLimitBurst: 0},
	} {
		router := gin.New()
		router.POST("/comments", commentRateLimitMiddleware(cfg), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		for i := 0; i < 3; i++ {
			request := httptest.NewRequest(http.MethodPost, "/comments", nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("config %#v request %d status = %d, want 204", cfg, i, response.Code)
			}
		}
	}
}

func TestCommentRateLimitMiddlewareCanonicalizesEquivalentIPv6(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := router.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}
	router.POST("/comments", commentRateLimitMiddleware(config.Config{
		AppEnv:                    "test",
		CommentRateLimitPerMinute: 1,
		CommentRateLimitBurst:     1,
	}), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for index, clientIP := range []string{"2001:0db8:0000:0000:0000:0000:0000:0001", "2001:db8::1"} {
		request := httptest.NewRequest(http.MethodPost, "/comments", nil)
		request.RemoteAddr = "127.0.0.1:10001"
		request.Header.Set("X-Forwarded-For", clientIP)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		want := http.StatusNoContent
		if index == 1 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("IPv6 form %q status = %d, want %d", clientIP, response.Code, want)
		}
	}
}

func TestCommentRateLimitMiddlewareIgnoresSpoofedForwardingHeaderFromUntrustedPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := router.SetTrustedProxies([]string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}
	router.POST("/comments", commentRateLimitMiddleware(config.Config{
		AppEnv:                    "test",
		CommentRateLimitPerMinute: 1,
		CommentRateLimitBurst:     1,
	}), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for index, spoofedIP := range []string{"203.0.113.10", "203.0.113.11"} {
		request := httptest.NewRequest(http.MethodPost, "/comments", nil)
		request.RemoteAddr = "198.51.100.20:10001"
		request.Header.Set("X-Forwarded-For", spoofedIP)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		want := http.StatusNoContent
		if index == 1 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("spoofed IP %q status = %d, want %d", spoofedIP, response.Code, want)
		}
	}
}

func TestCommentRateLimitMiddlewareRejectsInvalidProductionConfiguration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected invalid production rate-limit configuration to panic")
		}
	}()
	commentRateLimitMiddleware(config.Config{AppEnv: "production", CommentRateLimitPerMinute: 0, CommentRateLimitBurst: 5})
}

func TestAdminLoginRateLimiterEnforcesIPAndEmailDimensions(t *testing.T) {
	newLimiter := func() *adminLoginRateLimiter {
		return newAdminLoginRateLimiter(config.Config{
			AppEnv:                       "test",
			AdminLoginRateLimitPerMinute: 1,
			AdminLoginRateLimitBurst:     1,
		})
	}

	t.Run("same IP with different emails", func(t *testing.T) {
		limiter := newLimiter()
		if !limiter.allow("203.0.113.10", "first@example.com") {
			t.Fatal("first login attempt should be allowed")
		}
		if limiter.allow("203.0.113.10", "second@example.com") {
			t.Fatal("second attempt from the same IP should be limited")
		}
	})

	t.Run("same email from different IPs", func(t *testing.T) {
		limiter := newLimiter()
		if !limiter.allow("203.0.113.10", "admin@example.com") {
			t.Fatal("first login attempt should be allowed")
		}
		if limiter.allow("203.0.113.11", " ADMIN@example.com ") {
			t.Fatal("normalized email should share the same limiter bucket")
		}
	})
}

func TestAdminLoginRateLimiterDisabledInTestAllowsRequests(t *testing.T) {
	limiter := newAdminLoginRateLimiter(config.Config{AppEnv: "test"})
	for i := 0; i < 3; i++ {
		if !limiter.allow("203.0.113.10", "admin@example.com") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
}

func TestAdminLoginRateLimiterRejectsInvalidProductionConfiguration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected invalid production login rate-limit configuration to panic")
		}
	}()
	newAdminLoginRateLimiter(config.Config{AppEnv: "production", AdminLoginRateLimitPerMinute: 0, AdminLoginRateLimitBurst: 5})
}
