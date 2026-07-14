package httpapi

import "testing"

func TestHashAuditIPUsesKeyedHMAC(t *testing.T) {
	first := hashAuditIP("127.0.0.1", "secret-a")
	second := hashAuditIP("127.0.0.1", "secret-a")
	otherKey := hashAuditIP("127.0.0.1", "secret-b")
	if first == "" || first != second {
		t.Fatal("expected stable non-empty HMAC")
	}
	if first == otherKey {
		t.Fatal("expected different secrets to produce different hashes")
	}
}

func TestAuditAction(t *testing.T) {
	tests := map[string]string{
		auditAction("POST", "/api/v1/admin/posts", "posts"):                          "posts.create",
		auditAction("PATCH", "/api/v1/admin/posts/:id", "posts"):                     "posts.update",
		auditAction("POST", "/api/v1/admin/posts/:id/publish", "posts"):              "posts.publish",
		auditAction("POST", "/api/v1/admin/publish/releases/:id/promote", "publish"): "publish.promote",
	}
	for actual, expected := range tests {
		if actual != expected {
			t.Fatalf("expected %s, got %s", expected, actual)
		}
	}
}
