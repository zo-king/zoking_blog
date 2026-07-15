# 工作日志

## 2026-07-11 09:44 +08:00 - CENTER - SITE-P5-003

- 目标：修正 C 端视觉重心、删除左栏滚动条、清理历史测试内容并整体中文化。
- 实现：主容器增加 `1440px` 最大宽度与对称留白，重新平衡左/正文/右栏比例；左侧菜单保留滚动能力但隐藏 scrollbar；默认语言切换为简体中文并禁用其他语言。
- 内容：清空历史 demo、分页、发布和 E2E 文章/页面快照，重建中文首页、关于、归档、搜索，以及三篇带真实配图的中文展示文章。
- 验证：Hugo production build 通过；Playwright 在 1280x720 和 390x844 验证无页面横向溢出、语言为 `zh-cn`、三张配图均成功加载、左栏 scrollbar 不可见。
- 状态：`SITE-P5-003` Done。
- 下一步：`AUDIT-P5-001` 已进入 In Progress，由中心窗口实现，Avicenna 只读审阅边界。

## 2026-07-11 09:49 +08:00 - CENTER - AUDIT-P5-001

- 目标：为后台关键操作建立可查询、可追踪且不泄露敏感内容的审计日志。
- 子 agent：Avicenna 只读审阅，发现核心迁移已有 `audit_logs` 雏形，中心窗口据此改为增量扩表。
- 实现：新增 AuditLog model、认证后写请求审计 middleware、`GET /api/v1/admin/audit-logs` 查询 API、Admin Audit Logs 表格。
- 安全：不读取请求体；不保存密码、Authorization、token、文章/评论正文或文件；IP 使用 JWT secret 做 HMAC-SHA256；User-Agent 截断到 512 字符。
- 数据：迁移 `20260711000200` 增加 actor email 快照、标准化路由、HTTP method/status、error code、details JSON 和 IP hash version。
- 验证：Go tests、Admin build、Hugo build、migration、`preflight -SkipE2E` 通过；200 成功请求与 422 失败请求均生成正确审计记录。
- 边界：当前统一记录写请求元数据；事务级 before/after 安全摘要、worker 最终状态事件和 `audit:read` RBAC 属于后续增强。
- 状态：`AUDIT-P5-001` Done，`LOCK-AUDIT-P5` 已释放。
- 下一步：`SEC-P5-001` 更细 RBAC 权限控制。

## 2026-07-11 09:56 +08:00 - CENTER - SEC-P5-001

- 目标：把后台从“登录即全权限”收紧为服务端数据库实时 RBAC。
- 子 agent：Popper 只读审阅现有 schema、seed、JWT 和 Admin，确认现有五张 RBAC 表可直接使用，并指出停用用户旧 token、空角色和 Admin Promise.all 风险。
- 实现：新增权限加载与路由映射 middleware；每个请求检查 active/未删除用户，加载多角色权限并集；`/auth/me` 返回角色和权限；Admin 展示当前角色并只在有 `audit:read` 时加载/显示审计表。
- Seed：补齐 post/page/media/publish 等权限，并为 `super_admin/admin/editor/author/viewer` 建立可重复执行的声明式授权矩阵；站点默认设置同步为中文。
- 审计：403 统一记为 `denied`；普通 GET 不审计，但任何被 RBAC 拒绝的 GET 会记录。
- 验证：管理员返回 `super_admin` 与 27 项权限；临时撤销 `audit:read` 后同一 JWT 立即返回 403，恢复后无需重新登录；拒绝事件审计为 `denied/403`。
- 构建：Go tests、Admin build、Hugo build 与 `preflight -SkipE2E` 通过；为保持中文展示内容干净，未运行会生成 E2E 文章的完整 smoke。
- 边界：尚未实现用户/角色管理 API、最后一个超级管理员保护、author 对象级数据隔离和全部 Admin 按钮的权限裁剪。
- 状态：`SEC-P5-001` Done，`LOCK-SEC-RBAC-P5` 已释放。
- 下一步：实现后台用户与角色管理及最后一个超级管理员保护，或推进预览过期清理任务。

## 2026-07-11 10:59 +08:00 - CENTER - USER-RBAC-P5-001

- 目标：开放后台用户与角色管理，同时保证任何并发操作都不能删除最后一个有效超级管理员。
- 子 agent：Descartes 只读审阅，建议统一锁顺序、完整角色替换、密码脱敏和有限权限 Admin 独立加载。
- API：新增用户列表/创建、状态更新、角色替换和角色权限列表；权限为 `user:read`、`user:manage`、`role:read`。
- 防锁死：停用用户和移除 super_admin 角色均在事务内获取 `pg_advisory_xact_lock`，并按 active + 未删除 + super_admin 统计有效管理员；最后一人操作返回 409。
- Admin：新增 User & Role Management，支持创建账号、启停、角色多选和系统角色权限数量；模块按权限加载。
- 修复：历史 publish job 的 `log_json` 可能不是数组，Admin 改为类型安全渲染；主 Layout 增加 `min-width:0`，宽表格不再撑出页面横向滚动条。
- 验证：唯一超级管理员停用与降权均返回 409；临时用户成功创建为 viewer、改为 editor、停用并清理；审计中明文密码匹配数为 0。
- 视觉：Playwright 在 1440px 与 390px 验证用户管理可见且页面无横向溢出。
- 构建：Go tests、Admin/Hugo build、`preflight -SkipE2E` 通过；未运行会重新生成测试文章的完整 E2E。
- 状态：`USER-RBAC-P5-001` Done，`LOCK-USER-RBAC-P5` 已释放。
- 下一步：预览过期清理，或继续自定义角色/权限矩阵与密码重置。

## 2026-07-11 11:20 +08:00 - CENTER - PREVIEW-CLEANUP-P5-001

- 目标：让预览 TTL 真正生效，并自动回收数据库记录对应的静态目录。
- 子 agent：Aquinas 只读审阅，指出 expires_at 原先只写不读、GET 入口未校验 TTL、多 worker 需要 claim、building 必须保护。
- 实现：静态预览服务只放行 status=ready 且未到期记录；maintenance 只选择过期 ready/failed，条件更新 claim 为 cleaning，删除目录并验证不存在，再更新为 expired；失败进入 cleanup_failed。
- 自动化：API 内嵌 worker和独立 worker均启动定时器，默认 1h、每批 100；数据库 claim 保证并发实例不会重复删除。
- 手动操作：新增 `POST /publish/previews/cleanup`，默认 dry-run；Admin Preview Builds 提供 Dry Run/Apply，权限为 `publish:cleanup`。
- 安全：目录仅由 preview root + preview key 推导；拒绝磁盘根目录和与 release/current/media/Hugo 目录重叠的 preview root；building 和 expires_at null 永不参与普通 TTL 清理。
- 验证：预览到期前 200、到期后立即 404；dry-run 候选数 1 且目录仍在；apply 删除数 1、记录状态 expired、目录消失，正式 dist/site 仍存在。
- 构建：Go tests、Admin build、生产 compose config 与 `preflight -SkipE2E` 通过；未运行会生成测试文章的完整 E2E。
- 状态：`PREVIEW-CLEANUP-P5-001` Done，文件锁已释放。
- 下一步：自定义角色/权限矩阵与密码重置，或对象存储/CDN adapter。

- ## 顶部状态摘要
-
- 当前阶段：Phase 1 内容域最小闭环、Phase 3 发布 job/release/rollback/C端评论/异步 worker 最小闭环、Phase 4 E2E 冒烟脚本与 Docker Compose 生产基线已完成，Phase 5 已完成发布验收、媒体引用、页面管理、站点设置、部署前 preflight、媒体清理、release 保留策略与隔离预览，继续推进生产增强
- 当前分支：`master`
- 当前仓库：`D:\zoking\zoking-blog`
- 当前稳定点：已创建 `apps/site`、`apps/api`、`apps/admin`、`db/migrations`、`infra/docker`；Stack demo 风格前台、Gin/GORM/PostgreSQL API、React Admin 壳均可运行；Admin 已可登录、创建草稿、绑定分类/标签、上传媒体、插入 Markdown、审核评论、发布文章、管理独立页面、保存并发布站点设置；发布会异步创建 publish job，由 worker 生成 release、manifest 和 Hugo release 产物；历史 release 可 promote/rollback；C 端文章页已接入 Public Comments API；发布 verifier 已递归解析 sitemap `<loc>`；媒体引用已写入 `media_usages` 并保护删除；孤立媒体清理与旧 release 清理已提供默认 dry-run 的后台/API 操作；端到端冒烟脚本可重复验证文章/页面/设置/评论/媒体/rollback/清理 dry-run 核心闭环；API/worker/Admin 生产镜像与 compose 基线已可构建；部署前 preflight 已串起 Go/Admin/Hugo build、migrate/seed 和 E2E smoke
- 当前阻塞：完整目标尚未完成；后续需完善用户/角色管理、最后一个超级管理员保护、author 对象级隔离、预览过期清理，并把对象存储/CDN 从策略推进到实际 storage adapter
- 中心窗口职责：汇报、调度子 agent、合并结论、维护日志、控制范围
- 下一步建议：推进用户/角色管理与防锁死约束；预览过期清理和对象存储/CDN adapter 可作为独立后续任务登记
- 新窗口优先阅读：
  1. `docs/process/context-handoff.md`
  2. `docs/process/worklog.md`
  3. `docs/process/task-board.md`
  4. `docs/plan/engineering-execution-master-plan.md`
  5. `docs/plan/goal-plan.md`
  6. `docs/architecture/00-system-overview.md`
  7. `docs/requirements/00-index.md`
  8. `docs/frontend-engineering-guide.md`

## 2026-07-11 09:10 +08:00 - CENTER - MEDIA-P5-002

- 目标：补齐 release 保留期、孤立媒体清理、Admin 操作入口和对象存储/CDN 策略，让媒体与发布产物具备可运维的清理边界。
- 输入：
  - 已有 `media_usages` 可保护文章编辑态引用和 release 发布态引用。
  - 已有 publish release/active release/rollback 语义，清理不能破坏当前线上 release。
  - 子 agent Huygens 只读建议：先收口现有清理能力；清理默认 dry-run；保护 active release 与被引用媒体；S3/R2 作为 storage adapter 后续切片，不在上传处理里临时分支。
- 操作：
  - 新增配置：`MEDIA_ORPHAN_GRACE_PERIOD`、`MEDIA_CLEANUP_BATCH_SIZE`、`PUBLISH_RELEASE_KEEP_LATEST`、`PUBLISH_RELEASE_KEEP_DAYS`，并同步 `.env.example`、生产 env 模板和 compose。
  - 新增 `apps/api/internal/maintenance`，提供孤立媒体清理和旧 release 清理能力。
  - 新增后台 API：`POST /api/v1/admin/media/cleanup`、`POST /api/v1/admin/publish/releases/cleanup`；默认 dry-run，只有请求体显式 `"dry_run": false` 才执行破坏性清理。
  - Admin 媒体库新增孤立媒体 dry-run/apply 按钮，发布中心 release 区新增旧 release dry-run/apply 按钮。
  - E2E smoke 增加 orphan media cleanup dry-run 和 release cleanup dry-run 断言，确保 active release 不会进入清理候选。
  - 运维/架构/API/QA 文档补充清理语义、保留期配置、preflight 覆盖和对象存储/CDN 演进边界。
- 产出：
  - `apps/api/internal/config/config.go`
  - `apps/api/internal/maintenance/cleanup.go`
  - `apps/api/internal/maintenance/cleanup_test.go`
  - `apps/api/internal/httpapi/cleanup.go`
  - `apps/api/internal/httpapi/router.go`
  - `apps/admin/src/App.tsx`
  - `scripts/qa/e2e-smoke.ps1`
  - `.env.example`
  - `infra/docker/.env.prod.example`
  - `infra/docker/compose.prod.yml`
  - 文档同步：`docs/backend/00-api-contract.md`、`docs/operations/deployment-runbook.md`、`docs/architecture/frontmatter-and-snapshot-mapping.md`、`docs/qa/e2e-smoke.md`、`docs/qa/preflight.md`、`docs/process/*`
- 验证：
  - `gofmt`：通过。
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - PowerShell parser check：`scripts/qa/e2e-smoke.ps1` 通过。
  - `docker compose --env-file infra/docker/.env.prod.example -f infra/docker/compose.prod.yml config`：通过。
  - HTTP dry-run smoke：`POST /api/v1/admin/media/cleanup` 与 `POST /api/v1/admin/publish/releases/cleanup` 均返回 dry-run 结果。
  - `pwsh -NoProfile -File scripts/qa/e2e-smoke.ps1 -PublishTimeoutSeconds 180 -RestoreSettings`：通过，settings restore release `rel_20260711_090744_6fedc5f7`。
  - `pwsh -NoProfile -File scripts/qa/preflight.ps1 -PublishTimeoutSeconds 180`：通过，settings restore release `rel_20260711_090822_8473c7ac`。
  - `rg -n "Zoking Smoke|Smoke sidebar|restore settings payload" apps/site/config dist/site -S`：无匹配，配置与当前站点产物无 smoke 残留。
- 决策记录：
  - 清理 API 默认 dry-run；执行清理必须显式传入 `"dry_run": false`。
  - 孤立媒体只清理无 usage、非 deleted、超过 grace period 的本地媒体；路径必须位于 `MEDIA_LOCAL_DIR` 子路径内。
  - 旧 release 清理永远跳过 active release，同时保留最新 N 个 inactive release 和保留期内 release。
  - release 清理会删除对应 release 级 `media_usages`、标记 release 为 `pruned`、软删除 DB 记录并移除 release 目录。
  - 对象存储/CDN 当前先固化为 storage adapter 演进方向；本轮不把 S3/R2 分支塞入上传 handler。
- 下一步：
  - `PREVIEW-P5-001`：补草稿预览和发布前 release 预览。
  - `AUDIT-P5-001`：把页面、设置、发布、媒体清理/删除等后台操作写入审计日志。
  - `SEC-P5-001`：补更细 RBAC 权限点和后台危险操作保护。

## 2026-07-11 08:35 +08:00 - CENTER - CI-P5-001

- 目标：把 Go/Admin/Hugo build、数据库 migrate/seed、E2E smoke 和 rollback 验收串成部署前检查，并接入 GitHub Actions。
- 输入：
  - 已有 `scripts/qa/e2e-smoke.ps1` 可验证文章、页面、设置、媒体、评论和 rollback。
  - 已有生产 Docker/Compose 基线和 `docs/qa/test-strategy.md`。
  - 用户确认密钥占位不用作为当前阻塞，因此 CI 示例 secret 占位不阻塞本轮。
- 操作：
  - 新增 `scripts/qa/preflight.ps1`，顺序执行 Go tests、Admin build、Hugo production build、migrate/seed 和 E2E smoke。
  - preflight 支持 `-SkipE2E`、`-Install`、`-StartApi`、`-StopStartedApi`、`-SkipRollback`、`-BootstrapRollbackBaseline`、`-PublishTimeoutSeconds`。
  - 新增 `.github/workflows/preflight.yml`，在 push/master 与 pull_request 上启动 PostgreSQL 16 service，并运行 `preflight.ps1 -Install -StartApi -StopStartedApi -PublishTimeoutSeconds 240`。
  - 新增 `docs/qa/preflight.md`，并同步 README、QA、运维、Goal 文档索引。
  - 扩展 E2E smoke 的 `-RestoreSettings`：运行前捕获站点设置，测试后 PATCH 回原配置并发布恢复 release，避免 `Zoking Smoke ...` 标题残留。
  - 修复 PowerShell 变量名冲突：`$restoreSettings` 与 `[switch]$RestoreSettings` 大小写不敏感等同，导致 response `PSCustomObject` 被写入 switch；响应变量已改为 `$restoredSettingsResponse`。
- 产出：
  - `scripts/qa/preflight.ps1`
  - `.github/workflows/preflight.yml`
  - `docs/qa/preflight.md`
  - `scripts/qa/e2e-smoke.ps1`
  - 文档同步：`README.md`、`docs/README.md`、`docs/qa/test-strategy.md`、`docs/operations/README.md`、`docs/requirements/00-index.md`、`docs/plan/goal-plan.md`、`docs/process/*`
- 验证：
  - PowerShell parser check：`scripts/qa/e2e-smoke.ps1` 通过。
  - PowerShell parser check：`scripts/qa/preflight.ps1` 通过。
  - `pwsh -NoProfile -File scripts/qa/preflight.ps1 -SkipE2E`：通过。
  - `pwsh -NoProfile -File scripts/qa/e2e-smoke.ps1 -PublishTimeoutSeconds 180 -RestoreSettings`：通过，settings restore release `rel_20260711_083249_98891acc`。
  - `pwsh -NoProfile -File scripts/qa/preflight.ps1 -PublishTimeoutSeconds 180`：通过，settings restore release `rel_20260711_083333_b94aa3c3`。
  - `pwsh -NoProfile -File scripts/qa/preflight.ps1 -StartApi -StopStartedApi -PublishTimeoutSeconds 180`：通过，当前 API 已 ready，脚本复用现有 API；settings restore release `rel_20260711_083427_ca5cc406`。
  - `rg -n "Zoking Smoke|Smoke sidebar|restore settings payload" apps/site/config dist/site -S`：无匹配，配置与当前站点产物无 smoke 残留。
- 决策记录：
  - preflight 是部署前本地/CI 统一入口；默认会跑完整 E2E，`-SkipE2E` 仅用于快速构建检查。
  - E2E smoke 的站点设置恢复必须发布新的 site release，不能只回写数据库。
  - PowerShell 脚本中避免使用仅大小写不同的变量名，尤其不要让响应变量与 switch 参数同名。
  - `-StartApi` 若发现 API 已 ready，会复用现有进程，不会停止非 preflight 启动的进程。
- 下一步：
  - `MEDIA-P5-002`：补 release 保留期、孤立媒体清理和对象存储/CDN 策略。
  - `PREVIEW-P5-001`：补草稿预览和发布前页面预览。
  - `AUDIT-P5-001` / `SEC-P5-001`：补更细 RBAC 权限点、审计日志和后台操作保护。

## 2026-07-11 08:04 +08:00 - CENTER - SETTINGS-P5-001

- 目标：补齐面向 B 端控制的站点设置与独立页面管理，使后台不只管理文章，还能管理 Stack 风格 C 端常见页面与基础站点配置。
- 输入：
  - 用户目标：成熟完整全栈博客系统，C 端保留 Hugo Theme Stack 页面效果，B 端后台可控制。
  - 子 agent Ptolemy 只读建议：页面 slug 占用根路径，必须保留 slug 黑名单；设置必须白名单式更新，不覆盖整份 Hugo 配置；页面/设置发布必须进入 smoke 机器验收。
  - 已有发布 worker、release manifest、rollback、媒体 usage、E2E smoke。
- 操作：
  - 新增 `pages` migration/model/API，支持 public/admin list/get/create/update/publish/delete。
  - 新增 page slug 保留字校验：`search/categories/tags/p/post/api/admin/zh/en/ja/...` 等根路径冲突直接拒绝。
  - 新增 `site_settings` 显式模型，避免 GORM `Base` 软删除字段误加 `deleted_at` 查询。
  - 新增站点设置 API：public settings、admin settings 读取、白名单 PATCH、settings publish 入队。
  - 设置白名单覆盖 `site.title`、`site.base_url`、`sidebar.subtitle`、`sidebar.emoji`、`comments.enabled`、`comments.api_base`、`footer.since`、`pagination.pager_size`。
  - 发布器新增 `WritePage`、page release verifier、site settings publish、manifest `scope/page_id/settings_hash`。
  - 修复 Hugo 设置应用：同时写全局 `hugo.toml/params.toml` 和默认语言 `languages.toml` 覆盖层，避免 `[en] title`、`[en.params.sidebar] subtitle` 覆盖后台设置。
  - 新增 `publish_jobs.settings_hash` migration 和模型字段，记录 settings release 的配置快照 hash。
  - Admin 新增 Page Manager 与 Site Settings 控制区，发布中心显示 post/page/site 三类 target。
  - E2E smoke 扩展：reserved page slug、页面发布、页面 sitemap/menu/content/release、settings publish、settings hash、站点标题/侧栏副标题、最终 release 中文章和页面仍存在。
  - 冒烟测试后恢复默认站点设置，并发布默认 settings release，避免测试标题残留。
- 产出：
  - `db/migrations/20260710000700_create_pages_and_page_publish.sql`
  - `db/migrations/20260710000800_add_publish_job_settings_hash.sql`
  - `apps/api/internal/model/page.go`
  - `apps/api/internal/model/site_setting.go`
  - `apps/api/internal/model/publish.go`
  - `apps/api/internal/httpapi/pages.go`
  - `apps/api/internal/httpapi/settings.go`
  - `apps/api/internal/httpapi/router.go`
  - `apps/api/internal/publisher/hugo.go`
  - `apps/api/internal/publisher/job.go`
  - `apps/api/internal/publisher/worker.go`
  - `apps/api/internal/publisher/job_test.go`
  - `apps/api/cmd/seed/main.go`
  - `apps/admin/src/App.tsx`
  - `apps/admin/src/styles.css`
  - `scripts/qa/e2e-smoke.ps1`
- 验证：
  - `go run ./cmd/migrate up` in `apps/api`：迁移到 `20260710000800`。
  - `go run ./cmd/seed` in `apps/api`：通过。
  - 重启 API 后 `GET http://localhost:18080/readyz`：返回 `ready`。
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `pwsh -NoProfile -File scripts/qa/e2e-smoke.ps1 -SkipRollback -PublishTimeoutSeconds 180`：通过。
    - post release：`rel_20260711_080149_c64782ad`
    - page release：`rel_20260711_080151_cac4623c`
    - settings release：`rel_20260711_080153_eb77ea12`
  - `pwsh -NoProfile -File scripts/qa/e2e-smoke.ps1 -PublishTimeoutSeconds 180`：通过完整 rollback。
    - post release：`rel_20260711_080201_f9aa9ff3`
    - page release：`rel_20260711_080203_2b47dcb9`
    - settings release：`rel_20260711_080205_752ae833`
    - rollback release：`rel_20260711_080153_eb77ea12`
  - 设置恢复发布：`settings` 默认值发布 job `8fc50a9f-21b9-44c3-8791-5ab2de77297e`，release `rel_20260711_080313_8fc50a9f`。
  - `rg -n "Zoking Smoke|Smoke sidebar" apps/site/config dist/site -S`：无匹配，测试配置未残留到当前默认站点。
- 决策记录：
  - 独立页面采用 `/content/page/{slug}/index.md`，C 端 permalink 为 `/{slug}/`；页面 slug 必须避开 Stack/Hugo 根路径和语言前缀。
  - 设置 API 只允许白名单字段，保存只落库，`POST /settings/publish` 才触发 site 级 release。
  - 当前只同步默认语言覆盖层；多语言独立文案后续通过 locale settings 或 i18n 管理扩展。
  - Settings release 使用 `scope=site` 和 `settings_hash` 作为 manifest/DB 可审计快照。
- 下一步：
  - `CI-P5-001`：把 Go/Admin/Hugo build 与 E2E smoke 串成部署前检查。
  - `MEDIA-P5-002`：补 release 保留期、孤立媒体清理和对象存储/CDN 策略。
  - `AUDIT-P5-001`：把页面、设置、发布、媒体删除等后台操作写入审计日志。

## 2026-07-10 21:56 +08:00 - CENTER - PUB-P5-001 / MEDIA-P5-001

- 目标：收口发布 verifier 的 sitemap 假 warning，并把媒体引用追踪从建表推进到真实删除保护。
- 输入：
  - E2E smoke 之前只检查根 `sitemap.xml`，多语言 Hugo release 中新文章实际位于 `en/sitemap.xml` 等子 sitemap。
  - `media_usages` 表已存在，但文章正文引用媒体后尚未自动写入 usage。
  - 子 agent Leibniz 只读审阅：建议递归解析所有 sitemap `<loc>`，并区分文章编辑态引用与 release 引用。
  - 用户明确“密钥占位不用管”，因此 `.env.example`/prod env 的占位 secret 不作为当前阻塞。
- 操作：
  - 新增 `apps/api/internal/mediaref`，统一解析 Markdown 中对 `media_assets.public_url`、URL path 与 `storage_key` 的引用。
  - 文章 create/update/publish 时同步 `resource_type=post`、`usage_type=markdown` 的媒体引用。
  - 发布成功创建 release 后，在同一 promote 事务中同步 `resource_type=release`、`usage_type=markdown` 的媒体引用，用于保护历史 release / rollback 资源。
  - Admin 媒体库展示 `usage_count`，被引用媒体禁用 Delete。
  - `deleteMedia` 继续以 `media_usages` 计数作为 409 删除保护。
  - 发布 verifier 从字符串扫描升级为解析所有 sitemap XML `<loc>`，按 URL path 归一化匹配 `/p/{slug}/`。
  - E2E smoke 改为递归解析 release 下所有 `sitemap.xml`，新文章未收录直接失败，不再 warning。
  - E2E smoke 增加媒体 usage 计数与引用媒体删除 409 断言。
  - 新增 `mediaref` 与 publisher sitemap 单测。
- 产出：
  - `apps/api/internal/mediaref/mediaref.go`
  - `apps/api/internal/mediaref/mediaref_test.go`
  - `apps/api/internal/publisher/job.go`
  - `apps/api/internal/publisher/job_test.go`
  - `apps/api/internal/httpapi/router.go`
  - `apps/api/internal/httpapi/media.go`
  - `apps/api/internal/model/media.go`
  - `apps/admin/src/App.tsx`
  - `scripts/qa/e2e-smoke.ps1`
  - 文档同步：`docs/process/*`、`docs/qa/e2e-smoke.md`、`docs/architecture/*`
- 验证：
  - `docker compose -f infra/docker/compose.dev.yml up -d postgres`：PostgreSQL 启动成功。
  - `go run ./cmd/migrate up` in `apps/api`：迁移到 `20260710000600`。
  - `go run ./cmd/seed` in `apps/api`：通过。
  - 重新启动 API：`http://localhost:18080/readyz` 返回 `ready`。
  - `go test ./...` in `apps/api`：通过，新增 `internal/mediaref` 与 `internal/publisher` 测试。
  - `npm run build` in `apps/admin`：通过。
  - `pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\e2e-smoke.ps1 -SkipRollback`：通过。
    - slug：`e2e-smoke-20260710215107`
    - job：`43e0471f-f60d-4076-803c-99df13e601c6`
    - release：`rel_20260710_215109_43e0471f`
  - `pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\e2e-smoke.ps1`：通过完整 rollback。
    - slug：`e2e-smoke-20260710215633`
    - job：`a1516ae7-7625-4f3a-9197-a0c0b0d78e24`
    - release：`rel_20260710_215633_a1516ae7`
    - rollback release：`rel_20260710_215109_43e0471f`
- 决策记录：
  - Sitemap 验收以所有 sitemap XML 的 `<loc>` URL path 为准，不再只检查根 sitemap。
  - 媒体删除保护先覆盖当前文章引用和已生成 release 引用；后续再补可配置保留期与清理任务。
  - 示例环境变量中的占位密钥不作为当前工程阻塞。
- 下一步：
  - `SETTINGS-P5-001`：补站点设置与页面管理真实 API/Admin。
  - `CI-P5-001`：把 Go/Admin/Hugo build 与 E2E smoke 接入部署前检查。
  - `MEDIA-P5-002`：补 release 保留期、孤立媒体清理和对象存储/CDN 策略。

## 2026-07-10 18:49 +08:00 - CENTER - OPS-P4-DOCKER-COMPOSE-BASELINE

- 目标：把已跑通的 API、worker、Admin 和 PostgreSQL 固化为生产 Docker/Compose 基线，并明确 C 端 release 服务边界。
- 输入：
  - 已完成的 API/worker/Admin/Hugo 发布链路。
  - `docs/operations/deployment-runbook.md` 的生产部署顺序。
  - 当前只有 `infra/docker/compose.dev.yml` 的 Postgres 开发服务。
- 操作：
  - 新增 `apps/api/Dockerfile`，多阶段构建 `api`、`worker`、`migrate`、`seed` 四个 Go 二进制。
  - API runtime 从 Alpine 调整为 Debian bookworm-slim，解决 Hugo Extended glibc/libstdc++ 依赖问题。
  - API 镜像内置 Hugo Extended `0.160.1`，供 worker 执行 release build。
  - 新增 `apps/admin/Dockerfile`，使用 Node 构建 Vite 静态资源，再用 Nginx 服务。
  - 新增 Admin `runtime-config.js` 机制，通过 `ADMIN_API_BASE_URL` 在容器启动时注入 API 地址。
  - 新增 `infra/docker/compose.prod.yml`，包含 `postgres`、`api`、`worker`、`admin`。
  - 新增 `infra/docker/.env.prod.example`。
  - 新增 `.dockerignore`。
  - 更新 `docs/operations/deployment-runbook.md` 和 `README.md`。
- 产出：
  - `apps/api/Dockerfile`
  - `apps/admin/Dockerfile`
  - `apps/admin/nginx.conf`
  - `apps/admin/docker-entrypoint.d/40-runtime-config.sh`
  - `apps/admin/public/runtime-config.js`
  - `infra/docker/compose.prod.yml`
  - `infra/docker/.env.prod.example`
  - `.dockerignore`
  - `docs/operations/deployment-runbook.md`
  - `README.md`
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `docker compose --env-file infra/docker/.env.prod.example -f infra/docker/compose.prod.yml config`：通过。
  - `docker build -f apps/admin/Dockerfile -t zoking-blog-admin:local .`：通过。
  - `docker build -f apps/api/Dockerfile -t zoking-blog-api:local .`：通过。
  - Admin 镜像自检：`nginx -t` 通过，`index.html` 和 `runtime-config.js` 存在。
  - API 镜像自检：`/app/api`、`/app/worker` 可执行，`/usr/local/bin/hugo version` 输出 Hugo Extended `0.160.1`。
- 决策记录：
  - 生产 compose 中 `api` 和 `worker` 共用同一镜像，`api` 设置 `PUBLISH_WORKER_ENABLED=false`，由独立 `worker` 服务处理发布任务。
  - Admin 镜像不烘死 API 地址，使用运行时 `runtime-config.js`。
  - C 端线上静态服务的 `current` 指针尚未自动化，当前 runbook 明确为后续项。
- 下一步：
  - `PUB-P5-001`：补 release/current 自动切换、sitemap verifier、retry/cancel/timeout 和结构化构建日志。
  - `MEDIA-P5-001`：补媒体引用追踪、发布固化和保留策略。
  - `SETTINGS-P5-001`：补站点设置与页面管理真实 API。

## 2026-07-10 19:40 +08:00 - CENTER - OPS-P4-CURRENT-SWITCH-AND-JOB-HARDENING

- 目标：把 release/current 线上切换、发布任务日志与可控性、生产站点服务和评论环境变量口径继续补齐，并验证可重复发布。
- 输入：
  - 之前的 release/release manifest/rollback/worker 基线。
  - 生产 compose 仍缺 C 端静态站点入口。
  - E2E smoke 之前已提示 sitemap 仍为 verifier gap。
- 操作：
  - 在 `apps/api/internal/config/config.go` 新增 `PUBLISH_RELEASE_ROOT`、`PUBLISH_CURRENT_DIR`、`PUBLISH_JOB_TIMEOUT`、`PUBLISH_MAX_RETRIES`。
  - 在 `db/migrations/20260710000500_create_publish_jobs.sql` 和增量迁移中补 `log_json`、`canceled_at`。
  - 重写 `apps/api/internal/publisher/job.go`：
    - 发布流程写入结构化 job logs。
    - 增加 `RetryJob`、`CancelJob`。
    - 增加阶段性状态守卫和取消检查。
    - release promote/rollback 先校验 release，再切 `current` 目录。
    - `current` 不可用时回滚到备份或清理当前目录。
  - 在 `apps/api/internal/httpapi/router.go` 暴露 job retry/cancel API。
  - 在 `apps/admin/src/App.tsx` 的 Publishing Center 补 job logs、Retry、Cancel 操作。
  - 在 `apps/site/layouts/_partials/comments/include.html` 改为读 `HUGO_COMMENTS_API_BASE`。
  - 更新 `infra/docker/compose.prod.yml`，新增 `site` 服务并将其根目录指向 `/data/current`。
  - 更新 `.env.example`、`infra/docker/.env.prod.example`、`docs/operations/deployment-runbook.md`。
- 产出：
  - `apps/api/internal/config/config.go`
  - `apps/api/internal/model/publish.go`
  - `db/migrations/20260710000500_create_publish_jobs.sql`
  - `db/migrations/20260710000600_enhance_publish_jobs.sql`
  - `apps/api/internal/publisher/job.go`
  - `apps/api/internal/httpapi/router.go`
  - `apps/admin/src/App.tsx`
  - `apps/site/layouts/_partials/comments/include.html`
  - `infra/docker/compose.prod.yml`
  - `infra/docker/site.nginx.conf`
  - `.env.example`
  - `infra/docker/.env.prod.example`
  - `docs/operations/deployment-runbook.md`
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\e2e-smoke.ps1 -SkipRollback`：通过。
  - smoke 结果：
    - slug：`e2e-smoke-20260710194023`
    - job：`7973c8d3-4079-471e-92b8-db01cbb125ae`
    - job trace：`requested -> building -> published`
    - release key：`rel_20260710_194024_7973c8d3`
  - 生产/本地 compose 变量口径已统一到 `HUGO_COMMENTS_API_BASE`。
- 决策记录：
  - `current` 采用目录切换方案，而不是依赖软链接。
  - 评论 API base 通过 `HUGO_COMMENTS_API_BASE` 走 Hugo 允许的环境变量白名单。
  - job 日志以 `log_json` 存储，便于 Admin 和后续排障。
- 仍在保留的 verifier gap：
  - sitemap 新文章收录仍偶发 warning，未纳入阻断条件。
- 下一步：
  - 继续收口 sitemap verifier 或补更强发布探测。
  - 推进媒体引用追踪与站点设置/页面管理。

## 2026-07-10 17:47 +08:00 - CENTER - QA-P4-E2E-SMOKE

- 目标：把当前全栈博客核心闭环固化成可重复运行的端到端冒烟脚本，降低后续重构和生产化时的回归风险。
- 输入：
  - 已完成的 API/Admin/Hugo/评论/异步发布/rollback 最小闭环。
  - `docs/qa/test-strategy.md` 的冒烟清单。
  - 子 agent Heisenberg 的只读 QA 审阅意见。
- 操作：
  - 新增 `scripts/qa/e2e-smoke.ps1`。
  - 新增 `docs/qa/e2e-smoke.md`。
  - 脚本覆盖：`/healthz`、`/readyz`、Admin 登录、创建分类/标签、公开 taxonomy 校验、媒体上传和公开访问、创建文章、发布前评论失败、异步发布 job、release manifest、首页/文章页/RSS/分类页/标签页产物、文章 HTML 内容、评论容器、pending 评论不可见、Admin pending 列表、审核后公开可见、promote 旧 release 回滚和 active 唯一性。
  - 根据 Heisenberg 审阅修正：明确 PowerShell 7+、共享文件系统前提、rollback baseline 要求、pending 评论检查、taxonomy 检查、artifact 内容检查。
  - 将 sitemap 新文章收录检查暂设为 warning，并记录为发布 verifier gap。
- 产出：
  - `scripts/qa/e2e-smoke.ps1`
  - `docs/qa/e2e-smoke.md`
- 验证：
  - 首次运行发现 `sitemap.xml` 不包含新文章 slug，脚本失败；经确认 RSS、首页、文章页、taxonomy 页均包含新文章。
  - 调整 sitemap 新文章收录为 warning 后重新运行：
    - 命令：`pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\e2e-smoke.ps1`
    - 结果：通过。
    - slug：`e2e-smoke-20260710174636`
    - job：`9beb9d38-edf0-4270-afbf-803f26d2c9ee`
    - job trace：`requested -> requested -> building -> published`
    - release：`rel_20260710_174638_9beb9d38`
    - media：`efac3661-1560-4fd8-9e46-0a2299b3d2da`
    - comment：`a90723c1-dc47-4768-9b20-d38e6e78ff82`
    - rollback release：`rel_20260710_174520_9e0c2590`
- 决策记录：
  - 该脚本定位为本地/开发验收冒烟，依赖脚本与 API/worker 共享文件系统。
  - PowerShell 7+ 是明确前提。
  - Sitemap 新文章收录是后续发布 verifier 修复项，不阻塞当前 E2E 脚本通过。
- 下一步：
  - `OPS-P4-001`：补 Dockerfile、生产 compose、API/worker/site 进程拆分和 release/current 线上切换策略。
  - `PUB-P5-001`：补发布 verifier，尤其是 sitemap 新文章收录、retry/cancel/timeout 和结构化构建日志。

## 2026-07-10 17:06 +08:00 - CENTER - PHASE3-ASYNC-PUBLISH-WORKER-SMOKE

- 目标：把发布从 API 请求内同步 Hugo build 拆为异步 job + publish worker，避免后台请求阻塞，并为生产独立 worker 进程做准备。
- 输入：
  - 已完成的 publish job/release/manifest 与 rollback/promote。
  - `docs/architecture/publishing-pipeline.md` 约定的 `requested -> snapshotting -> building -> verifying -> published` 状态机。
  - 子 agent Aristotle：Admin 侧适配异步发布返回。
- 操作：
  - 新增 `apps/api/internal/publisher/worker.go`，实现 `ClaimNextJob`、`Worker.Run`、`Worker.ProcessOne`。
  - `ClaimNextJob` 使用 PostgreSQL `FOR UPDATE SKIP LOCKED`，从 `requested` job 中串行领取任务并标为 `queued`。
  - 新增 `apps/api/cmd/worker/main.go`，支持独立 worker 进程。
  - 更新 `apps/api/cmd/api/main.go`，开发环境默认通过 `PUBLISH_WORKER_ENABLED=true` 内嵌启动 worker。
  - 更新 `POST /api/v1/admin/posts/:id/publish`，仅更新文章状态、创建 job，并返回 HTTP 202 与 `{post, job}`。
  - 更新 Admin：Publish 成功提示 job queued，发布中心展示 `requested/queued/snapshotting/building/verifying/published/failed`，失败时展示 `error_code/error_message`。
  - 更新 `.env.example` 与 `scripts/dev/README.md`，记录 worker 开关和独立 worker 启动命令。
- 产出：
  - `apps/api/internal/publisher/worker.go`
  - `apps/api/cmd/worker/main.go`
  - `apps/api/cmd/api/main.go`
  - `apps/api/internal/httpapi/router.go`
  - `apps/api/internal/config/config.go`
  - `apps/admin/src/App.tsx`
  - `.env.example`
  - `scripts/dev/README.md`
- 验证：
  - `go test ./...` in `apps/api`：通过，包含 `cmd/worker`。
  - `npm run build` in `apps/admin`：通过。
  - 重启本地 API 后 `GET /readyz`：通过。
  - 创建 `async-worker-smoke-20260710170639` 并调用 publish：HTTP status `202`。
  - publish 初始 job status：`requested`。
  - worker 后台处理后 final job status：`published`。
  - status trace：`requested -> building -> published`。
  - release key：`rel_20260710_170640_21750c9a`。
  - release 为 active，`manifest.json` 存在，`p/async-worker-smoke-20260710170639/index.html` 存在。
- 决策记录：
  - 开发环境默认内嵌 worker，降低本地启动复杂度。
  - 生产环境可将 API 设置 `PUBLISH_WORKER_ENABLED=false`，另启 `go run ./cmd/worker` 或容器 worker。
  - 当前 worker 已使用 `SKIP LOCKED` 防重复领取；更完整的 retry/cancel/timeout 留到 QA/OPS 后续增强。
- 下一步：
  - `QA-P4-001`：将写作、媒体、评论、发布、回滚、C 端查看整理为可重复冒烟脚本。
  - `OPS-P4-001`：补 Dockerfile、生产 compose、API/worker/site 进程拆分和 release/current 线上切换策略。

## 2026-07-10 16:01 +08:00 - CENTER - PHASE3-ROLLBACK-COMMENTS-SMOKE

- 目标：补齐发布 rollback/promote 与 C 端评论嵌入两个关键闭环，并继续按中心窗口 + 子 agent 方式记录可接力日志。
- 输入：
  - 上一稳定点：publish job/release/manifest 已完成，release `rel_20260710_154328_70397d1f` 为 active。
  - `docs/architecture/publishing-pipeline.md` 的回滚流程要求：检查 release 产物，切换 active 指针，不重写历史 release。
  - 已有 Public Comments API：`GET/POST /api/v1/public/posts/:slug/comments`。
  - 子 agent Singer：负责 `apps/site` 本地 Hugo comments partial、TS、SCSS 接入。
- 操作：
  - 新增 `publisher.PromoteRelease`，promote 前检查 release 输出目录、`manifest.json` 和 `index.html`。
  - 修正新发布时旧 release 状态：旧 active release 同时设置 `is_active=false` 与 `status=inactive`。
  - 新增 `POST /api/v1/admin/publish/releases/:id/promote`。
  - 更新 Admin Publishing Center：release 表格增加 status、promoted time、Promote 操作。
  - Singer 在 `apps/site` 增加 `public-api` comments 覆盖、`publicComments.ts` 和 `public-comments.scss`。
  - 中心窗口补边界：评论 partial 尊重 `.Params.comments=false`，且 `public-api` 只在 `post` section 渲染，非 public-api provider 仍保留原 Stack provider 路径。
- 产出：
  - `apps/api/internal/publisher/job.go`
  - `apps/api/internal/httpapi/router.go`
  - `apps/admin/src/App.tsx`
  - `apps/site/config/_default/params.toml`
  - `apps/site/layouts/_partials/comments/include.html`
  - `apps/site/assets/ts/publicComments.ts`
  - `apps/site/assets/scss/public-comments.scss`
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `.tools\hugo\hugo.exe --source apps/site --destination $env:TEMP\zoking-site-build-main --minify`：通过。
  - 创建发布 smoke 文章 `rollback-smoke-20260710155734`，生成 release `rel_20260710_155734_979062a5`。
  - 调用 `POST /api/v1/admin/publish/releases/{old_id}/promote`，将 `rel_20260710_154328_70397d1f` 切回 active。
  - promote 后 `active_count=1`，新 release `rel_20260710_155734_979062a5` 为 inactive，旧 release 为 active。
  - Hugo 生成物抽查：`p/release-smoke-20260710154328/index.html` 含 `data-public-comments`；`about/index.html` 不含评论容器。
  - Public Comments 真实链路：对 `release-smoke-20260710154328` 提交评论 `9e7701a0-0ae4-4639-8b6a-5fe9c9b90b59` 返回 `pending`；后台审核为 `approved`；公开列表能读取到该评论。
- 决策记录：
  - rollback/promote 当前先切换 DB active release 指针；线上 `current` symlink/目录指针仍留到 OPS 发布阶段。
  - C 端评论采用自研 `public-api` provider 覆盖，不改根主题核心；API 失败只展示提示，不影响文章阅读。
  - 当前 publish 仍是 API 请求内同步执行，下一阶段应拆为 worker。
- 下一步：
  - `PUB-P3-008`：将同步发布处理拆为后台 worker 或 worker command。
  - `QA-P4-001`：把登录、写作、媒体、评论、发布、回滚、C 端查看整理为可重复冒烟脚本。
  - `OPS-P4-001`：补 Dockerfile、生产 compose、环境变量和 release/current 切换策略。

## 2026-07-10 14:43 +08:00 - CENTER - DOC-PHASE1-SYNC

- 目标：把运行时文档口径对齐到 Phase 1 最小发布闭环已完成。
- 输入：当前代码已具备 `apps/site`、`apps/api`、`apps/admin` 可运行基线，Admin 发布后可写入 Hugo content 并在前台可见。
- 操作：
  - 更新 `docs/process/context-handoff.md`，把当前阶段从 Phase 0 调整为 Phase 1。
  - 更新 `docs/process/task-board.md`，将 `PUB-P1-001` 标记为 Done，并保留后续发布演进方向。
  - 更新 `docs/plan/goal-plan.md`，把下一步聚焦到 API-P1-003、ADMIN-P2-003 和发布演进。
- 产出：
  - `docs/process/context-handoff.md`
  - `docs/process/task-board.md`
  - `docs/plan/goal-plan.md`
- 验证：
  - 文档状态与当前代码实现一致。
  - 运行时任务板和接力文档已从 Phase 0 叙述切换到 Phase 1 叙述。
- 决策记录：
  - 当前发布路径仍以同步写 Hugo content 为最小闭环；publish job / worker / release manifest 留到后续阶段。
- 下一步：
  - 继续推进文章 CRUD、分类标签、媒体、评论 API。
  - 把 Admin 文章列表和编辑器接到真实后端。

## 2026-07-10 15:12 +08:00 - CENTER - PHASE1-TAXONOMY-SMOKE

- 目标：把 taxonomy 最小闭环从数据库、API 到 Admin/C 端发布链路全部跑通。
- 输入：
  - 已完成的 Phase 1 最小发布闭环。
  - `db/migrations/20260710000200_create_taxonomy.sql` 新增的 categories/tags/post_categories/post_tags。
  - `apps/api/internal/httpapi/router.go`、`taxonomy.go`、`post_helpers.go`、`publisher/hugo.go` 的 taxonomy 支持。
  - `apps/admin/src/App.tsx` 的 taxonomy workbench 与 post multi-select。
- 操作：
  - 更新迁移并运行 `go run ./cmd/migrate up`。
  - 运行 `go run ./cmd/seed`，补种默认分类/标签与种子文章关联。
  - 重启 API 进程到新代码。
  - 通过 Admin API 创建 `taxonomy-smoke-20260710151103`，绑定 `Development`、`Go`、`Hugo`。
  - 发布文章并运行 Hugo build。
- 产出：
  - 分类/标签 CRUD API。
  - Admin taxonomy workbench。
  - front matter taxonomy 输出。
  - 新烟雾文章 `taxonomy-smoke-20260710151103`。
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `go run ./cmd/migrate up`：通过，迁移到 `20260710000200`。
  - `go run ./cmd/seed`：通过。
  - `GET http://localhost:18080/api/v1/public/categories`：返回 2 个分类。
  - `GET http://localhost:18080/api/v1/public/tags`：返回 3 个标签。
  - `POST /api/v1/admin/posts` + `/publish`：taxonomy smoke 成功。
  - `D:\zoking\zoking-blog\apps\site\content\post\taxonomy-smoke-20260710151103\index.md`：存在且 front matter 含 categories/tags。
  - `D:\zoking\zoking-blog\dist\site\p\taxonomy-smoke-20260710151103\index.html`：存在。
  - `GET http://localhost:1313/p/taxonomy-smoke-20260710151103/`：200。
- 决策记录：
  - taxonomy 先落在内容域与 front matter，未立即扩展 media/comments。
  - Admin 文章编辑器现在同时承担 taxonomy 绑定与基础工作台职责。
- 下一步：
  - 补媒体 schema 与媒体 API。
  - 补评论 schema 与评论审核 API。
  - 再推进 publish_jobs / release / rollback。

## 2026-07-10 15:24 +08:00 - CENTER - PHASE1-MEDIA-SMOKE

- 目标：把媒体库最小闭环从数据库、API、静态访问到 Admin 工作台跑通。
- 输入：
  - `MEDIA_LOCAL_DIR`、`MEDIA_STORAGE_DRIVER` 等已有环境变量约定。
  - 数据模型文档里的 `media_assets` 与 `media_usages` 规划。
  - 已有 Admin 工作台和文章 Markdown 编辑器。
- 操作：
  - 新增 `db/migrations/20260710000300_create_media.sql`，创建 `media_assets`、`media_usages`。
  - 新增 `apps/api/internal/model/media.go`。
  - 扩展 `apps/api/internal/config/config.go`，读取媒体存储目录、公开前缀和大小限制。
  - 新增 `apps/api/internal/httpapi/media.go`，实现图片上传、列表、详情、删除、静态访问。
  - 更新 `apps/api/internal/httpapi/router.go`，挂载 `/media-files/*filepath` 与后台 `/api/v1/admin/media`。
  - 更新 `apps/admin/src/App.tsx`，新增 Media Library，支持上传、预览、复制 URL、插入 Markdown、删除。
- 产出：
  - 媒体表与引用表。
  - 本地媒体文件存储 `storage/media`。
  - Admin 媒体库基础版。
  - `/media-files/...` 静态访问。
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `go run ./cmd/migrate up`：通过，迁移到 `20260710000300`。
  - `go run ./cmd/seed`：通过。
  - `POST /api/v1/admin/media` 上传 1x1 PNG：成功。
  - 上传结果：`38ddafbf-c709-444a-be4b-3f131c2da01f`，URL `/media-files/2026/07/a2d19c48-e2f9-48c9-b59a-2941b9353b34.png`。
  - `GET http://localhost:18080/media-files/2026/07/a2d19c48-e2f9-48c9-b59a-2941b9353b34.png`：200。
  - `GET /api/v1/admin/media`：能返回该资源。
  - 文件存在：`D:\zoking\zoking-blog\storage\media\2026\07\a2d19c48-e2f9-48c9-b59a-2941b9353b34.png`。
- 决策记录：
  - Phase 1 媒体先支持本地图片存储，生产对象存储仍按配置预留。
  - 当前媒体 URL 通过 API 静态路由服务，后续发布到 Hugo release 时再复制或 CDN 固化。
  - `media_usages` 已建表，但自动解析文章 Markdown 引用关系留到发布/媒体增强阶段。
- 下一步：
  - 补评论 schema、公开提交与后台审核。
  - 补媒体引用追踪与封面字段。
  - 推进 publish_jobs / release / rollback。

## 2026-07-10 15:33 +08:00 - CENTER - PHASE1-COMMENTS-SMOKE

- 目标：把评论最小闭环从 Public API、Admin API 到后台审核工作台跑通。
- 输入：
  - API 契约里的 `GET/POST /api/v1/public/posts/:slug/comments`。
  - API 契约里的后台 `/api/v1/admin/comments`、`/moderation`、`/reply`、`DELETE`。
  - 数据模型文档里的 `comments` 规划。
- 操作：
  - 新增 `db/migrations/20260710000400_create_comments.sql`，创建 `comments` 表和状态/文章索引。
  - 新增 `apps/api/internal/model/comment.go`。
  - 新增 `apps/api/internal/httpapi/comments.go`，实现公开评论读取/提交、后台评论列表、审核、回复、删除。
  - 更新 `apps/api/internal/httpapi/router.go`，挂载 Public/Admin 评论路由。
  - 更新 `apps/admin/src/App.tsx`，新增 Comment Moderation 表格和审核操作。
- 产出：
  - 匿名评论提交进入 `pending`。
  - 公开评论列表只返回 `approved`。
  - 后台可查看、通过、拒绝、标记 spam、删除评论。
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `go run ./cmd/migrate up`：通过，迁移到 `20260710000400`。
  - `go run ./cmd/seed`：通过。
  - 对 `taxonomy-smoke-20260710151103` 提交评论：返回 `pending`。
  - 审核前 `GET /api/v1/public/posts/taxonomy-smoke-20260710151103/comments`：0 条。
  - 后台 `PATCH /api/v1/admin/comments/49f960b1-ac87-4b7d-8440-3ad4e54b7ede/moderation` 设置 `approved`：成功。
  - 审核后公开评论列表：1 条。
  - 后台评论列表可查到 `49f960b1-ac87-4b7d-8440-3ad4e54b7ede`。
- 决策记录：
  - MVP 评论采用匿名提交 + 后台审核。
  - Public API 不暴露 pending/rejected/spam 评论。
  - C 端 Hugo 页面嵌入评论组件留到 `HUGO-P3-001`。
- 下一步：
  - 推进 `PUB-P3-001` 发布数据视图与任务化发布入口。
  - 推进 `PUB-P3-005` 发布任务状态机。
  - 推进 `HUGO-P3-001` 将评论 API 嵌入 C 端页面。

## 2026-07-10 15:44 +08:00 - CENTER - PHASE3-PUBLISH-JOB-RELEASE-SMOKE

- 目标：把同步写 Hugo content 的发布闭环升级为可追踪的 publish job / release / manifest 最小闭环。
- 输入：
  - 已完成的内容域最小闭环。
  - `docs/architecture/publishing-pipeline.md` 对 publish_jobs、状态机、release manifest 的设计。
  - 现有 `apps/api/internal/publisher/hugo.go` 写 content 能力。
- 操作：
  - 新增 `db/migrations/20260710000500_create_publish_jobs.sql`，创建 `publish_jobs`、`publish_releases`。
  - 新增 `apps/api/internal/model/publish.go`。
  - 新增 `apps/api/internal/publisher/job.go`，实现同步处理的 job 状态流转、Hugo build、verify、manifest、active release。
  - 扩展 `apps/api/internal/config/config.go`，支持 `HUGO_PUBLIC_DIR` 和 `HUGO_BIN`。
  - 改造 `POST /api/v1/admin/posts/:id/publish`，返回 post/job/release。
  - 新增 `GET /api/v1/admin/publish/jobs` 与 `/publish/releases`。
  - 更新 `apps/admin/src/App.tsx`，新增 Publishing Center 展示 job/release。
- 产出：
  - publish job/release 数据视图。
  - release 构建目录 `dist/releases/{release_key}`。
  - `manifest.json`。
  - Admin 发布中心基础版。
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `go run ./cmd/migrate up`：通过，迁移到 `20260710000500`。
  - `go run ./cmd/seed`：通过。
  - 发布 smoke 文章 `release-smoke-20260710154328`：job 状态为 `published`。
  - release key：`rel_20260710_154328_70397d1f`。
  - `D:\zoking\zoking-blog\dist\releases\rel_20260710_154328_70397d1f\manifest.json`：存在。
  - `D:\zoking\zoking-blog\dist\releases\rel_20260710_154328_70397d1f\p\release-smoke-20260710154328\index.html`：存在。
  - `GET http://localhost:1313/p/release-smoke-20260710154328/`：200。
  - `GET /api/v1/admin/publish/jobs` 与 `/publish/releases`：能查到本次发布记录。
- 决策记录：
  - 本阶段先采用同步处理的 publish job，保留后续拆成 worker 的空间。
  - active release 当前记录在数据库，尚未切换线上 `current` 指针。
  - release 产物放在 `dist/releases/{release_key}`，当前 Hugo dev server 仍读取 `apps/site`。
- 下一步：
  - 实现历史 release promote/rollback。
  - 将评论 API 嵌入 C 端 Stack 页面。
  - 将同步发布处理拆成后台 worker。

## 2026-07-10 14:32 +08:00 - CENTER - PHASE1-PUBLISH-SMOKE

- 目标：验证最小发布闭环可在稳定代码和配置下重复运行，并把结果写入工作日志。
- 输入：
  - 上一轮 Phase 0 基线成果。
  - 子 agent Carver 对发布闭环的审阅：P0 先同步写 Hugo content，不急着引入 publish_jobs。
  - 子 agent Meitner 对 Admin 接入的审阅：先接登录、列表、草稿、发布，保留最小改动。
- 操作：
  - 将 `apps/api/internal/publisher/hugo.go` 硬化：slug 校验、空正文拒绝、front matter 仅保留基础字段、路径转为绝对目录。
  - 将 `apps/api/internal/config/config.go` 路径解析改为仓库根下绝对路径，避免工作目录依赖。
  - 将 `apps/admin/src/App.tsx` 改成可登录、读写文章、发布文章的工作台界面。
  - 重启 API，执行登录 -> 新建文章 -> 发布 -> Hugo 重建 -> 前台可见的 smoke。
- 产出：
  - `apps/api/internal/publisher/hugo.go`
  - `apps/api/internal/config/config.go`
  - `apps/api/internal/httpapi/router.go`
  - `apps/admin/src/App.tsx`
  - 继续沿用 `apps/site/content/post/{slug}/index.md`
- 验证：
  - `go test ./...` in `apps/api`：通过。
  - `npm run build` in `apps/admin`：通过。
  - `POST /api/v1/admin/posts`：创建草稿成功。
  - `POST /api/v1/admin/posts/:id/publish`：发布成功，返回 `content_path`。
  - `Test-Path D:\zoking\zoking-blog\apps\site\content\post\admin-published-20260710143621\index.md`：True。
  - `D:\zoking\zoking-blog\.tools\hugo\hugo.exe --source D:\zoking\zoking-blog\apps\site --destination D:\zoking\zoking-blog\dist\site --minify`：通过。
  - `Test-Path D:\zoking\zoking-blog\dist\site\p\admin-published-20260710143621\index.html`：True。
  - `GET http://localhost:1313/p/admin-published-20260710143621/`：200。
  - `GET http://localhost:18080/api/v1/public/posts`：两篇已发布文章返回。
- 决策记录：
  - P1 最小闭环采用同步写 Hugo content，而非立即上 publish_jobs / worker。
  - 发布内容路径固定为 `apps/site/content/post/{slug}/index.md`。
  - 发布前硬性拒绝空正文和危险 slug。
  - 文章字段里的分类、标签、媒体先不强制映射，后续由专用表接管。
- 风险/问题：
  - 仍未实现分类标签、媒体、评论、发布 worker、版本回滚。
  - Admin 的草稿列表/编辑体验还不够“工作台化”。
- 下一步：
  - 将 Admin 文章列表做成可编辑/可发布的工作台。
  - 实现分类标签与媒体最小 API。
  - 引入发布日志和 snapshot/release 概念。

## 2026-07-10 14:15 +08:00 - CENTER - PHASE0-BASELINE

- 目标：按照 `https://demo.stack.cai.im/` 的最终 C 端效果方向，在主控审计 + 子窗口模式下落地前后端分离、有 PostgreSQL 数据库的全栈博客 Phase 0 工程基线。
- 输入：
  - 用户目标：C 端最终页面接近 Stack demo，项目为前后端分离、有数据库的博客系统。
  - 现有文档：工程总控手册、WBS、API 契约、数据模型、发布流水线、Stack 集成规范。
  - 浏览器/HTTP 检查：目标 demo 为三栏 Stack 卡片博客布局，含左侧站点栏、中间文章卡片、右侧搜索/归档/分类/标签。
  - 子 agent Godel：建议 API 独立 Go module，仓库级 migrations，Phase 0 先完成 health/ready/migrate/seed。
- 操作：
  - 从 `demo/` 复制配置、内容和资源到 `apps/site`，并改为本地 Hugo module 引用 `github.com/CaiJimmy/hugo-theme-stack/v4 => ../..`。
  - 新增 `apps/api`，使用 Gin + GORM + PostgreSQL，包含 `/healthz`、`/readyz`、公开文章列表、后台登录、后台文章基础接口。
  - 新增 goose SQL migration：扩展、用户、RBAC、refresh token、posts、site settings、audit logs。
  - 新增 seed：权限、角色、超级管理员、站点设置、数据库示例文章。
  - 新增 `apps/admin`，使用 React + Vite + Ant Design，提供后台 dashboard、API/DB 状态、文章表格和编辑器壳。
  - 新增 `infra/docker/compose.dev.yml`，PostgreSQL 开发端口使用 `15432`，避免占用本机 5432。
  - 新增 `.env.example`、根 README、开发命令说明。
- 产出：
  - `apps/site/*`
  - `apps/api/*`
  - `apps/admin/*`
  - `db/migrations/20260710000100_create_core.sql`
  - `infra/docker/compose.dev.yml`
  - `.env.example`
  - `scripts/dev/README.md`
  - 更新根 `README.md`、`.gitignore`
- 验证：
  - `go mod tidy`：通过。
  - `go test ./...` in `apps/api`：通过。
  - `docker compose -f infra/docker/compose.dev.yml up -d --force-recreate postgres`：通过。
  - `go run ./cmd/migrate up`：通过，迁移到 `20260710000100`。
  - `go run ./cmd/seed`：通过。
  - `GET http://localhost:18080/healthz`：返回 `status=ok`。
  - `GET http://localhost:18080/readyz`：返回 `status=ready`。
  - `GET http://localhost:18080/api/v1/public/posts`：返回 PostgreSQL seed 文章 `Hello from PostgreSQL`。
  - `POST /api/v1/admin/auth/login`：默认管理员 `admin@zoking.local` / `ChangeMe123!` 可登录并返回 JWT。
  - `npm install` in `apps/admin`：通过。
  - `npm run build` in `apps/admin`：通过。
  - Hugo Extended `0.160.1` 本地工具下载到 `.tools/hugo`。
  - `hugo --source apps/site --destination dist/site --minify`：通过，生成多语言 Stack 页面。
  - 本地服务：API `http://localhost:18080`，C 端 `http://localhost:1313`，Admin `http://localhost:5173`。
  - Playwright CLI 截图：
    - `.tools/screenshots/site-home.png`：确认 Stack 三栏卡片布局。
    - `.tools/screenshots/admin-home-after-api.png`：确认 Admin 能读到 API 中的 PostgreSQL 文章。
- 决策记录：
  - Phase 0 默认采用 Hybrid CMS 架构。
  - B 端采用 React + Vite + Ant Design 起步。
  - PostgreSQL 开发宿主端口采用 `15432`，避免与本机已有 5432 冲突。
  - API 开发端口采用 `18080`，避免本机 8080 冲突。
  - C 端前台保留 Hugo Theme Stack，本阶段不重写主题核心源码。
- 风险/问题：
  - 目标尚未完成，当前只是可运行工程基线。
  - 后台编辑器按钮仍是壳，尚未写入 API。
  - C 端内容仍主要来自 Hugo demo 快照，尚未由数据库发布 worker 自动生成。
  - 媒体库、评论审核、分类标签后台、发布中心状态机仍未实现。
- 下一步：
  - `API-P1-003`：完善文章 CRUD、分类标签、媒体、评论 API。
  - `ADMIN-P2-003`：将后台文章列表/编辑器接入真实 Admin API。
  - `PUB-P1-001`：实现 DB -> Hugo content snapshot -> Hugo build 的发布 worker。

## 2026-06-18 12:45 +08:00 - CENTER - DOC-P0-002

- 目标：把全栈博客系统的工程工作、goal 目标计划实施、多子 agent 调度、中心窗口汇报总控、工作日志与新窗口接力机制写入文档，达到可直接进入 Phase 0 实施的工程级详细度。
- 输入：
  - 用户要求“总窗口多个子agent干活，对话窗口负责汇报跟中心调控，产工作日志，等总量爆了就开新窗口读日志继续干”。
  - 已有需求、架构、后端、数据库、前端、运维、计划和协作文档。
  - 子 agent Ramanujan 的规划缺口审阅。
  - 子 agent Descartes 的协作机制审阅。
  - Hugo Theme Stack、Hugo 内容管理、headless CMS/博客 CMS 公开资料。
- 操作：
  - 使用 Architecture 和 Agent Orchestrator 技能约束架构与多 agent 流程。
  - 派发两个只读 explorer 子 agent 审阅规划缺口和协作机制缺口。
  - 中心窗口统一写入新增文档，避免多个 agent 同写文件。
  - 更新文档索引、goal 计划、WBS、协作规则、goal 执行规范、上下文接力说明。
- 产出：
  - `docs/plan/engineering-execution-master-plan.md`
  - `docs/process/task-board.md`
  - `docs/process/multi-agent-execution-sop.md`
  - `docs/backend/00-api-contract.md`
  - `docs/backend/01-auth-rbac-design.md`
  - `docs/database/00-data-model.md`
  - `docs/database/01-migration-and-seed-strategy.md`
  - `docs/architecture/publishing-pipeline.md`
  - `docs/architecture/frontmatter-and-snapshot-mapping.md`
  - `docs/frontend/site-stack-integration.md`
  - `docs/frontend/admin-tech-decision.md`
  - `docs/security/security-baseline.md`
  - `docs/qa/test-strategy.md`
  - `docs/operations/deployment-runbook.md`
  - `docs/references/sources.md`
- 验证：
  - 已检查 `docs/` 文件列表。
  - 已检查 `git status --short --branch`，当前仍只有 `docs/` 新增。
  - 已把 `DOC-P0-002` 标记为 Done 并释放文档文件锁。
- 决策记录：
  - 实施前 P0 决策集中记录在 `docs/plan/engineering-execution-master-plan.md` 和 `docs/process/task-board.md`。
  - 默认建议仍为 Hybrid CMS、React + Ant Design Pro 风格后台、开发本地媒体存储/生产对象存储预留、单 VPS Docker Compose 起步、匿名评论 + 后台审核。
- 风险/问题：
  - 仍未开始代码实现和 monorepo 改造。
  - ADR-0002 到 ADR-0015 仍需在实施前逐步补齐或确认。
  - P0 决策仍需用户最终确认。
- 下一步：
  - 等用户确认进入实施后，从 `DEC-P0-001` 到 `DEC-P0-008` 开始关闭决策。
  - 然后执行 `ARCH-P0-001`、`OPS-P0-001`、`API-P0-001`、`DB-P0-001`、`HUGO-P0-001`、`ADMIN-P0-001`。

## 2026-06-18 11:42 +08:00 - CENTER - REQ-ANALYSIS-001

- 目标：根据用户要求落实需求分析，并写入文档。
- 输入：
  - 用户要求使用 Hugo Theme Stack 构建博客。
  - 用户要求多个子 agent 干活，中心窗口负责汇报和调控。
  - 用户要求产工作日志，上下文过大时开新窗口读日志继续。
  - 官方 Stack 文档、Hugo 文档、Google SEO 文档、web.dev、W3C WAI。
- 操作：
  - 启动三个子 agent：
    - 博客产品功能调研 agent。
    - Stack 能力映射 agent。
    - 需求工程 agent。
  - 中心窗口同步进行网页资料核对。
  - 整合子 agent 结论到需求文档。
- 产出：
  - `docs/README.md`
  - `docs/requirements/00-index.md`
  - `docs/requirements/01-blog-feature-research.md`
  - `docs/requirements/02-stack-capability-map.md`
  - `docs/requirements/03-mvp-scope.md`
  - `docs/process/agent-collaboration.md`
  - `docs/process/context-handoff.md`
  - `docs/process/worklog.md`
- 子 agent 结果摘要：
  - 常见博客功能：P0 必须包括首页、文章页、About、归档、搜索、分类标签、RSS、SEO、移动端、无障碍、性能、写作部署流程。
  - Stack 能力：大多数功能可通过配置和内容约定完成；新 widget、评论 provider、深度视觉改造才需要自定义代码。
  - 需求工程：建议建立 `docs/requirements/` 与 `docs/process/`，用工作日志和接力文档保证多窗口连续性。
- 验证：
  - 已检查文档文件列表。
  - 已检查文档标题结构。
  - 已检查 `git status --short --branch`，当前只有 `docs/` 新增。
- 风险/问题：
  - 目前仍未确认最终仓库策略。
  - 评论、统计、多语言、部署平台需要用户确认。
  - 当前目录是主题源码，不是最终站点工程。
- 下一步：
  - 复核文档状态。
  - 汇报本轮文档产物。
  - 等用户确认是否进入实际实施。

## 2026-06-18 11:42 +08:00 - CENTER - FULLSTACK-PLAN-001

- 目标：把用户确认的“成熟完整全栈博客系统”目标写成可执行工程规划文档。
- 输入：
  - 用户要求 C 端保留 Hugo Theme Stack 页面设计效果。
  - 用户要求 B 端后台控制。
  - 用户要求 Go Gin + GORM + PostgreSQL。
  - 用户要求多个子 agent 干活，中心窗口汇报和总控。
  - 用户要求产工作日志，上下文过大时开新窗口读日志继续。
- 操作：
  - 启动四个子 agent：
    - 系统架构/ADR agent。
    - 后端数据库 agent。
    - 产品需求/前后台 agent。
    - 运维实施计划 agent。
  - 中心窗口整合子 agent 结论。
  - 写入架构、需求、后端、数据库、前端、运维、计划、执行规范文档。
- 产出：
  - `docs/architecture/00-system-overview.md`
  - `docs/architecture/01-architecture-decisions.md`
  - `docs/architecture/adr/adr-0001-hybrid-hugo-gin-cms.md`
  - `docs/requirements/04-fullstack-blog-system.md`
  - `docs/requirements/05-admin-and-reader-scope.md`
  - `docs/backend/backend-plan.md`
  - `docs/database/database-plan.md`
  - `docs/frontend/frontend-plan.md`
  - `docs/operations/README.md`
  - `docs/plan/goal-plan.md`
  - `docs/plan/work-breakdown-structure.md`
  - `docs/process/goal-execution.md`
- 关键决策草案：
  - 采用 Hybrid CMS：后台动态管理 + Hugo 静态发布。
  - PostgreSQL 为编辑源。
  - Hugo content 为发布快照。
  - C 端保留 Hugo Theme Stack。
  - Go Gin + GORM 构建模块化单体 API。
  - 发布由 publish job / worker 执行。
- 风险/问题：
  - 仍需用户确认 B 端技术栈。
  - 仍需确认生产媒体存储与部署目标。
  - 当前仓库仍是主题源码形态，尚未改造为 monorepo。
- 下一步：
  - 复核文档文件和标题结构。
  - 关闭已完成子 agent。
  - 汇报本轮产物。
## 2026-07-11 09:35 +08:00 - CENTER - PREVIEW-P5-001

- 目标：完成文章草稿、独立页面和发布前站点设置的隔离预览。
- 子 agent：Euler 只读审阅，确认预览必须复用 Hugo 构建能力，但与 publish job/release/current 完全隔离。
- 实现：新增 `publish_previews` migration/model、预览 builder、文章/页面/设置预览 API、静态预览路由、Admin Preview 按钮和 Publishing Center 预览记录表。
- 运行配置：新增 `PUBLISH_PREVIEW_ROOT`、`PUBLISH_PREVIEW_PUBLIC_BASE_URL`、`PUBLISH_PREVIEW_TTL`，生产 compose 的 API/worker 共用 `/data/previews`。
- 不变量：预览使用临时站点副本，不写正式 Hugo content，不创建 release，不切 active/current；设置 preview 不落库；静态响应带 `X-Robots-Tag: noindex, nofollow`。
- 修复：初始化 jsonb 字段为合法 `{}`/`[]`；预览子路径 baseURL 下改用基础 release 校验加目标 HTML 存在性校验，正式 sitemap verifier 不变。
- 验证：migration 当前版本 `20260711000100`；`go test ./...`、Admin `npm run build`、生产 compose config、完整 E2E smoke 均通过。
- E2E 结果：post/page/settings 三类 preview 可读，release 数量与 active release 未变化，settings hash 未变化；随后正式发布、rollback、设置恢复全部通过。
- 状态：`PREVIEW-P5-001` Done，`LOCK-PREVIEW-P5` 已释放。
- 下一步：`AUDIT-P5-001`，为页面、设置、发布、媒体等后台操作落 audit logs。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001

- 用户反馈：后台业务全部堆叠在一个页面、界面未中文化、菜单没有真实路由。
- 根因：早期 Admin 验证版把所有领域状态和 JSX 集中在 `App.tsx`，菜单只切换选中状态，业务卡片始终无条件挂载。
- 实现：引入 `react-router-dom`，新增 `src/router.tsx`，建立 `/dashboard`、`/posts`、`/pages`、`/taxonomy`、`/media`、`/comments`、`/publishing`、`/users`、`/settings`、`/audit` 路由。
- 实现：菜单改为 URL 导航，支持地址栏直达、刷新、浏览器前进后退和路由选中状态。
- 实现：各业务模块按 `section` 条件挂载；仪表盘仅显示指标，文章页显示文章列表与编辑器，其余页面只显示本领域模块。
- 中文化：完成导航、页头、登录、仪表盘、模块标题及文章管理高频文案的第一轮中文化。
- 后端检查：格式化并验证角色 CRUD、权限列表和管理员密码重置相关路由代码，`go test ./...` 通过。
- 验证：Admin `npm run build` 通过；`scripts/qa/preflight.ps1 -SkipE2E` 通过，未生成测试文章。
- 未完成：`App.tsx` 仍承担共享状态容器，需要继续拆分 `src/pages/*` 和领域组件；页面内部剩余英文表格列、状态值和确认文案需要统一映射。

## 2026-07-11 - CENTER - ROLE-MANAGE-P5-001 / ADMIN-I18N-P5-002

- 用户与权限页新增自定义角色创建表单，可设置角色编码、名称、说明和初始权限。
- 自定义角色支持权限完整替换和删除；系统角色保持只读，已分配给用户的角色由 API 返回 409 阻止删除。
- 用户操作列新增密码重置弹窗；密码最少 10 字符、最多 72 字符，成功后 API 撤销该用户刷新令牌。
- Admin `refresh` 接入 `/api/v1/admin/permissions`，仅在拥有 `role:read` 时加载权限目录。
- 完成页面管理、分类标签、媒体清理、发布预览和用户权限页的第二轮中文化。
- 重启本地 API 进程以加载新增路由；运行态验证 `super_admin` 拥有 `role:manage`，权限目录返回 29 项。
- 验证：`npm run build`、`go test ./...`、`scripts/qa/preflight.ps1 -SkipE2E` 全部通过。
- 后续：增加自定义角色名称/说明编辑 UI；将各领域 JSX 从 `App.tsx` 提取到 `src/pages/*`；继续统一后端枚举的中文显示映射。

## 2026-07-11 - CENTER - ROLE-MANAGE-P5-001 COMPLETE

- 新增自定义角色名称与说明编辑弹窗，系统角色继续保持只读。
- 新增 `src/labels.ts`，集中维护状态和权限编码的中文展示映射，API 请求仍使用稳定英文枚举。
- 运行态验收发现并修复角色创建缺陷：PostgreSQL UUID 返回字符串不能直接扫描到 `uuid.UUID`；改为先扫描字符串再显式解析。
- 使用临时角色完成创建、编辑、权限完整替换、删除闭环；临时数据已删除。
- `go test ./...`、Admin `npm run build` 和 `preflight.ps1 -SkipE2E` 均通过。
- `ROLE-MANAGE-P5-001` 标记 Done，`LOCK-ROLE-MANAGE-P5` 释放。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 PAGE EXTRACTION

- 新建 `apps/admin/src/pages/UserManagementPage.tsx`，迁移用户创建、启停、角色分配、自定义角色创建、权限矩阵和角色操作视图。
- `/users` 已切换到独立页面组件，旧内联 JSX 已从 `App.tsx` 删除，不保留重复死代码。
- 新建 `apps/admin/src/pages/DashboardPage.tsx`，迁移健康状态和内容计数指标；`ApiStatus` 类型与状态标签归属页面组件。
- `App.tsx` 继续作为暂时的数据/API 编排容器，页面层通过显式 props 接收数据和命令回调。
- 验证：`/dashboard`、`/users` 直达均返回 200；Admin build、Go tests、无污染 preflight 全部通过。
- 下一步：按相同模式迁移 Audit、Settings、Taxonomy，再处理依赖编辑器状态较多的 Posts/Pages/Publishing 页面。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 PAGE EXTRACTION 2

- 新建 `AuditPage.tsx`，迁移审计日志表格，并完成时间、操作者、资源、结果、请求 ID 等列名中文化。
- 新建 `SettingsPage.tsx`，迁移站点设置表单、预览和发布命令；字段、按钮和默认副标题全部中文化。
- 新建 `TaxonomyPage.tsx`，迁移分类/标签表单与表格，保留原有 CRUD 回调和 Form 实例。
- 三处旧内联 JSX 已从 `App.tsx` 删除；主文件由约 2516 行下降至 2342 行。
- 直接路由 `/settings`、`/audit`、`/taxonomy` 均返回 200，Admin build 与无污染 preflight 通过。
- 下一步：迁移 Media、Comments，再拆 Posts、Pages、Publishing；之后抽取 API client/context，降低页面 props 数量。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 PAGE EXTRACTION 3

- 新建 `MediaPage.tsx`，迁移媒体上传、预览、清理预演、孤立媒体清理、复制地址、插入 Markdown 和删除保护。
- 新建 `CommentsPage.tsx`，迁移评论列表、通过/拒绝/垃圾标记和删除操作。
- 媒体和评论页面的表格列、空状态、按钮、确认框已全部中文化；状态值继续通过集中映射展示。
- 两处旧内联 JSX 已从 `App.tsx` 删除，并清理不再使用的 `Image`、`Upload` import；主文件下降至 2214 行。
- `/media`、`/comments` 直接路由返回 200；Admin build、Go tests、Hugo build 和无污染 preflight 全部通过。
- 下一步：拆分 Posts、Pages、Publishing 三个高状态耦合页面，然后抽取共享 API client 与管理后台上下文。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 PAGE EXTRACTION 4

- 新建 `PostsPage.tsx`，将文章列表和文章编辑器合并为同一领域页面；保留分类标签选择、草稿保存、预览和发布能力。
- 新建 `PagesPage.tsx`，迁移独立页面列表、编辑、归档、预览和发布能力。
- 两个页面补齐 SEO 描述、菜单图标、Markdown 正文等中文字段和提示。
- 删除 `App.tsx` 中文章列表、文章编辑器和页面管理三块旧 JSX，并清理无用 `InputNumber`、`Switch` import。
- `App.tsx` 从 2214 行下降至 1979 行；`/posts`、`/pages` 直接访问均返回 200。
- Admin build、Go tests、Hugo build 与无污染 preflight 全部通过。
- 下一步：单独拆分 Publishing Center；之后把登录壳层、API client、数据刷新和权限判断迁入 provider/hooks。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 PAGE EXTRACTION 5

- 新建 `PublishingPage.tsx`，迁移发布任务、正式版本和预览构建三部分视图。
- 发布中心保留任务重试/取消、版本切换、release 清理、preview 清理和打开预览能力；表头、状态和操作全部中文化。
- 至此十个后台业务页面均已物理拆分到 `src/pages/*`，旧业务 JSX 不再堆叠于 `App.tsx`。
- 新建 `src/api/client.ts`，抽离运行时 API 地址、Authorization header、JSON Content-Type 和统一非 2xx 错误处理。
- 新增移动端抽屉导航，修复小屏隐藏侧栏后无菜单的问题；抽屉沿用真实路由和菜单选中状态。
- Admin 菜单改为按数据库实时权限过滤；数据刷新按权限选择管理 API 或公共 API，避免低权限账号因无关接口 403 导致整页刷新失败。
- 增加直接 URL 的模块权限判定，无权限页面不挂载业务组件并显示中文提示。
- `App.tsx` 从 1979 行下降至 1718 行；全部十个后台路由直达返回 200。
- Admin build、Go tests、Hugo build 和无污染 preflight 全部通过。
- 下一步：将领域类型迁入 `src/types`，将认证/数据刷新迁入 provider/hooks，将布局迁入 `AdminLayout`，继续把 `App.tsx` 收缩为路由页面编排层。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 INFRA EXTRACTION 1

- 新建 `src/types/admin.ts`，集中定义领域模型、表单值、发布模型、认证响应、API Envelope 和后台路由类型。
- 所有页面组件与 router 已改为从 `types/admin` 导入类型，不再反向依赖 `App.tsx`；发布模型抽取 `PublishTarget`/`PublishLogEntry` 公共结构减少重复。
- 新建 `src/layout/AdminLayout.tsx`，迁移桌面侧栏、移动抽屉、页头、当前角色、退出和刷新控制。
- `App.tsx` 删除领域类型和三层 Layout JSX，当前下降至 1353 行，职责收敛为表单状态、数据请求和业务命令编排。
- API Client、types、layout、pages 四层边界已形成；核心路由直达、Admin build 与无污染 preflight 全部通过。
- 下一步：抽取 `useAdminSession`、`useAdminData` 与领域 command hooks，进一步移除 App 中的认证、refresh 和 CRUD 实现。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 INFRA EXTRACTION 2

- 新建 `useAdminSession.ts`，迁移 Token 初始化/持久化、登录请求、登录 loading、退出和当前用户会话状态。
- 登录 loading 与文章保存 busy 分离，避免不同领域操作互相影响按钮状态。
- 新建 `useAdminData.ts`，迁移 health/ready、文章、页面、分类标签、媒体、评论、发布、审计、用户角色、权限和设置数据状态。
- 数据 Hook 负责按实时权限选择 admin/public API、并行加载、失败回退、设置表单同步和 Token 变化后的自动刷新。
- 显式暴露媒体、评论、设置 setter，保留上传/审核/保存命令的局部状态更新能力。
- `ApiStatus` 移入统一类型层；修复浏览器全局 `Comment` 与领域类型的命名冲突。
- `App.tsx` 从 1353 行下降至 1199 行；Admin build、核心路由和无污染 preflight 通过。
- 下一步：按 posts/pages、publishing、content administration、users/roles 四组抽 command hooks；最终把 App 收缩到表单装配与页面 props 编排。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 COMMAND EXTRACTION 1

- 新建 `useContentAdminCommands.ts`，迁移分类创建/删除、标签创建/删除、媒体上传/删除/清理、媒体地址与 Markdown 插入、评论审核/删除。
- 内容管理命令统一中文成功/警告提示、Token 检查、busy 状态和错误回传。
- 分类和标签创建共享受控内部命令，但保持对页面暴露的强类型领域方法。
- 媒体与评论继续使用数据 Hook 暴露的 setter 做局部更新，不需要每次操作都全量刷新。
- 主容器删除对应旧命令和四组 busy 状态，`App.tsx` 从 1199 行下降至 960 行。
- `/taxonomy`、`/media`、`/comments` 路由直达和无污染 preflight 均通过。
- 下一步：抽取 users/roles commands、publishing commands、posts/pages commands。

## 2026-07-11 - CENTER - ADMIN-ROUTER-P5-001 COMMAND EXTRACTION 2

- 新建 `useIdentityCommands.ts`，迁移用户创建/启停/角色替换、密码重置、自定义角色创建/编辑/权限替换/删除，以及两个 Modal 的领域状态。
- 用户和角色命令提示完成中文化；Hook 接收明确的 `canManageUsers`/`canManageRoles`，不在 UI 内部重新推导权限。
- 新建 `usePublishingCommands.ts`，迁移 release 提升、release/preview 清理、publish job 重试/取消和对应 busy 状态。
- 发布清理与任务动作使用受控内部复用函数，页面仍获得清晰的领域方法。
- 主容器删除身份和发布命令实现，`App.tsx` 从 960 行下降至 699 行。
- `/publishing`、`/users` 路由直达及 Admin/Go/Hugo 无污染 preflight 全部通过。
- 下一步：抽取 posts/pages/settings commands，完成 Admin 主容器收口；随后进入 C 端 Stack Demo 差异审计。

## 2026-07-12 - CENTER - ADMIN-UI-P6-001 ARCO DESIGN MIGRATION

- 目标：解决后台管理系统视觉粗糙、组件风格不统一的问题，引入成熟厂商开源组件库，并保留现有真实路由、RBAC、业务命令和 API 契约。
- 选型：调研 Arco Design、Semi Design、TDesign 和 Ant Design 后，采用字节跳动开源的 `@arco-design/web-react`；原因是后台组件覆盖完整、中文生态成熟、视觉密度适合 B 端，且现有页面迁移成本可控。
- 子 Agent：
  - Euclid 负责组件库选型调研，对比维护状态、React 兼容性、体积和迁移风险。
  - Meitner 负责只读盘点 10 个路由、页面/Hook 依赖和响应式风险。
  - Euler 负责六个业务 Hook 的 Message、FormInstance 和表单校验迁移。
  - Lagrange 负责文章、独立页面和站点设置页面迁移。
  - Anscombe 负责分类标签、媒体和评论页面迁移。
  - Goodall 负责发布、用户权限和审计页面迁移。
- 依赖迁移：
  - 引入 `@arco-design/web-react@2.66.15`。
  - 移除 `antd`、`@ant-design/icons`、`@ant-design/v5-patch-for-react-19`。
  - React/React DOM 固定为 `18.3.1`，避免 Arco 2.66.15 在 React 19 下触发 `element.ref` 兼容错误。
  - `react-router-dom` 升级到 `7.18.1`，消除 npm audit 报告的高危漏洞。
- 布局与设计系统：
  - 建立统一 `PageHeader`、`ContentPanel` 页面结构和后台设计令牌。
  - 重做深色分组侧栏、桌面/移动导航、面包屑、账号区和顶部命令区。
  - 10 个业务路由全部改为 Arco 组件和中文 B 端工作台结构：`/dashboard`、`/posts`、`/pages`、`/taxonomy`、`/media`、`/comments`、`/publishing`、`/users`、`/settings`、`/audit`。
  - 权限动作、系统角色、状态值和确认文案补齐中文映射；发布失败详情使用两行省略和 Tooltip，避免长异常信息破坏表格密度。
- 行为修复：已有文章和独立页面执行发布前会先保存当前编辑内容，避免发布旧版本；登录页在 761-799px 区间的横向溢出已修复。
- 兼容边界：保留现有路由直达、刷新、浏览器前进后退、RBAC 菜单过滤、移动 Drawer、文章行选择、Modal、Popconfirm、退出登录和 API 错误处理语义。
- 验证：
  - Admin `npm run build` 通过。
  - `npm audit` 为 `0 vulnerabilities`。
  - 源码无 Ant Design 包或组件残留引用。
  - 1440x900 与 390x844 下逐一验收全部 10 个路由：console error 0、异常 HTTP 响应 0、页面级横向溢出 0。
  - 登录、退出、移动导航、文章行选择、Modal 和 Popconfirm 核心交互通过。
- 证据：
  - `docs/process/evidence/admin-arco-posts-desktop-1440x900.png`
  - `docs/process/evidence/admin-arco-posts-mobile-390x844.png`
- 文档：更新 `docs/frontend/admin-tech-decision.md`、`docs/frontend/admin-design-system.md` 和 `docs/process/task-board.md`；`ADMIN-UI-P6-001` 已标记 Done，`LOCK-ADMIN-UI-P6` 已释放。
- 运行服务：Admin `http://localhost:5173/`，API `http://localhost:18080`，C 端 `http://localhost:1313/`；子 Agent 临时使用的 5174/5175 服务已关闭。
- 后续：同步历史决策和上下文交接文档，执行仓库级 `scripts/qa/preflight.ps1 -SkipE2E`，再进入 C 端 Stack Demo 差异审计或下一项产品功能。

## 2026-07-12 - CENTER - ADMIN-UX-P7-001 CMS INFORMATION ARCHITECTURE

- 用户反馈：Arco 迁移后的后台仍然凌乱，存在不必要的常驻功能和超长页面，需要参考真实后台重新编排。
- 在线调研：参考 Ghost Admin、WordPress、Strapi、Directus、Payload CMS、Sanity Studio 和 Arco Design Pro 官方资料；结论是内容列表与编辑必须分路由，低频维护/危险操作应降级，长表格必须分页，多工作流应使用 Tabs/Drawer/Modal。
- 只读审计：确认主要问题为文章列表与完整编辑器纵向堆叠、页面管理宽表和编辑器混排、发布中心三张长表、账号/角色全部常驻、分类/标签双表单、多个表格关闭分页，以及 `useAdminData` 每个路由加载全部模块。
- 多 Agent 分工：
  - Parfit：真实 CMS 官方样例与 12 条实施规则。
  - Lagrange：当前后台信息架构和页面高度审计。
  - Faraday：发布中心 Tabs、任务详情 Drawer 和分页。
  - Lorentz：账号/角色 Tabs、创建与权限 Modal、分页。
  - Goodall：分类/标签 Tabs、创建 Modal、分页。
  - Hilbert：媒体搜索与维护入口、评论状态筛选和分页。
  - Pasteur：独立页面列表/编辑工作区拆分。
- 中心实现：
  - 新增 `/posts/new`、`/posts/:postID/edit`、`/pages/new`、`/pages/:pageID/edit`，支持直达、刷新和浏览器导航。
  - `/posts`、`/pages` 只保留全宽列表、搜索、状态筛选和分页；编辑器改为独立主内容+右侧设置工作区。
  - 桌面编辑工作区固定在当前视口内，正文区和发布设置区独立滚动，不再拉长整页。
  - 侧栏压缩为“工作区/管理”两组，移除工作区卡片和重复面包屑；顶栏命令改为图标按钮。
  - 页面标题、面板间距、表单间距和工具栏密度统一收紧；装饰性 eyebrow 隐藏。
  - 站点设置改为两列紧凑配置，保存/预览/发布移至页头；发布页去除重复标题和计数标签。
  - 审计与发布表格按 6 条分页，其他列表按 8/10 条分页；API 尚未提供 total，界面不冒充数据库总数。
  - `useAdminData` 改为按路由加载当前模块，刷新只刷新当前工作区；模块失败不再清空整个后台状态。
- 验证：
  - 1280x720 下 12 个路由页面级横向溢出 0、纵向溢出 0。
  - 390x844 下 12 个路由页面级横向溢出 0；普通页面单屏，文章/页面编辑约 2 屏，设置约 1.2 屏。
  - 文章列表进入已有文章编辑路由、异步表单加载和返回列表通过。
  - 发布详情 Drawer、分类创建 Modal、媒体维护 Modal、账号创建 Modal 均通过。
  - 按路由网络矩阵无异常 HTTP 响应；每个模块只请求自身 API 与 `/auth/me`。
  - `npm run build`、官方 registry `npm audit`（0 漏洞）、`scripts/qa/preflight.ps1 -SkipE2E` 全部通过。
- 文档：新增 `docs/frontend/admin-ux-information-architecture.md`，记录官方参考、路由规范、页面编排规则、数据加载和后续服务端分页契约。
- 证据：
  - `docs/process/evidence/admin-ux-posts-list-desktop-1280x720.png`
  - `docs/process/evidence/admin-ux-post-editor-desktop-1280x720.png`
  - `docs/process/evidence/admin-ux-post-editor-mobile-390x844.png`
- 状态：`ADMIN-UX-P7-001` Done，`LOCK-ADMIN-UX-P7` 已释放。
- 后续：把客户端分页升级为 API 服务端分页，并将列表筛选、页码和排序写入 URL query；随后进入 C 端 Stack Demo 差异审计。

## 2026-07-12 - CENTER - LIST-PAGINATION-P8-001 SERVER PAGINATION

- 目标：将 P7 的客户端分页升级为真实 API 服务端分页，并让列表状态可通过 URL 分享、刷新和恢复。
- 多 Agent 分工：Linnaeus 负责文章/发布任务/版本，Harvey 负责页面/媒体/评论，Copernicus 负责用户/审计/预览；Zeno 负责文章/页面 Admin，Socrates 负责媒体/评论/审计，Helmholtz 负责发布/账号；Wegener 对 Count、排序白名单、精确媒体查询和权限数据范围做只读审阅，中心窗口统一集成和验收。
- API 契约：新增统一 `page/page_size/q/status/sort` 解析，兼容审计旧 `limit`；默认 20、最大 100；响应返回 `page/page_size/total/total_pages`，非法分页或排序返回 422。
- 后端：九类 Admin 列表完成 Count、筛选、排序白名单、Offset/Limit；文章 taxonomy join 使用 `COUNT(DISTINCT posts.id)`，媒体 `checksum/public_url` 精确查询继续返回旧数组契约。
- 审阅修复：所有列表排序追加唯一 ID；Count 后对越界页直接返回空数组，避免巨大 OFFSET；发布和评论关联目标只选择 `id/title/slug`，不泄露完整正文；补齐契约要求的 `total_pages`。
- Admin：新增 `useListQuery`，将页码、每页数量、搜索、状态、排序同步到 URL；筛选变化回到第一页。各列表使用服务端数据和真实 total，工作台计数来自 `pagination.total`。
- 编辑直达：文章和页面编辑路由按 ID 拉取单条详情，不要求目标位于当前列表分页中；浏览器返回可恢复列表 query。
- 运行验收：九个列表端点 `page_size=1` 均返回分页元数据；文章 total=3、发布任务 total=11、发布版本 total=10、审计 total=176；非法 sort 返回 422；`page=1000000&page_size=100` 返回空数组且保留真实 total。
- 浏览器验收：`/posts?page=2&page_size=1` 显示 1 行、共 3 条、活动页 2；`q=PostgreSQL` 刷新后保留；`status=published` 进入编辑并返回后 query 保留；页面、媒体、评论、发布、账号和审计均无加载错误，1280x720 无页面级横向或纵向溢出。
- 干净浏览器会话 console error 0。API 重启叠加 Vite HMR 时出现过一次 Hooks 热更新警告，关闭旧浏览器上下文后不可复现，判定为开发态 HMR 状态残留。
- 验证：`go test ./...`、Admin `npm run build`、`scripts/qa/preflight.ps1 -SkipE2E` 全部通过；preflight 同时通过 Hugo production build。
- 运行服务：Admin `http://localhost:5173`，API `http://localhost:18080`，C 端 `http://localhost:1313`。
- 状态：`LIST-PAGINATION-P8-001` Done，`LOCK-LIST-PAGINATION-P8` 已释放。
- 后续：登记并执行 C 端 Theme Stack Demo 差异审计；服务端分页的 PostgreSQL handler 集成测试可作为独立 QA 增强任务补充。

## 2026-07-12 - CENTER - SITE-UX-P9-001 C-END EXPERIENCE AUDIT

- 目标：对 Theme Stack C 端进行桌面/移动、导航、阅读、互动、中文、SEO、可访问性和性能审计，并通过站点本地覆盖落地高价值修复。
- 多 Agent：Turing/Banach/Volta 首轮因外部模型流中断未产出；Kierkegaard 完成阅读与互动审计；Pascal 完成信息架构与中文搜索审计；Newton 完成 SEO/a11y/性能产物审计；中心窗口负责浏览器矩阵、实现、重启、集成和验收。
- 阅读：文章详情使用唯一 `h1`，封面中文 alt 和首屏优先加载；增加上一篇/下一篇、分享/复制、相关文章回退和可访问章节锚点；重复正文封面已去除。
- 评论：全量中文化，补审核和邮箱隐私说明；加载失败不影响正文阅读。
- 发现：分类/标签进入主导航，RSS 进入图标区，删除伪 GitHub 主页；pagerSize 3 调整为 10；搜索索引加入中文 taxonomy 名称。
- SEO：启用 robots，补 sitemap；首页 WebSite/SearchAction、文章 BlogPosting、列表 CollectionPage JSON-LD；搜索和 404 noindex，搜索从 sitemap 排除；404 和分页标题中文化。
- a11y：新增 skip link；非首页站点名不占 H1；暗色模式改为原生 button + aria-pressed；移动菜单同步 aria-expanded 并管理焦点；搜索 label/input 关联；空锚点增加 aria-label。
- 性能：移除 Google Fonts 同步依赖，使用系统字体；头像由 649,926 B 降为 13,986 B，favicon 为 12,670 B；展示文章媒体改为同源静态资源。
- 浏览器：1280x720 与 390x844 下 8 个核心路由均 200、横向溢出 0、缺 alt 0、无名称控件 0、空名称链接 0；暗色键盘切换和移动菜单焦点通过。
- 搜索：“系统”返回 2 条，“效率”返回 1 条；文章页上一篇/下一篇、分享、2 条相关文章、中文评论区通过。
- 验证：Hugo production build、`scripts/qa/preflight.ps1 -SkipE2E`、Go tests、Admin production build 全部通过。
- 证据：`site-p9-home-desktop-1280x720.png`、`site-p9-article-mobile-390x844.png`。
- 状态：`SITE-UX-P9-001` Done，`LOCK-SITE-UX-P9` 已释放。
- 上线阻断：`PUBLISH-URL-P10-001` 已登记 Ready；需正式站点/API 域名后实现正式发布 loopback/HTTPS 校验，预览环境豁免。

## 2026-07-13 - CENTER - PUBLISH-URL-P10-001 PRODUCTION PUBLIC URL POLICY

- 正式域名：C 端 `https://zoking.tech/`，公开 API `https://api.zoking.tech`；开发环境继续使用 `http://localhost:1313/` 与 `http://localhost:18080`。
- 多 Agent：Locke 只读梳理正式发布、settings 与 preview 调用链；Carson 只读审计生产 Compose/环境变量传播；Raman 修改互斥的部署配置与 runbook；中心窗口实现发布策略、产物检查、回滚保护、seed 与测试。
- 配置：新增 `SITE_BASE_URL`、`PUBLIC_API_BASE_URL`；生产 Compose 同时传给 API/worker，并派生 Admin runtime API 与 Hugo 评论 API；CORS 包含站点和后台域名。
- seed：首次初始化按环境变量写入 `site.base_url` 与 `comments.api_base`；已有数据库通过 Admin settings/API 更新，避免为改域名单独重跑完整 seed。
- 正式发布：`APP_ENV=production` 时强制站点/API 使用 HTTPS 根路径，拒绝 localhost、loopback、unspecified IP、userinfo、query/fragment，并要求数据库值与部署声明一致；媒体允许干净的同源根相对路径或公开 HTTPS URL。
- 预览：隔离 preview 不调用生产 URL policy，localhost/request-origin 能力保持不变。
- 产物与回滚：Hugo 构建完成后扫描文本产物，拒绝回环地址，要求包含正式站点域名，评论标记存在时要求正式 API 域名；历史 release promote/rollback 前执行同一策略。
- 验证：`go test ./...` 通过；生产 Compose `config --quiet` 通过；Hugo production 实构建 45 pages，通过 `localhost|127.0.0.1|[::1]` 空扫描，并确认两个正式域名进入产物。
- 最终验收：`scripts/qa/preflight.ps1 -SkipE2E` 通过，包括 Go 全量测试、Admin production build 与 Hugo 45 pages production build。
- 状态：`PUBLISH-URL-P10-001` Done，`LOCK-PUBLISH-URL-P10` 已释放。

## 2026-07-13 - CENTER - AUDIT-P11-001 CODE AND RUNTIME SELF-AUDIT

- 范围：生产 URL policy、release/rollback、seed、preview、Compose/runbook、公开评论入口，以及本地 C 端/Admin/API 运行态。
- 多 Agent：Einstein 审计 Go 发布链路；Dirac 审计生产部署、安全边界与运维文档；中心窗口复核、整改、测试、重启服务和浏览器验收。
- 发布校验整改：产物检查从无上下文字符串搜索改为 HTML 结构解析，验证 canonical、OG、评论 API 和可点击 URL；允许技术正文/代码块出现 localhost；历史 release 不再依赖当前评论开关；精确拒绝 userinfo/query/fragment、私网、link-local、非标准 loopback 和错误 API 前缀。
- 配置整改：非 dev/test 环境默认启用生产 URL policy；媒体公开基址拒绝 `/`、点段和编码点段；seed 站点设置改为 insert-only，重跑不覆盖后台配置。
- 部署整改：生产 seed 强制替换默认超级管理员凭据；API/Admin/site 默认仅绑定 `127.0.0.1`；补齐 timeout/retry/root 配置传播、Admin 反代说明和 site active release healthcheck；删除生产运行破坏性 E2E 的错误指引。
- Preview：key 随机部分从 32 bit 提升到完整 UUID 128 bit，静态响应增加 `private, no-store` 和 `no-referrer`。
- 评论：使用 `golang.org/x/time/rate` 增加按可信客户端 IP 的令牌桶限流，默认每分钟 10 次、突发 5 次；可信代理来源可配置且启动时校验。
- 自动验证：`go test ./...`、`go vet ./...`、Admin build、Hugo 45 pages production build、生产 Compose config、`preflight -SkipE2E` 全部通过；官方 npm registry audit 为 0 vulnerabilities。Windows 当前 Go 环境未启用 CGO，因此 `go test -race` 未执行。
- 浏览器：1280x720 与 390x844 下 C 端和 Admin 登录页无横向溢出；Admin console error/warning 为 0；C 端单一 H1、缺失图片 alt 为 0。
- 运行态：Admin `http://localhost:5173`、C 端 `http://localhost:1313`、API `http://localhost:18080` 均返回 200；API 已重建并在原端口重启。
- 状态：`AUDIT-P11-001` Done，`LOCK-AUDIT-P11` 已释放。

## 2026-07-13 - CENTER - QA-P12-001 WHITE-BOX AND BLACK-BOX REGRESSION

- 目标：自行编写测试代码和用例，从实现内部与真实 HTTP 边界审计 API、Admin、C 端、发布和隔离数据链路。
- 多 Agent：Gibbs 完成首轮 Go 白盒测试；Dewey 因外部模型容量失败无产出；安全审计 Agent 补充高风险边界，中心窗口负责复现、最小整改、集成和最终验收。
- 白盒入口：新增 `scripts/qa/whitebox.ps1`，强制 `go test -count=1 -covermode=atomic`，生成 `dist/qa/whitebox-cover.out`，随后执行 `go vet ./...`。
- 黑盒入口：新增 `scripts/qa/http-blackbox.ps1` 与 `docs/qa/http-blackbox.md`；默认只读检查 API health/readiness、公开文章、认证/校验边界、CORS、C 端中文/SEO、Admin SPA/runtime/proxy，共 `15/15` 通过；可选评论限流观察真实 `422 -> 429`，共 `16/16` 通过且无成功评论写入。
- 完整 E2E：使用独立 `zoking_blog_test`、API `18081` 和 `storage/qa/preflight-runtime`，覆盖 taxonomy、媒体、文章、页面、三类 Preview、异步发布、设置发布、评论审核、rollback 和 manifest dry-run/apply 清理；安全整改后的最终 run `3729a24b-4604-47dc-80f9-8e96307d104d` 完成后测试记录、文件、隔离 API 与运行目录全部清理，development 数据未触碰。
- 测试发现并修复：非 dev/test seed 对 `prod/staging` fail-open、空白或超过 bcrypt 72 字节密码；seed 改为事务执行、并发同邮箱插入和固定授权锁顺序；评论限流统一等价 IPv6 key，生产异常配置 fail-closed；Preview root 拒绝与 release/current/media/Hugo 目录父子重叠；Preview 文件拒绝 manifest、穿越和 symlink 越界；生产产物补查 `srcset/formaction/object[data]/meta refresh/inline CSS/CSS url()` 与内部 DNS 名称。
- Seed 集成：新增 `scripts/qa/seed-concurrency.ps1`，只允许专用 `zoking_blog_seed_*_test` 数据库；全新空库迁移后两个 seed 进程并发首跑均成功，结果为 `active_admins=1`、`super_admin_links=1`、`duplicate_role_permissions=0`，临时库和测试二进制自动删除。
- 最终覆盖率：总计 `23.8%`；`ValidateReleasePublicURLs 94.4%`、`ValidateReleaseOutputPublicURLs 86.1%`、HTML URL 校验 `87.0%`、评论限流 `87.9%`、seed 配置门禁 `100%`、Preview 文件解析 `80.0%`。
- 最终验证：受影响包未缓存测试、全量白盒、`go vet`、Admin production build、Hugo 45 pages production build、整改后隔离完整 E2E、`preflight -SkipE2E`、PowerShell 5 文件 parser、seed 危险数据库名称门禁、`git diff --check` 全部通过；API/Admin/C 端最终均返回 200。
- 平台限制：Go 位于 `E:\Editor\go`，`CGO_ENABLED=0`，Windows 本机无法执行 race；`QA-P13-001` 已登记 Backlog，后续在 Linux CI 与隔离 PostgreSQL 补 Preview 终态竞争、forwarded origin 信任和发布调用点不变式。
- 运行态：Admin `http://localhost:5173`，C 端 `http://localhost:1313`，API `http://localhost:18080`，PostgreSQL `localhost:15432`；限流黑测后 API 已再次重启，内存桶已清空。
- 状态：`QA-P12-001` Done，`LOCK-QA-P12` 已释放。

## 2026-07-13 - CENTER - QA-P13-001 CONCURRENCY AND FAILURE INVARIANTS

- 目标：补齐 QA-P12 留下的 Preview 终态竞争、可信代理 origin、发布失败不变式与 Linux race 门禁，并重复执行完整黑盒/E2E 回归。
- 多 Agent：Heisenberg 负责 Preview 生产代码与测试；Singer 负责 Publisher 失败不变式集成测试；Sagan 负责 GitHub Actions race job 和说明；中心窗口负责审查、补 PostgreSQL race service、真实环境复验、运行态重启和日志收口。
- Preview：finish/fail 更新只允许从 `building` 转换，必须恰好影响 1 行；竞争失败返回明确终态转换错误。文章/页面预览将 malformed UUID、缺失记录和数据库错误分别映射为 422/404/500；相对 preview public base 统一规范化。
- 代理边界：仅当 TCP 即时对端命中 `TRUSTED_PROXIES` 的 IP/CIDR 时，Preview origin 才接受首个 `X-Forwarded-Proto/Host`；公网直连伪造头继续使用请求自身 TLS/Host。
- 真实数据库竞争：`TestPreviewFinishFailRacePostgresIntegration` 仅允许 `_test` PostgreSQL，每轮独立 schema，12 轮同时执行 finish/fail并验证恰好一胜一负、最终状态与全部终态字段一致。中心窗口 `-count=3` 共 36 轮通过，残留 schema 为 0。
- 发布不变式：生产 URL 产物校验失败必须写入 `PUBLIC_URL_OUTPUT_INVALID`，不得创建 release、切换 active/current 或保留未登记输出；历史 release 在 staging/swap 前重新验证。中心窗口在真实 PostgreSQL 连续 3 轮通过，旧 active/current 保持不变。
- Linux race：`golang:1.25-bookworm` 加入 PostgreSQL 容器网络，连接 `zoking_blog_test` 执行 `internal/httpapi`、`internal/publisher`、`internal/maintenance` 的 `go test -race -count=1`，三个 package 全部通过。CI race job 已增加 PostgreSQL service，使上述集成测试不会 skip；`actionlint` 通过。
- 白盒/黑盒：`scripts/qa/whitebox.ps1` 总覆盖率 `25.0%`，`go vet` 通过；重建并重启 `dist/dev/zoking-api.exe` 后，默认 HTTP 黑盒 `21/21`、含限流 `22/22`，限流测试后 API 再次重启清空内存桶。
- Seed/E2E：双进程首次 seed 结果为 `active_admins=1`、`super_admin_links=1`、`duplicate_role_permissions=0`；完整 preflight 使用 API `18081`、`zoking_blog_test` 和隔离运行目录，run `49eb1cd1-5218-4e72-98b4-a82d1d341478` 覆盖三类预览、文章/页面/设置发布、评论、rollback 和 manifest 清理并通过，测试 API、数据记录、临时 schema/数据库与运行目录均已清理。
- 运行态：API `http://localhost:18080`、Admin `http://localhost:5173`、C 端 `http://localhost:1313` 均在线；API health=`ok`、ready=`ready`。
- 状态：`QA-P13-001` Done，`LOCK-QA-P13` 已释放。

## 2026-07-13 - CENTER - QA-P14-001 ADMIN PAGINATION POSTGRESQL INTEGRATION

- 目标：为 Admin 服务端分页增加真实 PostgreSQL 回归，验证 taxonomy Count、组合筛选、稳定排序、媒体精确查询兼容、审计 legacy `limit` 和越界页，并修复测试暴露的契约问题。
- 多 Agent：Zeno 负责文章分页集成测试；Mill 负责媒体/审计集成测试；Harvey 只读审阅九类分页 handler；中心窗口建立 `_test` 独立 schema 基座、补验证测试、修复生产代码、运行 race/preflight 和运行态验收。
- 测试基座：新增 `openHTTPAPIPostgresTestSchema`，只接受 PostgreSQL 且数据库名以 `_test` 结尾；每个用例创建 UUID 后缀 schema，独立连接池，清理顺序为关闭业务连接、删除 schema、关闭管理连接。
- 文章：真实验证 `category_id + tag_id + status + q + keyword` 组合过滤、taxonomy 关联预载顺序、total/total_pages、相同 title 下按 ID 跨页稳定排序，以及 `page=1000000` 空数组但保留真实元数据。
- 媒体/审计：checksum/public_url 精确查询保持非分页数组 envelope 并返回正确 usage count；普通媒体 q+status 分页稳定；审计 `limit` 映射 page size，可与 q/result/resource_type/actor_id 组合，且同 created_at 按 ID 稳定排序。
- 测试发现并修复：文章 `category_id/tag_id`、评论 `post_id` 无效 UUID 从 PostgreSQL 500 改为 handler 422；Audit 接受 UUID URN 后改为绑定解析值，避免原字符串触发 500；媒体精确参数按“是否出现”判断，空 checksum/public_url 返回 422，不再退回普通分页；public_url 保留解码后的完整字符串精确比较，不再裁剪后误命中。
- PostgreSQL 验证：`internal/httpapi` 集成测试连续 3 轮通过；全量 `go test ./...` 与 `go vet ./...` 通过；所有 `pagination_*` 临时 schema 清理后为 0。
- Linux race：`golang:1.25-bookworm` 连接 `zoking_blog_test` 执行 `internal/httpapi`、`internal/publisher`、`internal/maintenance`，全部通过且无 data race。
- 白盒/完整回归：数据库集成用例纳入覆盖率后总覆盖率为 `30.4%`；完整 preflight/E2E run `64aa9101-f17e-4462-8820-66335c872020` 覆盖构建、三类预览、文章/页面/设置发布、评论、rollback 和 manifest 清理并通过，隔离 API 与运行目录已删除。
- 运行态：重建并重启 `dist/dev/zoking-api.exe`；管理员 HTTP 验证确认三类无效 UUID、空 checksum/public_url 返回 422，Audit UUID URN 返回 200，带空白 public_url 不命中；默认 HTTP 黑盒 `21/21` 通过。
- 状态：`QA-P14-001` Done，`LOCK-QA-P14` 已释放。

## 2026-07-13 - CENTER - SEC-P15-001 CONTENT OBJECT ACCESS CONTROL

- 目标：在现有数据库实时 RBAC 之上补齐文章、页面及关联发布记录的对象级范围，保证 author 仅管理本人内容、viewer 全局只读、editor/admin/super_admin 保持全局管理。
- 多 Agent：Arendt、Newton、Pauli 分别审阅角色矩阵、文章链路、页面/发布链路；Hilbert 审阅发布动作与编辑深链；Carson 横向审阅 taxonomy、媒体、评论、设置；Sartre 审计数据与 runtime 残留；Aquinas 给出文档收口点；Euler 扩展 Playwright；PostgreSQL worker 子任务由中心接管。
- 权限模型：新增 `content:read_all`、`content:manage_all`，系统角色 seed 精确对账；未登记后台路由默认拒绝。新建文章/页面强制当前认证 owner，请求体不能伪造；列表、详情、更新、删除、预览和发布均在首次 SQL 查询限制 owner，他人对象统一 404。
- 发布范围：job/release/preview 按关联内容 owner 过滤；retry/cancel 在调用发布服务前验证范围；owner-scoped 用户不能 promote 整站 release；无发布权限不能通过 create/update 绕过 published 状态边界。
- Admin：身份资料加载前默认拒绝；文章/页面编辑深链按 create/update 权限阻止挂载和提前请求；发布中心按能力隐藏 retry/cancel/promote/cleanup；taxonomy、媒体、评论、设置按独立权限移除写入口，设置表单只读。媒体对 author 保留上传和插入 Markdown，对 viewer 仅保留复制地址。
- PostgreSQL：`TestContentAccessPostgresOwnerIsolation` 连续 3 轮通过，覆盖 owner/global 列表和 total、强制 owner、他人 CRUD/preview/publish 无副作用、published 绕过、本人与他人 job retry/cancel、发布记录范围和 owner promote 403；每轮独立 schema 自动清理。
- 浏览器：Playwright 最终 run `ff53c1bde3e6` 通过 super_admin、author desktop/mobile、viewer 四场景；覆盖文章/页面深链、发布中心、分类/标签双 Tab、媒体只读复制与分级写动作、评论、设置，1280x720 与 390x844 无页面横向溢出，console/page error 为 0；fixture 清理为 `0|0|0`。
- 回归：Admin production build、Node/PowerShell parser、`go test ./cmd/seed ./internal/httpapi -count=1`、默认 HTTP 黑盒 `21/21` 通过；白盒覆盖率 `35.8%`、`go vet`、三个目标包 Linux PostgreSQL race 和完整 preflight/E2E run `e4d12c45-efbc-4e82-b66f-8ed85ad7776c` 已通过。
- 环境记录：首次 UI 重跑因 Docker 引擎未运行在 fixture 前中止，第二次因 Admin `5173` 未监听在 fixture 前中止；使用 `docker desktop start`、启动 `zoking-blog-postgres` 和独立 Vite 进程后成功，两个失败尝试均未创建测试数据。
- 残留审计：development P15 用户/文章/页面、NULL owner、额外 schema 均为 0；test 临时 schema/文章/页面为 0，保留历史 settings-only baseline jobs/releases/active=`5|5|1`；`18081` 关闭，preflight runtime 不存在，E2E runs 为空。
- 状态：`SEC-P15-001` Done，`LOCK-SEC-P15` 已释放。

## 2026-07-13 - CENTER - SITE-AUDIT-P16-001 READER EXPERIENCE AND FEATURE RESEARCH

- 目标：继续审计 C 端与全栈扩展边界，直接调研真实个人站点的特色能力，并落地一批高价值、低风险且可自动验收的阅读体验改进。
- 多 Agent：Volta 审阅 API/数据库/Admin 扩展能力和安全矩阵；Pasteur-P16 对 23 个 Sitemap URL、25 个站内链接、桌面/移动、axe、SEO 和性能做只读审计；中心窗口负责在线取证、筛选、实现、生产验证和文档收口。
- 在线来源：直接访问 Simon Willison、Maggie Appleton、Derek Sivers `/now`、uses.tech、Josh Comeau、Nicky Case、Gwern Design 和 W3C Webmention；调研矩阵写入 `docs/references/personal-site-feature-research.md`。
- 阅读体验：新增固定阅读进度和本地续读；状态按文章路径保存，500ms 节流，5%-98% 生效，完成删除，30 天过期，续读滚动尊重 reduced motion，全程不上传 API 或创建设备标识。
- 搜索：标题命中权重提升为正文的 5 倍；索引 HTTP/JSON 校验、中文失败态和原地重试完成；结果图片补 alt，标题改为 H1/H2/H3，结果数量使用 live region；无输入预加载移除。
- a11y/中文：移动菜单支持 Escape、外部 pointer 关闭和焦点归还；深色 secondary/tertiary token 达到 AA；左右 aside 具有唯一名称；`/post/`、`/page/` section 中文化；评论网络错误本地化并播报。
- 生产：Nginx 未知路由从首页 200 回退改为真实 404 + 本地中文 `404.html/noindex`，并对 CSS/JS/文本启用 gzip。
- 黑盒：`scripts/qa/site-reader-ui.mjs` 7/7 通过，覆盖续读/过期、搜索结构/断网恢复、深色对比度、中文 section/landmark、390x844 菜单和评论断网；评论 POST 在浏览器侧 abort，未写数据库。
- 生产容器：一次性 `nginx:1.27-alpine` 验证随机路径 `404`、`noindex=true`、CSS `Content-Encoding: gzip`，容器结束后自动停止。
- 回归：Hugo production build 45 pages；`git diff --check` 无新增空白错误；仓库 `preflight.ps1 -SkipE2E` 通过 Go tests、Admin production build 和 Hugo build。API/DB 未改，因此未重复完整写入型 E2E。
- 证据：`site-p16-reading-progress-desktop.png`、`site-p16-mobile-home-390x844.png`；QA 记录为 `docs/qa/site-audit-p16.md`。
- 后续建议：发布前质量检查、结构化系列、受约束评论线程、本地收藏、定时发布依次评审；Webmention、统计/点赞、Newsletter 和读者账号因 SSRF、隐私、反滥用或身份复杂度暂缓。
- 状态：`SITE-AUDIT-P16-001` Done，`LOCK-SITE-AUDIT-P16` 已释放。

## 2026-07-13 - CENTER - CONTENT-QUALITY-P17-001 PUBLISH QUALITY GATE

- 目标：实现可检查未保存草稿的内容质量报告，并把同一规则升级为文章、页面、全站发布、Worker 和 Retry 的服务端强制门禁。
- 多 Agent：Ohm 审阅 API/发布链路；Goodall 审阅 Admin UX；Godel 审阅 QA/安全绕过；Copernicus 在互斥文件范围实现 Admin Drawer 与交互；中心窗口负责规则核心、事务、纵深门禁、测试、浏览器验收、运行态和文档。
- 质量核心：新增 `internal/contentquality`，使用 Goldmark AST 与 HTML DOM；硬错误覆盖必填、公开可见性、危险 URL 与危险 HTML，警告覆盖摘要/SEO/长度/H1/alt/封面/taxonomy/菜单图标；代码围栏示例不误报，报告包含 score、hash 和 policy version。
- API：新增文章/页面新内容与已有对象四个 quality-check endpoint；新内容按 create 权限，已有对象按 update + owner scope；请求体可覆盖未保存字段。
- 发布事务：文章/页面使用 `FOR UPDATE`，在同一事务内完成 active job 检查、质量检查、状态更新、媒体引用和 job 创建；失败返回带报告的 `422 CONTENT_QUALITY_BLOCKED`，数据库无副作用。
- 绕过防护：普通 create/PATCH 即使具备 publish 权限也不能从草稿进入 published，返回 `PUBLISH_ENDPOINT_REQUIRED`；active job 期间 PATCH 返回 `CONTENT_PUBLISH_IN_PROGRESS`。
- 纵深防御：Worker 构建前复检，Retry 入队前复检，settings site publish 和 site worker 检查全部已发布内容；撤稿保持可用。文章 preview 改为加载 CoverMedia。
- Admin：文章/页面编辑器新增“发布检查”；流程为表单校验、检查、保存、publish；380px 右侧 Drawer，移动端全宽、内部滚动；表单修改使旧报告及在途结果失效；最终 publish 422 可从 ApiError details 恢复 Drawer。
- PostgreSQL：真实 `_test` schema 验证阻断零副作用、高权限绕过、双 publish 仅一个 active job、active job 禁止 PATCH、页面/设置门禁、Worker 失败和 Retry 不变；无 token 401、viewer 403。`internal/httpapi` 与 `internal/publisher` 全量集成通过。
- 浏览器：API 拦截模式不写 development/production 数据；1280x720 与 390x844 验证 Drawer、报告失效、危险项、quality->save->publish 顺序和无横向溢出；证据为 `content-quality-p17-desktop-1280x720.png`、`content-quality-p17-mobile-390x844.png`。
- 构建回归：`go test ./... -count=1`、`go vet ./...`、真实 PostgreSQL package tests、Admin `npm run build`、Playwright、仓库 `preflight.ps1 -SkipE2E` 全部通过；Hugo production build 保持 45 pages。
- 运行态：API 已重建为 `dist/dev/zoking-api-p17.exe` 并在 `18080` 重启；`readyz=200`，新质量路由无 token 返回 401；Admin `5173`、C 端 `1313` 保持在线。
- 文档：新增 `docs/backend/content-quality-publish-gate.md`、`docs/frontend/content-quality-panel.md`、`docs/qa/content-quality-p17.md`。
- 状态：`CONTENT-QUALITY-P17-001` Done，`LOCK-CONTENT-QUALITY-P17` 已释放。

## 2026-07-13 - CENTER - SERIES-P18-001 STRUCTURED SERIES AND IN-SERIES NAVIGATION

- 目标：为单篇文章增加独立系列归属与正整数显式排序，并贯通 PostgreSQL、Gin/GORM、发布门禁、Admin 内容组织和 Hugo Stack 风格系列导航。
- 多 Agent：Socrates-P18 负责 API/DB；Cicero-P18 负责 Admin；Kant-P18 负责 Hugo C 端；中心窗口负责共享发布链路、质量门禁、测试隔离审计、开发库迁移、运行态联调和文档收口。
- 数据库/API：新增 `series` 模型和 migration；系列 slug、`series_id + series_order`、成对字段、正序号和删除引用均由数据库约束与稳定 API 错误共同保护；Admin/Public 查询使用 grouped count，避免 N+1；软删除并发通过行锁收口。
- 发布/质量：Hugo front matter 输出结构化 `series.slug/name/order`；发布器、Worker、Retry、恢复与全站质量检查统一预载系列及封面，并拒绝不完整关系、ID 不一致、非法 slug 和非正序号；质量策略升级为 `2026-07-13.2`。
- Admin/C 端：内容组织新增紧凑系列 Tab 与 CRUD Modal，文章编辑器支持系列单选和序号；停用系列可回显但不可新选。文章页新增系列目录、当前位置及系列上一篇/下一篇，保留普通导航、分享和阅读进度，390px 无溢出。
- 自动验证：Go 全量测试、`go vet`、真实 PostgreSQL 契约、Admin production build、Hugo 45 pages、P18 Playwright 和仓库 `preflight.ps1 -SkipE2E` 全部通过；`git diff --check` 无错误。
- 隔离审计：修复 series migration 测试的 goose 版本表隔离；最终 `series_contract_*` schema 为 `0`，`series-test@example.com` 为 `0`。开发库只执行 schema migration，不写测试 fixture。
- 运行态：开发库已迁移至 `20260713000100`；API 构建为 `dist/dev/zoking-api-p18.exe` 并监听 `18080`，`readyz=200`、Admin 系列无 Token 401、Public 系列 200；真实管理员登录及 Admin `/taxonomy` 系列空状态联调通过。
- 证据：P18 桌面/移动 C 端和 Admin 截图位于 `docs/process/evidence/series-p18-*`，其中真实运行态截图为 `series-p18-admin-live-1280x720.png`。
- 状态：`SERIES-P18-001` Done，Socrates-P18/Cicero-P18/Kant-P18 Done，`LOCK-SERIES-P18` 已释放。

## 2026-07-13 - CENTER - SITE-GITHUB-P19-001 DISCOVERY IA AND GITHUB PROJECTS

- 目标：解决 C 端“归档、分类、标签”三个一级入口的重复感，并增加真实 `zo-king` GitHub 仓库展示能力。
- 多 Agent：Curie-P19 负责信息架构实现；Hopper-P19 负责项目页；Nielsen-P19 只读审阅导航、可靠性、隐私、a11y 和 QA；中心负责真实账号确认、安全整改、自动测试、视觉验收和日志收口。
- 信息架构：分类和标签数据模型、taxonomy 路由与文章元数据链接全部保留，只从一级菜单移除；归档页统一提供分类、标签和按年份文章；首页右栏从搜索/归档/分类/标签缩减为搜索和标签云。
- GitHub：新增 `/projects/` 和真实 profile 图标，front matter 配置 `zo-king`、最多 6 条；浏览器按推送时间读取 Public REST API，过滤 fork/archived，展示描述、语言、Star、Fork 和推送日期。
- 安全/可靠性：Hugo 构建不访问 GitHub；8 秒超时、无自动重试、空/403/429/网络失败回退；`credentials=omit`、`no-referrer`、URL host 白名单、`textContent` 渲染、5 分钟 sessionStorage 白名单缓存，无 PAT 和长期标识。
- QA：Mock Playwright 覆盖导航、taxonomy 深链、归档、过滤/排序/上限、XSS 文本、缓存字段、空态、403/手动重试、1280/390/320；真实 GitHub smoke 读取 6 个仓库且零运行时错误。
- 回归：Hugo 46 pages、仓库 preflight、Go tests、Admin build、`git diff --check` 均通过；首页、归档、项目、分类和标签路由均为 200。
- 证据：`docs/process/evidence/github-p19-*`；设计与 QA 分别见 `docs/frontend/github-projects-p19.md`、`docs/qa/github-projects-p19.md`。
- 状态：`SITE-GITHUB-P19-001` Done，Curie-P19/Hopper-P19/Nielsen-P19 Done，`LOCK-SITE-GITHUB-P19` 已释放。

## 2026-07-13 - CENTER - SITE-TIMELINE-P20-001 SEARCH DE-EMPHASIS AND ARCHIVE TIMELINE

- 目标：解决 3 篇文章阶段搜索入口重复且偏重的问题，并参考成熟个人博客，为归档增加符合 Theme Stack 的低干扰时间表达。
- 多 Agent：Feynman-P20 调研 Anthony Fu、Simon Willison、Maggie Appleton、Derek Sivers 和 Gwern；Shannon-P20 审阅搜索、归档模板和排序边界；Norman-P20 审阅响应式、a11y 和 Playwright 矩阵；中心窗口负责取舍、实现、视觉检查、回归和文档。
- 信息架构：搜索从一级导航和首页右栏撤下；`/search/`、JSON 索引、404 搜索和实现代码保留。分类/标签继续集中在归档页，不恢复为一级入口。
- 时间线：归档按发布日期显式倒序并按年份分组，显示年份数量、月日节点、标题、摘要和可选小图；语义为 H1/H2/H3 与 `ol/li/article/time`，无卡片嵌套、月份空层、滚动监听或装饰动画。
- 响应式：桌面使用独立年份/日期轨道；390/320px 收为左侧细线单列，缩略图 56/48px，长中文标题自然换行，触控区不低于 44px，reduced motion 禁用图片过渡。
- 浏览器：P20 Playwright 已通过搜索入口、兼容路由、倒序、语义、图片、深色、1280/390/320 无溢出与零运行时错误；P19 GitHub/归档回归同步通过；截图位于 `docs/process/evidence/archive-p20-*`。
- 文档：调研、设计和 QA 分别写入 `docs/references/archive-patterns-p20.md`、`docs/frontend/archive-timeline-p20.md`、`docs/qa/archive-timeline-p20.md`。
- 回归：Hugo Extended 0.160.1 minify build 46 pages；仓库 `preflight.ps1 -SkipE2E` 通过 Go tests、Admin production build 和 Hugo build；`git diff --check` 无空白错误。
- 状态：`SITE-TIMELINE-P20-001` Done，Feynman-P20/Shannon-P20/Norman-P20 Done，`LOCK-SITE-TIMELINE-P20` 已释放。

## 2026-07-13 - CENTER - SITE-ABOUT-PLUGIN-P21-001 ABOUT/LINKS CLEANUP AND BLOG PLUGIN RESEARCH

- 目标：让关于页只呈现关于内容，移除友链页下方的相关文章，并将友链改为带站点头像、简介、域名和外链指示的可扫描布局；同时调研有价值的博客插件和特色能力。
- 多 Agent：Ada-P21 调研 Pagefind、Giscus、Mermaid、KaTeX、Plausible、Umami、Webmention、ActivityPub、PhotoSwipe；Turing-P21 调研 Simon Willison、Derek Sivers、Julia Evans、Drew DeVault、Gwern、Maggie Appleton、overreacted 等真实博客；中心窗口负责页面实现、头像失败回退、视觉验收和文档收口。
- 页面边界：`single.html` 只对 `post` section 渲染相关文章，关于、友链等独立页面不再出现推荐文章。
- 友链布局：新增本地 links partial 和卡片样式；两列桌面、单列移动；头像尝试站点自身 favicon，再回退 DuckDuckGo favicon 服务，最终显示首字母；`no-referrer`、无 Token、失败不阻塞正文。
- 测试：P21 Playwright 覆盖关于/友链无相关文章、头像成功/失败回退、外链安全属性和 1280/390 无溢出；Hugo 46 pages build 通过；截图位于 `docs/process/evidence/about-p21-*` 与 `links-p21-*`。
- 调研结论：Mermaid、KaTeX、PhotoSwipe、RSS 已有，不重复引入；Pagefind 等文章规模增长后评估；统计 Plausible/Umami 二选一；Giscus 不与现有 Go 评论并用；Webmention 和 ActivityPub 后置独立设计。
- 文档：插件调研、前端设计、QA 分别写入 `docs/references/blog-plugin-research-p21.md`、`docs/frontend/about-links-p21.md`、`docs/qa/about-links-p21.md`。
- 回归：P21 与 P20 Playwright、Hugo 46 pages、`preflight.ps1 -SkipE2E` 的 Go tests/Admin production build/Hugo build 全部通过；`git diff --check` 无空白错误。
- 状态：`SITE-ABOUT-PLUGIN-P21-001` Done，Ada-P21/Turing-P21 Done，`LOCK-SITE-ABOUT-PLUGIN-P21` 已释放。

## 2026-07-13 - CENTER - ACHIEVEMENTS-P22-001 FULLSTACK ACHIEVEMENTS AND TECHNICAL CONTENT

- 成果领域：PostgreSQL migration、Gin/GORM 模型与 Admin CRUD、状态机、RBAC、媒体约束、审计和发布入口已贯通。
- 发布器：只导出 published 成果到 `apps/site/data/achievements.json`，release/preview manifest 记录 hash 和路径，原子替换与稳定排序已测试。
- C 端：`/archives/` 改为 2024 至今的成果年份轨道，不再展示文章；无数据、无 JavaScript、reduced motion、1280/390/320 均可用。
- 内容：新增 Go、Gin、GORM、C++ 数据结构共 8 篇工程向中文文章；侧栏头像统一为 GitHub `zo-king` 头像。
- Admin：新增成果列表、编辑、新建、媒体选择、发布、归档、删除与站点发布流程，修复 `/achievements/new` 被误当 ID 的路由缺陷。
- 验证：Go 全量测试、真实 PostgreSQL 成果契约、Admin build、Hugo 54 pages、P22 Admin/C 端 Playwright、preflight 均通过；黑盒测试数据已清理。
- 状态：`ACHIEVEMENTS-P22-001` Done，全部 P22 Agent 已关闭，P22 文件锁已释放。

## 2026-07-13 - CENTER - SITE-LAYOUT-P23-001 MOK STYLE DENSITY TUNING AND PIXIV COVERS

- 参考：在线检查 `mok.moe` 桌面首页，提取细顶栏、三栏资料布局、连续文章流、克制圆角与低阴影，不复制品牌、文案和站点组件。
- 布局：新增顶部站点栏；左栏增加文章/分类/字数统计；首页从分散大卡片收为统一文章流，桌面 210px 横向封面，移动端单列封面。
- 精简：右栏继续只保留标签，不引入参考站最新评论或热门文章，避免页面重新变长变乱。
- 视觉回退：曾接入的三张装饰封面因不符合用户审美已完整移除，技术文章恢复无封面信息流，模板和文档不保留失效署名逻辑。
- 响应式：修复 sticky 顶部偏移误作用于移动端导致菜单与文章重叠的问题；1280/390/320 均无页面级横向溢出，图片加载正常。
- 验证：Hugo Extended 0.160.1 production/minify 54 pages、`git diff --check`、暗色切换和浏览器尺寸矩阵通过。
- 状态：`SITE-LAYOUT-P23-001` Done，`LOCK-SITE-LAYOUT-P23` 已释放。

## 2026-07-14 - CENTER - SEC-RELIABILITY-P24-001 SECURITY AND RECOVERY AUDIT

- 多 Agent：Mill 审计媒体崩溃一致性与引用锁；Curie 审计发布状态机、Commit 不确定与 stale recovery；Hilbert 审计 auth/session、Preview 生产拓扑、runbook 和 QA；中心窗口负责修复、故障回归和归档。
- 认证：Admin 改为 HttpOnly Cookie + CSRF + Origin/CORS 隔离；新增 `/auth/session` 轮换 CSRF；登录/恢复均不返回 JWT；logout 网络与 Cookie 契约已在 26 项黑盒中验证。
- 媒体：生产私有暂存/隔离目录固定在媒体卷内；post/page/series/achievement/release 引用创建统一持有 `FOR KEY SHARE`；删除方持有 `FOR UPDATE`；孤立候选使用 `NOT EXISTS` 排除已引用媒体，避免 dry-run 误报和批次饥饿。
- 发布：`publishing` migration、失败独立恢复 context、Cancel/Retry 行锁、promotion/cleanup advisory lock、RowsAffected 校验和 stale publishing 恢复已落地；失败写入不覆盖 `requested/canceled/published`；Worker 周期恢复 stale job，并按数据库唯一 active release 对账 `current`。对账失败不会阻塞新发布自愈。
- 配置与运维：生产 Preview 必须使用 `https://preview.zoking.tech/preview-files`，且独立于站点、API、Admin Origin；runbook 修正专用 seed service、Cookie/CSRF 运维调用、DNS/TLS/代理和精确 migration 回退命令。
- QA 修复：黑盒增加 Cookie 属性、CSRF 轮换、错误 Origin、旧 Token、logout 清 Cookie和退出后 401；E2E 按公开评论 DTO 验证不泄漏 moderation status，再由 Admin 列表验证 pending。
- 验收：真实 PostgreSQL `go test ./... -count=1`、`go vet ./...`、Admin production build、npm 官方源 0 漏洞、Hugo production/minify 54 pages、development/production Compose config、migration Down/Up、26/26 HTTP 黑盒、SkipE2E preflight 和完整隔离 E2E 全部通过。
- 清理：临时 `18081/18082` 均停止，`storage/qa/preflight-runtime` 已删除；原 `18080/5173/1313` 未停止。测试库旧分页 fixture `public-*` 仅在 `zoking_blog_test` 被归档，不涉及开发或生产数据。
- 残余：跨数据库/文件系统的 COMMIT ACK 丢失和进程终止恢复需要新增持久化 media operation/manifest；已登记 `MEDIA-RECOVERY-P25-001`，要求 Toxiproxy 和进程级故障注入，不以普通单测替代。
- 状态：`SEC-RELIABILITY-P24-001` Done，三位只读 Agent 完成，`LOCK-SEC-RELIABILITY-P24` 已释放。

## 2026-07-14 - CENTER - SITE-HOME-SPLASH-P26-001 HOME DENSITY, ABOUT ARTICLE AND SPLASH

- 目标：首页文章列表从每页 3 篇调整为 6 篇；新增置顶的中文个人介绍文章并使用现有本地建筑封面；增加与当前中性、工程博客视觉一致的简约开屏动画。
- 首页与内容：`apps/site/config/_default/hugo.toml` 将 `pagination.pagerSize` 调整为 `6`；新增 `apps/site/content/post/about-zoking/index.md`，使用 `2026-07-14` 日期和稳定 slug，包含个人方向、博客内容边界和写作目的，首篇封面为 `/img/showcase/architecture.jpg`。
- 开屏：`baseof.html` 接入独立的指纹化 `splash.scss` 与 `splash.ts`；首次会话播放约 1.8 秒，支持“跳过”和 `Esc`，使用 `sessionStorage` 避免同一会话重复播放；`prefers-reduced-motion: reduce` 直接移除，脚本异常或禁用时不阻塞正文。
- 视觉与可访问性：开屏仅使用站名、ZK 方印、细线和短文案，不引入外部图片、渐变球、粒子或复杂动效；跳过按钮不放在 `aria-hidden` 容器内，保留键盘焦点轮廓；字号固定并在窄屏使用明确媒体查询。
- 构建：Hugo Extended 0.160.1 development build 通过，生成 61 pages；生产构建继续受既有 `comments.public.apiBase` 必填配置门禁约束，未伪造生产密钥或改变评论配置。
- 黑盒：Playwright 运行态验证首页 6 篇、首篇个人介绍、封面加载、首次开屏自动消失、刷新不重复、减少动效跳过、390px 无横向溢出和零控制台错误。
- 状态：`SITE-HOME-SPLASH-P26-001` Done，未改动 P24 API/数据库/权限代码，文件锁已释放。

## 2026-07-14 - CENTER - SITE-VISUAL-WIDGET-P27-001 REUSABLE SPLASH AND PIXIV SIDEBAR

- 调研：在线检查 CSS Loaders、Awwwards loader 集合和 Tobias Ahlin 的 SpinKit；SpinKit 为 MIT 许可、纯 CSS、无运行时依赖。开屏选用其 `circle-fade` 思路，不引入完整库。
- 开屏：移除原 ZK 方印、细线和说明文案，仅保留站名和 28px 八点淡入环；显示时间从约 1.8 秒缩短到 1.4 秒，退出改为 420ms 自下而上的遮罩收起；跳过、Escape、sessionStorage、无脚本不阻塞和 reduced-motion 行为保持不变。
- Pixiv：参考 `https://mok.moe/p/pixiv-daily-ranking` 的官方推荐方式，新增 Hugo `pixiv-ranking` widget；首页右栏嵌入 `https://pixiv.mokeyjay.com/?limit=10`，380px 高、自适应宽度、lazy loading、no-referrer 和受限 sandbox，附原文章与 GitHub 组件来源。
- 边界：第三方 iframe 不向主页面注入脚本，不读取本站 Cookie；移动端继续隐藏整个右栏；榜单内容、可用性和每日更新由第三方服务负责。
- 验证：Hugo Extended 0.160.1 production/minify 61 pages；iframe HTTP 200、10 张图片且首屏图片真实加载；1280 浅色/暗色、1024 临界宽度和 390 移动端均无横向溢出、零运行时错误。
- 证据：`docs/process/evidence/site-splash-spinkit-p27-1280x800.png`、`site-pixiv-widget-p27-1280x900.png`、`site-pixiv-widget-p27-dark-1280x900.png`。
- 状态：`SITE-VISUAL-WIDGET-P27-001` Done，文件锁已释放。

## 2026-07-14 - CENTER - SITE-SIDEBAR-WIDGETS-P28-001 SIDEBAR DISCOVERY AND DAILY CONTENT

- 目标：把首页右栏收敛为稳定的三段内容发现入口，依次展示“大家抢着看”、Pixiv 每日排行和一言，同时保留移动端隐藏右栏的既有信息架构。
- 大家抢着看：新增本地 Hugo widget，按配置中的稳定 slug 白名单解析 `about-zoking`、`city-walk`、`thoughtful-workspace`；严格保持配置顺序，缺失文章静默跳过，展示本地封面、标题、日期和站内链接，不引入浏览量伪数据或外部请求。
- Pixiv：沿用 P27 的 `https://pixiv.mokeyjay.com/?limit=10` iframe，继续使用 lazy loading、`no-referrer` 和受限 sandbox；标题和组件来源链接保留，第三方内容、每日更新与可用性不作为本站构建真相。
- 一言：新增 `https://v1.hitokoto.cn/` JSON 请求，进入可视区后加载；请求不携带凭据或 referrer，5 秒超时，响应做非空与长度校验，使用 `textContent` 渲染；30 分钟 `sessionStorage` 缓存降低重复请求，刷新按钮可主动换句，API/CORS/网络失败时保留“保持好奇，也保持耐心。/本地寄语”。
- 验收修复：首次构建发现一言刷新按钮引用的 `refresh.svg` 尚未进入 Hugo 资源目录并导致首页 500；实现窗口补齐本地图标后，首页恢复 200，最终 production/minify 内存构建通过 61 pages。
- 测试：Pixiv 与一言端点均返回 HTTP 200；首页静态契约确认 3 篇热门、Pixiv 安全属性与指纹化一言脚本；Edge headless 在 1280/1024 显示三组件且顺序正确，在 390 隐藏右栏，全部无横向溢出、console/page error。请求拦截验证一言首次成功、手动刷新、刷新后缓存复用和断网本地降级。
- 上传准备：P28 组件级检查为 Ready，第三方通知已补齐。生产构建必须注入 `HUGO_COMMENTS_API_BASE=https://api.zoking.tech`，不得上传开发态 `apps/site/public` 作为发布真相，应由生产构建或既有发布流水线生成。
- 记录：专项矩阵与上传清单见 `docs/qa/site-sidebar-widgets-p28.md`；第三方边界见 `docs/references/third-party-frontend-notices.md`。
- 状态：`SITE-SIDEBAR-WIDGETS-P28-001` Done，文件锁已释放。

## 2026-07-14 - CENTER - GITHUB UPLOAD AND REPOSITORY HYGIENE

- 仓库整理：删除误生成的 Go coverage 文件；补充覆盖率忽略规则和 scoped `.gitattributes`；修复根 `public` 忽略规则误伤 `apps/admin/public`，将 Admin favicon 与 runtime config 占位文件纳入版本控制。
- 上游元数据：保留 Theme Stack 源码、GPL 许可证和 `theme.toml`；移除原作者 Funding 与 demo Wrangler 配置；Issue 模板改为本项目全栈问题入口，并保留纯上游主题缺陷的官方链接。
- 安全审计：未发现真实私钥、GitHub/云 Token 或生产密钥；暂存区不含 `.env`、`node_modules`、构建目录、日志、coverage、工具二进制和异常大文件。固定开发/CI 密码仅用于本地测试，生产仍必须替换 `__REQUIRED_*` 占位值。
- 回归：`go test ./...`、`go vet ./...`、白盒覆盖率任务（总语句覆盖率 30.5%）、Admin production build、npm 官方源 audit（0 vulnerabilities）、Hugo production/minify 61 pages、`preflight.ps1 -SkipE2E` 与 `git diff --check` 通过。
- 首次上传：曾将完整项目上传到旧仓库分支 `codex/fullstack-blog` 并通过 Draft PR 的 Linux race 与完整 PostgreSQL preflight；该远程仓库随后由所有者删除，本记录仅保留为历史验证依据。

## 2026-07-14 - CENTER - CLEAN REPOSITORY REINITIALIZATION

- 新仓库：目标改为 `https://github.com/zo-king/zoking_blog`，创建时为空仓库，默认分支尚未初始化。
- 模块路径：API、Site 与工程文档中的 Go module/import 路径统一为 `github.com/zo-king/zoking_blog/...`，避免新仓库地址与包标识长期不一致。
- 历史策略：以当前已验证工作树生成单一干净根提交并推送为 `main`，不携带已删除仓库的合并历史；Theme Stack 远程继续保留为 `upstream`。
- 发布基线：新仓库 `main` 是后续提交、CI、部署和新窗口接力的唯一代码基线。

## 2026-07-15 - CENTER - SITE-DISCOVERY-P29-001 BLOG DISCOVERY AND COMMAND PALETTE

- 调研：在线检查 `lvyovo-wiki.tech/bloggers` 的 Next.js/Motion/Canvas 实现，并对照 Anthony Fu、Innei、Josh Comeau、Rauno 与 Cassie Evans；采用局部反馈与互动内容，拒绝全站持续 Canvas、自定义鼠标、整卡 1.05 放大和玻璃卡堆叠。
- 友链：从 3 项扩充为 9 项常读站点，增加显式分类、名称/简介/域名搜索、结果计数和仅在当前可见结果内随机拜访；头像继续使用站点 favicon、DuckDuckGo、首字母三级回退，改为 lazy/async 加载。
- 快捷面板：顶部增加图标入口，全站支持 `Ctrl/Cmd+K`；面板由现有主导航、搜索页、随机文章和主题切换动态组成，支持输入过滤、上下键、Enter、Escape、焦点恢复和焦点循环，无 JavaScript 时不影响原导航。
- 动效：使用浏览器 Web Animations API 为首页文章和友链卡片提供一次性 `opacity + translateY(8px)`、220ms 入场；等待开屏退出后启动，`prefers-reduced-motion` 下完全跳过，不引入第三方动画运行时。
- QA：新增 `scripts/qa/site-discovery-p29.mjs`，覆盖 9 项渲染、搜索、分类、随机范围、快捷导航、主题命令、1280/390、横向溢出与 reduced-motion；同步更新 P21 友链数量/元数据回归。专项 P29 与 P21 均通过，零 page/console error。
- 证据：`docs/process/evidence/site-discovery-links-p29-1280x900.png`、`site-command-palette-p29-1280x900.png`、`site-command-palette-p29-390x844.png`。
- 状态：`SITE-DISCOVERY-P29-001` Done，`LOCK-SITE-DISCOVERY-P29` 已释放。

## 2026-07-15 - CENTER - SITE-GITHUB-SNAPSHOT-P30-001 SCHEDULED PROJECT SNAPSHOT

- 决策：移除项目页访客侧 GitHub REST 请求、5 分钟 sessionStorage 缓存、加载骨架、403 提示和重试按钮；项目列表改为 Hugo 构建期读取已提交 JSON，访问页面不再消耗 GitHub API 配额。
- 同步：新增 `scripts/sync-github-projects.mjs`，对白名单字段执行验证、Fork/archived 过滤、推送时间排序和 GitHub URL 主机校验，通过临时文件原子替换 `apps/site/data/github_projects.json`；失败在写入前退出，保留最后成功快照。
- 调度：新增 `Sync GitHub projects` Actions 工作流，每月 1 日、16 日 UTC 02:00（北京时间 10:00）运行，并支持手动触发；使用仓库 `GITHUB_TOKEN`，同步后执行 Hugo production build，由 `github-actions[bot]` 只提交快照文件。
- 当前数据：真实同步得到 4 个 `zo-king` 公开非 Fork/归档仓库，页面明确显示快照日期；桌面双列、390/320 单列布局保持稳定。
- QA：同步 fixture 黑盒覆盖鉴权、过滤、排序、字段白名单、不安全 URL 和失败不覆盖；Playwright 覆盖静态卡片顺序、零 GitHub API 请求、旧脚本缺失、1280/390/320 与零运行时错误。
- 证据：`github-p30-projects-static-1280x800.png`、`github-p30-projects-static-390x844.png`；状态 Done，文件锁已释放。

## 2026-07-15 - CENTER - SITE-NAVIGATION-NOW-P31-001 BLOG MOTION AND NOW PAGE

- 调研：并行检查 20 余个中日与西方个人技术博客，重点复核页面过渡、活动目录、阅读设置、脚注、链接预览和 `/now`；实际浏览器检查确认 `shud.in`、Maggie Appleton、Paco Coursey 等使用 View Transition 或等价连续性处理。
- 去重：本站已有阅读进度、30 天断点续读、TOC scrollspy、标题锚点、代码复制、分享、上一篇/下一篇和命令面板，因此撤回未提交的返回顶部方案，不增加第二套阅读侧轨或文章快捷键。
- 动效：`custom.scss` 启用原生跨文档 View Transition，只使用 140ms 淡出、180ms 淡入与 4px 位移；不拦截链接、不引入运行时依赖，reduced-motion 下动画为 none。
- 内容：新增 `/now/` 近况页，包含“正在开发、正在学习、最近关注”，进入主菜单并由现有快捷面板自动索引；它与关于、成果时间线分工明确，后续直接替换已结束事项。
- QA：新增 P31 Playwright，覆盖真实 `pageswap/pagereveal.viewTransition`、CSS 动画名、reduced-motion、主菜单、命令面板、浏览器前进后退、canonical、唯一 H1、1280/390、横向溢出和零运行时错误。
- 状态：`SITE-NAVIGATION-NOW-P31-001` Done，文件锁已释放；下一候选为轻量阅读设置，不包含重型字体/SVG 资源。

## 2026-07-15 - CENTER - SITE-READING-SETTINGS-P32-001 LOCAL READING PREFERENCES

- 入口：只在文章页顶部操作区增加 `Aa` 图标，不在首页、近况、项目或友链展示；原生 dialog 使用字号分段控件、宽松行距和正文链接下划线复选框，避免新增常驻侧栏。
- 样式：复用 `--article-font-size`、`--article-line-height` 和 `.article-content` 范围，桌面较大/特大为 1.85/2rem，移动端为 1.75/1.9rem；不放大导航、评论或代码工具。
- 持久化：单一本地键 `zoking:reading-preferences:v1` 只允许三档字号和两个布尔字段；文章 head 在绘制前恢复数据属性，避免刷新闪动；全部默认时删除存储键。
- 缺陷修复：刷新恢复后，宽泛 `[data-reading-spacing]` 选择器会先匹配 `<html>` 属性而非 checkbox；测试暴露后收紧为 `input[data-reading-spacing]` 与 `input[data-reading-links]`。
- QA：P32 Playwright 覆盖实际 font-size/line-height、链接 text-decoration、存储白名单、DOMContentLoaded 前恢复、刷新控件同步、重置、Escape、焦点返回、非文章隔离、1280/390/320 和零运行时错误。
- 回归：P16 读者测试的评论 GET 原先依赖本地 API；本轮改为空列表 fixture，仍保留 POST 主动断网测试，最终阅读进度、搜索、暗色对比、移动菜单和评论错误本地化共 7 项通过。
- 状态：`SITE-READING-SETTINGS-P32-001` Done，文件锁已释放。
