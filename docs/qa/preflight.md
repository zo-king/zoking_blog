# 部署前检查脚本

本文记录 `scripts/qa/preflight.ps1` 的用途、运行方式和 CI 行为。

## 1. 目标

`preflight.ps1` 是上线前的一键检查入口：

```text
Go tests
-> Admin npm ci/build
-> Hugo production build
-> test database guard + migrate (only when preflight starts the API)
-> optional seed + settings-only baseline bootstrap
-> guarded E2E draft/site preview and publish smoke with manifest cleanup
-> release rollback smoke
```

默认不会 seed 或创建 baseline。只有显式传入 `-BootstrapTestData -StartApi`，且 `APP_ENV=test`、`DATABASE_URL` 中的数据库名以 `_test` 结尾时，preflight 才会在 migration 后运行 `cmd/seed`。API ready 后若没有可用于本次隔离运行目录的 active release，脚本会登录 Admin，调用 settings publish API 创建 settings-only baseline，并轮询 publish job 到 `published`。

## 2. 本地运行

完整检查：

```powershell
pwsh -NoProfile -File .\scripts\qa\preflight.ps1
```

仅跑构建与单元测试：

```powershell
pwsh -NoProfile -File .\scripts\qa\preflight.ps1 -SkipE2E
```

如果 API 没有提前启动，但测试 PostgreSQL 已经可用：

```powershell
$env:APP_ENV = "test"
$env:DATABASE_URL = "postgres://zoking:password@localhost:15432/zoking_blog_test?sslmode=disable"
pwsh -NoProfile -File .\scripts\qa\preflight.ps1 -ApiBase http://localhost:18081 -StartApi -StopStartedApi
```

空测试库需要显式 bootstrap：

```powershell
$env:APP_ENV = "test"
$env:DATABASE_URL = "postgres://zoking:password@localhost:15432/zoking_blog_test?sslmode=disable"
$env:SEED_ADMIN_EMAIL = "admin@zoking.local"
$env:SEED_ADMIN_PASSWORD = "ChangeMe123!"
pwsh -NoProfile -File .\scripts\qa\preflight.ps1 -ApiBase http://localhost:18081 -StartApi -BootstrapTestData -StopStartedApi -AdminEmail $env:SEED_ADMIN_EMAIL -AdminPassword $env:SEED_ADMIN_PASSWORD
```

使用 `-StartApi` 时，`ApiBase` 必须是未占用的 loopback HTTP 地址，脚本会从 URL 自动设置子进程 `APP_PORT`；若该地址已有 API 响应则直接拒绝，避免误复用开发进程。不传 `-StartApi` 时可以复用已 ready 的 test API，但不会执行 migration、seed 或 baseline bootstrap，且调用方必须保证该 API 使用独立测试数据库和文件根。

preflight 自己启动临时 API 时要求调用环境显式提供：

```powershell
$env:APP_ENV = "test"
$env:DATABASE_URL = "postgres://.../database_name_test?sslmode=disable"
```

托管 API 会强制设置 `APP_ENV=test`、`QA_E2E_CLEANUP_ENABLED=true`，并把 Hugo source/public、release、preview 和 media 全部指向 `storage/qa/preflight-runtime`。`APP_ENV=development`、未设置 `APP_ENV`、缺少/无效 `DATABASE_URL` 或数据库名不以 `_test` 结尾都会在任何 migration/seed 之前被拒绝。

## 3. 参数

| 参数 | 说明 |
|---|---|
| `-ApiBase` | API 地址，默认 `http://localhost:18080`；`-StartApi` 时必须是未占用的 loopback URL，并用于推导 `APP_PORT` |
| `-AdminEmail` / `-AdminPassword` | E2E 登录账号 |
| `-PublishTimeoutSeconds` | 发布 job 等待时间，默认 180 秒 |
| `-SkipE2E` | 跳过 migrate/smoke，只跑 Go/Admin/Hugo |
| `-SkipRollback` | E2E smoke 跳过 rollback |
| `-Install` | 强制执行 `npm ci` |
| `-StartApi` | 由 preflight 启动 test API；仅允许 `APP_ENV=test` 和 `_test` 数据库 |
| `-BootstrapTestData` | migration 后运行 seed，并在无 active release 时创建 settings-only baseline；必须与 `-StartApi` 一起使用 |
| `-StopStartedApi` | 结束时停止 preflight 启动的 API |

## 4. E2E 安全门禁

运行 E2E 前，preflight 会做以下检查：

- 静态解析 `scripts/qa/e2e-smoke.ps1`，确认脚本没有 parser error。
- 拒绝运行缺少 manifest run id、`/api/v1/admin/qa/e2e-runs/.../cleanup` 调用或 `finally` cleanup 块的 smoke。
- 拒绝 E2E smoke 自身包含 `cmd/seed` 或“用 `-SkipRollback` 创建 baseline release”旧提示；显式 test bootstrap 由 preflight 在 smoke 之外完成。
- API ready 后，用当前管理员账号探测 test-only QA cleanup 路由；只有返回路由级 `422` 校验错误时才认为 cleanup 路由可用。
- 每次 migration 和 seed 前都会重新校验 `APP_ENV=test` 与 `_test` 数据库名。
- `-StartApi` 会复制一份不含 `public/resources/dist` 的 Hugo source 到 `storage/qa/preflight-runtime/site`，并修正副本的本地 Hugo Module replace；正式 `apps/site` 不参与 E2E 写入。
- 托管 API 的 `HUGO_SITE_DIR`、`HUGO_PUBLIC_DIR`、`PUBLISH_RELEASE_ROOT`、`PUBLISH_CURRENT_DIR`、`PUBLISH_PREVIEW_ROOT`、`MEDIA_LOCAL_DIR` 都位于同一个受控 runtime 根下。
- 托管 API 会先编译到 runtime 的 `bin/zoking-api(.exe)`，再直接启动该可执行文件；结束时等待真实 API PID 退出并释放句柄，不通过 `go run` 留下编译子进程。
- 若 preflight 启动 API 后任一步失败，会停止仅由 preflight 启动的 API 父/子进程；成功后是否停止由 `-StopStartedApi` 控制。
- 失败或传入 `-StopStartedApi` 时会在进程退出后校验路径边界并删除隔离 runtime；若故意保留托管 API，runtime 也会保留并输出位置。

## 5. CI

GitHub Actions 工作流位于 `.github/workflows/preflight.yml`。

CI 会：

- 启动数据库名为 `zoking_blog_test` 的 PostgreSQL 16 service。
- 安装 Go、Node.js、Hugo Extended。
- 设置 `APP_ENV=test`、显式 seed 管理员凭据和 API 使用的 Hugo binary 路径。
- 执行：

```powershell
./scripts/qa/preflight.ps1 -Install -StartApi -BootstrapTestData -StopStartedApi -AdminEmail $env:SEED_ADMIN_EMAIL -AdminPassword $env:SEED_ADMIN_PASSWORD -PublishTimeoutSeconds 240
```

## 6. 通过标准

- `go test ./...` 通过。
- `npm run build` 通过。
- Hugo build 生成 `dist/preflight-site`。
- E2E smoke 必须是具备 manifest/finally cleanup 的版本，并完成文章/页面/设置隔离预览，以及文章、页面、设置、媒体、清理 dry-run、评论和 rollback 验收。
- E2E smoke 使用 `-RestoreSettings`，并通过 QA cleanup 路由清理本次 manifest 内的测试数据，避免临时标题/侧栏文案和 E2E 内容残留。
- 使用 `-StopStartedApi` 时，`storage/qa/preflight-runtime` 最终不存在，开发数据库、`apps/site`、正式 media/release 目录的基线计数与文件均不改变。
- 脚本退出码为 0。
