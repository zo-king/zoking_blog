param(
    [string]$CoverageFile = "dist/qa/whitebox-cover.out"
)

$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
    throw "PowerShell 7+ is required. Use pwsh to run scripts/qa/whitebox.ps1."
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$ApiDir = Join-Path $RepoRoot "apps/api"
$coveragePath = [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $CoverageFile))
$repoPrefix = $RepoRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
if (-not $coveragePath.StartsWith($repoPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "CoverageFile must stay inside the repository: $coveragePath"
}
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $coveragePath) | Out-Null

Write-Host "[whitebox] go test -count=1 -coverprofile"
Push-Location $ApiDir
try {
    $testArguments = @("test", "-count=1", "-covermode=atomic", "-coverprofile=$coveragePath", "./...")
    & go @testArguments
    if ($LASTEXITCODE -ne 0) {
        throw "Go white-box tests failed with exit code $LASTEXITCODE."
    }

    Write-Host "[whitebox] go vet"
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go vet failed with exit code $LASTEXITCODE."
    }

    $coverageArguments = @("tool", "cover", "-func=$coveragePath")
    $coverageSummary = & go @coverageArguments
    if ($LASTEXITCODE -ne 0) {
        throw "coverage summary failed with exit code $LASTEXITCODE."
    }
} finally {
    Pop-Location
}

if (-not (Test-Path -LiteralPath $coveragePath -PathType Leaf)) {
    throw "Coverage file was not created: $coveragePath"
}

$totalLine = $coverageSummary | Select-Object -Last 1
Write-Host "[whitebox] $totalLine"

[pscustomobject]@{
    ok = $true
    coverage_file = $coveragePath
    total = [string]$totalLine
    cache_disabled = $true
    vet = "passed"
} | ConvertTo-Json -Depth 4
