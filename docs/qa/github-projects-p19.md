# P19 GitHub 项目页验收记录

日期：2026-07-13

## 1. 自动化场景

`scripts/qa/github-projects-p19.mjs` 使用 Mock GitHub API，不消耗真实额度，也不写数据库，覆盖：

- 一级菜单包含归档/项目，不包含分类/标签；真实 taxonomy 路由继续返回 200。
- 归档页同时存在分类、标签和年份文章导航；P20 将年份区域升级为 `.archives-year` 时间线，P19 回归断言已同步。
- fork/archived 过滤、`pushed_at` 排序、最多 6 条和同时间稳定排序。
- 空描述、空语言、恶意名称和非 GitHub URL 的安全回退。
- 请求无 Authorization、Cookie、Referer；session cache 不含 owner/clone URL 等额外字段。
- 空响应、403、无自动重试、单次手动重试成功。
- 1280x800、390x844、320x568 无横向溢出，移动端单列。
- console/page error 除刻意模拟的 403 浏览器网络日志外均为 0。

## 2. 验收结果

- Hugo minify build：PASS，46 pages。
- `github-projects-p19.mjs`：PASS。
- 真实 GitHub smoke：PASS，读取 `zo-king` 6 个公开仓库，包含 `aic-multimodal-grounding` 和 `zoking-blog`，运行时错误为 0。
- 仓库 `preflight.ps1 -SkipE2E`：PASS，Go tests、Admin production build、Hugo build 全部通过。
- `git diff --check`：PASS。
- 运行态：`/`、`/archives/`、`/projects/`、`/categories/`、`/tags/` 均返回 200。

## 3. 证据

- `docs/process/evidence/github-p19-projects-live-1280x800.png`
- `docs/process/evidence/github-p19-projects-desktop-1280x800.png`
- `docs/process/evidence/github-p19-projects-mobile-390x844.png`
- `docs/process/evidence/github-p19-archives-desktop-1280x800.png`

## 4. 运行命令

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\github-projects-p19.mjs
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
```
