param(
    [string]$DatabaseName = "zoking_blog_seed_concurrency_test",
    [string]$DatabaseHost = "localhost",
    [int]$DatabasePort = 15432,
    [string]$DatabaseUser = "zoking",
    [string]$DatabasePassword = "zoking_dev_password",
    [string]$PostgresContainer = "zoking-blog-postgres",
    [string]$AdminEmail = "admin@zoking.local",
    [string]$AdminPassword = "ChangeMe123!"
)

$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
    throw "PowerShell 7+ is required."
}
if ($DatabaseName -notmatch '^zoking_blog_seed_[a-z0-9_]*_test$') {
    throw "DatabaseName must be a dedicated zoking_blog_seed_*_test database."
}
if ($AdminEmail -notmatch '^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+$') {
    throw "AdminEmail contains unsupported characters for this isolated test."
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$ApiDir = Join-Path $RepoRoot "apps/api"
$QaDir = Join-Path $RepoRoot "dist/qa"
$binaryName = if ($IsWindows) { "zoking-seed-concurrency.exe" } else { "zoking-seed-concurrency" }
$binaryPath = Join-Path $QaDir $binaryName
$stdout1 = Join-Path $QaDir "seed-concurrency-1.out.log"
$stderr1 = Join-Path $QaDir "seed-concurrency-1.err.log"
$stdout2 = Join-Path $QaDir "seed-concurrency-2.out.log"
$stderr2 = Join-Path $QaDir "seed-concurrency-2.err.log"
$artifacts = @($binaryPath, $stdout1, $stderr1, $stdout2, $stderr2)
$environmentKeys = @("APP_ENV", "DATABASE_URL", "SEED_ADMIN_EMAIL", "SEED_ADMIN_PASSWORD")
$previousEnvironment = @{}

function Invoke-DockerPSQL {
    param(
        [string]$Database,
        [string]$SQL,
        [switch]$TuplesOnly
    )

    $arguments = @("exec", $PostgresContainer, "psql", "-U", $DatabaseUser, "-d", $Database, "-v", "ON_ERROR_STOP=1")
    if ($TuplesOnly) {
        $arguments += "-At"
    }
    $arguments += @("-c", $SQL)
    $output = & docker @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "psql failed for database $Database with exit code $LASTEXITCODE."
    }
    return @($output)
}

function Start-SeedProcess {
    param(
        [string]$StandardOutput,
        [string]$StandardError
    )

    $arguments = @{
        FilePath               = $binaryPath
        WorkingDirectory       = $ApiDir
        RedirectStandardOutput = $StandardOutput
        RedirectStandardError  = $StandardError
        PassThru               = $true
    }
    if ($IsWindows) {
        $arguments.WindowStyle = "Hidden"
    }
    return Start-Process @arguments
}

New-Item -ItemType Directory -Force -Path $QaDir | Out-Null
foreach ($key in $environmentKeys) {
    $previousEnvironment[$key] = [System.Environment]::GetEnvironmentVariable($key, "Process")
}

$databaseCreated = $false
try {
    Write-Host "[seed-concurrency] recreating isolated database $DatabaseName"
    [void](Invoke-DockerPSQL -Database "postgres" -SQL "drop database if exists $DatabaseName with (force);")
    [void](Invoke-DockerPSQL -Database "postgres" -SQL "create database $DatabaseName;")
    $databaseCreated = $true

    $escapedUser = [System.Uri]::EscapeDataString($DatabaseUser)
    $escapedPassword = [System.Uri]::EscapeDataString($DatabasePassword)
    $databaseURL = "postgres://${escapedUser}:${escapedPassword}@${DatabaseHost}:${DatabasePort}/${DatabaseName}?sslmode=disable"
    [System.Environment]::SetEnvironmentVariable("APP_ENV", "test", "Process")
    [System.Environment]::SetEnvironmentVariable("DATABASE_URL", $databaseURL, "Process")
    [System.Environment]::SetEnvironmentVariable("SEED_ADMIN_EMAIL", $AdminEmail, "Process")
    [System.Environment]::SetEnvironmentVariable("SEED_ADMIN_PASSWORD", $AdminPassword, "Process")

    Write-Host "[seed-concurrency] migrating isolated database"
    Push-Location $ApiDir
    try {
        & go run ./cmd/migrate up
        if ($LASTEXITCODE -ne 0) {
            throw "migration failed with exit code $LASTEXITCODE."
        }
        & go build -o $binaryPath ./cmd/seed
        if ($LASTEXITCODE -ne 0) {
            throw "seed build failed with exit code $LASTEXITCODE."
        }
    } finally {
        Pop-Location
    }

    Write-Host "[seed-concurrency] starting two first-run seed processes"
    $process1 = Start-SeedProcess -StandardOutput $stdout1 -StandardError $stderr1
    $process2 = Start-SeedProcess -StandardOutput $stdout2 -StandardError $stderr2
    $process1.WaitForExit()
    $process2.WaitForExit()
    if ($process1.ExitCode -ne 0 -or $process2.ExitCode -ne 0) {
        Get-Content -LiteralPath $stderr1 -ErrorAction SilentlyContinue
        Get-Content -LiteralPath $stderr2 -ErrorAction SilentlyContinue
        throw "concurrent seed exit codes were $($process1.ExitCode) and $($process2.ExitCode)."
    }

    $sql = @"
select 'active_admins=' || count(*) from users where email='$AdminEmail' and status='active' and deleted_at is null;
select 'super_admin_links=' || count(*) from user_roles ur join users u on u.id=ur.user_id join roles r on r.id=ur.role_id where u.email='$AdminEmail' and r.code='super_admin';
select 'duplicate_role_permissions=' || count(*) from (select role_id, permission_id, count(*) from role_permissions group by role_id, permission_id having count(*) > 1) duplicates;
"@
    $result = Invoke-DockerPSQL -Database $DatabaseName -SQL $sql -TuplesOnly
    $expected = @("active_admins=1", "super_admin_links=1", "duplicate_role_permissions=0")
    foreach ($value in $expected) {
        if ($result -notcontains $value) {
            throw "missing seed invariant $value; actual output: $($result -join ', ')"
        }
    }

    [pscustomobject]@{
        ok                         = $true
        database                   = $DatabaseName
        concurrent_processes       = 2
        active_admins              = 1
        super_admin_links          = 1
        duplicate_role_permissions = 0
    } | ConvertTo-Json -Depth 4
} finally {
    if ($databaseCreated) {
        Write-Host "[seed-concurrency] dropping isolated database $DatabaseName"
        try {
            [void](Invoke-DockerPSQL -Database "postgres" -SQL "drop database if exists $DatabaseName with (force);")
        } catch {
            Write-Warning $_
        }
    }
    foreach ($path in $artifacts) {
        Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue
    }
    foreach ($key in $environmentKeys) {
        [System.Environment]::SetEnvironmentVariable($key, $previousEnvironment[$key], "Process")
    }
}
