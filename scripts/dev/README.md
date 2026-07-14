# Development Commands

From the repository root:

```powershell
docker compose -f infra/docker/compose.dev.yml up -d postgres
```

PostgreSQL is exposed on `localhost:15432` to avoid colliding with an existing local database.

API:

```powershell
cd apps/api
go mod tidy
go run ./cmd/migrate up
go run ./cmd/seed
go run ./cmd/api
```

The API listens on `http://localhost:18080` by default. In development, `PUBLISH_WORKER_ENABLED=true` starts an embedded publish worker in the API process so publish jobs are processed automatically.

Standalone publish worker, useful when `PUBLISH_WORKER_ENABLED=false` in the API process:

```powershell
cd apps/api
go run ./cmd/worker
```

Admin:

```powershell
cd apps/admin
npm install
npm run dev
```

Site:

```powershell
hugo server --source apps/site
```

The Hugo site listens on `http://localhost:1313` by default.

End-to-end smoke:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\e2e-smoke.ps1
```
