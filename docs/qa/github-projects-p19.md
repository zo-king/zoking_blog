# P19/P30 GitHub 项目页验收记录

更新时间：2026-07-15

## 自动化范围

`scripts/qa/github-project-sync-p30.mjs` 使用本地 HTTP fixture 验证：

- 定时同步携带运行时 Token，但输出文件不包含 Token。
- Fork/archived 过滤、推送时间排序和字段白名单。
- 非 GitHub URL 回退到账号主页。
- API 失败时退出非零且不覆盖最后一次成功快照。

`scripts/qa/github-projects-p19.mjs` 使用真实已提交快照验证：

- 项目页卡片数量、顺序、URL 与 `github_projects.json` 一致。
- 访客浏览器对 `api.github.com` 的请求数为 0。
- 旧 `githubProjects.ts`、加载中、403 和重试界面均不存在。
- 快照更新时间可见，外链 `rel` 安全。
- 1280x800、390x844、320x568 无横向溢出，移动端单列。

## 当前结果

| 检查 | 结果 |
|---|---|
| 真实 `zo-king` 快照 | 4 个公开非 Fork/归档仓库 |
| 同步脚本黑盒 | Pass |
| 静态项目页 Playwright | Pass |
| 访客 GitHub API 请求 | 0 |
| Hugo production/minify 61 pages | Pass |
| 390/320 响应式 | Pass |
| console/page error | 0 |

## 证据

- `docs/process/evidence/github-p30-projects-static-1280x800.png`
- `docs/process/evidence/github-p30-projects-static-390x844.png`

## 运行命令

```powershell
node .\scripts\qa\github-project-sync-p30.mjs
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\github-projects-p19.mjs
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
```
