# 部署 Runbook

本文描述从本地到生产的部署操作思路。实际命令在 Phase 4 根据真实环境固化。

## 1. 环境分层

| 环境 | 用途 | 数据 |
|---|---|---|
| local | 本地开发 | 可重建 |
| test | 自动化测试 | 临时 |
| staging | 上线前验证 | 接近生产 |
| production | 正式环境 | 必须备份 |

## 2. 服务组成

- PostgreSQL。
- API。
- Admin 静态资源或 dev server。
- Hugo build worker。
- C 端静态站点。
- Nginx。
- 可选对象存储。

## 3. 本地启动顺序

1. 复制 `.env.example` 为 `.env`。
2. 启动 PostgreSQL。
3. 执行 migration。
4. 执行 seed。
5. 启动 API。
6. 启动 Admin。
7. 启动 Hugo server 或执行 build。

## 4. 生产部署顺序

1. 备份数据库。
2. 拉取代码或镜像。
3. 检查环境变量。
4. 执行 migration。
5. 启动 API 新版本。
6. 健康检查。
7. 构建 Admin。
8. 构建/发布 C 端 release。
9. 切换 Nginx 或 release 指针。
10. 冒烟测试。
11. 记录部署日志。

## 4.1 Docker Compose 生产基线

当前生产基线文件：

- `infra/docker/compose.prod.yml`
- `infra/docker/.env.prod.example`
- `apps/api/Dockerfile`
- `apps/admin/Dockerfile`

准备环境变量：

```powershell
Copy-Item infra/docker/.env.prod.example infra/docker/.env.prod
```

必须修改：

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `SEED_ADMIN_EMAIL`
- `SEED_ADMIN_PASSWORD`（至少 16 个字符，不能保留 `change-me` 或开发默认值）
- `SITE_BASE_URL`（正式环境为 `https://zoking.tech/`，保留末尾 `/`）
- `PUBLIC_API_BASE_URL`（正式环境为 `https://api.zoking.tech`，不带末尾 `/`）
- `PUBLISH_PREVIEW_PUBLIC_BASE_URL`（正式环境为 `https://preview.zoking.tech/preview-files`，必须独立于站点、API 和 Admin Origin）
- `CORS_ALLOWED_ORIGINS`（公开 API 调用方，正式环境为 `https://zoking.tech`，不启用凭据）
- `ADMIN_ALLOWED_ORIGINS`（后台唯一可信浏览器来源，正式环境为 `https://admin.zoking.tech`）
- `MEDIA_UPLOAD_MAX_CONCURRENCY`（单实例媒体流式上传并发上限，默认 `4`）

`PUBLIC_API_BASE_URL` 是生产公开 API 地址的唯一配置源。Compose 会把 `SITE_BASE_URL` 和 `PUBLIC_API_BASE_URL` 同时传给 `api` 与 `worker`，并用 `PUBLIC_API_BASE_URL` 派生 Admin 容器的 `ADMIN_API_BASE_URL` 和 Hugo 构建使用的 `HUGO_COMMENTS_API_BASE`，不要再单独配置这两个变量。开发环境仍使用 localhost 默认值。

公开评论提交默认按可信客户端 IP 限制为每分钟 10 次、突发 5 次，可通过 `COMMENT_RATE_LIMIT_PER_MINUTE` / `COMMENT_RATE_LIMIT_BURST` 调整。`TRUSTED_PROXIES` 只能包含实际反向代理来源；默认生产值覆盖本机和 Docker 私网，若网络拓扑不同必须同步收窄，不能信任任意公网代理。

### DNS 与 TLS

- 为 `zoking.tech` 配置指向 C 端入口的 A/AAAA（或等价 CNAME/ALIAS）记录。
- 为 `api.zoking.tech`、`admin.zoking.tech` 和 `preview.zoking.tech` 分别配置指向对应反向代理的 DNS 记录。
- 在公网入口为四个域名签发可信 TLS 证书，启用自动续期，并将 HTTP 永久重定向到 HTTPS。
- 将 `https://zoking.tech/` 代理或映射到 `site`/active release，将 `https://api.zoking.tech` 代理到 API 的 `18080` 端口。转发时保留 `Host`、`X-Forwarded-Proto` 和客户端地址头。
- 将 `https://admin.zoking.tech` 代理到 Admin 的 `8081` 端口。三个 Compose 端口默认只绑定 `127.0.0.1`，公网只能经 TLS 反向代理进入；若代理运行在独立容器，应使用共享 Docker network 和服务名，不要改成 `0.0.0.0` 暴露管理端口。
- 将 `https://preview.zoking.tech/preview-files/*` 代理到 API 的同路径，并保留外部 `Host` 与 `X-Forwarded-Proto`。不要在 `api.zoking.tech` 或 `zoking.tech` 暴露该路径；生产 API 会对错误 Host 返回 404。
- TLS 与路由就绪后，再执行生产发布冒烟检查；不要直接把 Compose 暴露的开发端口作为正式入口。

构建并启动：

```powershell
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml build
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml up -d postgres
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml run --rm api /app/migrate up
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml run --rm seed
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml up -d api worker admin site
```

首次初始化时，`seed` 会直接使用 `SITE_BASE_URL` 和 `PUBLIC_API_BASE_URL` 写入 `site.base_url` 与 `comments.api_base`，并拒绝已知默认管理员凭据。站点设置采用 insert-only，重跑 seed 不会覆盖后台已经调整的标题、评论开关、分页或域名。已有数据库改域名时仍应在 Admin「站点设置」更新并重新发布。也可以通过 `PATCH /api/v1/admin/settings` 完成同一操作：

```json
{
  "site": { "base_url": "https://zoking.tech/" },
  "comments": { "api_base": "https://api.zoking.tech" }
}
```

更新后通过 `GET /api/v1/admin/settings` 核对持久化值，再触发 `POST /api/v1/admin/settings/publish`。浏览器和运维脚本使用登录返回的 HttpOnly Cookie 会话；写请求还必须携带登录响应中的 `X-CSRF-Token`。账号必须具有 `setting:update` / `publish:create` 权限。

说明：

- `api` 和 `worker` 共用同一个 Go 镜像。
- `api` 默认 `PUBLISH_WORKER_ENABLED=false`，发布任务由独立 `worker` 服务处理。
- `admin` 是 Nginx 静态站，启动时根据 `ADMIN_API_BASE_URL` 生成 `runtime-config.js`。
- `worker` 镜像内置 Hugo Extended，用于执行 release build。
- `site_releases` 卷保存 Hugo release 产物；生产 Nginx/Caddy 应将线上 C 端指向当前 active release 或后续 `current` 指针。
- `site` 服务直接读取 `/data/current`；worker/promote 在发布成功后切换该目录。
- `PUBLISH_RELEASE_KEEP_LATEST` / `PUBLISH_RELEASE_KEEP_DAYS` 控制历史 inactive release 清理策略。
- `PUBLISH_PREVIEW_ROOT` 保存隔离预览产物，`PUBLISH_PREVIEW_PUBLIC_BASE_URL` 控制公开路径，`PUBLISH_PREVIEW_TTL` 默认 `24h`。
- `PUBLISH_PREVIEW_CLEANUP_INTERVAL` 默认 `1h`，`PUBLISH_PREVIEW_CLEANUP_BATCH_SIZE` 默认 `100`；设置 interval 为 `0` 可关闭定时清理。
- `MEDIA_ORPHAN_GRACE_PERIOD` / `MEDIA_CLEANUP_BATCH_SIZE` 控制孤立媒体清理策略。

生产发布会校验站点公开 URL：`site.base_url` 和启用评论时的 `comments.api_base` 必须是 HTTPS，且主机不能是 localhost、`127.0.0.0/8`、`::1` 等 loopback 地址；同时必须分别与部署时的 `SITE_BASE_URL`（`https://zoking.tech/`）和 `PUBLIC_API_BASE_URL`（`https://api.zoking.tech`）一致。任一 URL 非 HTTPS、使用 loopback，或与部署域名不一致时，生产发布会被拒绝。环境变量正确并不会自动修正数据库中的旧值，发布前必须完成上面的站点设置更新。

预览静态文件只通过 `https://preview.zoking.tech/preview-files/{preview_key}/...` 提供，并带 `noindex, nofollow`、`private, no-store`、`no-referrer` 和严格 CSP。生产反向代理必须保留该独立 Host；同一路径通过 `api.zoking.tech` 必须返回 404。预览目录与 release 共用持久卷，但不允许将预览目录作为 C 端 `current`。

过期预览即使目录尚未被定时器删除，静态入口也会根据数据库 `expires_at` 返回 404。worker 定时执行清理并保留 `expired` 记录；手动清理应先调用 dry-run，再显式 apply。

最小健康检查：

```powershell
Invoke-RestMethod http://localhost:18080/healthz
Invoke-RestMethod http://localhost:18080/readyz
Invoke-WebRequest http://localhost:8081/healthz
Invoke-WebRequest http://localhost:1313/
```

首次部署必须先登录 Admin，核对站点设置并执行一次“发布站点”；在 active release 生成前，`site` 首页检查和容器 healthcheck 应保持失败，不能据 API/Admin 健康状态判定整站上线。发布后使用只读检查：

```powershell
Invoke-WebRequest https://zoking.tech/
Invoke-WebRequest https://zoking.tech/sitemap.xml
Invoke-RestMethod https://api.zoking.tech/healthz
Invoke-WebRequest https://admin.zoking.tech/healthz
Resolve-DnsName preview.zoking.tech
```

创建一条临时预览后，必须确认预览域名返回 200 和上述安全响应头，且相同 `/preview-files/...` 路径经 `https://api.zoking.tech` 返回 404。

禁止对生产数据库运行 `scripts/qa/e2e-smoke.ps1`。该脚本会创建测试内容并临时修改站点设置，只能按 `docs/qa/e2e-smoke.md` 在隔离的 `_test` 数据库和文件根中执行。

## 4.2 C 端 release 服务策略

当前实现已经在数据库中维护 active release，并把 release 构建到 `site_releases` 卷内的 `releases/{release_key}`。

当前策略：

- 将 active release 暴露给 Web 服务器的 `current` 目录。
- 在 publish/promote/rollback 时同步更新 `current`。
- Web 服务只读取 `current`，不直接读所有历史 release。
- 媒体文件开发和 compose 生产基线使用本地 volume + `/media-files` 静态路由；生产 CDN 可以把 `MEDIA_PUBLIC_BASE_URL` 指向 CDN 域名。

当前 compose 已提供 `site` 服务，但若生产环境不用 compose，仍需要确保外部 Web 服务器根目录指向 `current`。

## 4.3 媒体与 Release 清理

默认先 dry-run：

```powershell
$login = Invoke-RestMethod -Method POST -Uri http://localhost:18080/api/v1/admin/auth/login -SessionVariable adminSession -ContentType "application/json" -Body '{"email":"admin@zoking.local","password":"<admin-password>"}'
$headers = @{ "X-CSRF-Token" = [string]$login.data.csrf_token }
Invoke-RestMethod -Method POST -Uri http://localhost:18080/api/v1/admin/media/cleanup -WebSession $adminSession -Headers $headers -ContentType "application/json" -Body '{"dry_run":true}'
Invoke-RestMethod -Method POST -Uri http://localhost:18080/api/v1/admin/publish/releases/cleanup -WebSession $adminSession -Headers $headers -ContentType "application/json" -Body '{"dry_run":true}'
```

确认候选无误后再执行：

```powershell
Invoke-RestMethod -Method POST -Uri http://localhost:18080/api/v1/admin/media/cleanup -WebSession $adminSession -Headers $headers -ContentType "application/json" -Body '{"dry_run":false}'
Invoke-RestMethod -Method POST -Uri http://localhost:18080/api/v1/admin/publish/releases/cleanup -WebSession $adminSession -Headers $headers -ContentType "application/json" -Body '{"dry_run":false}'
```

安全规则：

- active release 永远不清理。
- release 清理只处理 inactive release，并同时移除该 release 的媒体引用记录。
- 被 post/page/release 引用的媒体不会被孤立媒体清理删除。
- 本地文件删除前会校验路径必须在配置的媒体目录或 release 根目录下。

对象存储/CDN 策略：

- 当前实现保留 `storage_driver/storage_bucket/storage_key/public_url` 字段，生产第一版仍使用 local volume。
- CDN 接入优先通过 `MEDIA_PUBLIC_BASE_URL` 指向 CDN 前缀，保持 Markdown URL 稳定。
- S3/R2/MinIO 后续应新增 storage adapter，不直接在 upload handler 中堆多分支。

## 4.4 Hugo 内容首次导入

首次把仓库中的 Hugo 文章纳入 Admin 数据库时，使用受控导入器，不直接执行 SQL：

```powershell
cd apps/api
go run ./cmd/import-hugo --email admin@zoking.local --password '<admin-password>' --publish=true
```

导入前置条件：

- `scripts/content/taxonomy-map.yaml` 显式维护中文 taxonomy 名称到稳定英文 slug 的映射。
- Hugo 文章目录名必须与 front matter `slug` 完全一致。
- 本地图片使用 Hugo `static` 根相对路径；已被发布器改写的媒体 URL 必须能通过 Admin API `public_url` 精确查询。
- 同一 PostgreSQL 和 Hugo 源目录只能运行一个 publish worker。开发模式使用 API 内嵌 worker 时，不得同时运行 `go run ./cmd/worker`。

导入器在首次写操作前完成 taxonomy、媒体和文章的远端发现；媒体按 SHA-256 复用，文章按 slug 幂等 upsert，并保留原始 `published_at`。`--publish=true` 会逐篇创建 job、等待终态后才继续，因此中途失败可以修复后重新执行。发布器生成的 taxonomy slug 和媒体 URL 也能被下一次 preflight 识别，不需要手工恢复源文件。

导入完成后至少核对：

- 数据库文章数、分类数、标签数和媒体数与导入清单一致。
- 每篇文章为 `published/public`，历史发布时间未被导入时间覆盖。
- 只有一个 active release。
- `/p/{slug}/`、`/categories/{slug}/`、`/tags/{slug}/`、媒体 URL 均返回 200。
- taxonomy URL 使用稳定 slug，页面标题仍显示中文名称。

## 5. 回滚

API 回滚：

- 保留上一镜像。
- 回滚前确认 migration 是否兼容。
- 若 migration 不可逆，先走热修或兼容层。

C 端回滚：

- 使用 `publish_releases` 历史版本。
- 切换 active release 指针。
- 刷新缓存。

数据库回滚：

- 生产优先恢复备份到新库验证。
- 不直接盲目执行 Down。
- 必须先停止 worker 并确认没有 `requested/queued/snapshotting/building/verifying/promoting` 任务；`20260714000200` 的 Down 会在存在活跃发布任务时明确拒绝。
- 精确回退命令为 `go run ./cmd/migrate down-to <目标版本>`，回退后执行 `go run ./cmd/migrate status`，再按备份恢复或兼容性方案继续。

## 6. 健康检查

API：

- `/healthz`：进程存活。
- `/readyz`：数据库、必要依赖可用。

后台：

- 静态资源可加载。
- 登录页可打开。

C 端：

- 首页 200。
- 文章页 200。
- RSS/Sitemap 200。

## 7. 备份

对象：

- PostgreSQL。
- 媒体文件。
- Hugo snapshots。
- releases。
- 环境变量模板。

频率：

- 日备份保留 7 天。
- 周备份保留 4 周。
- 月备份保留 3 个月。

每月至少一次恢复演练。

## 8. 部署记录模板

```markdown
## YYYY-MM-DD HH:mm +08:00 - DEPLOY

- 版本：
- 环境：
- 操作者：
- migration：
- 镜像/commit：
- 发布 release：
- 验证：
- 问题：
- 回滚点：
```
