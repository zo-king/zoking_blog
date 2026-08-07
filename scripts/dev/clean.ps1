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

# Go caches and QA sandboxes created at the repository root are disposable.
$targets += Get-ChildItem -LiteralPath $RepoRoot -Directory -Force -Filter ".tmp-*" |
    ForEach-Object { $_.Name }

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
    $path = [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $relative))
    $relativePath = [System.IO.Path]::GetRelativePath($RepoRoot, $path)
    if ($relativePath -eq "." -or $relativePath.StartsWith("..$([System.IO.Path]::DirectorySeparatorChar)")) {
        throw "refusing to clean path outside repository: $path"
    }
    if (-not (Test-Path $path)) { continue }

    $tracked = git -C $RepoRoot ls-files -- $relativePath
    if ($tracked) {
        Write-Warning "skipping '$relativePath': contains git-tracked files"
        continue
    }

    $sizeMB = Get-DirectorySizeMB $path
    $totalMB += $sizeMB
    if ($DryRun) {
        Write-Host ("would remove {0,8:N1} MB  {1}" -f $sizeMB, $relativePath)
    } else {
        try {
            Remove-Item $path -Recurse -Force -Confirm:$false -ErrorAction Stop
            Write-Host ("removed {0,8:N1} MB  {1}" -f $sizeMB, $relativePath)
        } catch {
            $totalMB -= $sizeMB
            Write-Warning "skipping '$relativePath': $($_.Exception.Message)"
        }
    }
}

$verb = if ($DryRun) { "reclaimable" } else { "reclaimed" }
Write-Host ("total {0}: {1:N1} MB" -f $verb, $totalMB)
