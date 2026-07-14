# 认证与 RBAC 设计

本文定义后台认证、会话、权限模型和接口授权矩阵。

## 1. 认证边界

MVP 默认：

- B 端后台必须登录。
- C 端读者不要求登录。
- Public API 对评论、点赞、浏览量做限流和反垃圾。
- 后续如启用读者账号，需要新增 reader auth，不与后台用户混用权限。

## 2. Token 策略

后台登录使用：

- Access Token：短期有效，默认 30 分钟。
- Refresh Token：长期有效，默认 30 天，数据库只保存哈希。

刷新流程：

```text
login -> access token + refresh token
access token expired -> refresh -> new access token + rotated refresh token
logout -> revoke refresh token
```

安全规则：

- Refresh token 必须可按设备吊销。
- token 哈希入库，不保存明文。
- 登录失败限流。
- 密码使用 bcrypt 或 argon2id。
- 超级管理员初始化后必须提示修改初始密码。

## 3. 用户模型

用户状态：

| 状态 | 含义 |
|---|---|
| `active` | 正常 |
| `disabled` | 禁用，不可登录 |
| `pending` | 待激活，MVP 可不启用 |

默认角色：

| 角色 | code | 描述 |
|---|---|---|
| 超级管理员 | `super_admin` | 所有权限，不可删除最后一个 |
| 管理员 | `admin` | 站点运营、用户、设置 |
| 编辑 | `editor` | 审核、发布、管理内容 |
| 作者 | `author` | 创建和编辑自己的内容 |
| 观察者 | `viewer` | 只读后台 |

## 4. 权限码

文章：

- `post:read`
- `post:create`
- `post:update`
- `post:update:any`
- `post:delete`
- `post:submit_review`
- `post:review`
- `post:publish`
- `post:unpublish`

页面：

- `page:read`
- `page:create`
- `page:update`
- `page:delete`
- `page:publish`

分类标签：

- `taxonomy:read`
- `taxonomy:manage`

媒体：

- `media:read`
- `media:upload`
- `media:update`
- `media:delete`
- `media:delete:any`

评论：

- `comment:read`
- `comment:moderate`
- `comment:reply`
- `comment:delete`

发布：

- `publish:read`
- `publish:create`
- `publish:rollback`
- `publish:cancel`

系统：

- `setting:read`
- `setting:update`
- `user:read`
- `user:manage`
- `role:manage`
- `audit:read`
- `system:read`

## 5. 授权层次

中间件授权：

- 登录态校验。
- 权限码校验。
- 基础限流。

Service 授权：

- 是否只能编辑自己的文章。
- 文章当前状态是否允许操作。
- 是否允许删除被引用媒体。
- 是否允许发布未审核内容。
- 是否允许修改系统角色。

规则：

- 资源级权限必须在 service 中判断。
- 超级管理员可以绕过普通权限码，但不能绕过系统安全约束，如不能删除最后一个超级管理员。

## 6. 初始 Seed

必须 seed：

- 系统权限码。
- 默认角色。
- 角色权限关系。
- 第一个超级管理员。

超级管理员来源：

- 开发环境：`.env` 中设置初始账号密码。
- 生产环境：部署时通过一次性命令创建，初始密码不写入仓库。

## 7. 审计要求

必须审计：

- 登录成功/失败。
- 登出。
- 创建/禁用用户。
- 修改角色和权限。
- 发布、下线、回滚。
- 删除文章、页面、媒体、评论。
- 修改站点设置。

审计日志不得记录：

- 密码。
- token。
- secret。
- 完整 Cookie。
- 评论提交者完整 IP，生产可存 hash 或脱敏值。

## 8. 接口授权矩阵

| 模块 | 接口类型 | 最低权限 |
|---|---|---|
| 文章列表 | 读 | `post:read` |
| 创建文章 | 写 | `post:create` |
| 修改自己文章 | 写 | `post:update` + owner |
| 修改任意文章 | 写 | `post:update:any` |
| 发布文章 | 高危 | `post:publish` |
| 媒体上传 | 写 | `media:upload` |
| 媒体删除 | 高危 | `media:delete` 或 `media:delete:any` |
| 评论审核 | 高危 | `comment:moderate` |
| 站点设置 | 高危 | `setting:update` |
| 发布回滚 | 高危 | `publish:rollback` |
| 用户管理 | 高危 | `user:manage` |

## 9. 失败处理

- 401：未登录、token 过期、refresh 失败。
- 403：已登录但缺少路由权限或全局高危操作能力。
- 404：对象不存在，或对象不在当前用户的数据范围内；两种情况对客户端保持一致。
- 409：状态不允许，例如已发布文章重复发布。
- 高危操作失败必须写审计，标记 `result=failed`。
# 当前实现状态

`SEC-P5-001` 已实现数据库实时 RBAC：JWT 仅保存用户 ID 与邮箱，每个后台请求都会检查用户仍为 active 且未删除，并通过 `user_roles -> role_permissions -> permissions` 加载权限并集。

请求链：

```text
JWT 验证 -> active user 检查 -> 数据库角色/权限加载 -> 路由权限判断 -> handler
```

- 缺少或无效 token：401。
- 用户停用或软删除：401，旧 JWT 立即失效。
- 已登录但缺少权限：403，并写入 `denied` 审计记录。
- 权限数据库不可用：503，默认拒绝访问。
- `/api/v1/admin/auth/me` 返回 `roles` 与 `permissions`。

系统角色默认矩阵由 seed 精确对账：`super_admin` 全权限；`admin` 管理业务功能；`editor` 管理内容工作流；`author` 只能访问本人内容且不进入整站发布中心；`viewer` 为全局只读。`content:read_all` 和 `content:manage_all` 分别控制跨作者读取与管理，自定义角色必须显式获得所需数据范围能力。

文章、页面及其 publish jobs/releases/previews 已按 `author_id` 实施对象级隔离。创建内容时所有者强制取当前认证用户，详情和写操作在首次 SQL 查询时校验所有权，越权对象统一返回 404。完整规则见 `docs/security/content-object-access.md`。

## 用户与角色管理

当前已支持创建用户、启停账号、完整替换角色和读取系统角色权限。所有可能减少有效超级管理员数量的操作都在事务内获取同一个 PostgreSQL advisory lock，再统计：

```text
status = active AND deleted_at IS NULL AND role.code = super_admin
```

若目标操作会使有效超级管理员数量降为 0，则返回 409。该锁也覆盖并发请求，避免两个管理员被同时停用或同时降权。

当前边界：已开放自定义角色 CRUD、权限矩阵编辑和管理员密码重置；用户软删除及受审计的内容作者转交命令仍未实现。
