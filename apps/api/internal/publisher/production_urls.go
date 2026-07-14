package publisher

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

var (
	ambiguousIPLiteralPattern = regexp.MustCompile(`(?i)^(?:0x[0-9a-f]+|[0-9]+)(?:\.(?:0x[0-9a-f]+|[0-9]+)){0,3}$`)
	cssURLPattern             = regexp.MustCompile(`(?i)url\(\s*['"]?([^'")]+)`)
)

// ValidateReleasePublicURLs protects canonical and browser-facing URLs before a
// production release is built. Preview builds intentionally do not call it.
func ValidateReleasePublicURLs(appEnv string, settings SiteSettingsSnapshot, expectedSiteBaseURL string, expectedAPIBaseURL string, mediaPublicBaseURL string) error {
	if !enforceProductionURLPolicy(appEnv) {
		return nil
	}

	if err := validateProductionHTTPSURL(expectedSiteBaseURL, true); err != nil {
		return fmt.Errorf("SITE_BASE_URL: %w", err)
	}
	if err := validateProductionHTTPSURL(expectedAPIBaseURL, true); err != nil {
		return fmt.Errorf("PUBLIC_API_BASE_URL: %w", err)
	}
	if err := validateProductionHTTPSURL(settings.Site.BaseURL, true); err != nil {
		return fmt.Errorf("site.base_url: %w", err)
	}
	if !samePublicBaseURL(settings.Site.BaseURL, expectedSiteBaseURL) {
		return fmt.Errorf("site.base_url must match SITE_BASE_URL")
	}
	if settings.Comments.Enabled {
		if strings.TrimSpace(settings.Comments.APIBase) == "" {
			return fmt.Errorf("comments.api_base: URL is required")
		}
		if err := validateProductionHTTPSURL(settings.Comments.APIBase, true); err != nil {
			return fmt.Errorf("comments.api_base: %w", err)
		}
		if !samePublicBaseURL(settings.Comments.APIBase, expectedAPIBaseURL) {
			return fmt.Errorf("comments.api_base must match PUBLIC_API_BASE_URL")
		}
	}
	if err := validateProductionMediaBaseURL(mediaPublicBaseURL); err != nil {
		return fmt.Errorf("media public base URL: %w", err)
	}
	return nil
}

func ValidateReleaseOutputPublicURLs(appEnv string, outputPath string, expectedSiteBaseURL string, expectedAPIBaseURL string) error {
	if !enforceProductionURLPolicy(appEnv) {
		return nil
	}

	expectedSite := strings.TrimRight(strings.TrimSpace(expectedSiteBaseURL), "/")
	expectedAPI := strings.TrimRight(strings.TrimSpace(expectedAPIBaseURL), "/")
	textExtensions := map[string]struct{}{
		".css": {}, ".html": {}, ".js": {}, ".json": {}, ".map": {}, ".txt": {}, ".xml": {},
	}
	canonicalSeen := false

	err := filepath.WalkDir(outputPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if _, ok := textExtensions[strings.ToLower(filepath.Ext(path))]; !ok {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".html" {
			document, err := xhtml.Parse(bytes.NewReader(raw))
			if err != nil {
				return fmt.Errorf("parse generated HTML: %w", err)
			}
			seen, err := validateGeneratedHTMLPublicURLs(document, expectedSite, expectedAPI)
			if err != nil {
				relative, _ := filepath.Rel(outputPath, path)
				return fmt.Errorf("release output %s: %w", filepath.ToSlash(relative), err)
			}
			canonicalSeen = canonicalSeen || seen
		} else if extension == ".css" {
			if err := validateGeneratedCSSPublicURLs(string(raw)); err != nil {
				relative, _ := filepath.Rel(outputPath, path)
				return fmt.Errorf("release output %s: %w", filepath.ToSlash(relative), err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !canonicalSeen {
		return fmt.Errorf("release output does not contain a canonical URL")
	}
	return nil
}

func enforceProductionURLPolicy(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "development", "dev", "test":
		return false
	default:
		return true
	}
}

func validateGeneratedHTMLPublicURLs(document *xhtml.Node, expectedSite string, expectedAPI string) (bool, error) {
	canonicalSeen := false
	var visit func(*xhtml.Node) error
	visit = func(node *xhtml.Node) error {
		if node.Type == xhtml.ElementNode {
			elementName := strings.ToLower(node.Data)
			attributes := map[string]string{}
			for _, attribute := range node.Attr {
				attributes[strings.ToLower(attribute.Key)] = strings.TrimSpace(attribute.Val)
			}
			switch elementName {
			case "link":
				if hasSpaceSeparatedToken(attributes["rel"], "canonical") {
					canonicalSeen = true
					if !sameURLOrigin(attributes["href"], expectedSite) {
						return fmt.Errorf("canonical URL must use SITE_BASE_URL origin")
					}
				}
			case "meta":
				if strings.EqualFold(attributes["property"], "og:url") && !sameURLOrigin(attributes["content"], expectedSite) {
					return fmt.Errorf("og:url must use SITE_BASE_URL origin")
				}
				if strings.EqualFold(attributes["http-equiv"], "refresh") {
					if refreshURL, ok := metaRefreshURL(attributes["content"]); ok && isUnsafeBrowserFacingURL(refreshURL) {
						return fmt.Errorf("meta refresh contains a non-public URL %q", refreshURL)
					}
				}
			}
			if apiBase, ok := attributes["data-api-base"]; ok && !samePublicBaseURL(apiBase, expectedAPI) {
				return fmt.Errorf("comments data-api-base must match PUBLIC_API_BASE_URL")
			}
			for _, attributeName := range []string{"href", "src", "action", "poster", "formaction"} {
				if value, ok := attributes[attributeName]; ok && isUnsafeBrowserFacingURL(value) {
					return fmt.Errorf("%s contains a non-public URL %q", attributeName, value)
				}
			}
			if elementName == "object" && isUnsafeBrowserFacingURL(attributes["data"]) {
				return fmt.Errorf("data contains a non-public URL %q", attributes["data"])
			}
			if value, ok := attributes["srcset"]; ok {
				for _, candidate := range srcsetURLs(value) {
					if isUnsafeBrowserFacingURL(candidate) {
						return fmt.Errorf("srcset contains a non-public URL %q", candidate)
					}
				}
			}
			if value, ok := attributes["style"]; ok {
				if err := validateGeneratedCSSPublicURLs(value); err != nil {
					return fmt.Errorf("style attribute: %w", err)
				}
			}
			if elementName == "style" {
				var content strings.Builder
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					if child.Type == xhtml.TextNode {
						content.WriteString(child.Data)
					}
				}
				if err := validateGeneratedCSSPublicURLs(content.String()); err != nil {
					return fmt.Errorf("style element: %w", err)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(document); err != nil {
		return false, err
	}
	return canonicalSeen, nil
}

func srcsetURLs(value string) []string {
	var values []string
	for _, candidate := range strings.Split(value, ",") {
		fields := strings.Fields(strings.TrimSpace(candidate))
		if len(fields) > 0 {
			values = append(values, fields[0])
		}
	}
	return values
}

func metaRefreshURL(value string) (string, bool) {
	parts := strings.Split(value, ";")
	for _, part := range parts[1:] {
		trimmed := strings.TrimSpace(part)
		separator := strings.Index(trimmed, "=")
		if separator < 0 || !strings.EqualFold(strings.TrimSpace(trimmed[:separator]), "url") {
			continue
		}
		candidate := strings.Trim(strings.TrimSpace(trimmed[separator+1:]), "'\"")
		return candidate, candidate != ""
	}
	return "", false
}

func validateGeneratedCSSPublicURLs(content string) error {
	for _, match := range cssURLPattern.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		candidate := strings.TrimSpace(match[1])
		if isUnsafeBrowserFacingURL(candidate) {
			return fmt.Errorf("CSS url() contains a non-public URL %q", candidate)
		}
	}
	return nil
}

func isUnsafeBrowserFacingURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "//") {
		parsed, err := url.Parse(trimmed)
		if err != nil || !parsed.IsAbs() {
			return false
		}
	} else {
		trimmed = "https:" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	return err == nil && isUnsafePublicHostname(parsed.Hostname())
}

func hasSpaceSeparatedToken(value string, expected string) bool {
	for _, token := range strings.Fields(value) {
		if strings.EqualFold(token, expected) {
			return true
		}
	}
	return false
}

func sameURLOrigin(value string, expectedBase string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	expected, expectedErr := url.Parse(strings.TrimSpace(expectedBase))
	if err != nil || expectedErr != nil || parsed.Host == "" || expected.Host == "" || parsed.User != nil || expected.User != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, expected.Scheme) && strings.EqualFold(parsed.Host, expected.Host)
}

func samePublicBaseURL(left string, right string) bool {
	leftURL, leftErr := url.Parse(strings.TrimSpace(left))
	rightURL, rightErr := url.Parse(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	if leftURL.User != nil || rightURL.User != nil || leftURL.RawQuery != "" || rightURL.RawQuery != "" || leftURL.Fragment != "" || rightURL.Fragment != "" {
		return false
	}
	normalize := func(parsed *url.URL) string {
		path := strings.TrimRight(parsed.EscapedPath(), "/")
		return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host) + path
	}
	return normalize(leftURL) == normalize(rightURL)
}

func validateProductionMediaBaseURL(value string) error {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
		parsed, err := url.ParseRequestURI(trimmed)
		if err != nil || parsed.Path == "" || parsed.RawQuery != "" || parsed.Fragment != "" || !validPublicBasePath(parsed.EscapedPath()) {
			return fmt.Errorf("must be a clean root-relative path or a public HTTPS URL")
		}
		return nil
	}
	if err := validateProductionHTTPSURL(trimmed, false); err != nil {
		return err
	}
	parsed, _ := url.Parse(trimmed)
	if !validPublicBasePath(parsed.EscapedPath()) {
		return fmt.Errorf("must include a clean non-root path")
	}
	return nil
}

func validPublicBasePath(escapedPath string) bool {
	unescaped, err := url.PathUnescape(escapedPath)
	if err != nil || unescaped == "" || unescaped == "/" || strings.Contains(unescaped, "\\") {
		return false
	}
	normalized := strings.TrimRight(unescaped, "/")
	return strings.HasPrefix(normalized, "/") && path.Clean(normalized) == normalized
}

func validateProductionHTTPSURL(value string, requireRootPath bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("must be an absolute URL")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("must use HTTPS")
	}
	if parsed.User != nil {
		return fmt.Errorf("must not contain user information")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("must not contain a query or fragment")
	}
	if requireRootPath && parsed.EscapedPath() != "" && parsed.EscapedPath() != "/" {
		return fmt.Errorf("must use the origin root path")
	}

	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "" {
		return fmt.Errorf("must contain a hostname")
	}
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return fmt.Errorf("must not use localhost")
	}
	if isUnsafePublicHostname(hostname) {
		return fmt.Errorf("must use a public hostname, not a loopback, private, link-local, unspecified, or ambiguous IP address")
	}
	return nil
}

func isUnsafePublicHostname(value string) bool {
	hostname := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") || ambiguousIPLiteralPattern.MatchString(hostname) {
		return true
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}
	if !strings.Contains(hostname, ".") {
		return true
	}
	for _, suffix := range []string{".local", ".internal", ".lan", ".home", ".test", ".invalid", ".example"} {
		if strings.HasSuffix(hostname, suffix) {
			return true
		}
	}
	return false
}
