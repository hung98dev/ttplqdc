#!/usr/bin/env pwsh
# Canonical Q0-Q6 verifier front-end (scripts/verify.ps1, IMP-000).
# Requires pwsh 7.6.6+; drives `go run ./cmd/verify` and provisions the
# Postgres test instance for DB gates when not already provided.
#Requires -Version 7.6
[CmdletBinding()]
param(
    [string]$RepoRoot,
    [ValidateSet('all','pre-unity','unity')][string]$Phase = 'all',
    [string]$PreReport,
    [string]$UnityResultsDir,
    [string]$ReportOut,
    [switch]$LocalDeferMissing,
    [switch]$MergeReports,
    [string]$LinuxReport,
    [string]$WindowsReport,
    [string]$OutManifest
)

$ErrorActionPreference = 'Stop'

# pwsh floor: 7.6.6 (technology_versions.md). Windows PowerShell 5.1 is rejected.
if ($PSVersionTable.PSVersion -lt [version]'7.6.6') {
    throw "pwsh 7.6.6+ required, found $($PSVersionTable.PSVersion)"
}
if ($env:CI -and $LocalDeferMissing) {
    throw "-LocalDeferMissing is forbidden under CI"
}

if (-not $RepoRoot) {
    $RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
}
$RepoRoot = (Resolve-Path $RepoRoot).Path
$serverDir = Join-Path $RepoRoot 'server'

# Pinned-editor sanity: a set UNITY_EDITOR_PATH must point at 6000.6.1f1.
if ($env:UNITY_EDITOR_PATH -and $env:UNITY_EDITOR_PATH -notlike '*6000.6.1f1*') {
    throw "UNITY_EDITOR_PATH must reference the pinned editor 6000.6.1f1: $env:UNITY_EDITOR_PATH"
}

function Resolve-TestPgDsn {
    if ($env:THINHTHAN_TEST_PG_DSN) { return $env:THINHTHAN_TEST_PG_DSN }
    $toolsPg = Join-Path $RepoRoot 'tools/pgsql'
    $pgIsReady = Join-Path $toolsPg 'bin/pg_isready.exe'
    if (-not (Test-Path $pgIsReady) -and $IsWindows) {
        # EDB pinned binaries (URL + sha256 resolve through stackpin so the
        # hash lives in exactly one place).
        Push-Location $serverDir
        try {
            $url = (& go run ./cmd/verify -repo-root $RepoRoot -print-pin edb-url)
            $sha = (& go run ./cmd/verify -repo-root $RepoRoot -print-pin edb-sha256)
        } finally { Pop-Location }
        if (-not $url -or -not $sha) { throw 'EDB pin unavailable via -print-pin' }
        $zip = Join-Path ([IO.Path]::GetTempPath()) 'edb-pg.zip'
        Write-Host "verify: fetching pinned EDB postgres binaries"
        Invoke-WebRequest -Uri $url -OutFile $zip -TimeoutSec 300
        $got = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($got -ne $sha.ToLowerInvariant()) {
            Remove-Item $zip -Force -ErrorAction SilentlyContinue
            throw "EDB zip sha256 mismatch: got $got expected $($sha.ToLowerInvariant())"
        }
        Expand-Archive -Path $zip -DestinationPath $toolsPg -Force
        Remove-Item $zip -Force -ErrorAction SilentlyContinue
        # The EDB zip nests everything under pgsql/; flatten to tools/pgsql.
        $nested = Join-Path $toolsPg 'pgsql'
        if (Test-Path $nested) {
            Get-ChildItem -Force $nested | Move-Item -Destination $toolsPg -Force
            Remove-Item $nested -Recurse -Force
        }
    }
    if (Test-Path $toolsPg) {
        # EDB binaries unpacked to ignored tools/pgsql; start on a random port.
        if (-not (Test-Path $pgIsReady)) {
            throw "tools/pgsql exists but is incomplete (missing $pgIsReady)"
        }
        $port = Get-Random -Minimum 20000 -Maximum 50000
        $data = Join-Path $toolsPg 'data'
        if (-not (Test-Path $data)) {
            & (Join-Path $toolsPg 'bin/initdb.exe') -D $data -E UTF8 -A trust | Out-Null
        }
        & (Join-Path $toolsPg 'bin/pg_ctl.exe') -D $data -o "-p $port" -l (Join-Path $toolsPg 'pg.log') start | Out-Null
        return "postgres://postgres@localhost:$port/postgres?sslmode=disable"
    }
    if ($env:RUNNER_OS -ne 'Windows' -or $IsLinux) {
        $docker = Get-Command docker -ErrorAction SilentlyContinue
        if ($docker) {
            # pinned digest image from technology_versions.md
            $img = 'postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722'
            $port = Get-Random -Minimum 20000 -Maximum 50000
            & docker run -d -e POSTGRES_PASSWORD=postgres -p "${port}:5432" $img | Out-Null
            Start-Sleep -Seconds 3
            return "postgres://postgres:postgres@localhost:$port/postgres?sslmode=disable"
        }
    }
    if ($LocalDeferMissing) { return $null }
    throw 'no PostgreSQL test server: set THINHTHAN_TEST_PG_DSN, provision tools/pgsql, or install docker'
}

if ($MergeReports) {
    # Evidence merge early-exits on branches without an IMP-\d+ task reference
    # (claim/ops/spec/status PRs): manifest generation is skipped, never a gate.
    $branch = $env:GITHUB_HEAD_REF
    if (-not $branch) { $branch = $env:GITHUB_REF_NAME }
    if (-not $branch) { $branch = (& git -C $RepoRoot branch --show-current) }
    if ($branch -notmatch 'IMP-\d+') {
        Write-Host "verify merge: head '$branch' has no IMP-N task — skipping manifest generation"
        $linuxOk = (Test-Path $LinuxReport); $winOk = (Test-Path $WindowsReport)
        if (-not ($linuxOk -and $winOk)) { throw 'both reports required for merge' }
        exit 0
    }
    Push-Location $serverDir
    try {
        $margs = @('run', './cmd/verify', '-repo-root', $RepoRoot, '-merge',
            '-linux', (Resolve-Path $LinuxReport).Path,
            '-windows', (Resolve-Path $WindowsReport).Path,
            '-out', $OutManifest)
        & go @margs
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally { Pop-Location }
    exit 0
}

$dsn = Resolve-TestPgDsn
if ($dsn) { $env:THINHTHAN_TEST_PG_DSN = $dsn }

Push-Location $serverDir
try {
    $args = @('run', './cmd/verify', '-repo-root', $RepoRoot, '-phase', $Phase)
    if ($PreReport) { $args += @('-pre-report', $PreReport) }
    if ($UnityResultsDir) { $args += @('-unity-results-dir', $UnityResultsDir) }
    if ($ReportOut) { $args += @('-report-out', $ReportOut) }
    if ($LocalDeferMissing) { $args += '-local-defer-missing' }
    & go @args
    exit $LASTEXITCODE
} finally { Pop-Location }
