package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadReadsPublicURLProxyAndRateLimitEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SITE_BASE_URL", "https://site.example/")
	t.Setenv("PUBLIC_API_BASE_URL", "https://api.example")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 2001:db8::1")
	t.Setenv("COMMENT_RATE_LIMIT_PER_MINUTE", "37")
	t.Setenv("COMMENT_RATE_LIMIT_BURST", "9")
	t.Setenv("ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE", "11")
	t.Setenv("ADMIN_LOGIN_RATE_LIMIT_BURST", "4")
	t.Setenv("MEDIA_PUBLIC_BASE_URL", "https://cdn.example/media")
	t.Setenv("MEDIA_UPLOAD_MAX_CONCURRENCY", "7")

	cfg := Load()
	if cfg.AppEnv != "production" {
		t.Fatalf("AppEnv = %q, want production", cfg.AppEnv)
	}
	if cfg.SiteBaseURL != "https://site.example/" || cfg.PublicAPIBaseURL != "https://api.example" {
		t.Fatalf("public URLs = %q, %q", cfg.SiteBaseURL, cfg.PublicAPIBaseURL)
	}
	if cfg.TrustedProxies != "10.0.0.1, 2001:db8::1" {
		t.Fatalf("TrustedProxies = %q", cfg.TrustedProxies)
	}
	if cfg.CommentRateLimitPerMinute != 37 || cfg.CommentRateLimitBurst != 9 {
		t.Fatalf("rate limits = %d/%d", cfg.CommentRateLimitPerMinute, cfg.CommentRateLimitBurst)
	}
	if cfg.AdminLoginRateLimitPerMinute != 11 || cfg.AdminLoginRateLimitBurst != 4 {
		t.Fatalf("admin login rate limits = %d/%d", cfg.AdminLoginRateLimitPerMinute, cfg.AdminLoginRateLimitBurst)
	}
	if cfg.MediaPublicBaseURL != "https://cdn.example/media" {
		t.Fatalf("MediaPublicBaseURL = %q", cfg.MediaPublicBaseURL)
	}
	if cfg.MediaUploadMaxConcurrency != 7 {
		t.Fatalf("MediaUploadMaxConcurrency = %d", cfg.MediaUploadMaxConcurrency)
	}
}

func TestValidateRuntimeAllowsDevelopmentDefaults(t *testing.T) {
	if err := (Config{AppEnv: "development"}).ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func TestValidateRuntimeRejectsUnsafeProductionJWTSecrets(t *testing.T) {
	for _, secret := range []string{"", defaultJWTSecret, "change-me-very-long-random-secret"} {
		cfg := validProductionRuntimeConfig()
		cfg.JWTSecret = secret
		if err := cfg.ValidateRuntime(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
			t.Fatalf("JWTSecret %q error = %v, want JWT_SECRET validation error", secret, err)
		}
	}
}

func TestValidateRuntimeAllowsExplicitStagingConfiguration(t *testing.T) {
	cfg := validProductionRuntimeConfig()
	cfg.AppEnv = "staging"
	if err := cfg.ValidateRuntime(); err != nil {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func TestValidateRuntimeRejectsUnsafeProductionOrigins(t *testing.T) {
	base := validProductionRuntimeConfig()
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"empty public origins", func(cfg *Config) { cfg.AllowedOrigins = "" }},
		{"http admin origin", func(cfg *Config) { cfg.AdminAllowedOrigins = "http://admin.zoking.tech" }},
		{"origin with path", func(cfg *Config) { cfg.AdminAllowedOrigins = "https://admin.zoking.tech/login" }},
		{"origin with trailing slash", func(cfg *Config) { cfg.AllowedOrigins = "https://zoking.tech/" }},
		{"public site trusted as admin", func(cfg *Config) { cfg.AdminAllowedOrigins = "https://admin.zoking.tech,https://zoking.tech" }},
		{"preview shares API origin", func(cfg *Config) { cfg.PublishPreviewPublicBaseURL = "https://api.zoking.tech/preview-files" }},
		{"preview shares site origin", func(cfg *Config) { cfg.PublishPreviewPublicBaseURL = "https://zoking.tech/preview-files" }},
		{"preview shares admin origin", func(cfg *Config) { cfg.PublishPreviewPublicBaseURL = "https://admin.zoking.tech/preview-files" }},
		{"relative production preview", func(cfg *Config) { cfg.PublishPreviewPublicBaseURL = "/preview-files" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			test.mutate(&cfg)
			if err := cfg.ValidateRuntime(); err == nil {
				t.Fatal("ValidateRuntime() accepted unsafe production origin configuration")
			}
		})
	}
}

func TestLoadPreservesMalformedProductionEnvironmentAsValidationError(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE", "not-a-number")
	cfg := Load()
	err := cfg.ValidateRuntime()
	if err == nil || !strings.Contains(err.Error(), "ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE must be an integer") {
		t.Fatalf("ValidateRuntime() error = %v", err)
	}
}

func validProductionRuntimeConfig() Config {
	return Config{
		AppEnv:                       "production",
		AppPort:                      "18080",
		DatabaseURL:                  "postgres://blog:secret@postgres:5432/blog?sslmode=require",
		JWTSecret:                    "0123456789abcdef0123456789abcdef",
		AccessTokenTTL:               30 * time.Minute,
		RefreshTokenTTL:              24 * time.Hour,
		PublishWorkerPollInterval:    time.Second,
		PublishJobTimeout:            time.Minute,
		PublishMaxRetries:            3,
		DBMaxOpenConns:               20,
		DBMaxIdleConns:               5,
		DBConnMaxLifetime:            time.Hour,
		CommentRateLimitPerMinute:    10,
		CommentRateLimitBurst:        5,
		AdminLoginRateLimitPerMinute: 10,
		AdminLoginRateLimitBurst:     5,
		RequestMaxBytes:              16 * 1024 * 1024,
		MediaMaxBytes:                10 * 1024 * 1024,
		MediaUploadMaxConcurrency:    4,
		SiteBaseURL:                  "https://zoking.tech/",
		AllowedOrigins:               "https://zoking.tech",
		AdminAllowedOrigins:          "https://admin.zoking.tech",
		PublicAPIBaseURL:             "https://api.zoking.tech",
		PublishPreviewPublicBaseURL:  "https://preview.zoking.tech/preview-files",
	}
}

func TestLoadInvalidRateLimitEnvironmentUsesDefaults(t *testing.T) {
	t.Setenv("COMMENT_RATE_LIMIT_PER_MINUTE", "not-a-number")
	t.Setenv("COMMENT_RATE_LIMIT_BURST", "")

	cfg := Load()
	if cfg.CommentRateLimitPerMinute != 10 || cfg.CommentRateLimitBurst != 5 {
		t.Fatalf("rate limits = %d/%d, want defaults 10/5", cfg.CommentRateLimitPerMinute, cfg.CommentRateLimitBurst)
	}
	if cfg.PublishPreviewTTL <= 0 || cfg.DBConnMaxLifetime != time.Hour {
		t.Fatalf("unexpected unrelated defaults: preview TTL=%s db lifetime=%s", cfg.PublishPreviewTTL, cfg.DBConnMaxLifetime)
	}
}
