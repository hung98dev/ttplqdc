#!/usr/bin/env bash
# SessionStart hook — inject a compact repo-state brief so every session
# starts knowing what exists, what is blocked, and what verify command works.
cd "$(dirname "$0")/../.." 2>/dev/null || true
. "$(dirname "$0")/lib.sh"
read_stdin

# --- phase detection -----------------------------------------------------------
present=""; absent=""
for d in server client proto scripts deploy .github; do
  if [ -d "$d" ]; then present="$present $d/"; else absent="$absent $d/"; fi
done
if [ -z "$present" ]; then
  phase="spec-only"
else
  foundation=""
  [ -f scripts/verify.ps1 ] && foundation="$foundation IMP-000"
  [ -d server/internal/core/id ] && foundation="$foundation IMP-001"
  [ -f scripts/codegen.ps1 ] && [ -d proto/thinhthan/v1 ] && foundation="$foundation IMP-061"
  phase="implementation active (foundation present:${foundation:- none}; task status is canonical in docs/10_implementation/task_queue.md)"
fi

# --- toolchain ------------------------------------------------------------------
tools=""
if has_cmd go; then
  gv="$(go version 2>/dev/null | grep -oE 'go[0-9.]+' | head -1)"
  if [ "$gv" = "go1.27.1" ]; then tools="$tools go=1.27.1(ok)"; else tools="$tools go=${gv#go}(PIN-MISMATCH expect 1.27.1)"; fi
else
  tools="$tools go=MISSING"
fi
if has_cmd protoc; then
  pv="$(protoc --version 2>/dev/null | grep -oE '[0-9.]+' | head -1)"
  [ "$pv" = "36.2" ] && tools="$tools protoc=36.2(global)" || tools="$tools protoc=${pv:-unknown}(PIN-MISMATCH expect 36.2)"
elif [ -x tools/protobuf/protoc/bin/protoc ] || [ -f tools/protobuf/protoc/bin/protoc.exe ]; then
  tools="$tools protoc=36.2(managed-cache)"
elif [ -f scripts/codegen.ps1 ]; then
  tools="$tools protoc=36.2(managed-bootstrap)"
else
  tools="$tools protoc=MISSING"
fi
unity_editor="$(unity_editor_path || true)"
[ -n "$unity_editor" ] && tools="$tools unity=6000.6.1f1(ok)" || tools="$tools unity=MISSING(6000.6.1f1)"

# --- governance state -----------------------------------------------------------
blk=0
if [ -f docs/10_implementation/known_blockers.md ]; then
  blk="$(grep -c '^### `BLK-' docs/10_implementation/known_blockers.md 2>/dev/null || true)"
  blk="${blk:-0}"
fi
dirty="$(changed_files | wc -l | tr -d ' ')"
branch="$(git branch --show-current 2>/dev/null || echo '?')"
if [ -f scripts/verify.ps1 ]; then verify="pwsh -NoProfile -File scripts/verify.ps1 -LocalDeferMissing (canonical Q0-Q6; CI = GitHub-hosted Linux + Windows jobs, authoritative, materializes Unity files) + .devin/scripts/verify_delta.sh --full (local diff supplements)"; else verify="none yet — repository is docs-only; IMP-000 creates scripts/verify.ps1"; fi

note SessionStart "thinhthan repo state:
- phase: $phase | present:${present:-none} | absent:${absent:-none}
- tools:${tools}
- open blockers: $blk (docs/10_implementation/known_blockers.md) — check before claiming tasks
- git: branch=$branch, dirty_files=$dirty
- verify: $verify
- role: ${THINHTHAN_AGENT_ROLE:-implementer} | docs policy: read all; implementers write only docs/10_implementation/; protected specs change only via spec-owner spec-change PRs
- read order: AGENTS.md -> docs/README.md -> technology_versions.md -> owning spec + ADRs -> docs/10_implementation/; handbook: .devin/HANDBOOK.md"
