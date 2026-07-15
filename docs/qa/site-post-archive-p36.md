# SITE-POST-ARCHIVE-P36-001 QA

## 运行

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\site-post-archive-p36.mjs
```

默认站点地址为 `http://localhost:1313`，可通过 `P36_SITE_BASE` 覆盖。

## 覆盖范围

- `/post/` 返回 200、唯一中文 H1 和正确总篇数。
- 年份倒序、年份导航与分组一一对应。
- 每年文章发布日期倒序，`time[datetime]` 可解析。
- `ol > li > article`、H1/H2/H3 与文章链接语义。
- 文章链接指向 `/p/`，分类与阅读时长不影响标题布局。
- 页面初始请求全部同源。
- 年份锚点更新 hash，允许动态时执行短促高亮。
- reduced-motion 下无年份动画。
- 无 JavaScript 时完整列表仍可读取。
- 1280、390、320 视口无横向溢出；移动标题触控目标至少 44px。
- console 和 page error 为零。

## 本轮结果

| 检查 | 结果 |
|---|---|
| P36 专项 Playwright | Pass |
| Hugo production build | Pass，63 pages |
| 1280/390/320 | Pass |
| 无 JavaScript | Pass |
| 同源请求边界 | Pass |
| P22 成果时间线回归 | Pass |
| P29 内容发现回归 | Pass |
| P34 打印/Feed 回归 | Pass |
| `preflight.ps1 -SkipE2E` | Pass，Go/Admin/Hugo |

P20 旧脚本要求运行中的成果 API 返回固定三条 2026 fixture；当前无 API 的站点预览使用其后继 P22 测试验证 2024 至今空状态、年份导航和响应式，不把外部运行态缺失误判为 P36 回归。

证据：

- `docs/process/evidence/site-p36-post-archive-desktop-1280x900.png`
- `docs/process/evidence/site-p36-post-archive-mobile-390x844.png`
