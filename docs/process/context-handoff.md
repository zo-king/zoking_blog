# 项目上下文交接

本文件是新窗口接手时的当前事实源，不是历史工作日志。更新时间：2026-07-15。

## 当前结论

- 工作区：`D:\zoking\zoking-blog`。
- `ADMIN-UI-P6-001`、`ADMIN-UX-P7-001`、`LIST-PAGINATION-P8-001` 与 `SITE-UX-P9-001` 均已完成，相关文件锁已释放。
- `PUBLISH-URL-P10-001` 至 `SITE-LAYOUT-P23-001` 的既有阶段均已完成；`SEC-RELIABILITY-P24-001` 也已完成并释放文件锁。
- P24 已完成 HttpOnly Cookie + CSRF + Origin/CORS 隔离，`POST /api/v1/admin/auth/session` 可恢复浏览器会话并轮换 CSRF；登录和恢复都不返回 JWT。
- P24 已统一 post/page/series/achievement/release 媒体引用锁，发布 Worker 周期恢复 stale job，并按数据库唯一 active release 对账 `current`；失败写入不会覆盖已确认的取消、重试或发布终态。
- 生产预览固定为独立 `https://preview.zoking.tech/preview-files`，配置校验禁止与站点、API、Admin 同源；生产 seed 必须使用 Compose `seed` service。
- 最新验收：全量 Go/PostgreSQL、vet、Admin build、npm 0 漏洞、Hugo 54 pages、Compose、migration Down/Up、26/26 HTTP 黑盒和完整隔离 E2E 均通过。记录见 `docs/qa/security-reliability-p24.md`。
- Admin 已全量迁移到 `@arco-design/web-react 2.66.15`，运行于 React `18.3.1`，路由使用 `react-router-dom 7.18.1`。
- `antd`、`@ant-design/icons` 和 React 19 compatibility patch 已移除，不要重新引入，也不要按旧 Ant Design 文档继续改造。
- 十个后台领域路由已完成中文化和 B 端工作台结构；文章与页面另有独立编辑子路由：`/posts/new`、`/posts/:postID/edit`、`/pages/new`、`/pages/:pageID/edit`。
- `/posts` 和 `/pages` 只显示列表，完整编辑器不再与列表同页；发布、账号权限、分类标签使用 Tabs，创建和详情使用 Modal/Drawer，低频清理进入维护入口。
- 侧栏只分“工作区/管理”两组；桌面文章/页面编辑器固定在视口内并使用内部滚动，设置页已压缩为两列配置。
- Admin 已形成 `router -> layout -> pages -> hooks/api/types` 的结构；菜单、路由访问和数据加载均按数据库实时 RBAC 权限处理，桌面侧栏与移动抽屉导航均可用。
- 1280x720 与 390x844 两种视口已完成 12 个路由的运行态验收：无异常 HTTP、无页面横向溢出；桌面全部单屏，移动普通页面单屏，文章/页面编辑约 2 屏。
- `useAdminData` 已按当前路由加载模块，刷新只刷新当前工作区；任一模块失败不再清空整个后台状态。
- 九类 Admin 列表已使用统一服务端分页：`page/page_size/q/status/sort` 写入 URL，响应包含 `total/total_pages`；文章和页面编辑直达按 ID 加载，不依赖当前列表页。
- 分页排序具有唯一 ID 兜底，越界页不会执行巨大 OFFSET；发布和评论列表不会通过关联预加载暴露完整正文。
- Admin `npm run build` 已通过；`npm audit` 为 0 漏洞。
- 当前不是“只完成后台壳”的阶段。内容、发布、预览、评论、媒体、审计和权限管理的全栈闭环均已落地。
- C 端已完成 Theme Stack 体验审计：中文评论、文章导航/分享/相关文章、中文 taxonomy 搜索、RSS/robots、JSON-LD、键盘暗色模式、skip link、系统字体和头像优化均已落地；P16 另新增阅读进度/30 天本地续读、搜索异常重试和标题权重、完整移动菜单键盘交互、深色 AA 对比度、中文 section 与唯一 landmark。
- C 端审计报告：`docs/frontend/site-ux-audit.md`；证据：`site-p9-home-desktop-1280x720.png` 与 `site-p9-article-mobile-390x844.png`。
- P16 功能调研：`docs/references/personal-site-feature-research.md`；QA：`docs/qa/site-audit-p16.md`；生产 Nginx 已返回真实 404 并启用 gzip。
- P17 已实现 Goldmark AST + HTML DOM 内容质量报告、文章/页面事务发布门禁、普通 CRUD published 绕过防护、active job 编辑锁，以及 Worker/Retry/全站发布复检。Admin 使用 380px/移动全宽 Drawer，发布顺序固定为检查、保存、publish。
- P18 已实现 PostgreSQL 独立系列模型、文章单系列正整数排序、Gin/GORM 系列 CRUD 与 RBAC、发布 front matter/质量门禁、Admin 系列管理及文章字段、Hugo 系列目录与系列内导航；开发库 migration 已执行且未写测试 fixture。
- P19/P30 的 `/projects/` 已改为静态仓库快照：`.github/workflows/sync-github-projects.yml` 每月 1 日、16 日北京时间 10:00 抓取 `zo-king` 公开非 Fork/归档仓库并提交 `apps/site/data/github_projects.json`；同步失败保留旧快照，访客侧不再请求 GitHub API。
- P20 已从一级导航和首页右栏移除显式搜索，保留 `/search/`、JSON 索引与 404 恢复能力；归档升级为语义化单列时间线，按发布日期倒序显示年份、数量、月日、摘要和可选小图，专项 Playwright 与视觉复核已通过。
- P21 已让关于页和友链页不再渲染相关文章；友链改为两列/移动单列卡片，头像按站点 favicon、DuckDuckGo favicon 服务、首字母三级回退；博客插件调研建议见 `docs/references/blog-plugin-research-p21.md`。
- P26 已将首页分页调整为每页 6 篇，并新增置顶个人介绍文章 `/p/about-zoking/`，使用现有 `/img/showcase/architecture.jpg` 封面；基础模板已加入一次会话开屏动画，支持跳过、Escape、sessionStorage 和 reduced-motion，详情见 `docs/qa/site-home-splash-p26.md`。
- P27 已用 MIT SpinKit `circle-fade` 思路替换 P26 原开屏视觉，只保留站名和八点淡入环并缩短为 1.4 秒；首页右栏新增 Pixiv 每日排行 iframe widget，限制前 10 项、380px 高、lazy/no-referrer/sandbox，明暗主题及 1280/1024/390 已验收，详情见 `docs/qa/site-visual-widget-p27.md`。
- P28 已把首页右栏固定为“大家抢着看 → Pixiv 每日排行 → 一言”：热门文章按本地 slug 白名单稳定解析，一言具备可视区懒加载、5 秒超时、30 分钟会话缓存、刷新和本地寄语降级；production/minify 61 pages 与 1280/1024/390 Edge headless 矩阵通过，详情及上传清单见 `docs/qa/site-sidebar-widgets-p28.md`。
- P29 已把友链升级为 9 项博客发现目录，支持站内搜索、分类筛选、仅在当前结果中随机拜访；全站新增 `Ctrl/Cmd+K` 快捷面板，可导航、搜索文章、随机文章和切换主题。首页文章与友链卡片只执行一次 220ms 原生入场动画，reduced-motion 完全关闭，未引入 Motion/Canvas 等依赖。

## 全栈能力

- 架构：PostgreSQL 是编辑源，Hugo content 是发布快照，Hugo + Theme Stack 构建 C 端静态站；API 为 Go Gin + GORM 的模块化单体。
- 内容：文章、分类、标签、独立页面和站点设置已有真实数据库、API 与 Admin 管理界面。
- 媒体：支持本地上传、预览、复制 URL、插入 Markdown、引用追踪和删除保护；孤立媒体清理默认 dry-run。
- 评论：支持公开匿名提交、后台审核/回复/删除和 C 端 approved 评论读取；API 失败不阻塞文章阅读。
- 发布：发布请求在行锁事务内完成质量检查、状态/媒体引用更新和异步 job 创建；worker 生成 Hugo release、manifest 和构建目录并二次复检；历史 release 支持 promote/rollback，active release 受唯一性与产物校验保护。
- 预览：文章、页面、临时站点设置均使用隔离预览，不写正式 content、不创建 release、不切换 active/current；TTL 访问控制、定时清理及 dry-run/apply 已完成。
- 设置与页面：页面 slug 使用根路径并校验 reserved slug；站点设置仅允许白名单字段，保存只落库，显式发布才生成 site release。
- 安全：认证后写请求审计、HMAC IP、数据库实时 RBAC、active user 检查、用户启停/角色分配、最后一个超级管理员保护、自定义角色 CRUD、权限矩阵和管理员密码重置均已完成；文章、页面和关联发布记录已按 global/owner scope 做对象级隔离。
- QA/运维：E2E smoke 覆盖登录、内容、taxonomy、媒体、页面、设置、预览、发布、评论、rollback 和清理 dry-run；preflight 串联 Go tests、Admin build、Hugo build、migrate/seed 与 E2E。
- QA 基线：`scripts/qa/whitebox.ps1` 最近记录覆盖率 `35.8%` 且 `go vet` 通过；`scripts/qa/http-blackbox.ps1` 默认 `21/21`、含限流 `22/22`；Preview 竞争、发布失败不变式、Admin 分页、内容对象隔离与 P17 发布门禁真实 SQL、Linux PostgreSQL `go test -race` 均通过；完整 E2E 使用 `zoking_blog_test`，P17 Playwright 使用 API 拦截且不写 development/production 数据。
- 清理策略：active release 永不清理；默认保留最新 20 个 inactive release 和 30 天内 release。媒体 grace period 默认 `168h`，批量大小默认 `100`。

## 本地凭据与地址

- 默认开发管理员：`admin@zoking.local` / `ChangeMe123!`。
- 上述账号仅用于本地 seed。生产部署必须替换默认密码、`JWT_SECRET` 和所有示例 secret。
- PostgreSQL：`localhost:15432`。
- API：`http://localhost:18080`，健康检查 `/healthz`，就绪检查 `/readyz`。
- Admin：`http://localhost:5173`。
- C 端：`http://localhost:1313`。
- 这些是约定地址，不代表接手时进程一定仍在运行；新窗口应先探测，再决定是否重启。

## 本地运行方式

在仓库根目录启动 PostgreSQL：

```powershell
docker compose -f infra/docker/compose.dev.yml up -d postgres
```

首次运行时创建本地环境文件，然后迁移、seed 并启动 API：

```powershell
if (-not (Test-Path .env)) { Copy-Item .env.example .env }
Set-Location apps/api
go run ./cmd/migrate up
go run ./cmd/seed
go run ./cmd/api
```

另开终端启动 Admin：

```powershell
Set-Location D:\zoking\zoking-blog\apps\admin
npm install
npm run dev
```

另开终端启动 C 端：

```powershell
Set-Location D:\zoking\zoking-blog
hugo server --source apps/site
```

若系统没有全局 Hugo，可使用仓库内工具：

```powershell
.\.tools\hugo\hugo.exe server --source apps/site
```

部署前完整检查：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1
```

只做无 E2E 的构建检查：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
```

独立白盒和只读 HTTP 黑盒：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\whitebox.ps1
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\http-blackbox.ps1
```

## 验收证据

- Admin 桌面：`docs/process/evidence/admin-arco-posts-desktop-1440x900.png`。
- Admin 移动端：`docs/process/evidence/admin-arco-posts-mobile-390x844.png`。
- Admin P7 列表：`docs/process/evidence/admin-ux-posts-list-desktop-1280x720.png`。
- Admin P7 编辑：`docs/process/evidence/admin-ux-post-editor-desktop-1280x720.png`、`docs/process/evidence/admin-ux-post-editor-mobile-390x844.png`。
- SEC-P15 权限证据：`docs/process/evidence/sec-p15-super-admin-posts-desktop-1280x720.png`、`docs/process/evidence/sec-p15-author-editor-desktop-1280x720.png`、`docs/process/evidence/sec-p15-author-editor-mobile-390x844.png`、`docs/process/evidence/sec-p15-viewer-posts-desktop-1280x720.png`；最终 Playwright run `ff53c1bde3e6`。
- C 端证据也位于 `docs/process/evidence/`。
- P16 C 端证据：`docs/process/evidence/site-p16-reading-progress-desktop.png`、`docs/process/evidence/site-p16-mobile-home-390x844.png`。
- P17 Admin 证据：`docs/process/evidence/content-quality-p17-desktop-1280x720.png`、`docs/process/evidence/content-quality-p17-mobile-390x844.png`；QA 记录为 `docs/qa/content-quality-p17.md`。
- P18 证据：`docs/process/evidence/series-p18-site-desktop-1280x800.png`、`docs/process/evidence/series-p18-site-mobile-390x844.png`、`docs/process/evidence/series-p18-admin-live-1280x720.png`；QA 记录为 `docs/qa/structured-series-p18.md`。
- P30 项目快照证据：`docs/process/evidence/github-p30-projects-static-1280x800.png`、`docs/process/evidence/github-p30-projects-static-390x844.png`；QA 记录为 `docs/qa/github-projects-p19.md` 与 `docs/qa/github-project-sync-p30.md`。
- P20 证据：`docs/process/evidence/archive-p20-desktop-1280x800.png`、`docs/process/evidence/archive-p20-dark-1280x800.png`、`docs/process/evidence/archive-p20-mobile-390x844.png`、`docs/process/evidence/archive-p20-mobile-320x568.png`；QA 记录为 `docs/qa/archive-timeline-p20.md`。
- P21 证据：`docs/process/evidence/about-p21-1280x800.png`、`docs/process/evidence/about-p21-390x844.png`、`docs/process/evidence/links-p21-1280x800.png`、`docs/process/evidence/links-p21-390x844.png`；QA 记录为 `docs/qa/about-links-p21.md`。
- Admin 最近确认的依赖版本以 `apps/admin/package-lock.json` 为准；不要根据旧工作日志恢复 React 19 或 Ant Design。

## 当前边界与下一步

当前没有已登记的 In Progress 任务。正式域名为 C 端 `https://zoking.tech/`、API `https://api.zoking.tech`、Admin `https://admin.zoking.tech`、Preview `https://preview.zoking.tech`。P24、P26、P27、P28、P29 与 P30 已完成，新窗口不要重复实现。代码基线为新仓库 `https://github.com/zo-king/zoking_blog` 的 `main` 分支；旧 `zoking-blog` 仓库及 Draft PR 已删除，不再作为接力依据。第三方端点不可用时一言保留本地寄语、Pixiv iframe 自身失败，GitHub 项目保留最后一次成功快照，都不应阻塞本站正文阅读。

当前 Go 位于 `E:\Editor\go`，版本 `go1.25.4`，Windows 本机 `CGO_ENABLED=0`；Linux race 已通过 `golang:1.25-bookworm` 容器并连接 `zoking_blog_test` 执行，CI 也已配置 PostgreSQL race job。seed 双进程首次初始化、Preview finish/fail 终态竞争和发布失败不变式均已完成隔离数据库验证。

建议按以下顺序选择并登记下一项工作：

1. 优先实施已登记的 `MEDIA-RECOVERY-P25-001`：持久化 media operation/manifest、pending upload 与 quarantine 启动 reconciliation、COMMIT ACK 丢失确认、Toxiproxy 和进程终止测试。
2. 进入真实生产部署前，按 runbook 配置四个域名的 DNS/TLS、替换 secret、迁移/seed、更新已有数据库站点设置，并完成公网 Preview Host 隔离、备份恢复和回滚演练。
3. 实现生产对象存储/CDN storage adapter、迁移任务和回源配置；当前 local volume 是已验证的开发与单机生产基线。

以上是后续增强，不是当前 Admin 迁移的阻塞项。开始新任务前，必须先在 `docs/process/task-board.md` 登记任务与文件锁，并在 `docs/process/worklog.md` 记录实施结果。

## 关键不变量

- 不要清空目录、重新克隆或覆盖当前仓库。
- 不要大规模修改上游 Theme Stack 核心源码；优先使用 `apps/site` 的配置、内容和本地 override。
- 发布预览与正式 release/current 必须继续隔离。
- 媒体物理删除前必须确认无编辑态和 release 态引用。
- 清理 API 默认 dry-run；apply 必须显式请求，且 active release 永远受保护。
- 前端权限裁剪只改善体验，授权真相必须继续由 API 和数据库实时 RBAC 决定。
- 多 agent 写任务必须先分配互斥文件范围；遇到他人改动时保留并协同，不得回退。

## 新窗口启动清单

1. 阅读本文档。
2. 阅读 `docs/process/worklog.md` 的顶部摘要和最新记录。
3. 阅读 `docs/process/task-board.md`，确认当前任务和文件锁。
4. 运行 `git status --short --branch`，识别并保留已有改动。
5. 探测 `5173`、`18080`、`1313` 和 PostgreSQL `15432`，不要盲目重复启动服务。
6. 选择下一项任务，登记任务编号、Owner、范围和文件锁后再实施。
7. 首次汇报中明确本窗口的任务编号、写入范围、验证范围和不会触碰的边界。
