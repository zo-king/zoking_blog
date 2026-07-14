# P17 内容质量与发布门禁验收

日期：2026-07-13

## 1. 自动化入口

Go 全量：

```powershell
Set-Location apps/api
go test ./... -count=1
go vet ./...
```

真实 PostgreSQL：

```powershell
$env:DATABASE_URL='postgres://zoking:zoking_dev_password@localhost:15432/zoking_blog_test?sslmode=disable'
go test ./internal/httpapi ./internal/publisher -count=1
```

Admin：

```powershell
Set-Location apps/admin
npm run build
```

无数据库写入 Playwright：

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
$env:CONTENT_QUALITY_ADMIN_BASE='http://localhost:5173'
node .\scripts\qa\content-quality-ui.mjs
```

## 2. 覆盖矩阵

| 范围 | 断言 | 结果 |
|---|---|---|
| Markdown/HTML | 空正文、纯注释、危险协议、危险 HTML/属性被阻断 | PASS |
| 解析准确性 | fenced code 中危险示例不误报 | PASS |
| 建议项 | H1、图片 alt、SEO、正文长度、封面/分类/标签、菜单图标 | PASS |
| API | 新草稿默认值、已有对象 owner scope、无 token 401、viewer 403 | PASS |
| 发布事务 | 422 后 status/visibility/published_at/job/media usage 全不变 | PASS |
| 绕过 | 高权限 create/PATCH 也不能直接进入 published | PASS |
| 并发 | 同一文章双 publish 仅一个 202，另一个 409 | PASS |
| 编辑锁 | active job 期间 PATCH 409 且标题不变 | PASS |
| 纵深门禁 | Worker、Retry、Settings site publish 均拒绝无效内容 | PASS |
| Preview | 文章预览加载 CoverMedia | PASS |
| Admin | 检查未保存表单、变更使报告失效、阻断 Drawer、发布请求顺序 | PASS |
| 响应式 | 1280x720 右侧 380px；390x844 全宽；无横向溢出 | PASS |

## 3. PostgreSQL 隔离

测试只允许 `DATABASE_URL` 指向名称以 `_test` 结尾的 PostgreSQL。每个测试创建 UUID 后缀 schema，结束后 cascade 删除。P17 未向 development 或 production 数据库写测试 fixture。

并发测试在真实行锁语义下执行：同一内容行 `FOR UPDATE`，第一个事务创建 requested job；第二个事务获得锁后看到 active job 并返回冲突。该测试不依赖 SQLite 行为。

## 4. Playwright 说明

`content-quality-ui.mjs` 在浏览器层拦截 API，注入最小权限编辑身份和确定性报告，因此不会登录或写入 development 数据库。脚本验证：

- 手工检查在保存前发起。
- 编辑后旧 Drawer 自动关闭。
- 危险链接显示阻断项。
- 发布网络顺序为 quality-check、create/save、publish。
- Drawer 动画结束后贴合视口，不以动画中间帧作为布局证据。

证据：

- `docs/process/evidence/content-quality-p17-desktop-1280x720.png`
- `docs/process/evidence/content-quality-p17-mobile-390x844.png`

## 5. 运行态

- API：`http://localhost:18080`，当前二进制 `dist/dev/zoking-api-p17.exe`。
- Admin：`http://localhost:5173`。
- C 端：`http://localhost:1313`。
- PostgreSQL：`localhost:15432`。

API 重启后 `/readyz` 返回 200；无 token 调用新质量路由返回 401，证明新路由已在实际服务生效。
