#!/usr/bin/env bash
# Classify the current worktree diff into scope flags + risk level.
# Output (stdout):
#   scope:  space-separated area flags
#   risk:   LOW | MEDIUM | HIGH
#   counts: per-area file counts
# Used by stop_verify_gate.sh and verify_delta.sh; safe to run manually.
cd "$(dirname "$0")/../.." 2>/dev/null || true
. "$(dirname "$0")/lib.sh"

files="$(changed_files)"

n_docs=$(printf '%s\n' "$files" | grep -c '^docs/' || true)
n_go=$(printf '%s\n' "$files" | grep -c '\.go$' || true)
n_proto=$(printf '%s\n' "$files" | grep -c '^proto/.*\.proto$' || true)
n_cs=$(printf '%s\n' "$files" | grep -c '^client/.*\.cs$' || true)
n_assets=$(printf '%s\n' "$files" | grep -cE '^client/.*\.(unity|prefab|asset|meta|mat|controller|anim|asmdef|png|psd|psb|wav|ogg|ttf|otf)$' || true)
n_mig=$(printf '%s\n' "$files" | grep -cE 'server/migrations/' || true)
n_gen=$(printf '%s\n' "$files" | grep -cE '(^server/internal/protocol/|^client/Assets/Scripts/Protocol/|^client/Assets(\/Scripts(\/Protocol)?)?\.meta$|^proto/testdata/golden/)' || true)
n_scripts=$(printf '%s\n' "$files" | grep -c '^scripts/' || true)
n_ci=$(printf '%s\n' "$files" | grep -c '^\.github/' || true)
n_deploy=$(printf '%s\n' "$files" | grep -c '^deploy/' || true)
n_evid=$(printf '%s\n' "$files" | grep -c '^docs/10_implementation/evidence/' || true)
n_devin=$(printf '%s\n' "$files" | grep -c '^\.devin/' || true)
n_pins=$(printf '%s\n' "$files" | grep -cE '(^|/)go\.mod$|(^|/)go\.sum$|^server/internal/stackpin/|^client/Packages/(manifest|packages-lock)\.json$|^client/ProjectSettings/ProjectVersion\.txt$|^client/Assets/Plugins/Google\.Protobuf/|^docs/00_context/technology_versions\.md$' || true)

scopes=""
[ "$n_docs" -gt 0 ] && scopes="$scopes DOCS"
[ "$n_go" -gt 0 ] && scopes="$scopes GO"
[ "$n_proto" -gt 0 ] && scopes="$scopes PROTO"
[ "$n_cs" -gt 0 ] && scopes="$scopes CLIENT_CS"
[ "$n_assets" -gt 0 ] && scopes="$scopes CLIENT_ASSETS"
[ "$n_mig" -gt 0 ] && scopes="$scopes MIGRATIONS"
[ "$n_gen" -gt 0 ] && scopes="$scopes GENERATED"
[ "$n_scripts" -gt 0 ] && scopes="$scopes SCRIPTS"
[ "$n_ci" -gt 0 ] && scopes="$scopes CI"
[ "$n_deploy" -gt 0 ] && scopes="$scopes DEPLOY"
[ "$n_evid" -gt 0 ] && scopes="$scopes EVIDENCE"
[ "$n_devin" -gt 0 ] && scopes="$scopes DEVIN"
[ "$n_pins" -gt 0 ] && scopes="$scopes PINS"
scopes="${scopes# }"
[ -z "$scopes" ] && scopes="CLEAN"

# Numbered spec dirs touched (cross-directory rule: >2 requires full consumer list + policy-review).
n_spec_dirs=$(printf '%s\n' "$files" | grep -oE '^docs/[0-9]{2}_[a-z_]+/' | sort -u | wc -l | tr -d ' ')

# Sensitive code areas.
n_sensitive=$(printf '%s\n' "$files" \
  | grep -cE 'server/internal/(durable|edge|sim|global|protocol|config)/|^client/Assets/Scripts/Net/|AGENTS\.md$|\.devin/(hooks\.v1\.json|config\.json)' || true)

risk="LOW"
why="isolated non-contract change"
if [ "$n_proto" -gt 0 ] || [ "$n_mig" -gt 0 ] || [ "$n_gen" -gt 0 ] || [ "$n_pins" -gt 0 ] \
   || [ "$n_assets" -gt 0 ] || [ "$n_ci" -gt 0 ] || [ "$n_deploy" -gt 0 ] || [ "$n_evid" -gt 0 ] || [ "$n_spec_dirs" -gt 2 ] \
   || [ "$n_sensitive" -gt 0 ] \
   || printf '%s\n' "$files" | grep -q '^docs/11_decisions/'; then
  risk="HIGH"
  why=""
  [ "$n_proto" -gt 0 ] && why="$why protocol"
  [ "$n_mig" -gt 0 ] && why="$why persistence"
  [ "$n_gen" -gt 0 ] && why="$why generated"
  [ "$n_pins" -gt 0 ] && why="$why pins"
  [ "$n_assets" -gt 0 ] && why="$why Unity-serialization"
  [ "$n_ci" -gt 0 ] && why="$why CI"
  [ "$n_deploy" -gt 0 ] && why="$why deployment"
  [ "$n_evid" -gt 0 ] && why="$why evidence"
  [ "$n_spec_dirs" -gt 2 ] && why="$why cross-spec"
  [ "$n_sensitive" -gt 0 ] && why="$why sensitive-architecture/governance"
  printf '%s\n' "$files" | grep -q '^docs/11_decisions/' && why="$why ADR"
  why="${why# }"
elif [ "$n_go" -gt 0 ] || [ "$n_cs" -gt 0 ] || [ "$n_assets" -gt 0 ] || [ "$n_scripts" -gt 0 ] \
     || [ "$n_spec_dirs" -gt 0 ]; then
  risk="MEDIUM"
  why="runtime code / serialized assets / implementation-spec edits"
fi

printf 'scope: %s\n' "$scopes"
printf 'risk: %s\n' "$risk"
printf 'why: %s\n' "$why"
printf 'spec_dirs: %s\n' "$n_spec_dirs"
printf 'counts: docs=%s go=%s proto=%s cs=%s assets=%s mig=%s gen=%s scripts=%s ci=%s deploy=%s evid=%s devin=%s pins=%s\n' \
  "$n_docs" "$n_go" "$n_proto" "$n_cs" "$n_assets" "$n_mig" "$n_gen" "$n_scripts" "$n_ci" "$n_deploy" "$n_evid" "$n_devin" "$n_pins"
