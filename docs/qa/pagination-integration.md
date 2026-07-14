# Admin 分页 PostgreSQL 集成测试

## 1. 目标

分页 handler 的参数单测不能证明 PostgreSQL 的 Count、Join、ILIKE、UUID 类型转换、稳定排序和软删除行为。QA-P14 增加真实 PostgreSQL 集成测试，直接调用 Gin handler，并在每个用例的独立 schema 中创建模型表和数据。

覆盖文件：

- `postgres_integration_test.go`：只允许 `DATABASE_URL` 指向 `_test` PostgreSQL，为每个用例创建并清理独立 schema。
- `pagination_posts_integration_test.go`：文章 taxonomy、组合过滤、预载、分页和稳定排序。
- `pagination_media_audit_integration_test.go`：媒体精确查询兼容、usage count、媒体分页，以及审计过滤和 legacy `limit`。
- `pagination_validation_integration_test.go`：UUID 与媒体精确参数的错误边界。

## 2. 核心不变量

- `category_id`、`tag_id`、评论 `post_id` 和审计 `actor_id` 必须先解析为 UUID，再绑定 PostgreSQL 查询；无效输入返回 422，不得退化为 500。
- 文章 taxonomy 过滤后的 `total` 必须是文章数量，不能被 Join 或关联预载放大。
- 相同业务排序键必须追加唯一 ID，跨页读取结果稳定且不重不漏。
- 越界页返回空数组，但保留真实 `total`、`total_pages`、请求页码和 page size。
- 媒体 `checksum` / `public_url` 精确查询继续返回非分页数组 envelope；参数一旦出现就进入精确分支，空值返回 422。
- checksum 允许大小写归一化；public URL 按解码后的完整字符串比较，不裁剪首尾空白后再匹配。
- 审计旧 `limit` 参数继续映射为 `page_size`，并可与 q/result/resource_type/actor_id 组合使用。

## 3. 本地运行

```powershell
$env:APP_ENV = "test"
$env:DATABASE_URL = "postgres://zoking:zoking_dev_password@localhost:15432/zoking_blog_test?sslmode=disable"
Set-Location apps/api
go test -count=1 ./internal/httpapi
```

重复和竞态验证：

```powershell
go test -count=3 ./internal/httpapi

docker run --rm --network docker_default `
  -v "D:\zoking\zoking-blog:/workspace" `
  -w /workspace/apps/api `
  -e CGO_ENABLED=1 `
  -e APP_ENV=test `
  -e "DATABASE_URL=postgres://zoking:zoking_dev_password@zoking-blog-postgres:5432/zoking_blog_test?sslmode=disable" `
  golang:1.25-bookworm `
  go test -race -count=1 ./internal/httpapi
```

没有 `DATABASE_URL` 或数据库名不以 `_test` 结尾时，集成用例会跳过。禁止将 development 或 production 数据库传给这些测试。

## 4. QA-P14 结果

2026-07-13 验收：HTTP API PostgreSQL 集成测试连续 3 轮通过；Linux PostgreSQL race 通过；白盒总覆盖率 `30.4%`，`go vet` 通过；完整 preflight/E2E run `64aa9101-f17e-4462-8820-66335c872020` 通过并自动清理；重建 API 后的管理员 HTTP 边界验证和默认黑盒 `21/21` 通过。
