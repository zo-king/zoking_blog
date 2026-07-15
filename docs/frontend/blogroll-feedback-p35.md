# P35 Blogroll 与内容纠错

日期：2026-07-15

## Blogroll 数据契约

数据文件：`apps/site/data/blogroll.json`

```json
{
  "schema_version": 1,
  "updated_at": "2026-07-15T19:30:00+08:00",
  "links": [
    {
      "title": "Anthony Fu",
      "description": "...",
      "website": "https://antfu.me/",
      "feed_url": "https://antfu.me/feed.xml",
      "category": "技术博客"
    }
  ]
}
```

- `website` 必须是无用户名密码的绝对 HTTPS URL。
- `feed_url` 只能是经过人工验证的绝对 HTTPS URL；没有 Feed 时为 `null`。
- `updated_at` 是数据清单更新时间，不使用每次构建的当前时间，避免非确定性产物。
- 友链页按数组顺序展示，OPML 只筛选非空 `feed_url`。

## OPML 输出

- Hugo 首页增加 `OPML` 输出格式，生成 `/blogroll.opml`，页面总输出从 62 增至 63。
- Media Type 为 `text/x-opml`，版本为 OPML 2.0。
- `notAlternative = true`，不会在首页 head 增加无意义 alternate；入口只出现在友链页。
- XML 使用 Hugo 模板和 XML escaping 生成，不用正则拼装或浏览器脚本。
- 下载链接使用原生 `download="zoking-blogroll.opml"`，触控高度至少 44px。

## 内容纠错

文章页生成：

```text
https://github.com/zo-king/zoking_blog/issues/new
```

Query 只包含：

- `[内容纠错] {文章标题}`。
- 当前文章 permalink。
- `Lastmod` 日期。
- 章节、问题描述、建议修改的空白骨架。

链接为普通 `<a>`，无 JavaScript 可用，`target="_blank"` 与 `rel="noopener noreferrer"`。关于、友链等普通页面不显示，打印模式因文章导航整体隐藏而不进入纸面。

仓库 Issue chooser 另提供 `content_correction.yml`，用于从 GitHub 直接创建纠错单。文章页使用预填 blank issue URL，是因为 GitHub Issue Form 不可靠地支持从 URL 预填自定义字段。

## 分享反馈

- `navigator.share()` Promise 成功后显示“分享操作已完成”，不再错误声称面板刚打开。
- 不支持 Web Share 时通过 `copyPlainText` 复制文章 URL，继续提供 Clipboard API 和 textarea fallback。
- 纠错链接、分享按钮和断点续读按钮共用文章操作区，触控高度为 44px。

## 非目标

- 不从访客浏览器请求 Feed。
- 不展示友链活跃度、热度或虚假更新时间。
- 不把 blogroll 暂时纳入 Admin Page 模型。
- 不实现 Webring、Webmention 或远程文章聚合。
