param(
    [string]$ApiBase = "http://localhost:18080",
    [string]$AdminEmail = "admin@zoking.local",
    [string]$AdminPassword = "ChangeMe123!",
    [string]$HugoSiteDir = "",
    [int]$PublishTimeoutSeconds = 60,
    [switch]$SkipRollback,
    [switch]$RestoreSettings,
    [switch]$SkipE2ECleanup
)

$ErrorActionPreference = "Stop"
$script:E2EJournalPath = $null
$script:AdminWebSession = $null

if ($PSVersionTable.PSVersion.Major -lt 7) {
    throw "PowerShell 7+ is required because the script uses Invoke-RestMethod -Form for multipart upload."
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if ([string]::IsNullOrWhiteSpace($HugoSiteDir)) {
    $HugoSiteDir = Join-Path $RepoRoot "apps/site"
}
if (-not (Test-Path -LiteralPath $HugoSiteDir -PathType Container)) {
    throw "Hugo site directory does not exist: $HugoSiteDir"
}
$HugoSiteDir = (Resolve-Path -LiteralPath $HugoSiteDir).Path

function Write-Step {
    param([string]$Message)
    Write-Host "[smoke] $Message"
}

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST", "PATCH", "DELETE")]
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null
    )

    $parameters = @{
        Method = $Method
        Uri = $Uri
        Headers = $Headers
    }
	if ($null -ne $script:AdminWebSession) {
		$parameters.WebSession = $script:AdminWebSession
	}
    if ($null -ne $Body) {
        $parameters.ContentType = "application/json"
        $parameters.Body = ($Body | ConvertTo-Json -Depth 12)
    }
    Invoke-RestMethod @parameters
}

function Assert-True {
    param(
        [bool]$Condition,
        [string]$Message
    )
    if (-not $Condition) {
        throw $Message
    }
}

function New-E2ERunJournal {
    param([string]$RunID)

    return [pscustomobject]@{
        schema_version = 1
        run_id = $RunID
        cleanup_completed = $false
        baseline_release_id = $null
        settings_before = $null
        posts = [System.Collections.Generic.List[object]]::new()
        pages = [System.Collections.Generic.List[object]]::new()
        categories = [System.Collections.Generic.List[object]]::new()
        tags = [System.Collections.Generic.List[object]]::new()
        comments = [System.Collections.Generic.List[object]]::new()
        media = [System.Collections.Generic.List[object]]::new()
        previews = [System.Collections.Generic.List[object]]::new()
        jobs = [System.Collections.Generic.List[object]]::new()
        releases = [System.Collections.Generic.List[object]]::new()
    }
}

function Get-E2EJournalDirectory {
    return Join-Path $RepoRoot "storage/qa/e2e-runs"
}

function Save-E2EJournal {
    param([object]$Journal)

    if (-not $script:E2EJournalPath -or $null -eq $Journal) {
        return
    }
    $directory = Split-Path -Parent $script:E2EJournalPath
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    $temporaryPath = "$($script:E2EJournalPath).tmp-$([guid]::NewGuid())"
    try {
        $json = $Journal | ConvertTo-Json -Depth 12
        [System.IO.File]::WriteAllText($temporaryPath, $json, [System.Text.UTF8Encoding]::new($false))
        [System.IO.File]::Move($temporaryPath, $script:E2EJournalPath, $true)
    } finally {
        if (Test-Path -LiteralPath $temporaryPath) {
            Remove-Item -LiteralPath $temporaryPath -Force
        }
    }
}

function Test-JournalHasID {
    param(
        [object]$Journal,
        [string]$Collection,
        [string]$ID
    )

    foreach ($item in $Journal.$Collection) {
        if ([string]$item.id -eq $ID) {
            return $true
        }
    }
    return $false
}

function Add-E2ESlugRef {
    param(
        [object]$Journal,
        [string]$Collection,
        [string]$ID,
        [string]$Slug
    )

    if ([string]::IsNullOrWhiteSpace($ID) -or [string]::IsNullOrWhiteSpace($Slug)) {
        return
    }
    if (Test-JournalHasID -Journal $Journal -Collection $Collection -ID $ID) {
        return
    }
    $Journal.$Collection.Add([pscustomobject]@{
        id = $ID
        slug = $Slug
    }) | Out-Null
    Save-E2EJournal -Journal $Journal
}

function Add-E2EIDRef {
    param(
        [object]$Journal,
        [string]$Collection,
        [string]$ID
    )

    if ([string]::IsNullOrWhiteSpace($ID)) {
        return
    }
    if (Test-JournalHasID -Journal $Journal -Collection $Collection -ID $ID) {
        return
    }
    $Journal.$Collection.Add([pscustomobject]@{
        id = $ID
    }) | Out-Null
    Save-E2EJournal -Journal $Journal
}

function Add-E2EKeyRef {
    param(
        [object]$Journal,
        [string]$Collection,
        [string]$ID,
        [string]$Key
    )

    if ([string]::IsNullOrWhiteSpace($ID) -or [string]::IsNullOrWhiteSpace($Key)) {
        return
    }
    if (Test-JournalHasID -Journal $Journal -Collection $Collection -ID $ID) {
        return
    }
    $Journal.$Collection.Add([pscustomobject]@{
        id = $ID
        key = $Key
    }) | Out-Null
    Save-E2EJournal -Journal $Journal
}

function Add-E2EPreviewRef {
    param(
        [object]$Journal,
        [string]$PreviewKey,
        [hashtable]$Headers
    )

    if ([string]::IsNullOrWhiteSpace($PreviewKey)) {
        return
    }
    $previews = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/previews" -Headers $Headers
    $preview = @($previews.data | Where-Object { $_.preview_key -eq $PreviewKey })[0]
    Assert-True ($null -ne $preview) "preview $PreviewKey was not found for cleanup journal"
    Add-E2EKeyRef -Journal $Journal -Collection "previews" -ID ([string]$preview.id) -Key ([string]$preview.preview_key)
}

function Invoke-E2EPreview {
    param(
        [string]$Uri,
        [hashtable]$Headers,
        [object]$Journal,
        [object]$Body = $null
    )

    $before = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/previews" -Headers $Headers
    $beforeIDs = @{}
    foreach ($preview in @($before.data)) {
        $beforeIDs[[string]$preview.id] = $true
    }

    try {
        if ($null -ne $Body) {
            $result = Invoke-Json -Method POST -Uri $Uri -Headers $Headers -Body $Body
        } else {
            $result = Invoke-Json -Method POST -Uri $Uri -Headers $Headers
        }
        Add-E2EPreviewRef -Journal $Journal -PreviewKey ([string]$result.data.preview_key) -Headers $Headers
        return $result
    } catch {
        $after = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/previews" -Headers $Headers
        foreach ($preview in @($after.data | Where-Object { -not $beforeIDs.ContainsKey([string]$_.id) })) {
            Add-E2EKeyRef -Journal $Journal -Collection "previews" -ID ([string]$preview.id) -Key ([string]$preview.preview_key)
        }
        throw
    }
}

function Get-RequiredCount {
    param(
        [object]$Object,
        [string]$Name
    )

    if ($null -eq $Object) {
        throw "cleanup response is missing $Name"
    }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property -or $null -eq $property.Value) {
        throw "cleanup response is missing count field $Name"
    }
    $count = 0
    if (-not [int]::TryParse([string]$property.Value, [ref]$count) -or $count -lt 0) {
        throw "cleanup response count field $Name is invalid"
    }
    return $count
}

function Assert-CleanupDeletedMatchesCandidates {
    param(
        [object]$DryRun,
        [object]$Apply
    )

    foreach ($name in @("posts", "pages", "categories", "tags", "comments", "media", "previews", "jobs", "releases")) {
        $candidateCount = Get-RequiredCount -Object $DryRun.candidates -Name $name
        $applyCandidateCount = Get-RequiredCount -Object $Apply.candidates -Name $name
        $deletedCount = Get-RequiredCount -Object $Apply.deleted -Name $name
        Assert-True ($applyCandidateCount -eq $candidateCount) "E2E cleanup apply candidates for $name ($applyCandidateCount) did not match dry-run candidates ($candidateCount)"
        Assert-True ($deletedCount -eq $candidateCount) "E2E cleanup deleted count for $name ($deletedCount) did not match dry-run candidates ($candidateCount)"
    }
}

function Wait-E2EJournalJobsTerminal {
    param(
        [string]$ApiBase,
        [hashtable]$Headers,
        [object]$Manifest,
        [int]$TimeoutSeconds
    )

    $jobIDs = @($Manifest.jobs | ForEach-Object { [string]$_.id } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($jobIDs.Count -eq 0) {
        return
    }

    $activeStatuses = @("requested", "queued", "snapshotting", "building", "verifying", "promoting")
    $deadline = (Get-Date).AddSeconds([Math]::Max(5, $TimeoutSeconds))
    while ((Get-Date) -lt $deadline) {
        $jobs = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/jobs" -Headers $Headers
        $active = @()
        foreach ($jobID in $jobIDs) {
            $job = @($jobs.data | Where-Object { $_.id -eq $jobID })[0]
            if ($job -and $activeStatuses -contains [string]$job.status) {
                $active += "${jobID}:$($job.status)"
            }
        }
        if ($active.Count -eq 0) {
            return
        }
        Start-Sleep -Milliseconds 750
    }

    throw "E2E cleanup cannot run while journaled publish jobs are still active"
}

function Sync-E2EJournalReleases {
    param(
        [string]$ApiBase,
        [hashtable]$Headers,
        [object]$Manifest
    )

    $jobIDs = @($Manifest.jobs | ForEach-Object { [string]$_.id } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($jobIDs.Count -eq 0) {
        return
    }
    $jobSet = @{}
    foreach ($jobID in $jobIDs) {
        $jobSet[$jobID] = $true
    }

    $releases = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $Headers
    foreach ($release in @($releases.data | Where-Object { $jobSet.ContainsKey([string]$_.job_id) })) {
        Add-E2EKeyRef -Journal $Manifest -Collection "releases" -ID ([string]$release.id) -Key ([string]$release.release_key)
    }
}

function Invoke-E2ERunCleanup {
    param(
        [string]$ApiBase,
        [hashtable]$Headers,
        [string]$RunID,
        [object]$Manifest,
        [int]$TimeoutSeconds
    )

    Wait-E2EJournalJobsTerminal -ApiBase $ApiBase -Headers $Headers -Manifest $Manifest -TimeoutSeconds $TimeoutSeconds
    Sync-E2EJournalReleases -ApiBase $ApiBase -Headers $Headers -Manifest $Manifest

    Write-Step "cleaning E2E run $RunID (dry-run)"
    $dryRun = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/qa/e2e-runs/$RunID/cleanup" -Headers $Headers -Body @{
        dry_run = $true
        manifest = $Manifest
    }
    Assert-True ([bool]$dryRun.data.dry_run) "E2E cleanup dry-run did not report dry_run=true"

    Write-Step "cleaning E2E run $RunID (apply)"
    $apply = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/qa/e2e-runs/$RunID/cleanup" -Headers $Headers -Body @{
        dry_run = $false
        manifest = $Manifest
    }
    Assert-True (-not [bool]$apply.data.dry_run) "E2E cleanup apply did not report dry_run=false"
    Assert-CleanupDeletedMatchesCandidates -DryRun $dryRun.data -Apply $apply.data

    return [pscustomobject]@{
        dry_run = $dryRun.data
        apply = $apply.data
    }
}

function Invoke-PendingE2ERunRecovery {
    param(
        [string]$ApiBase,
        [hashtable]$Headers,
        [int]$TimeoutSeconds
    )

    $journalDirectory = Get-E2EJournalDirectory
    if (-not (Test-Path -LiteralPath $journalDirectory)) {
        return
    }
    foreach ($file in @(Get-ChildItem -LiteralPath $journalDirectory -Filter "*.json" -File | Sort-Object Name)) {
        $manifest = Get-Content -Raw -LiteralPath $file.FullName | ConvertFrom-Json
        $pendingRunID = [string]$manifest.run_id
        $parsedRunID = [guid]::Empty
        if (-not [guid]::TryParse($pendingRunID, [ref]$parsedRunID) -or $parsedRunID -eq [guid]::Empty) {
            throw "pending E2E journal has an invalid run_id: $($file.FullName)"
        }
        if ([bool]$manifest.cleanup_completed) {
            Remove-Item -LiteralPath $file.FullName -Force
            Write-Step "removed completed E2E journal $pendingRunID"
            continue
        }
        Write-Step "recovering pending E2E run $pendingRunID"
        Invoke-E2ERunCleanup -ApiBase $ApiBase -Headers $Headers -RunID $pendingRunID -Manifest $manifest -TimeoutSeconds $TimeoutSeconds | Out-Null
        $manifest.cleanup_completed = $true
        $previousJournalPath = $script:E2EJournalPath
        try {
            $script:E2EJournalPath = $file.FullName
            Save-E2EJournal -Journal $manifest
        } finally {
            $script:E2EJournalPath = $previousJournalPath
        }
        Remove-Item -LiteralPath $file.FullName -Force
        Write-Step "recovered pending E2E run $pendingRunID"
    }
}

function Normalize-UrlPath {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return "/"
    }
    try {
        $uri = [System.Uri]::new($Value, [System.UriKind]::RelativeOrAbsolute)
        if ($uri.IsAbsoluteUri) {
            $Value = $uri.AbsolutePath
        }
    } catch {
    }
    $Value = [System.Uri]::UnescapeDataString($Value)
    if (-not $Value.StartsWith("/")) {
        $Value = "/$Value"
    }
    if (-not $Value.EndsWith("/")) {
        $Value = "$Value/"
    }
    return $Value
}

function Join-PathSegments {
    param(
        [string]$Root,
        [string[]]$Segments
    )

    $result = $Root
    foreach ($segment in $Segments) {
        $result = Join-Path $result $segment
    }
    return $result
}

function Test-SitemapsContain {
    param(
        [string]$ReleaseOutputPath,
        [string]$ExpectedPath
    )

    $expected = Normalize-UrlPath $ExpectedPath
    $sitemaps = @(Get-ChildItem -LiteralPath $ReleaseOutputPath -Recurse -Filter "sitemap.xml")
    foreach ($sitemap in $sitemaps) {
        $raw = Get-Content -Raw -LiteralPath $sitemap.FullName
        try {
            [xml]$xml = $raw
            foreach ($loc in $xml.GetElementsByTagName("loc")) {
                if ((Normalize-UrlPath $loc.InnerText) -eq $expected) {
                    return $true
                }
            }
        } catch {
            if ($raw.Contains($expected)) {
                return $true
            }
        }
    }
    return $false
}

function Wait-PublishJob {
    param(
        [string]$JobID,
        [hashtable]$Headers,
        [int]$TimeoutSeconds
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $trace = New-Object System.Collections.Generic.List[string]
    $job = $null

    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 750
        $jobs = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/jobs" -Headers $Headers
        $job = @($jobs.data | Where-Object { $_.id -eq $JobID })[0]
        if ($job) {
            $trace.Add([string]$job.status)
            if ($job.status -eq "published" -or $job.status -eq "failed") {
                return [pscustomobject]@{
                    Job = $job
                    Trace = ($trace -join " -> ")
                }
            }
        }
    }

    throw "publish job $JobID did not finish within $TimeoutSeconds seconds; trace=$($trace -join ' -> ')"
}

$runID = $null
$journal = $null
$headers = $null
$cleanupResult = $null
$summary = $null
$scriptSucceeded = $false
$mediaPath = $null
$orphanMediaPath = $null

try {
Write-Step "checking API readiness"
$health = Invoke-Json -Method GET -Uri "$ApiBase/healthz"
Assert-True ($health.data.status -eq "ok") "API health check failed"
$ready = Invoke-Json -Method GET -Uri "$ApiBase/readyz"
Assert-True ($ready.data.status -eq "ready") "API is not ready"

Write-Step "logging in as admin"
$login = Invoke-RestMethod -Method POST -Uri "$ApiBase/api/v1/admin/auth/login" -SessionVariable AdminWebSession -ContentType "application/json" -Body (@{
    email = $AdminEmail
    password = $AdminPassword
} | ConvertTo-Json)
$script:AdminWebSession = $AdminWebSession
$headers = @{ "X-CSRF-Token" = [string]$login.data.csrf_token }
Invoke-PendingE2ERunRecovery -ApiBase $ApiBase -Headers $headers -TimeoutSeconds $PublishTimeoutSeconds

$runID = [guid]::NewGuid().ToString()
$journal = New-E2ERunJournal -RunID $runID
$script:E2EJournalPath = Join-Path (Get-E2EJournalDirectory) "$runID.json"
Save-E2EJournal -Journal $journal

Write-Step "capturing site settings before smoke"
$settingsBefore = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/settings" -Headers $headers
$journal.settings_before = $settingsBefore.data.settings
Save-E2EJournal -Journal $journal
$settingsForPreview = $settingsBefore

$stamp = Get-Date -Format "yyyyMMddHHmmss"
$slug = "e2e-smoke-$runID"
$pageSlug = "e2e-page-$runID"
$categorySlug = "e2e-category-$runID"
$tagSlug = "e2e-tag-$runID"
$siteTitle = "Zoking Smoke $stamp"
$sidebarSubtitle = "Smoke sidebar $stamp"

Write-Step "capturing active release before publish"
$releasesBefore = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $headers
$previousActive = @($releasesBefore.data | Where-Object { $_.is_active })[0]
if ($previousActive) {
    $journal.baseline_release_id = [string]$previousActive.id
    Save-E2EJournal -Journal $journal
} elseif (-not $SkipE2ECleanup) {
    throw "E2E cleanup requires an existing active baseline release; run a diagnostic bootstrap with -SkipRollback -SkipE2ECleanup first, then rerun normally."
}

Write-Step "creating taxonomy"
$category = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/categories" -Headers $headers -Body @{
    name = "E2E Category $stamp"
    slug = $categorySlug
    description = "Smoke test category"
    sort_order = 99
    enabled = $true
}
Add-E2ESlugRef -Journal $journal -Collection "categories" -ID ([string]$category.data.id) -Slug ([string]$category.data.slug)
$tag = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/tags" -Headers $headers -Body @{
    name = "E2E Tag $stamp"
    slug = $tagSlug
    description = "Smoke test tag"
    color = "#1677ff"
}
Add-E2ESlugRef -Journal $journal -Collection "tags" -ID ([string]$tag.data.id) -Slug ([string]$tag.data.slug)
$publicCategories = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/public/categories"
$publicTags = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/public/tags"
Assert-True (@($publicCategories.data | Where-Object { $_.id -eq $category.data.id }).Count -eq 1) "created category is not visible publicly"
Assert-True (@($publicTags.data | Where-Object { $_.id -eq $tag.data.id }).Count -eq 1) "created tag is not visible publicly"

Write-Step "checking reserved page slug protection"
$reservedPageRejected = $false
try {
    Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/pages" -Headers $headers -Body @{
        title = "Reserved Page"
        slug = "search"
        summary = "Reserved slug smoke"
        content_md = "# Reserved`n`nThis page should be rejected."
        status = "draft"
        visibility = "public"
        show_in_menu = $false
        menu_weight = 0
        allow_comment = $false
        seo_title = "Reserved Page"
        seo_description = "Reserved slug smoke"
    } | Out-Null
} catch {
    $response = $_.Exception.Response
    if ($response -and [int]$response.StatusCode -eq 422) {
        $reservedPageRejected = $true
    } else {
        throw
    }
}
Assert-True $reservedPageRejected "reserved page slug was not rejected"

Write-Step "uploading media"
$mediaPath = Join-Path ([System.IO.Path]::GetTempPath()) "zoking-e2e-$runID-referenced.png"
$pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII="
$pngBytes = [Convert]::FromBase64String($pngBase64)
$mediaPayload = [byte[]]($pngBytes + [System.Text.Encoding]::UTF8.GetBytes("referenced-$runID"))
[System.IO.File]::WriteAllBytes($mediaPath, $mediaPayload)
$media = Invoke-RestMethod -Method Post -Uri "$ApiBase/api/v1/admin/media" -Headers $headers -WebSession $script:AdminWebSession -Form @{
    file = Get-Item -LiteralPath $mediaPath
}
Add-E2EKeyRef -Journal $journal -Collection "media" -ID ([string]$media.data.id) -Key ([string]$media.data.storage_key)
$duplicateMedia = Invoke-RestMethod -Method Post -Uri "$ApiBase/api/v1/admin/media" -Headers $headers -WebSession $script:AdminWebSession -Form @{
    file = Get-Item -LiteralPath $mediaPath
}
Assert-True ($duplicateMedia.data.id -eq $media.data.id) "duplicate media upload did not reuse the checksum identity"
$mediaURL = [string]$media.data.public_url
if (-not $mediaURL.StartsWith("http")) {
    $mediaURL = "$ApiBase$mediaURL"
}
$mediaCheck = Invoke-WebRequest -Method Get -Uri $mediaURL
Assert-True ([int]$mediaCheck.StatusCode -eq 200) "uploaded media is not publicly readable"

Write-Step "creating page $pageSlug"
$page = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/pages" -Headers $headers -Body @{
    title = "E2E Page $stamp"
    slug = $pageSlug
    summary = "End-to-end smoke test page."
    content_md = "# E2E Page $stamp`n`nThis page verifies standalone page publishing and menu rendering.`n`n![Smoke image]($mediaURL)"
    status = "draft"
    visibility = "public"
    show_in_menu = $true
    menu_weight = 90
    menu_icon = "user"
    allow_comment = $false
    seo_title = "E2E Page $stamp"
    seo_description = "End-to-end smoke test page."
}
Add-E2ESlugRef -Journal $journal -Collection "pages" -ID ([string]$page.data.id) -Slug ([string]$page.data.slug)
Assert-True ($page.data.slug -eq $pageSlug) "created page slug mismatch"

Write-Step "creating post $slug"
$post = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/posts" -Headers $headers -Body @{
    title = "E2E Smoke $stamp"
    slug = $slug
    summary = "End-to-end smoke test post."
    content_md = "# E2E Smoke`n`nThis post verifies writing, taxonomy, media, async publishing, comments, and rollback.`n`n![Smoke image]($mediaURL)"
    status = "draft"
    visibility = "public"
    allow_comment = $true
    seo_title = "E2E Smoke $stamp"
    seo_description = "End-to-end smoke test post."
    category_ids = @($category.data.id)
    tag_ids = @($tag.data.id)
}
Add-E2ESlugRef -Journal $journal -Collection "posts" -ID ([string]$post.data.id) -Slug ([string]$post.data.slug)

Write-Step "building draft post preview"
$postPreview = Invoke-E2EPreview -Uri "$ApiBase/api/v1/admin/posts/$($post.data.id)/preview" -Headers $headers -Journal $journal
Assert-True ($postPreview.data.scope -eq "post") "post preview scope mismatch"
Assert-True (-not [string]::IsNullOrWhiteSpace([string]$postPreview.data.preview_key)) "post preview key is missing"
$postPreviewHTML = (Invoke-WebRequest -Method GET -Uri $postPreview.data.target_url).Content
Assert-True ($postPreviewHTML.Contains("E2E Smoke $stamp")) "post preview title missing"
Assert-True ($postPreviewHTML.Contains("This post verifies writing")) "post preview body missing"
Assert-True ($postPreviewHTML.Contains($mediaURL)) "post preview media URL missing"
Assert-True ($postPreviewHTML.Contains("E2E Category $stamp")) "post preview category missing"
Assert-True ($postPreviewHTML.Contains("E2E Tag $stamp")) "post preview tag missing"
$previewManifest = Invoke-WebRequest -Method GET -Uri "$ApiBase/preview-files/$($postPreview.data.preview_key)/manifest.json" -SkipHttpErrorCheck
Assert-True ([int]$previewManifest.StatusCode -eq 404) "preview manifest must not be publicly accessible"

Write-Step "building draft page preview"
$pagePreview = Invoke-E2EPreview -Uri "$ApiBase/api/v1/admin/pages/$($page.data.id)/preview" -Headers $headers -Journal $journal
Assert-True ($pagePreview.data.scope -eq "page") "page preview scope mismatch"
$pagePreviewHTML = (Invoke-WebRequest -Method GET -Uri $pagePreview.data.target_url).Content
Assert-True ($pagePreviewHTML.Contains("E2E Page $stamp")) "page preview title missing"
Assert-True ($pagePreviewHTML.Contains("standalone page publishing")) "page preview body missing"

Write-Step "building transient site settings preview"
$previewSiteTitle = "Zoking Preview $stamp"
$previewSidebarSubtitle = "Preview sidebar $stamp"
$settingsPreview = Invoke-E2EPreview -Uri "$ApiBase/api/v1/admin/settings/preview" -Headers $headers -Journal $journal -Body @{
    site = @{
        title = $previewSiteTitle
        base_url = [string]$settingsForPreview.data.settings.site.base_url
    }
    sidebar = @{
        emoji = [string]$settingsForPreview.data.settings.sidebar.emoji
        subtitle = $previewSidebarSubtitle
    }
    comments = @{
        enabled = [bool]$settingsForPreview.data.settings.comments.enabled
        api_base = [string]$settingsForPreview.data.settings.comments.api_base
    }
    footer = @{
        since = [int]$settingsForPreview.data.settings.footer.since
    }
    pagination = @{
        pager_size = [int]$settingsForPreview.data.settings.pagination.pager_size
    }
}
$settingsPreviewHTML = (Invoke-WebRequest -Method GET -Uri $settingsPreview.data.target_url).Content
Assert-True ($settingsPreviewHTML.Contains($previewSiteTitle)) "settings preview title missing"
Assert-True ($settingsPreviewHTML.Contains($previewSidebarSubtitle)) "settings preview subtitle missing"
$settingsAfterPreview = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/settings" -Headers $headers
Assert-True ($settingsAfterPreview.data.hash -eq $settingsForPreview.data.hash) "settings preview persisted transient settings"
$releasesAfterPreview = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $headers
$activeAfterPreview = @($releasesAfterPreview.data | Where-Object { $_.is_active })[0]
Assert-True (@($releasesAfterPreview.data).Count -eq @($releasesBefore.data).Count) "preview unexpectedly created a publish release"
if ($previousActive) {
    Assert-True ($activeAfterPreview.id -eq $previousActive.id) "preview unexpectedly changed the active release"
}

Write-Step "submitting public comment before publish should fail"
$prePublishFailed = $false
try {
    Invoke-Json -Method POST -Uri "$ApiBase/api/v1/public/posts/$slug/comments" -Body @{
        author_name = "Smoke Reader"
        author_email = "reader-$stamp@example.com"
        content = "This should not be accepted before publish."
    } | Out-Null
} catch {
    $response = $_.Exception.Response
    if ($response -and [int]$response.StatusCode -eq 404) {
        $prePublishFailed = $true
    } else {
        throw
    }
}
Assert-True $prePublishFailed "comment unexpectedly succeeded before publish or failed with the wrong status"

Write-Step "queueing publish job"
$publish = Invoke-WebRequest -Method POST -Uri "$ApiBase/api/v1/admin/posts/$($post.data.id)/publish" -Headers $headers -WebSession $script:AdminWebSession
Assert-True ([int]$publish.StatusCode -eq 202) "publish endpoint did not return HTTP 202"
$publishPayload = $publish.Content | ConvertFrom-Json
$jobID = $publishPayload.data.job.id
Add-E2EIDRef -Journal $journal -Collection "jobs" -ID ([string]$jobID)
Assert-True ($publishPayload.data.job.status -eq "requested") "publish job did not start as requested"

Write-Step "waiting for worker to publish job $jobID"
$jobResult = Wait-PublishJob -JobID $jobID -Headers $headers -TimeoutSeconds $PublishTimeoutSeconds
Assert-True ($jobResult.Job.status -eq "published") "publish job failed or did not publish: $($jobResult.Job.status)"

Write-Step "checking release artifacts"
$releasesAfter = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $headers
$release = @($releasesAfter.data | Where-Object { $_.job_id -eq $jobID })[0]
Assert-True ($null -ne $release) "release for job $jobID not found"
Add-E2EKeyRef -Journal $journal -Collection "releases" -ID ([string]$release.id) -Key ([string]$release.release_key)
Assert-True ([bool]$release.is_active) "new release is not active"
$manifestPath = Join-Path $release.output_path "manifest.json"
$articleHTMLPath = Join-PathSegments -Root $release.output_path -Segments @("p", $slug, "index.html")
$homeHTMLPath = Join-Path $release.output_path "index.html"
$sitemapPath = Join-Path $release.output_path "sitemap.xml"
$rssPath = Join-Path $release.output_path "index.xml"
$categoryHTMLPath = Join-PathSegments -Root $release.output_path -Segments @("categories", $categorySlug, "index.html")
$tagHTMLPath = Join-PathSegments -Root $release.output_path -Segments @("tags", $tagSlug, "index.html")
$contentPath = Join-PathSegments -Root $HugoSiteDir -Segments @("content", "post", $slug, "index.md")
$categoryContentPath = Join-PathSegments -Root $HugoSiteDir -Segments @("content", "categories", $categorySlug, "_index.md")
$tagContentPath = Join-PathSegments -Root $HugoSiteDir -Segments @("content", "tags", $tagSlug, "_index.md")

Assert-True (Test-Path $manifestPath) "release manifest is missing"
Assert-True (Test-Path $articleHTMLPath) "release article HTML is missing"
Assert-True (Test-Path $homeHTMLPath) "release home HTML is missing"
Assert-True (Test-Path $sitemapPath) "release sitemap is missing"
Assert-True (Test-Path $rssPath) "release RSS is missing"
Assert-True (Test-Path $categoryHTMLPath) "release category page is missing"
Assert-True (Test-Path $tagHTMLPath) "release tag page is missing"
Assert-True (Test-Path $contentPath) "Hugo content snapshot is missing"
Assert-True (Test-Path $categoryContentPath) "Hugo category taxonomy snapshot is missing"
Assert-True (Test-Path $tagContentPath) "Hugo tag taxonomy snapshot is missing"

$manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
Assert-True ($manifest.job_id -eq $jobID) "manifest job_id mismatch"
Assert-True ($manifest.slug -eq $slug) "manifest slug mismatch"
Assert-True ($manifest.release_key -eq $release.release_key) "manifest release_key mismatch"

$contentMarkdown = Get-Content -Raw -LiteralPath $contentPath
Assert-True ($contentMarkdown.Contains($categorySlug)) "content front matter category slug missing"
Assert-True ($contentMarkdown.Contains($tagSlug)) "content front matter tag slug missing"
Assert-True ($contentMarkdown.Contains($mediaURL)) "content Markdown media URL missing"
$categoryMarkdown = Get-Content -Raw -LiteralPath $categoryContentPath
$tagMarkdown = Get-Content -Raw -LiteralPath $tagContentPath
Assert-True ($categoryMarkdown.Contains("E2E Category $stamp")) "category taxonomy display name missing"
Assert-True ($tagMarkdown.Contains("E2E Tag $stamp")) "tag taxonomy display name missing"

$articleHTML = Get-Content -Raw -LiteralPath $articleHTMLPath
$homeHTML = Get-Content -Raw -LiteralPath $homeHTMLPath
$rssXML = Get-Content -Raw -LiteralPath $rssPath
Assert-True ($articleHTML.Contains("E2E Smoke $stamp")) "article HTML title missing"
Assert-True ($articleHTML.Contains("This post verifies writing")) "article HTML body missing"
Assert-True ($articleHTML.Contains($mediaURL)) "article HTML media URL missing"
Assert-True ($articleHTML.Contains("data-public-comments")) "article HTML comments container missing"
Assert-True ($homeHTML.Contains("E2E Smoke $stamp")) "home HTML does not include new post"
Assert-True (Test-SitemapsContain -ReleaseOutputPath $release.output_path -ExpectedPath "/p/$slug/") "release sitemaps do not include new post"
Assert-True ($rssXML.Contains("E2E Smoke $stamp")) "RSS does not include new post"

Write-Step "queueing page publish job"
$pagePublish = Invoke-WebRequest -Method POST -Uri "$ApiBase/api/v1/admin/pages/$($page.data.id)/publish" -Headers $headers -WebSession $script:AdminWebSession
Assert-True ([int]$pagePublish.StatusCode -eq 202) "page publish endpoint did not return HTTP 202"
$pagePublishPayload = $pagePublish.Content | ConvertFrom-Json
$pageJobID = $pagePublishPayload.data.job.id
Add-E2EIDRef -Journal $journal -Collection "jobs" -ID ([string]$pageJobID)
Assert-True ($pagePublishPayload.data.job.status -eq "requested") "page publish job did not start as requested"

Write-Step "waiting for worker to publish page job $pageJobID"
$pageJobResult = Wait-PublishJob -JobID $pageJobID -Headers $headers -TimeoutSeconds $PublishTimeoutSeconds
Assert-True ($pageJobResult.Job.status -eq "published") "page publish job failed or did not publish: $($pageJobResult.Job.status)"

Write-Step "checking page release artifacts"
$releasesAfterPage = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $headers
$pageRelease = @($releasesAfterPage.data | Where-Object { $_.job_id -eq $pageJobID })[0]
Assert-True ($null -ne $pageRelease) "release for page job $pageJobID not found"
Add-E2EKeyRef -Journal $journal -Collection "releases" -ID ([string]$pageRelease.id) -Key ([string]$pageRelease.release_key)
Assert-True ([bool]$pageRelease.is_active) "page release is not active"
$pageManifestPath = Join-Path $pageRelease.output_path "manifest.json"
$pageHTMLPath = Join-PathSegments -Root $pageRelease.output_path -Segments @($pageSlug, "index.html")
$pageHomeHTMLPath = Join-Path $pageRelease.output_path "index.html"
$pageContentPath = Join-PathSegments -Root $HugoSiteDir -Segments @("content", "page", $pageSlug, "index.md")
Assert-True (Test-Path $pageManifestPath) "page release manifest is missing"
Assert-True (Test-Path $pageHTMLPath) "release page HTML is missing"
Assert-True (Test-Path $pageContentPath) "Hugo page content snapshot is missing"
$pageManifest = Get-Content -Raw -LiteralPath $pageManifestPath | ConvertFrom-Json
Assert-True ($pageManifest.job_id -eq $pageJobID) "page manifest job_id mismatch"
Assert-True ($pageManifest.scope -eq "page") "page manifest scope mismatch"
Assert-True ($pageManifest.slug -eq $pageSlug) "page manifest slug mismatch"
$pageMarkdown = Get-Content -Raw -LiteralPath $pageContentPath
Assert-True ($pageMarkdown.Contains("E2E Page $stamp")) "page content title missing"
Assert-True ($pageMarkdown.Contains($mediaURL)) "page Markdown media URL missing"
$pageHTML = Get-Content -Raw -LiteralPath $pageHTMLPath
$pageHomeHTML = Get-Content -Raw -LiteralPath $pageHomeHTMLPath
Assert-True ($pageHTML.Contains("E2E Page $stamp")) "page HTML title missing"
Assert-True ($pageHTML.Contains("standalone page publishing")) "page HTML body missing"
Assert-True ($pageHomeHTML.Contains("E2E Page $stamp")) "home page menu does not include new page"
Assert-True (Test-SitemapsContain -ReleaseOutputPath $pageRelease.output_path -ExpectedPath "/$pageSlug/") "release sitemaps do not include new page"
Assert-True (Test-SitemapsContain -ReleaseOutputPath $pageRelease.output_path -ExpectedPath "/p/$slug/") "page release sitemaps lost the post"

Write-Step "checking media reference protection"
$mediaAfterPublish = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/media" -Headers $headers
$publishedMedia = @($mediaAfterPublish.data | Where-Object { $_.id -eq $media.data.id })[0]
Assert-True ($null -ne $publishedMedia) "published media record is missing"
Assert-True ([int]$publishedMedia.usage_count -gt 0) "published media usage was not recorded"
$mediaDeleteBlocked = $false
try {
    Invoke-Json -Method DELETE -Uri "$ApiBase/api/v1/admin/media/$($media.data.id)" -Headers $headers | Out-Null
} catch {
    $response = $_.Exception.Response
    if ($response -and [int]$response.StatusCode -eq 409) {
        $mediaDeleteBlocked = $true
    } else {
        throw
    }
}
Assert-True $mediaDeleteBlocked "referenced media delete was not blocked"

Write-Step "checking orphan media cleanup dry run"
$orphanMediaPath = Join-Path ([System.IO.Path]::GetTempPath()) "zoking-e2e-$runID-orphan.png"
$orphanMediaPayload = [byte[]]($pngBytes + [System.Text.Encoding]::UTF8.GetBytes("orphan-$runID"))
[System.IO.File]::WriteAllBytes($orphanMediaPath, $orphanMediaPayload)
$orphanMedia = Invoke-RestMethod -Method Post -Uri "$ApiBase/api/v1/admin/media" -Headers $headers -WebSession $script:AdminWebSession -Form @{
    file = Get-Item -LiteralPath $orphanMediaPath
}
Add-E2EKeyRef -Journal $journal -Collection "media" -ID ([string]$orphanMedia.data.id) -Key ([string]$orphanMedia.data.storage_key)
$mediaCleanupPlan = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/media/cleanup" -Headers $headers -Body @{
    dry_run = $true
    orphan_grace_seconds = 0
    batch_size = 500
}
$orphanCandidate = @($mediaCleanupPlan.data.items | Where-Object { $_.id -eq $orphanMedia.data.id -and $_.action -eq "candidate" })
Assert-True ([bool]$mediaCleanupPlan.data.dry_run) "media cleanup dry run did not report dry_run=true"
Assert-True ($orphanCandidate.Count -eq 1) "orphan media cleanup dry run did not include uploaded orphan media"

Write-Step "saving and publishing site settings"
$settingsSave = Invoke-Json -Method PATCH -Uri "$ApiBase/api/v1/admin/settings" -Headers $headers -Body @{
    site = @{
        title = $siteTitle
        base_url = "http://localhost:1313/"
    }
    sidebar = @{
        emoji = "S"
        subtitle = $sidebarSubtitle
    }
    comments = @{
        enabled = $true
        api_base = $ApiBase
    }
    footer = @{
        since = 2026
    }
    pagination = @{
        pager_size = 3
    }
}
Assert-True ($settingsSave.data.settings.site.title -eq $siteTitle) "settings save did not persist site title"
Assert-True ($settingsSave.data.settings.sidebar.subtitle -eq $sidebarSubtitle) "settings save did not persist sidebar subtitle"
$settingsPublish = Invoke-WebRequest -Method POST -Uri "$ApiBase/api/v1/admin/settings/publish" -Headers $headers -WebSession $script:AdminWebSession
Assert-True ([int]$settingsPublish.StatusCode -eq 202) "settings publish endpoint did not return HTTP 202"
$settingsPublishPayload = $settingsPublish.Content | ConvertFrom-Json
$settingsJobID = $settingsPublishPayload.data.job.id
Add-E2EIDRef -Journal $journal -Collection "jobs" -ID ([string]$settingsJobID)

Write-Step "waiting for worker to publish site settings job $settingsJobID"
$settingsJobResult = Wait-PublishJob -JobID $settingsJobID -Headers $headers -TimeoutSeconds $PublishTimeoutSeconds
Assert-True ($settingsJobResult.Job.status -eq "published") "settings publish job failed or did not publish: $($settingsJobResult.Job.status)"

Write-Step "checking settings release artifacts"
$releasesAfterSettings = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $headers
$settingsRelease = @($releasesAfterSettings.data | Where-Object { $_.job_id -eq $settingsJobID })[0]
Assert-True ($null -ne $settingsRelease) "release for settings job $settingsJobID not found"
Add-E2EKeyRef -Journal $journal -Collection "releases" -ID ([string]$settingsRelease.id) -Key ([string]$settingsRelease.release_key)
Assert-True ([bool]$settingsRelease.is_active) "settings release is not active"
$settingsManifestPath = Join-Path $settingsRelease.output_path "manifest.json"
$settingsHomeHTMLPath = Join-Path $settingsRelease.output_path "index.html"
$settingsPageHTMLPath = Join-PathSegments -Root $settingsRelease.output_path -Segments @($pageSlug, "index.html")
Assert-True (Test-Path $settingsManifestPath) "settings release manifest is missing"
Assert-True (Test-Path $settingsHomeHTMLPath) "settings release home HTML is missing"
Assert-True (Test-Path $settingsPageHTMLPath) "settings release page HTML is missing"
$settingsManifest = Get-Content -Raw -LiteralPath $settingsManifestPath | ConvertFrom-Json
Assert-True ($settingsManifest.job_id -eq $settingsJobID) "settings manifest job_id mismatch"
Assert-True ($settingsManifest.scope -eq "site") "settings manifest scope mismatch"
Assert-True (-not [string]::IsNullOrWhiteSpace([string]$settingsManifest.settings_hash)) "settings manifest missing settings_hash"
$settingsHomeHTML = Get-Content -Raw -LiteralPath $settingsHomeHTMLPath
Assert-True ($settingsHomeHTML.Contains($siteTitle)) "settings release home HTML does not include site title"
Assert-True ($settingsHomeHTML.Contains($sidebarSubtitle)) "settings release home HTML does not include sidebar subtitle"
Assert-True ($settingsHomeHTML.Contains("E2E Page $stamp")) "settings release lost page menu"
Assert-True ($settingsHomeHTML.Contains("E2E Smoke $stamp")) "settings release lost post"
Assert-True (Test-SitemapsContain -ReleaseOutputPath $settingsRelease.output_path -ExpectedPath "/$pageSlug/") "settings release sitemaps do not include page"
Assert-True (Test-SitemapsContain -ReleaseOutputPath $settingsRelease.output_path -ExpectedPath "/p/$slug/") "settings release sitemaps do not include post"

Write-Step "submitting and approving public comment"
$comment = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/public/posts/$slug/comments" -Body @{
    author_name = "Smoke Reader $stamp"
    author_email = "reader-$stamp@example.com"
    author_website = "https://example.com"
    content = "Approved smoke comment $stamp"
}
Add-E2EIDRef -Journal $journal -Collection "comments" -ID ([string]$comment.data.id)
Assert-True ($comment.data.PSObject.Properties.Name -notcontains "status") "public comment response leaked moderation status"
$publicBeforeModeration = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/public/posts/$slug/comments"
Assert-True (@($publicBeforeModeration.data | Where-Object { $_.id -eq $comment.data.id }).Count -eq 0) "pending comment is visible publicly before moderation"
$adminPendingComments = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/comments?status=pending" -Headers $headers
Assert-True (@($adminPendingComments.data | Where-Object { $_.id -eq $comment.data.id }).Count -eq 1) "pending comment is not visible in admin moderation list"
$approved = Invoke-Json -Method PATCH -Uri "$ApiBase/api/v1/admin/comments/$($comment.data.id)/moderation" -Headers $headers -Body @{
    status = "approved"
}
Assert-True ($approved.data.status -eq "approved") "comment was not approved"
$publicComments = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/public/posts/$slug/comments"
$foundComment = @($publicComments.data | Where-Object { $_.id -eq $comment.data.id })
Assert-True ($foundComment.Count -eq 1) "approved comment not visible in public comments"

$rollbackRelease = $null
if (-not $SkipRollback -and $previousActive) {
    Write-Step "promoting previous active release for rollback smoke"
    $rollbackRelease = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/publish/releases/$($previousActive.id)/promote" -Headers $headers
    Assert-True ([bool]$rollbackRelease.data.is_active) "previous release was not promoted back to active"
    $releasesAfterRollback = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $headers
    $activeReleases = @($releasesAfterRollback.data | Where-Object { $_.is_active })
    $newReleaseAfterRollback = @($releasesAfterRollback.data | Where-Object { $_.id -eq $settingsRelease.id })[0]
    Assert-True ($activeReleases.Count -eq 1) "rollback left more than one active release"
    Assert-True ($activeReleases[0].id -eq $previousActive.id) "rollback did not restore previous active release"
    Assert-True (-not [bool]$newReleaseAfterRollback.is_active) "settings release is still active after rollback"
} elseif (-not $SkipRollback) {
    throw "rollback smoke requires an existing active release; use -SkipRollback -SkipE2ECleanup only for an explicit diagnostic bootstrap, then rerun normally"
}

Write-Step "checking release cleanup dry run"
$releaseCleanupPlan = Invoke-Json -Method POST -Uri "$ApiBase/api/v1/admin/publish/releases/cleanup" -Headers $headers -Body @{
    dry_run = $true
    keep_latest = 1
    keep_days = 0
}
$releasesForCleanupCheck = Invoke-Json -Method GET -Uri "$ApiBase/api/v1/admin/publish/releases" -Headers $headers
$activeForCleanupCheck = @($releasesForCleanupCheck.data | Where-Object { $_.is_active })
$activeCleanupCandidate = @($releaseCleanupPlan.data.items | Where-Object { $_.id -eq $activeForCleanupCheck[0].id })
Assert-True ([bool]$releaseCleanupPlan.data.dry_run) "release cleanup dry run did not report dry_run=true"
Assert-True ($activeCleanupCandidate.Count -eq 0) "release cleanup dry run included the active release"

$summary = [pscustomobject]@{
    ok = $true
    run_id = $runID
    slug = $slug
    page_slug = $pageSlug
    post_id = $post.data.id
    page_id = $page.data.id
    post_preview_key = $postPreview.data.preview_key
    post_preview_url = $postPreview.data.target_url
    page_preview_key = $pagePreview.data.preview_key
    page_preview_url = $pagePreview.data.target_url
    settings_preview_key = $settingsPreview.data.preview_key
    settings_preview_url = $settingsPreview.data.target_url
    job_id = $jobID
    job_status = $jobResult.Job.status
    job_trace = $jobResult.Trace
    page_job_id = $pageJobID
    page_job_status = $pageJobResult.Job.status
    settings_job_id = $settingsJobID
    settings_job_status = $settingsJobResult.Job.status
    post_release_key = $release.release_key
    page_release_key = $pageRelease.release_key
    release_key = $settingsRelease.release_key
    release_output = $settingsRelease.output_path
    hugo_site_dir = $HugoSiteDir
    media_id = $media.data.id
    media_url = $mediaURL
    comment_id = $comment.data.id
    rollback_release_key = if ($rollbackRelease) { $rollbackRelease.data.release_key } else { $null }
    cleanup_skipped = [bool]$SkipE2ECleanup
}

$scriptSucceeded = $true
} finally {
    try {
        if ($headers -and $journal -and $runID -and -not $SkipE2ECleanup) {
            $cleanupResult = Invoke-E2ERunCleanup -ApiBase $ApiBase -Headers $headers -RunID $runID -Manifest $journal -TimeoutSeconds $PublishTimeoutSeconds
            if ($summary) {
                $summary | Add-Member -NotePropertyName "cleanup" -NotePropertyValue $cleanupResult -Force
            }
            $journal.cleanup_completed = $true
            Save-E2EJournal -Journal $journal
            Remove-Item -LiteralPath $script:E2EJournalPath -Force
            $script:E2EJournalPath = $null
            Write-Step "cleanup completed for E2E run $runID"
        } elseif ($SkipE2ECleanup -and $journal) {
            Write-Step "E2E cleanup skipped for diagnostic run $runID"
        }
    } finally {
        foreach ($temporaryPath in @($mediaPath, $orphanMediaPath)) {
            if ($temporaryPath -and (Test-Path -LiteralPath $temporaryPath)) {
                Remove-Item -LiteralPath $temporaryPath -Force
            }
        }
    }
}

if ($scriptSucceeded -and $summary) {
    Write-Step "completed"
    $summary | ConvertTo-Json -Depth 12
}
