# 后端工程规划

## 1. 技术定位

后端采用 Go Gin + GORM + PostgreSQL，先按模块化单体建设，避免过早微服务化。

C 端负责公开博客访问，保留 Hugo Theme Stack 的内容结构与展示语义。B 端负责内容、评论、媒体、发布、站点配置、统计与权限控制。

## 2. Gin 项目结构

```text
apps/api/
  cmd/server/main.go
  internal/
    bootstrap/
    config/
    router/
    middleware/
    handler/
    service/
    repository/
    model/
    dto/
    enum/
    pkg/
      errors/
      response/
      pagination/
      storage/
      auth/
      audit/
  migrations/
```

职责：

- `handler`：Gin handler，只处理 HTTP 输入输出。
- `service`：业务编排、事务边界、状态流转。
- `repository`：GORM 查询封装。
- `model`：GORM model。
- `dto`：request / response DTO。
- `middleware`：Auth、RBAC、CORS、RateLimit、Recover、TraceID。
- `pkg/storage`：本地/S3/OSS 存储适配。
- `pkg/auth`：JWT、密码、token。

## 3. 模块划分

- Auth：登录、刷新 token、登出、密码修改、会话吊销。
- Users：后台用户、作者资料、账号状态。
- RBAC：角色、权限、用户角色绑定、接口权限校验。
- Posts：文章、草稿、发布状态、置顶、摘要、封面、SEO 信息。
- Taxonomy：分类、标签、文章与分类/标签关系。
- Comments：评论、嵌套回复、审核、反垃圾、IP/UserAgent 记录。
- Media：图片/附件上传、元数据、引用关系、存储驱动。
- Pages：独立页面，如 About、Links、Archive。
- Menus/Settings：导航、站点配置、主题配置。
- PublishJobs：发布、定时发布、重新生成索引、缓存刷新。
- Stats：文章浏览、点赞、评论数、日聚合统计。
- AuditLogs：后台敏感操作审计。

## 4. API 分层

Handler 层：

- 参数绑定。
- 参数校验。
- 调用 service。
- 返回统一响应。
- 不直接写 GORM 查询。
- 不承载业务决策。

Service 层：

- 业务规则。
- 状态流转。
- 权限上下文。
- 事务边界。
- 复杂写入必须从 service 发起事务。

Repository 层：

- 封装 GORM 查询。
- 只返回 model/domain 与明确错误。
- 不处理 HTTP 状态码。

## 5. 统一响应

成功：

```json
{
  "data": {},
  "request_id": "..."
}
```

分页：

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100
  },
  "request_id": "..."
}
```

失败：

```json
{
  "error": {
    "code": "POST_NOT_FOUND",
    "message": "文章不存在"
  },
  "request_id": "..."
}
```

## 6. REST API 规划

公开 C 端：

- `GET /api/v1/public/posts`
- `GET /api/v1/public/posts/:slug`
- `GET /api/v1/public/categories`
- `GET /api/v1/public/tags`
- `GET /api/v1/public/pages/:slug`
- `POST /api/v1/public/posts/:id/comments`

后台 B 端：

- `POST /api/v1/admin/auth/login`
- `POST /api/v1/admin/auth/refresh`
- `POST /api/v1/admin/auth/logout`
- `GET /api/v1/admin/posts`
- `POST /api/v1/admin/posts`
- `PATCH /api/v1/admin/posts/:id`
- `POST /api/v1/admin/posts/:id/publish`
- `POST /api/v1/admin/posts/:id/unpublish`
- `GET /api/v1/admin/comments`
- `PATCH /api/v1/admin/comments/:id/moderation`
- `POST /api/v1/admin/media`
- `GET /api/v1/admin/stats/overview`

## 7. GORM 使用规范

- Model 使用 UUID，数据库默认 `gen_random_uuid()`。
- 所有主表包含 `created_at`、`updated_at`、`deleted_at`。
- 后台可见删除默认软删除。
- 不可恢复清理任务单独实现硬删除。
- 查询列表默认排除软删除、草稿、未发布内容。
- 禁止在 handler 里直接使用 `db`。
- 禁止拼接 SQL。
- 复杂查询用 GORM 参数绑定或 `db.Raw(sql, args...)`。
- 预加载关系必须显式控制字段，避免 N+1。
- `Save` 慎用，更新使用 `Updates` 或明确字段白名单。
- 所有写入操作设置 `created_by` / `updated_by`。
- 使用 `db.WithContext(ctx)`。

## 8. 事务策略

必须开启事务：

- 创建文章并绑定分类/标签。
- 发布文章并写发布记录、统计初始值、审计日志。
- 删除文章并解除媒体引用、评论状态更新。
- 创建用户并分配角色。
- 评论审核状态变更并更新文章评论计数。

约定：

- Service 提供统一 `WithTx(ctx, fn)`。
- Repository 接受 `*gorm.DB`。
- 事务中不执行外部网络 IO。
- 媒体上传先临时落库，再异步确认。

## 9. 认证授权

认证：

- 后台使用 Access Token + Refresh Token。
- Access Token 建议 15-30 分钟有效。
- Refresh Token 存库，只保存哈希值，可按设备吊销。
- 密码使用 bcrypt 或 argon2id。
- 登录失败限流，连续失败可短暂锁定账号。

授权：

- 使用 RBAC：用户 -> 角色 -> 权限。
- 权限粒度示例：
  - `post:read`
  - `post:create`
  - `post:update`
  - `post:publish`
  - `comment:moderate`
  - `media:upload`
  - `system:setting`
- 中间件先校验登录，再校验权限。
- 资源级权限在 service 校验。

## 10. 错误码

通用：

- `BAD_REQUEST`
- `VALIDATION_FAILED`
- `UNAUTHORIZED`
- `FORBIDDEN`
- `NOT_FOUND`
- `CONFLICT`
- `RATE_LIMITED`
- `INTERNAL_ERROR`

业务：

- `AUTH_INVALID_CREDENTIALS`
- `AUTH_TOKEN_EXPIRED`
- `USER_DISABLED`
- `POST_NOT_FOUND`
- `POST_SLUG_CONFLICT`
- `POST_INVALID_STATUS`
- `COMMENT_NOT_FOUND`
- `COMMENT_ALREADY_REVIEWED`
- `MEDIA_TOO_LARGE`
- `MEDIA_TYPE_NOT_ALLOWED`
- `PUBLISH_JOB_CONFLICT`
