#!/usr/bin/env bash
# PostToolUse hook for edit / write / apply_patch.
# Runs fast per-file checks and injects a short context nudge when needed.
# Never blocks (always exit 0). Heavy verification lives in the Stop gate.
cd "$(dirname "$0")/../.." 2>/dev/null || true
. "$(dirname "$0")/lib.sh"
read_stdin

path="$(json_get .tool_input.file_path)"
[ -z "$path" ] && exit 0
p="$(norm_path "$path")"
[ -f "$p" ] || exit 0

case "$p" in
  *.go)
    # gofmt drift on the edited file (instant, no build).
    if has_cmd gofmt; then
      if [ -n "$(gofmt -l "$p" 2>/dev/null)" ]; then
        note PostToolUse "GOFMT: $p is not gofmt-clean. Run: gofmt -w $p"
      fi
    fi
    # Forbidden imports on the edited file only.
    bad="$(grep -nE '"(math/rand"|github\.com/(gorilla|gin-gonic|go-chi|labstack/echo|gofiber|redis/go-redis|go-redis|segmentio/kafka-go|confluentinc|nats-io|jmoiron/sqlx)|gorm\.io|entgo\.io|go\.uber\.org/zap|sirupsen/logrus|rs/zerolog|google\.golang\.org/grpc|golang\.org/x/time/rate)' "$p" 2>/dev/null | head -3 || true)"
    [ -n "$bad" ] && note PostToolUse "FORBIDDEN IMPORT in $p (AGENTS.md forbid list): $bad"
    ;;

  proto/thinhthan/v1/*.proto|proto/**/*.proto)
    note PostToolUse "WIRE CONTRACT: field numbers are immutable. Run scripts/codegen.ps1, update affected golden fixtures with the protocol test flow, then run verify_delta.sh --full." ;;

  server/go.mod|server/go.sum|client/Packages/manifest.json|client/Packages/packages-lock.json|client/ProjectSettings/ProjectVersion.txt)
    note PostToolUse "PIN CHECK: $p changed — every version must equal docs/00_context/technology_versions.md exactly. No floating/latest/prerelease pins." ;;

  *.asmdef)
    note PostToolUse "ASMDEF: assembly references must stay acyclic; ThinhThan.Protocol references no other project assembly." ;;

  client/Assets/Art/*|client/Assets/Audio/*)
    note PostToolUse "ART/AUDIO: finish at exact 2x size, declare asset_class, pass Cutout + Volume & Depth gates, capture Visual Review screenshots, add provenance row (/produce-art-asset)." ;;

  *.unity|*.prefab|*.asset|*.meta|*.mat|*.controller|*.anim)
    note PostToolUse "SERIALIZED ASSET: $p changed — confirm it is inside task scope; unrelated YAML churn fails review." ;;

  */migrations/*.sql)
    note PostToolUse "MIGRATION: server/migrations/ only; new monotonic NNNNNN_name pair (.up + .down); committed migrations are immutable; 000001 is the baseline owned by IMP-005." ;;

  docs/10_implementation/task_queue.md|docs/10_implementation/*.md)
    note PostToolUse "TASK PACKET: status DONE requires docs/10_implementation/evidence/IMP-XXX/manifest.json with ci_run_id (ADR-0045). Verify fail = not DONE." ;;

  client/**/*.cs)
    hot="$(grep -nE 'FindObjectOfType|FindObjectsOfType|FindFirstObjectByType|FindAnyObjectByType|GameObject\.Find|Camera\.main|using System\.Linq|StartCoroutine|IEnumerator|async void|SendMessage|BroadcastMessage|InvokeRepeating|Resources\.Load|Debug\.Log|new Material\(|\.material\b|GC\.Collect|void (Update|FixedUpdate|LateUpdate|OnGUI)\(' "$p" 2>/dev/null | head -3 || true)"
    [ -n "$hot" ] && note PostToolUse "CLIENT API FENCE in $p: $hot — forbidden in first-party runtime code (engineering_conventions.md §2.5; Q4 fails). Use FrameLoop/IFrameSystem, FrameBudget, Pool<T>, Log, injected services."
    ;;
esac

exit 0
