# P18 结构化系列验收计划

日期：2026-07-13

## 1. 数据库与 API

| 场景 | 预期 |
|---|---|
| 系列 slug 重复 | 409，原记录不变 |
| 同系列序号重复 | 409 `SERIES_ORDER_CONFLICT`，无部分写入 |
| 只给 series_id 或 series_order | 422 |
| series_order 为 0、负数或小数 | 422 |
| series_id 无效或不存在 | 422 或 404 稳定错误 |
| 删除被文章引用系列 | 409，文章关系不变 |
| 删除未引用系列 | 200，后续后台读取 404 |
| public 系列详情 | 仅公开已发布文章，严格按 order |
| RBAC | viewer 可读不可写，author 可读，manage 角色可写 |
| owner scope | 系列读取不泄露草稿正文，文章对象权限保持原规则 |

真实约束使用 `_test` PostgreSQL 独立 schema 验证，并检查测试后无残留 schema。禁止向 development/production 插入 P18 fixture。

## 2. 发布与预览

- `buildPostMarkdown` 输出结构化 series 对象并正确 YAML 转义。
- 无系列文章不产生 `series:`。
- 关系缺失、ID 不匹配、非正序号或非法 slug 被发布器拒绝。
- Preview、普通 publish、Worker、Retry、site publish 使用相同系列关系。
- 撤稿失败恢复仍写回原系列 front matter。
- 系列变更后新 release 的 content hash 改变，旧 active release 在 promote 前不被静默修改。

## 3. Admin

- 内容组织三个 Tab 均可用，切换不重置已有列表。
- 系列创建、编辑、删除确认和冲突提示完整。
- 文章选择系列后序号必填；清除系列后序号清空。
- 编辑已有文章、保存、Preview、Publish 后字段不丢失。
- 1280x720 与 390x844 无文档级横向溢出，Modal footer 可见，表格区域内部滚动。
- viewer/author 不出现无权限写按钮；API 403 仍可正确反馈。

## 4. C 端

- 无系列文章不显示模块。
- 三篇乱序源数据按 1/2/3 显示，当前项准确。
- 首篇只有“系列下一篇”，末篇只有“系列上一篇”，中间篇两者都有。
- 普通上一篇/下一篇仍存在且语义不变。
- 长中文/英文标题、深色模式、键盘焦点、390px 视口均通过。
- Hugo production build 无 warning/error，生成 HTML 不包含空 href 或重复 ID。

## 5. 回归命令

```powershell
Set-Location D:\zoking\zoking-blog\apps\api
go test ./... -count=1
go vet ./...

Set-Location D:\zoking\zoking-blog\apps\admin
npm run build

Set-Location D:\zoking\zoking-blog
.\.tools\hugo\hugo.exe --source apps/site --destination dist\qa\series-site --cleanDestinationDir
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
```

最终结果、Playwright 证据路径、运行态端口和任何平台限制在完成实现后回填工作日志。

## 6. 实际结果

2026-07-13 最终验收通过：

- `go test ./... -count=1`、`go vet ./...`、真实 PostgreSQL 系列契约测试全部通过。
- `_test` 库残留 `series_contract_*` schema 为 `0`，`series-test@example.com` 测试用户为 `0`；测试 migration 的 goose 版本表保持在隔离 schema。
- Admin `npm run build` 通过；Hugo minify 与仓库 preflight 均通过，生产构建保持 45 pages。
- `scripts/qa/series-p18-ui.mjs` 通过首/中/末系列导航、文章字段回填、系列管理 Modal、1280/390 视口和零运行时错误。
- 开发库仅执行 `20260713000100_create_series.sql` schema migration，未写入 P18 fixture；migration 版本、`series` 表和 `posts.series_id/series_order` 均已核验。
- 真实 API 验证：`/readyz` 200、无 Token `/api/v1/admin/series` 401、`/api/v1/public/series` 200；本地管理员登录后系列与文章列表均为 200。
- 真实 Admin 验证：`/taxonomy` 可切换“系列”并显示空状态，1280x720 无横向溢出，console/page error 为 0。

证据：

- `docs/process/evidence/series-p18-site-desktop-1280x800.png`
- `docs/process/evidence/series-p18-site-mobile-390x844.png`
- `docs/process/evidence/series-p18-admin-desktop-1280x720.png`
- `docs/process/evidence/series-p18-admin-modal-desktop-1280x720.png`
- `docs/process/evidence/series-p18-admin-mobile-390x844.png`
- `docs/process/evidence/series-p18-admin-live-1280x720.png`

当前运行态：API `http://localhost:18080`，Admin `http://localhost:5173`，C 端 `http://localhost:1313`，PostgreSQL `localhost:15432`。
