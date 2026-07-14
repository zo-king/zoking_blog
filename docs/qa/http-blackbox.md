# HTTP 黑盒回归测试

## 1. 定位

`scripts/qa/http-blackbox.ps1` 只从 HTTP 边界观察系统，不导入 Go 包、不读取数据库、不依赖实现细节。它用于本地联调、部署前只读冒烟和人工审核前回归，与会创建并清理业务数据的 `e2e-smoke.ps1` 分工明确。

覆盖范围：

- API health/readiness、JSON Content-Type、响应 envelope 与 request ID header/body 一致性。
- 公开文章、页面、分类、标签、站点设置读取，以及不存在文章的结构化 404。
- Admin 未授权边界、错误凭据与请求校验。
- 可选真实管理员登录：HttpOnly Cookie、CSRF、可信 Origin、会话恢复轮换、退出后失效。
- CORS 允许站点 origin、拒绝未知 origin。
- C 端中文 HTML、canonical、main、robots、sitemap，以及中文 noindex 404。
- Admin 根路由、SPA history fallback、runtime config 与开发代理。
- 可选评论限流探针。

## 2. 默认运行

先启动 PostgreSQL、API、Admin 和 Hugo，再从仓库根目录执行：

```powershell
pwsh -NoProfile -File .\scripts\qa\http-blackbox.ps1
```

默认地址：

- API：`http://localhost:18080`
- Admin：`http://localhost:5173`
- C 端：`http://localhost:1313`

指定环境：

```powershell
pwsh -NoProfile -File .\scripts\qa\http-blackbox.ps1 `
  -ApiBase https://api.zoking.tech `
  -AdminBase https://admin.zoking.tech `
  -SiteBase https://zoking.tech `
  -AdminOrigin https://admin.zoking.tech `
  -AdminEmail $env:ZOKING_QA_ADMIN_EMAIL `
  -AdminPassword $env:ZOKING_QA_ADMIN_PASSWORD
```

只有同时提供 `AdminOrigin`、`AdminEmail`、`AdminPassword` 时才启用认证检查。该模式验证登录不泄漏 JWT、Cookie 属性、`/auth/session` 的 CSRF 轮换、缺失/错误 CSRF、错误 Origin、logout 删除 Cookie 以及 logout 后 `/auth/me` 返回 401。脚本不创建文章、页面、评论、预览或 release，也不修改站点设置。

## 3. 限流探针

评论限流探针发送语法不完整的 JSON。请求会在 handler 校验前经过限流器，但不会产生成功写入：

```powershell
pwsh -NoProfile -File .\scripts\qa\http-blackbox.ps1 -TestRateLimit
```

它要求同一 API 进程先返回 `422`，随后在突发额度耗尽后返回 `429`。限流状态保存在进程内存中，运行后该来源 IP 可能短暂受到限制；建议在隔离 API 上执行，或执行后重启本地 API。不要把该开关用于共享生产入口。

## 4. 白盒对应关系

Go 白盒测试位于：

- `apps/api/internal/publisher/*_test.go`：生产 URL、产物 HTML、Preview key、release 路径。
- `apps/api/internal/httpapi/*_test.go`：CORS、路由、分页、RBAC、可信代理、评论限流、Preview 文件与 symlink 边界。
- `apps/api/internal/config/*_test.go`：环境变量传播。
- `apps/api/cmd/seed/*_test.go`：非开发环境 seed fail-closed、空白/占位/超长 bcrypt 凭据门禁。

运行：

```powershell
pwsh -NoProfile -File .\scripts\qa\whitebox.ps1
```

该入口使用 `go test -count=1 -covermode=atomic` 禁用测试缓存，覆盖全部 Go package，再执行 `go vet ./...`。覆盖率文件默认写入 `dist/qa/whitebox-cover.out`，也可通过 `-CoverageFile` 指定仓库内其他位置。

完整写链路仍使用隔离测试数据库：

```powershell
$env:APP_ENV = "test"
$env:DATABASE_URL = "postgres://.../zoking_blog_test?sslmode=disable"
pwsh -NoProfile -File .\scripts\qa\preflight.ps1 `
  -ApiBase http://localhost:18081 `
  -StartApi -BootstrapTestData -StopStartedApi
```

禁止针对 development 或 production 数据库运行完整 E2E。

2026-07-14 P24 验收结果：最新隔离 API 的认证黑盒 `26/26`，覆盖 Cookie/CSRF/session 恢复；默认模式仍保持只读。完整 E2E 使用 `zoking_blog_test` 与 `storage/qa/preflight-runtime`，覆盖登录、taxonomy、媒体、文章、页面、三类预览、异步发布、设置发布、评论审核、rollback 和 manifest 清理，结束后隔离 API 停止且运行目录删除。
