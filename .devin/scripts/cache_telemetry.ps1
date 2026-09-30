# IMP-106 / CI-003: append one cached-step telemetry entry to the JSONL file at
# $env:THINHTHAN_CACHE_TELEMETRY (default $env:RUNNER_TEMP\cache-telemetry.jsonl).
# verify.ps1 folds the file into verify-report.json (cached_steps[]).
# Dot-source this file, then call:
#   Write-CacheTelemetry -Step <name> -Result hit|miss -WallSeconds <seconds>
function Write-CacheTelemetry {
    param(
        [Parameter(Mandatory = $true)][string]$Step,
        [Parameter(Mandatory = $true)][string]$Result,
        [Parameter(Mandatory = $true)][double]$WallSeconds
    )
    if ($Result -notin @('hit', 'miss')) {
        throw "Write-CacheTelemetry: result must be hit|miss, got '$Result'"
    }
    $file = $env:THINHTHAN_CACHE_TELEMETRY
    if (-not $file) {
        if (-not $env:RUNNER_TEMP) { throw 'Write-CacheTelemetry: RUNNER_TEMP unset and no THINHTHAN_CACHE_TELEMETRY' }
        $file = Join-Path $env:RUNNER_TEMP 'cache-telemetry.jsonl'
    }
    $parent = Split-Path $file -Parent
    if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    $wall = $WallSeconds.ToString('F3', [System.Globalization.CultureInfo]::InvariantCulture)
    Add-Content -Path $file -Value ('{"step":"' + $Step + '","result":"' + $Result + '","wall_seconds":' + $wall + '}')
}
