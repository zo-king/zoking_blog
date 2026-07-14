# 数据库工程规划

## 1. 迁移策略

- 使用 SQL-first 迁移，推荐 goose 或 golang-migrate。
- 迁移文件命名：`YYYYMMDDHHMMSS_create_posts.sql`。
- 每个迁移包含 Up/Down。
- 本地和 CI 必须能从空库完整迁移。
- 生产只向前迁移；Down 主要用于开发回滚验证。
- GORM AutoMigrate 仅允许本地实验，不作为生产迁移手段。
- 初始化扩展：`pgcrypto`，可选 `citext`、`pg_trgm`。

## 2. 通用字段

主业务表建议包含：

- `id uuid primary key default gen_random_uuid()`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`
- `deleted_at timestamptz null`
- `created_by uuid null`
- `updated_by uuid null`
- `deleted_by uuid null`

软删除：

- GORM 使用 `gorm.DeletedAt`。
- 唯一索引如 slug/email 需考虑软删除：`where deleted_at is null`。

审计：

- `audit_logs` 记录后台敏感操作。
- 不在审计日志中写密码、token、密钥、完整请求头。

## 3. 表清单初版

### users

后台用户与作者账号。

字段要点：

- `email`
- `username`
- `password_hash`
- `display_name`
- `avatar_url`
- `bio`
- `status`
- `last_login_at`

索引：

- unique `email where deleted_at is null`
- unique `username where deleted_at is null`
- index `status`

### roles

角色。

字段要点：

- `code`
- `name`
- `description`
- `is_system`

索引：

- unique `code`

### permissions

权限点。

字段要点：

- `code`
- `name`
- `resource`
- `action`

索引：

- unique `code`

### user_roles

用户角色绑定。

索引：

- unique `(user_id, role_id)`
- index `role_id`

### role_permissions

角色权限绑定。

索引：

- unique `(role_id, permission_id)`

### refresh_tokens

刷新令牌。

字段要点：

- `user_id`
- `token_hash`
- `device_id`
- `user_agent`
- `ip`
- `expires_at`
- `revoked_at`

索引：

- unique `token_hash`
- index `(user_id, expires_at)`
- index `revoked_at`

### posts

文章主表。

字段要点：

- `title`
- `slug`
- `summary`
- `content_md`
- `content_html`
- `cover_media_id`
- `status`
- `visibility`
- `is_pinned`
- `allow_comment`
- `published_at`
- `author_id`
- `seo_title`
- `seo_description`

状态：

- `draft`
- `scheduled`
- `published`
- `archived`

索引：

- unique `slug where deleted_at is null`
- index `(status, published_at desc)`
- index `(author_id, created_at desc)`
- index `is_pinned`
- 可选全文索引：`to_tsvector('simple', title || ' ' || content_md)`

### categories

分类。

字段要点：

- `name`
- `slug`
- `description`
- `parent_id`
- `sort_order`

索引：

- unique `slug where deleted_at is null`
- index `parent_id`

### tags

标签。

字段要点：

- `name`
- `slug`
- `description`

索引：

- unique `slug where deleted_at is null`

### post_categories

文章分类关系。

索引：

- unique `(post_id, category_id)`
- index `category_id`

### post_tags

文章标签关系。

索引：

- unique `(post_id, tag_id)`
- index `tag_id`

### comments

评论。

字段要点：

- `post_id`
- `parent_id`
- `author_name`
- `author_email_hash`
- `author_website`
- `content`
- `status`
- `ip_hash`
- `user_agent`
- `reviewed_by`
- `reviewed_at`

状态：

- `pending`
- `approved`
- `rejected`
- `spam`

索引：

- index `(post_id, status, created_at desc)`
- index `parent_id`
- index `(status, created_at desc)`

### media_assets

媒体资源。

字段要点：

- `filename`
- `original_name`
- `mime_type`
- `size_bytes`
- `width`
- `height`
- `storage_driver`
- `storage_key`
- `public_url`
- `checksum`
- `uploaded_by`

索引：

- unique `storage_key`
- index `mime_type`
- index `(uploaded_by, created_at desc)`
- unique `checksum` 可选，用于去重

### media_usages

媒体引用关系。

字段要点：

- `media_id`
- `resource_type`
- `resource_id`
- `usage_type`

索引：

- unique `(media_id, resource_type, resource_id, usage_type)`
- index `(resource_type, resource_id)`

### pages

独立页面。

字段要点：

- `title`
- `slug`
- `content_md`
- `content_html`
- `status`
- `published_at`

索引：

- unique `slug where deleted_at is null`
- index `(status, published_at desc)`

### site_settings

站点配置。

字段要点：

- `key`
- `value_json`
- `description`
- `is_public`

索引：

- unique `key`

### menus / menu_items

菜单与菜单项。

索引：

- `menus.code` unique
- `menu_items(menu_id, sort_order)`
- `menu_items.parent_id`

### publish_jobs

发布任务。

字段要点：

- `post_id`
- `job_type`
- `status`
- `run_at`
- `locked_at`
- `finished_at`
- `error_message`
- `retry_count`

索引：

- index `(status, run_at)`
- index `post_id`

### post_stats / post_daily_stats

文章累计统计与每日统计。

索引：

- `post_stats.post_id` unique
- `post_daily_stats(post_id, date)` unique
- `post_daily_stats.date`

### audit_logs

后台审计。

字段要点：

- `actor_id`
- `action`
- `resource_type`
- `resource_id`
- `before_json`
- `after_json`
- `ip`
- `user_agent`

索引：

- index `(actor_id, created_at desc)`
- index `(resource_type, resource_id)`
- index `(action, created_at desc)`

## 4. 安全注意点

- 所有输入做 DTO 校验。
- 评论内容需做 HTML 清洗或只允许 Markdown 安全子集。
- CORS 只允许后台域名与前台域名。
- 媒体上传限制大小、MIME、扩展名，并校验真实文件头。
- 后台接口全量限流。
- 登录、刷新、上传接口单独更严格限流。
- 生产错误响应不暴露 SQL、堆栈、内部路径。
- PostgreSQL 账号最小权限，应用账号不使用 superuser。
- 定时任务使用 `FOR UPDATE SKIP LOCKED`，避免多实例重复执行。
