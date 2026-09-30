#!/usr/bin/env bash
# IMP-106 / CI-003: append one cached-step telemetry entry to the JSONL file at
# ${THINHTHAN_CACHE_TELEMETRY:-$RUNNER_TEMP/cache-telemetry.jsonl}. Verify.ps1
# folds the file into verify-report.json (cached_steps[]). Source this file,
# then call cache_telemetry_emit <step> <hit|miss> <wall_seconds>.
cache_telemetry_emit() {
  local step="$1" result="$2" wall="$3"
  case "$result" in
    hit|miss) ;;
    *) printf 'cache_telemetry_emit: result must be hit|miss, got %s\n' "$result" >&2; return 1 ;;
  esac
  case "$wall" in
    ''|*[!0-9.]*) printf 'cache_telemetry_emit: wall_seconds must be numeric, got %s\n' "$wall" >&2; return 1 ;;
  esac
  local file="${THINHTHAN_CACHE_TELEMETRY:-${RUNNER_TEMP:-/tmp}/cache-telemetry.jsonl}"
  mkdir -p "$(dirname "$file")"
  printf '{"step":"%s","result":"%s","wall_seconds":%s}\n' "$step" "$result" "$wall" >> "$file"
}
