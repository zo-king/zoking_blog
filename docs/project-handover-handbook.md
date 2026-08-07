# Zoking Blog 项目介绍与交接手册

> 最后核验：2026-08-07（Asia/Shanghai）
> 适用仓库：`https://github.com/zo-king/zoking_blog`
> 生产域名：`zoking.tech`
> 文档原则：不记录密码、私钥、Token、WireGuard 私钥或完整生产环境变量值。

## 1. 文档用途与信息可信度

这份手册面向第一次接手项目的开发者和运维人员，目标是让接手者能够回答四个问题：

1. 这是一个什么产品，各个组件分别负责什么。
2. 如何在本地启动、开发、测试和构建。
3. 生产流量如何进入系统，内容如何从后台变成线上静态页面。
4. 出现故障、需要发布、回滚、备份或恢复时，应当执行什么操作。

信息来源按以下优先级理解：

1. 生产服务器的实时状态和实际配置。
2. `infra/docker/compose.prod.yml`、数据库 migration 与应用代码。
3. 本手册和 `docs/operations/deployment-runbook.md`。
4. 设计文档、计划文档与历史工作日志。

设计文档可能描述目标形态，不能替代对生产实况的检查。每次重大部署后都应更新本手册的“生产环境实况”和“交接清单”。

## 2. 一句话介绍

Zoking Blog 是一个“静态阅读体验 + 动态内容控制面”的全栈个人博客系统：读者访问由 Hugo 构建的静态站点，管理员在 React 后台编辑内容，Go API 将结构化内容保存在 PostgreSQL 中，独立 Worker 生成并原子切换 Hugo release，评论、浏览、点赞和统计等动态能力通过 API 与 GoatCounter 提供。

## 3. 产品边界

### 3.1 面向读者的能力

- 文章、独立页面、分类、标签、系列和归档。
- Hugo Theme Stack 风格的响应式阅读体验。
- Pagefind 本地全文搜索。
- RSS、Sitemap、Open Graph 社交卡片和 PWA 基础能力。
- 文章目录、章节链接、代码复制、阅读设置、打印样式。
- 文章评论、浏览量和点赞等渐进增强功能。
- Projects、Now、Links、About 等个人站点页面。
- GitHub 项目快照展示；访客浏览器不会直接调用 GitHub API。

静态页面是主阅读路径。评论或统计 API 短暂不可用时，文章正文仍然应当可读。

### 3.2 面向管理员的能力

- 登录、Cookie 会话、CSRF 防护和数据库实时 RBAC。
- 仪表盘、文章、页面、分类、标签、系列和成果管理。
- 媒体上传、引用保护和孤立文件清理。
- 评论审核与回复。
- 内容质量检查、隔离预览、发布、失败重试和 release 回滚。
- 用户、角色和权限管理。
- 站点设置与操作审计。

### 3.3 当前不属于系统的能力

- Azure 不运行应用容器或数据库。
- Hugo 站点不直接读取 PostgreSQL。
- Admin 不直接写 Hugo 发布目录。
- PostgreSQL 不对局域网、WireGuard 或公网发布端口。
- `preview.zoking.tech` 没有首页；它只承载带随机预览键的 `/preview-files/*` 路径。

## 4. 总体架构

```text
公网用户
  |
  | HTTPS 443 / HTTP 80 -> 308
  v
Azure VPS 104.41.165.102
  |- Caddy：TLS 终止、自动证书、按域名反向代理
  `- WireGuard wg0：10.20.0.1/24，UDP 51820
         |
         | 加密隧道
         v
物理服务器 192.168.0.21 / 10.20.0.2
  `- Docker Compose：/opt/zoking-blog
       |- site        10.20.0.2:1313 -> Nginx -> active Hugo release
       |- api         10.20.0.2:18080 -> Gin API
       |- admin       10.20.0.2:8081 -> Nginx -> React 静态资源
       |- goatcounter 10.20.0.2:8100 -> 访问统计
       |- worker      无宿主机端口 -> 发布任务
       `- postgres    仅 Compose 网络 -> PostgreSQL 16
```

### 4.1 公网域名映射

| 域名 | Azure Caddy 上游 | 用途 | 根路径预期 |
|---|---:|---|---:|
| `zoking.tech` | `10.20.0.2:1313` | 读者站点和媒体 | `200` |
| `api.zoking.tech` | `10.20.0.2:18080` | Public/Admin API | 根路径 `404`，`/readyz` 为 `200` |
| `admin.zoking.tech` | `10.20.0.2:8081` | 管理后台 | `200` |
| `preview.zoking.tech` | `10.20.0.2:18080` | 隔离预览文件 | 根路径 `404` |
| `stats.zoking.tech` | `10.20.0.2:8100` | GoatCounter | 根路径通常 `303` |

所有 A 记录在 2026-08-07 均解析到 `104.41.165.102`。

### 4.2 为什么采用这套结构

- 物理服务器保存应用和数据，不需要直接暴露公网端口。
- Azure 提供稳定公网 IP、TLS 入口和云侧网络安全组。
- WireGuard 把反向代理上游限制在两台机器之间的私网。
- Hugo 静态页面降低读路径复杂度，便于缓存、SEO 和快速回滚。
- PostgreSQL 作为编辑事实源，避免直接编辑发布产物造成状态漂移。

## 5. 组件说明

### 5.1 Reader Site：`apps/site`

- 技术：Hugo Extended，Hugo Theme Stack v4.0.3。
- 输入：仓库模板、数据库内容快照、站点设置、媒体引用。
- 输出：静态 HTML、CSS、JavaScript、图片、RSS、Sitemap 和 Pagefind 索引。
- 生产读取：`site` Nginx 容器只读挂载 `site_releases` 与 `media_data`。
- 主题来源：仓库根目录通过 Hugo module `replace` 作为本地主题模块。

### 5.2 API：`apps/api/cmd/api`

- 技术：Go 1.23、Gin 1.10、GORM 1.30、pgx。
- 监听：容器内 `18080`。
- 探针：`GET /healthz` 检查进程，`GET /readyz` 同时检查数据库连通性。
- 职责：认证、RBAC、内容 CRUD、评论、媒体、设置、审计、预览与发布任务编排。
- 生产配置中 `PUBLISH_WORKER_ENABLED=false`，API 本身不会消费发布队列。

### 5.3 Publish Worker：`apps/api/cmd/worker`

- 与 API 使用同一镜像和数据库。
- 使用数据库锁领取发布任务，执行快照、Hugo 构建、Pagefind、校验和原子 promote。
- 不发布宿主机端口。
- Worker 停止不会影响既有静态页面阅读，但所有新发布任务会停留在队列中。
- 生产检查不能只看五个对外服务；必须确认 `docker-worker-1` 处于运行状态。

### 5.4 Admin：`apps/admin`

- 技术：React 18、TypeScript、Vite 6、Arco Design、ECharts。
- 生产镜像由 Nginx 提供静态资源。
- `runtime-config.js` 在容器启动时注入 API 和站点地址，无需为域名变化重新构建前端。
- 浏览器通过 HttpOnly Cookie 保存会话；写请求额外要求 CSRF Token。

### 5.5 PostgreSQL

- 镜像：`postgres:16-alpine`。
- 卷：`postgres_data`。
- 仅 Compose 内部网络可访问，不映射宿主机端口。
- migration 由 `apps/api/cmd/migrate` 与 `db/migrations` 管理。
- Seed 是幂等初始化，不会覆盖已存在管理员密码或后台已修改的站点设置。

### 5.6 GoatCounter

- 自托管访问统计，数据卷为 `goatcounter_data`。
- 对外域名为 `stats.zoking.tech`。
- GoatCounter 账号密码只用于首次初始化，不能进入 Git。

### 5.7 Caddy 与 WireGuard

- Caddy 只运行在 Azure VPS，配置路径 `/etc/caddy/Caddyfile`。
- Caddy 自动签发并续期证书，HTTP 自动重定向 HTTPS。
- WireGuard VPS 地址为 `10.20.0.1`，物理机地址为 `10.20.0.2`。
- 应用端口应绑定 `10.20.0.2`，并由物理机防火墙限制为 `wg0` 来源。

## 6. 仓库地图

```text
zoking-blog/
|- apps/
|  |- api/                 Go API、worker、migrate、seed 与测试
|  |- admin/               React 管理后台与生产 Nginx 配置
|  `- site/                Hugo 站点配置、内容、布局、资源和静态文件
|- db/migrations/          PostgreSQL migration，按时间戳顺序执行
|- infra/docker/           开发/生产 Compose、Dockerfile 辅助配置
|- scripts/
|  |- dev/                 本地启动和清理
|  |- qa/                  preflight、E2E、黑盒与白盒检查
|  `- ops/                 生产备份、恢复和健康检查
|- docs/
|  |- architecture/        架构与 ADR
|  |- backend/             API 契约、认证与后端规划
|  |- database/            数据模型和迁移策略
|  |- frontend/            Reader/Admin 设计与功能文档
|  |- operations/          部署和生产运维
|  |- qa/                  测试策略与验收记录
|  |- requirements/        需求与能力边界
|  `- process/             工作日志、任务板与交接规则
|- layouts/assets/...      上游 Stack 主题及仓库级覆盖
|- .github/workflows/      CI preflight 与 GitHub 项目快照同步
|- .env.example            本地开发变量模板
`- README.md               快速入口
```

2026-08-07 盘点时仓库约有 768 个跟踪文件，其中包括 107 个 Go 文件、41 个 Go 测试、64 个 TypeScript/TSX 文件、118 个 Hugo/HTML 模板、19 个 SQL migration 和 183 个 Markdown 文档。

## 7. 关键数据与状态

### 7.1 数据库是编辑事实源

后台中的文章、页面、分类、标签、系列、成果、媒体元数据、评论、用户权限、站点设置、发布任务和审计日志以 PostgreSQL 为准。

主要表域：

- 身份：`users`、`roles`、`permissions`、`user_roles`、`role_permissions`、`refresh_tokens`。
- 内容：`posts`、`post_revisions`、`pages`、`categories`、`tags`、`series` 及关系表。
- 媒体：`media_assets`、`media_usages`。
- 互动：`comments`、文章浏览与点赞统计表。
- 发布：`publish_jobs`、`publish_snapshots`、`publish_releases`、`publish_previews`。
- 运维：`site_settings`、`audit_logs`。

实际 schema 必须以 `db/migrations/*.sql` 为准，设计文档只用于解释。

### 7.2 Docker 持久化卷

| Compose 卷 | 内容 | 是否必须备份 |
|---|---|---|
| `postgres_data` | PostgreSQL 数据目录 | 是，但主要使用逻辑 `pg_dump` |
| `media_data` | 上传媒体 | 是 |
| `site_releases` | release、current、preview 等发布产物 | 是 |
| `publisher_site` | Worker 的 Hugo 工作树 | 建议 |
| `goatcounter_data` | 访问统计数据 | 是 |

Compose 项目目录为 `infra/docker`，默认生成的实际卷名通常以 `docker_` 开头。运维脚本应使用 Compose 查询卷，不依赖人工猜测。

### 7.3 发布状态机

```text
requested -> snapshotting -> building -> verifying -> promoting -> published
                   |             |            |
                   `-----------> failed <-----'

人工取消 -> cancelled
```

发布成功前不会覆盖 active release。回滚是将历史 release 重新 promote，而不是反向改写数据库文章。

## 8. 关键请求链路

### 8.1 读者访问文章

1. 浏览器请求 `https://zoking.tech/...`。
2. DNS 返回 Azure 公网 IP。
3. Caddy 终止 TLS，并经 WireGuard 代理到 `10.20.0.2:1313`。
4. `site` Nginx 从 active release 返回静态 HTML 和资源。
5. 评论、浏览和点赞脚本按需请求 `https://api.zoking.tech/api/v1/public/*`。

### 8.2 管理员编辑内容

1. 浏览器访问 `admin.zoking.tech`。
2. Admin 使用运行时配置连接 `api.zoking.tech`。
3. 登录成功后 API 设置 Secure、HttpOnly Cookie，并返回 CSRF Token。
4. Admin 的写请求经过来源白名单、认证、CSRF、审计和权限中间件。
5. API 在事务中更新 PostgreSQL；Admin 不触碰发布目录。

### 8.3 发布内容

1. Admin 发起预览或发布。
2. API 校验权限、内容质量、slug、媒体引用和状态。
3. API 创建 `publish_job`。
4. Worker 从 PostgreSQL 领取任务。
5. Worker 生成 Hugo 内容与配置快照。
6. Worker 执行 Hugo Extended 和 Pagefind 构建。
7. Worker 校验首页、目标页、RSS、Sitemap 和静态资源。
8. Worker 创建不可变 release 并原子切换 `current`。
9. `site` Nginx 后续请求立即读取新 release。

## 9. API 边界

### 9.1 健康检查

- `GET /healthz`：API 进程可响应。
- `GET /readyz`：数据库连接也正常。

### 9.2 Public API

前缀：`/api/v1/public`

- 文章、页面、分类、标签、系列读取。
- 公开评论读取与提交。
- 浏览、点赞和文章指标。
- 公开站点设置。

### 9.3 Admin API

前缀：`/api/v1/admin`

- 登录、刷新、恢复会话、登出和当前用户。
- 内容、分类、标签、系列、成果、媒体和评论管理。
- 发布任务、预览、release、重试、取消、清理与 promote。
- 用户、角色、权限和密码重置。
- 站点设置、统计概览和审计日志。

完整字段和权限以 `docs/backend/00-api-contract.md`、`apps/api/internal/httpapi/router.go` 与 RBAC migration 为准。

## 10. 配置管理

### 10.1 配置文件边界

| 文件 | 环境 | 可提交 |
|---|---|---|
| `.env.example` | 本地开发模板 | 是 |
| `.env` | 本机开发值 | 否，已忽略 |
| `infra/docker/.env.prod.example` | 生产模板 | 是 |
| `infra/docker/.env.prod` | 生产真实值 | 否，必须仅服务器可读 |

生产 `.env.prod` 建议权限：目录 `0750`，文件 `0600`，所有者为部署账号或 root。备份副本同样必须限制权限。

### 10.2 必须重点保护的变量

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `PRIVACY_HASH_SECRET`
- `SEED_ADMIN_PASSWORD`
- `GOATCOUNTER_PASSWORD`

这些值不能出现在 Git、聊天记录、命令历史、CI 日志、截图或普通运维日志中。

### 10.3 单一来源变量

- `SITE_BASE_URL`：正式值 `https://zoking.tech/`，保留末尾 `/`。
- `PUBLIC_API_BASE_URL`：正式值 `https://api.zoking.tech`，不带末尾 `/`。
- Admin API 地址和 Hugo 评论 API 地址都由上述生产变量派生，不另设第二套生产来源。

### 10.4 生产绑定与来源

- `API_BIND_ADDRESS=10.20.0.2`
- `ADMIN_BIND_ADDRESS=10.20.0.2`
- `SITE_BIND_ADDRESS=10.20.0.2`
- `STATS_BIND_ADDRESS=10.20.0.2`
- `CORS_ALLOWED_ORIGINS=https://zoking.tech`
- `ADMIN_ALLOWED_ORIGINS=https://admin.zoking.tech`
- `PUBLISH_PREVIEW_PUBLIC_BASE_URL=https://preview.zoking.tech/preview-files`

非 loopback 绑定只有在物理机防火墙已严格限制 WireGuard 来源时才安全。

## 11. 本地开发

### 11.1 依赖

- Go 1.23 或更高兼容版本。
- Node.js 22。
- npm（使用 `package-lock.json` 和 `npm ci`）。
- Hugo Extended 0.160.1；Windows 仓库可使用 `.tools/hugo` 中的本地工具。
- Docker Desktop 与 Docker Compose v2。
- PowerShell 7。
- Git。

### 11.2 首次启动

```powershell
Copy-Item .env.example .env
pwsh -NoProfile -File .\scripts\dev\up.ps1
```

然后分别启动：

```powershell
# API，默认 http://localhost:18080
Set-Location apps/api
go run ./cmd/api

# Admin，默认 http://localhost:5173
Set-Location apps/admin
npm ci
npm run dev

# Reader，默认 http://localhost:1313
Set-Location ../..
.\.tools\hugo\hugo.exe server --source apps/site
```

开发 Compose 只启动 PostgreSQL，并将其映射到 `localhost:15432`。

### 11.3 Migration 与 Seed

```powershell
Set-Location apps/api
go run ./cmd/migrate up
go run ./cmd/seed
```

新 migration 必须新增文件，不能修改已经在生产执行过的 migration。Seed 必须保持幂等。

### 11.4 常用地址

| 服务 | 地址 |
|---|---|
| Reader | `http://localhost:1313` |
| Admin | `http://localhost:5173` |
| API health | `http://localhost:18080/healthz` |
| API ready | `http://localhost:18080/readyz` |
| PostgreSQL | `localhost:15432` |

## 12. 测试、质量门禁与 CI

### 12.1 本地门禁

完整 preflight：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1
```

跳过 E2E 的构建检查：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
```

生产变量只读校验：

```powershell
pwsh -NoProfile -File .\scripts\qa\production-preflight.ps1 \
  -EnvFile infra/docker/.env.prod \
  -AllowNonLoopbackBindings
```

### 12.2 GitHub Actions

`Preflight` 工作流在 PR 和 `main` push 时执行：

- golangci-lint。
- Admin ESLint 与 Prettier。
- Go race detector。
- PostgreSQL 集成环境。
- Go 测试、Admin 构建、Hugo 构建、migration/seed 和 E2E。

`Sync GitHub projects` 每月 1 日和 16 日刷新 `apps/site/data/github_projects.json`，验证 Hugo build 后由 bot 提交到 `main`。因此本地长期分支可能被定时提交拉开，推送前必须先 fetch/rebase。

## 13. 生产环境实况

### 13.1 物理服务器

| 项目 | 当前值 |
|---|---|
| 局域网 IP | `192.168.0.21` |
| WireGuard IP | `10.20.0.2` |
| 系统 | Ubuntu Server 24.04.4 LTS |
| 项目目录 | `/opt/zoking-blog` |
| Compose 文件 | `infra/docker/compose.prod.yml` |

已交接运行容器：`docker-site-1`、`docker-api-1`、`docker-admin-1`、`docker-goatcounter-1`、`docker-postgres-1`。`site-init` 是一次性容器，不要求常驻；`docker-worker-1` 按当前 Compose 设计必须常驻，但此前清单未列出，接手后应优先核验。

端口：

- `10.20.0.2:1313`：Reader。
- `10.20.0.2:18080`：API。
- `10.20.0.2:8081`：Admin。
- `10.20.0.2:8100`：GoatCounter。

Docker 和 `wg-quick@wg0` 已配置开机启动。

### 13.2 Azure VPS

| 项目 | 当前值 |
|---|---|
| 公网 IP | `104.41.165.102` |
| WireGuard IP | `10.20.0.1/24` |
| WireGuard UDP | `51820` |
| SSH 用户 | `zoking` |
| SSH 私钥位置（本机） | `C:\Users\zhaoxi\.azure\zoking_key.pem` |
| Caddy 配置 | `/etc/caddy/Caddyfile` |

`caddy`、`wg-quick@wg0`、`ssh` 在 2026-08-07 均为 enabled + active。Azure NSG 和 UFW 当前放行 22/TCP、80/TCP、443/TCP、51820/UDP；SSH 来源仍应进一步收窄。

### 13.3 本机网络绕过

Chaoshihui 配置位于：

```text
C:\Users\zhaoxi\AppData\Roaming\chaoshihui\chaoshihui\shared_preferences.json
```

已绕过 `104.41.165.102`、`192.168.0.21`、`10.20.*`。Windows 还存在 `104.41.165.102/32 -> 192.168.0.1` 的 WLAN 持久路由，目的是让 SSH 与 WireGuard 流量直连，同时浏览器和聊天保持代理。

配置备份：

```text
C:\Users\zhaoxi\AppData\Roaming\chaoshihui\chaoshihui\shared_preferences.json.bak-codex-20260807-023907
```

## 14. 生产部署流程

### 14.1 部署前

1. 确认 Git 工作区干净、目标 commit 已通过 CI。
2. 在物理机执行一次成功备份，并校验 manifest。
3. 核对磁盘空间、WireGuard、Caddy 和当前公网探针。
4. 检查 `.env.prod` 不含占位符且未被 Git 跟踪。
5. 记录当前 commit、当前 active release 和容器镜像 ID，作为回滚基线。

### 14.2 更新代码并校验

```bash
cd /opt/zoking-blog
git fetch --prune origin
git status --short --branch
git pull --ff-only origin main
pwsh -NoProfile -File scripts/qa/production-preflight.ps1 -AllowNonLoopbackBindings
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml config --quiet
```

生产服务器不应存在临时手工代码修改。若 `git pull --ff-only` 失败，应停止并确认漂移，不能强制覆盖。

### 14.3 构建、迁移和启动

```bash
cd /opt/zoking-blog
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml build
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml up -d postgres
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml run --rm api /app/migrate up
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml up -d api worker admin site goatcounter
```

Seed 只在首次初始化或明确需要修复基础角色/权限时运行：

```bash
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml run --rm seed
```

### 14.4 部署后验证

```bash
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml ps
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml logs --tail=100
curl -fsS http://10.20.0.2:18080/readyz
curl -fsS http://10.20.0.2:1313/ >/dev/null
curl -fsS http://10.20.0.2:8081/ >/dev/null
```

公网检查：

```bash
curl -fsS https://zoking.tech/ >/dev/null
curl -fsS https://api.zoking.tech/readyz
curl -fsS https://admin.zoking.tech/ >/dev/null
curl -sS -o /dev/null -w '%{http_code}\n' https://preview.zoking.tech/
curl -sS -o /dev/null -w '%{http_code}\n' https://stats.zoking.tech/
curl -sS -o /dev/null -w '%{http_code} %{redirect_url}\n' http://zoking.tech/
```

预期依次为站点/API/Admin `200`、Preview 根路径 `404`、Stats `303`、HTTP 跳转 `308`。

## 15. 日常运维

### 15.1 Azure 登录与检查

```powershell
ssh -o IdentitiesOnly=yes -i "C:\Users\zhaoxi\.azure\zoking_key.pem" zoking@104.41.165.102
```

```bash
sudo wg show
sudo systemctl status wg-quick@wg0 --no-pager
sudo systemctl status caddy --no-pager
sudo journalctl -u caddy -n 100 --no-pager
sudo caddy validate --config /etc/caddy/Caddyfile
```

### 15.2 物理机检查

```bash
cd /opt/zoking-blog
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml ps
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml logs --tail=100
sudo wg show
df -hT
```

按服务查看日志：

```bash
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml logs --tail=200 api
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml logs --tail=200 worker
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml logs --tail=200 postgres
```

### 15.3 安全重启顺序

常规 Compose 变更：

```bash
cd /opt/zoking-blog
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml up -d
```

避免无必要执行 `down -v`。`-v` 会删除数据库和媒体等持久卷，属于灾难性操作。

## 16. 备份策略

### 16.1 备份对象

每次生产备份至少包含：

- PostgreSQL 逻辑备份（custom format）。
- `media_data` 上传媒体。
- `site_releases` 与当前 active release。
- `publisher_site` 工作树。
- `goatcounter_data` 统计数据。
- `infra/docker/.env.prod`、Compose 文件和当前 Git commit。
- SHA-256 manifest 与备份日志。

### 16.2 保留策略

- 日备份：保留 7 天。
- 周备份：保留 4 份。
- 月备份：保留 3 份。
- 至少一份副本必须位于另一台机器或对象存储。
- 环境文件和数据库备份在异机保存时必须限制访问，推荐额外加密。

### 16.3 自动任务

仓库提供：

- `scripts/ops/backup-production.sh`：创建备份、manifest 和分层保留副本。
- `scripts/ops/restore-production.sh`：显式恢复数据库或卷，默认只做检查。
- `infra/systemd/zoking-backup.service` 与 `.timer`：每日执行。

安装步骤见 `docs/operations/backup-and-monitoring.md`。

### 16.4 恢复原则

- 恢复前先停止 Worker 和所有写入 API，避免备份恢复期间产生新数据。
- 恢复数据库前再备份一次当前状态，即使当前状态有问题。
- 不直接覆盖 active release；先恢复到临时位置并校验。
- 恢复完成后执行 `/readyz`、登录、媒体读取、文章发布和公网黑盒验证。
- 每月至少演练一次恢复，并记录 RPO、RTO、耗时和问题。

## 17. 监控与告警

### 17.1 物理机检查项

- Docker daemon 与 Compose 必需服务状态。
- API `/readyz`、Reader、Admin、Stats 本机端口。
- PostgreSQL 健康状态。
- Worker 是否运行、失败发布任务是否增长。
- WireGuard 最近握手时间。
- 根分区和 Docker 数据分区空间。
- 最近一次备份年龄与 manifest 完整性。
- systemd 失败单元。

### 17.2 Azure 检查项

- Caddy 与 WireGuard active 状态。
- `10.20.0.2` 四个上游端口。
- 五个公网 HTTPS 域名状态。
- HTTP -> HTTPS 重定向。
- TLS 证书剩余天数。
- WireGuard 最近握手时间。
- 根分区空间和 systemd 失败单元。

### 17.3 告警渠道

健康检查脚本支持可选 `ALERT_WEBHOOK_URL`。未配置外部 Webhook 时，失败会进入 systemd journal 并使服务单元失败，但不会主动通知手机或邮箱。生产交接必须明确告警接收人和渠道，不能把“有检查脚本”误认为“有人会收到告警”。

## 18. 安全基线

### 18.1 SSH

- 两台服务器必须先验证至少一个密钥可登录，再关闭密码登录。
- 推荐设置 `PasswordAuthentication no`、`KbdInteractiveAuthentication no`、`PermitRootLogin no`。
- 修改前运行 `sshd -t`，使用新 SSH 会话验证后再退出旧会话。
- Azure NSG 的 22/TCP 应限制到固定管理公网 IP `/32`，或改用 Bastion/VPN。
- UFW 与 NSG 都要收窄；只改其中一层不算完成。

### 18.2 网络

- Azure 公网仅开放 80/TCP、443/TCP、51820/UDP 和受限来源的 22/TCP。
- API、Admin、Stats 和 Reader 上游只绑定物理机 WireGuard IP。
- PostgreSQL 不发布端口。
- `TRUSTED_PROXIES` 只列真实代理来源，不允许任意公网代理。

### 18.3 应用与凭据

- 管理员密码至少 16 字符，泄露后立即轮换。
- 重置管理员密码后必须同步更新生产 `.env.prod` 中的 seed 密码，避免灾难重建重新使用旧密码。
- 重置密码会删除目标用户 refresh token；JWT secret 轮换会使所有访问 Token 失效。
- 不在日志中记录 Cookie、Token、密码、完整请求体或真实 IP；审计只保存哈希化网络标识和安全元数据。

## 19. 故障处理

### 19.1 所有公网域名同时故障

按顺序检查：

1. DNS 是否仍指向 Azure 公网 IP。
2. Azure VM 是否运行，NSG/UFW 是否允许 80/443。
3. Caddy 状态与日志。
4. WireGuard 握手。
5. Azure 到 `10.20.0.2` 的上游访问。
6. 物理机电源、网络、Docker 和 Compose。

### 19.2 站点可读但后台发布不动

优先检查 `worker`，而不是重启站点：

```bash
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml ps worker
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml logs --tail=200 worker
```

然后检查数据库中的 `publish_jobs` 状态、失败原因、锁和重试次数。

### 19.3 API health 正常但 ready 失败

这通常表示 API 进程存活但 PostgreSQL 不可用。检查 `postgres` 健康状态、磁盘、数据库日志、连接字符串和 migration，不应把 `/healthz` 当作完整可用性证明。

### 19.4 Preview 根路径 404

这是正常行为。必须使用后台生成的 `/preview-files/<key>/...` 地址验证预览。

### 19.5 Stats 303

GoatCounter 根路径跳转通常返回 `303`，属于当前正常基线。探针应允许该状态或检查 `/status`。

### 19.6 证书问题

检查 Caddy 日志、DNS、80/443 入站和系统时间。Caddy 自动续期，不要手工覆盖其证书存储。修复后使用外部网络重新检查证书链和到期时间。

## 20. Git 与发布纪律

- `origin` 是项目仓库；`upstream` 是 Hugo Theme Stack 上游。
- GitHub Actions 会自动向 `main` 提交项目快照，本地推送前始终执行 `git fetch origin`。
- 本地与远端都新增提交时，先检查 `git log --left-right --cherry-pick HEAD...origin/main`，再 rebase；不要强推共享 `main`。
- `.env`、`.env.prod`、备份、媒体数据、构建产物、缓存和本地工具不能提交。
- 字体、图片等二进制必须在 `.gitattributes` 中标记为 binary，避免换行转换损坏。
- 生产部署只接受可追溯 commit；服务器不应成为代码编辑工作区。

## 21. 本地无用文件清理

安全清理：

```powershell
pwsh -NoProfile -File .\scripts\dev\clean.ps1 -DryRun
pwsh -NoProfile -File .\scripts\dev\clean.ps1
```

默认只删除 Git 忽略且可重建的 build/QA/cache 目录。以下内容默认保留：

- `storage/media`：可能包含开发上传媒体。
- `.tools`：本地 Hugo 等固定工具。
- `apps/admin/node_modules`：可重建但会影响开发启动速度。
- `.env`：本地配置。

只有明确确认媒体可删除时才使用 `-IncludeMedia`。

## 22. 新接手者建议阅读顺序

1. 本手册。
2. `README.md` 与 `docs/README.md`。
3. `docs/architecture/00-system-overview.md`。
4. `docs/architecture/publishing-pipeline.md`。
5. `docs/database/00-data-model.md`。
6. `docs/backend/00-api-contract.md`。
7. `docs/security/security-baseline.md`。
8. `docs/operations/deployment-runbook.md`。
9. `docs/operations/backup-and-monitoring.md`。
10. 当前 `git status --short --branch`、CI 状态和生产健康检查。

## 23. 交接检查清单

### 23.1 访问与权限

- [ ] GitHub 仓库读写权限已验证。
- [ ] Azure 门户/CLI 权限已验证。
- [ ] Azure SSH 密钥登录已验证。
- [ ] 物理机 SSH 密钥登录已验证。
- [ ] 管理后台账号已轮换并安全交付。
- [ ] DNS 管理权限已交付。
- [ ] 告警渠道和接收人已确认。

### 23.2 运行状态

- [ ] 五个域名 DNS 正确。
- [ ] Caddy、WireGuard、Docker 开机自启。
- [ ] `api`、`worker`、`postgres`、`admin`、`site`、`goatcounter` 正常。
- [ ] API `/readyz` 为 200。
- [ ] Preview 根路径 404 被记录为正常。
- [ ] HTTP -> HTTPS 为 308。

### 23.3 安全

- [ ] SSH 密码登录关闭且新密钥会话已验证。
- [ ] Azure NSG 与 UFW 的 SSH 来源已收窄。
- [ ] 物理机应用端口仅允许 WireGuard 来源。
- [ ] 生产管理员、数据库、JWT、隐私哈希和统计密码未泄露。
- [ ] Git 历史和工作区无生产 secret。

### 23.4 灾备与监控

- [ ] 每日备份 timer active。
- [ ] 最近备份可通过 manifest 校验。
- [ ] 异机或对象存储副本存在。
- [ ] 数据库和媒体恢复演练完成。
- [ ] 物理机与 Azure 健康检查 timer active。
- [ ] 磁盘、服务、隧道、备份和证书告警能送达接收人。

## 24. 2026-08-07 已验证基线与已知风险

已验证：

- Azure 密钥 SSH 成功。
- Caddy、WireGuard、SSH 服务 active + enabled。
- 五个域名解析到 `104.41.165.102`。
- `https://zoking.tech/`、`https://api.zoking.tech/healthz`、`/readyz`、`https://admin.zoking.tech/` 返回 200。
- `preview.zoking.tech` 根路径返回 404。
- `stats.zoking.tech` 根路径返回 303。
- HTTP 跳转 HTTPS 返回 308。

仍需在获得服务器管理权限后闭环：

- 物理机当前只确认过密码 SSH，当前本机 Ed25519 密钥尚未被接受。
- Azure 账号需要 sudo 密码，当前非交互命令不能修改 SSH/UFW。
- 本机尚未登录 Azure CLI，不能直接修改 NSG。
- 生产管理员当前密码不等于本地开发 `.env`，不能通过现有本地值登录并轮换。
- 先前容器清单未列出 `worker`，必须检查发布队列消费者。
- 备份与外部告警在服务器上是否已部署，尚需实时核验。

这些项目属于权限/凭据交接问题，不应通过在聊天中再次粘贴密码解决。推荐先把本机公钥加入物理机、用 Azure Portal/Run Command 完成一次密钥和 sudo 加固，再继续自动化部署。
