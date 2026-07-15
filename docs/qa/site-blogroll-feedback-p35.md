# SITE-BLOGROLL-FEEDBACK-P35-001 QA

## 自动化

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\site-blogroll-feedback-p35.mjs
```

## 覆盖

- JSON schema version、9 个站点、4 个已验证 Feed。
- website/feed URL 只允许无凭据 HTTPS。
- 友链页面 front matter 不再包含 `links:`。
- 页面 9 张卡片与 JSON 顺序、标题完全一致。
- OPML 链接、download 文件名和 44px 触控高度。
- 浏览器没有请求任何第三方 Feed URL。
- 使用 DOMParser 验证 OPML 2.0、Content-Type、XML 无 parsererror。
- OPML title、ownerName、RFC822 dateModified 和 4 个 outline 字段。
- 内容纠错 GitHub host/path、中文 title、文章 URL、Lastmod 和问题骨架。
- 点击前没有 GitHub issue 请求；新窗口安全属性完整。
- Web Share payload 与“分享操作已完成”。
- 无 Web Share 时剪贴板复制文章 URL。
- 关于页不显示纠错，打印模式隐藏文章操作区。
- 无 JavaScript 时友链、OPML 和纠错链接可用。
- 1280x900、390x844、320x568 无横向溢出。

## 结果

| 检查 | 结果 |
|---|---|
| P35 专项 Playwright | Pass |
| Hugo production build，63 pages | Pass |
| OPML XML/Content-Type | Pass |
| 访客 Feed 请求 | 0 |
| 纠错预填与隐私边界 | Pass |
| Web Share/Clipboard | Pass |
| 无 JavaScript | Pass |
| 1280/390/320 | Pass |
| P21/P29 友链回归 | Pass |
| P33/P34/P16 文章回归 | Pass |
| console/page error | 0 |

首轮专项因第三方站点 `favicon.ico` 返回 404 产生 console error；测试随后与 P21 一致固定 favicon 网络，继续单独断言没有任何 Feed 请求。

证据：`docs/process/evidence/site-p35-blogroll-opml-desktop-1280x900.png`、`site-p35-article-feedback-desktop-1280x900.png`、`site-p35-article-feedback-mobile-390x844.png`。
