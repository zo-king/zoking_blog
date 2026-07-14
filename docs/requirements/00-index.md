# 需求总览

## 项目目标

基于 Hugo Theme Stack、Go Gin、GORM 和 PostgreSQL 构建一个成熟完整、可长期演进的全栈博客系统。C 端保留 Hugo Theme Stack 的页面设计、阅读体验、SEO 和静态站点性能；B 端提供内容管理、发布控制、媒体管理、评论审核、用户权限、站点设置、数据看板和审计能力。

## 当前阶段

- 阶段：全栈博客系统 Phase 5 增强实施
- 负责人：当前对话窗口作为中心调度
- 最近更新：2026-07-11 09:35 +08:00
- 当前阻塞：完整目标尚未完成；用户/角色管理、最后一个超级管理员保护、author 对象级隔离、预览过期清理和生产对象存储/CDN adapter 仍待补齐
- 当前边界：已采用 Hybrid CMS、基于 React 18.3.1 + Vite + Arco Design React 2.66.15 的 Admin、PostgreSQL、本地媒体与 Docker Compose 起步；Admin 路由固定为 react-router-dom 7.18.1，npm audit 为 0 漏洞；核心写作、页面管理、站点设置、发布、隔离预览、评论、媒体引用保护、清理、E2E 冒烟和部署前 preflight 已落地

## 需求来源

| ID | 来源 | 类型 | 摘要 | 状态 |
|---|---|---|---|---|
| SRC-001 | 用户明确要求 | 显性需求 | 使用 Hugo Theme Stack 构建个人博客项目 | 已确认 |
| SRC-002 | 用户明确要求 | 显性需求 | 使用多个子 agent 干活，中心窗口负责汇报和总控 | 已确认 |
| SRC-003 | 用户明确要求 | 显性需求 | 产出工作日志，上下文过大时开新窗口读日志继续 | 已确认 |
| SRC-004 | 官方 Stack 文档 | 外部资料 | Stack 支持响应式图片、懒加载、深色模式、本地搜索、PhotoSwipe、归档、TOC 等博客能力 | 已采纳 |
| SRC-005 | Hugo 官方文档 | 外部资料 | Hugo 支持 front matter、page bundles、taxonomies、RSS、sitemap 等静态博客基础能力 | 已采纳 |
| SRC-006 | Google Search Central | 外部资料 | SEO 需要帮助搜索引擎理解内容、使用清晰标题、描述和站点结构 | 已采纳 |
| SRC-007 | web.dev / W3C | 外部资料 | 性能和无障碍应进入非功能需求 | 已采纳 |
| SRC-008 | 本地主题源码分析 | 代码事实 | 当前主题 v4.0.3 已内置大量博客功能，多数需求可通过配置和内容约定完成 | 已采纳 |
| SRC-009 | 用户明确要求 | 显性需求 | 落地成熟完整的全栈博客系统，B 端后台控制，C 端阅读浏览，保留 Hugo 页面设计效果 | 已确认 |
| SRC-010 | 架构子 agent | 派生需求 | 采用后台动态管理 + Hugo 静态发布，PostgreSQL 为编辑源，Hugo content 为发布快照 | 已采纳 |
| SRC-011 | 后端数据库子 agent | 派生需求 | 规划 Gin + GORM + PostgreSQL 模块、迁移、RBAC、审计、评论、媒体、发布任务 | 已采纳 |
| SRC-012 | 前后台需求子 agent | 派生需求 | 规划 C 端、B 端、后台菜单、文章/页面/媒体/评论/发布/权限/审计工作流 | 已采纳 |
| SRC-013 | 运维计划子 agent | 派生需求 | 规划 Docker Compose、CI/CD、备份恢复、监控、安全、roadmap、DoD、接力机制 | 已采纳 |

## 用户角色

| 角色 | 目标 | 核心需求 |
|---|---|---|
| 博客作者 | 长期写作、发布和维护文章 | 后台编辑、草稿、预览、分类标签、图片管理、发布流程 |
| 普通访客 | 阅读、检索、订阅内容 | 首页、文章页、搜索、归档、分类标签、RSS、移动端体验 |
| 回访读者 | 跟踪更新和专题内容 | RSS、归档、标签、相关文章、评论或反馈入口 |
| 编辑/管理员 | 管理内容、用户、权限和发布 | 审核、发布、回滚、媒体、评论、站点配置、审计 |
| 维护者 / Agent | 可持续迭代项目 | 文档、工作日志、需求边界、验收标准、接力规则 |

## 核心场景

| ID | 场景 | 触发 | 期望结果 | 优先级 |
|---|---|---|---|---|
| SCN-001 | 发布新文章 | 作者新增 Markdown page bundle | 本地预览正常，构建通过，线上可访问 | P0 |
| SCN-002 | 阅读文章 | 访客打开文章页 | 正文、目录、代码块、图片、移动端布局稳定 | P0 |
| SCN-003 | 查找旧内容 | 访客使用搜索、归档、分类或标签 | 能快速找到相关内容 | P0 |
| SCN-004 | 订阅更新 | 读者使用 RSS | 可在订阅器中收到更新 | P0 |
| SCN-005 | 修改站点信息 | 作者调整头像、简介、菜单、社交链接 | 配置集中，变更可验证 | P0 |
| SCN-006 | 评论互动 | 读者评论或作者回复 | 评论系统可用、隐私策略清楚 | P1 |
| SCN-007 | 部署上线 | main 分支变更或手动触发 | 静态站点成功构建并发布 | P1 |
| SCN-008 | 新窗口接手 | 当前窗口上下文过大 | 新窗口读日志即可继续，不重做需求分析 | P0 |
| SCN-009 | 后台发布内容 | 管理员在 B 端发布文章 | 生成 Hugo 快照并构建 C 端静态页面 | P0 |
| SCN-010 | 评论审核 | 读者提交评论 | 后台可审核，C 端只展示通过评论 | P0 |
| SCN-011 | 发布回滚 | 新版本构建失败或线上异常 | 可回滚到上一次成功 release | P1 |

## 文档索引

- [常见博客功能调研](01-blog-feature-research.md)
- [Stack 能力映射](02-stack-capability-map.md)
- [MVP 范围与验收](03-mvp-scope.md)
- [全栈博客系统需求](04-fullstack-blog-system.md)
- [C 端与 B 端范围规划](05-admin-and-reader-scope.md)
- [系统架构总览](../architecture/00-system-overview.md)
- [Goal 目标计划](../plan/goal-plan.md)
- [子 agent 协作规则](../process/agent-collaboration.md)
- [Goal 执行与多 Agent 调控规范](../process/goal-execution.md)
- [上下文接力规则](../process/context-handoff.md)
- [工作日志](../process/worklog.md)

## 当前开放问题

| ID | 问题 | 影响 | 状态 |
|---|---|---|---|
| Q-001 | 是否接受 Hybrid CMS：PostgreSQL 为编辑源，Hugo content 为发布快照？ | 影响全局架构 | 已采用 |
| Q-002 | B 端后台采用何种前端技术栈与组件体系？ | 影响 `apps/admin` 初始化 | 已采用 React 18.3.1 + Vite + Arco Design React 2.66.15，react-router-dom 7.18.1 |
| Q-003 | 生产媒体存储使用本地、S3/R2/MinIO 还是其他？ | 影响媒体模块和部署 | 开发本地已采用，生产对象存储/CDN 待增强 |
| Q-004 | 生产部署目标是单机 Docker、VPS、云平台还是分层部署？ | 影响 CI/CD 和运维 | Docker Compose 起步已采用 |
| Q-005 | 是否第一阶段就做完整评论审核，还是只建模型和接口骨架？ | 影响 MVP 范围 | 匿名评论 + 后台审核已实现 |
| Q-006 | 是否需要注册读者和访客登录？ | 影响认证复杂度 | 建议后置 |

## 参考来源

- [Stack 中文 Guide](https://stack.cai.im/zh/guide/)
- [Stack Widgets 文档](https://stack.cai.im/zh/config/widgets/)
- [Stack 评论系统文档](https://stack.cai.im/zh/config/comments/)
- [Stack Cookies 文档](https://stack.cai.im/zh/config/cookies/)
- [Hugo Front matter](https://gohugo.io/content-management/front-matter/)
- [Hugo Page bundles](https://gohugo.io/content-management/page-bundles/)
- [Hugo Taxonomies](https://gohugo.io/content-management/taxonomies/)
- [Hugo RSS templates](https://gohugo.io/templates/rss/)
- [Hugo Sitemap templates](https://gohugo.io/templates/sitemap/)
- [Google SEO Starter Guide](https://developers.google.com/search/docs/fundamentals/seo-starter-guide)
- [web.dev Web Vitals](https://web.dev/articles/vitals)
- [W3C WAI Accessibility Principles](https://www.w3.org/WAI/fundamentals/accessibility-principles/)
