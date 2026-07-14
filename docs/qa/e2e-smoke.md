# 端到端冒烟脚本

本文记录 `scripts/qa/e2e-smoke.ps1` 的用途、前置条件和验收范围。

## 1. 目标

该脚本用于验证当前全栈博客最重要的用户链路：

```text
API ready
-> Admin 登录
-> 创建分类/标签
-> 校验页面 reserved slug 保护
-> 上传媒体
-> 创建独立页面
-> 创建文章
-> 构建文章、页面和临时站点设置预览
-> 校验预览可读且不创建 release、不切换 active release、不持久化设置
-> 异步发布文章 job
-> 异步发布页面 job
-> 保存并异步发布站点设置 job
-> worker 生成 Hugo release
-> 检查 manifest、文章 HTML、页面 HTML、首页菜单和设置生效
-> 递归解析 Sitemap `<loc>` 校验新文章和新页面 URL
-> 校验媒体引用记录与删除保护
-> 校验孤立媒体清理 dry-run
-> 公开评论提交
-> 后台审核评论
-> 公开评论可见
-> promote 历史 release 验证回滚
-> 校验 release 清理 dry-run 不包含 active release
-> finally 调用 QA E2E run cleanup，dry-run 后 apply，并校验 deleted=candidates
```

## 2. 前置条件

必须已经启动：

- PostgreSQL：`localhost:15432`
- API：`http://localhost:18080`
- API 进程内嵌 publish worker，或单独运行 `go run ./cmd/worker`
- API 必须显式运行在 `APP_ENV=test`，并启用 `QA_E2E_CLEANUP_ENABLED=true`
- PowerShell 7+：脚本使用 `Invoke-RestMethod -Form` 上传媒体。
- 脚本与 API/worker 共享同一文件系统：脚本会读取 release `output_path`、Hugo content 和构建产物做本地文件断言。
- API 必须使用 `_test` 数据库以及独立的 Hugo source/public、release、preview、media 文件根；本地优先通过 `preflight.ps1 -StartApi` 启动。

默认管理员：

```text
admin@zoking.local / ChangeMe123!
```

## 3. 运行命令

从仓库根目录运行：

```powershell
.\scripts\qa\e2e-smoke.ps1
```

指定 API 地址：

```powershell
.\scripts\qa\e2e-smoke.ps1 -ApiBase http://localhost:18081 -HugoSiteDir .\storage\qa\preflight-runtime\site
```

跳过回滚验证：

```powershell
.\scripts\qa\e2e-smoke.ps1 -SkipRollback
```

诊断时跳过 E2E run cleanup：

```powershell
.\scripts\qa\e2e-smoke.ps1 -SkipE2ECleanup
```

`-SkipE2ECleanup` 只允许在完全隔离的 test API 上诊断。正常执行应由 preflight 显式 bootstrap baseline，并保持 cleanup 开启。

## 4. 输出

成功时输出 JSON 摘要：

```json
{
  "ok": true,
  "run_id": "...",
  "slug": "e2e-smoke-...",
  "page_slug": "e2e-page-...",
  "post_preview_key": "prev-...",
  "post_preview_url": "http://localhost:18080/preview-files/prev-.../p/e2e-smoke-.../",
  "page_preview_url": "http://localhost:18080/preview-files/prev-.../e2e-page-.../",
  "settings_preview_url": "http://localhost:18080/preview-files/prev-.../",
  "job_status": "published",
  "page_job_status": "published",
  "settings_job_status": "published",
  "release_key": "rel_...",
  "hugo_site_dir": ".../storage/qa/preflight-runtime/site",
  "media_url": "http://localhost:18080/media-files/...",
  "comment_id": "...",
  "cleanup_skipped": false,
  "cleanup": {
    "dry_run": {
      "candidates": {
        "posts": 1,
        "pages": 1,
        "categories": 1,
        "tags": 1,
        "comments": 1,
        "media": 2,
        "previews": 3,
        "jobs": 3,
        "releases": 3
      }
    },
    "apply": {
      "deleted": {
        "posts": 1,
        "pages": 1,
        "categories": 1,
        "tags": 1,
        "comments": 1,
        "media": 2,
        "previews": 3,
        "jobs": 3,
        "releases": 3
      }
    }
  }
}
```

失败时 PowerShell 会抛出异常，并在异常文本中说明失败断言。

## 5. 注意事项

- 脚本会写入数据库、上传本地测试图片、生成 Hugo content 和 release 产物；默认会在 `finally` 中调用 `POST /api/v1/admin/qa/e2e-runs/{run_id}/cleanup` 清理本次 run 的资源。
- 登录后脚本会先恢复 `storage/qa/e2e-runs/*.json` 中未完成的 run；恢复失败会立即停止，禁止在旧污染上继续创建资源。
- 当前 run 的 manifest journal 原子持久化到 `storage/qa/e2e-runs/{run_id}.json`，记录 post/page/category/tag/comment/media/preview/job/release 的 id 和 slug/key；每次追加都会先写临时文件再替换正式 journal。
- `settings_before` 和 baseline active release 会在任何写操作前捕获；cleanup apply 会先恢复设置和 baseline release，再删除本次 run 的资源。
- cleanup 会先 dry-run，再 apply；响应必须显式包含每一类 `candidates`/`deleted` 字段，且 apply candidates 与 deleted 都必须逐项等于 dry-run candidates。
- 默认会把测试前的 active release promote 回来，以验证 rollback；如果当前数据库还没有 active release，先使用 `-SkipRollback -SkipE2ECleanup` 创建诊断 baseline release，再运行一次默认脚本完成可清理验证。
- 产物检查包括 manifest、首页、文章页、页面页、RSS、递归 Sitemap、分类页、标签页、文章 HTML 内容、页面菜单、站点标题/侧栏副标题、媒体引用和评论容器。
- Sitemap 检查会递归解析 release 下所有 `sitemap.xml` 的 `<loc>`，新文章或新页面 URL 未收录会直接失败。
- 媒体检查会确认发布后 `usage_count > 0`，并确认被引用媒体删除返回 409。
- 清理检查只做 dry-run：孤立媒体 dry-run 必须能识别测试孤立图片；release dry-run 不能包含 active release。
- 页面检查会先尝试创建 reserved slug `search` 并要求 API 返回 422，再用唯一 slug 创建真实页面。
- 设置检查会保存临时时间戳标题/侧栏副标题并发布 site release，确认 Hugo 默认语言覆盖层生效。
- 预览检查发生在正式发布前，覆盖文章 taxonomy/media、独立页面和临时设置；同时比较 release 数量、active release 与设置 hash，防止预览污染正式状态。
- `-RestoreSettings` 保留为兼容参数；当前默认 cleanup 已无条件捕获并恢复运行前设置。
- cleanup 成功后 journal 会先标记 `cleanup_completed=true` 再删除；cleanup 失败、进程中断或 `-SkipE2ECleanup` 诊断运行会保留 journal，供下次运行恢复。
- `-SkipE2ECleanup` 会保留本次 run 的文章、页面、媒体、预览、job、release 等资源，仅用于诊断。
- 文章、页面、分类和标签 slug 都包含完整 `run_id`，媒体原始文件名也包含同一 `run_id`，用于服务端严格校验资源归属。
- 服务端 cleanup 路由只在 `APP_ENV=test` 且 `QA_E2E_CLEANUP_ENABLED=true` 时注册；manifest 中任一 ID、slug/key、媒体原始文件名或数据库关系不匹配都会以冲突失败，不会退回前缀批量删除。
- `-HugoSiteDir` 必须与 API 的 `HUGO_SITE_DIR` 指向同一副本；文章、页面、分类、标签和 release 产物路径都使用跨平台逐段拼接，可在 Windows 与 Ubuntu CI 运行。
- 该脚本是开发/验收冒烟，不替代单元测试、集成测试和生产部署巡检。
