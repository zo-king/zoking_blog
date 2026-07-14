package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/zo-king/zoking_blog/apps/api/internal/auth"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func main() {
	cfg := config.Load()
	if err := validateSeedConfig(cfg); err != nil {
		log.Fatalf("invalid seed configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := seed(ctx, db, cfg); err != nil {
		log.Fatalf("seed database: %v", err)
	}
}

func validateSeedConfig(cfg config.Config) error {
	switch strings.ToLower(strings.TrimSpace(cfg.AppEnv)) {
	case "development", "dev", "test":
		return nil
	}
	emailValue := strings.TrimSpace(cfg.SeedAdminEmail)
	email := strings.ToLower(emailValue)
	password := cfg.SeedAdminPassword
	if emailValue != cfg.SeedAdminEmail || email == "" || email == "admin@zoking.local" {
		return fmt.Errorf("SEED_ADMIN_EMAIL must be set to a production administrator address")
	}
	if strings.TrimSpace(password) != password || password == "" {
		return fmt.Errorf("SEED_ADMIN_PASSWORD must not be empty or contain surrounding whitespace")
	}
	lowerPassword := strings.ToLower(password)
	if len([]byte(password)) < 16 || len([]byte(password)) > 72 || password == "ChangeMe123!" || strings.HasPrefix(lowerPassword, "change-me") {
		return fmt.Errorf("SEED_ADMIN_PASSWORD must be replaced with 16 to 72 bytes")
	}
	return nil
}

func seed(ctx context.Context, db *sql.DB, cfg config.Config) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := seedTx(ctx, tx, cfg); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type seedDB interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

type seedRoleGrant struct {
	RoleCode string
	Patterns []string
}

func seedPermissionCodes() []string {
	return []string{
		"content:read_all", "content:manage_all",
		"post:read", "post:create", "post:update", "post:delete", "post:publish",
		"page:read", "page:create", "page:update", "page:delete", "page:publish",
		"taxonomy:read", "taxonomy:manage",
		"achievement:read", "achievement:create", "achievement:update", "achievement:delete", "achievement:publish",
		"media:read", "media:upload", "media:delete",
		"comment:read", "comment:moderate",
		"publish:read", "publish:create", "publish:rollback", "publish:cleanup", "setting:read", "setting:update",
		"user:read", "user:manage", "role:read", "role:manage", "audit:read", "system:read",
		"qa:cleanup",
	}
}

func seedRolePermissionGrants() []seedRoleGrant {
	return []seedRoleGrant{
		{RoleCode: "super_admin", Patterns: []string{"%"}},
		{RoleCode: "admin", Patterns: []string{
			"content:%", "post:%", "page:%", "taxonomy:%", "achievement:%", "media:%", "comment:%", "publish:%", "setting:%", "audit:read", "system:read",
		}},
		{RoleCode: "editor", Patterns: []string{
			"content:%", "post:%", "page:%", "taxonomy:%", "achievement:%", "media:read", "media:upload", "comment:read", "comment:moderate", "publish:read", "publish:create", "setting:read",
		}},
		{RoleCode: "author", Patterns: []string{"post:read", "post:create", "post:update", "page:read", "taxonomy:read", "media:read", "media:upload", "comment:read", "setting:read", "system:read"}},
		{RoleCode: "viewer", Patterns: []string{"content:read_all", "post:read", "page:read", "taxonomy:read", "achievement:read", "media:read", "comment:read", "publish:read", "setting:read", "system:read"}},
	}
}

func seedTx(ctx context.Context, db seedDB, cfg config.Config) error {
	for _, code := range seedPermissionCodes() {
		if _, err := db.ExecContext(ctx, `
			insert into permissions (code, name, resource, action)
			values ($1, $1, split_part($1, ':', 1), split_part($1, ':', 2))
			on conflict (code) do nothing`, code); err != nil {
			return err
		}
	}

	roles := []struct {
		Code string
		Name string
	}{
		{"super_admin", "Super Admin"},
		{"admin", "Admin"},
		{"editor", "Editor"},
		{"author", "Author"},
		{"viewer", "Viewer"},
	}
	for _, role := range roles {
		if _, err := db.ExecContext(ctx, `
			insert into roles (code, name, description, is_system)
			values ($1, $2, $2, true)
			on conflict (code) do nothing`, role.Code, role.Name); err != nil {
			return err
		}
	}

	for _, grant := range seedRolePermissionGrants() {
		if _, err := db.ExecContext(ctx, `
			delete from role_permissions rp
			using roles r
			where rp.role_id = r.id and r.code = $1`, grant.RoleCode); err != nil {
			return err
		}
		for _, pattern := range grant.Patterns {
			if _, err := db.ExecContext(ctx, `
				insert into role_permissions (role_id, permission_id)
				select r.id, p.id from roles r cross join permissions p
				where r.code = $1 and p.code like $2
				on conflict (role_id, permission_id) do nothing`, grant.RoleCode, pattern); err != nil {
				return err
			}
		}
	}

	adminID, err := ensureAdmin(ctx, db, cfg)
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		insert into user_roles (user_id, role_id)
		select $1, id from roles where code = 'super_admin'
		on conflict (user_id, role_id) do nothing`, adminID); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		insert into site_settings (key, value_json, description, is_public)
		values
			('site.title', '"Zoking 博客"', 'Public site title', true),
			('site.base_url', to_jsonb($1::text), 'Public site canonical base URL', true),
			('sidebar.subtitle', '"记录技术、设计与日常思考"', 'Public sidebar subtitle', true),
			('sidebar.emoji', '"✏️"', 'Public sidebar emoji mark', true),
			('comments.enabled', 'true', 'Public comments switch', true),
			('comments.api_base', to_jsonb($2::text), 'Public comments API base URL', true),
			('footer.since', '2026', 'Public footer start year', true),
			('pagination.pager_size', '3', 'Public home/list pager size', true)
		on conflict (key) do nothing`, cfg.SiteBaseURL, cfg.PublicAPIBaseURL); err != nil {
		return err
	}

	return nil
}

func ensureAdmin(ctx context.Context, db seedDB, cfg config.Config) (string, error) {
	email := strings.ToLower(strings.TrimSpace(cfg.SeedAdminEmail))
	var id string
	var status string
	err := db.QueryRowContext(ctx, `select id, status from users where email = $1 and deleted_at is null`, email).Scan(&id, &status)
	if err == nil {
		if status != "active" {
			return "", fmt.Errorf("seed administrator %s is not active", email)
		}
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	hash, err := auth.HashPassword(cfg.SeedAdminPassword)
	if err != nil {
		return "", err
	}

	err = db.QueryRowContext(ctx, `
		insert into users (email, username, password_hash, display_name, status)
		values ($1, $2, $3, $4, 'active')
		on conflict (email) where deleted_at is null do nothing
		returning id`, email, "admin", hash, "Super Admin").Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = db.QueryRowContext(ctx, `select id, status from users where email = $1 and deleted_at is null`, email).Scan(&id, &status)
		if err == nil && status != "active" {
			return "", fmt.Errorf("seed administrator %s is not active", email)
		}
	}
	return id, err
}
