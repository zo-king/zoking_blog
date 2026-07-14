# 运维与部署规划

## 1. 系统组成

- C 端：Hugo + Hugo Theme Stack，静态博客站点。
- B 端：后台管理系统，用于文章、分类、标签、资源、用户、发布流管理。
- API：Go Gin + GORM。
- 数据库：PostgreSQL。
- 部署形态：本地开发使用 Docker Compose；生产环境建议 API、PostgreSQL、静态站点分层部署。

## 2. 本地开发环境

基础依赖：

- Go 1.22+
- Node.js LTS
- Hugo Extended
- Docker / Docker Compose
- PostgreSQL 客户端工具
- Git

推荐本地服务：

- `postgres`：开发数据库。
- `api`：Gin API 服务。
- `admin`：后台前端。
- `hugo`：本地博客预览。
- `nginx`：可选，用于模拟生产路由。

常用流程：

```bash
docker compose -f infra/docker/compose.dev.yml up -d postgres
cd apps/api && go run ./cmd/migrate up && go run ./cmd/seed && go run ./cmd/api
cd apps/admin && npm install && npm run dev
hugo server --source apps/site
```

## 3. Docker Compose 规划

建议服务：

- `postgres`：持久化数据库，挂载 volume。
- `api`：读取 `.env`，连接 PostgreSQL。
- `admin`：开发时走 dev server，生产时构建静态资源。
- `hugo`：本地预览或构建静态站点。
- `nginx`：生产镜像或本地集成测试入口。

原则：

- 数据库数据必须挂载 volume。
- API 不应把密钥写死到镜像。
- 开发与生产配置拆分：`compose.dev.yml`、`compose.prod.yml`。
- 本地端口统一登记。

## 4. 环境变量

推荐变量：

```env
APP_ENV=development
APP_NAME=zoking-blog
APP_PORT=18080

DATABASE_URL=postgres://user:password@localhost:15432/zoking_blog?sslmode=disable
DB_HOST=localhost
DB_PORT=15432
DB_NAME=zoking_blog
DB_USER=zoking
DB_PASSWORD=change-me

JWT_SECRET=change-me
ACCESS_TOKEN_TTL=30m
REFRESH_TOKEN_TTL=720h

MEDIA_STORAGE_DRIVER=local
MEDIA_LOCAL_DIR=./storage/media

HUGO_SITE_DIR=./apps/site
HUGO_PUBLIC_DIR=./dist/site
```

规则：

- `.env.example` 可以提交。
- `.env`、生产密钥、数据库备份不得提交。
- 新增变量必须同步更新文档、示例文件、部署配置。
- 生产环境使用平台 Secret。

## 5. 构建与发布

API：

```bash
go test ./...
go build -o dist/api ./cmd/api
```

后台：

```bash
npm ci
npm run lint
npm run build
```

Hugo：

```bash
hugo --minify
```

发布前检查：

```bash
pwsh -NoProfile -File ./scripts/qa/preflight.ps1
```

该脚本会执行 Go 测试、Admin 构建、Hugo 构建、数据库 migration/seed 和 E2E 冒烟；详见 `docs/qa/preflight.md`。

## 6. Hugo 发布流水线

流程：

1. 后台保存文章草稿到数据库。
2. 发布时生成 Hugo Markdown 内容。
3. 写入 Hugo content 快照目录。
4. 执行 `hugo --minify`。
5. 将 `public/` 发布到静态托管平台。
6. 清理 CDN 缓存。
7. 记录发布日志。

策略：

- 草稿、预览、正式发布分离。
- 每次发布必须可追溯。
- Hugo 构建失败时不得覆盖线上静态文件。
- 支持回滚到上一次成功构建产物。

## 7. CI/CD

Pull Request：

- GitHub Actions 运行 `.github/workflows/preflight.yml`。
- preflight 覆盖 Go test、Admin build、Hugo build、migrate/seed 和 E2E smoke。

Main 合并：

- 构建 API 镜像。
- 构建后台静态资源。
- 构建 Hugo public。
- 推送镜像或产物。
- 部署 staging。
- 人工确认后部署 production。

分支规则：

- `main`：生产稳定分支。
- `develop`：集成分支。
- `feature/<task-id>-desc`：功能分支。
- `fix/<task-id>-desc`：修复分支。
- `docs/<task-id>-desc`：文档分支。

## 8. 备份与恢复

备份对象：

- PostgreSQL 数据库。
- 上传文件目录。
- Hugo content 快照。
- 部署配置与环境变量模板。
- CI/CD 配置。

数据库备份：

```bash
pg_dump "$DATABASE_URL" > backups/zoking-blog-$(date +%Y%m%d-%H%M%S).sql
```

恢复：

```bash
psql "$DATABASE_URL" < backups/backup.sql
```

要求：

- 生产数据库每日备份。
- 至少保留 7 天日备份、4 周周备份、3 个月月备份。
- 每月至少演练一次恢复。
- 恢复演练必须记录耗时、结果、问题。

## 9. 日志与监控

日志要求：

- API 输出结构化日志。
- 每个请求包含 request id。
- 错误日志包含错误码、路径、用户 ID、堆栈摘要。
- 不记录密码、token、密钥、完整 Cookie。

监控指标：

- API 可用性。
- HTTP 5xx 比例。
- 平均响应时间 / P95 响应时间。
- 数据库连接数。
- PostgreSQL 慢查询。
- Hugo 构建成功率。
- 发布耗时。
- 磁盘空间与备份状态。

告警建议：

- API 连续 3 次健康检查失败。
- 5xx 比例超过阈值。
- 数据库连接耗尽。
- 备份任务失败。
- 发布流水线失败。
- 磁盘空间低于 20%。

## 10. 安全清单

- 密钥不入库。
- 生产启用 HTTPS。
- JWT secret 足够长且定期轮换。
- CORS 使用白名单。
- 管理后台必须登录。
- 高危操作记录审计日志。
- 上传文件限制类型、大小、路径。
- 防止路径穿越。
- API 参数校验。
- GORM 查询避免拼接 SQL。
- 数据库账号最小权限。
- 生产关闭 debug 日志。
- 定期升级 Go、Node、Hugo、依赖包。
- CI 中增加依赖漏洞扫描。
- 备份文件加密存储。
