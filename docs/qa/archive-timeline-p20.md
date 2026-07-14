# P20 搜索降级与归档时间线验收

日期：2026-07-13

## 1. 自动化范围

`scripts/qa/archive-timeline-p20.mjs` 覆盖：

- 首页主导航和右栏均不显示搜索。
- `/search/`、`/search/index.json`、`/categories/`、`/tags/` 继续返回 200。
- 归档唯一 H1，年份 H2、文章 H3 无标题跳级。
- `ol/li/article/time` 语义完整，3 篇文章按 07-10、07-08、07-05 倒序，年份数量为 3。
- 两张缩略图加载成功且不进入辅助技术树。
- 1280x800 浅色、1280x800 深色、390x844、320x568 无页面级横向溢出。
- 移动链接触控高度不低于 44px，reduced motion 模式可用。
- 非预期 `pageerror` 和 `console.error` 为 0。

## 2. 验收结果

- Hugo Extended 0.160.1 minify build：PASS，46 pages。
- P20 Playwright：PASS。
- 桌面、深色、390px 与 320px 人工截图复核：PASS。
- P19 GitHub/归档回归：PASS。
- 仓库 `preflight.ps1 -SkipE2E`：PASS，Go tests、Admin production build、Hugo build 全部通过。
- `git diff --check`：PASS；仅输出工作树既有 LF/CRLF 提示，无空白错误。

## 3. 证据

- `docs/process/evidence/archive-p20-desktop-1280x800.png`
- `docs/process/evidence/archive-p20-dark-1280x800.png`
- `docs/process/evidence/archive-p20-mobile-390x844.png`
- `docs/process/evidence/archive-p20-mobile-320x568.png`

## 4. 运行命令

```powershell
.\.tools\hugo\hugo.exe --source apps/site --destination dist\p20-build --cleanDestinationDir --gc --minify
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\archive-timeline-p20.mjs
node .\scripts\qa\github-projects-p19.mjs
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
git diff --check
```
