#!/usr/bin/env pwsh
# Dev bootstrap: start Postgres, wait until ready, run migrations + seed.
# Then start the API yourself with:  go run ./cmd/api
# Prerequisite: Docker Desktop must be running.

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)  # repo root (scripts/dev -> ..\..)
Set-Location $root

# Ensure a local .env exists (migrate/api read DATABASE_URL etc. from it).
$envFile = Join-Path $root '.env'
if (-not (Test-Path $envFile)) {
    Copy-Item (Join-Path $root '.env.example') $envFile
    Write-Host '==> Created .env from .env.example' -ForegroundColor Cyan
}

Write-Host '==> Starting PostgreSQL (docker compose) ...' -ForegroundColor Cyan
docker compose -f infra/docker/compose.dev.yml up -d postgres

Write-Host '==> Waiting for Postgres to accept connections ...' -ForegroundColor Cyan
$ready = $false
for ($i = 0; $i -lt 40; $i++) {
    docker compose -f infra/docker/compose.dev.yml exec -T postgres pg_isready -U zoking -d zoking_blog *> $null
    if ($LASTEXITCODE -eq 0) { $ready = $true; break }
    Start-Sleep -Seconds 1
}
if (-not $ready) {
    Write-Warning 'Postgres did not report ready within 40s. Is Docker Desktop running? Check: docker ps'
    exit 1
}
Write-Host '    Postgres is ready.' -ForegroundColor Green

Set-Location (Join-Path $root 'apps\api')

Write-Host '==> Running migrations ...' -ForegroundColor Cyan
go run ./cmd/migrate up

Write-Host '==> Seeding (safe to re-run) ...' -ForegroundColor Cyan
go run ./cmd/seed

Write-Host ''
Write-Host 'Ready. Now start the API in this window:' -ForegroundColor Green
Write-Host '    go run ./cmd/api' -ForegroundColor Yellow
