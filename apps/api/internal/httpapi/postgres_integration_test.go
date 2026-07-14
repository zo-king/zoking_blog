package httpapi

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var postgresTestSchemaPrefix = regexp.MustCompile(`^[a-z][a-z0-9_]{0,30}$`)

func openHTTPAPIPostgresTestSchema(t *testing.T, schemaPrefix string) *gorm.DB {
	t.Helper()
	if !postgresTestSchemaPrefix.MatchString(schemaPrefix) {
		t.Fatalf("invalid PostgreSQL test schema prefix %q", schemaPrefix)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for PostgreSQL HTTP API integration tests")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		t.Skipf("HTTP API integration tests require PostgreSQL, got scheme %q", parsed.Scheme)
	}
	databaseName := strings.Trim(parsed.Path, "/")
	if !strings.HasSuffix(strings.ToLower(databaseName), "_test") {
		t.Skipf("HTTP API integration tests require a *_test database, got %q", databaseName)
	}

	adminDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	adminDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = adminDB.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		t.Fatalf("ping PostgreSQL test database: %v", err)
	}

	schema := schemaPrefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminDB.ExecContext(context.Background(), `create schema "`+schema+`"`); err != nil {
		t.Fatalf("create PostgreSQL test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), `drop schema if exists "`+schema+`" cascade`)
	})

	scopedURL := *parsed
	query := scopedURL.Query()
	query.Set("search_path", schema)
	scopedURL.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(scopedURL.String()), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve isolated PostgreSQL pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
