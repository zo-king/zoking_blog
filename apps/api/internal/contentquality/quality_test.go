package contentquality

import (
	"strings"
	"testing"
)

func TestEvaluateBlocksRequiredAndNonPublicContent(t *testing.T) {
	report := Evaluate(Document{Kind: "post", Visibility: "private"})
	for _, code := range []string{"TITLE_REQUIRED", "SLUG_REQUIRED", "CONTENT_REQUIRED", "VISIBILITY_NOT_PUBLIC"} {
		if !hasIssue(report, code, "error") {
			t.Fatalf("missing blocking issue %s: %#v", code, report.Issues)
		}
	}
	if report.Ready || report.Status != "blocked" {
		t.Fatalf("report should be blocked: %#v", report)
	}
}

func TestEvaluateMarkdownUsesASTAndIgnoresCodeExamples(t *testing.T) {
	content := strings.Repeat("可靠的工程内容。", 40) + "\n\n## 示例\n\n" +
		"```markdown\n![ ](javascript:alert(1))\n<script>alert(1)</script>\n```\n\n" +
		"![架构图](/media-files/architecture.png)\n"
	report := Evaluate(validPost(content))
	if hasIssue(report, "UNSAFE_URL", "error") || hasIssue(report, "DANGEROUS_HTML", "error") {
		t.Fatalf("code example was treated as executable content: %#v", report.Issues)
	}
	if !report.Ready {
		t.Fatalf("valid post should be publishable: %#v", report.Issues)
	}
}

func TestEvaluateBlocksDangerousLinksAndRawHTML(t *testing.T) {
	for _, content := range []string{
		"[运行](JaVaScRiPt%3Aalert(1))",
		"<a href=\"javascript:alert(1)\">运行</a>",
		"<img src=\"/ok.png\" onerror=\"alert(1)\" alt=\"图\">",
		"<p style=\"position:fixed;inset:0\">覆盖页面</p>",
		"<style>body{display:none}</style>",
		"<meta http-equiv=\"refresh\" content=\"0;url=https://example.com\">",
		"<script>alert(1)</script>",
	} {
		report := Evaluate(validPost(content))
		if report.Ready {
			t.Fatalf("dangerous content was accepted: %q %#v", content, report.Issues)
		}
	}
}

func TestEvaluateReturnsStableWarningsAndHash(t *testing.T) {
	document := validPost("# 一级标题\n\n![](/image.png)\n\n短正文")
	document.HasCover = false
	document.CategoryCount = 0
	document.TagCount = 0
	report := Evaluate(document)
	for _, code := range []string{"BODY_H1_PRESENT", "IMAGE_ALT_MISSING", "CONTENT_SHORT", "COVER_MISSING", "CATEGORY_MISSING", "TAG_MISSING"} {
		if !hasIssue(report, code, "warning") {
			t.Fatalf("missing warning %s: %#v", code, report.Issues)
		}
	}
	if !report.Ready || len(report.ContentHash) != 64 || report.PolicyVersion != PolicyVersion {
		t.Fatalf("unexpected report: %#v", report)
	}
	if second := Evaluate(document); second.ContentHash != report.ContentHash {
		t.Fatalf("content hash is unstable: %s != %s", report.ContentHash, second.ContentHash)
	}
}

func TestEvaluatePageMenuWarning(t *testing.T) {
	report := Evaluate(Document{
		Kind: "page", Title: "关于", Slug: "about", ContentMD: strings.Repeat("页面正文。", 60),
		Visibility: "public", Summary: strings.Repeat("摘要", 15), SEODescription: strings.Repeat("页面搜索描述", 10),
		ShowInMenu: true,
	})
	if !hasIssue(report, "MENU_ICON_MISSING", "warning") || !report.Ready {
		t.Fatalf("unexpected page report: %#v", report)
	}
}

func TestEvaluateBlocksIncompleteOrInvalidSeriesAssignment(t *testing.T) {
	incomplete := validPost(strings.Repeat("系列正文。", 80))
	incomplete.SeriesSelected = true
	if report := Evaluate(incomplete); report.Ready || !hasIssue(report, "SERIES_ASSIGNMENT_INCOMPLETE", "error") {
		t.Fatalf("incomplete series should be blocked: %#v", report)
	}

	invalidOrder := validPost(strings.Repeat("系列正文。", 80))
	invalidOrder.SeriesSelected = true
	invalidOrder.SeriesOrderSet = true
	if report := Evaluate(invalidOrder); report.Ready || !hasIssue(report, "SERIES_ORDER_INVALID", "error") {
		t.Fatalf("non-positive series order should be blocked: %#v", report)
	}
}

func validPost(content string) Document {
	return Document{
		Kind: "post", Title: "工程文章", Slug: "engineering-post", ContentMD: content,
		Visibility: "public", Summary: strings.Repeat("文章摘要", 10), SEODescription: strings.Repeat("搜索描述", 12),
		HasCover: true, CategoryCount: 1, TagCount: 1,
	}
}

func hasIssue(report Report, code string, severity string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code && issue.Severity == severity {
			return true
		}
	}
	return false
}
