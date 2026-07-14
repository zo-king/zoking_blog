# 架构决策清单

本文件记录全栈博客系统必须沉淀的 ADR。详细 ADR 放在 `docs/architecture/adr/`。

## ADR 列表

| ID | 标题 | 状态 | 摘要 |
|---|---|---|---|
| ADR-0001 | 采用后台动态管理 + Hugo 静态发布架构 | Proposed | B 端和 API 动态管理内容，C 端由 Hugo 构建静态站点 |
| ADR-0002 | C 端保留 Hugo Theme Stack | Proposed | 不重写为 SPA/SSR，复用 Stack 阅读体验和 SEO 能力 |
| ADR-0003 | PostgreSQL 作为后台主数据库 | Proposed | 存储内容编辑态、权限、配置、发布、审计、统计 |
| ADR-0004 | Go Gin + GORM 构建 API | Proposed | 使用 Go 生态实现模块化单体后端 |
| ADR-0005 | PostgreSQL 为编辑源，Hugo content 为发布快照 | Proposed | 后台写 DB，发布时生成 Hugo 文件树 |
| ADR-0006 | 发布快照不可变 | Proposed | 线上版本通过 active release 指针切换 |
| ADR-0007 | 发布构建由 worker 执行 | Proposed | API 请求不直接同步执行 Hugo build |
| ADR-0008 | Monorepo 组织 apps/site、apps/api、apps/admin | Proposed | 统一项目工程管理 |
| ADR-0009 | 后端采用模块化单体 | Proposed | 不过早拆微服务 |
| ADR-0010 | REST + OpenAPI 作为 API 风格 | Proposed | 便于 Admin 前端、测试和文档生成 |
| ADR-0011 | SQL-first migration | Proposed | 生产不依赖 GORM AutoMigrate |
| ADR-0012 | RBAC 权限模型 | Proposed | 支持作者、编辑、管理员、超级管理员 |
| ADR-0013 | 媒体存储策略 | Proposed | 开发本地，生产可迁移对象存储 |
| ADR-0014 | 预览机制 | Proposed | 支持后台文章预览，尽量使用 C 端主题样式 |
| ADR-0015 | C 端动态能力边界 | Proposed | 评论、统计、点赞通过 Public API 异步增强 |

## ADR 模板

```markdown
# ADR-XXXX: 标题

## 状态

Proposed | Accepted | Deprecated | Superseded

## 背景

需要解决什么问题？有哪些约束？

## 方案

最终选择什么？

## 备选方案

| 方案 | 优点 | 缺点 | 复杂度 | 适用条件 |
|---|---|---|---|---|

## 决策理由

为什么选择该方案？

## 接受的权衡

放弃了什么？为什么可以接受？

## 后果

- 正向：
- 负向：
- 缓解：

## 重新评估触发条件

什么时候需要重新讨论？
```
