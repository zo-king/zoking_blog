# Zoking Blog

Zoking Blog is a full-stack blog project that keeps the reader-facing Hugo Theme Stack experience while adding a separated Go API, PostgreSQL database, and B-side admin console.

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

- [Engineering execution master plan](docs/plan/engineering-execution-master-plan.md)
- [Task board](docs/process/task-board.md)
- [Worklog](docs/process/worklog.md)
- [Architecture overview](docs/architecture/00-system-overview.md)
- [API contract](docs/backend/00-api-contract.md)
- [Stack integration](docs/frontend/site-stack-integration.md)

## Theme Attribution

The C-side reader experience is based on [Hugo Theme Stack](https://github.com/CaiJimmy/hugo-theme-stack), licensed under GPL-3.0-only. Keep the theme attribution in generated pages.
