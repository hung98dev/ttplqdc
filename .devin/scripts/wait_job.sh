#!/usr/bin/env bash
# Wait until the job named $WAIT_JOB of this workflow run attempt completes and
# write its conclusion to $GITHUB_OUTPUT (ADR-0075, ADR-0078). Used by
# Q0-Q6 verify (Windows) to join Unity (Windows) and Q0-Q6 verify (Linux) for the
# evidence manifest. Polls every 10 s (GITHUB_TOKEN budget: ~1000 requests/hour/repository).
# Bounded by WAIT_TIMEOUT_SECONDS (default 2700 = 45 min) to prevent hanging.
set -euo pipefail
: "${WAIT_JOB:?WAIT_JOB is required}"
timeout_s="${WAIT_TIMEOUT_SECONDS:-2700}"
start_ts=$(date +%s)
url="repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/attempts/${GITHUB_RUN_ATTEMPT}/jobs?per_page=100"
state=""
while :; do
  now=$(date +%s)
  if (( now - start_ts > timeout_s )); then
    echo "wait_job timed out waiting for ${WAIT_JOB} after ${timeout_s}s" >&2
    exit 1
  fi
  resp=$(gh api "$url" --jq ".jobs[] | select(.name == \"${WAIT_JOB}\" or (.name | endswith(\"/ ${WAIT_JOB}\"))) | .status + \" \" + (.conclusion // \"\")" 2>&1 || true)
  if [[ "$resp" =~ (rate.limit|API.rate.limit|403|429) ]]; then
    echo "wait_job: rate limit encountered, backing off 30s..." >&2
    sleep 30
    continue
  fi
  state="$resp"
  case "$state" in
    completed*) break ;;
  esac
  sleep 10
done
conclusion="${state#completed }"
echo "${WAIT_JOB}: ${conclusion}"
if [ -n "${GITHUB_OUTPUT:-}" ]; then
  echo "conclusion=${conclusion}" >> "$GITHUB_OUTPUT"
fi
