# Zoking Blog Docs

这是 Zoking Blog 工程文档入口。后续多 agent 协作、需求分析、实现计划、工作日志和跨窗口接力都应以这里的文档为准。

## 当前阶段

- 阶段：全栈博客系统已进入 Phase 5 增强实施
- 中心窗口职责：总控、调度子 agent、整合结论、维护文档和工作日志
- 当前边界：C 端中文 Stack 风格站点、文章章节复制/代码工具栏/阅读设置、Gin/GORM/PostgreSQL API、React Admin、发布 worker、release/current、评论、媒体保护与清理、页面/设置、隔离预览、操作审计、数据库实时 RBAC 和部署前 preflight 已落地；下一项高优先级工程工作是媒体故障恢复与生产对象存储 adapter。

## 文档索引

- [前端工程指导文档](frontend-engineering-guide.md)
- [需求总览](requirements/00-index.md)
- [常见博客功能调研](requirements/01-blog-feature-research.md)
- [Stack 能力映射](requirements/02-stack-capability-map.md)
- [MVP 范围与验收](requirements/03-mvp-scope.md)
- [全栈博客系统需求](requirements/04-fullstack-blog-system.md)
- [C 端与 B 端范围规划](requirements/05-admin-and-reader-scope.md)
- [系统架构总览](architecture/00-system-overview.md)
- [架构决策清单](architecture/01-architecture-decisions.md)
- [Hugo 发布流水线设计](architecture/publishing-pipeline.md)
- [Front Matter 与快照映射](architecture/frontmatter-and-snapshot-mapping.md)
- [API 契约设计](backend/00-api-contract.md)
- [认证与 RBAC 设计](backend/01-auth-rbac-design.md)
- [后端工程规划](backend/backend-plan.md)
- [数据模型设计](database/00-data-model.md)
- [数据库迁移与 Seed 策略](database/01-migration-and-seed-strategy.md)
- [数据库工程规划](database/database-plan.md)
- [C 端 Stack 集成规范](frontend/site-stack-integration.md)
- [P33 文章轻量工具](frontend/article-utilities-p33.md)
- [B 端后台技术决策](frontend/admin-tech-decision.md)
- [前后台前端规划](frontend/frontend-plan.md)
- [运维与部署规划](operations/README.md)
- [部署 Runbook](operations/deployment-runbook.md)
- [工程实施总控手册](plan/engineering-execution-master-plan.md)
- [Goal 目标计划](plan/goal-plan.md)
- [工程工作分解 WBS](plan/work-breakdown-structure.md)
- [当前任务看板与文件锁](process/task-board.md)
- [子 agent 协作规则](process/agent-collaboration.md)
- [多 Agent 工程执行 SOP](process/multi-agent-execution-sop.md)
- [Goal 执行与多 Agent 调控规范](process/goal-execution.md)
- [上下文接力规则](process/context-handoff.md)
- [工作日志](process/worklog.md)
- [安全基线](security/security-baseline.md)
- [测试与验收策略](qa/test-strategy.md)
- [部署前检查脚本](qa/preflight.md)
- [P33 文章工具验收](qa/site-article-utilities-p33.md)
- [参考资料](references/sources.md)

## 新窗口接手顺序

当当前对话窗口上下文过大，需要开新窗口继续时，新窗口必须先读：

1. [上下文接力规则](process/context-handoff.md)
2. [工作日志](process/worklog.md)
3. [当前任务看板与文件锁](process/task-board.md)
4. [工程实施总控手册](plan/engineering-execution-master-plan.md)
5. [Goal 目标计划](plan/goal-plan.md)
6. [系统架构总览](architecture/00-system-overview.md)
7. [需求总览](requirements/00-index.md)
8. [前端工程指导文档](frontend-engineering-guide.md)
9. 当前 `git status --short --branch`
