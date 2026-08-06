# Removes local build artifacts and QA outputs that accumulate over time.
# All targets are gitignored; the script refuses to delete anything git tracks.
param(
    [switch]$DryRun,
    [switch]$IncludeMedia
)

$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path

$targets = @(
    "dist",
    "apps/site/dist",
    "apps/site/public",
    "apps/site/resources",
    "apps/site/storage",
    "apps/site/.hugo_build.lock",
    "apps/api/api-dev.log",
    "storage/qa",
    "storage/logs"
)

if ($IncludeMedia) {
    $targets += "storage/media"
}

function Get-DirectorySizeMB {
    param([string]$Path)
    if (Test-Path $Path -PathType Container) {
        $bytes = (Get-ChildItem $Path -Recurse -File -Force -ErrorAction SilentlyContinue | Measure-Object Length -Sum).Sum
    } else {
        $bytes = (Get-Item $Path -Force).Length
    }
    return [math]::Round(($bytes ?? 0) / 1MB, 1)
}

$totalMB = 0.0
foreach ($relative in $targets) {
    $path = Join-Path $RepoRoot $relative
    if (-not (Test-Path $path)) { continue }

    $tracked = git -C $RepoRoot ls-files -- $relative
    if ($tracked) {
        Write-Warning "skipping '$relative': contains git-tracked files"
        continue
    }

    $sizeMB = Get-DirectorySizeMB $path
    $totalMB += $sizeMB
    if ($DryRun) {
        Write-Host ("would remove {0,8:N1} MB  {1}" -f $sizeMB, $relative)
    } else {
        Remove-Item $path -Recurse -Force -Confirm:$false
        Write-Host ("removed {0,8:N1} MB  {1}" -f $sizeMB, $relative)
    }
}

$verb = if ($DryRun) { "reclaimable" } else { "reclaimed" }
Write-Host ("total {0}: {1:N1} MB" -f $verb, $totalMB)
