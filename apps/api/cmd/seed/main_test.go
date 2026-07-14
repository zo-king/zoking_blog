package main

import (
	"strings"
	"testing"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func TestSeedContentAccessPermissionsAndRoleGrants(t *testing.T) {
	permissions := make(map[string]bool)
	for _, permission := range seedPermissionCodes() {
		permissions[permission] = true
	}
	for _, permission := range []string{"content:read_all", "content:manage_all"} {
		if !permissions[permission] {
			t.Fatalf("seed permission %q is missing", permission)
		}
	}

	grants := seedRolePermissionGrants()
	tests := []struct {
		role       string
		permission string
		want       bool
	}{
		{role: "admin", permission: "content:read_all", want: true},
		{role: "admin", permission: "content:manage_all", want: true},
		{role: "editor", permission: "content:read_all", want: true},
		{role: "editor", permission: "content:manage_all", want: true},
		{role: "editor", permission: "achievement:publish", want: true},
		{role: "viewer", permission: "content:read_all", want: true},
		{role: "viewer", permission: "achievement:read", want: true},
		{role: "viewer", permission: "achievement:update"},
		{role: "viewer", permission: "content:manage_all"},
		{role: "author", permission: "content:read_all"},
		{role: "author", permission: "content:manage_all"},
		{role: "author", permission: "publish:read"},
		{role: "author", permission: "achievement:read"},
		{role: "super_admin", permission: "content:read_all", want: true},
		{role: "super_admin", permission: "role:manage", want: true},
	}
	for _, test := range tests {
		if got := seededRoleHasPermission(grants, test.role, test.permission); got != test.want {
			t.Fatalf("role %s permission %s = %v, want %v", test.role, test.permission, got, test.want)
		}
	}
}

func seededRoleHasPermission(grants []seedRoleGrant, role, permission string) bool {
	for _, grant := range grants {
		if grant.RoleCode != role {
			continue
		}
		for _, pattern := range grant.Patterns {
			if pattern == permission {
				return true
			}
			if strings.HasSuffix(pattern, "%") && strings.HasPrefix(permission, strings.TrimSuffix(pattern, "%")) {
				return true
			}
		}
	}
	return false
}

func TestValidateSeedConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{name: "development defaults", cfg: config.Config{AppEnv: "development", SeedAdminEmail: "admin@zoking.local", SeedAdminPassword: "ChangeMe123!"}},
		{name: "test defaults", cfg: config.Config{AppEnv: "test", SeedAdminEmail: "admin@zoking.local", SeedAdminPassword: "ChangeMe123!"}},
		{name: "production values", cfg: config.Config{AppEnv: "production", SeedAdminEmail: "owner@zoking.tech", SeedAdminPassword: "a-strong-production-password"}},
		{name: "production default email", cfg: config.Config{AppEnv: "production", SeedAdminEmail: "admin@zoking.local", SeedAdminPassword: "a-strong-production-password"}, wantErr: true},
		{name: "production placeholder password", cfg: config.Config{AppEnv: "production", SeedAdminEmail: "owner@zoking.tech", SeedAdminPassword: "change-me-admin-password"}, wantErr: true},
		{name: "production short password", cfg: config.Config{AppEnv: "production", SeedAdminEmail: "owner@zoking.tech", SeedAdminPassword: "short"}, wantErr: true},
		{name: "staging fails closed", cfg: config.Config{AppEnv: "staging", SeedAdminEmail: "admin@zoking.local", SeedAdminPassword: "ChangeMe123!"}, wantErr: true},
		{name: "prod fails closed", cfg: config.Config{AppEnv: "prod", SeedAdminEmail: "admin@zoking.local", SeedAdminPassword: "ChangeMe123!"}, wantErr: true},
		{name: "production whitespace password", cfg: config.Config{AppEnv: "production", SeedAdminEmail: "owner@zoking.tech", SeedAdminPassword: "                "}, wantErr: true},
		{name: "production surrounding whitespace password", cfg: config.Config{AppEnv: "production", SeedAdminEmail: "owner@zoking.tech", SeedAdminPassword: " strong-production-password "}, wantErr: true},
		{name: "production bcrypt oversize password", cfg: config.Config{AppEnv: "production", SeedAdminEmail: "owner@zoking.tech", SeedAdminPassword: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSeedConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSeedConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
