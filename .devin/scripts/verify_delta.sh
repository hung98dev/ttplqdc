#!/usr/bin/env bash
# Scope-aware verification for the current worktree diff.
# Usage: verify_delta.sh [--full] [--quiet]
# --full runs canonical Q0-Q6 plus scoped race/Unity/governance supplements.
# Exit 1 means the change is not ready; local dirty-tree Q6 may be deferred only
# when every other canonical check passes and verification creates no new drift.
cd "$(dirname "$0")/../.." 2>/dev/null || true
. "$(dirname "$0")/lib.sh"

FULL=0
QUIET=0
for a in "$@"; do
  case "$a" in
    --full) FULL=1 ;;
    --quiet) QUIET=1 ;;
    *) printf 'FAIL  unknown argument: %s\n' "$a"; exit 1 ;;
  esac
done

FAILS=0
WARNS=0
pass() { [ "$QUIET" -eq 0 ] && printf 'PASS  %s\n' "$1" || true; }
fail() { printf 'FAIL  %s\n' "$1"; FAILS=$((FAILS+1)); }
warn() { printf 'WARN  %s\n' "$1"; WARNS=$((WARNS+1)); }
skip() { [ "$QUIET" -eq 0 ] && printf 'SKIP  %s\n' "$1" || true; }
hdr()  { [ "$QUIET" -eq 0 ] && printf '\n== %s ==\n' "$1" || true; }

run_limited() {
  local limit="$1"
  shift
  if has_cmd timeout; then timeout "$limit" "$@"; else "$@"; fi
}

generated_snapshot() {
  git ls-files --cached --others --exclude-standard -- \
    server/internal/protocol client/Assets/Scripts/Protocol proto/testdata/golden \
    client/Assets.meta client/Assets/Scripts.meta client/Assets/Scripts/Protocol.meta 2>/dev/null \
    | sort -u \
    | while IFS= read -r f; do
        if [ -f "$f" ]; then
          printf '%s %s\n' "$f" "$(git hash-object "$f" 2>/dev/null || printf HASH_ERROR)"
        else
          printf '%s MISSING\n' "$f"
        fi
      done
}

# Bootstrap mode (ADR-0057): before IMP-000 creates scripts/verify.ps1, missing
# canonical tooling is SKIP(bootstrap), never FAIL.
BOOTSTRAP=0
[ -f scripts/verify.ps1 ] || BOOTSTRAP=1

files="$(changed_files)"
scope_out="$(bash .devin/scripts/diff_scope.sh)"
scopes="$(printf '%s\n' "$scope_out" | sed -n 's/^scope: //p')"
risk="$(printf '%s\n' "$scope_out" | sed -n 's/^risk: //p')"
spec_dirs="$(printf '%s\n' "$scope_out" | sed -n 's/^spec_dirs: //p')"
in_scope() { printf '%s' " $scopes " | grep -q " $1 "; }

hdr "Scope"
printf 'files: %s changed | scope: %s | risk: %s\n' \
  "$(printf '%s\n' "$files" | grep -c . || true)" "$scopes" "$risk"

# --- 0. Canonical verifier ----------------------------------------------------
canonical_ran=0
canonical_functional_pass=0
if [ "$FULL" -eq 1 ]; then
  hdr "Canonical verifier (Q0-Q6)"
  if [ "$BOOTSTRAP" -eq 1 ]; then
    skip "SKIP(bootstrap): scripts/verify.ps1 does not exist yet (created by IMP-000)"
  elif ! ps_cmd >/dev/null; then
    warn "pwsh (PowerShell 7) not installed locally; canonical Q0-Q6 deferred to CI (authoritative, ADR-0058)"
  else
    canonical_ran=1
    before_status="$(git status --porcelain=v1 --untracked-files=all 2>/dev/null)"
    rm -f verify-report.json
    # -LocalDeferMissing: a missing local Unity editor, PostgreSQL or Windows-only
    # binary is DEFERRED(local-missing) in verify-report.json, never in CI (ADR-0072).
    canonical_out="$(run_ps_script scripts/verify.ps1 -LocalDeferMissing 2>&1)"
    canonical_rc=$?
    after_status="$(git status --porcelain=v1 --untracked-files=all 2>/dev/null)"
    [ "$QUIET" -eq 0 ] && printf '%s\n' "$canonical_out"

    if [ "$canonical_rc" -eq 0 ]; then
      canonical_functional_pass=1
      pass "canonical Q0-Q6"
      if has_cmd jq && [ -f verify-report.json ]; then
        deferred_ids="$(jq -r '[.gates[].checks[] | select(.status == "DEFERRED") | .id] | sort | unique | join(" ")' verify-report.json 2>/dev/null || true)"
        [ -n "$deferred_ids" ] && warn "DEFERRED(local-missing) to CI (authoritative): $deferred_ids"
      fi
    else
      canonical_fail_ids=""
      if has_cmd jq && [ -f verify-report.json ]; then
        canonical_fail_ids="$(jq -r '[.gates[].checks[] | select(.status == "FAIL") | .id] | sort | unique | join(" ")' verify-report.json 2>/dev/null || true)"
      fi
      if [ "$canonical_fail_ids" = "Q6.clean_tree" ] && [ "$before_status" = "$after_status" ]; then
        canonical_functional_pass=1
        warn "canonical Q0-Q5 and Q6 evidence passed; Q6.clean_tree is deferred for the reviewed local diff (verification created no new drift)"
      else
        [ "$before_status" != "$after_status" ] && fail "canonical verification created new worktree drift"
        fail "canonical verifier failed (${canonical_fail_ids:-report unavailable})"
      fi
    fi
  fi
else
  skip "canonical Q0-Q6 not run in scoped mode; use --full at checkpoint"
fi

# --- 1. Spec/task integrity ----------------------------------------------------
hdr "Spec integrity"
protected_docs="$(printf '%s\n' "$files" | grep -E '^docs/|^\.devin/|^AGENTS\.md$|^README\.md$' | grep -v '^docs/10_implementation/' || true)"
if [ -n "$protected_docs" ]; then
  if [ "${THINHTHAN_AGENT_ROLE:-}" = "spec-owner" ]; then
    warn "spec-change diff touches protected docs (spec-owner): $protected_docs — PR needs policy-review"
  else
    fail "implementer diff touches protected docs (only spec-owner may): $protected_docs"
  fi
else
  pass "no protected docs modified"
fi

if [ "$canonical_functional_pass" -eq 1 ]; then
  pass "task/DAG/evidence integrity covered by canonical Q0/Q6"
elif in_scope DOCS || in_scope EVIDENCE; then
  if has_cmd go && [ -f server/go.mod ]; then
    (cd server && run_limited 180 go test ./internal/conformance/...) \
      && pass "canonical conformance tests" || fail "canonical conformance tests"
  elif [ "$BOOTSTRAP" -eq 1 ] || [ ! -f server/go.mod ]; then
    skip "SKIP(bootstrap): conformance tests do not exist yet (IMP-000)"
  else
    fail "Go/conformance unavailable for changed implementation docs/evidence"
  fi
else
  skip "no task/evidence surface changed"
fi

if [ "${spec_dirs:-0}" -gt 2 ]; then
  warn "diff touches $spec_dirs numbered spec directories — change packet must list every consumer and the PR needs policy-review"
fi

# --- 2. Generated-code integrity ---------------------------------------------
hdr "Generated code"
if in_scope GENERATED && ! in_scope PROTO; then
  fail "generated files changed without proto source change — suspected hand-edit"
elif in_scope GENERATED; then
  pass "generated changes accompany proto source changes"
else
  pass "no generated files touched"
fi

# --- 3. Migration integrity ---------------------------------------------------
hdr "Migrations"
mig_mutated="$(printf '%s\n' "$files" | grep -E 'server/migrations/[0-9]{6}_.*\.sql$' \
  | while IFS= read -r m; do
      if is_committed "$m" && ! git diff --quiet HEAD -- "$m" 2>/dev/null; then printf '%s\n' "$m"; fi
    done)"
mig_deleted="$(git diff --name-only --diff-filter=D HEAD 2>/dev/null | grep -E 'server/migrations/' || true)"
if [ -n "$mig_mutated$mig_deleted" ]; then
  fail "committed migration file(s) modified/deleted: $mig_mutated $mig_deleted"
else
  pass "no committed migration mutated"
fi

if in_scope MIGRATIONS; then
  invalid_mig="$(printf '%s\n' "$files" | grep -E 'server/migrations/' \
    | grep -vE '/[0-9]{6}_[a-z0-9_]+\.(up|down)\.sql$' || true)"
  [ -n "$invalid_mig" ] && fail "invalid migration filename(s): $invalid_mig" || pass "migration filenames valid"

  missing_pair="$(printf '%s\n' "$files" | grep -E 'server/migrations/[0-9]{6}_[a-z0-9_]+\.(up|down)\.sql$' \
    | sed -E 's/\.(up|down)\.sql$//' | sort -u \
    | while IFS= read -r base; do
        [ -f "$base.up.sql" ] && [ -f "$base.down.sql" ] || printf '%s\n' "$base"
      done)"
  [ -n "$missing_pair" ] && fail "migration pair incomplete: $missing_pair" || pass "migration up/down pairs complete"

  duplicate_numbers="$(git ls-files --cached --others --exclude-standard -- server/migrations 2>/dev/null \
    | grep -E '/[0-9]{6}_[a-z0-9_]+\.(up|down)\.sql$' \
    | sed -E 's/\.(up|down)\.sql$//' | sort -u \
    | sed -E 's|.*/([0-9]{6})_.*|\1|' | sort | uniq -d || true)"
  [ -n "$duplicate_numbers" ] && fail "migration numbers reused by multiple names: $duplicate_numbers" \
    || pass "migration numbers are unique"

  if [ "$FULL" -eq 1 ] && [ "$canonical_ran" -eq 0 ]; then
    fail "migration changed but canonical Q5 apply/down/apply verifier is unavailable"
  fi
fi

# --- 4. Version pins ----------------------------------------------------------
hdr "Dependency pins"
if [ "$canonical_functional_pass" -eq 1 ]; then
  pass "version/dependency/CI pins covered by canonical Q1"
else
if in_scope PINS || in_scope CI || in_scope DEPLOY \
   || printf '%s\n' "$files" | grep -qE '^scripts/verify\.ps1$|^client/Assets/Plugins/Google\.Protobuf/'; then
  if has_cmd go && [ -f server/go.mod ]; then
    (cd server && run_limited 180 go test ./internal/stackpin) \
      && pass "canonical stackpin tests" || fail "canonical stackpin tests"
  else
    fail "Go/stackpin unavailable for changed pin surface"
  fi
else
  skip "no dependency/toolchain pin surface changed"
fi

fi

# --- 5. Go backend ------------------------------------------------------------
if in_scope GO || { [ "$FULL" -eq 1 ] && [ "$canonical_functional_pass" -eq 0 ] && [ -f server/go.mod ]; }; then
  hdr "Go backend"
  stray_go="$(printf '%s\n' "$files" | grep '\.go$' | grep -v '^server/' || true)"
  [ -n "$stray_go" ] && fail "Go files outside canonical server module: $stray_go"

  if [ ! -f server/go.mod ]; then
    fail "*.go files changed but server/go.mod does not exist"
  elif ! has_cmd go; then
    fail "Go toolchain unavailable; backend verification cannot pass"
  else
    installed_go="$(go version 2>/dev/null | grep -oE 'go[0-9.]+' | head -1)"
    [ "$installed_go" = "go1.27.1" ] && pass "Go toolchain is 1.27.1" \
      || fail "Go toolchain pin mismatch: ${installed_go:-unknown} (expected go1.27.1)"

    gofiles="$(printf '%s\n' "$files" | grep '^server/.*\.go$' || true)"
    unformatted="$(printf '%s\n' "$gofiles" | while IFS= read -r f; do [ -f "$f" ] && gofmt -l "$f"; done)"
    [ -n "$unformatted" ] && fail "gofmt: $unformatted" || pass "gofmt clean"

    if [ "$canonical_functional_pass" -eq 1 ]; then
      pass "architecture/import fences covered by canonical Q4"
    else
      (cd server && run_limited 180 go test ./internal/conformance/...) \
        && pass "canonical conformance tests" || fail "canonical conformance tests"
    fi

    pkgs="$(printf '%s\n' "$gofiles" | xargs -r -n1 dirname | sort -u | sed 's|^server/|./|; s|^server$|./|' || true)"
    race_pkgs="$(printf '%s\n' "$pkgs" | grep -E '(^|/)(sim|global|edge|durable)(/|$)' || true)"
    # -race needs cgo + a C compiler; CI runs it on the Linux job only (ADR-0072).
    race_ok=0
    if [ "$(go env CGO_ENABLED 2>/dev/null)" = "1" ] && has_cmd "$(go env CC 2>/dev/null | awk '{print $1}')"; then race_ok=1; fi
    if [ -n "$race_pkgs" ] && [ "$race_ok" -eq 0 ]; then
      warn "go test -race deferred to the Linux CI job (no cgo C compiler locally): $race_pkgs"
      race_pkgs=""
    fi
    if [ "$canonical_functional_pass" -eq 1 ]; then
      pass "Go vet/tests/build covered by canonical Q3"
      if [ -n "$race_pkgs" ]; then
        if (cd server && for d in $race_pkgs; do run_limited 180 go test -count=1 -race "$d" || exit 1; done); then
          pass "go test -race ($race_pkgs)"
        else
          fail "go test -race ($race_pkgs)"
        fi
      fi
    else
      if [ -n "$pkgs" ]; then
        if (cd server && for d in $pkgs; do go vet "$d" || exit 1; done); then
          pass "go vet ($pkgs)"
        else
          fail "go vet ($pkgs)"
        fi

        if (cd server && for d in $pkgs; do
          extra=()
          [ "$race_ok" -eq 1 ] && case "$d" in *sim*|*global*|*edge*|*durable*) extra=(-race) ;; esac
          run_limited 180 go test -count=1 "${extra[@]}" "$d" || exit 1
        done); then
          pass "go test affected packages ($pkgs)"
        else
          fail "go test affected packages ($pkgs)"
        fi
      fi

      if [ "$FULL" -eq 1 ]; then
        (cd server && go vet ./...) && pass "go vet ./..." || fail "go vet ./..."
        if has_cmd staticcheck; then
          (cd server && staticcheck ./...) && pass "staticcheck ./..." || fail "staticcheck ./..."
        else
          warn "staticcheck not installed locally (CI Q4 is authoritative; go install honnef.co/go/tools/cmd/staticcheck@v0.8.1)"
        fi
        (cd server && run_limited 300 go test -count=1 ./...) && pass "go test ./..." || fail "go test ./..."
        (cd server && go build ./...) && pass "go build ./..." || fail "go build ./..."
      fi
    fi
  fi
else
  skip "no Go changes"
fi

# --- 6. Wire contract ---------------------------------------------------------
if in_scope PROTO; then
  hdr "Protobuf wire contract"
  if [ "$canonical_functional_pass" -eq 1 ]; then
    pass "protobuf codegen drift + Go registry/golden parity covered by canonical Q2/Q3"
  elif [ ! -f scripts/codegen.ps1 ]; then
    fail "proto changed but scripts/codegen.ps1 is missing (IMP-061 not done yet or regressed)"
  elif ! ps_cmd >/dev/null; then
    warn "pwsh (PowerShell 7) not installed locally; codegen drift deferred to CI Q2 (ADR-0058)"
  else
    before="$(generated_snapshot)"
    codegen_out="$(run_ps_script scripts/codegen.ps1 2>&1)"
    codegen_rc=$?
    after="$(generated_snapshot)"
    if [ "$codegen_rc" -ne 0 ]; then
      fail "scripts/codegen.ps1 failed: $(printf '%s\n' "$codegen_out" | tail -10)"
    elif [ "$before" != "$after" ]; then
      fail "codegen changed generated content — committed outputs were stale"
    else
      pass "codegen content drift = 0"
    fi
  fi
fi

# --- 7. Unity client ----------------------------------------------------------
if in_scope CLIENT_CS || in_scope CLIENT_ASSETS; then
  hdr "Unity client"
  unity_editor="$(unity_editor_path || true)"
  if [ "$FULL" -eq 0 ]; then
    skip "Unity compile/tests are checkpoint checks; rerun with --full"
  elif [ -z "$unity_editor" ] && [ "$BOOTSTRAP" -eq 1 ]; then
    skip "SKIP(bootstrap): Unity editor not provisioned locally"
  elif [ -z "$unity_editor" ]; then
    warn "pinned Unity Editor 6000.6.1f1 not installed locally; Unity tests deferred to CI (authoritative, ADR-0058; set UNITY_EDITOR_PATH to run locally)"
  elif [ ! -d client ]; then
    fail "client changes detected but client/ project is absent"
  else
    unity_before_status="$(git status --porcelain=v1 --untracked-files=all 2>/dev/null)"
    tmpdir="$(mktemp -d 2>/dev/null || printf '')"
    if [ -z "$tmpdir" ]; then
      fail "cannot create temporary directory for Unity test artifacts"
    else
      edit_log="$tmpdir/editmode.log"
      edit_results="$tmpdir/editmode.xml"
      if run_limited 900 "$unity_editor" -batchmode -projectPath client -runTests -testPlatform EditMode \
        -testResults "$edit_results" -quit -logFile "$edit_log" >/dev/null 2>&1; then
        pass "Unity compile + EditMode tests"
      else
        fail "Unity EditMode tests failed: $(tail -20 "$edit_log" 2>/dev/null)"
      fi

      if printf '%s\n' "$files" | grep -Eq '^client/.*\.(unity|prefab)$|^client/Assets/Tests/PlayMode/.*\.cs$'; then
        play_log="$tmpdir/playmode.log"
        play_results="$tmpdir/playmode.xml"
        if run_limited 1200 "$unity_editor" -batchmode -projectPath client -runTests -testPlatform PlayMode \
          -testResults "$play_results" -quit -logFile "$play_log" >/dev/null 2>&1; then
          pass "Unity PlayMode tests"
        else
          fail "Unity PlayMode tests failed: $(tail -20 "$play_log" 2>/dev/null)"
        fi
      else
        skip "PlayMode not required by current client diff"
      fi
      if ! rm -rf "$tmpdir" 2>/dev/null; then
        sleep 1
        rm -rf "$tmpdir" 2>/dev/null || warn "temporary Unity test artifacts remain: $tmpdir"
      fi
      unity_after_status="$(git status --porcelain=v1 --untracked-files=all 2>/dev/null)"
      if [ "$unity_before_status" != "$unity_after_status" ]; then
        fail "Unity editor materialized files under client/ — review and commit them (same rule as CI artifact unity-materialized-<os>, ADR-0072): $(printf '%s\n' "$unity_after_status" | tail -20)"
      else
        pass "Unity verification created no worktree drift"
      fi
    fi
  fi
fi

# --- 8. Devin governance ------------------------------------------------------
if in_scope DEVIN; then
  hdr "Devin governance"
  if ! has_cmd jq; then
    fail "jq unavailable; cannot validate Devin JSON/hook contracts"
  else
    jq empty .devin/config.json .devin/hooks.v1.json >/dev/null 2>&1 \
      && pass "Devin JSON parses" || fail "invalid .devin/config.json or hooks.v1.json"

    missing_hook_scripts="$(jq -r '.. | objects | select(.type? == "command") | .command' .devin/hooks.v1.json 2>/dev/null \
      | awk '$1=="bash"{print $2}' \
      | while IFS= read -r script; do [ -f "$script" ] || printf '%s\n' "$script"; done)"
    [ -n "$missing_hook_scripts" ] && fail "hook command references missing script(s): $missing_hook_scripts" \
      || pass "hook command paths exist"

    if jq -e '
      (.PreToolUse | type == "array" and length >= 2) and
      (.PostToolUse | type == "array" and length >= 1) and
      (.SessionStart | type == "array" and length >= 1) and
      (.Stop | type == "array" and length >= 1) and
      ([.Stop[].hooks[].timeout] | max >= 2100)
    ' .devin/hooks.v1.json >/dev/null 2>&1; then
      pass "hook lifecycle schema and Stop timeout are coherent"
    else
      fail "hook lifecycle schema incomplete or Stop timeout shorter than Unity full gate"
    fi
  fi

  script_syntax_errors="$(for script in .devin/scripts/*.sh; do bash -n "$script" 2>&1 || printf '%s: syntax failed\n' "$script"; done)"
  [ -n "$script_syntax_errors" ] && fail "hook script syntax: $script_syntax_errors" \
    || pass "all governance scripts are Bash-syntax clean"

  write_block_rc=0
  printf '%s' '{"tool_input":{"file_path":"docs/05_network/messages.md"}}' \
    | bash .devin/scripts/pre_write_guard.sh >/dev/null 2>&1 || write_block_rc=$?
  write_allow_rc=0
  printf '%s' '{"tool_input":{"file_path":"docs/10_implementation/task_queue.md"}}' \
    | bash .devin/scripts/pre_write_guard.sh >/dev/null 2>&1 || write_allow_rc=$?
  generated_block_rc=0
  printf '%s' '{"tool_input":{"file_path":"client/Assets/Scripts/Protocol/Combat.cs.meta"}}' \
    | bash .devin/scripts/pre_write_guard.sh >/dev/null 2>&1 || generated_block_rc=$?
  tools_block_rc=0
  printf '%s' '{"tool_input":{"file_path":"tools/protobuf/protoc/bin/protoc.exe"}}' \
    | bash .devin/scripts/pre_write_guard.sh >/dev/null 2>&1 || tools_block_rc=$?
  exec_block_rc=0
  printf '%s' '{"tool_input":{"command":"git push --force"}}' \
    | bash .devin/scripts/pre_exec_guard.sh >/dev/null 2>&1 || exec_block_rc=$?
  if [ "$write_block_rc" -eq 2 ] && [ "$write_allow_rc" -eq 0 ] \
     && [ "$generated_block_rc" -eq 2 ] && [ "$tools_block_rc" -eq 2 ] && [ "$exec_block_rc" -eq 2 ]; then
    pass "core hook allow/block smoke tests"
  else
    fail "hook smoke mismatch: docs=$write_block_rc docs10=$write_allow_rc generated=$generated_block_rc tools=$tools_block_rc force_push=$exec_block_rc"
  fi

  if ! has_cmd devin; then
    fail "Devin CLI unavailable; cannot validate rules/skills/agents"
  else
    doctor_out="$(devin doctor --json 2>&1)"
    if printf '%s' "$doctor_out" | jq -e '.ok == true and ([.checks[]? | select(.check == "custom subagent profiles") | .detail | contains("9 profile(s) loaded")] | any)' >/dev/null 2>&1; then
      pass "Devin doctor (9 custom agents loaded)"
    else
      fail "devin doctor: $doctor_out"
    fi

    rules_out="$(devin rules list 2>&1)"
    if printf '%s' "$rules_out" | grep -q '^Errors:'; then
      fail "Devin rule loading errors: $(printf '%s\n' "$rules_out" | sed -n '/^Errors:/,$p')"
    else
      missing_rules=""
      for rule in 00-engineering-baseline 01-verification-risk 10-go-backend 11-unity-client 12-proto-contract 13-spec-docs 14-sql-migrations 15-art-assets; do
        printf '%s\n' "$rules_out" | grep -q "  $rule \[Devin\]" || missing_rules="$missing_rules $rule"
      done
      [ -n "$missing_rules" ] && fail "project rules not loaded:$missing_rules" || pass "all 8 project rules load"
    fi

    skills_out="$(devin skills list 2>&1)"
    missing_skills=""
    for skill in run-wave run-imp-task repo-architecture implement-backend-feature implement-unity-feature client-server-feature fix-backend-bug fix-unity-bug network-debugging database-change code-review produce-art-asset; do
      printf '%s\n' "$skills_out" | grep -q "^  /$skill " || missing_skills="$missing_skills $skill"
    done
    [ -n "$missing_skills" ] && fail "project skills not loaded:$missing_skills" || pass "all 12 project skills load"
  fi
fi

# --- 9. Hygiene ---------------------------------------------------------------
hdr "Hygiene"
debug_artifacts="$(printf '%s\n' "$files" | grep -Ei '(^|/)(\.ds_store|thumbs\.db|desktop\.ini|[^/]+\.(log|tmp|bak|swp)|coverage\.out|testresults?)(/|$)|^client/(library|temp|logs|obj|builds?)/' || true)"
[ -n "$debug_artifacts" ] && fail "debug/build artifacts in diff: $debug_artifacts" || pass "no debug/build artifacts"

hdr "Result"
if [ "$FAILS" -eq 0 ]; then
  printf 'VERIFY_DELTA: PASS (%s warnings)\n' "$WARNS"
  exit 0
else
  printf 'VERIFY_DELTA: FAIL (%s failures, %s warnings)\n' "$FAILS" "$WARNS"
  exit 1
fi
