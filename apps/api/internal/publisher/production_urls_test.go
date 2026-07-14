package publisher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReleasePublicURLs(t *testing.T) {
	valid := defaultSiteSettings()
	valid.Site.BaseURL = "https://zoking.tech/"
	valid.Comments.APIBase = "https://api.zoking.tech"
	expectedSite := "https://zoking.tech/"
	expectedAPI := "https://api.zoking.tech"

	tests := []struct {
		name         string
		appEnv       string
		settings     SiteSettingsSnapshot
		mediaBase    string
		expectedSite string
		expectedAPI  string
		wantError    string
	}{
		{name: "production domains", appEnv: "production", settings: valid, expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files"},
		{name: "absolute media CDN", appEnv: "production", settings: valid, expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "https://cdn.zoking.tech/media"},
		{name: "development permits localhost", appEnv: "development", settings: defaultSiteSettings(), mediaBase: "http://localhost:18080/media-files"},
		{name: "production alias fails closed", appEnv: "prod", settings: withSiteURL(valid, "http://zoking.tech/"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "must use HTTPS"},
		{name: "reject missing deployment URL", appEnv: "production", settings: valid, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "SITE_BASE_URL"},
		{name: "reject deployment mismatch", appEnv: "production", settings: valid, expectedSite: "https://other.com/", expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "match SITE_BASE_URL"},
		{name: "reject HTTP site", appEnv: "production", settings: withSiteURL(valid, "http://zoking.tech/"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "must use HTTPS"},
		{name: "reject localhost site", appEnv: "production", settings: withSiteURL(valid, "https://localhost/"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "localhost"},
		{name: "reject IPv4 loopback", appEnv: "production", settings: withSiteURL(valid, "https://127.0.0.1/"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "loopback"},
		{name: "reject IPv6 loopback", appEnv: "production", settings: withSiteURL(valid, "https://[::1]/"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "loopback"},
		{name: "reject user info", appEnv: "production", settings: withSiteURL(valid, "https://user@zoking.tech/"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "user information"},
		{name: "reject site subpath", appEnv: "production", settings: withSiteURL(valid, "https://zoking.tech/blog/"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "origin root"},
		{name: "reject malformed URL", appEnv: "production", settings: withSiteURL(valid, "://bad"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "absolute URL"},
		{name: "reject HTTP comments API", appEnv: "production", settings: withCommentsURL(valid, "http://api.zoking.tech"), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "comments.api_base"},
		{name: "reject missing comments API", appEnv: "production", settings: withCommentsURL(valid, "  "), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "URL is required"},
		{name: "disabled comments ignore API URL", appEnv: "production", settings: withoutComments(withCommentsURL(valid, "http://localhost:18080")), expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media-files"},
		{name: "reject protocol relative media", appEnv: "production", settings: valid, expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "//localhost/media-files", wantError: "absolute URL"},
		{name: "reject loopback media", appEnv: "production", settings: valid, expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "https://127.0.0.1/media-files", wantError: "loopback"},
		{name: "reject private API", appEnv: "production", settings: withCommentsURL(valid, "https://10.0.0.1"), expectedSite: expectedSite, expectedAPI: "https://10.0.0.1", mediaBase: "/media-files", wantError: "public hostname"},
		{name: "reject single-label internal API", appEnv: "production", settings: withCommentsURL(valid, "https://postgres"), expectedSite: expectedSite, expectedAPI: "https://postgres", mediaBase: "/media-files", wantError: "public hostname"},
		{name: "reject mDNS API", appEnv: "production", settings: withCommentsURL(valid, "https://service.local"), expectedSite: expectedSite, expectedAPI: "https://service.local", mediaBase: "/media-files", wantError: "public hostname"},
		{name: "reject shortened loopback", appEnv: "production", settings: withSiteURL(valid, "https://127.1/"), expectedSite: "https://127.1/", expectedAPI: expectedAPI, mediaBase: "/media-files", wantError: "public hostname"},
		{name: "reject root media path", appEnv: "production", settings: valid, expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/", wantError: "clean root-relative"},
		{name: "reject media dot segment", appEnv: "production", settings: valid, expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media/../files", wantError: "clean root-relative"},
		{name: "reject encoded media dot segment", appEnv: "production", settings: valid, expectedSite: expectedSite, expectedAPI: expectedAPI, mediaBase: "/media/%2e%2e/files", wantError: "clean root-relative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReleasePublicURLs(tt.appEnv, tt.settings, tt.expectedSite, tt.expectedAPI, tt.mediaBase)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestValidateReleaseOutputPublicURLs(t *testing.T) {
	tests := []struct {
		name      string
		appEnv    string
		content   string
		wantError string
	}{
		{name: "valid production output", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><section data-public-comments data-api-base="https://api.zoking.tech"></section>`},
		{name: "development output is exempt", appEnv: "development", content: `<link href="http://localhost:1313/">`},
		{name: "reject localhost", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><a href="http://localhost:18080">bad</a>`, wantError: "localhost"},
		{name: "reject IPv4 loopback range", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><a href="http://127.0.0.2">bad</a>`, wantError: "127.0.0.2"},
		{name: "reject expanded IPv6 loopback", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><a href="http://[0:0:0:0:0:0:0:1]">bad</a>`, wantError: "0:0:0:0:0:0:0:1"},
		{name: "allow localhost prose and code", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><p>本地开发可以使用 localhost。</p><code>curl http://localhost:18080/healthz</code>`},
		{name: "reject actionable private URL", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><a href="http://192.168.1.2/admin">bad</a>`, wantError: "non-public URL"},
		{name: "require site domain", appEnv: "production", content: `<link rel="canonical" href="https://other.example/">`, wantError: "SITE_BASE_URL"},
		{name: "reject wrong OG domain", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><meta property="og:url" content="https://other.example/">`, wantError: "og:url"},
		{name: "require exact API domain", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><section data-public-comments data-api-base="https://api.zoking.tech.evil"></section>`, wantError: "PUBLIC_API_BASE_URL"},
		{name: "reject API userinfo", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><section data-public-comments data-api-base="https://user@api.zoking.tech"></section>`, wantError: "PUBLIC_API_BASE_URL"},
		{name: "reject API query", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><section data-public-comments data-api-base="https://api.zoking.tech?token=x"></section>`, wantError: "PUBLIC_API_BASE_URL"},
		{name: "reject canonical userinfo", appEnv: "production", content: `<link rel="canonical" href="https://user@zoking.tech/">`, wantError: "SITE_BASE_URL"},
		{name: "reject clickable private src", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><img src="https://10.0.0.2/media.jpg">`, wantError: "non-public URL"},
		{name: "reject clickable localhost action", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><form action="http://localhost:18080/comments"></form>`, wantError: "localhost"},
		{name: "reject localhost srcset", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><img srcset="https://zoking.tech/a.png 1x, http://localhost:1313/a.png 2x">`, wantError: "srcset"},
		{name: "reject localhost formaction", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><button formaction="http://localhost:18080/admin">bad</button>`, wantError: "formaction"},
		{name: "reject localhost object data", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><object data="http://127.0.0.1/private"></object>`, wantError: "data"},
		{name: "reject localhost meta refresh", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><meta http-equiv="refresh" content="0; URL=http://localhost:18080/admin">`, wantError: "meta refresh"},
		{name: "reject localhost inline CSS", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><div style="background:url(http://localhost:1313/a.png)"></div>`, wantError: "CSS url"},
		{name: "allow localhost in script code", appEnv: "production", content: `<link rel="canonical" href="https://zoking.tech/"><script>const health = "http://localhost:18080/healthz";</script>`},
		{name: "unknown environment fails closed", appEnv: "staging", content: `<link rel="canonical" href="http://localhost:1313/"><a href="http://localhost:18080">bad</a>`, wantError: "SITE_BASE_URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := t.TempDir()
			if err := os.WriteFile(filepath.Join(outputPath, "index.html"), []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			err := ValidateReleaseOutputPublicURLs(tt.appEnv, outputPath, "https://zoking.tech/", "https://api.zoking.tech")
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestValidateReleaseOutputPublicURLsRejectsUnsafeCSSFile(t *testing.T) {
	outputPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputPath, "index.html"), []byte(`<link rel="canonical" href="https://zoking.tech/">`), 0o644); err != nil {
		t.Fatalf("write HTML fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputPath, "theme.CSS"), []byte(`.hero { background-image: url("http://192.168.1.2/cover.png"); }`), 0o644); err != nil {
		t.Fatalf("write CSS fixture: %v", err)
	}
	err := ValidateReleaseOutputPublicURLs("production", outputPath, "https://zoking.tech/", "https://api.zoking.tech")
	if err == nil || !strings.Contains(err.Error(), "CSS url()") {
		t.Fatalf("expected CSS URL rejection, got %v", err)
	}
}

func withSiteURL(settings SiteSettingsSnapshot, value string) SiteSettingsSnapshot {
	settings.Site.BaseURL = value
	return settings
}

func withCommentsURL(settings SiteSettingsSnapshot, value string) SiteSettingsSnapshot {
	settings.Comments.APIBase = value
	return settings
}

func withoutComments(settings SiteSettingsSnapshot) SiteSettingsSnapshot {
	settings.Comments.Enabled = false
	return settings
}
