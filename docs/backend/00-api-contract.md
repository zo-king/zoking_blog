# API 契约设计

本文定义 Go Gin API 的外部契约，供 B 端 Admin、C 端动态增强、测试和 OpenAPI 文档使用。

## 1. API 分区

| 分区 | 前缀 | 调用方 | 鉴权 | 用途 |
|---|---|---|---|---|
| Admin API | `/api/v1/admin` | B 端后台 | 必须 | 内容、媒体、发布、权限、设置 |
| Public API | `/api/v1/public` | C 端静态站 JS | 可选 | 评论、浏览量、点赞、公开配置 |
| Internal API | `/api/v1/internal` | worker/运维 | 内部 token 或本机网络 | 发布回调、任务探活 |
| Health API | `/healthz`, `/readyz` | 负载均衡/监控 | 无 | 健康检查 |

## 2. 统一响应

成功：

```json
{
  "data": {},
  "request_id": "req_..."
}
```

分页：

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5
  },
  "request_id": "req_..."
}
```

失败：

```json
{
  "error": {
    "code": "POST_SLUG_CONFLICT",
    "message": "文章 slug 已存在",
    "details": {}
  },
  "request_id": "req_..."
}
```

规则：

- 业务错误必须有稳定 `code`。
- `message` 可本地化，但 `code` 不可随意变化。
- 生产环境不返回 SQL、堆栈、本地路径。
- 所有响应带 `request_id`。

## 3. 分页、排序、筛选

分页参数：

| 参数 | 默认 | 规则 |
|---|---|---|
| `page` | 1 | >= 1 |
| `page_size` | 20 | 1-100 |

排序参数：

```text
sort=-created_at,title
```

约定：

- `-field` 表示降序。
- 只允许白名单字段排序。
- 未指定时使用业务默认排序。

列表筛选示例：

```text
GET /api/v1/admin/posts?status=draft&keyword=golang&category_id=...&page=1&page_size=20
```

## 4. 后台会话与 CSRF

浏览器和管理脚本先调用登录接口。API 只通过 HttpOnly Cookie 建立管理会话，不在 JSON 响应中返回 JWT；登录响应中的 `csrf_token` 用于后续写请求：

```http
Cookie: zoking_admin_access=<http-only-session>
X-CSRF-Token: <login-response-csrf-token>
```

- Cookie 仅覆盖 `/api/v1/admin`，不会发送到公开 API、媒体或预览文件路径。
- `POST`、`PATCH`、`DELETE` 等 Cookie 认证写请求必须携带 `X-CSRF-Token`。
- 浏览器后台请求的 `Origin` 必须属于 `ADMIN_ALLOWED_ORIGINS`。
- 非浏览器内部测试仍可使用独立生成的 Bearer JWT，但登录接口不负责签发可读 Bearer Token。

内部任务：

```http
X-Internal-Token: <internal_token>
```

请求追踪：

```http
X-Request-ID: <optional-client-request-id>
```

若客户端未传，API 自动生成。

## 5. 错误码分层

通用错误码：

| code | HTTP | 含义 |
|---|---:|---|
| `BAD_REQUEST` | 400 | 请求格式错误 |
| `VALIDATION_FAILED` | 422 | 参数校验失败 |
| `UNAUTHORIZED` | 401 | 未登录或 token 无效 |
| `FORBIDDEN` | 403 | 权限不足 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 资源冲突 |
| `RATE_LIMITED` | 429 | 触发限流 |
| `INTERNAL_ERROR` | 500 | 内部错误 |

业务错误码：

| code | HTTP | 场景 |
|---|---:|---|
| `AUTH_INVALID_CREDENTIALS` | 401 | 登录失败 |
| `CSRF_FAILED` | 403 | Cookie 会话写请求的 CSRF 校验失败 |
| `ORIGIN_NOT_ALLOWED` | 403 | 管理写请求来自非可信 Origin |
| `SESSION_RESUME_FORBIDDEN` | 403 | 会话恢复缺少可信 Origin 或 Cookie 会话 |
| `USER_DISABLED` | 403 | 用户被禁用 |
| `POST_NOT_FOUND` | 404 | 文章不存在 |
| `POST_SLUG_CONFLICT` | 409 | slug 冲突 |
| `POST_INVALID_STATUS` | 409 | 状态流转非法 |
| `MEDIA_TOO_LARGE` | 413 | 文件过大 |
| `MEDIA_TYPE_NOT_ALLOWED` | 415 | 类型不允许 |
| `COMMENT_ALREADY_REVIEWED` | 409 | 评论已审核 |
| `PUBLISH_JOB_CONFLICT` | 409 | 发布任务冲突 |
| `PUBLISH_BUILD_FAILED` | 500 | Hugo 构建失败 |

## 6. Admin API 初版路由

### Auth

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| POST | `/auth/login` | 无 | 登录 |
| POST | `/auth/session` | 有效访问 Cookie + 可信 Origin | 恢复浏览器会话并轮换 CSRF Token |
| POST | `/auth/logout` | 登录 | 登出当前设备 |
| GET | `/auth/me` | 登录 | 当前用户信息与权限 |

`POST /auth/login` 和 `POST /auth/session` 都只返回 `csrf_token`、Cookie token 类型和有效期，不返回 JWT。`/auth/session` 要求非空且属于 `ADMIN_ALLOWED_ORIGINS` 的 `Origin`，仅接受 Cookie 认证，并通过新的 HttpOnly CSRF Cookie 使旧 CSRF Token 立即失效。当前 token 缺失、过期或无效统一返回 `401 UNAUTHORIZED`。

`/auth/me` 当前返回：

```json
{
  "id": "uuid",
  "email": "admin@zoking.local",
  "roles": ["super_admin"],
  "permissions": ["post:read", "post:create", "audit:read"]
}
```

后台业务路由由服务端实时检查数据库权限。前端按钮隐藏或禁用仅用于体验，不能替代 API 授权。

### Posts

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| GET | `/posts` | `post:read` | 文章列表 |
| POST | `/posts` | `post:create` | 创建文章 |
| GET | `/posts/:id` | `post:read` | 文章详情 |
| PATCH | `/posts/:id` | `post:update` | 更新文章 |
| DELETE | `/posts/:id` | `post:delete` | 删除文章 |
| POST | `/posts/:id/submit-review` | `post:update` | 提交审核 |
| POST | `/posts/:id/publish` | `post:publish` | 创建发布任务 |
| POST | `/posts/:id/preview` | `post:update` | 构建隔离的文章草稿预览，不创建 release |
| POST | `/posts/:id/unpublish` | `post:publish` | 下线 |
| GET | `/posts/:id/revisions` | `post:read` | 修订历史 |

### Taxonomy

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| GET | `/categories` | `taxonomy:read` | 分类列表 |
| POST | `/categories` | `taxonomy:manage` | 创建分类 |
| PATCH | `/categories/:id` | `taxonomy:manage` | 更新分类 |
| DELETE | `/categories/:id` | `taxonomy:manage` | 删除分类 |
| GET | `/tags` | `taxonomy:read` | 标签列表 |
| POST | `/tags` | `taxonomy:manage` | 创建标签 |
| PATCH | `/tags/:id` | `taxonomy:manage` | 更新标签 |
| DELETE | `/tags/:id` | `taxonomy:manage` | 删除标签 |

### Media

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| GET | `/media` | `media:read` | 媒体列表；支持 `checksum` 或 `public_url` 精确过滤 |
| POST | `/media` | `media:upload` | 上传 |
| POST | `/media/cleanup` | `media:delete` | 孤立媒体清理，默认 dry-run |
| GET | `/media/:id` | `media:read` | 媒体详情 |
| DELETE | `/media/:id` | `media:delete` | 删除 |

`POST /media/cleanup` 请求体：

```json
{
  "dry_run": true,
  "orphan_grace_seconds": 604800,
  "batch_size": 100
}
```

默认 `dry_run=true`，只有显式传入 `dry_run=false` 才会软删除媒体记录并删除本地文件。清理只处理没有 `media_usages` 引用、且超过宽限期的媒体。

`GET /media` 的精确过滤规则：

- `checksum` 必须是 64 位 SHA-256 十六进制字符串。
- `public_url` 按完整字符串精确匹配，用于识别已经被发布器改写过的媒体地址。
- 两个参数互斥；同时提供返回 `422 VALIDATION_FAILED`。
- 精确查询不会退化为模糊搜索，调用方必须检查结果唯一且字段与请求完全一致。

### Comments

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| GET | `/comments` | `comment:read` | 评论列表 |
| PATCH | `/comments/:id/moderation` | `comment:moderate` | 审核 |
| POST | `/comments/:id/reply` | `comment:reply` | 后台回复 |
| DELETE | `/comments/:id` | `comment:delete` | 删除 |

### Publish

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| GET | `/publish/jobs` | `publish:read` | 发布任务列表 |
| POST | `/publish/jobs` | `publish:create` | 创建站点发布任务 |
| GET | `/publish/jobs/:id` | `publish:read` | 发布任务详情 |
| POST | `/publish/jobs/:id/retry` | `publish:create` | 重试 |
| GET | `/publish/releases` | `publish:read` | 发布版本 |
| GET | `/publish/previews` | `publish:read` | 最近 50 条预览构建记录 |
| POST | `/publish/previews/cleanup` | `publish:cleanup` | 过期预览清理，默认 dry-run |
| POST | `/publish/releases/cleanup` | `publish:rollback` | 旧 release 清理，默认 dry-run |
| POST | `/publish/releases/:id/promote` | `publish:rollback` | 切换版本 |

`POST /publish/releases/cleanup` 请求体：

```json
{
  "dry_run": true,
  "keep_latest": 20,
  "keep_days": 30
}
```

清理永远跳过 active release，只会处理超过保留数量和保留天数的 inactive release。真实执行时会删除 release 目录，并移除对应 release 级媒体引用。

预览相关路由还包括：

- `POST /pages/:id/preview`：构建已保存页面草稿的隔离预览。
- `POST /settings/preview`：用请求体中的临时设置构建站点预览，但不持久化设置。
- `GET /preview-files/{preview_key}/...`：读取预览静态产物；生产环境只允许 `PUBLISH_PREVIEW_PUBLIC_BASE_URL` 配置的独立 Host。响应包含 `X-Robots-Tag: noindex, nofollow`、`Cache-Control: private, no-store`、`Referrer-Policy: no-referrer` 和严格 CSP。过期、非 ready、越界、符号链接及 `manifest.json` 请求均返回 404。

预览输出写入独立 `PUBLISH_PREVIEW_ROOT`，不得写正式 `apps/site/content`、不得创建 `publish_releases`、不得切换 active release 或 `current`。

预览清理请求：

```json
{
  "dry_run": true,
  "preview_batch_size": 100
}
```

只处理 `ready/failed` 且已过 `expires_at` 的记录。真实执行使用 `cleaning -> expired` 状态机；`building` 永远不进入普通 TTL 清理。

### Audit

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| GET | `/audit-logs` | 当前为已登录管理员，后续收紧为 `audit:read` | 最近审计记录，支持 `limit`、`resource_type`、`result`、`actor_id` 过滤 |

认证后的 POST/PATCH/DELETE 请求统一记录标准化路由、业务资源、HTTP 结果、Request ID、操作者邮箱快照、HMAC IP 和截断后的 User-Agent。审计层不读取原始请求体，因此不会保存密码、Bearer token、文章正文或上传文件。

### Users And Roles

| 方法 | 路由 | 权限 | 说明 |
|---|---|---|---|
| GET | `/users` | `user:read` | 用户列表及角色代码 |
| POST | `/users` | `user:manage` | 创建 active 用户并分配角色 |
| PATCH | `/users/:id/status` | `user:manage` | 启用或停用用户 |
| PATCH | `/users/:id/roles` | `user:manage` | 完整替换用户角色 |
| GET | `/roles` | `role:read` | 系统角色及其权限列表 |

创建用户密码要求 10–128 字符，进入 API 后立即 bcrypt。密码不返回、不写审计详情。停用最后一个有效 `super_admin` 或移除其超级管理员角色返回 `409 LAST_SUPER_ADMIN`。

## 7. Public API 初版路由

| 方法 | 路由 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/posts/:slug/comments` | 无 | 已通过评论 |
| POST | `/posts/:slug/comments` | 无或可选 | 提交评论 |
| POST | `/posts/:slug/view` | 无 | 记录浏览 |
| POST | `/posts/:slug/like` | 无 | 点赞 |
| GET | `/site/public-settings` | 无 | 公开站点动态配置 |

规则：

- Public API 必须限流。
- 评论内容必须清洗。
- 不暴露未发布内容。
- API 失败不影响 C 端文章阅读。

## 8. OpenAPI 产物规则

- OpenAPI 文件位置：`packages/contracts/openapi.yaml`。
- Admin 前端只依赖 OpenAPI 或生成的 TypeScript 类型。
- API 路由变更必须同步更新契约。
- PR 验收必须检查契约是否过期。
