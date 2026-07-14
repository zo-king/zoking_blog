# 工程工作分解 WBS

本文件把全栈博客系统拆成可分派、可跟踪、可验收的工程任务。中心窗口后续应按任务编号派发给子 agent，并把执行结果追加到 `docs/process/worklog.md`。

## 1. 工作分解原则

- 每个任务必须有编号。
- 每个任务必须有明确产物。
- 每个任务必须有验收标准。
- 每个任务必须指定建议 agent 角色。
- 子 agent 不得无编号执行大改动。
- 阶段完成后必须更新工作日志和接力文档。

## 2. Phase 0：工程基线

| ID | 任务 | 建议 Agent | 产物 | 依赖 | 验收 |
|---|---|---|---|---|---|
| ARCH-P0-001 | 确认核心 ADR | Architecture | `docs/architecture/adr/*` 状态更新为 Accepted | 用户确认 | ADR-0001 至 ADR-0008 有状态和理由 |
| DOC-P0-001 | 更新项目 README | Docs | 根 `README.md` | 架构确认 | README 说明系统目标、本地启动、文档入口 |
| OPS-P0-001 | 创建 monorepo 目录骨架 | Ops | `apps/`, `db/`, `infra/`, `scripts/` | ARCH-P0-001 | `git status` 只包含预期目录 |
| OPS-P0-002 | 编写 `.env.example` | Ops | `.env.example` | OPS-P0-001 | 不含真实密钥，变量与文档一致 |
| API-P0-001 | 初始化 Gin API | Backend | `apps/api` Go module、health check | OPS-P0-001 | `go test ./...` 通过，`/healthz` 可访问 |
| DB-P0-001 | 初始化迁移工具 | Backend/DB | migration 目录和首个扩展迁移 | API-P0-001 | 空库可执行迁移 |
| HUGO-P0-001 | 整理 Hugo site 位置 | Hugo | `apps/site` | OPS-P0-001 | Hugo 可本地 build |
| ADMIN-P0-001 | 确认并初始化后台栈 | Admin | `apps/admin` | 用户确认后台技术栈 | dev server 可启动 |
| CI-P0-001 | 基础 CI 草案 | Ops | CI workflow | API/Admin/Hugo 初始化 | PR 可跑基础检查 |

## 3. Phase 1：核心数据与 API

| ID | 任务 | 建议 Agent | 产物 | 依赖 | 验收 |
|---|---|---|---|---|---|
| DB-P1-001 | 用户/角色/权限 schema | Backend/DB | users、roles、permissions、关联表 migration | DB-P0-001 | migration 通过，基础 seed 可执行 |
| API-P1-001 | Auth 模块 | Backend | login、refresh、logout API | DB-P1-001 | 登录成功/失败/刷新/退出测试通过 |
| API-P1-002 | RBAC 中间件 | Backend | auth + permission middleware | API-P1-001 | 未授权请求被拒绝 |
| DB-P1-002 | 内容 schema | Backend/DB | posts、pages、categories、tags、关联表 | DB-P0-001 | slug 唯一索引和软删除索引存在 |
| API-P1-003 | 文章 CRUD API | Backend | admin posts API | DB-P1-002 | 可创建、编辑、删除、查询文章 |
| API-P1-004 | 分类标签 API | Backend | categories/tags API | DB-P1-002 | 可绑定到文章 |
| DB-P1-003 | 媒体 schema | Backend/DB | media_assets、media_usages | DB-P0-001 | 引用关系可记录 |
| API-P1-005 | 媒体上传 API | Backend | upload/list/delete API | DB-P1-003 | 类型、大小校验有效 |
| DB-P1-004 | 评论 schema | Backend/DB | comments migration | DB-P0-001 | 评论状态索引存在 |
| API-P1-006 | 评论提交与审核 API | Backend | public submit、admin moderate | DB-P1-004 | 待审核、通过、拒绝流程可用 |
| DB-P1-005 | 审计 schema | Backend/DB | audit_logs migration | DB-P0-001 | 高危操作能写审计 |

## 4. Phase 2：B 端后台

| ID | 任务 | 建议 Agent | 产物 | 依赖 | 验收 |
|---|---|---|---|---|---|
| ADMIN-P2-001 | 后台登录页 | Admin | login page | API-P1-001 | 登录成功进入后台 |
| ADMIN-P2-002 | 后台布局和菜单 | Admin | layout、side nav、top bar | ADMIN-P2-001 | 菜单与权限适配 |
| ADMIN-P2-003 | 文章列表 | Admin | post table | API-P1-003 | 搜索、筛选、分页可用 |
| ADMIN-P2-004 | Markdown 编辑器 | Admin | editor page | API-P1-003/API-P1-005 | 可保存草稿、插入媒体 |
| ADMIN-P2-005 | 分类标签管理 | Admin | taxonomy pages | API-P1-004 | CRUD 可用 |
| ADMIN-P2-006 | 媒体库 | Admin | media library | API-P1-005 | 上传、预览、复制链接可用 |
| ADMIN-P2-007 | 评论审核 | Admin | comment moderation page | API-P1-006 | 审核状态可操作 |
| ADMIN-P2-008 | 站点设置基础版 | Admin | settings page | site settings API | 保存后可被发布使用 |
| ADMIN-P2-009 | 数据看板基础版 | Admin | dashboard | stats API or SQL | 基础指标展示 |

## 5. Phase 3：Hugo 集成发布

| ID | 任务 | 建议 Agent | 产物 | 依赖 | 验收 |
|---|---|---|---|---|---|
| PUB-P3-001 | 发布数据视图 | Backend/Hugo | publish view/query | API-P1-003 | 可读取发布所需完整数据 |
| PUB-P3-002 | Front matter 生成器 | Hugo | DB -> frontmatter mapping | PUB-P3-001 | 字段与 Stack 约定匹配 |
| PUB-P3-003 | Markdown 快照生成 | Hugo | snapshot writer | PUB-P3-002 | 生成 `content/post/.../index.md` |
| PUB-P3-004 | Hugo build worker | Backend/Ops | worker | PUB-P3-003 | 可触发 `hugo --minify` |
| PUB-P3-005 | 发布任务状态机 | Backend | publish_jobs 状态流转 | PUB-P3-004 | 成功/失败状态正确 |
| PUB-P3-006 | 发布日志与 manifest | Backend/Ops | release manifest | PUB-P3-004 | 可追踪内容 hash 和操作者 |
| PUB-P3-007 | 回滚机制 | Backend/Ops | active release switch | PUB-P3-006 | 可切回上一个 release |
| HUGO-P3-001 | C 端动态增强接入 | Hugo/Frontend | comments/views fetch | API-P1-006/stats | API 失败不影响阅读 |

## 6. Phase 4：生产化

| ID | 任务 | 建议 Agent | 产物 | 依赖 | 验收 |
|---|---|---|---|---|---|
| OPS-P4-001 | Dockerfile | Ops | API/Admin/Hugo Dockerfile | Phase 0-3 | 镜像可构建 |
| OPS-P4-002 | Docker Compose | Ops | compose.dev/prod | OPS-P4-001 | 本地一键启动核心服务 |
| CI-P4-001 | CI/CD 流水线 | Ops | workflow | OPS-P4-001 | PR/main 检查通过 |
| OPS-P4-003 | 备份恢复脚本 | Ops | backup/restore scripts | DB ready | 恢复演练通过 |
| OPS-P4-004 | 日志与监控 | Ops | logging/metrics docs/config | API ready | health、5xx、DB 指标可观测 |
| SEC-P4-001 | 安全加固 | Security/Ops | security checklist | all | CORS、rate limit、secret、upload 校验 |
| QA-P4-001 | 端到端冒烟 | QA | smoke checklist | all | 写作-发布-C端查看闭环通过 |

## 7. Phase 5：增强能力

| ID | 任务 | 建议 Agent | 产物 | 依赖 | 验收 |
|---|---|---|---|---|---|
| SEARCH-P5-001 | 全文搜索增强 | Backend/Hugo | pg_trgm/Meilisearch 方案 | 内容稳定 | 搜索质量优于本地 JSON |
| PUB-P5-001 | 发布 verifier 稳定化 | Backend/QA | sitemap/RSS/home/article/taxonomy verifier | 发布链路稳定 | E2E 不再对 sitemap 只 warning |
| MEDIA-P5-001 | 媒体引用追踪与删除保护 | Backend/Admin/QA | post/release media_usages、usage_count、删除 409 | 媒体库稳定 | 被文章或 release 引用的媒体不能删除 |
| SETTINGS-P5-001 | 站点设置与页面管理 | Backend/Admin/Hugo | settings/pages API 与 Admin | 内容域稳定 | 设置和页面可被发布使用 |
| CI-P5-001 | 部署前检查流水线 | Ops/QA | Go/Admin/Hugo/E2E 检查脚本或 workflow | smoke 稳定 | 一条命令可跑完上线前检查 |
| MEDIA-P5-002 | 图片处理与保留策略增强 | Backend/Ops | resize/webp pipeline、孤立媒体清理、保留期 | MEDIA-P5-001 | 缩略图、压缩、清理策略可用 |
| PREVIEW-P5-001 | 草稿预览 | Backend/Hugo/Admin | preview token 与预览构建 | 发布链路稳定 | 未发布内容可安全预览 |
| SEC-P5-001 | 权限审计增强 | Security/Backend/Admin | RBAC 权限点、审计日志、危险操作确认 | 核心后台稳定 | 高危操作有权限和审计 |
| ANALYTICS-P5-001 | 数据分析面板 | Backend/Admin | PV/UV/trends | stats ready | 趋势图可用 |
| WORKFLOW-P5-001 | 审批流增强 | Backend/Admin | review workflow | RBAC ready | 驳回、原因、通知可用 |
| I18N-P5-001 | 多语言内容 | Hugo/Admin | language model | 用户确认 | 多语言 URL 和切换可用 |

## 8. 中心窗口执行规则

每轮实施前，中心窗口必须：

1. 从本 WBS 选择任务。
2. 给子 agent 明确允许编辑和禁止编辑范围。
3. 要求子 agent 汇报验证结果。
4. 合并结果后更新工作日志。
5. 若上下文过大，更新接力文档后开新窗口。

## 9. 任务执行记录字段

中心窗口实际派发 WBS 任务时，必须同步更新 `docs/process/task-board.md`，并补充以下字段：

| 字段 | 说明 |
|---|---|
| 状态 | 使用 `Backlog / Ready / Assigned / In Progress / Review / Blocked / Done / Superseded` |
| Owner | 中心窗口负责人或当前承接窗口 |
| Agent | 实际执行 agent |
| 文件锁 | 本任务允许写入的文件或目录 |
| 开始时间 | 派发时间 |
| 完成时间 | 验收通过时间 |
| 验证命令 | 必须执行或说明无法执行 |
| 验收结论 | 通过 / 不通过 / 有条件通过 |

任何任务进入 `In Progress` 前，必须登记 owner、允许编辑范围、禁止编辑范围和验证方式。
