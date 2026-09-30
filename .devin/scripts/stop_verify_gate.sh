#!/usr/bin/env bash
# Stop hook — full diff-aware verification gate before the agent ends its turn.
#
# Loop safety: when the payload's stop_hook_active is true the hook never
# blocks again; it emits a warning context instead, so the agent cannot get
# trapped re-running checks it cannot fix.
cd "$(dirname "$0")/../.." 2>/dev/null || true
. "$(dirname "$0")/lib.sh"
read_stdin

already="$(json_bool .stop_hook_active)"

# Nothing changed -> nothing to verify.
if ! git status --porcelain 2>/dev/null | grep -q .; then
  exit 0
fi

out="$(bash .devin/scripts/verify_delta.sh --full --quiet 2>&1)"
rc=$?

if [ "$rc" -eq 0 ]; then
  warnings="$(printf '%s\n' "$out" | grep '^WARN' | head -8 || true)"
  [ -n "$warnings" ] && note Stop "Verification passed with local-only warnings; do not present deferred Q6 cleanliness as clean-source CI evidence:
$warnings"
  exit 0
fi

summary="$(printf '%s\n' "$out" | grep -E '^(FAIL|WARN)' | head -12)"
[ -z "$summary" ] && summary="$(printf '%s\n' "$out" | tail -10)"

if [ "$already" = "true" ]; then
  note Stop "VERIFICATION STILL FAILING (not blocking again to avoid a loop). Do not report DONE until these are resolved:
$summary"
fi

block "Verification failed — resolve before ending this turn (run: bash .devin/scripts/verify_delta.sh --full):
$summary"
