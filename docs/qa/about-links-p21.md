# P21 关于页与友链页验收

日期：2026-07-13

## 自动化覆盖

`scripts/qa/about-links-p21.mjs` 覆盖：

- `/about/` 和 `/links/` 返回 200，均只有一个主 H1；
- 两页均没有 `related-content` 或“相关文章”标题；
- 普通文章页仍然保留相关文章，避免独立页面清理破坏文章阅读链路；
- 友链卡片数量、host、外链箭头和头像容器完整；
- favicon 成功显示、图片加载失败后的首字母回退和二级 favicon 服务路径；
- 1280x800、390x844 无横向溢出；
- 初始页面无 `pageerror` 和 `console.error`；
- 友链外链使用新窗口和 `noopener noreferrer`。

## 结果

- Hugo Extended 0.160.1 minify build：PASS，46 pages。
- P21 Playwright：PASS。
- 桌面/移动视觉复核：PASS，无相关文章和破图图标。
- `preflight.ps1 -SkipE2E`：PASS，Go tests、Admin production build、Hugo build 全部通过。
- P20 归档时间线回归：PASS。
- `git diff --check`：PASS；仅有工作树既有 LF/CRLF 提示，无空白错误。

## 证据

- `docs/process/evidence/about-p21-1280x800.png`
- `docs/process/evidence/about-p21-390x844.png`
- `docs/process/evidence/links-p21-1280x800.png`
- `docs/process/evidence/links-p21-390x844.png`
