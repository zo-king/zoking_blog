param(
    [string]$ApiBase = "http://localhost:18080",
    [string]$AdminBase = "http://localhost:5173",
    [string]$SiteBase = "http://localhost:1313",
    [string]$AdminOrigin = "",
    [string]$AdminEmail = "",
    [string]$AdminPassword = "",
    [switch]$TestRateLimit,
    [int]$RateLimitAttempts = 20
)

$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
    throw "PowerShell 7+ is required. Use pwsh to run scripts/qa/http-blackbox.ps1."
}

$ApiBase = $ApiBase.TrimEnd("/")
$AdminBase = $AdminBase.TrimEnd("/")
$SiteBase = $SiteBase.TrimEnd("/")
$SiteOrigin = ([System.Uri]$SiteBase).GetLeftPart([System.UriPartial]::Authority)
$AdminOrigin = $AdminOrigin.TrimEnd("/")
$results = New-Object System.Collections.Generic.List[object]
$adminAuthValues = @($AdminOrigin, $AdminEmail, $AdminPassword)
$adminAuthEnabled = ($adminAuthValues | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }).Count -gt 0

function Add-Pass {
    param([string]$Name, [string]$Detail)
    $script:results.Add([pscustomobject]@{ name = $Name; ok = $true; detail = $Detail })
    Write-Host "[blackbox] PASS $Name - $Detail"
}

function Assert-Condition {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) {
        throw $Message
    }
}

if ($adminAuthEnabled) {
    Assert-Condition (($adminAuthValues | Where-Object { [string]::IsNullOrWhiteSpace($_) }).Count -eq 0) `
        "AdminOrigin, AdminEmail, and AdminPassword must all be provided for authenticated checks."
    $AdminOrigin = ([System.Uri]$AdminOrigin).GetLeftPart([System.UriPartial]::Authority)
}

function Invoke-Probe {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [string]$ContentType = "",
        [string]$Body = ""
    )
    $arguments = @{
        Method              = $Method
        Uri                 = $Uri
        Headers             = $Headers
        TimeoutSec          = 10
        SkipHttpErrorCheck  = $true
        MaximumRedirection  = 0
    }
    if ($ContentType) {
        $arguments.ContentType = $ContentType
    }
    if ($Body) {
        $arguments.Body = $Body
    }
    return Invoke-WebRequest @arguments
}

function Convert-JsonBody {
    param($Response)
    try {
        return $Response.Content | ConvertFrom-Json -Depth 30
    } catch {
        throw "Response from $($Response.BaseResponse.RequestMessage.RequestUri) is not valid JSON."
    }
}

function Get-HeaderValues {
    param($Response, [string]$Name)
    try {
        return @($Response.Headers.GetValues($Name))
    } catch {
        $value = $Response.Headers[$Name]
        if ($null -eq $value) { return @() }
        return @($value)
    }
}

function Assert-ErrorResponse {
    param($Response, [int]$Status, [string]$Code, [string]$Label)
    Assert-Condition ([int]$Response.StatusCode -eq $Status) `
        "$Label returned $($Response.StatusCode), expected $Status."
    $payload = Convert-JsonBody $Response
    Assert-Condition ([string]$payload.error.code -eq $Code) `
        "$Label returned error code '$($payload.error.code)', expected '$Code'."
}

$health = Invoke-Probe -Method GET -Uri "$ApiBase/healthz"
Assert-Condition ([int]$health.StatusCode -eq 200) "API healthz returned $($health.StatusCode)."
$healthJSON = Convert-JsonBody $health
Assert-Condition ($healthJSON.data.status -eq "ok") "API health contract is invalid."
Assert-Condition ([string]$health.Headers["Content-Type"] -match '^application/json') "API health content type is not JSON."
Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$health.Headers["X-Request-ID"])) "API health response is missing X-Request-ID."
Assert-Condition ([string]$health.Headers["X-Request-ID"] -eq [string]$healthJSON.request_id) "API request ID header/body mismatch."
Add-Pass "api-health" "200 status=ok"

$ready = Invoke-Probe -Method GET -Uri "$ApiBase/readyz"
Assert-Condition ([int]$ready.StatusCode -eq 200) "API readyz returned $($ready.StatusCode)."
$readyJSON = Convert-JsonBody $ready
Assert-Condition ($readyJSON.data.status -eq "ready") "API readiness contract is invalid."
Add-Pass "api-readiness" "200 status=ready"

$posts = Invoke-Probe -Method GET -Uri "$ApiBase/api/v1/public/posts"
Assert-Condition ([int]$posts.StatusCode -eq 200) "Public posts returned $($posts.StatusCode)."
$postsJSON = Convert-JsonBody $posts
Assert-Condition ($null -ne $postsJSON.data) "Public posts response is missing data."
Add-Pass "public-posts" "200 response envelope present"

$pages = Invoke-Probe -Method GET -Uri "$ApiBase/api/v1/public/pages"
Assert-Condition ([int]$pages.StatusCode -eq 200) "Public pages returned $($pages.StatusCode)."
$pagesJSON = Convert-JsonBody $pages
Assert-Condition ($null -ne $pagesJSON.data) "Public pages response is missing data."
Add-Pass "public-pages" "200 response envelope present"

$categories = Invoke-Probe -Method GET -Uri "$ApiBase/api/v1/public/categories"
Assert-Condition ([int]$categories.StatusCode -eq 200) "Public categories returned $($categories.StatusCode)."
$categoriesJSON = Convert-JsonBody $categories
Assert-Condition ($null -ne $categoriesJSON.data) "Public categories response is missing data."
Add-Pass "public-categories" "200 response envelope present"

$tags = Invoke-Probe -Method GET -Uri "$ApiBase/api/v1/public/tags"
Assert-Condition ([int]$tags.StatusCode -eq 200) "Public tags returned $($tags.StatusCode)."
$tagsJSON = Convert-JsonBody $tags
Assert-Condition ($null -ne $tagsJSON.data) "Public tags response is missing data."
Add-Pass "public-tags" "200 response envelope present"

$publicSettings = Invoke-Probe -Method GET -Uri "$ApiBase/api/v1/public/site/public-settings"
Assert-Condition ([int]$publicSettings.StatusCode -eq 200) "Public site settings returned $($publicSettings.StatusCode)."
$publicSettingsJSON = Convert-JsonBody $publicSettings
Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$publicSettingsJSON.data.hash)) "Public site settings hash is missing."
Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$publicSettingsJSON.data.settings.site.title)) "Public site title is missing."
Assert-Condition ($null -ne $publicSettingsJSON.data.settings.comments.enabled) "Public comments switch is missing."
Add-Pass "public-site-settings" "200 settings snapshot with hash"

$missingPost = Invoke-Probe -Method GET -Uri "$ApiBase/api/v1/public/posts/blackbox-definitely-missing"
Assert-Condition ([int]$missingPost.StatusCode -eq 404) "Missing public post returned $($missingPost.StatusCode), expected 404."
$missingPostJSON = Convert-JsonBody $missingPost
Assert-Condition ($missingPostJSON.error.code -eq "POST_NOT_FOUND") "Missing public post error code is invalid."
Add-Pass "public-post-not-found" "404 POST_NOT_FOUND"

$unauthorized = Invoke-Probe -Method GET -Uri "$ApiBase/api/v1/admin/posts"
Assert-Condition ([int]$unauthorized.StatusCode -eq 401) "Admin endpoint without token returned $($unauthorized.StatusCode), expected 401."
$unauthorizedJSON = Convert-JsonBody $unauthorized
Assert-Condition ($unauthorizedJSON.error.code -eq "UNAUTHORIZED") "Unauthorized response code is invalid."
Add-Pass "admin-auth-boundary" "401 UNAUTHORIZED"

$invalidLogin = Invoke-Probe `
    -Method POST `
    -Uri "$ApiBase/api/v1/admin/auth/login" `
    -ContentType "application/json" `
    -Body '{"email":"nobody@invalid.example","password":"definitely-wrong"}'
Assert-Condition ([int]$invalidLogin.StatusCode -eq 401) "Invalid login returned $($invalidLogin.StatusCode), expected 401."
$invalidLoginJSON = Convert-JsonBody $invalidLogin
Assert-Condition ($invalidLoginJSON.error.code -eq "AUTH_INVALID_CREDENTIALS") "Invalid login response code is invalid."
Add-Pass "invalid-login" "401 AUTH_INVALID_CREDENTIALS"

$invalidPayload = Invoke-Probe `
    -Method POST `
    -Uri "$ApiBase/api/v1/admin/auth/login" `
    -ContentType "application/json" `
    -Body '{}'
Assert-Condition ([int]$invalidPayload.StatusCode -eq 422) "Malformed login returned $($invalidPayload.StatusCode), expected 422."
Add-Pass "login-validation" "422 validation boundary"

$corsHeaders = @{
    Origin = $SiteOrigin
    "Access-Control-Request-Method" = "GET"
}
$allowedCORS = Invoke-Probe -Method OPTIONS -Uri "$ApiBase/api/v1/public/posts" -Headers $corsHeaders
Assert-Condition ([int]$allowedCORS.StatusCode -eq 204) "Allowed CORS preflight returned $($allowedCORS.StatusCode)."
Assert-Condition ([string]$allowedCORS.Headers["Access-Control-Allow-Origin"] -eq $SiteOrigin) "Allowed CORS origin header is missing or incorrect."
Add-Pass "cors-allowlist" "allows $SiteOrigin"

$blockedHeaders = @{
    Origin = "https://untrusted.invalid"
    "Access-Control-Request-Method" = "GET"
}
$blockedCORS = Invoke-Probe -Method OPTIONS -Uri "$ApiBase/api/v1/public/posts" -Headers $blockedHeaders
Assert-Condition ([int]$blockedCORS.StatusCode -eq 204) "Unknown CORS preflight returned $($blockedCORS.StatusCode)."
Assert-Condition ([string]::IsNullOrEmpty([string]$blockedCORS.Headers["Access-Control-Allow-Origin"])) "Unknown origin received an allow-origin header."
Add-Pass "cors-deny-unknown" "no allow-origin header"

if ($adminAuthEnabled) {
    $adminPreflight = Invoke-Probe -Method OPTIONS -Uri "$ApiBase/api/v1/admin/auth/me" -Headers @{
        Origin = $AdminOrigin
        "Access-Control-Request-Method" = "GET"
        "Access-Control-Request-Headers" = "Content-Type, X-CSRF-Token"
    }
    Assert-Condition ([int]$adminPreflight.StatusCode -eq 204) "Admin CORS preflight returned $($adminPreflight.StatusCode)."
    Assert-Condition ([string]$adminPreflight.Headers["Access-Control-Allow-Origin"] -eq $AdminOrigin) `
        "Admin CORS allow-origin header is missing or incorrect."
    Assert-Condition ([string]$adminPreflight.Headers["Access-Control-Allow-Credentials"] -eq "true") `
        "Admin CORS response does not allow credentials."
    Add-Pass "admin-cors-credentials" "allows credentialed requests from $AdminOrigin"

    $adminSession = $null
    $csrfToken = ""
    $loginSucceeded = $false
    try {
        $loginResponse = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/login" `
            -Headers @{ Origin = $AdminOrigin } `
            -SessionVariable AuthenticatedAdminSession `
            -ContentType "application/json" `
            -Body (@{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json -Compress) `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck `
            -MaximumRedirection 0
        Assert-Condition ([int]$loginResponse.StatusCode -eq 200) `
            "Authenticated login returned $($loginResponse.StatusCode), expected 200."
        $adminSession = $AuthenticatedAdminSession
        $loginSucceeded = $true
        $loginJSON = Convert-JsonBody $loginResponse
        Assert-Condition ($loginJSON.data.PSObject.Properties.Name -notcontains "access_token") `
            "Authenticated login leaked access_token in the response."
        $csrfToken = [string]$loginJSON.data.csrf_token
        Assert-Condition (-not [string]::IsNullOrWhiteSpace($csrfToken)) "Authenticated login omitted csrf_token."
        $setCookies = @(Get-HeaderValues $loginResponse "Set-Cookie")
        foreach ($cookieName in @("zoking_admin_access", "zoking_admin_csrf")) {
            $cookieHeader = @($setCookies | Where-Object { [string]$_ -match "(?i)^$([regex]::Escape($cookieName))=" })
            Assert-Condition ($cookieHeader.Count -eq 1) `
                "Login did not return exactly one $cookieName cookie."
            $cookieText = [string]$cookieHeader[0]
            Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)Path=/api/v1/admin(?:;|$)') `
                "$cookieName cookie Path is not /api/v1/admin."
            Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)HttpOnly(?:;|$)') `
                "$cookieName cookie is missing HttpOnly."
            Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)SameSite=Strict(?:;|$)') `
                "$cookieName cookie is missing SameSite=Strict."
            if (([System.Uri]$ApiBase).Scheme -eq "https") {
                Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)Secure(?:;|$)') `
                    "$cookieName cookie is missing Secure over HTTPS."
            }
        }
        Add-Pass "admin-login-cookie-contract" "no access_token; scoped HttpOnly SameSite=Strict cookies"

        $meResponse = Invoke-WebRequest `
            -Method GET `
            -Uri "$ApiBase/api/v1/admin/auth/me" `
            -Headers @{ Origin = $AdminOrigin } `
            -WebSession $adminSession `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-Condition ([int]$meResponse.StatusCode -eq 200) "Authenticated auth/me returned $($meResponse.StatusCode)."
        Assert-Condition ([string]$meResponse.Headers["Access-Control-Allow-Origin"] -eq $AdminOrigin) `
            "Authenticated response is missing the Admin CORS origin."
        Assert-Condition ([string]$meResponse.Headers["Access-Control-Allow-Credentials"] -eq "true") `
            "Authenticated response is missing CORS credentials."

        $resumeResponse = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/session" `
            -Headers @{ Origin = $AdminOrigin } `
            -WebSession $adminSession `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-Condition ([int]$resumeResponse.StatusCode -eq 200) `
            "Session resume returned $($resumeResponse.StatusCode), expected 200."
        $resumeJSON = Convert-JsonBody $resumeResponse
        Assert-Condition ($resumeJSON.data.PSObject.Properties.Name -notcontains "access_token") `
            "Session resume leaked access_token in the response."
        $resumedCSRFToken = [string]$resumeJSON.data.csrf_token
        Assert-Condition (-not [string]::IsNullOrWhiteSpace($resumedCSRFToken)) `
            "Session resume omitted csrf_token."
        Assert-Condition ($resumedCSRFToken -ne $csrfToken) `
            "Session resume did not rotate the CSRF token."
        $csrfToken = $resumedCSRFToken

        $negativeLogin = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/login" `
            -Headers @{ Origin = $AdminOrigin } `
            -SessionVariable NegativeAdminSession `
            -ContentType "application/json" `
            -Body (@{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json -Compress) `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-Condition ([int]$negativeLogin.StatusCode -eq 200) "Negative-probe login failed."
        $negativeSession = $NegativeAdminSession
        $negativeLoginJSON = Convert-JsonBody $negativeLogin
        $negativeOriginalCSRF = [string]$negativeLoginJSON.data.csrf_token
        $negativeResume = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/session" `
            -Headers @{ Origin = $AdminOrigin } `
            -WebSession $negativeSession `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-Condition ([int]$negativeResume.StatusCode -eq 200) "Negative-probe session resume failed."
        $negativeResumeJSON = Convert-JsonBody $negativeResume
        $negativeCSRFToken = [string]$negativeResumeJSON.data.csrf_token

        $staleCSRF = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/logout" `
            -Headers @{ Origin = $AdminOrigin; "X-CSRF-Token" = $negativeOriginalCSRF } `
            -WebSession $negativeSession `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-ErrorResponse $staleCSRF 403 "CSRF_FAILED" "Logout with pre-resume CSRF"
        Add-Pass "admin-session-resume" "cookie session restored and CSRF token rotated without JWT exposure"

        $missingCSRF = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/logout" `
            -Headers @{ Origin = $AdminOrigin } `
            -WebSession $negativeSession `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-ErrorResponse $missingCSRF 403 "CSRF_FAILED" "Logout without CSRF"

        $wrongCSRF = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/logout" `
            -Headers @{ Origin = $AdminOrigin; "X-CSRF-Token" = "blackbox-intentionally-wrong-csrf-token" } `
            -WebSession $negativeSession `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-ErrorResponse $wrongCSRF 403 "CSRF_FAILED" "Logout with wrong CSRF"

        $wrongOrigin = "https://blackbox-untrusted.invalid"
        if ($AdminOrigin -eq $wrongOrigin) { $wrongOrigin = "https://blackbox-other-untrusted.invalid" }
        $blockedAdminOrigin = Invoke-WebRequest `
            -Method POST `
            -Uri "$ApiBase/api/v1/admin/auth/logout" `
            -Headers @{ Origin = $wrongOrigin; "X-CSRF-Token" = $negativeCSRFToken } `
            -WebSession $negativeSession `
            -TimeoutSec 10 `
            -SkipHttpErrorCheck
        Assert-ErrorResponse $blockedAdminOrigin 403 "ORIGIN_NOT_ALLOWED" "Logout from untrusted Origin"
        Add-Pass "admin-csrf-boundary" "missing, wrong token, and untrusted Origin rejected"
    } finally {
        if ($loginSucceeded -and $null -ne $adminSession -and -not [string]::IsNullOrWhiteSpace($csrfToken)) {
            $logoutResponse = Invoke-WebRequest `
                -Method POST `
                -Uri "$ApiBase/api/v1/admin/auth/logout" `
                -Headers @{ Origin = $AdminOrigin; "X-CSRF-Token" = $csrfToken } `
                -WebSession $adminSession `
                -TimeoutSec 10 `
                -SkipHttpErrorCheck
            Assert-Condition ([int]$logoutResponse.StatusCode -eq 200) `
                "Final authenticated logout returned $($logoutResponse.StatusCode), expected 200."
            $logoutCookies = @(Get-HeaderValues $logoutResponse "Set-Cookie")
            foreach ($cookieName in @("zoking_admin_access", "zoking_admin_csrf")) {
                $cookieHeader = @($logoutCookies | Where-Object { [string]$_ -match "(?i)^$([regex]::Escape($cookieName))=" })
                Assert-Condition ($cookieHeader.Count -eq 1) "Logout did not clear exactly one $cookieName cookie."
                $cookieText = [string]$cookieHeader[0]
                Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)Path=/api/v1/admin(?:;|$)') `
                    "Cleared $cookieName cookie Path is not /api/v1/admin."
                Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)Max-Age=0(?:;|$)') `
                    "Cleared $cookieName cookie is missing Max-Age=0."
                Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)HttpOnly(?:;|$)') `
                    "Cleared $cookieName cookie is missing HttpOnly."
                Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)SameSite=Strict(?:;|$)') `
                    "Cleared $cookieName cookie is missing SameSite=Strict."
                if (([System.Uri]$ApiBase).Scheme -eq "https") {
                    Assert-Condition ($cookieText -match '(?i)(?:^|;\s*)Secure(?:;|$)') `
                        "Cleared $cookieName cookie is missing Secure over HTTPS."
                }
            }
            $afterLogout = Invoke-WebRequest `
                -Method GET `
                -Uri "$ApiBase/api/v1/admin/auth/me" `
                -Headers @{ Origin = $AdminOrigin } `
                -WebSession $adminSession `
                -TimeoutSec 10 `
                -SkipHttpErrorCheck
            Assert-ErrorResponse $afterLogout 401 "UNAUTHORIZED" "auth/me after logout"
            Add-Pass "admin-csrf-success-logout" "matching CSRF accepted, cookies expired, and session rejected"
        }
    }
}

$site = Invoke-Probe -Method GET -Uri "$SiteBase/"
Assert-Condition ([int]$site.StatusCode -eq 200) "Site home returned $($site.StatusCode)."
Assert-Condition ($site.Content -match '<html[^>]+lang=(["'']?)zh') "Site home is not marked as Chinese."
Assert-Condition ($site.Content -match '<link[^>]+rel=(["'']?)canonical') "Site home is missing canonical metadata."
Assert-Condition ($site.Content -match '<main\b') "Site home is missing main content."
Add-Pass "site-home" "200 Chinese HTML with canonical and main"

$robots = Invoke-Probe -Method GET -Uri "$SiteBase/robots.txt"
Assert-Condition ([int]$robots.StatusCode -eq 200) "robots.txt returned $($robots.StatusCode)."
Assert-Condition ($robots.Content -match '(?im)^sitemap:\s*https?://') "robots.txt is missing an absolute sitemap declaration."
Add-Pass "site-robots" "200 absolute sitemap declaration"

$sitemap = Invoke-Probe -Method GET -Uri "$SiteBase/sitemap.xml"
Assert-Condition ([int]$sitemap.StatusCode -eq 200) "sitemap.xml returned $($sitemap.StatusCode)."
Assert-Condition ($sitemap.Content -match '<(sitemapindex|urlset)\b') "sitemap.xml root is invalid."
Add-Pass "site-sitemap" "200 valid sitemap root"

$siteNotFound = Invoke-Probe -Method GET -Uri "$SiteBase/blackbox-definitely-missing/"
Assert-Condition ([int]$siteNotFound.StatusCode -eq 404) "Site missing page returned $($siteNotFound.StatusCode), expected 404."
Assert-Condition ($siteNotFound.Content -match '(?i)noindex') "Site 404 is missing noindex metadata."
Assert-Condition ($siteNotFound.Content -match '页面未找到|找不到|返回首页') "Site 404 is not localized in Chinese."
Assert-Condition ($siteNotFound.Content -match '<main\b') "Site 404 is missing main content."
Add-Pass "site-not-found" "404 Chinese noindex page"

$adminRoot = Invoke-Probe -Method GET -Uri "$AdminBase/"
Assert-Condition ([int]$adminRoot.StatusCode -eq 200) "Admin root returned $($adminRoot.StatusCode)."
Assert-Condition ($adminRoot.Content -match 'Zoking 内容管理后台') "Admin root title is missing."
Add-Pass "admin-root" "200 localized shell"

$adminRoute = Invoke-Probe -Method GET -Uri "$AdminBase/dashboard"
Assert-Condition ([int]$adminRoute.StatusCode -eq 200) "Admin route fallback returned $($adminRoute.StatusCode)."
Assert-Condition ($adminRoute.Content -match '<div id="root"></div>') "Admin route fallback did not return the SPA shell."
Add-Pass "admin-spa-route" "200 dashboard fallback"

$runtimeConfig = Invoke-Probe -Method GET -Uri "$AdminBase/runtime-config.js"
Assert-Condition ([int]$runtimeConfig.StatusCode -eq 200) "Admin runtime config returned $($runtimeConfig.StatusCode)."
Assert-Condition ($runtimeConfig.Content -match '__ZOKING_ADMIN_CONFIG__') "Admin runtime config contract is missing."
Add-Pass "admin-runtime-config" "200 runtime config contract"

$adminProxyHealth = Invoke-Probe -Method GET -Uri "$AdminBase/healthz"
Assert-Condition ([int]$adminProxyHealth.StatusCode -eq 200) "Admin dev proxy health returned $($adminProxyHealth.StatusCode)."
Add-Pass "admin-api-proxy" "200 health proxy"

if ($TestRateLimit) {
    Assert-Condition ($RateLimitAttempts -ge 2) "RateLimitAttempts must be at least 2."
    $statuses = New-Object System.Collections.Generic.List[int]
    for ($attempt = 1; $attempt -le $RateLimitAttempts; $attempt++) {
        $response = Invoke-Probe `
            -Method POST `
            -Uri "$ApiBase/api/v1/public/posts/blackbox-no-write/comments" `
            -ContentType "application/json" `
            -Body '{"author_name":'
        $statuses.Add([int]$response.StatusCode)
    }
    Assert-Condition (-not ($statuses | Where-Object { $_ -ge 200 -and $_ -lt 300 })) "Rate-limit probe unexpectedly persisted a comment."
    Assert-Condition ($statuses.Contains(422)) "Rate-limit probe did not reach request validation."
    Assert-Condition ($statuses.Contains(429)) "Rate-limit probe did not observe HTTP 429; statuses=$($statuses -join ',')."
    Add-Pass "comment-rate-limit" "observed 422 then 429 without successful writes"
}

[pscustomobject]@{
    ok = $true
    api_base = $ApiBase
    admin_base = $AdminBase
    site_base = $SiteBase
    rate_limit_tested = [bool]$TestRateLimit
    authenticated_admin_tested = [bool]$adminAuthEnabled
    passed = $results.Count
    results = @($results | ForEach-Object { $_ })
} | ConvertTo-Json -Depth 8
