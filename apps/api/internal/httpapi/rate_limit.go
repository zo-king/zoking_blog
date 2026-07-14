package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"golang.org/x/time/rate"
)

type rateLimitClient struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type keyedRateLimiter struct {
	mu          sync.Mutex
	clients     map[string]*rateLimitClient
	lastCleanup time.Time
	requestRate rate.Limit
	burst       int
}

func newKeyedRateLimiter(perMinute, burst int) *keyedRateLimiter {
	return &keyedRateLimiter{
		clients:     map[string]*rateLimitClient{},
		lastCleanup: time.Now(),
		requestRate: rate.Every(time.Minute / time.Duration(perMinute)),
		burst:       burst,
	}
}

func (l *keyedRateLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= 10*time.Minute {
		for clientKey, client := range l.clients {
			if now.Sub(client.lastSeen) >= 30*time.Minute {
				delete(l.clients, clientKey)
			}
		}
		l.lastCleanup = now
	}
	client := l.clients[key]
	if client == nil {
		client = &rateLimitClient{limiter: rate.NewLimiter(l.requestRate, l.burst)}
		l.clients[key] = client
	}
	client.lastSeen = now
	return client.limiter.Allow()
}

type adminLoginRateLimiter struct {
	byIP    *keyedRateLimiter
	byEmail *keyedRateLimiter
}

func newAdminLoginRateLimiter(cfg config.Config) *adminLoginRateLimiter {
	perMinute := cfg.AdminLoginRateLimitPerMinute
	burst := cfg.AdminLoginRateLimitBurst
	if perMinute <= 0 || burst <= 0 {
		if productionLikeEnvironment(cfg.AppEnv) {
			panic("ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE and ADMIN_LOGIN_RATE_LIMIT_BURST must be positive outside development and test")
		}
		return nil
	}
	return &adminLoginRateLimiter{
		byIP:    newKeyedRateLimiter(perMinute, burst),
		byEmail: newKeyedRateLimiter(perMinute, burst),
	}
}

func (l *adminLoginRateLimiter) allow(clientIP, email string) bool {
	if l == nil {
		return true
	}
	ipAllowed := l.byIP.allow(canonicalClientIP(clientIP))
	emailAllowed := l.byEmail.allow(strings.ToLower(strings.TrimSpace(email)))
	return ipAllowed && emailAllowed
}

func commentRateLimitMiddleware(cfg config.Config) gin.HandlerFunc {
	perMinute := cfg.CommentRateLimitPerMinute
	burst := cfg.CommentRateLimitBurst
	if perMinute <= 0 || burst <= 0 {
		if productionLikeEnvironment(cfg.AppEnv) {
			panic("COMMENT_RATE_LIMIT_PER_MINUTE and COMMENT_RATE_LIMIT_BURST must be positive outside development and test")
		}
		return func(c *gin.Context) { c.Next() }
	}

	limiter := newKeyedRateLimiter(perMinute, burst)

	return func(c *gin.Context) {
		clientIP := canonicalClientIP(c.ClientIP())
		if !limiter.allow(clientIP) {
			c.Header("Retry-After", "60")
			Fail(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many comment submissions")
			c.Abort()
			return
		}
		c.Next()
	}
}

func canonicalClientIP(value string) string {
	trimmed := strings.TrimSpace(value)
	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}
	return strings.ToLower(trimmed)
}

func productionLikeEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "test":
		return false
	default:
		return true
	}
}
