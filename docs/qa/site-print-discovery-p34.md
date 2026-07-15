# SITE-PRINT-DISCOVERY-P34-001 QA

## 自动化

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\site-print-discovery-p34.mjs
```

生产构建：

```powershell
$env:HUGO_COMMENTS_API_BASE='https://api.zoking.tech'
.\.tools\hugo\hugo.exe --source apps/site --destination ../../dist/p34-site --cleanDestinationDir --environment production --gc --minify
```

## 覆盖

- 文章页加载一份 `media="print"` 样式；首页不加载。
- 屏幕端侧栏可见、打印 byline 隐藏。
- 打印端标题、作者、发布日期、正文和许可可见。
- 开屏、顶栏、左右侧栏、进度、目录、系列、翻页、分享、评论、相关文章、站点页脚和代码工具栏隐藏。
- A4 794x1123 与窄视口 390x844 无横向溢出。
- 代码 `pre-wrap`、overflow visible，不把长行硬裁掉。
- 深色主题打印仍为纯白背景和 `#111` 正文。
- 无 JavaScript 时开屏层打印隐藏，作者和 RSS discovery 仍存在。
- 文章页只有一个站点 RSS discovery；首页和 taxonomy 各只有自己的一个 feed。
- BlogPosting 作者/发布者为 `Zoking` Person，URL 为公开 GitHub；发布日期和修改日期不漂移。

## 结果

| 检查 | 结果 |
|---|---|
| P34 专项 Playwright | Pass |
| Production Hugo build，62 pages | Pass |
| A4/390 无横向溢出 | Pass |
| 深色主题与无 JavaScript | Pass |
| RSS discovery 去重 | Pass |
| BlogPosting 作者与日期 | Pass |
| P16 读者回归，7 项 | Pass |
| P31/P32/P33 回归 | Pass |
| console/page error | 0 |

专项测试先后发现并修复：开屏已移除时测试等待、标题 locator 不唯一、主题背景过渡导致近白底、标题和代码块负边距造成 10px A4 溢出。

证据：`docs/process/evidence/site-p34-print-a4-794x1123.png`、`site-p34-print-narrow-390x844.png`。
