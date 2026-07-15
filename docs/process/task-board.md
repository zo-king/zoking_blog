# 当前任务看板与文件锁

本文记录实际实施中的任务状态、文件锁和交接信息。它是 `docs/plan/work-breakdown-structure.md` 的运行时看板，不替代 WBS。

## 1. 状态枚举

所有任务必须使用以下状态：

| 状态 | 含义 |
|---|---|
| Backlog | 已记录，未准备实施 |
| Ready | 依赖满足，可派发 |
| Assigned | 已分配给中心窗口或子 agent |
| In Progress | 正在执行 |
| Review | 已回报，等待中心窗口验收 |
| Blocked | 因决策、依赖、冲突或验证失败暂停 |
| Done | 中心窗口验收完成，日志已更新 |
| Superseded | 被新任务替代，不再执行 |

状态流转：

```text
Backlog -> Ready -> Assigned -> In Progress -> Review -> Done
                         |          |            |
                         v          v            v
                      Blocked    Blocked      Blocked
```

## 2. 当前任务总览

| 任务编号 | 状态 | Owner | Agent | 文件锁 | 阻塞 | 下一步 |
|---|---|---|---|---|---|---|
| DOC-P0-002 | Done | Center | Center + Explorer agents | 已释放 | 无 | 已补齐工程实施与多 agent 调控文档 |
| ARCH-P0-001 | Done | Center | Center | 已释放 | 无 | 默认采用 Hybrid CMS、React Admin、PostgreSQL、Docker Compose 起步 |
| OPS-P0-001 | Done | Center | Center | 已释放 | 无 | 已创建 monorepo 骨架和本地 compose |
| API-P0-001 | Done | Center | Center + Backend explorer | 已释放 | 无 | 已初始化 Gin/GORM API、healthz/readyz/auth/posts |
| DB-P0-001 | Done | Center | Center + Backend explorer | 已释放 | 无 | 已初始化 goose migration 和 seed |
| HUGO-P0-001 | Done | Center | Center | 已释放 | 无 | 已整理 Stack demo 到 `apps/site` 并通过 Hugo build |
| ADMIN-P0-001 | Done | Center | Center | 已释放 | 无 | 已初始化 React/Vite 后台壳，原 Ant Design 基线后续由 ADMIN-UI-P6-001 替代 |
| PUB-P1-001 | Done | Center | Center + API | 已释放 | 无 | 同步写 Hugo content 的最小发布闭环已完成，后续演进 publish_jobs / release manifest / rollback |
| DB-P1-002 | Done | Center | Center + API | 已释放 | 无 | posts/categories/tags/post_categories/post_tags 已落库 |
| API-P1-003 | Done | Center | Center + API | 已释放 | 无 | 文章 CRUD 已接入 taxonomy 关系与 front matter 输出 |
| API-P1-004 | Done | Center | Center + API | 已释放 | 无 | 分类/标签 CRUD、公开列表与后台管理已可用 |
| ADMIN-P2-003 | Done | Center | Center + Admin | 已释放 | 无 | 文章列表/编辑器已接入真实 API、taxonomy 选择与管理入口已补齐 |
| DB-P1-003 | Done | Center | Center | 已释放 | 无 | media_assets/media_usages 已落库 |
| API-P1-005 | Done | Center | Center | 已释放 | 无 | 媒体上传、列表、详情、删除和静态访问已可用 |
| ADMIN-P2-006 | Done | Center | Center | 已释放 | 无 | Admin 媒体库已支持上传、预览、复制 URL、插入 Markdown |
| DB-P1-004 | Done | Center | Center | 已释放 | 无 | comments 已落库 |
| API-P1-006 | Done | Center | Center | 已释放 | 无 | 公开评论提交/读取、后台列表/审核/回复/删除 API 已可用 |
| ADMIN-P2-007 | Done | Center | Center | 已释放 | 无 | Admin 评论审核列表与审核操作已接入 |
| PUB-P3-001 | Done | Center | Center | 已释放 | 无 | 发布 job/release 数据视图与同步任务化发布入口已完成 |
| PUB-P3-005 | Done | Center | Center | 已释放 | 无 | 发布任务状态机最小版已完成 requested/building/verifying/published/failed |
| PUB-P3-006 | Done | Center | Center | 已释放 | 无 | release manifest、active release 记录与构建产物目录已完成 |
| PUB-P3-007 | Done | Center | Center | 已释放 | 无 | release promote/rollback API、Admin 操作与真实切换 smoke 已完成 |
| HUGO-P3-001 | Done | Center | Singer + Center | 已释放 | 无 | Public Comments API 已嵌入 C 端 Stack 文章页，主控验收通过 |
| PUB-P3-008 | Done | Center | Center + Aristotle | 已释放 | 无 | 同步发布已拆为异步 job + publish worker，API 返回 202 入队 |
| QA-P4-001 | Done | Center | Center + Heisenberg | 已释放 | 无 | 端到端冒烟脚本与 QA 文档已完成并通过本地验证 |
| OPS-P4-001 | Done | Center | Center | 已释放 | 无 | API/worker/Admin Dockerfile、生产 compose、env 模板与部署 runbook 已完成并通过镜像构建验证 |
| PUB-P5-001 | Done | Center | Center + Leibniz | 已释放 | 无 | sitemap verifier 已递归解析所有 sitemap `<loc>`，E2E 不再 warning |
| MEDIA-P5-001 | Done | Center | Center + Leibniz | 已释放 | 无 | 文章/release 媒体引用已写入 `media_usages`，Admin 展示引用数并保护删除 |
| SETTINGS-P5-001 | Done | Center | Center + Ptolemy | 已释放 | 无 | 站点设置与页面管理真实 API/Admin/发布 smoke 已完成 |
| CI-P5-001 | Done | Center | Center | 已释放 | 无 | 部署前 preflight 脚本、GitHub Actions 工作流与 QA 文档已完成并通过本地验证 |
| MEDIA-P5-002 | Done | Center | Center + Huygens | 已释放 | 无 | release 保留期、孤立媒体清理、Admin 操作、E2E/preflight 验收和对象存储/CDN 策略已完成 |
| PREVIEW-P5-001 | Done | Center | Center + Euler | 已释放 | 无 | 文章/页面/设置隔离预览 API、Admin、E2E 与生产配置已完成 |
| SITE-P5-003 | Done | Center | Center | 已释放 | 无 | C 端版心居中、隐藏左栏滚动条、单一中文化并重建三篇配图展示文章 |
| AUDIT-P5-001 | Done | Center | Center + Avicenna | 已释放 | 无 | 后台写请求审计、HMAC IP、查询 API、Admin 审计表与成功/失败验证已完成 |
| SEC-P5-001 | Done | Center | Center + Popper | 已释放 | 无 | 数据库实时 RBAC、系统角色 seed、active user 检查、auth/me 权限与 403 审计已完成 |
| USER-RBAC-P5-001 | Done | Center | Center + Descartes | 已释放 | 无 | 用户创建/启停/角色分配、角色读取、最后超级管理员保护与 Admin 界面已完成 |
| PREVIEW-CLEANUP-P5-001 | Done | Center | Center + Aquinas | 已释放 | 无 | 严格 TTL 访问、定时清理、dry-run/apply、Admin 操作和安全 claim 已完成 |
| ROLE-MANAGE-P5-001 | Done | Center | Center | 已释放 | 无 | 自定义角色 CRUD、权限矩阵和管理员密码重置均已完成并通过运行态闭环验证 |
| ADMIN-ROUTER-P5-001 | Done | Center | Center | 已释放 | 无 | 十个业务页面、API Client、真实路由、移动导航、RBAC 菜单、types/layout/hooks 均已完成 |
| ADMIN-UI-P6-001 | Done | Center | Center + 4 Workers | 已释放 | 无 | Arco 全量迁移、10 路由重构、React Router 安全升级、桌面/移动端验收和证据归档均已完成 |
| ADMIN-UX-P7-001 | Done | Center | Center + 5 Workers + Research/Explorer | 已释放 | 无 | 真实 CMS 信息架构、文章/页面编辑子路由、首屏工作区、低频操作收纳、路由级加载与桌面/移动验收完成 |
| LIST-PAGINATION-P8-001 | Done | Center | Center + Backend/Frontend/QA agents | 已释放 | 无 | Admin 九类列表服务端分页、URL 状态恢复、稳定排序、越界保护和回归已完成 |
| SITE-UX-P9-001 | Done | Center | Center + UX/QA agents | 已释放 | 无 | C 端阅读、导航、中文、SEO、a11y 与性能审计修复完成 |
| PUBLISH-URL-P10-001 | Done | Center | Center + Backend/Ops agents | 已释放 | 无 | zoking.tech/api.zoking.tech、生产发布与历史 release URL 防护已完成 |
| AUDIT-P11-001 | Done | Center | Center + Backend/Ops audit agents | 已释放 | 无 | 生产发布、部署、seed、preview、评论限流与桌面/移动运行态自检完成 |
| QA-P12-001 | Done | Center | Center + QA agents | 已释放 | 无 | 白盒 23.8%、HTTP 黑盒 15/15 与 16/16、隔离完整 E2E、构建预检和缺陷整改均通过 |
| QA-P13-001 | Done | Center | Center + 3 QA agents | 已释放 | 无 | Preview/PostgreSQL 终态竞争、可信代理、发布失败不变式、Linux race CI 与完整回归均通过 |
| QA-P14-001 | Done | Center | Center + 3 QA agents | 已释放 | 无 | Admin 文章/媒体/审计分页 PostgreSQL 集成、过滤契约修复、Linux race 与完整回归均通过 |
| SEC-P15-001 | Done | Center | Center + Security/QA agents | 已释放 | 无 | author owner scope、viewer 全局只读、发布记录收敛、权限感知 Admin、PostgreSQL/Playwright/完整回归均通过 |
| SITE-AUDIT-P16-001 | Done | Center | Center + Audit/Research/QA agents | 已释放 | 无 | 调研、阅读续读、搜索/评论/a11y、生产 404/gzip、Playwright/preflight 均已完成 |
| CONTENT-QUALITY-P17-001 | Done | Center | Center + API/Admin/QA agents | 已释放 | 无 | 草稿报告、事务发布门禁、Worker/Retry/站点复检、Admin Drawer 和完整验收均已完成 |
| SERIES-P18-001 | Done | Center | Center + API/Admin/Site agents | 已释放 | 无 | 独立系列模型、文章单系列显式排序、Admin 管理、C 端系列目录导航、真实 PostgreSQL/Playwright/preflight 验收均已完成 |
| SITE-GITHUB-P19-001 | Done | Center | Center + Site/UX/QA agents | 已释放 | 无 | 一级内容发现入口已收敛，`zo-king` GitHub 项目页、异常回退、隐私边界和桌面/移动验收均完成 |
| SITE-TIMELINE-P20-001 | Done | Center | Center + Research/UX/QA agents | 已释放 | 无 | 搜索入口降级、归档时间线、响应式、可访问性、兼容路由与仓库回归均已完成 |
| SITE-ABOUT-PLUGIN-P21-001 | Done | Center | Center + Research agents | 已释放 | 无 | 关于/友链相关文章已清理，友链头像与响应式布局、插件调研、专项回归和仓库预检均完成 |
| ACHIEVEMENTS-P22-001 | Done | Center | Center + Backend/Admin/Site/Content agents | 已释放 | 无 | 成果全栈 CRUD/发布、时间线、GitHub 头像、技术文章与专项 QA 已完成 |
| SITE-LAYOUT-P23-001 | Done | Center | Center | 已释放 | 无 | 参考 mok.moe 完成 C 端密度微调与响应式验收；装饰封面已按用户反馈移除 |
| SEC-RELIABILITY-P24-001 | Done | Center | Center + Mill + Curie + Hilbert | 已释放 | 无 | Cookie/CSRF、媒体引用锁、发布恢复、域名隔离、PostgreSQL/黑盒/E2E 与文档已验收 |
| MEDIA-RECOVERY-P25-001 | Backlog | 未分配 | Center + Backend/Chaos agents | 未锁定 | 需要新增持久化操作模型与进程级故障注入 | 设计 media_operations/manifest、pending upload/quarantine reconciliation、Commit ACK 丢失确认及 Toxiproxy 测试 |
| SITE-HOME-SPLASH-P26-001 | Done | Center | Center | 已释放 | 无 | 首页每页 6 篇、置顶个人介绍文章、简约会话开屏动画与桌面/移动黑盒验收已完成 |
| SITE-VISUAL-WIDGET-P27-001 | Done | Center | Center | 已释放 | 无 | SpinKit 开屏替换、Pixiv 日榜右栏组件、明暗主题与 1280/1024/390 验收已完成 |
| SITE-SIDEBAR-WIDGETS-P28-001 | Done | Center | Center + Site/QA agents | 已释放 | 无 | “大家抢着看”、Pixiv 日榜、一言插件、降级边界和测试矩阵已完成；当前项目以新仓库 `main` 为发布基线 |
| SITE-DISCOVERY-P29-001 | Done | Center | Center | 已释放 | 无 | 友链搜索/分类/随机拜访、全站快捷面板、低干扰入场动画与专项回归已完成 |
| SITE-GITHUB-SNAPSHOT-P30-001 | Done | Center | Center | 已释放 | 无 | GitHub 项目改为每月 1/16 日静态快照、失败保留旧数据、零访客 API 请求与桌面/移动验收已完成 |
| SITE-NAVIGATION-NOW-P31-001 | Done | Center | Center + Research/Audit agents | 已释放 | 无 | 博客组件调研、原生跨页过渡、近况页、reduced-motion 与桌面/移动验收已完成 |
| SITE-READING-SETTINGS-P32-001 | Done | Center | Center | 已释放 | 无 | 文章页字号/行距/链接设置、预绘制恢复、本地持久化、重置和桌面/移动验收已完成 |
| SITE-ARTICLE-UTILITIES-P33-001 | Done | Center | Center + Research/Audit agents | 已释放 | 无 | 章节链接真实复制、代码语言工具栏、失败恢复、reduced-motion 与桌面/移动验收已完成 |
| SITE-PRINT-DISCOVERY-P34-001 | Done | Center | Center + Research/Audit agents | 已释放 | 无 | 文章打印版式、RSS 自动发现、作者结构化语义与 A4/深色/无 JS 验收已完成 |

## 3. 文件锁登记

| 锁编号 | 任务编号 | Agent | 允许编辑 | 禁止编辑 | 锁定时间 | 释放条件 |
|---|---|---|---|---|---|---|
| LOCK-DOC-P0-002 | DOC-P0-002 | Center | `docs/*` | 主题源码、Go/API/Admin 实现代码 | 2026-06-18 | 已释放 |
| LOCK-PHASE0-BASELINE | OPS/API/DB/HUGO/ADMIN-P0 | Center | `apps/*`, `db/*`, `infra/*`, `scripts/*`, root config/docs | 主题核心源码删除/重写 | 2026-07-10 | 已释放 |
| LOCK-MEDIA-P1 | DB-P1-003/API-P1-005/ADMIN-P2-006 | Center | `apps/api/*`, `apps/admin/*`, `db/migrations/*`, `storage/*`, docs runtime files | 主题核心源码删除/重写 | 2026-07-10 | 已释放 |
| LOCK-COMMENT-P1 | DB-P1-004/API-P1-006/ADMIN-P2-007 | Center | `apps/api/*`, `apps/admin/*`, `db/migrations/*`, docs runtime files | 主题核心源码删除/重写 | 2026-07-10 | 已释放 |
| LOCK-PUBLISH-P3 | PUB-P3-001/PUB-P3-005/PUB-P3-006 | Center | `apps/api/*`, `apps/admin/*`, `db/migrations/*`, `apps/site/content/*`, `dist/*`, docs runtime files | 主题核心源码删除/重写 | 2026-07-10 | 已释放 |
| LOCK-ROLLBACK-P3 | PUB-P3-007 | Center | `apps/api/*`, `apps/admin/*`, `dist/*`, docs runtime files | 主题核心源码删除/重写 | 2026-07-10 | 已释放 |
| LOCK-HUGO-COMMENTS-P3 | HUGO-P3-001 | Singer + Center | `apps/site/layouts/*`, `apps/site/assets/*`, `apps/site/config/*`, docs runtime files | 根主题核心源码删除/重写、API/Admin 发布中心文件 | 2026-07-10 | 已释放 |
| LOCK-PUBLISH-WORKER-P3 | PUB-P3-008 | Center + Aristotle | `apps/api/*`, `apps/admin/*`, `.env.example`, `scripts/dev/README.md`, docs runtime files | 主题核心源码删除/重写、无关业务重构 | 2026-07-10 | 已释放 |
| LOCK-QA-E2E-P4 | QA-P4-001 | Center + Heisenberg | `scripts/qa/*`, `docs/qa/*`, docs runtime files | API/Admin/站点功能代码大改 | 2026-07-10 | 已释放 |
| LOCK-OPS-P4 | OPS-P4-001 | Center | `apps/api/Dockerfile`, `apps/admin/*`, `infra/docker/*`, `.dockerignore`, `README.md`, docs runtime files | 业务功能大改、主题核心源码删除/重写 | 2026-07-10 | 已释放 |
| LOCK-PUB-MEDIA-P5 | PUB-P5-001/MEDIA-P5-001 | Center | `apps/api/internal/publisher/*`, `apps/api/internal/mediaref/*`, `apps/api/internal/httpapi/*`, `apps/api/internal/model/media.go`, `apps/admin/src/App.tsx`, `scripts/qa/*`, docs runtime files | 主题核心源码删除/重写、无关业务重构 | 2026-07-10 | 已释放 |
| LOCK-SETTINGS-P5 | SETTINGS-P5-001 | Center | `apps/api/internal/model/*`, `apps/api/internal/httpapi/*`, `apps/api/internal/publisher/*`, `apps/api/cmd/seed/main.go`, `apps/admin/src/*`, `db/migrations/*`, `scripts/qa/*`, docs runtime files | 主题核心源码删除/重写、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-CI-P5 | CI-P5-001 | Center | `scripts/qa/*`, `.github/workflows/*`, `docs/qa/*`, `README.md`, docs runtime files | 业务功能大改、主题核心源码删除/重写 | 2026-07-11 | 已释放 |
| LOCK-MEDIA-CLEANUP-P5 | MEDIA-P5-002 | Center | `apps/api/internal/config/*`, `apps/api/internal/maintenance/*`, `apps/api/internal/httpapi/*`, `apps/admin/src/*`, `scripts/qa/*`, `infra/docker/*`, docs runtime files | 主题核心源码删除/重写、发布核心语义破坏、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-PREVIEW-P5 | PREVIEW-P5-001 | Center | `apps/api/internal/config/*`, `apps/api/internal/publisher/*`, `apps/api/internal/httpapi/*`, `apps/admin/src/*`, `scripts/qa/*`, `infra/docker/*`, docs runtime files | 主题核心源码删除/重写、正式 release promote/current 语义破坏、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-AUDIT-P5 | AUDIT-P5-001 | Center | `apps/api/internal/model/*`, `apps/api/internal/audit/*`, `apps/api/internal/httpapi/*`, `apps/admin/src/*`, `db/migrations/*`, `scripts/qa/*`, docs runtime files | 主题核心源码删除/重写、认证 token/密码/文章正文写入审计详情、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-SEC-RBAC-P5 | SEC-P5-001 | Center | `apps/api/internal/authz/*`, `apps/api/internal/httpapi/*`, `apps/api/cmd/seed/*`, `apps/admin/src/*`, `scripts/qa/*`, docs runtime files | 主题核心源码删除/重写、破坏 super_admin 访问、把权限信任放在前端、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-USER-RBAC-P5 | USER-RBAC-P5-001 | Center | `apps/api/internal/httpapi/*`, `apps/api/internal/model/*`, `apps/api/cmd/seed/*`, `apps/admin/src/*`, `scripts/qa/*`, docs runtime files | 主题核心源码删除/重写、删除最后有效超级管理员、记录明文密码、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-PREVIEW-CLEANUP-P5 | PREVIEW-CLEANUP-P5-001 | Center | `apps/api/internal/config/*`, `apps/api/internal/maintenance/*`, `apps/api/internal/httpapi/*`, `apps/api/cmd/api/*`, `apps/api/cmd/worker/*`, `apps/admin/src/*`, `infra/docker/*`, scripts/docs runtime files | 正式 release/current、building 预览、主题核心源码、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-ROLE-MANAGE-P5 | ROLE-MANAGE-P5-001 | Center | `apps/api/internal/httpapi/*`, `apps/admin/src/*`, scripts/docs runtime files | 修改/删除系统角色、明文密码进入日志或审计、无关业务重构 | 2026-07-11 | 已释放 |
| LOCK-ADMIN-UI-P6 | ADMIN-UI-P6-001 | Center + 4 Workers | `apps/admin/*`, `docs/frontend/*`, docs runtime files | API/数据库/Hugo 发布语义、C 端主题核心、无关业务重构 | 2026-07-12 | 已释放：build、审计、路由/RBAC/核心交互、1440px/390px 验收均通过 |
| LOCK-ADMIN-UX-P7 | ADMIN-UX-P7-001 | Center + UX/Research agents | `apps/admin/src/*`, `docs/frontend/*`, docs runtime files | API/数据库/Hugo 发布语义、删除既有业务能力、无关后端重构 | 2026-07-12 | 已释放：12 路由完成 1280px/390px 验收，build、preflight、核心交互和依赖审计通过 |
| LOCK-LIST-PAGINATION-P8 | LIST-PAGINATION-P8-001 | Center + Backend/Frontend/QA agents | `apps/api/internal/httpapi/*`, `apps/admin/src/*`, `scripts/qa/*`, API/frontend/docs tests | 发布状态机、数据库 schema、Hugo 主题、无关业务重构 | 2026-07-12 | 已释放 |
| LOCK-SITE-UX-P9 | SITE-UX-P9-001 | Center + UX/QA agents | `apps/site/config/*`, `apps/site/layouts/*`, `apps/site/assets/*`, `apps/site/content/*`, C 端 QA 与过程文档 | API/Admin、数据库 schema、上游主题核心大改 | 2026-07-12 | 已释放 |
| LOCK-PUBLISH-URL-P10 | PUBLISH-URL-P10-001 | Center + Backend/Ops agents | `apps/api/internal/publisher/*`, `apps/api/internal/httpapi/settings.go`, `apps/api/cmd/seed/*`, `apps/site/config/*`, `.env.example`, `infra/docker/*`, deployment/process docs and tests | Admin UI、数据库 schema、无关业务重构 | 2026-07-13 | 已释放：生产 URL/产物/历史 release 校验、预览豁免、配置传播与 preflight 均通过 |
| LOCK-AUDIT-P11 | AUDIT-P11-001 | Center + Backend/Ops audit agents | URL/preview/seed/rate-limit 实现与测试、生产 Compose/runbook、过程文档 | Admin 业务 UI、数据库 schema、上游主题核心、无关业务重构 | 2026-07-13 | 已释放：代码审计整改、构建、静态检查、浏览器矩阵与运行态探测均完成 |
| LOCK-QA-P12 | QA-P12-001 | Center + QA agents | `apps/api/**/*_test.go`, `scripts/qa/*`, `docs/qa/*`, process docs；测试暴露缺陷时允许最小实现修复 | Admin 业务 UI、数据库 schema、上游主题核心、破坏性生产数据操作 | 2026-07-13 | 已释放：白盒/黑盒、隔离 E2E、preflight、缺陷整改和日志全部完成 |
| LOCK-QA-P13 | QA-P13-001 | Center + 3 QA agents | `apps/api/internal/httpapi/preview*`, `apps/api/internal/publisher/*job*`, `.github/workflows/*`, `scripts/qa/*`, `docs/qa/*`, process docs；测试暴露缺陷时允许最小实现修复 | Admin 业务 UI、数据库 schema、上游主题核心、development/production 数据库破坏性操作 | 2026-07-13 | 已释放：Preview/代理、发布不变式、PostgreSQL race CI、全量回归和日志完成 |
| LOCK-QA-P14 | QA-P14-001 | Center + 3 QA agents | `apps/api/internal/httpapi/*pagination*_test.go`, PostgreSQL test helper, pagination handler tests, `docs/qa/*`, process docs；测试暴露缺陷时允许最小 handler 修复 | Admin UI、数据库 migrations、Hugo 主题、development/production 数据库写入、无关业务重构 | 2026-07-13 | 已释放：PostgreSQL 分页集成用例、缺陷整改、race/preflight、运行态验收和日志完成 |
| LOCK-SEC-P15 | SEC-P15-001 | Center + Security agents | `apps/api/cmd/seed/*`, `apps/api/internal/httpapi/*`, `apps/admin/src/App.tsx`, `apps/admin/src/pages/{PostsPage,PagesPage,PublishingPage,TaxonomyPage,MediaPage,CommentsPage,SettingsPage}.tsx`, `apps/admin/src/hooks/{useAdminData,useContentAdminCommands,useEditorialCommands}.ts`, `apps/admin/src/types/admin.ts`, `apps/admin/src/labels.ts`, `scripts/qa/content-object-access-ui.*`, `docs/security/*`, `docs/qa/*`, process docs；仅允许对象访问策略、权限感知 UI、最小权限 seed、隔离测试和测试暴露的最小修复 | 数据库 migrations、Hugo 主题、development/production 数据库写入、无关业务重构 | 2026-07-13 | 已释放：对象范围、Admin 权限裁剪、PostgreSQL 三连跑、Playwright、race/preflight/E2E、残留审计和日志完成 |
| LOCK-SITE-AUDIT-P16 | SITE-AUDIT-P16-001 | Center + QA agents | `apps/site/assets/ts/*`, `apps/site/assets/scss/custom.scss`, `apps/site/layouts/page/search.html`, `apps/site/layouts/_partials/{article,comments,sidebar}/*`, `apps/site/content/{post,page}/_index.md`, `assets/ts/search.tsx`, `infra/docker/site.nginx.conf`, `scripts/qa/site-reader-ui.mjs`, `docs/references/*`, `docs/qa/*`, process docs；仅允许本地阅读状态、菜单交互、搜索/评论可访问性、深色对比度、中文 section、生产 404/gzip、自动测试和文档 | API/Admin、数据库 schema、development/production 数据写入、上游主题无关重构 | 2026-07-13 | 已释放：Hugo、7 组 Playwright、Nginx 404/gzip、仓库 preflight 和过程文档均通过 |
| LOCK-CONTENT-QUALITY-P17 | CONTENT-QUALITY-P17-001 | Center + API/Admin/QA agents | `apps/api/go.{mod,sum}`, `apps/api/internal/contentquality/*`, `apps/api/internal/httpapi/{quality.go,router.go,pages.go,preview.go,settings.go,rbac.go,*quality*_test.go,rbac_test.go}`, `apps/api/internal/publisher/{quality.go,worker.go,job.go,*quality*_test.go}`, `apps/admin/src/{App.tsx,styles.css}`, `apps/admin/src/api/client.ts`, `apps/admin/src/components/ContentQualityPanel.tsx`, `apps/admin/src/pages/{PostsPage,PagesPage}.tsx`, `apps/admin/src/hooks/useEditorialCommands.ts`, `apps/admin/src/types/admin.ts`, `scripts/qa/*quality*`, `docs/{backend,frontend,qa}/*`, process docs；仅允许内容质量静态检查、发布门禁、紧凑面板和对应测试 | 数据库 migrations、Hugo C 端主题、development/production 数据写入、外部 URL 抓取、无关发布状态机重构 | 2026-07-13 | 已释放：单元、真实 PostgreSQL、权限/并发/零副作用、Admin build、桌面/移动 Playwright、preflight 和运行态均通过 |
| LOCK-SERIES-P18 | SERIES-P18-001 | Center + API/Admin/Site agents | API/DB：`db/migrations/*series*`, `apps/api/internal/model/{post,series}.go`, `apps/api/internal/httpapi/{series,posts,post_helpers,router,rbac}*.go` 及系列测试；Admin：`apps/admin/src/{App.tsx,styles.css}`, `apps/admin/src/pages/{TaxonomyPage,PostsPage}.tsx`, `apps/admin/src/hooks/{useAdminData,useContentAdminCommands,useEditorialCommands}.ts`, `apps/admin/src/{api/client,types/admin}.ts`；Site：`apps/site/layouts/_partials/article/components/*series*`, `apps/site/layouts/_partials/article/components/navigation.html`, `apps/site/assets/scss/custom.scss`；Center：`apps/api/internal/{contentquality/*,httpapi/quality.go,publisher/*}`, `apps/site/content/post/*/index.md` 演示系列元数据、共享集成测试、P18 QA/设计文档与 process docs | development/production 数据写入、删除被引用系列、上游主题无关重构、拖拽排序、多系列归属、破坏现有普通文章导航 | 2026-07-13 | 已释放：PostgreSQL 契约/隔离、发布门禁、Admin 与 C 端 Playwright、仓库 preflight、开发库迁移和真实运行态联调均通过 |
| LOCK-SITE-GITHUB-P19 | SITE-GITHUB-P19-001 | Center + Site/UX/QA agents | `apps/site/content/{categories,tags}/_index.md`, `apps/site/content/page/{archives,projects,links}/*`, `apps/site/layouts/{archives,projects}.html`, `apps/site/assets/{ts,scss}/*github*`, `apps/site/config/_default/{params,menu}.toml`, P19 C 端 QA/设计文档、`scripts/qa/*github*` 与 process docs | API/Admin、数据库 schema、文章正文、上游主题核心、运行态业务数据 | 2026-07-13 | 已释放：Hugo 46 pages、Mock/真实 GitHub、成功/空/403/重试、缓存安全、1280/390/320 响应式和 preflight 均通过 |
| LOCK-SITE-TIMELINE-P20 | SITE-TIMELINE-P20-001 | Center + Research/UX/QA agents | `apps/site/content/page/{search,archives}/index.md`, `apps/site/config/_default/params.toml`, `apps/site/layouts/{archives.html,_partials/article-list/timeline.html}`, `apps/site/assets/scss/archive-timeline.scss`, `scripts/qa/{archive-timeline-p20,github-projects-p19}.mjs`, P20 C 端调研/设计/QA 文档与 process docs | API/Admin、数据库 schema、文章正文、上游主题核心、运行态业务数据 | 2026-07-13 | 已释放：Hugo 46 pages、P20/P19 Playwright、1280/390/320、明暗主题、兼容路由、preflight 和文档均通过 |
| LOCK-SITE-ABOUT-PLUGIN-P21 | SITE-ABOUT-PLUGIN-P21-001 | Center + Research agents | `apps/site/layouts/single.html`, `apps/site/layouts/_partials/article/components/links.html`, `apps/site/assets/ts/linksAvatars.ts`, `apps/site/assets/scss/custom.scss`, `scripts/qa/about-links-p21.mjs`, P21 插件调研/设计/QA 文档与 process docs | API/Admin、数据库 schema、文章正文、上游主题核心、运行态业务数据、第三方插件安装 | 2026-07-13 | 已释放：Hugo 46 pages、P21/P20 Playwright、头像三级回退、1280/390 响应式、preflight 和文档均通过 |
| LOCK-P22-BACKEND | ACHIEVEMENTS-P22-001 | Backend Worker | `db/migrations/*achievement*`, `apps/api/internal/model/*achievement*`, `apps/api/internal/httpapi/*achievement*`, achievement RBAC/router tests | Admin、Hugo 模板与样式、文章正文、非成果发布状态机 | 2026-07-13 | 已释放：Backend 契约、PostgreSQL 测试与中心验收完成 |
| LOCK-P22-PUBLISHER | ACHIEVEMENTS-P22-001 | Publisher Worker | `apps/api/internal/publisher/*achievement*`, 成果发布快照测试；共享发布入口仅输出补丁建议由 Center 集成 | Admin、Hugo UI、数据库 migration、非成果发布语义 | 2026-07-13 | 已释放：发布快照、重试/回滚不变式与中心验收完成 |
| LOCK-P22-ADMIN | ACHIEVEMENTS-P22-001 | Admin Worker | `apps/admin/src/pages/*Achievement*`, `apps/admin/src/components/*Achievement*`, achievement 专属 hook/类型/样式；共享路由与 client 仅输出补丁建议由 Center 集成 | API、数据库、Hugo、文章内容 | 2026-07-13 | 已释放：桌面/移动后台成果管理验收完成 |
| LOCK-P22-SITE | ACHIEVEMENTS-P22-001 | Site Worker | `apps/site/layouts/archives.html`, `apps/site/layouts/_partials/*achievement*`, `apps/site/assets/{scss,ts}/*achievement*`, `apps/site/content/page/archives/index.md`, P22 C 端专项测试 | API/Admin、数据库、文章正文、上游主题核心 | 2026-07-13 | 已释放：2024 至今成果时间线、响应式/a11y 验收完成 |
| LOCK-P22-CONTENT | ACHIEVEMENTS-P22-001 | Content Worker | `apps/site/content/post/*`, `apps/site/static/img/p22/*`, 技术内容事实核验与图片授权清单 | API/Admin、数据库、Hugo 公共模板、虚构用户真实成果 | 2026-07-13 | 已释放：技术文章、事实核验和构建验收完成 |
| LOCK-P22-CENTER | ACHIEVEMENTS-P22-001 | Center | `apps/site/config/_default/params.toml`, GitHub 头像本地资产、共享 API router/RBAC/publisher 入口、Admin App/client/types/labels、P22 集成测试与 process/QA 文档 | 上游主题核心、用户未确认的真实获奖信息 | 2026-07-13 | 已释放：全链路验收、日志更新和运行态确认完成 |
| LOCK-SITE-LAYOUT-P23 | SITE-LAYOUT-P23-001 | Center | `apps/site/layouts/{baseof,home}.html`, 本地 sidebar/article partial、`apps/site/assets/scss/custom.scss`, P23 文档 | API/Admin、数据库 schema、上游主题核心 | 2026-07-13 | 已释放：Hugo、1280/390/320、暗色切换验收完成；装饰封面已移除 |
| LOCK-SEC-RELIABILITY-P24 | SEC-RELIABILITY-P24-001 | Center + read-only audit agents | `apps/api/internal/{httpapi,mediaref,publisher,maintenance,config}/*`, `apps/api/cmd/migrate/*`, `apps/admin/src/hooks/useAdminSession.ts`, `scripts/qa/*`, `db/migrations/*`, `infra/docker/*`, backend/operations/QA/process docs | C 端主题与内容、Admin 业务界面、development/production 业务数据、无关重构 | 2026-07-14 | 已释放：全量 Go/vet、Admin/Hugo、Compose、26 项黑盒、migration Down/Up 和隔离完整 E2E 通过 |
| LOCK-SITE-HOME-SPLASH-P26 | SITE-HOME-SPLASH-P26-001 | Center | `apps/site/config/_default/hugo.toml`, `apps/site/layouts/baseof.html`, `apps/site/assets/{scss/splash.scss,ts/splash.ts}`, `apps/site/content/post/about-zoking/index.md`, P26 QA/过程文档 | API/Admin、数据库 schema、上游 Theme Stack 核心、现有审计修复和运行态业务数据 | 2026-07-14 | 已释放：Hugo build、首页分页/首篇封面、开屏首次/跳过/复访/reduced-motion、390px 和运行时错误检查通过 |
| LOCK-SITE-VISUAL-WIDGET-P27 | SITE-VISUAL-WIDGET-P27-001 | Center | `apps/site/layouts/baseof.html`, `apps/site/layouts/_partials/widget/pixiv-ranking.html`, `apps/site/assets/{scss/splash.scss,scss/custom.scss,ts/splash.ts}`, `apps/site/config/_default/params.toml`, P27 QA/参考/过程文档 | API/Admin、数据库 schema、文章正文、上游 Theme Stack 核心、运行态业务数据 | 2026-07-14 | 已释放：第三方许可确认、Hugo production build、开屏回归、iframe 实载、明暗主题和 1280/1024/390 验收通过 |
| LOCK-SITE-SIDEBAR-WIDGETS-P28 | SITE-SIDEBAR-WIDGETS-P28-001 | Center + Site/QA agents | `apps/site/layouts/_partials/widget/{popular-posts,pixiv-ranking,hitokoto}.html`, `apps/site/assets/{ts/hitokoto.ts,scss/custom.scss}`, `apps/site/config/_default/params.toml`, `assets/icons/refresh.svg`, P28 QA/第三方通知/过程文档 | API/Admin、数据库 schema、文章正文、上游 Theme Stack 核心、生产数据与部署凭据 | 2026-07-14 | 已释放：Hugo production 61 pages、第三方端点、成功/刷新/缓存/断网降级、1280/1024/390 和上传清单通过 |
| LOCK-SITE-DISCOVERY-P29 | SITE-DISCOVERY-P29-001 | Center | `apps/site/content/page/links/index.md`, `apps/site/layouts/{baseof.html,_partials/article/components/links.html}`, `apps/site/assets/{ts/custom.ts,ts/linksAvatars.ts,scss/custom.scss,scss/command-palette.scss}`, `scripts/qa/{about-links-p21,site-discovery-p29}.mjs`, P29 参考/QA/过程文档 | API/Admin、数据库 schema、文章正文、上游 Theme Stack 核心、生产数据与部署凭据、持续 Canvas/重型动画依赖 | 2026-07-15 | 已释放：Hugo production、P29/P21 Playwright、1280/390、键盘、reduced-motion 与仓库 preflight 通过 |
| LOCK-SITE-GITHUB-SNAPSHOT-P30 | SITE-GITHUB-SNAPSHOT-P30-001 | Center | `apps/site/{data/github_projects.json,layouts/projects.html,assets/scss/github-projects.scss}`, `scripts/{sync-github-projects.mjs,qa/*github*}`, `.github/workflows/sync-github-projects.yml`, P19/P30 文档与证据 | API/Admin、数据库 schema、文章正文、上游 Theme Stack 核心、个人 PAT、私有仓库数据 | 2026-07-15 | 已释放：同步脚本、静态 Hugo 渲染、失败保留、零浏览器 API 请求、1280/390/320 与 production build 通过 |
| LOCK-SITE-NAVIGATION-NOW-P31 | SITE-NAVIGATION-NOW-P31-001 | Center | `apps/site/assets/scss/custom.scss`, `apps/site/content/page/now/index.md`, `scripts/qa/site-navigation-now-p31.mjs`, P31 调研/QA/过程文档与证据 | API/Admin、数据库 schema、上游 Theme Stack 核心、持续动画依赖、重复阅读侧轨、远程链接抓取 | 2026-07-15 | 已释放：真实 ViewTransition 事件、reduced-motion、主菜单/快捷面板、前进后退、1280/390 与 production build 通过 |
| LOCK-SITE-READING-SETTINGS-P32 | SITE-READING-SETTINGS-P32-001 | Center | `apps/site/layouts/baseof.html`, `apps/site/assets/{ts/articleActions.ts,scss/reading-settings.scss}`, `scripts/qa/{site-reading-settings-p32,site-reader-ui}.mjs`, P32 前端/QA/过程文档与证据 | API/Admin、数据库 schema、上游 Theme Stack 核心、任意字体下载、账号同步、统计跟踪 | 2026-07-15 | 已释放：三档字号、二项开关、预绘制恢复、重置、P16 回归、1280/390/320 与 production build 通过 |
| LOCK-SITE-ARTICLE-UTILITIES-P33 | SITE-ARTICLE-UTILITIES-P33-001 | Center | `apps/site/layouts/_markup/render-heading.html`, `apps/site/assets/{ts/clipboard.ts,ts/code-copy.ts,ts/headingLinks.ts,ts/smoothAnchors.ts,ts/custom.ts,scss/custom.scss}`, `scripts/qa/site-article-utilities-p33.mjs`, P33 研究/QA/过程文档与证据 | API/Admin、数据库 schema、文章内容批量改写、第三方运行器、上游 Theme Stack 核心直接修改 | 2026-07-15 | 已释放：P33 专项、P16/P31/P32 回归、1280/390/320、production build 与 preflight 通过 |
| LOCK-SITE-PRINT-DISCOVERY-P34 | SITE-PRINT-DISCOVERY-P34-001 | Center | `apps/site/layouts/baseof.html`, `apps/site/layouts/_partials/{head/custom.html,article/components/details.html}`, `apps/site/assets/scss/print.scss`, `apps/site/config/_default/params.toml`, `scripts/qa/site-print-discovery-p34.mjs`, P34 研究/前端/QA/过程文档与证据 | API/Admin、数据库 schema、Hugo 输出格式、Service Worker/PWA、本地收藏、Markdown 导出、上游 Theme Stack 核心直接修改 | 2026-07-15 | 已释放：P34 专项、P16/P31/P32/P33 回归、A4/390、深色/无 JS、production build 与 preflight 通过 |

规则：

- 写入型任务必须先登记文件锁。
- 只读审阅任务不占文件锁，但派发时必须声明只读。
- 同一文件同一时间只能被一个写入型任务锁定。
- 多个 agent 都需要影响同一文件时，子 agent 只输出建议，由中心窗口统一写入。
- 中心窗口验收后释放文件锁。
- 发现未登记改动时，先判断是否为用户改动，不得直接回滚。

## 4. 子 Agent 状态

| Agent | 类型 | 任务 | 状态 | 结论 |
|---|---|---|---|---|
| Ramanujan | Explorer | 规划缺口审阅 | Done | 缺 API 契约、数据模型、发布流水线、RBAC、安全、QA 等工程文档 |
| Descartes | Explorer | 协作机制审阅 | Done | 缺任务状态机、任务看板、文件锁、调度 SOP、接力 checklist |
| Godel | Explorer | Go/API/DB Phase 0 审阅 | Done | 建议独立 `apps/api` module、仓库级 migrations、health/ready/migrate/seed |
| Heisenberg | Explorer | Admin 实施审阅 | Failed | 429，中心窗口已接管 Admin 基线 |
| Singer | Worker | HUGO-P3-001 C 端评论增强 | Done | 使用 `apps/site` 本地 comments partial 覆盖接入 Public Comments API，中心窗口已补边界并验收 |
| Aristotle | Worker | Admin 异步发布适配 | Done | Publish 按钮适配 202/job 入队语义，发布中心展示 job 状态与失败信息，中心窗口补 queued 状态并验收 |
| Heisenberg | Explorer | QA-P4-001 冒烟脚本审阅 | Done | 指出 rollback、artifact、pending 评论、taxonomy、health 等覆盖缺口，中心窗口已修正或记录 verifier gap |
| Leibniz | Explorer | PUB/MEDIA-P5 只读审阅 | Done | 建议 sitemap 解析所有 `<loc>` 并记录 post/release 双层媒体引用；中心窗口已实现并验收 |
| Ptolemy | Explorer | SETTINGS-P5-001 页面/设置设计审阅 | Done | 建议页面真实落库、根路径 slug 黑名单、设置白名单更新、页面/设置发布纳入 smoke；中心窗口已实现并验收 |
| Huygens | Explorer | MEDIA-P5-002 媒体/release 清理审阅 | Done | 建议先收口现有清理能力、默认 dry-run、保护 active release 与被引用媒体，S3/R2 作为 storage adapter 后续切片 |
| Euler | Explorer | PREVIEW-P5-001 预览架构审阅 | Done | 建议复用 Hugo builder/verifier，但使用临时站点与独立预览目录，禁止进入 release/current 正式发布语义 |
| Avicenna | Explorer | AUDIT-P5-001 审计边界审阅 | Done | 发现核心迁移已有 audit_logs 雏形；建议增量扩表、HMAC IP、严格脱敏与后续事务级 before/after 审计 |
| Popper | Explorer | SEC-P5-001 RBAC 审阅 | Done | 建议 JWT 只存身份、数据库实时加载权限、active user 即时校验、声明式角色 seed、防锁死与 Admin 按权限加载 |
| Descartes | Explorer | USER-RBAC-P5-001 用户角色管理审阅 | Done | 建议事务锁保护最后有效超级管理员、完整替换角色、密码严格脱敏、Admin 按权限加载和并发验收 |
| Aquinas | Explorer | PREVIEW-CLEANUP-P5-001 预览清理审阅 | Done | 建议严格 TTL 文件访问、building 保护、数据库 claim、状态化清理、受控根目录和 worker 定时器 |
| Euclid | Research | ADMIN-UI-P6-001 组件库选型 | Done | 对比 Arco/Semi/TDesign/Ant 的维护、兼容、体积与迁移风险，中心结合当前浅依赖范围采用 Arco |
| Meitner | Explorer | ADMIN-UI-P6-001 Admin 结构与迁移风险盘点 | Done | 核对 10 路由、页面/Hook 依赖和 React 19、响应式、发布语义风险 |
| Euler | Worker | ADMIN-UI-P6-001 Hook 迁移 | Done | 六个业务 Hook 完成 Arco Message/FormInstance/validate 迁移 |
| Lagrange | Worker | ADMIN-UI-P6-001 编辑与设置页 | Done | Posts、Pages、Settings 完成 Arco 和工作台结构迁移 |
| Anscombe | Worker | ADMIN-UI-P6-001 内容运营页 | Done | Taxonomy、Media、Comments 完成 Arco 与高密度工具页迁移 |
| Goodall | Worker | ADMIN-UI-P6-001 发布与系统页 | Done | Publishing、Users、Audit 完成 Arco 与表格/权限工具迁移 |
| Parfit | Research | ADMIN-UX-P7-001 真实 CMS 样例调研 | Done | 提炼列表/编辑分路由、动作分层、短侧栏、筛选分页和默认隐藏企业能力等 12 条规则 |
| Lagrange | Explorer | ADMIN-UX-P7-001 当前后台凌乱度审计 | Done | 确认文章/页面、发布、权限、分类纵向堆叠及全量数据加载为主要问题 |
| Faraday | Worker | ADMIN-UX-P7-001 发布中心 | Done | 发布任务/版本/预览改为 Tabs，任务详情进入 Drawer，表格分页 |
| Lorentz | Worker | ADMIN-UX-P7-001 用户与权限 | Done | 账号/角色改为 Tabs，创建和权限编辑进入 Modal，表格分页 |
| Goodall | Worker | ADMIN-UX-P7-001 内容组织 | Done | 分类/标签改为 Tabs，创建表单进入 Modal，表格分页 |
| Hilbert | Worker | ADMIN-UX-P7-001 媒体与评论 | Done | 媒体维护收纳、搜索分页；评论状态筛选和分页 |
| Pasteur | Worker | ADMIN-UX-P7-001 独立页面 | Done | 页面列表与编辑器分离，新增页面编辑子路由工作区 |
| Dewey | Explorer | QA-P12-001 测试缺口审阅 | Failed | 外部模型容量失败，无代码或审阅产出，中心窗口接管 |
| Gibbs | Worker | QA-P12-001 Go 白盒测试 | Done | 生产 URL、Preview key、限流、配置和 seed 门禁测试通过，中心窗口继续补安全边界 |
| QA Security Auditor | Explorer | QA-P12-001 安全测试缺口复核 | Done | 识别 seed fail-open、Preview 路径重叠、URL 属性/CSS 漏检和限流 IP 规范化风险，中心窗口已复现整改 |
| Heisenberg | Worker | QA-P13-001 Preview 并发与代理边界 | Done | 终态更新限制为 building 单行转换，补可信即时代理、错误映射、相对 base 和真实 PostgreSQL finish/fail 竞争测试 |
| Singer | Worker | QA-P13-001 发布失败不变式 | Done | 补生产产物 URL 失败与历史 release promote 前重验，确认旧 active/current 不变且无孤立 release/输出 |
| Sagan | Worker | QA-P13-001 Linux race CI | Done | 新增 Ubuntu race job 与 QA 文档，中心窗口补 PostgreSQL service 使集成竞争在 `-race` 下实际执行 |
| Zeno | Worker | QA-P14-001 文章分页 PostgreSQL 集成 | Done | 覆盖 taxonomy 组合过滤、total、关联预载、稳定排序和巨大越界页 |
| Mill | Worker | QA-P14-001 媒体/审计分页 PostgreSQL 集成 | Done | 覆盖媒体精确查询旧契约、usage count、分页排序，以及审计 legacy limit、组合过滤和越界页 |
| Harvey | Explorer | QA-P14-001 九类分页只读审阅 | Done | 发现 Audit UUID URN 原值绑定和媒体空精确参数/URL 裁剪两个 P2，中心窗口已复现整改 |
| Arendt | Explorer | SEC-P15-001 角色矩阵与对象访问策略审阅 | Done | 明确 global read/manage 与 owner scope 分层，viewer 全局只读，author 仅本人内容 |
| Newton | Explorer | SEC-P15-001 文章调用链审阅 | Done | 梳理文章 CRUD、预览、发布和身份传递，测试矩阵由中心落地 |
| Pauli | Explorer | SEC-P15-001 页面与发布链路审阅 | Done | 梳理页面及 job/release/preview 所有权范围、站点级记录和 promote 风险 |
| PostgreSQL Worker | Worker | SEC-P15-001 PostgreSQL 越权测试 | Done | 子任务未形成可合并产出，由中心窗口接管并完成真实 PostgreSQL 隔离测试与三连跑 |
| Hilbert | Explorer | SEC-P15-001 Admin 发布与深链审阅 | Done | 发现发布写动作和编辑深链权限缺口，中心窗口已修复并纳入 Playwright |
| Sartre | Explorer | SEC-P15-001 数据与运行态残留审计 | Done | development/test schema、fixture、runtime 均无 P15 残留，历史 settings baseline 保留 |
| Aquinas | Explorer | SEC-P15-001 文档收口审阅 | Done | 给出 QA、工作日志、任务板和交接文档的收口位置 |
| Carson | Explorer | SEC-P15-001 Admin 横向权限审阅 | Done | 发现 taxonomy、媒体、评论、设置的低权限写入口，中心窗口已完成权限裁剪 |
| Euler | Worker | SEC-P15-001 Playwright 横向权限矩阵 | Done | 扩展 super_admin/author/viewer 在 taxonomy、媒体、评论、设置的 UI 黑盒并通过静态检查 |
| Volta | Explorer | SITE-AUDIT-P16-001 API/Admin 扩展能力审阅 | Done | 给出阅读增强、系列、定时发布、修订、统计和高风险能力矩阵，明确静态私密内容边界 |
| Pasteur-P16 | Explorer | SITE-AUDIT-P16-001 C 端 QA/a11y/SEO/性能审计 | Done | 发现搜索 alt/层级、深色对比度、soft 404、评论错误、英文 section、landmark 和 gzip 问题，中心窗口已整改验收 |
| Ohm | Explorer | CONTENT-QUALITY-P17-001 API/发布链路审阅 | Done | 发现发布半提交、普通 CRUD 绕过、Worker 读取最新正文和 Retry/站点发布复检缺口 |
| Goodall-P17 | Explorer | CONTENT-QUALITY-P17-001 Admin UX 审阅 | Done | 明确使用紧凑 Drawer、未保存表单检查、报告失效和检查后保存发布顺序 |
| Godel-P17 | Explorer | CONTENT-QUALITY-P17-001 QA/安全审阅 | Done | 给出权限、零副作用、并发、Worker/Retry、危险 HTML 和桌面/移动测试矩阵 |
| Copernicus | Worker | CONTENT-QUALITY-P17-001 Admin 发布检查 UI | Done | 完成文章/页面检查流程、共享 Drawer、状态失效和响应式样式；中心窗口补服务端错误回显并验收 |
| Socrates-P18 | Worker | SERIES-P18-001 API/DB 系列模型与契约 | Done | migration、model、系列 CRUD、文章关系、RBAC、并发约束与真实 PostgreSQL 契约测试完成，中心已验收 |
| Cicero-P18 | Worker | SERIES-P18-001 Admin 系列管理与文章字段 | Done | 内容组织系列 Tab、管理 Modal、文章系列字段、冲突反馈和响应式交互完成，中心已验收 |
| Kant-P18 | Worker | SERIES-P18-001 C 端系列目录与导航 | Done | Stack 风格系列目录、首中末导航、普通导航兼容和移动端样式完成，中心已验收 |
| Curie-P19 | Worker | SITE-GITHUB-P19-001 内容发现信息架构 | Done | 分类/标签从一级导航降级，归档统一探索结构与 homepage widgets 精简完成，中心已验收 |
| Hopper-P19 | Worker | SITE-GITHUB-P19-001 GitHub 仓库展示 | Done | projects 页面、GitHub 白名单数据、短期缓存、异常回退和响应式样式完成，中心已验收 |
| Nielsen-P19 | Explorer | SITE-GITHUB-P19-001 UX/QA 审阅 | Done | 完成导航密度、卡片层级、第三方请求边界、a11y 和 Playwright 矩阵审阅 |
| Feynman-P20 | Research | SITE-TIMELINE-P20-001 真实博客归档与装饰调研 | Done | 调研 Anthony Fu、Simon Willison、Maggie Appleton、Derek Sivers、Gwern，建议单列时间河和淡年份锚点 |
| Ada-P21 | Research | SITE-ABOUT-PLUGIN-P21-001 Hugo/博客插件生态调研 | Done | 完成 Pagefind、Giscus、Mermaid、KaTeX、Plausible/Umami、Webmention、ActivityPub、PhotoSwipe 的成熟度与安全边界调研 |
| Turing-P21 | Research | SITE-ABOUT-PLUGIN-P21-001 真实博客特色能力调研 | Done | 调研 7 个以上成熟个人博客，建议 Now/Uses/精选项目优先，高成本互动能力后置 |
| Shannon-P20 | Explorer | SITE-TIMELINE-P20-001 模板与搜索边界审阅 | Done | 建议保留搜索路由与 404 恢复入口、撤下显式入口，并使用语义化年份时间线 |
| Norman-P20 | Explorer | SITE-TIMELINE-P20-001 响应式/a11y/QA 审阅 | Done | 给出无卡片时间线、移动端细线、标题层级与 Playwright 验收矩阵 |

## 5. 阻塞与待决策

| 编号 | 决策 | 默认建议 | 当前状态 |
|---|---|---|---|
| DEC-P0-001 | Hybrid CMS 架构 | 接受 | 已采用 |
| DEC-P0-002 | 后台技术栈 | React + Arco Design | 2026-07-12 已迁移为 React/Vite/Arco，禁止长期混用完整组件库 |
| DEC-P0-003 | 媒体生产存储 | 开发本地，生产预留对象存储 | 开发本地媒体库已实现，生产对象存储待 OPS 阶段 |
| DEC-P0-004 | 部署目标 | 单 VPS Docker Compose 起步 | 已采用本地 Docker Compose 起步 |
| DEC-P0-005 | MVP 评论范围 | 匿名评论 + 后台审核 | 已实现 API/Admin/C 端最小闭环 |
| DEC-P16-001 | 匿名阅读状态存储 | 仅浏览器本地、500ms 节流、完成删除、30 天过期 | 已采用；不上传阅读历史，不创建设备 ID |

## 6. 看板维护规则

- 中心窗口每次派发任务前更新本文件。
- 子 agent 完成并验收后，把对应任务改为 `Done` 或 `Blocked`。
- 每轮实施结束必须同步更新 `docs/process/worklog.md`。
- 新窗口接手后，第一件事是核对此文件与 `git status --short --branch` 是否一致。
