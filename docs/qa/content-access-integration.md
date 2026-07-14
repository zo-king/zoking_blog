# 内容对象级访问控制验收

## 1. 目标

本验收对应 `SEC-P15-001`，证明后台路由权限与对象所有权范围同时生效。授权真相位于 API 和 PostgreSQL；Admin 的菜单、路由和按钮裁剪只负责避免向用户展示不可执行的操作。

核心不变量：

- 新建文章和页面的 `author_id` 只能来自当前认证用户，请求体不能伪造所有者。
- owner-scoped 用户的列表、分页总数、详情、更新、删除、预览和发布都在首次 SQL 查询中限定 owner。
- 他人对象统一返回 404，拒绝路径不能改内容、创建任务或生成预览。
- 未拥有 `post:publish` / `page:publish` 时，不能通过 create/update 进入、退出或修改 `published` 对象。
- 发布任务、版本和预览按关联文章/页面 owner 收敛；站点级记录不向 owner-scoped 用户开放。
- retry/cancel 先检查任务关联内容范围；release promote 还必须具备全局内容管理范围。
- `viewer` 全局只读，`author` 只管理本人草稿；前端深链默认拒绝且不能提前请求无权对象。

## 2. PostgreSQL 集成测试

测试文件：

- `apps/api/internal/httpapi/content_access_postgres_integration_test.go`
- `apps/api/internal/httpapi/content_access_test.go`
- `apps/api/cmd/seed/main_test.go`

本地运行：

```powershell
$env:DATABASE_URL = "postgres://zoking:zoking_dev_password@localhost:15432/zoking_blog_test?sslmode=disable"
Set-Location apps/api
go test ./cmd/seed ./internal/httpapi -count=1
go test ./internal/httpapi -run '^TestContentAccessPostgresOwnerIsolation$' -count=1 -v
```

2026-07-13 最终结果：

- owner A/B 文章和页面列表、分页总数、NULL owner 隐藏及全局读取范围通过。
- 文章/页面创建强制当前认证 owner，通过请求体提交他人 `author_id` 无效。
- 他人文章/页面的 GET、PATCH、DELETE、preview、publish 均返回 404，内容、job 和 preview 无副作用。
- 文章与页面的 published create、draft-to-published transition、已发布对象普通更新均在缺少发布权限时返回 403，数据库状态不变。
- 本人失败任务 retry 后进入 requested、重试计数加一，随后 cancel 成功；他人任务 retry/cancel 均为 404 且状态不变。
- owner-scoped 用户即使具备 `publish:rollback` 路由权限，也因缺少全局内容管理范围而无法 promote release，返回 403。
- 关联本人内容的 job/release/preview 可见；他人记录和站点级记录隐藏；`content:read_all` 保持全局读取兼容。
- 最终集成测试连续 3 轮通过，每轮使用独立 schema，结束后临时 schema 数量为 0。

## 3. Admin Playwright 黑盒

入口：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\content-object-access-ui.ps1
```

脚本只接受 loopback HTTP 地址，并只允许本地 `zoking_blog` 或名称以 `_test` 结尾的数据库。每轮创建随机前缀的 author、viewer、本人/他人文章和他人页面；无论断言成功或失败，`finally` 都会删除 fixture 内容、账号、关联发布记录和 fixture actor 审计记录。Playwright 不在项目依赖中时，可通过 `-PlaywrightPackagePath` 指向已安装的 `playwright` package 目录。

最终成功 run：`ff53c1bde3e6`。

| 角色 | 运行态验收 |
|---|---|
| `super_admin` | 可见本人和他人文章/页面；文章、页面、分类、媒体、评论、设置及发布中心的写动作保持可用 |
| `author` | 文章列表只有本人草稿；可新建/编辑/保存/预览和上传媒体、插入 Markdown；不见归档、发布、taxonomy 管理、媒体维护/删除、评论审核和设置写动作；设置表单只读；页面新建/编辑及发布中心深链默认拒绝 |
| `viewer` | 全局可见文章、页面和发布列表；taxonomy、媒体、评论、设置均为只读，媒体仅保留复制地址；不见内容编辑、发布任务管理、release/preview cleanup；文章/页面编辑深链不渲染表单 |

桌面和移动端均校验浏览器 console/page error 为空，页面宽度不超过 viewport。证据：

- `docs/process/evidence/sec-p15-super-admin-posts-desktop-1280x720.png`
- `docs/process/evidence/sec-p15-author-editor-desktop-1280x720.png`
- `docs/process/evidence/sec-p15-author-editor-mobile-390x844.png`
- `docs/process/evidence/sec-p15-viewer-posts-desktop-1280x720.png`

## 4. 回归证据

- `go test ./cmd/seed ./internal/httpapi -count=1`：通过。
- `scripts/qa/whitebox.ps1`：禁用缓存的全量 Go 测试通过，`go vet ./...` 通过，总覆盖率 `35.8%`。
- Admin `npm run build`：TypeScript 与 Vite production build 通过。
- `scripts/qa/http-blackbox.ps1`：默认只读黑盒 `21/21` 通过。
- `scripts/qa/content-object-access-ui.ps1`：最终 run `ff53c1bde3e6`，四个浏览器场景通过，fixture 清理为 `0|0|0`。
- Linux `golang:1.25-bookworm`：`internal/httpapi`、`internal/publisher`、`internal/maintenance` 的 `go test -race -count=1` 全部通过，无 data race。
- seed 双进程首次初始化：`active_admins=1`、`super_admin_links=1`、`duplicate_role_permissions=0`。
- 完整 preflight/E2E：run `e4d12c45-efbc-4e82-b66f-8ed85ad7776c` 通过，覆盖三类预览、文章/页面/设置发布、评论、rollback 与 manifest 清理；隔离 API、本轮 E2E 数据和运行目录已清理。

## 5. 清理与边界

最终只读核查：

- development：P15 UI 用户、文章、页面均为 0；额外 schema 为 0；`posts.author_id IS NULL=0`，`pages.author_id IS NULL=0`。
- runtime：`18081` 无测试 API；`storage/qa/preflight-runtime` 不存在；`storage/qa/e2e-runs` 为空。
- test：临时 integration schema 为 0，文章和页面为 0。
- `zoking_blog_test` 保留早期 settings-only 测试 baseline 的 5 个 publish job/release 和 1 个 active 标记，其输出目录已删除。它不是 P15 fixture，也不被当前开发 API 使用；本任务未擅自执行破坏性清库。
- 未连接生产数据库，生产发布前仍必须按 `docs/security/content-object-access.md` 盘点 NULL owner，并依据真实作者关系人工回填。

## 6. 结论

`SEC-P15-001` 的对象范围、发布动作、Admin 权限感知和测试清理要求均已获得直接证据。生产部署仍需执行最新 migration/seed、NULL owner 盘点和公网环境冒烟，这些属于部署门禁，不是本地实现缺口。
