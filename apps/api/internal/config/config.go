package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultDatabaseURL = "postgres://zoking:zoking_dev_password@localhost:15432/zoking_blog?sslmode=disable"
	defaultJWTSecret   = "dev-only-change-me-with-long-random-string"
)

type Config struct {
	AppEnv                         string
	AppPort                        string
	SiteBaseURL                    string
	PublicAPIBaseURL               string
	DatabaseURL                    string
	JWTSecret                      string
	AccessTokenTTL                 time.Duration
	RefreshTokenTTL                time.Duration
	MigrationsDir                  string
	HugoSiteDir                    string
	HugoPublicDir                  string
	HugoBin                        string
	PublishReleaseRoot             string
	PublishCurrentDir              string
	PublishPreviewRoot             string
	PublishPreviewPublicBaseURL    string
	PublishPreviewTTL              time.Duration
	PublishPreviewCleanupInterval  time.Duration
	PublishPreviewCleanupBatchSize int
	MediaStorageDriver             string
	MediaLocalDir                  string
	MediaPublicBaseURL             string
	MediaMaxBytes                  int64
	MediaUploadMaxConcurrency      int
	MediaOrphanGracePeriod         time.Duration
	MediaCleanupBatchSize          int
	PublishWorkerEnabled           bool
	PublishWorkerPollInterval      time.Duration
	PublishJobTimeout              time.Duration
	PublishMaxRetries              int
	PublishReleaseKeepLatest       int
	PublishReleaseKeepDays         int
	SeedAdminEmail                 string
	SeedAdminPassword              string
	DBMaxOpenConns                 int
	DBMaxIdleConns                 int
	DBConnMaxLifetime              time.Duration
	AllowedOrigins                 string
	AdminAllowedOrigins            string
	TrustedProxies                 string
	CommentRateLimitPerMinute      int
	CommentRateLimitBurst          int
	AdminLoginRateLimitPerMinute   int
	AdminLoginRateLimitBurst       int
	RequestMaxBytes                int64
	QAE2ECleanupEnabled            bool
	loadErrors                     []error
}

func Load() Config {
	_ = godotenv.Load("../../.env", ".env")
	repoRoot := repoRoot()
	hugoPublicDir := absPath(env("HUGO_PUBLIC_DIR", filepath.Join(repoRoot, "dist", "site")))

	cfg := Config{
		AppEnv:                         env("APP_ENV", "development"),
		AppPort:                        env("APP_PORT", "18080"),
		SiteBaseURL:                    env("SITE_BASE_URL", "http://localhost:1313/"),
		PublicAPIBaseURL:               env("PUBLIC_API_BASE_URL", "http://localhost:18080"),
		DatabaseURL:                    env("DATABASE_URL", defaultDatabaseURL),
		JWTSecret:                      env("JWT_SECRET", defaultJWTSecret),
		AccessTokenTTL:                 durationEnv("ACCESS_TOKEN_TTL", 30*time.Minute),
		RefreshTokenTTL:                durationEnv("REFRESH_TOKEN_TTL", 720*time.Hour),
		MigrationsDir:                  absPath(env("MIGRATIONS_DIR", filepath.Join(repoRoot, "db", "migrations"))),
		HugoSiteDir:                    absPath(env("HUGO_SITE_DIR", filepath.Join(repoRoot, "apps", "site"))),
		HugoPublicDir:                  hugoPublicDir,
		HugoBin:                        absPath(env("HUGO_BIN", filepath.Join(repoRoot, ".tools", "hugo", "hugo.exe"))),
		PublishReleaseRoot:             absPath(env("PUBLISH_RELEASE_ROOT", filepath.Join(filepath.Dir(hugoPublicDir), "releases"))),
		PublishCurrentDir:              absPath(env("PUBLISH_CURRENT_DIR", hugoPublicDir)),
		PublishPreviewRoot:             absPath(env("PUBLISH_PREVIEW_ROOT", filepath.Join(filepath.Dir(hugoPublicDir), "previews"))),
		PublishPreviewPublicBaseURL:    env("PUBLISH_PREVIEW_PUBLIC_BASE_URL", "/preview-files"),
		PublishPreviewTTL:              durationEnv("PUBLISH_PREVIEW_TTL", 24*time.Hour),
		PublishPreviewCleanupInterval:  durationEnv("PUBLISH_PREVIEW_CLEANUP_INTERVAL", time.Hour),
		PublishPreviewCleanupBatchSize: intEnv("PUBLISH_PREVIEW_CLEANUP_BATCH_SIZE", 100),
		MediaStorageDriver:             env("MEDIA_STORAGE_DRIVER", "local"),
		MediaLocalDir:                  absPath(env("MEDIA_LOCAL_DIR", filepath.Join(repoRoot, "storage", "media"))),
		MediaPublicBaseURL:             env("MEDIA_PUBLIC_BASE_URL", "/media-files"),
		MediaMaxBytes:                  int64Env("MEDIA_MAX_BYTES", 10*1024*1024),
		MediaUploadMaxConcurrency:      intEnv("MEDIA_UPLOAD_MAX_CONCURRENCY", 4),
		MediaOrphanGracePeriod:         durationEnv("MEDIA_ORPHAN_GRACE_PERIOD", 7*24*time.Hour),
		MediaCleanupBatchSize:          intEnv("MEDIA_CLEANUP_BATCH_SIZE", 100),
		PublishWorkerEnabled:           boolEnv("PUBLISH_WORKER_ENABLED", true),
		PublishWorkerPollInterval:      durationEnv("PUBLISH_WORKER_POLL_INTERVAL", 2*time.Second),
		PublishJobTimeout:              durationEnv("PUBLISH_JOB_TIMEOUT", 2*time.Minute),
		PublishMaxRetries:              intEnv("PUBLISH_MAX_RETRIES", 3),
		PublishReleaseKeepLatest:       intEnv("PUBLISH_RELEASE_KEEP_LATEST", 20),
		PublishReleaseKeepDays:         intEnv("PUBLISH_RELEASE_KEEP_DAYS", 30),
		SeedAdminEmail:                 env("SEED_ADMIN_EMAIL", "admin@zoking.local"),
		SeedAdminPassword:              env("SEED_ADMIN_PASSWORD", "ChangeMe123!"),
		DBMaxOpenConns:                 intEnv("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns:                 intEnv("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:              durationEnv("DB_CONN_MAX_LIFETIME", time.Hour),
		AllowedOrigins:                 env("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:1313"),
		AdminAllowedOrigins:            env("ADMIN_ALLOWED_ORIGINS", "http://localhost:5173"),
		TrustedProxies:                 env("TRUSTED_PROXIES", "127.0.0.1,::1"),
		CommentRateLimitPerMinute:      intEnv("COMMENT_RATE_LIMIT_PER_MINUTE", 10),
		CommentRateLimitBurst:          intEnv("COMMENT_RATE_LIMIT_BURST", 5),
		AdminLoginRateLimitPerMinute:   intEnv("ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE", 10),
		AdminLoginRateLimitBurst:       intEnv("ADMIN_LOGIN_RATE_LIMIT_BURST", 5),
		RequestMaxBytes:                int64Env("REQUEST_MAX_BYTES", 16*1024*1024),
		QAE2ECleanupEnabled:            boolEnv("QA_E2E_CLEANUP_ENABLED", false),
	}
	if productionLikeEnvironment(cfg.AppEnv) {
		cfg.loadErrors = productionEnvironmentParseErrors()
	}
	return cfg
}

func (cfg Config) ValidateRuntime() error {
	if !productionLikeEnvironment(cfg.AppEnv) {
		return nil
	}

	errs := append([]error(nil), cfg.loadErrors...)
	secret := strings.TrimSpace(cfg.JWTSecret)
	if secret == "" || secret != cfg.JWTSecret || len(secret) < 32 || obviousPlaceholder(secret) {
		errs = append(errs, errors.New("JWT_SECRET must be a non-placeholder secret of at least 32 characters"))
	}

	databaseURL := strings.TrimSpace(cfg.DatabaseURL)
	parsedDatabaseURL, err := url.Parse(databaseURL)
	if err != nil || databaseURL == "" || databaseURL == defaultDatabaseURL ||
		(parsedDatabaseURL.Scheme != "postgres" && parsedDatabaseURL.Scheme != "postgresql") || parsedDatabaseURL.Host == "" {
		errs = append(errs, errors.New("DATABASE_URL must be an explicit PostgreSQL connection URL"))
	}
	if port, err := strconv.Atoi(strings.TrimSpace(cfg.AppPort)); err != nil || port < 1 || port > 65535 {
		errs = append(errs, errors.New("APP_PORT must be between 1 and 65535"))
	}
	if cfg.AccessTokenTTL <= 0 || cfg.RefreshTokenTTL <= 0 {
		errs = append(errs, errors.New("token TTL values must be positive"))
	}
	if cfg.DBMaxOpenConns <= 0 || cfg.DBMaxIdleConns < 0 || cfg.DBMaxIdleConns > cfg.DBMaxOpenConns || cfg.DBConnMaxLifetime <= 0 {
		errs = append(errs, errors.New("database pool configuration is invalid"))
	}
	if cfg.CommentRateLimitPerMinute <= 0 || cfg.CommentRateLimitBurst <= 0 {
		errs = append(errs, errors.New("comment rate-limit values must be positive"))
	}
	if cfg.AdminLoginRateLimitPerMinute <= 0 || cfg.AdminLoginRateLimitBurst <= 0 {
		errs = append(errs, errors.New("admin login rate-limit values must be positive"))
	}
	if cfg.PublishWorkerPollInterval <= 0 || cfg.PublishJobTimeout <= 0 || cfg.PublishMaxRetries < 0 {
		errs = append(errs, errors.New("publish worker configuration is invalid"))
	}
	if cfg.RequestMaxBytes <= 0 || cfg.RequestMaxBytes > 128*1024*1024 {
		errs = append(errs, errors.New("REQUEST_MAX_BYTES must be between 1 byte and 128 MiB"))
	}
	if cfg.MediaMaxBytes <= 0 || cfg.MediaMaxBytes > cfg.RequestMaxBytes {
		errs = append(errs, errors.New("MEDIA_MAX_BYTES must be positive and no greater than REQUEST_MAX_BYTES"))
	}
	if cfg.MediaUploadMaxConcurrency <= 0 || cfg.MediaUploadMaxConcurrency > 128 {
		errs = append(errs, errors.New("MEDIA_UPLOAD_MAX_CONCURRENCY must be between 1 and 128"))
	}
	publicOrigins, publicOriginErr := validateProductionOrigins("CORS_ALLOWED_ORIGINS", cfg.AllowedOrigins)
	if publicOriginErr != nil {
		errs = append(errs, publicOriginErr)
	}
	adminOrigins, adminOriginErr := validateProductionOrigins("ADMIN_ALLOWED_ORIGINS", cfg.AdminAllowedOrigins)
	if adminOriginErr != nil {
		errs = append(errs, adminOriginErr)
	}
	siteOrigin, siteOriginErr := productionSiteOrigin(cfg.SiteBaseURL)
	if siteOriginErr != nil {
		errs = append(errs, siteOriginErr)
	} else {
		if !publicOrigins[siteOrigin] {
			errs = append(errs, errors.New("CORS_ALLOWED_ORIGINS must include the SITE_BASE_URL origin"))
		}
		if adminOrigins[siteOrigin] {
			errs = append(errs, errors.New("ADMIN_ALLOWED_ORIGINS must not include the public SITE_BASE_URL origin"))
		}
	}
	apiOrigin, apiOriginErr := productionRootOrigin("PUBLIC_API_BASE_URL", cfg.PublicAPIBaseURL)
	if apiOriginErr != nil {
		errs = append(errs, apiOriginErr)
	}
	previewOrigin, previewOriginErr := productionPreviewOrigin(cfg.PublishPreviewPublicBaseURL)
	if previewOriginErr != nil {
		errs = append(errs, previewOriginErr)
	} else {
		if apiOriginErr == nil && previewOrigin == apiOrigin {
			errs = append(errs, errors.New("PUBLISH_PREVIEW_PUBLIC_BASE_URL must use an origin separate from PUBLIC_API_BASE_URL"))
		}
		if siteOriginErr == nil && previewOrigin == siteOrigin {
			errs = append(errs, errors.New("PUBLISH_PREVIEW_PUBLIC_BASE_URL must use an origin separate from SITE_BASE_URL"))
		}
		if adminOrigins[previewOrigin] {
			errs = append(errs, errors.New("PUBLISH_PREVIEW_PUBLIC_BASE_URL must use an origin separate from ADMIN_ALLOWED_ORIGINS"))
		}
	}

	return errors.Join(errs...)
}

func validateProductionOrigins(name, value string) (map[string]bool, error) {
	origins := map[string]bool{}
	var errs []error
	for _, raw := range strings.Split(value, ",") {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
			parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" ||
			strings.EqualFold(origin, "null") || origin != parsed.Scheme+"://"+parsed.Host {
			errs = append(errs, fmt.Errorf("%s contains invalid production origin %q", name, origin))
			continue
		}
		origins[origin] = true
	}
	if len(origins) == 0 {
		errs = append(errs, fmt.Errorf("%s must contain at least one HTTPS origin", name))
	}
	return origins, errors.Join(errs...)
}

func productionSiteOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("SITE_BASE_URL must be an HTTPS site root in production")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func productionRootOrigin(name, value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must be an HTTPS root URL in production", name)
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func productionPreviewOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path == "" || parsed.Path == "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("PUBLISH_PREVIEW_PUBLIC_BASE_URL must be an absolute HTTPS URL with a path in production")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func productionLikeEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "test":
		return false
	default:
		return true
	}
}

func obviousPlaceholder(value string) bool {
	lower := strings.ToLower(value)
	if value == defaultJWTSecret {
		return true
	}
	for _, marker := range []string{"change-me", "changeme", "replace-me", "dev-only", "example-secret"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func productionEnvironmentParseErrors() []error {
	var errs []error
	for _, key := range []string{
		"COMMENT_RATE_LIMIT_PER_MINUTE",
		"COMMENT_RATE_LIMIT_BURST",
		"ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE",
		"ADMIN_LOGIN_RATE_LIMIT_BURST",
		"PUBLISH_MAX_RETRIES",
		"DB_MAX_OPEN_CONNS",
		"DB_MAX_IDLE_CONNS",
		"MEDIA_UPLOAD_MAX_CONCURRENCY",
	} {
		if value, ok := os.LookupEnv(key); ok {
			if _, err := strconv.Atoi(strings.TrimSpace(value)); err != nil {
				errs = append(errs, fmt.Errorf("%s must be an integer", key))
			}
		}
	}
	for _, key := range []string{
		"ACCESS_TOKEN_TTL",
		"REFRESH_TOKEN_TTL",
		"PUBLISH_WORKER_POLL_INTERVAL",
		"PUBLISH_JOB_TIMEOUT",
		"DB_CONN_MAX_LIFETIME",
	} {
		if value, ok := os.LookupEnv(key); ok {
			if _, err := time.ParseDuration(strings.TrimSpace(value)); err != nil {
				errs = append(errs, fmt.Errorf("%s must be a duration", key))
			}
		}
	}
	if value, ok := os.LookupEnv("REQUEST_MAX_BYTES"); ok {
		if _, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err != nil {
			errs = append(errs, errors.New("REQUEST_MAX_BYTES must be an integer"))
		}
	}
	if value, ok := os.LookupEnv("MEDIA_MAX_BYTES"); ok {
		if _, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err != nil {
			errs = append(errs, errors.New("MEDIA_MAX_BYTES must be an integer"))
		}
	}
	return errs
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	configDir := filepath.Dir(file)
	return filepath.Clean(filepath.Join(configDir, "../../../.."))
}

func absPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64Env(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
