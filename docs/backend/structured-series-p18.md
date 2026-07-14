# 结构化系列数据与发布契约

日期：2026-07-13
任务：`SERIES-P18-001`

## 1. 目标与边界

系列是独立内容实体，不使用标签、JSON 设置或 Markdown 文本模拟。首版坚持单篇文章最多属于一个系列，系列内位置由正整数显式表达，保证数据库、Admin、API 与 Hugo 构建得到同一顺序。

首版不实现拖拽排序、多系列归属、自动挤位或读者账号进度。普通分类/标签和全站上一篇/下一篇保持原语义。

## 2. 数据模型

`series`：

- `id uuid` 主键，沿用全站 opaque ID。
- `name text not null`、`slug text not null`、`description text not null default ''`。
- `cover_media_id uuid null`，删除媒体时 `set null`。
- `sort_order integer not null default 0 check (sort_order >= 0)`。
- `enabled boolean not null default true`。
- `created_at/updated_at/deleted_at` 与其他内容组织实体一致。
- 未软删记录的 `slug` 使用部分唯一索引。

`posts` 增加：

- `series_id uuid null references series(id) on delete restrict`。
- `series_order integer null`。
- CHECK：两个字段必须同时为空，或同时非空且 `series_order > 0`。
- `series_id` 普通索引用于详情与删除保护查询。
- 未软删文章的 `(series_id, series_order)` 部分唯一索引用于最终并发兜底。

删除系列时 API 先检查引用并返回业务冲突；数据库 `RESTRICT` 处理检查与删除之间的竞争。软删文章不占用系列序号，但撤稿发布状态机完成前文章仍存在，不能提前释放引用。

## 3. API 契约

公开读取：

```http
GET /api/v1/public/series
GET /api/v1/public/series/:slug
```

列表只返回启用系列并稳定按 `sort_order,name,id` 排序。详情只暴露公开系列，文章限定 `status=published AND visibility=public AND deleted_at IS NULL`，按 `series_order,id` 排序。

后台管理：

```http
GET    /api/v1/admin/series
POST   /api/v1/admin/series
GET    /api/v1/admin/series/:id
PATCH  /api/v1/admin/series/:id
DELETE /api/v1/admin/series/:id
```

系列读取/写入复用 `taxonomy:read` 与 `taxonomy:manage`。Admin 列表包含停用系列和文章引用计数；删除被引用系列返回 `409`，不隐式解绑文章。

文章创建/更新增加：

```json
{
  "series_id": "uuid-or-empty",
  "series_order": 2
}
```

清除系列时 `series_id` 为空且 `series_order` 为 `null`。只提交一个字段、非正序号、无效 UUID 或不存在系列均为 `422/404` 的稳定客户端错误。唯一索引冲突映射为 `409 SERIES_ORDER_CONFLICT`，不能把 PostgreSQL 错误文本直接暴露给调用方。

## 4. 发布一致性

所有文章读取路径必须预载 `Series`，需要封面时预载 `Series.CoverMedia`：

- 文章公开/后台详情与列表。
- Preview 加载。
- Worker 正式发布与撤稿失败恢复。
- Retry、站点发布和 Worker 二次质量检查。

Hugo front matter 使用结构化对象：

```yaml
series:
  slug: "go-engineering"
  name: "Go 工程实践"
  order: 3
```

只有 `series_id`、`series_order`、已加载 Series 三者一致时才允许写快照。发布器再次检查正整数、关系 ID、name 与安全 slug，避免漏预载生成残缺静态页面。

## 5. 并发与迁移

- 迁移通过 goose 管理，Up/Down 对称；不在应用启动时 AutoMigrate 生产 schema。
- 唯一序号由 PostgreSQL 保证，两个并发请求不能占用同一系列位置。
- 首版冲突要求编辑者选择新序号，不自动批量移动后续文章，避免长事务和静默重排。
- migration 先建 `series`，再给 `posts` 增列/约束/索引；Down 先删文章约束与列，再删表。
- 测试只连接名称以 `_test` 结尾的数据库，并为每个用例创建独立 schema。

## 6. 可观测与审计

系列写接口继续经过全局审计 middleware，资源类型为 `series`。业务冲突保留 request ID；日志不得记录数据库 DSN、Token 或完整 Markdown 正文。正式发布 job 的快照 hash 自然覆盖新增 front matter，系列变更后的重新发布可被 release manifest 区分。
