#!/usr/bin/env bash
# PreToolUse guard for edit / write / apply_patch / notebook_edit.
# Blocks writes to protected specs, generated code, VCS internals, Unity build
# artifacts, vendored deps, and already-committed migration files.
cd "$(dirname "$0")/../.." 2>/dev/null || true
. "$(dirname "$0")/lib.sh"
read_stdin

path="$(json_get .tool_input.file_path)"
[ -z "$path" ] && path="$(json_get .tool_input.notebook_path)"
[ -z "$path" ] && exit 0
p="$(norm_path "$path")"
plc="${p,,}"

deny() { block "BLOCKED (write guard): $1 — path: $p"; }

case "$plc" in
  docs/10_implementation|docs/10_implementation/*)
    ;;
  docs|docs/*)
    # Protected specs/ADRs change only in spec-change PRs from the spec-owner role (AGENTS.md, ADR-0050).
    [ "${THINHTHAN_AGENT_ROLE:-}" = "spec-owner" ] \
      || deny "protected spec — only the spec-owner role (THINHTHAN_AGENT_ROLE=spec-owner) may edit docs outside docs/10_implementation/; record the gap in known_blockers.md" ;;
  .devin/*|agents.md|readme.md)
    [ "${THINHTHAN_AGENT_ROLE:-}" = "spec-owner" ] \
      || deny "protected governance file — only the spec-owner role may edit .devin/, AGENTS.md, README.md (spec-change PR + policy-review)" ;;
  .git/*|*/.git/*)
    deny "never write inside .git/" ;;
  client/library/*|client/temp/*|client/logs/*|client/obj/*|client/build/*|client/builds/*|*/library/*|*/temp/*|*/obj/*)
    deny "Unity Library/Temp/Logs/obj/Build output is generated — never edited" ;;
  server/vendor/*)
    deny "vendor/ is managed by the Go toolchain — never hand-edited" ;;
  tools/*)
    deny "tools/ is a downloaded pinned-toolchain cache — regenerate via approved scripts" ;;
esac

case "$plc" in
  *.pb.go|server/internal/protocol/*)
    deny "generated protobuf Go — edit proto/thinhthan/v1/*.proto and run scripts/codegen.ps1" ;;
  client/assets/scripts/protocol/*|client/assets.meta|client/assets/scripts.meta|client/assets/scripts/protocol.meta)
    deny "generated protobuf C#/Unity metadata — edit proto source and run scripts/codegen.ps1" ;;
  proto/testdata/golden/*)
    deny "golden wire fixtures are generated test outputs — use the approved update-golden test flow" ;;
esac

# Committed migration files are immutable (docs/06_data/migrations.md).
# New files (including staged additions) are allowed; paths present in HEAD are blocked.
if printf '%s' "$p" | grep -Eq '(^|/)server/migrations/[0-9]{6}_[A-Za-z0-9_]+\.(up|down)\.sql$'; then
  if is_committed "$p"; then
    deny "migration files are immutable once committed — create the next numbered .up/.down pair instead"
  fi
fi

exit 0
