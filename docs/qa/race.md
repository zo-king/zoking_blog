# Go 竞态检测

## 1. 定位

GitHub Actions 的 `Go race detector` job 在 Ubuntu 上启用 CGO，并使用 Go race detector 检查并发敏感 package。它使用独立的 `zoking_blog_test` PostgreSQL service，因此真实 Preview 终态竞争和发布失败不变式集成测试不会被跳过；该 job 不安装 Node.js 或 Hugo，也不重复完整 preflight/E2E 链路。

## 2. Package 范围

CI 从 `apps/api` 执行：

```bash
CGO_ENABLED=1 go test -race -count=1 \
  ./internal/httpapi \
  ./internal/publisher \
  ./internal/maintenance
```

范围固定为：

- `internal/httpapi`：覆盖共享内存状态与并发 HTTP 请求，包括评论限流器的互斥访问。
- `internal/httpapi` 的 PostgreSQL 集成用例：真实并发执行 Preview finish/fail，要求恰好一个终态更新成功，另一个返回终态竞争错误，并核对最终字段一致。
- `internal/publisher`：覆盖后台发布 worker、任务认领、发布文件操作和生产产物失败不变式；失败不得创建 release、切换 active/current 或遗留未登记产物。
- `internal/maintenance`：覆盖后台清理循环和清理事务所在 package。

`cmd/seed` 的双进程首次初始化竞争由 `scripts/qa/seed-concurrency.ps1` 验证；Go race detector 不能检测不同进程之间的数据库竞争，因此不在本 gate 中重复执行。

## 3. CI 行为

工作流位于 `.github/workflows/preflight.yml`。race job 使用 `ubuntu-latest`、`actions/setup-go@v5` 和 `apps/api/go.mod` 指定的 Go 版本，并显式设置 `CGO_ENABLED=1`、`APP_ENV=test` 和只指向 `zoking_blog_test` 的 `DATABASE_URL`。集成用例继续为每个测试创建独立 schema，结束后自动删除。

`go test -race` 发现 data race 或测试失败时会返回非零退出码，job 和整个工作流随即失败。`-count=1` 禁用测试结果缓存，确保每次 CI 都实际执行目标测试。

现有 `preflight` job 保持原有 PostgreSQL、Go、Node.js、Hugo 和完整 E2E 语义；两个 job 并行运行，race gate 不依赖 E2E service 或产物。

## 4. 本地验证

Linux 且已安装支持 CGO 的 C toolchain 时，可从仓库根目录运行：

```bash
cd apps/api
CGO_ENABLED=1 go test -race -count=1 \
  ./internal/httpapi \
  ./internal/publisher \
  ./internal/maintenance
```

Windows 当前环境的 `CGO_ENABLED=0`，只能运行同范围的普通测试来验证 package 和测试依赖；race 结果以 Linux CI 为准。

2026-07-13 本地验收使用 `golang:1.25-bookworm` 加入 PostgreSQL 容器网络，实际连接 `zoking_blog_test` 执行上述三个 package，全部通过且未报告 data race。
