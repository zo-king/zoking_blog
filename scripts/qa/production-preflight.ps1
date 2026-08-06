param(
    [string]$EnvFile = "infra/docker/.env.prod",
    [switch]$AllowNonLoopbackBindings
)

$ErrorActionPreference = "Stop"

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "../.."))
Set-Location $repoRoot

function Fail([string]$Message) {
    throw "production preflight failed: $Message"
}

function Pass([string]$Message) {
    Write-Host "[production-preflight] PASS $Message" -ForegroundColor Green
}

function Read-EnvFile([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        Fail "missing $Path; copy infra/docker/.env.prod.example and fill every required value"
    }

    $values = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) { continue }
        if ($trimmed -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
            Fail "invalid line in env file"
        }
        $key = $Matches[1]
        $value = $Matches[2].Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $values[$key] = $value
    }
    return $values
}

function RequiredValue($Values, [string]$Key) {
    if (-not $Values.ContainsKey($Key) -or [string]::IsNullOrWhiteSpace([string]$Values[$Key])) {
        Fail "$Key is required"
    }
    return [string]$Values[$Key]
}

function SecretValue($Values, [string]$Key, [int]$MinimumLength) {
    $value = RequiredValue $Values $Key
    $lower = $value.ToLowerInvariant()
    if ($value -match '__REQUIRED_|change[-_]?me|changeme|dev[-_]?only|replace[-_]?me|example-secret') {
        Fail "$Key still contains a placeholder"
    }
    if ($value.Length -lt $MinimumLength) {
        Fail "$Key must contain at least $MinimumLength characters"
    }
    if ($value -ne $value.Trim()) {
        Fail "$Key must not contain surrounding whitespace"
    }
    return $value
}

$values = Read-EnvFile $EnvFile
$null = SecretValue $values "POSTGRES_PASSWORD" 16
$null = SecretValue $values "JWT_SECRET" 32
$null = SecretValue $values "PRIVACY_HASH_SECRET" 32
$null = SecretValue $values "SEED_ADMIN_PASSWORD" 16
$null = SecretValue $values "GOATCOUNTER_PASSWORD" 8

$adminUsername = RequiredValue $values "SEED_ADMIN_USERNAME"
if ($adminUsername -ne "zoking") {
    Fail "SEED_ADMIN_USERNAME must be zoking for the configured production administrator"
}
$adminEmail = RequiredValue $values "SEED_ADMIN_EMAIL"
if ($adminEmail -eq "admin@zoking.local" -or $adminEmail -notmatch '^[^@\s]+@[^@\s]+\.[^@\s]+$') {
    Fail "SEED_ADMIN_EMAIL must be a real production email address"
}

foreach ($key in @("SITE_BASE_URL", "PUBLIC_API_BASE_URL", "PUBLISH_PREVIEW_PUBLIC_BASE_URL", "CORS_ALLOWED_ORIGINS", "ADMIN_ALLOWED_ORIGINS")) {
    $value = RequiredValue $values $key
    if ($value -notmatch '^https://') {
        Fail "$key must use HTTPS"
    }
}
if ([string]$values["CORS_ALLOWED_ORIGINS"] -match '/\s*(,|$)' -or [string]$values["ADMIN_ALLOWED_ORIGINS"] -match '/\s*(,|$)') {
    Fail "CORS_ALLOWED_ORIGINS and ADMIN_ALLOWED_ORIGINS must contain origins without paths or trailing slashes"
}

foreach ($key in @("API_BIND_ADDRESS", "ADMIN_BIND_ADDRESS", "SITE_BIND_ADDRESS", "STATS_BIND_ADDRESS")) {
    $value = RequiredValue $values $key
    if (-not $AllowNonLoopbackBindings -and $value -notin @("127.0.0.1", "::1")) {
        Fail "$key must bind to loopback unless -AllowNonLoopbackBindings is explicitly supplied"
    }
}

foreach ($relativePath in @("infra/docker/compose.prod.yml", "apps/api/Dockerfile", "apps/admin/Dockerfile", "db/migrations")) {
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot $relativePath))) {
        Fail "required deployment path is missing: $relativePath"
    }
}

if (git ls-files --error-unmatch -- "$EnvFile" 2>$null) {
    Fail "$EnvFile is tracked by git; remove it from the index before adding secrets"
}

docker compose --env-file $EnvFile -f infra/docker/compose.prod.yml config --quiet
if ($LASTEXITCODE -ne 0) {
    Fail "docker compose config validation failed"
}

Pass "production env file contains required non-placeholder values"
Pass "administrator username is zoking"
Pass "production URLs and origin allowlists use HTTPS"
Pass "service bindings are loopback-only"
Pass "deployment files and migrations are present"
Pass "Compose production configuration is valid"
Write-Host "[production-preflight] READY: run migration, seed, then start api/worker/admin/site with the deployment runbook." -ForegroundColor Cyan
