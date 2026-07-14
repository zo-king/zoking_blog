package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCurrentContentUserIDFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name   string
		userID string
	}{
		{name: "missing"},
		{name: "malformed", userID: "not-a-uuid"},
		{name: "nil uuid", userID: uuid.Nil.String()},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, recorder := newContentAccessTestContext()
			if test.userID != "" {
				c.Set("user_id", test.userID)
			}

			if userID, ok := currentContentUserID(c); ok || userID != uuid.Nil {
				t.Fatalf("currentContentUserID() = %s, %v; want nil UUID and false", userID, ok)
			}
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
			if !c.IsAborted() {
				t.Fatal("context was not aborted")
			}
		})
	}
}

func TestHasGlobalContentAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		permissions []string
		roles       []string
		mode        contentAccessMode
		want        bool
	}{
		{name: "read permission", permissions: []string{"content:read_all"}, mode: contentAccessRead, want: true},
		{name: "read permission cannot manage", permissions: []string{"content:read_all"}, mode: contentAccessManage},
		{name: "manage permission can read", permissions: []string{"content:manage_all"}, mode: contentAccessRead, want: true},
		{name: "manage permission can manage", permissions: []string{"content:manage_all"}, mode: contentAccessManage, want: true},
		{name: "wildcard can read", permissions: []string{"*"}, mode: contentAccessRead, want: true},
		{name: "wildcard can manage", permissions: []string{"*"}, mode: contentAccessManage, want: true},
		{name: "legacy super admin read", roles: []string{"super_admin"}, mode: contentAccessRead, want: true},
		{name: "legacy super admin manage", roles: []string{"super_admin"}, mode: contentAccessManage, want: true},
		{name: "legacy admin read", roles: []string{"admin"}, mode: contentAccessRead, want: true},
		{name: "legacy admin manage", roles: []string{"admin"}, mode: contentAccessManage, want: true},
		{name: "legacy editor read", roles: []string{"editor"}, mode: contentAccessRead, want: true},
		{name: "legacy editor manage", roles: []string{"editor"}, mode: contentAccessManage, want: true},
		{name: "legacy viewer read", roles: []string{"viewer"}, mode: contentAccessRead, want: true},
		{name: "legacy viewer cannot manage", roles: []string{"viewer"}, mode: contentAccessManage},
		{name: "author remains owner scoped", roles: []string{"author"}, mode: contentAccessRead},
		{name: "invalid mode", permissions: []string{"*"}, roles: []string{"super_admin"}, mode: contentAccessMode(99)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := newContentAccessTestContext()
			c.Set("permissions", test.permissions)
			c.Set("roles", test.roles)
			if got := hasGlobalContentAccess(c, test.mode); got != test.want {
				t.Fatalf("hasGlobalContentAccess() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestScopeContentQueryUsesOwnerFilter(t *testing.T) {
	db := newContentAccessDryRunDB(t)
	ownerID := uuid.New()

	for _, table := range []string{"posts", "pages"} {
		t.Run(table, func(t *testing.T) {
			c, _ := newContentAccessTestContext()
			setContentAccessIdentity(c, ownerID, []string{"author"}, nil)

			scoped, ok := scopeContentQuery(c, db.Table(table), table, contentAccessRead)
			if !ok {
				t.Fatal("scopeContentQuery() unexpectedly failed")
			}
			tx := scoped.Find(&[]map[string]interface{}{})
			assertSQLContains(t, tx, table+".author_id = $1")
			assertUUIDVars(t, tx, ownerID)
		})
	}
}

func TestScopePublishQueryUsesRelatedContentOwnerFilter(t *testing.T) {
	db := newContentAccessDryRunDB(t)
	ownerID := uuid.New()

	for _, table := range []string{"publish_jobs", "publish_releases", "publish_previews"} {
		t.Run(table, func(t *testing.T) {
			c, _ := newContentAccessTestContext()
			setContentAccessIdentity(c, ownerID, []string{"author"}, nil)

			scoped, ok := scopePublishQuery(c, db.Table(table), table, contentAccessRead)
			if !ok {
				t.Fatal("scopePublishQuery() unexpectedly failed")
			}
			tx := scoped.Find(&[]map[string]interface{}{})
			assertSQLContains(t, tx, "EXISTS (SELECT 1 FROM posts WHERE posts.id = "+table+".post_id AND posts.author_id = $1)")
			assertSQLContains(t, tx, "EXISTS (SELECT 1 FROM pages WHERE pages.id = "+table+".page_id AND pages.author_id = $2)")
			assertUUIDVars(t, tx, ownerID, ownerID)
		})
	}
}

func TestContentScopeLeavesGlobalQueriesUnfiltered(t *testing.T) {
	db := newContentAccessDryRunDB(t)
	c, _ := newContentAccessTestContext()
	setContentAccessIdentity(c, uuid.New(), []string{"author"}, []string{"content:read_all"})

	scoped, ok := scopeContentQuery(c, db.Table("posts"), "posts", contentAccessRead)
	if !ok {
		t.Fatal("scopeContentQuery() unexpectedly failed")
	}
	tx := scoped.Find(&[]map[string]interface{}{})
	if strings.Contains(tx.Statement.SQL.String(), "author_id") {
		t.Fatalf("global query was owner filtered: %s", tx.Statement.SQL.String())
	}
}

func TestContentScopesRejectInvalidIdentityWithoutUnscopedQuery(t *testing.T) {
	db := newContentAccessDryRunDB(t)
	c, recorder := newContentAccessTestContext()
	c.Set("user_id", "invalid")
	c.Set("permissions", []string{"*"})

	scoped, ok := scopeContentQuery(c, db.Table("posts"), "posts", contentAccessRead)
	if ok {
		t.Fatal("scopeContentQuery() accepted an invalid identity")
	}
	if recorder.Code != http.StatusUnauthorized || !c.IsAborted() {
		t.Fatalf("invalid identity response = %d, aborted=%v", recorder.Code, c.IsAborted())
	}
	tx := scoped.Find(&[]map[string]interface{}{})
	assertSQLContains(t, tx, "1 = 0")
}

func TestContentScopesRejectUnknownTables(t *testing.T) {
	db := newContentAccessDryRunDB(t)
	ownerID := uuid.New()

	for _, test := range []struct {
		name  string
		scope func(*gin.Context, *gorm.DB, string, contentAccessMode) (*gorm.DB, bool)
	}{
		{name: "content", scope: scopeContentQuery},
		{name: "publish", scope: scopePublishQuery},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, recorder := newContentAccessTestContext()
			setContentAccessIdentity(c, ownerID, []string{"super_admin"}, []string{"*"})

			scoped, ok := test.scope(c, db.Table("posts"), "users", contentAccessRead)
			if ok {
				t.Fatal("scope accepted an unknown table")
			}
			if recorder.Code != http.StatusInternalServerError || !c.IsAborted() {
				t.Fatalf("unknown table response = %d, aborted=%v", recorder.Code, c.IsAborted())
			}
			tx := scoped.Find(&[]map[string]interface{}{})
			assertSQLContains(t, tx, "1 = 0")
		})
	}
}

func newContentAccessTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, recorder
}

func setContentAccessIdentity(c *gin.Context, userID uuid.UUID, roles, permissions []string) {
	c.Set("user_id", userID.String())
	c.Set("roles", roles)
	c.Set("permissions", permissions)
}

func newContentAccessDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open("postgres://test:test@localhost/test"), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	return db
}

func assertSQLContains(t *testing.T, tx *gorm.DB, fragment string) {
	t.Helper()
	if tx.Error != nil {
		t.Fatalf("build query: %v", tx.Error)
	}
	sql := tx.Statement.SQL.String()
	if !strings.Contains(sql, fragment) {
		t.Fatalf("SQL %q does not contain %q", sql, fragment)
	}
}

func assertUUIDVars(t *testing.T, tx *gorm.DB, want ...uuid.UUID) {
	t.Helper()
	if len(tx.Statement.Vars) != len(want) {
		t.Fatalf("query vars = %#v, want %d UUIDs", tx.Statement.Vars, len(want))
	}
	for i, expected := range want {
		got, ok := tx.Statement.Vars[i].(uuid.UUID)
		if !ok || got != expected {
			t.Fatalf("query var %d = %#v, want %s", i, tx.Statement.Vars[i], expected)
		}
	}
}
