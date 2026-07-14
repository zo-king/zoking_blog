package httpapi

import (
	"crypto/subtle"
	"net/http"
	"runtime/debug"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/auth"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func validRequestID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char > unicode.MaxASCII || char < 0x21 || char == 0x7f {
			return false
		}
	}
	return true
}

func requestBodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			Fail(c, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

func recoverMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = debug.Stack()
				Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}

func corsMiddleware(cfg config.Config) gin.HandlerFunc {
	publicAllowed := originAllowlist(cfg.AllowedOrigins)
	adminAllowed := originAllowlist(cfg.AdminAllowedOrigins)

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
		}
		isAdmin := strings.HasPrefix(c.Request.URL.Path, "/api/v1/admin/")
		allowed := publicAllowed
		if isAdmin {
			allowed = adminAllowed
		}
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			if isAdmin {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-CSRF-Token")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func originAllowlist(value string) map[string]bool {
	allowed := map[string]bool{}
	for _, origin := range strings.Split(value, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	return allowed
}

func adminOriginMiddleware(cfg config.Config) gin.HandlerFunc {
	allowedOrigins := originAllowlist(cfg.AdminAllowedOrigins)
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" && !allowedOrigins[origin] {
			Fail(c, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "untrusted admin origin")
			c.Abort()
			return
		}
		c.Next()
	}
}

func authMiddleware(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenValue := ""
		if strings.HasPrefix(header, "Bearer ") {
			tokenValue = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			c.Set("auth_transport", "bearer")
		} else if cookie, err := c.Cookie(adminAccessCookieName); err == nil {
			tokenValue = strings.TrimSpace(cookie)
			c.Set("auth_transport", "cookie")
		}
		if tokenValue == "" {
			Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
			c.Abort()
			return
		}

		claims, err := auth.ParseAccessToken(cfg.JWTSecret, tokenValue)
		if err != nil {
			Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid bearer token")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Next()
	}
}

func csrfMiddleware(cfg config.Config) gin.HandlerFunc {
	allowedOrigins := originAllowlist(cfg.AdminAllowedOrigins)
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if c.GetString("auth_transport") != "cookie" {
			c.Next()
			return
		}
		if origin := strings.TrimSpace(c.GetHeader("Origin")); origin != "" && !allowedOrigins[origin] {
			Fail(c, http.StatusForbidden, "CSRF_FAILED", "untrusted request origin")
			c.Abort()
			return
		}
		cookieToken, err := c.Cookie(adminCSRFCookieName)
		headerToken := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
		if err != nil || cookieToken == "" || headerToken == "" ||
			subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
			Fail(c, http.StatusForbidden, "CSRF_FAILED", "invalid CSRF token")
			c.Abort()
			return
		}
		c.Next()
	}
}
