package contentquality

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"net/url"
	"sort"
	"strings"
	"unicode"

	markdown "github.com/yuin/goldmark"
	markdownast "github.com/yuin/goldmark/ast"
	markdownhtml "github.com/yuin/goldmark/renderer/html"
	markdowntext "github.com/yuin/goldmark/text"
	xhtml "golang.org/x/net/html"
)

const PolicyVersion = "2026-07-13.2"

type Document struct {
	Kind           string
	Title          string
	Slug           string
	SlugError      string
	Summary        string
	ContentMD      string
	Visibility     string
	SEOTitle       string
	SEODescription string
	HasCover       bool
	CategoryCount  int
	TagCount       int
	SeriesSelected bool
	SeriesOrderSet bool
	SeriesOrder    int
	ShowInMenu     bool
	MenuIcon       string
}

type Issue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

type Report struct {
	Status        string  `json:"status"`
	Ready         bool    `json:"ready"`
	Score         int     `json:"score"`
	ErrorCount    int     `json:"error_count"`
	WarningCount  int     `json:"warning_count"`
	ContentHash   string  `json:"content_hash"`
	PolicyVersion string  `json:"policy_version"`
	Issues        []Issue `json:"issues"`
}

type markdownFacts struct {
	visibleText      string
	imageCount       int
	missingImageAlt  bool
	hasBodyH1        bool
	hasUnsafeURL     bool
	hasDangerousHTML bool
}

func Evaluate(document Document) Report {
	document = normalizeDocument(document)
	facts := inspectMarkdown(document.ContentMD)
	issues := make([]Issue, 0, 12)
	seen := make(map[string]struct{})
	add := func(code, severity, field, message string) {
		key := severity + ":" + code + ":" + field
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		issues = append(issues, Issue{Code: code, Severity: severity, Field: field, Message: message})
	}

	if document.Title == "" {
		add("TITLE_REQUIRED", "error", "title", "标题不能为空")
	}
	if document.Slug == "" {
		add("SLUG_REQUIRED", "error", "slug", "Slug 不能为空")
	} else if document.SlugError != "" {
		add("SLUG_INVALID", "error", "slug", document.SlugError)
	}
	if trimVisible(facts.visibleText) == "" && facts.imageCount == 0 {
		add("CONTENT_REQUIRED", "error", "content_md", "正文必须包含可见文本、代码或图片")
	}
	if document.Visibility != "public" {
		add("VISIBILITY_NOT_PUBLIC", "error", "visibility", "正式发布仅支持公开可见性")
	}
	if facts.hasUnsafeURL {
		add("UNSAFE_URL", "error", "content_md", "正文包含不安全或不受支持的链接协议")
	}
	if facts.hasDangerousHTML {
		add("DANGEROUS_HTML", "error", "content_md", "正文包含脚本、嵌入表单或事件处理属性")
	}

	if document.Summary == "" {
		add("SUMMARY_MISSING", "warning", "summary", "建议填写摘要，便于列表页快速浏览")
	} else if runeLength(document.Summary) < 20 {
		add("SUMMARY_SHORT", "warning", "summary", "摘要较短，建议补充到 20 个字符以上")
	}
	if document.SEODescription == "" {
		add("SEO_DESCRIPTION_MISSING", "warning", "seo_description", "建议填写搜索摘要")
	} else if length := runeLength(document.SEODescription); length < 40 || length > 160 {
		add("SEO_DESCRIPTION_LENGTH", "warning", "seo_description", "搜索摘要建议保持在 40 到 160 个字符")
	}
	if runeLength(document.SEOTitle) > 60 {
		add("SEO_TITLE_LONG", "warning", "seo_title", "SEO 标题超过 60 个字符，搜索结果可能被截断")
	}
	if runeLength(trimVisible(facts.visibleText)) < 200 {
		add("CONTENT_SHORT", "warning", "content_md", "正文较短，请确认内容已经完整")
	}
	if facts.hasBodyH1 {
		add("BODY_H1_PRESENT", "warning", "content_md", "页面标题已经是一级标题，正文建议从二级标题开始")
	}
	if facts.missingImageAlt {
		add("IMAGE_ALT_MISSING", "warning", "content_md", "部分图片缺少替代文本，请确认是否为装饰图片")
	}
	if document.Kind == "post" {
		if document.SeriesSelected != document.SeriesOrderSet {
			add("SERIES_ASSIGNMENT_INCOMPLETE", "error", "series_id", "系列和系列序号必须同时填写或同时清空")
		} else if document.SeriesSelected && document.SeriesOrder <= 0 {
			add("SERIES_ORDER_INVALID", "error", "series_order", "系列序号必须是大于 0 的整数")
		}
		if !document.HasCover {
			add("COVER_MISSING", "warning", "cover_media_id", "建议为文章选择封面图")
		}
		if document.CategoryCount == 0 {
			add("CATEGORY_MISSING", "warning", "category_ids", "建议至少选择一个分类")
		}
		if document.TagCount == 0 {
			add("TAG_MISSING", "warning", "tag_ids", "建议添加标签以改善内容发现")
		}
	}
	if document.Kind == "page" && document.ShowInMenu && document.MenuIcon == "" {
		add("MENU_ICON_MISSING", "warning", "menu_icon", "页面显示在菜单时建议配置图标")
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity == "error"
		}
		if issues[i].Field != issues[j].Field {
			return issues[i].Field < issues[j].Field
		}
		return issues[i].Code < issues[j].Code
	})

	report := Report{Score: 100, ContentHash: contentHash(document), PolicyVersion: PolicyVersion, Issues: issues}
	for _, issue := range issues {
		if issue.Severity == "error" {
			report.ErrorCount++
			report.Score -= 30
		} else {
			report.WarningCount++
			report.Score -= 6
		}
	}
	if report.Score < 0 {
		report.Score = 0
	}
	report.Ready = report.ErrorCount == 0
	switch {
	case report.ErrorCount > 0:
		report.Status = "blocked"
	case report.WarningCount > 0:
		report.Status = "warning"
	default:
		report.Status = "passed"
	}
	return report
}

func normalizeDocument(document Document) Document {
	document.Kind = strings.TrimSpace(document.Kind)
	document.Title = strings.TrimSpace(document.Title)
	document.Slug = strings.TrimSpace(document.Slug)
	document.SlugError = strings.TrimSpace(document.SlugError)
	document.Summary = strings.TrimSpace(document.Summary)
	document.Visibility = strings.TrimSpace(document.Visibility)
	document.SEOTitle = strings.TrimSpace(document.SEOTitle)
	document.SEODescription = strings.TrimSpace(document.SEODescription)
	document.MenuIcon = strings.TrimSpace(document.MenuIcon)
	return document
}

func inspectMarkdown(source string) markdownFacts {
	facts := markdownFacts{}
	reader := markdowntext.NewReader([]byte(source))
	document := markdown.DefaultParser().Parse(reader)
	_ = markdownast.Walk(document, func(node markdownast.Node, entering bool) (markdownast.WalkStatus, error) {
		if !entering {
			return markdownast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *markdownast.Image:
			if unsafeURL(string(typed.Destination), false) {
				facts.hasUnsafeURL = true
			}
		case *markdownast.Link:
			if unsafeURL(string(typed.Destination), true) {
				facts.hasUnsafeURL = true
			}
		case *markdownast.Heading:
			if typed.Level == 1 {
				facts.hasBodyH1 = true
			}
		}
		return markdownast.WalkContinue, nil
	})

	var rendered bytes.Buffer
	engine := markdown.New(markdown.WithRendererOptions(markdownhtml.WithUnsafe()))
	if err := engine.Convert([]byte(source), &rendered); err != nil {
		return facts
	}
	root, err := xhtml.Parse(strings.NewReader(rendered.String()))
	if err != nil {
		return facts
	}
	var textBuilder strings.Builder
	inspectHTMLNode(root, &facts, &textBuilder)
	facts.visibleText = textBuilder.String()
	return facts
}

func inspectHTMLNode(node *xhtml.Node, facts *markdownFacts, textBuilder *strings.Builder) {
	if node.Type == xhtml.ElementNode {
		tag := strings.ToLower(node.Data)
		switch tag {
		case "script", "iframe", "object", "embed", "form", "input", "button", "select", "textarea", "style", "link", "meta", "base":
			facts.hasDangerousHTML = true
		case "img":
			facts.imageCount++
			if alt, exists := htmlAttribute(node, "alt"); !exists || trimVisible(alt) == "" {
				facts.missingImageAlt = true
			}
		}
		for _, attribute := range node.Attr {
			name := strings.ToLower(strings.TrimSpace(attribute.Key))
			if strings.HasPrefix(name, "on") || name == "srcdoc" || name == "style" {
				facts.hasDangerousHTML = true
			}
			switch name {
			case "href", "action", "formaction":
				if unsafeURL(attribute.Val, true) {
					facts.hasUnsafeURL = true
				}
			case "src", "poster", "data":
				if unsafeURL(attribute.Val, false) {
					facts.hasUnsafeURL = true
				}
			}
		}
	}
	if node.Type == xhtml.TextNode {
		textBuilder.WriteString(node.Data)
		textBuilder.WriteByte(' ')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		inspectHTMLNode(child, facts, textBuilder)
	}
}

func htmlAttribute(node *xhtml.Node, name string) (string, bool) {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val, true
		}
	}
	return "", false
}

func unsafeURL(value string, allowMailto bool) bool {
	value = strings.TrimSpace(html.UnescapeString(value))
	if value == "" || strings.HasPrefix(value, "#") {
		return false
	}
	if strings.HasPrefix(value, "//") || strings.HasPrefix(value, "\\\\") {
		return true
	}
	for range 2 {
		decoded, err := url.PathUnescape(value)
		if err != nil || decoded == value {
			break
		}
		value = decoded
	}
	compact := strings.Map(func(r rune) rune {
		if r <= 0x20 || r == 0x7f {
			return -1
		}
		return unicode.ToLower(r)
	}, value)
	parsed, err := url.Parse(compact)
	if err != nil {
		return true
	}
	switch parsed.Scheme {
	case "", "http", "https":
		return false
	case "mailto":
		return !allowMailto
	default:
		return true
	}
}

func trimVisible(value string) string {
	return strings.TrimFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\u200b' || r == '\ufeff'
	})
}

func runeLength(value string) int {
	return len([]rune(strings.TrimSpace(value)))
}

func contentHash(document Document) string {
	payload, _ := json.Marshal(document)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
