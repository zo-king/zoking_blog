# P22 成果时间线验收

## 自动化范围

专项脚本 `scripts/qa/archive-achievements-p22.mjs` 覆盖：

- `/archives/` 返回 200，唯一 H1 为“成果时间线”；页面没有文章选择器或文章内容。
- 当前年份至 2024 的全部年份轨道和年份锚点存在，锚点目标稳定。
- 缺少成果 data 时显示“暂无已发布成果”，同时保留空年份轨道。
- Hugo 生成的侧栏头像可加载。
- 1280px、390px、320px 无页面级横向溢出；移动年份轨道为单列。
- reduced motion 下不启用平滑滚动，浏览器无 `pageerror` 或 `console.error`。
- IntersectionObserver 运行后只有一个年份链接获得 `active`；完全禁用 JavaScript 时全部年份和空态仍可阅读。

## 命令

```powershell
.\.tools\hugo\hugo.exe --source apps/site --destination dist\p22-build --cleanDestinationDir --gc --minify
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\archive-achievements-p22.mjs
git diff --check -- apps/site/layouts/archives.html apps/site/layouts/_partials/achievement-list/item.html apps/site/assets/scss/archive-timeline.scss apps/site/assets/ts/archiveTimeline.ts apps/site/content/page/archives/index.md scripts/qa/archive-achievements-p22.mjs docs/frontend/archive-achievements-p22.md docs/qa/archive-achievements-p22.md
```

脚本默认访问 `http://localhost:1313`，可通过 `P22_SITE_BASE` 和 `P22_EVIDENCE_DIR` 覆盖。
