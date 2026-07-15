# SITE-ARTICLE-UTILITIES-P33-001 QA

## 自动化

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\site-article-utilities-p33.mjs
```

生产构建：

```powershell
$env:HUGO_COMMENTS_API_BASE='https://api.zoking.tech'
.\.tools\hugo\hugo.exe --source apps/site --destination ../../dist/p33-site --cleanDestinationDir --environment production --gc --minify
```

## 覆盖

- 标题链接名称同时描述定位与复制行为。
- 键盘聚焦后 `#` 可见并具有明确 outline。
- 中文标题复制为完整 pathname + 编码 hash URL。
- 成功勾选和 `aria-live` 宣告。
- 无 JavaScript 时保留原生 `href="#id"`。
- Go 语言标签和单一复制按钮。
- 代码复制严格等于 `code.textContent`，不包含 Chroma 行号。
- Clipboard 拒绝时显示中文失败信息和手动选择恢复路径；API 缺失时验证本地 textarea fallback。
- reduced-motion 下锚点滚动行为为 `auto`。
- 触屏标题链接 44x44px、持续可见。
- 1280x900、390x844、320x568 无页面横向溢出。

## 结果

| 检查 | 结果 |
|---|---|
| P33 专项 Playwright | Pass |
| Production Hugo build，62 pages | Pass |
| P16 读者回归，7 项 | Pass |
| P31 页面过渡/近况回归 | Pass |
| P32 阅读设置回归 | Pass |
| Clipboard 成功/失败/fallback | Pass |
| 无 JavaScript 与 reduced-motion | Pass |
| 1280/390/320 | Pass |
| console/page error | 0 |

证据：`docs/process/evidence/site-p33-article-tools-desktop-1280x900.png`、`site-p33-article-tools-mobile-390x844.png`。
