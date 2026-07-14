# 测试与验收策略

本文定义工程阶段的测试层次、验收命令和发布前检查。

## 1. 测试层次

| 层次 | 目标 | 示例 |
|---|---|---|
| Unit | 单个函数或 service 逻辑 | slug、权限判断、front matter 生成 |
| Integration | API + DB | 文章 CRUD、登录刷新、评论审核 |
| Contract | API 契约 | OpenAPI 与 Admin 类型 |
| E2E | 用户流程 | 写作、发布、前台查看 |
| Build | 构建产物 | Go build、Admin build、Hugo build |
| Security | 基线检查 | 未授权访问、上传限制、限流 |

## 2. Phase 0 验收

命令：

```powershell
git status --short --branch
go test ./...
hugo --minify
```

后台初始化后：

```powershell
npm run lint
npm run build
```

通过标准：

- API health check 可访问。
- PostgreSQL 可启动。
- Hugo 可构建。
- Admin dev/build 可执行。
- 当前 Phase 0 实际端口：PostgreSQL `15432`，API `18080`，Admin `5173`，Hugo `1313`。

## 3. Phase 1 验收

必须测试：

- migration 空库执行。
- seed 超级管理员。
- 登录成功/失败。
- refresh/logout。
- RBAC 拒绝无权限接口。
- 文章 CRUD。
- 分类标签绑定。
- 媒体上传限制。
- 评论提交和审核。

## 4. Phase 2 验收

后台流程：

```text
登录 -> 创建文章 -> 上传封面 -> 选择分类标签 -> 保存草稿 -> 提交审核 -> 发布请求
```

页面检查：

- loading。
- empty。
- error。
- form validation。
- permission denied。
- destructive confirm。

## 5. Phase 3 验收

发布闭环：

```text
后台发布 -> 生成快照 -> Hugo build -> release -> C端首页可见 -> 文章页可见
```

必须检查：

- Front matter 字段正确。
- Markdown 正文完整。
- 图片可访问。
- 分类标签页更新。
- RSS/Sitemap 更新。
- build 失败不覆盖上一 release。
- 回滚可执行。

## 6. Phase 4 验收

生产化：

- compose 可启动核心服务。
- Nginx 路由正确。
- HTTPS 配置明确。
- 备份恢复演练通过。
- 监控指标存在。
- 安全基线通过。
- CI/CD 可跑。

## 6.1 部署前检查

一键检查入口：

```powershell
pwsh -NoProfile -File .\scripts\qa\preflight.ps1
```

构建-only 快速检查：

```powershell
pwsh -NoProfile -File .\scripts\qa\preflight.ps1 -SkipE2E
```

检查范围：

- `go test ./...`
- Admin `npm run build`
- Hugo production build
- 数据库 migrate/seed
- E2E smoke，包含文章、页面、设置、媒体、评论和 rollback

详见 [部署前检查脚本](preflight.md)。

## 6.2 白盒与 HTTP 黑盒回归

Go 白盒统一入口：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\whitebox.ps1
```

该入口强制 `go test -count=1 -covermode=atomic`，生成 `dist/qa/whitebox-cover.out`，随后执行 `go vet ./...`。当前重点覆盖生产 URL 与构建产物校验、Preview 路径隔离与静态文件边界、可信代理和评论限流、Preview 终态竞争、发布失败不变式、配置传播与 seed fail-closed 门禁。

只读 HTTP 黑盒：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\http-blackbox.ps1
```

可选限流黑盒会发送无效 JSON，观察 `422 -> 429`，不会产生成功评论写入：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\http-blackbox.ps1 -TestRateLimit
```

2026-07-13 QA-P13 基线结果：白盒总覆盖率 `25.0%`，`go vet` 通过；默认 HTTP 黑盒 `21/21`，含限流 `22/22`；Preview finish/fail 在真实 PostgreSQL 上连续 36 轮竞争通过；发布失败不变式连续 3 轮通过；隔离 PostgreSQL 完整 E2E 通过并自动清理测试数据与运行目录。Linux Docker 中连接同一 `_test` 数据库执行三个目标 package 的 `go test -race` 通过，CI 已配置同等 PostgreSQL race gate。

2026-07-13 QA-P14 基线结果：新增 Admin 分页 PostgreSQL 集成测试，覆盖文章 taxonomy、媒体精确查询、审计 legacy `limit`、组合过滤、稳定排序和越界页；修复无效 UUID 过滤返回 500、Audit UUID URN 原值绑定、空媒体精确参数退回分页和 public URL 裁剪误匹配。白盒总覆盖率提升到 `30.4%`，Linux PostgreSQL race、完整 preflight/E2E、管理员 HTTP 边界和默认黑盒 `21/21` 均通过。详见 [分页 PostgreSQL 集成测试](pagination-integration.md)。

2026-07-13 SEC-P15 基线结果：新增文章/页面 owner scope、发布记录范围和系统角色精确权限对账，白盒总覆盖率提升到 `35.8%`。真实 PostgreSQL 对象隔离连续 3 轮通过；Admin Playwright run `ff53c1bde3e6` 覆盖 super_admin、author、viewer 的内容深链、发布中心、taxonomy、媒体、评论和设置权限裁剪；默认 HTTP 黑盒 `21/21`、Admin production build、Linux PostgreSQL race 与完整 preflight/E2E 均通过。详见 [内容对象级访问控制验收](content-access-integration.md)。

2026-07-13 P17 基线结果：新增 AST/DOM 内容质量规则、草稿检查 API、事务级发布门禁和 Worker/Retry/site 纵深复检。真实 PostgreSQL 覆盖 422 零副作用、双发布、active job 禁改、高权限绕过、401/403、Worker/Retry；Admin Playwright 以 API 拦截模式验证未保存表单、状态失效、发布请求顺序和 1280x720/390x844 Drawer，不写 development/production 数据。`go vet`、全量 Go、Admin build、Hugo 45 pages 与 `preflight -SkipE2E` 均通过。详见 [P17 内容质量验收](content-quality-p17.md)。

首次 seed 并发集成测试会创建并删除专用 `zoking_blog_seed_*_test` 数据库，同时启动两个 seed 进程：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\seed-concurrency.ps1
```

脚本拒绝 development/production 数据库名称，结束时删除临时数据库和测试二进制。通过标准为两个进程均成功、active 管理员唯一、super_admin 绑定唯一、角色权限无重复。

## 7. 冒烟清单

每次阶段完成至少跑：

- API `/healthz`。
- Admin 登录。
- 文章创建。
- 媒体上传。
- 发布任务。
- Hugo build。
- C 端首页。
- C 端文章页。
- 评论提交。
- 评论审核。

## 8. 测试数据

测试数据必须包含：

- 一个超级管理员。
- 一个作者。
- 一篇草稿。
- 一篇已发布文章。
- 一个分类。
- 两个标签。
- 一个封面图。
- 一条待审核评论。

## 9. 缺陷记录

缺陷进入工作日志或 issue 时必须包含：

- 任务编号。
- 环境。
- 复现步骤。
- 实际结果。
- 期望结果。
- 日志摘要。
- 截图或命令输出。
- 严重级别。
