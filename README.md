# Zoking Blog

Zoking Blog 是一个“静态阅读体验 + 动态内容控制面”的全栈个人博客系统。读者端由 Hugo Theme Stack 构建为静态站点；React Admin 负责内容运营；Go Gin API、PostgreSQL 和独立 Worker 负责编辑数据、权限、预览、发布与回滚；GoatCounter 提供自托管访问统计。

生产环境采用两层部署：Azure VPS 只负责 Caddy HTTPS 与 WireGuard 公网入口，应用和数据实际运行在内网物理服务器的 Docker Compose 中。

完整接手请先阅读 [项目介绍与交接手册](docs/project-handover-handbook.md)，部署、备份和监控分别见 [部署 Runbook](docs/operations/deployment-runbook.md) 与 [生产备份、恢复与监控](docs/operations/backup-and-monitoring.md)。

Current shape:

```text
apps/
  site/   Hugo + Stack reader site, seeded from the Stack demo
  api/    Go Gin + GORM API
  admin/  React/Vite admin shell
db/
  migrations/
infra/
  docker/
docs/
```

核心数据流：

```text
Admin -> Gin API -> PostgreSQL -> Publish Worker -> Hugo release -> Reader Nginx
                                  |
                                  `-> Preview / Pagefind / rollback metadata
```

## Local Baseline

Prerequisites: Go 1.23+, Node.js 22+, PowerShell 7+, Docker Desktop, and Git. The repository includes a local Hugo Extended binary under `.tools/hugo` for Windows development; CI installs the pinned version independently.

Start PostgreSQL:

```powershell
docker compose -f infra/docker/compose.dev.yml up -d postgres
```

Run API migration, seed, and server:

```powershell
cd apps/api
go mod tidy
Copy-Item ..\..\.env.example ..\..\.env
go run ./cmd/migrate up
go run ./cmd/seed
go run ./cmd/api
```

Run Admin:

```powershell
cd apps/admin
npm install
npm run dev
```

Run Hugo site:

```powershell
.\.tools\hugo\hugo.exe server --source apps/site
```

Refresh the reader site's GitHub project snapshot manually:

```powershell
node .\scripts\sync-github-projects.mjs --username zo-king
```

GitHub Actions also refreshes `apps/site/data/github_projects.json` on the 1st and 16th of every month. The public project page reads this committed snapshot and never calls the GitHub API from a visitor's browser.

Local services:

- Reader site: `http://localhost:1313`
- Admin console: `http://localhost:5173`
- API health check: `http://localhost:18080/healthz`

Run the local end-to-end smoke after API/PostgreSQL are running:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\e2e-smoke.ps1
```

Run the deployment preflight before merging or deploying:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1
```

For build-only checks without E2E:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
```

Production Docker Compose baseline:

```powershell
Copy-Item infra/docker/.env.prod.example infra/docker/.env.prod
docker compose --env-file infra/docker/.env.prod -f infra/docker/compose.prod.yml config
```

See [deployment runbook](docs/operations/deployment-runbook.md) before running production services.

## Important Docs

- [项目介绍与交接手册](docs/project-handover-handbook.md)
- [Engineering execution master plan](docs/plan/engineering-execution-master-plan.md)
- [Task board](docs/process/task-board.md)
- [Worklog](docs/process/worklog.md)
- [Architecture overview](docs/architecture/00-system-overview.md)
- [API contract](docs/backend/00-api-contract.md)
- [Stack integration](docs/frontend/site-stack-integration.md)
- [Production backup and monitoring](docs/operations/backup-and-monitoring.md)

## Theme Attribution

The C-side reader experience is based on [Hugo Theme Stack](https://github.com/CaiJimmy/hugo-theme-stack), licensed under GPL-3.0-only. Keep the theme attribution in generated pages.
