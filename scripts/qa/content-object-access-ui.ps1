param(
    [string]$ApiBase = "http://localhost:18080",
    [string]$AdminBase = "http://localhost:5173",
    [string]$AdminEmail = "admin@zoking.local",
    [string]$AdminPassword = "ChangeMe123!",
    [string]$DatabaseContainer = "zoking-blog-postgres",
    [string]$DatabaseUser = "zoking",
    [string]$DatabaseName = "zoking_blog",
    [string]$PlaywrightPackagePath = "",
    [string]$EvidenceDir = ""
)

$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
    throw "PowerShell 7+ is required."
}

function Assert-LoopbackUri {
    param([string]$Value, [string]$Name)
    $uri = [System.Uri]$Value
    if (-not $uri.IsAbsoluteUri -or $uri.Scheme -ne "http" -or -not [System.Net.IPAddress]::IsLoopback(([System.Net.Dns]::GetHostAddresses($uri.Host) | Select-Object -First 1))) {
        throw "$Name must be an absolute loopback HTTP URL."
    }
}

function Invoke-ApiJson {
    param(
        [string]$Method,
        [string]$Path,
		[object]$Token = $null,
        [object]$Body = $null
    )
    $headers = @{}
	if ($null -ne $Token) {
		$headers["X-CSRF-Token"] = [string]$Token.CSRFToken
    }
    $arguments = @{
        Method = $Method
        Uri = "$ApiBase$Path"
        Headers = $headers
        TimeoutSec = 20
        SkipHttpErrorCheck = $true
    }
	if ($null -ne $Token) {
		$arguments.WebSession = $Token.WebSession
	}
    if ($null -ne $Body) {
        $arguments.ContentType = "application/json"
        $arguments.Body = $Body | ConvertTo-Json -Depth 30 -Compress
    }
    $response = Invoke-WebRequest @arguments
    if ([int]$response.StatusCode -lt 200 -or [int]$response.StatusCode -ge 300) {
        throw "$Method $Path returned HTTP $($response.StatusCode): $($response.Content)"
    }
    return $response.Content | ConvertFrom-Json -Depth 30
}

function Invoke-Login {
    param([string]$Email, [string]$Password)
	$body = @{ email = $Email; password = $Password } | ConvertTo-Json -Compress
	$response = Invoke-RestMethod -Method POST -Uri "$ApiBase/api/v1/admin/auth/login" -SessionVariable LoginWebSession -ContentType "application/json" -Body $body
	return [pscustomobject]@{
		CSRFToken = [string]$response.data.csrf_token
		WebSession = $LoginWebSession
    }
}

function Invoke-DatabaseScalar {
    param([string]$Sql)
    $output = & docker exec $DatabaseContainer psql -v ON_ERROR_STOP=1 -U $DatabaseUser -d $DatabaseName -Atc $Sql
    if ($LASTEXITCODE -ne 0) {
        throw "PostgreSQL command failed with exit code $LASTEXITCODE."
    }
    return (@($output) -join "`n").Trim()
}

$ApiBase = $ApiBase.TrimEnd("/")
$AdminBase = $AdminBase.TrimEnd("/")
Assert-LoopbackUri -Value $ApiBase -Name "ApiBase"
Assert-LoopbackUri -Value $AdminBase -Name "AdminBase"

if ($DatabaseName -ne "zoking_blog" -and -not $DatabaseName.EndsWith("_test", [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing UI fixtures against database '$DatabaseName'. Use zoking_blog locally or a database ending in _test."
}

$actualDatabase = Invoke-DatabaseScalar -Sql "select current_database();"
if ($actualDatabase -ne $DatabaseName) {
    throw "Database container resolved '$actualDatabase', expected '$DatabaseName'."
}

if (-not $PlaywrightPackagePath) {
    $cacheRoot = Join-Path $env:USERPROFILE ".codex\cache\npm\_npx"
    $candidate = Get-ChildItem -Path $cacheRoot -Directory -ErrorAction SilentlyContinue |
        ForEach-Object { Get-Item (Join-Path $_.FullName "node_modules\playwright") -ErrorAction SilentlyContinue } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if ($candidate) {
        $PlaywrightPackagePath = $candidate.FullName
    }
}
if (-not $PlaywrightPackagePath -or -not (Test-Path (Join-Path $PlaywrightPackagePath "index.js"))) {
    throw "Playwright package was not found. Pass -PlaywrightPackagePath or run npx playwright once."
}

if (-not $EvidenceDir) {
    $EvidenceDir = Join-Path $PSScriptRoot "..\..\docs\process\evidence"
}
$EvidenceDir = [System.IO.Path]::GetFullPath($EvidenceDir)
[System.IO.Directory]::CreateDirectory($EvidenceDir) | Out-Null

$runID = [Guid]::NewGuid().ToString("N").Substring(0, 12)
$prefix = "sec-p15-ui-$runID"
$authorEmail = "$prefix-author@zoking.local"
$viewerEmail = "$prefix-viewer@zoking.local"
$fixturePassword = "SecP15-$runID-Aa1!"
$authorPostSlug = "$prefix-author-post"
$adminPostSlug = "$prefix-admin-post"
$adminPageSlug = "$prefix-admin-page"
$authorPostTitle = "SEC P15 作者草稿 $runID"
$adminPostTitle = "SEC P15 管理员草稿 $runID"
$adminPageTitle = "SEC P15 管理员页面 $runID"

$health = Invoke-ApiJson -Method GET -Path "/readyz"
if ($health.data.status -ne "ready") {
    throw "API is not ready."
}
$adminShell = Invoke-WebRequest -Uri "$AdminBase/" -TimeoutSec 20
if ([int]$adminShell.StatusCode -ne 200) {
    throw "Admin shell is not available."
}

try {
    $adminToken = Invoke-Login -Email $AdminEmail -Password $AdminPassword
    $author = Invoke-ApiJson -Method POST -Path "/api/v1/admin/users" -Token $adminToken -Body @{
        email = $authorEmail
        username = "$prefix-author"
        display_name = "SEC P15 Author"
        password = $fixturePassword
        role_codes = @("author")
    }
    $viewer = Invoke-ApiJson -Method POST -Path "/api/v1/admin/users" -Token $adminToken -Body @{
        email = $viewerEmail
        username = "$prefix-viewer"
        display_name = "SEC P15 Viewer"
        password = $fixturePassword
        role_codes = @("viewer")
    }

    $authorToken = Invoke-Login -Email $authorEmail -Password $fixturePassword
    $authorPost = Invoke-ApiJson -Method POST -Path "/api/v1/admin/posts" -Token $authorToken -Body @{
        title = $authorPostTitle
        slug = $authorPostSlug
        summary = "SEC-P15 UI owner-scoped author draft."
        content_md = "# SEC P15 Author Draft`n`nOwner-scoped UI acceptance fixture."
        status = "draft"
        visibility = "public"
        allow_comment = $true
    }
    if ([string]$authorPost.data.author_id -ne [string]$author.data.id) {
        throw "Author post owner was not forced to the authenticated author."
    }

    $adminPost = Invoke-ApiJson -Method POST -Path "/api/v1/admin/posts" -Token $adminToken -Body @{
        title = $adminPostTitle
        slug = $adminPostSlug
        summary = "SEC-P15 UI global-scope admin draft."
        content_md = "# SEC P15 Admin Draft`n`nGlobal read UI acceptance fixture."
        status = "draft"
        visibility = "public"
        allow_comment = $false
    }
    $adminPage = Invoke-ApiJson -Method POST -Path "/api/v1/admin/pages" -Token $adminToken -Body @{
        title = $adminPageTitle
        slug = $adminPageSlug
        summary = "SEC-P15 UI global-scope admin page."
        content_md = "# SEC P15 Admin Page`n`nPage owner-scope UI acceptance fixture."
        status = "draft"
        visibility = "public"
        show_in_menu = $false
        menu_weight = 0
        allow_comment = $false
    }

    $env:PLAYWRIGHT_PACKAGE_PATH = $PlaywrightPackagePath
    $env:SEC_P15_UI_ADMIN_BASE = $AdminBase
    $env:SEC_P15_UI_ADMIN_EMAIL = $AdminEmail
    $env:SEC_P15_UI_ADMIN_PASSWORD = $AdminPassword
    $env:SEC_P15_UI_AUTHOR_EMAIL = $authorEmail
    $env:SEC_P15_UI_AUTHOR_PASSWORD = $fixturePassword
    $env:SEC_P15_UI_VIEWER_EMAIL = $viewerEmail
    $env:SEC_P15_UI_VIEWER_PASSWORD = $fixturePassword
    $env:SEC_P15_UI_AUTHOR_POST_TITLE = $authorPostTitle
    $env:SEC_P15_UI_ADMIN_POST_TITLE = $adminPostTitle
    $env:SEC_P15_UI_ADMIN_POST_ID = [string]$adminPost.data.id
    $env:SEC_P15_UI_ADMIN_PAGE_TITLE = $adminPageTitle
    $env:SEC_P15_UI_ADMIN_PAGE_ID = [string]$adminPage.data.id
    $env:SEC_P15_UI_EVIDENCE_DIR = $EvidenceDir

    & node (Join-Path $PSScriptRoot "content-object-access-ui.mjs")
    if ($LASTEXITCODE -ne 0) {
        throw "Playwright UI acceptance failed with exit code $LASTEXITCODE."
    }
} finally {
    $cleanupSql = @"
begin;
delete from audit_logs
where actor_id in (select id from users where email in ('$authorEmail', '$viewerEmail'))
   or resource_id in (
       select id from users where email in ('$authorEmail', '$viewerEmail')
       union select id from posts where slug in ('$authorPostSlug', '$adminPostSlug')
       union select id from pages where slug = '$adminPageSlug'
   );
delete from publish_previews where post_id in (select id from posts where slug in ('$authorPostSlug', '$adminPostSlug')) or page_id in (select id from pages where slug = '$adminPageSlug');
delete from publish_jobs where post_id in (select id from posts where slug in ('$authorPostSlug', '$adminPostSlug')) or page_id in (select id from pages where slug = '$adminPageSlug');
delete from publish_releases where post_id in (select id from posts where slug in ('$authorPostSlug', '$adminPostSlug')) or page_id in (select id from pages where slug = '$adminPageSlug');
delete from posts where slug in ('$authorPostSlug', '$adminPostSlug');
delete from pages where slug = '$adminPageSlug';
delete from users where email in ('$authorEmail', '$viewerEmail');
commit;
"@
    Invoke-DatabaseScalar -Sql $cleanupSql | Out-Null
}

$remaining = Invoke-DatabaseScalar -Sql "select (select count(*) from users where email in ('$authorEmail', '$viewerEmail')) || '|' || (select count(*) from posts where slug in ('$authorPostSlug', '$adminPostSlug')) || '|' || (select count(*) from pages where slug = '$adminPageSlug');"
if ($remaining -ne "0|0|0") {
    throw "UI acceptance cleanup left fixture rows: $remaining"
}

[pscustomobject]@{
    ok = $true
    run_id = $runID
    api_base = $ApiBase
    admin_base = $AdminBase
    evidence_dir = $EvidenceDir
    remaining_fixture_counts = $remaining
} | ConvertTo-Json -Depth 5
