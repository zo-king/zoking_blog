# SITE-ARTICLE-CALLOUT-P37-001 QA

## 运行

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\site-article-callout-p37.mjs
```

默认站点地址为 `http://localhost:1313`，可用 `P37_SITE_BASE` 覆盖。

## 覆盖范围

- Gin 文章包含 `NOTE` 与 `WARNING` 两种提示块。
- 其他两篇文章分别包含 `TIP` 与 `IMPORTANT`。
- `alert-header`、`alert-icon`、`alert-title`、`alert-body` 结构完整，Markdown 标记不泄漏。
- 中文内容、文章链接、代码和目录不受影响。
- 桌面轻阴影/圆角/间距样式生效，暗色背景和标题颜色不塌陷。
- 390px、320px 无横向溢出，提示块宽度不超过视口。
- 无 JavaScript 时提示块和正文完整可读。
- console/page error 不作为评论 API 的依赖；专项结构检查不依赖第三方服务。

## 结果

| 检查 | 结果 |
|---|---|
| P37 专项 Playwright | Pass |
| Hugo production build | Pass，63 pages |
| 1280/390/320 | Pass |
| 暗色主题 | Pass |
| 无 JavaScript | Pass |
| P16 阅读回归 | Pass，7/7 |
| P29 发现回归 | Pass |
| P34 打印/Feed 回归 | Pass |
| `preflight.ps1 -SkipE2E` | Pass，Go/Admin/Hugo |

证据：

- `docs/process/evidence/site-p37-article-callouts-desktop-1280x900.png`
- `docs/process/evidence/site-p37-article-callouts-mobile-390x844.png`
