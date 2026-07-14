param(
    [string]$ApiBase = "http://localhost:18080",
    [string]$AdminEmail = "admin@zoking.local",
    [string]$AdminPassword = "ChangeMe123!",
    [int]$PublishTimeoutSeconds = 180,
    [switch]$SkipE2E,
    [switch]$SkipRollback,
    [switch]$Install,
    [switch]$StartApi,
    [switch]$BootstrapTestData,
    [switch]$StopStartedApi
)

$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
    throw "PowerShell 7+ is required. Use pwsh to run scripts/qa/preflight.ps1."
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$ApiDir = Join-Path $RepoRoot "apps/api"
$AdminDir = Join-Path $RepoRoot "apps/admin"
$SiteDir = Join-Path $RepoRoot "apps/site"
$PreflightSiteDir = Join-Path $RepoRoot "dist/preflight-site"
$PreflightRuntimeRoot = Join-Path $RepoRoot "storage/qa/preflight-runtime"
$ManagedHugoSiteDir = Join-Path $PreflightRuntimeRoot "site"
$ManagedHugoPublicDir = Join-Path $PreflightRuntimeRoot "public"
$ManagedReleaseRoot = Join-Path $PreflightRuntimeRoot "releases"
$ManagedPreviewRoot = Join-Path $PreflightRuntimeRoot "previews"
$ManagedMediaDir = Join-Path $PreflightRuntimeRoot "media"
$PreflightLogDir = Join-Path $PreflightRuntimeRoot "logs"
$ManagedApiBinDir = Join-Path $PreflightRuntimeRoot "bin"
$ManagedApiBinary = Join-Path $ManagedApiBinDir $(if ($IsWindows) { "zoking-api.exe" } else { "zoking-api" })
$script:StartedApiProcess = $null
$script:ManagedRuntimeInitialized = $false
$script:SmokeHugoSiteDir = $SiteDir
$script:PreflightSucceeded = $false
$script:AdminWebSession = $null

function Write-Step {
    param([string]$Message)
    Write-Host "[preflight] $Message"
}

function Invoke-CheckedCommand {
    param(
        [string]$Name,
        [string]$WorkingDirectory,
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    Write-Step $Name
    Push-Location $WorkingDirectory
    try {
        & $FilePath @Arguments
        $exitCode = $LASTEXITCODE
        if ($null -ne $exitCode -and $exitCode -ne 0) {
            throw "$Name failed with exit code $exitCode"
        }
    } finally {
        Pop-Location
    }
}

function Test-ApiReady {
    try {
        $ready = Invoke-RestMethod -Method GET -Uri "$ApiBase/readyz" -TimeoutSec 5
        return $ready.data.status -eq "ready"
    } catch {
        return $false
    }
}

function Test-ApiResponding {
    try {
        $health = Invoke-RestMethod -Method GET -Uri "$ApiBase/healthz" -TimeoutSec 5
        return $health.data.status -eq "ok"
    } catch {
        return $false
    }
}

function Wait-ApiReady {
    param([int]$TimeoutSeconds = 60)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-ApiReady) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "API did not become ready at $ApiBase within $TimeoutSeconds seconds"
}

function Assert-TestDatabaseTarget {
    param([string]$Operation)

    $appEnv = [string]$env:APP_ENV
    if ($appEnv -eq "development") {
        throw "Refusing $Operation with APP_ENV=development. Preflight-managed API and database writes require APP_ENV=test."
    }
    if ($appEnv -ne "test") {
        throw "Refusing $Operation with APP_ENV=$appEnv. Set APP_ENV=test explicitly."
    }

    $databaseURL = [string]$env:DATABASE_URL
    if ([string]::IsNullOrWhiteSpace($databaseURL)) {
        throw "Refusing $Operation without DATABASE_URL. Use an explicit PostgreSQL test database URL."
    }

    try {
        $databaseUri = [System.Uri]::new($databaseURL)
    } catch {
        throw "Refusing $Operation because DATABASE_URL is not a valid absolute PostgreSQL URL."
    }
    if (-not $databaseUri.IsAbsoluteUri -or $databaseUri.Scheme -notin @("postgres", "postgresql")) {
        throw "Refusing $Operation because DATABASE_URL is not a valid absolute PostgreSQL URL."
    }

    $databaseName = [System.Uri]::UnescapeDataString($databaseUri.AbsolutePath.Trim([char]"/"))
    if ([string]::IsNullOrWhiteSpace($databaseName) -or $databaseName.Contains("/")) {
        throw "Refusing $Operation because DATABASE_URL does not contain one database name."
    }
    if (-not $databaseName.EndsWith("_test", [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing $Operation against database '$databaseName'. Test database names must end with _test."
    }

    return $databaseName
}

function Get-ManagedApiPort {
    try {
        $uri = [System.Uri]::new($ApiBase)
    } catch {
        throw "-StartApi requires ApiBase to be an absolute loopback HTTP URL."
    }
    if (-not $uri.IsAbsoluteUri -or $uri.Scheme -ne "http" -or $uri.Host -notin @("localhost", "127.0.0.1", "::1")) {
        throw "-StartApi requires ApiBase to be an absolute loopback HTTP URL."
    }
    if ($uri.AbsolutePath -ne "/" -or -not [string]::IsNullOrEmpty($uri.Query) -or -not [string]::IsNullOrEmpty($uri.Fragment)) {
        throw "-StartApi requires ApiBase without a path, query, or fragment."
    }
    if ($uri.Port -lt 1 -or $uri.Port -gt 65535) {
        throw "-StartApi could not derive a valid APP_PORT from ApiBase."
    }
    return [string]$uri.Port
}

function Assert-PreflightRuntimePath {
    param([string]$Path)

    $qaRoot = [System.IO.Path]::GetFullPath((Join-Path $RepoRoot "storage/qa"))
    $target = [System.IO.Path]::GetFullPath($Path)
    $qaPrefix = $qaRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    if ($target -eq $qaRoot -or -not $target.StartsWith($qaPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "refusing to manage preflight path outside storage/qa: $target"
    }
    return $target
}

function Remove-PreflightRuntime {
    $target = Assert-PreflightRuntimePath -Path $PreflightRuntimeRoot
    $lastError = $null
    for ($attempt = 1; $attempt -le 20; $attempt++) {
        try {
            if (Test-Path -LiteralPath $target) {
                Remove-Item -LiteralPath $target -Recurse -Force
            }
            $script:ManagedRuntimeInitialized = $false
            return
        } catch {
            $lastError = $_
            if ($attempt -lt 20) {
                Start-Sleep -Milliseconds 250
            }
        }
    }
    throw $lastError
}

function Initialize-PreflightRuntime {
    Write-Step "creating isolated preflight runtime at $PreflightRuntimeRoot"
    Remove-PreflightRuntime
    $runtimeRoot = Assert-PreflightRuntimePath -Path $PreflightRuntimeRoot
    New-Item -ItemType Directory -Force -Path $runtimeRoot, $ManagedHugoSiteDir, $ManagedHugoPublicDir, $ManagedReleaseRoot, $ManagedPreviewRoot, $ManagedMediaDir, $PreflightLogDir, $ManagedApiBinDir | Out-Null
    $script:ManagedRuntimeInitialized = $true

    $excludedNames = @("public", "resources", "dist", ".hugo_build.lock")
    foreach ($item in @(Get-ChildItem -LiteralPath $SiteDir -Force)) {
        if ($item.Name -in $excludedNames) {
            continue
        }
        Copy-Item -LiteralPath $item.FullName -Destination $ManagedHugoSiteDir -Recurse -Force
    }

    $runtimeGoMod = Join-Path $ManagedHugoSiteDir "go.mod"
    if (-not (Test-Path -LiteralPath $runtimeGoMod -PathType Leaf)) {
        throw "isolated Hugo site is missing go.mod: $runtimeGoMod"
    }
    $repoModulePath = $RepoRoot.Replace("\", "/")
    Invoke-CheckedCommand `
        -Name "Bind isolated Hugo module to repository theme" `
        -WorkingDirectory $ManagedHugoSiteDir `
        -FilePath "go" `
        -Arguments @("mod", "edit", "-replace=github.com/CaiJimmy/hugo-theme-stack/v4=$repoModulePath")

    $script:SmokeHugoSiteDir = $ManagedHugoSiteDir
}

function Assert-E2ESmokeScriptHasManifestCleanup {
    $smokePath = Join-Path $RepoRoot "scripts/qa/e2e-smoke.ps1"
    if (-not (Test-Path -LiteralPath $smokePath)) {
        throw "E2E smoke script not found: $smokePath"
    }

    $tokens = $null
    $parseErrors = $null
    [System.Management.Automation.Language.Parser]::ParseFile($smokePath, [ref]$tokens, [ref]$parseErrors) | Out-Null
    if ($parseErrors.Count -gt 0) {
        $messages = $parseErrors | ForEach-Object { "$($_.Extent.StartLineNumber):$($_.Extent.StartColumnNumber) $($_.Message)" }
        throw "E2E smoke script has parser errors:`n$($messages -join "`n")"
    }

    $raw = Get-Content -Raw -LiteralPath $smokePath
    $requiredPatterns = @(
        @{ Name = "manifest run id"; Pattern = "(?is)\bmanifest\b.*\brun_?id\b|\brun_?id\b.*\bmanifest\b" },
        @{ Name = "QA E2E cleanup route"; Pattern = "/api/v1/admin/qa/e2e-runs/" },
        @{ Name = "finally cleanup block"; Pattern = "(?im)\bfinally\b" }
    )
    foreach ($required in $requiredPatterns) {
        if ($raw -notmatch $required.Pattern) {
            throw "Refusing to run $smokePath because it does not contain $($required.Name). Preflight only runs manifest-cleaned E2E smoke."
        }
    }

    $forbiddenPatterns = @(
        @{ Name = "seed command"; Pattern = "(?im)\bcmd/seed\b|\.\\cmd\\seed\b" },
        @{ Name = "polluting rollback baseline guidance"; Pattern = "(?i)run once with -SkipRollback to create a baseline release|bootstrapping rollback baseline" }
    )
    foreach ($forbidden in $forbiddenPatterns) {
        if ($raw -match $forbidden.Pattern) {
            throw "Refusing to run $smokePath because it contains $($forbidden.Name)."
        }
    }
}

function Assert-QAE2ECleanupRouteAvailable {
    $headers = Get-AdminHeaders
    $probe = Invoke-WebRequest `
        -Method POST `
        -Uri "$ApiBase/api/v1/admin/qa/e2e-runs/not-a-uuid/cleanup" `
        -Headers $headers `
		-WebSession $script:AdminWebSession `
        -ContentType "application/json" `
        -Body "{}" `
        -SkipHttpErrorCheck
    $statusCode = [int]$probe.StatusCode
    if ($statusCode -eq 422) {
        Write-Step "QA E2E cleanup route is available"
        return
    }
    throw "QA E2E cleanup route is not available for preflight at $ApiBase (status $statusCode). Ensure the API uses APP_ENV=test and QA_E2E_CLEANUP_ENABLED=true."
}

function Get-DescendantProcessIDs {
    param([int]$ParentID)

    $ids = New-Object System.Collections.Generic.List[int]
    if ($IsWindows) {
        $children = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $_.ParentProcessId -eq $ParentID })
        foreach ($child in $children) {
            $childID = [int]$child.ProcessId
            $ids.Add($childID)
            foreach ($descendantID in (Get-DescendantProcessIDs -ParentID $childID)) {
                $ids.Add([int]$descendantID)
            }
        }
        return $ids
    }

    if (Get-Command pgrep -ErrorAction SilentlyContinue) {
        $children = @(& pgrep -P $ParentID 2>$null)
        foreach ($child in $children) {
            if ($child -match "^\d+$") {
                $childID = [int]$child
                $ids.Add($childID)
                foreach ($descendantID in (Get-DescendantProcessIDs -ParentID $childID)) {
                    $ids.Add([int]$descendantID)
                }
            }
        }
    }
    return $ids
}

function Start-ManagedApi {
    if (Test-ApiResponding) {
        throw "An API is already responding at $ApiBase. -StartApi requires an unused loopback endpoint."
    }

    $managedDatabaseName = Assert-TestDatabaseTarget -Operation "starting the preflight API"
    $managedPort = Get-ManagedApiPort
    Initialize-PreflightRuntime
    Invoke-CheckedCommand `
        -Name "Build isolated test API" `
        -WorkingDirectory $ApiDir `
        -FilePath "go" `
        -Arguments @("build", "-o", $ManagedApiBinary, "./cmd/api")

    New-Item -ItemType Directory -Force -Path $PreflightLogDir | Out-Null
    $stdoutPath = Join-Path $PreflightLogDir "preflight-api.stdout.log"
    $stderrPath = Join-Path $PreflightLogDir "preflight-api.stderr.log"

    Write-Step "starting isolated test API on port $managedPort with database=$managedDatabaseName"
    $environmentOverrides = [ordered]@{
        APP_ENV = "test"
        APP_PORT = $managedPort
        QA_E2E_CLEANUP_ENABLED = "true"
        HUGO_SITE_DIR = $ManagedHugoSiteDir
        HUGO_PUBLIC_DIR = $ManagedHugoPublicDir
        HUGO_BIN = (Resolve-HugoBinary)
        PUBLISH_RELEASE_ROOT = $ManagedReleaseRoot
        PUBLISH_CURRENT_DIR = $ManagedHugoPublicDir
        PUBLISH_PREVIEW_ROOT = $ManagedPreviewRoot
        MEDIA_LOCAL_DIR = $ManagedMediaDir
        PUBLISH_WORKER_ENABLED = "true"
        SEED_ADMIN_EMAIL = $AdminEmail
        SEED_ADMIN_PASSWORD = $AdminPassword
    }
    $previousEnvironment = @{}
    foreach ($key in $environmentOverrides.Keys) {
        $previousEnvironment[$key] = [System.Environment]::GetEnvironmentVariable($key, "Process")
    }
    $startArgs = @{
        FilePath               = $ManagedApiBinary
        WorkingDirectory       = $ApiDir
        RedirectStandardOutput = $stdoutPath
        RedirectStandardError  = $stderrPath
        PassThru               = $true
    }
    if ($IsWindows) {
        $startArgs.WindowStyle = "Hidden"
    }
    try {
        foreach ($key in $environmentOverrides.Keys) {
            [System.Environment]::SetEnvironmentVariable($key, [string]$environmentOverrides[$key], "Process")
        }
        $script:StartedApiProcess = Start-Process @startArgs
        Wait-ApiReady -TimeoutSeconds 90
    } catch {
        if (Test-Path $stderrPath) {
            Get-Content -Tail 120 -LiteralPath $stderrPath
        }
        if (Test-Path $stdoutPath) {
            Get-Content -Tail 120 -LiteralPath $stdoutPath
        }
        throw
    } finally {
        foreach ($key in $environmentOverrides.Keys) {
            [System.Environment]::SetEnvironmentVariable($key, $previousEnvironment[$key], "Process")
        }
    }
    return $true
}

function Stop-ManagedApi {
    param([switch]$DueToFailure)

    if ($null -eq $script:StartedApiProcess) {
        return
    }
    if (-not $StopStartedApi -and -not $DueToFailure) {
        return
    }

    Write-Step "stopping API started by preflight"
    $processIDs = @()
    $processIDs += @(Get-DescendantProcessIDs -ParentID $script:StartedApiProcess.Id)
    $processIDs += @($script:StartedApiProcess.Id)
    $processIDs = @($processIDs | Select-Object -Unique)
    foreach ($processID in $processIDs) {
        $process = Get-Process -Id $processID -ErrorAction SilentlyContinue
        if ($process) {
            Stop-Process -Id $processID -Force -ErrorAction SilentlyContinue
        }
    }
    foreach ($processID in $processIDs) {
        Wait-Process -Id $processID -Timeout 10 -ErrorAction SilentlyContinue
    }
    try {
        [void]$script:StartedApiProcess.WaitForExit(10000)
    } catch {
    }
    $script:StartedApiProcess.Dispose()
    $script:StartedApiProcess = $null
    Start-Sleep -Milliseconds 250
}

function Remove-PreflightSiteDir {
    $repoFull = [System.IO.Path]::GetFullPath($RepoRoot)
    $targetFull = [System.IO.Path]::GetFullPath($PreflightSiteDir)
    $repoPrefix = $repoFull.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    if (-not $targetFull.StartsWith($repoPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "refusing to remove path outside repo: $targetFull"
    }
    if ($targetFull -eq $repoFull) {
        throw "refusing to remove repository root"
    }
    if (Test-Path -LiteralPath $targetFull) {
        Remove-Item -LiteralPath $targetFull -Recurse -Force
    }
}

function Resolve-HugoBinary {
    if ($env:HUGO_BIN -and (Test-Path -LiteralPath $env:HUGO_BIN)) {
        return (Resolve-Path -LiteralPath $env:HUGO_BIN).Path
    }

    $bundled = Join-Path $RepoRoot ".tools/hugo/hugo.exe"
    if (Test-Path -LiteralPath $bundled) {
        return (Resolve-Path -LiteralPath $bundled).Path
    }

    $command = Get-Command hugo -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    throw "Hugo binary not found. Install Hugo Extended or set HUGO_BIN."
}

function Get-AdminHeaders {
    $login = Invoke-RestMethod `
        -Method POST `
        -Uri "$ApiBase/api/v1/admin/auth/login" `
		-SessionVariable AdminWebSession `
        -ContentType "application/json" `
        -Body (@{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json)
	$script:AdminWebSession = $AdminWebSession
	return @{ "X-CSRF-Token" = [string]$login.data.csrf_token }
}

function Get-ActivePublishRelease {
    param([hashtable]$Headers)

	$releases = Invoke-RestMethod `
        -Method GET `
        -Uri "$ApiBase/api/v1/admin/publish/releases" `
		-Headers $Headers `
		-WebSession $script:AdminWebSession
    $activeReleases = @($releases.data | Where-Object { [bool]$_.is_active })
    if ($activeReleases.Count -gt 1) {
        throw "Expected at most one active release, found $($activeReleases.Count)."
    }
    if ($activeReleases.Count -eq 0) {
        return $null
    }
    return $activeReleases[0]
}

function Wait-PublishJobPublished {
    param(
        [string]$JobID,
        [hashtable]$Headers,
        [int]$TimeoutSeconds
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastStatus = "not-found"
    while ((Get-Date) -lt $deadline) {
		$payload = Invoke-RestMethod `
            -Method GET `
            -Uri "$ApiBase/api/v1/admin/publish/jobs/$JobID" `
			-Headers $Headers `
			-WebSession $script:AdminWebSession
        $job = $payload.data
        if ($null -ne $job) {
            $lastStatus = [string]$job.status
            if ($lastStatus -eq "published") {
                return $job
            }
            if ($lastStatus -in @("failed", "canceled")) {
                throw "baseline publish job $JobID ended with status=$lastStatus, error_code=$($job.error_code), error_message=$($job.error_message)"
            }
        }
        Start-Sleep -Milliseconds 750
    }

    throw "baseline publish job $JobID did not reach published within $TimeoutSeconds seconds; last status=$lastStatus"
}

function Invoke-TestDatabaseSeed {
    [void](Assert-TestDatabaseTarget -Operation "test database seed")
    $keys = @("SEED_ADMIN_EMAIL", "SEED_ADMIN_PASSWORD")
    $previous = @{}
    foreach ($key in $keys) {
        $previous[$key] = [System.Environment]::GetEnvironmentVariable($key, "Process")
    }
    try {
        [System.Environment]::SetEnvironmentVariable("SEED_ADMIN_EMAIL", $AdminEmail, "Process")
        [System.Environment]::SetEnvironmentVariable("SEED_ADMIN_PASSWORD", $AdminPassword, "Process")
        Invoke-CheckedCommand -Name "Test database seed" -WorkingDirectory $ApiDir -FilePath "go" -Arguments @("run", "./cmd/seed")
    } finally {
        foreach ($key in $keys) {
            [System.Environment]::SetEnvironmentVariable($key, $previous[$key], "Process")
        }
    }
}

function Ensure-TestBaselineRelease {
    [void](Assert-TestDatabaseTarget -Operation "test baseline bootstrap")
    Write-Step "checking for an active test baseline release"
    $headers = Get-AdminHeaders
    $activeRelease = Get-ActivePublishRelease -Headers $headers
    if ($null -ne $activeRelease) {
        $expectedPath = [System.IO.Path]::GetFullPath((Join-Path $ManagedReleaseRoot ([string]$activeRelease.release_key)))
        $recordedPath = [System.IO.Path]::GetFullPath([string]$activeRelease.output_path)
        $manifestPath = Join-Path $expectedPath "manifest.json"
        if ($recordedPath -eq $expectedPath -and (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
            Write-Step "active release $($activeRelease.release_key) is present in the isolated runtime; baseline publish skipped"
            return
        }
        Write-Step "active release $($activeRelease.release_key) has no usable isolated output; publishing a fresh baseline"
    }

    Write-Step "creating a dedicated baseline through the Admin settings publish API"
    $response = Invoke-WebRequest `
        -Method POST `
        -Uri "$ApiBase/api/v1/admin/settings/publish" `
        -Headers $headers `
		-WebSession $script:AdminWebSession `
        -ContentType "application/json" `
        -Body "{}"
    if ([int]$response.StatusCode -ne 202) {
        throw "settings baseline publish returned HTTP $([int]$response.StatusCode), expected 202"
    }

    $payload = $response.Content | ConvertFrom-Json
    $jobID = [string]$payload.data.job.id
    if ([string]::IsNullOrWhiteSpace($jobID)) {
        throw "settings baseline publish response did not contain a job id"
    }

    Write-Step "waiting for baseline publish job $jobID"
    $job = Wait-PublishJobPublished -JobID $jobID -Headers $headers -TimeoutSeconds $PublishTimeoutSeconds
    $activeRelease = Get-ActivePublishRelease -Headers $headers
    if ($null -eq $activeRelease -or [string]($activeRelease.job_id) -ne [string]($job.id)) {
        throw "baseline publish job $jobID reached published but did not create the active release"
    }
    Write-Step "baseline release $($activeRelease.release_key) is published and active"
}

function Invoke-E2ESmoke {
    param([bool]$RunSkipRollback)

    Assert-E2ESmokeScriptHasManifestCleanup
    $args = @(
        "-NoProfile",
        "-File", (Join-Path $RepoRoot "scripts/qa/e2e-smoke.ps1"),
        "-ApiBase", $ApiBase,
        "-AdminEmail", $AdminEmail,
        "-AdminPassword", $AdminPassword,
        "-HugoSiteDir", $script:SmokeHugoSiteDir,
        "-PublishTimeoutSeconds", [string]$PublishTimeoutSeconds,
        "-RestoreSettings"
    )
    if ($RunSkipRollback) {
        $args += "-SkipRollback"
    }
    Invoke-CheckedCommand `
        -Name ($(if ($RunSkipRollback) { "E2E smoke with manifest cleanup (-SkipRollback)" } else { "E2E smoke with manifest cleanup" })) `
        -WorkingDirectory $RepoRoot `
        -FilePath "pwsh" `
        -Arguments $args
}

try {
    Write-Step "repo root: $RepoRoot"

    if ($BootstrapTestData -and -not $StartApi) {
        throw "-BootstrapTestData requires -StartApi. Preflight never seeds or bootstraps a reused API."
    }
    if ($BootstrapTestData -and $SkipE2E) {
        throw "-BootstrapTestData cannot be combined with -SkipE2E."
    }
    if ($StartApi) {
        [void](Assert-TestDatabaseTarget -Operation "using -StartApi")
        [void](Get-ManagedApiPort)
    }

    Invoke-CheckedCommand -Name "Go tests" -WorkingDirectory $ApiDir -FilePath "go" -Arguments @("test", "./...")

    if ($Install -or -not (Test-Path -LiteralPath (Join-Path $AdminDir "node_modules"))) {
        Invoke-CheckedCommand -Name "Admin dependencies" -WorkingDirectory $AdminDir -FilePath "npm" -Arguments @("ci")
    }
    Invoke-CheckedCommand -Name "Admin production build" -WorkingDirectory $AdminDir -FilePath "npm" -Arguments @("run", "build")

    Remove-PreflightSiteDir
    $hugo = Resolve-HugoBinary
    $previousCommentsAPI = $env:HUGO_COMMENTS_API_BASE
    try {
        $env:HUGO_COMMENTS_API_BASE = $ApiBase
        Invoke-CheckedCommand `
            -Name "Hugo production build" `
            -WorkingDirectory $RepoRoot `
            -FilePath $hugo `
            -Arguments @("--source", $SiteDir, "--destination", $PreflightSiteDir, "--minify")
    } finally {
        $env:HUGO_COMMENTS_API_BASE = $previousCommentsAPI
    }

    if (-not $SkipE2E) {
        Assert-E2ESmokeScriptHasManifestCleanup
        if ($StartApi) {
            if (Test-ApiResponding) {
                throw "An API is already responding at $ApiBase. Choose an unused loopback port for -StartApi."
            }
            [void](Assert-TestDatabaseTarget -Operation "database migrations")
            Invoke-CheckedCommand -Name "Test database migrations" -WorkingDirectory $ApiDir -FilePath "go" -Arguments @("run", "./cmd/migrate", "up")

            if ($BootstrapTestData) {
                Invoke-TestDatabaseSeed
            }

            [void](Start-ManagedApi)
            if ($BootstrapTestData) {
                Ensure-TestBaselineRelease
            }
        } elseif (Test-ApiReady) {
            Write-Step "reusing the ready test API at $ApiBase without migration, seed, or baseline bootstrap"
        } elseif (Test-ApiResponding) {
            throw "A test API is responding at $ApiBase but is not ready."
        } else {
            throw "Test API is not ready at $ApiBase. Start a test API first or rerun with -StartApi."
        }

        Assert-QAE2ECleanupRouteAvailable
        Invoke-E2ESmoke -RunSkipRollback ([bool]$SkipRollback)
    } else {
        Write-Step "E2E smoke skipped"
    }

    Write-Step "completed successfully"
    $script:PreflightSucceeded = $true
} finally {
    $shouldRemoveRuntime = $script:ManagedRuntimeInitialized -and ((-not $script:PreflightSucceeded) -or $StopStartedApi)
    Stop-ManagedApi -DueToFailure:(-not $script:PreflightSucceeded)
    if ($shouldRemoveRuntime) {
        Write-Step "removing isolated preflight runtime"
        Remove-PreflightRuntime
    } elseif ($script:ManagedRuntimeInitialized) {
        Write-Step "isolated preflight runtime retained because the managed API is still running: $PreflightRuntimeRoot"
    }
}

# Native cleanup helpers such as pgrep may leave a non-zero LASTEXITCODE even
# when the full preflight and its finally block succeeded.
if ($script:PreflightSucceeded) {
    exit 0
}
