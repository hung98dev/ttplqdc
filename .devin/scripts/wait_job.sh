#!/usr/bin/env bash
# Wait until the job named $WAIT_JOB of this workflow run attempt completes and
# write its conclusion to $GITHUB_OUTPUT (ADR-0075). Used by the required
# verify jobs to join their parallel Unity job and by the Linux job to join
# the Windows report for the evidence manifest. Polls every 10 s (GITHUB_TOKEN
# budget: ~1000 requests/hour/repository); the job timeout bounds the wait.
set -euo pipefail
: "${WAIT_JOB:?WAIT_JOB is required}"
url="repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/attempts/${GITHUB_RUN_ATTEMPT}/jobs?per_page=100"
state=""
while :; do
  state=$(gh api "$url" --jq ".jobs[] | select(.name == \"${WAIT_JOB}\") | .status + \" \" + (.conclusion // \"\")" 2>/dev/null || true)
  case "$state" in
    completed*) break ;;
  esac
  sleep 10
done
conclusion="${state#completed }"
echo "${WAIT_JOB}: ${conclusion}"
echo "conclusion=${conclusion}" >> "$GITHUB_OUTPUT"
