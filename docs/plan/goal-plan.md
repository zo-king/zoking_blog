# Goal 目标计划

## 1. 总目标

落地一个成熟完整的项目级全栈博客系统：

- C 端：Hugo + Hugo Theme Stack 静态阅读站点。
- B 端：后台控制台。
- API：Go Gin + GORM。
- 数据库：PostgreSQL。
- 发布：后台内容生成 Hugo 快照并构建静态产物。
- 协作：中心窗口总控，多个子 agent 分工，工作日志可接力。

## 2. 当前阶段

- 当前阶段：Phase 1 内容域最小闭环、Phase 3 发布 job/release/rollback/C端评论/异步 worker 最小闭环、Phase 4 E2E 冒烟脚本已完成，继续推进生产化。
- 当前实现状态：已创建 `apps/site`、`apps/api`、`apps/admin`、`db/migrations`、`infra/docker`；Admin 已可登录、创建草稿、绑定分类/标签、上传媒体、插入 Markdown、审核评论、发布文章；发布会异步创建 publish job，由 worker 生成 release、manifest 与 Hugo release 产物；历史 release 可 promote/rollback；C 端文章页已接入 Public Comments API；端到端冒烟脚本已可重复验证核心闭环。
- 当前文档状态：架构、需求、后端、数据库、前端、运维、计划、协作机制已规划，并已记录 Phase 1 工作日志。

## 3. 阶段路线图

### Phase 0：工程基线

目标：让项目可运行、可构建、可协作。

交付：

- Monorepo 目录。
- 本地开发文档。
- Docker Compose。
- `.env.example`。
- API health check。
- 数据库初始化脚本。
- Hugo 本地预览流程。
- 基础 CI。

验收：

- 从空环境能启动 postgres。
- API health check 可访问。
- Hugo site 可预览。
- 工作日志更新。

### Phase 1：内容模型与 API

目标：完成博客核心数据能力。

交付：

- 用户、角色、权限。
- 文章、页面、分类、标签。
- 媒体表。
- 评论表。
- GORM model。
- SQL migration。
- 文章 CRUD API。
- 分类标签 API。
- 登录认证。
- RBAC 中间件。

验收：

- migration 从空库可执行。
- 管理员可登录。
- 文章可 CRUD。
- 分类标签可绑定。
- 基础 API 测试通过。

### Phase 2：后台管理

目标：完成 B 端基础工作流。

交付：

- 登录页。
- 数据看板基础版。
- 文章列表。
- Markdown 编辑器。
- 草稿保存。
- 分类标签管理。
- 媒体上传。
- 评论审核。
- 站点基础设置。

验收：

- 管理员能创建文章草稿。
- 能上传封面。
- 能选择分类标签。
- 能预览文章。
- 权限不通过时不能访问后台接口。

### Phase 3：Hugo 集成发布

目标：后台内容可发布到 Hugo C 端。

交付：

- 数据库文章转 Markdown。
- Front matter 生成。
- Hugo content 快照生成。
- Hugo 构建任务。
- 发布日志。
- 失败回滚策略。
- C 端静态页面可访问。

验收：

- 后台发布文章后，C 端首页和文章页出现内容。
- Hugo build 失败不会覆盖上一版产物。
- 发布记录可查。

### Phase 4：生产化

目标：可安全上线。

交付：

- 生产 Dockerfile。
- CI/CD 部署流水线。
- HTTPS 配置。
- 备份恢复方案。
- 日志监控。
- 安全检查清单。
- 发布 runbook。

验收：

- staging 可部署。
- production 发布流程明确。
- 备份恢复可演练。
- 基础监控和告警存在。

### Phase 5：增强能力

目标：提升效率和质量。

交付：

- 全文搜索。
- 图片处理。
- SEO 元数据增强。
- 文章预览链接。
- 定时发布。
- 多用户权限增强。
- 操作审计完善。
- 数据分析面板。

## 4. 任务编号体系

格式：

```text
<领域>-<阶段>-<序号>
```

领域：

- `ARCH`：架构。
- `REQ`：需求。
- `API`：Go Gin / GORM / PostgreSQL。
- `ADMIN`：后台管理。
- `HUGO`：C 端 Hugo。
- `PUB`：发布流水线。
- `OPS`：运维部署。
- `CI`：CI/CD。
- `SEC`：安全。
- `DOC`：文档。
- `QA`：测试验收。

示例：

- `API-P1-001`：文章模型与 migration。
- `ADMIN-P2-003`：文章编辑器。
- `HUGO-P3-002`：Front matter 生成。
- `OPS-P4-001`：生产 Docker Compose。

任务状态：

- `Backlog`：已记录，未准备实施。
- `Ready`：依赖满足，可派发。
- `Assigned`：已分配。
- `In Progress`：正在执行。
- `Review`：等待中心窗口验收。
- `Blocked`：因依赖、决策、冲突或验证失败暂停。
- `Done`：验收完成，日志已更新。
- `Superseded`：被新任务替代。

运行时状态维护在 `docs/process/task-board.md`。

## 5. Definition of Done

通用 DoD：

- 需求已实现。
- 本地可运行。
- 相关测试通过。
- 文档已更新。
- 配置项已同步 `.env.example`。
- 无明显安全风险。
- 无无关改动。
- 工作日志已记录。
- 中心窗口已汇总状态。

后端 DoD：

- API 有输入校验。
- 错误码清晰。
- 数据库 migration 可重复执行或有明确版本管理。
- 关键路径有测试。
- 日志不泄露敏感信息。

前端 DoD：

- 主要流程可点击完成。
- 加载、空状态、错误状态明确。
- 表单校验清晰。
- 窄屏不明显错位。

Hugo DoD：

- Markdown 输出正确。
- Front matter 字段完整。
- `hugo --minify` 成功。
- 生成页面 URL 可访问。
- 发布失败不会破坏上一次产物。

运维 DoD：

- 可从空环境启动。
- 有备份恢复说明。
- 有健康检查。
- 有回滚方案。
- 密钥不入库。

## 6. 下一步任务建议

Phase 1 内容域最小闭环、发布 job/release/rollback/C端评论/异步 worker 最小闭环、E2E 冒烟脚本、生产 compose 基线、sitemap verifier、媒体引用保护、站点设置/页面管理和部署前 preflight 已完成。下一步建议按顺序执行：

1. `MEDIA-P5-002`：补媒体 release 保留期、孤立媒体清理、对象存储/CDN 策略。
2. `PREVIEW-P5-001`：补草稿预览和发布前页面预览。
3. `AUDIT-P5-001` / `SEC-P5-001`：补更细 RBAC 权限点、审计日志和后台操作保护。
4. `SEARCH-P5-001`：评估全文搜索增强策略。
5. `OBS-P5-001`：补发布/运行监控指标和告警文档。

## 7. 实施前必读文档

开始写代码前必须先读：

1. `docs/plan/engineering-execution-master-plan.md`
2. `docs/process/task-board.md`
3. `docs/process/multi-agent-execution-sop.md`
4. `docs/backend/00-api-contract.md`
5. `docs/database/00-data-model.md`
6. `docs/architecture/publishing-pipeline.md`
7. `docs/frontend/site-stack-integration.md`
8. `docs/security/security-baseline.md`
9. `docs/qa/test-strategy.md`
