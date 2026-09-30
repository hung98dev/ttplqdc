# Posts or updates the `policy-review` check run for one PR head SHA as the
# GitHub App thinhthan-policy-reviewer (ADR-0072). Reviewer role only.
# Requires PowerShell 7.6.6 (pinned) and no extra modules.
#   env THINHTHAN_POLICY_APP_ID        numeric App ID
#   env THINHTHAN_POLICY_APP_KEY_FILE  path of the App private key (.pem); read only here
# Usage:
#   pwsh -NoProfile -File .devin/scripts/policy_review.ps1 -Repo <owner/repo> -Sha <40-hex> -Conclusion success|failure -SummaryFile <path>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$')][string]$Repo,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{40}$')][string]$Sha,
    [Parameter(Mandatory)][ValidateSet('success', 'failure')][string]$Conclusion,
    [Parameter(Mandatory)][string]$SummaryFile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$CheckName = 'policy-review'
$ApiRoot = 'https://api.github.com'
$MaxSummaryChars = 65000
$JwtLifetimeSeconds = 540
$ClockSkewSeconds = 60

function Get-RequiredEnv([string]$Name)
{
    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value))
    {
        throw "missing environment variable $Name"
    }
    return $value
}

function ConvertTo-Base64Url([byte[]]$Bytes)
{
    return [Convert]::ToBase64String($Bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function New-AppJwt([long]$AppId, [string]$KeyFile)
{
    $now = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    $header = ConvertTo-Base64Url ([Text.Encoding]::UTF8.GetBytes('{"alg":"RS256","typ":"JWT"}'))
    $claims = [ordered]@{ iat = $now - $ClockSkewSeconds; exp = $now + $JwtLifetimeSeconds; iss = $AppId }
    $payload = ConvertTo-Base64Url ([Text.Encoding]::UTF8.GetBytes(($claims | ConvertTo-Json -Compress)))
    $signingInput = [Text.Encoding]::ASCII.GetBytes("$header.$payload")
    $rsa = [Security.Cryptography.RSA]::Create()
    try
    {
        $rsa.ImportFromPem((Get-Content -Raw -LiteralPath $KeyFile))
        $signature = $rsa.SignData(
            $signingInput,
            [Security.Cryptography.HashAlgorithmName]::SHA256,
            [Security.Cryptography.RSASignaturePadding]::Pkcs1)
    }
    finally
    {
        $rsa.Dispose()
    }
    return "$header.$payload.$(ConvertTo-Base64Url $signature)"
}

function Invoke-GitHub([string]$Method, [string]$Path, [string]$Token, [string]$Scheme, $Body = $null)
{
    $headers = @{
        Authorization          = "$Scheme $Token"
        Accept                 = 'application/vnd.github+json'
        'X-GitHub-Api-Version' = '2022-11-28'
        'User-Agent'           = 'thinhthan-policy-reviewer'
    }
    $request = @{ Method = $Method; Uri = "$ApiRoot$Path"; Headers = $headers }
    if ($null -ne $Body)
    {
        $request.Body = ($Body | ConvertTo-Json -Depth 5 -Compress)
        $request.ContentType = 'application/json'
    }
    return Invoke-RestMethod @request
}

$appId = [long](Get-RequiredEnv 'THINHTHAN_POLICY_APP_ID')
$keyFile = Get-RequiredEnv 'THINHTHAN_POLICY_APP_KEY_FILE'
if (-not (Test-Path -LiteralPath $SummaryFile -PathType Leaf))
{
    throw "summary file not found: $SummaryFile"
}
$summary = Get-Content -Raw -LiteralPath $SummaryFile
if ([string]::IsNullOrWhiteSpace($summary))
{
    throw 'summary file is empty; the verdict and checked-spec list are required'
}
if ($summary.Length -gt $MaxSummaryChars)
{
    $summary = $summary.Substring(0, $MaxSummaryChars)
}

$jwt = New-AppJwt -AppId $appId -KeyFile $keyFile
$installation = Invoke-GitHub -Method Get -Path "/repos/$Repo/installation" -Token $jwt -Scheme 'Bearer'
$repoName = $Repo.Split('/')[1]
$tokenBody = @{ repositories = @($repoName); permissions = @{ checks = 'write' } }
$access = Invoke-GitHub -Method Post -Path "/app/installations/$($installation.id)/access_tokens" -Token $jwt -Scheme 'Bearer' -Body $tokenBody
$token = $access.token

$output = @{ title = "policy-review: $Conclusion"; summary = $summary }
$completedAt = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$existing = Invoke-GitHub -Method Get -Path "/repos/$Repo/commits/$Sha/check-runs?check_name=$CheckName&filter=latest" -Token $token -Scheme 'token'
$own = @($existing.check_runs | Where-Object { $_.app.id -eq $appId -and $_.name -eq $CheckName })

if ($own.Count -gt 0)
{
    $body = @{ status = 'completed'; conclusion = $Conclusion; completed_at = $completedAt; output = $output }
    $run = Invoke-GitHub -Method Patch -Path "/repos/$Repo/check-runs/$($own[0].id)" -Token $token -Scheme 'token' -Body $body
}
else
{
    $body = @{ name = $CheckName; head_sha = $Sha; status = 'completed'; conclusion = $Conclusion; completed_at = $completedAt; output = $output }
    $run = Invoke-GitHub -Method Post -Path "/repos/$Repo/check-runs" -Token $token -Scheme 'token' -Body $body
}

Write-Output "policy-review $Conclusion posted for $Sha (check run $($run.id))"
