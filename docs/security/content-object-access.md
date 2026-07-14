# 内容对象级访问控制

## 1. 目标

后台路由权限只回答“用户能否执行某类动作”，对象级访问控制继续回答“用户能对哪些文章、页面和发布记录执行该动作”。两层检查必须同时通过，前端隐藏菜单不构成安全边界。

## 2. 数据范围能力

| 能力 | 含义 | 默认系统角色 |
|---|---|---|
| `content:read_all` | 可读取所有文章、页面及其发布任务、版本和预览 | `super_admin`、`admin`、`editor`、`viewer` |
| `content:manage_all` | 可修改、删除、预览、发布所有文章和页面，并管理其发布任务 | `super_admin`、`admin`、`editor` |

`author` 不具备全局数据范围能力，只能访问 `author_id` 等于当前用户 ID 的内容，并且默认不具备 `publish:read`，后台不向作者展示整站发布中心。自定义角色若需要跨作者访问或读取本人内容关联的发布记录，必须显式授予对应能力，不能通过前端或请求参数扩大范围。

为保证旧数据库在重新执行 seed 前不会使系统角色意外降级，服务端同时识别既有系统角色作为兼容路径；部署仍必须按 runbook 执行幂等 seed，使权限表成为最终事实来源。系统角色 seed 使用精确对账：先清除该系统角色的旧权限关联，再按声明矩阵重建，权限收缩不会残留历史授权。

## 3. 所有权规则

- 新建文章或页面时，`author_id` 强制取 JWT 身份对应的当前用户 ID。
- 请求体不接受 `author_id`，客户端不能代替服务端指定所有者。
- `author_id` 当前不可通过文章或页面更新接口修改；后续如需转交作者，应设计独立高权限命令和审计事件。
- 没有 `post:publish` / `page:publish` 时，不能通过普通 create/update 请求进入或退出 `published` 状态。
- 没有发布权限时，已发布对象禁止普通更新；在引入独立 revision/draft 模型前，不能让作者直接改写公开 API 当前读取的已发布正文。
- owner-scoped 用户的列表查询必须在 SQL 层添加 `author_id = current_user_id`，不能先读取全量数据再在应用内过滤。
- 详情、更新、删除、预览和发布必须在首次数据库查询时同时匹配对象 ID 与所有者。

## 4. 发布记录范围

`publish_jobs`、`publish_releases`、`publish_previews` 通过 `post_id` 或 `page_id` 关联内容所有者：

- 全局读取者可查看全部记录。
- owner-scoped 用户只能查看关联到本人文章或页面的记录。
- 仅有 `requested_by` 但没有文章或页面关联的站点级记录不向 owner-scoped 用户开放。
- 重试和取消发布任务在调用发布服务前先校验关联内容所有权。
- release promote 会切换整站当前版本，必须具有全局内容管理能力；单篇内容所有权不足以授权该操作。

## 5. 响应语义

- 缺失或非法身份上下文：`401 UNAUTHORIZED`，默认拒绝。
- 缺少路由级权限：`403 FORBIDDEN`。
- 未登记的后台路由默认映射到拒绝权限，禁止沿用低权限 `system:read` 兜底。
- 对象不存在或不在当前用户数据范围：统一返回资源对应的 `404`，避免通过 ID 枚举确认其他作者对象是否存在。
- 列表接口返回过滤后的 `data`、`total` 和 `total_pages`，分页统计必须基于同一所有权条件。

## 6. 验证要求

PostgreSQL 集成测试至少覆盖：

- author 列表只能看到本人文章和页面，分页总数同步收敛。
- viewer/editor/admin 的全局读取兼容。
- 新建文章和页面强制写入当前用户 `author_id`。
- author 对他人内容的详情、更新、删除、预览和发布均为 404，且数据库和发布任务无副作用。
- 发布任务、版本、预览列表不泄露其他作者或站点级记录。
- owner-scoped 用户不能 promote release；具备全局管理能力的用户保持原有能力。
- author 的系统角色权限精确对账后不再包含 `publish:read`。
- 非法用户 ID、非法内部表标识和缺失权限上下文均 fail closed。

部署前必须执行只读盘点：

```sql
select count(*) from posts where author_id is null;
select count(*) from pages where author_id is null;
```

历史 NULL owner 内容只对全局角色可见。必须依据真实作者关系逐条回填，禁止默认批量归给任意作者。
