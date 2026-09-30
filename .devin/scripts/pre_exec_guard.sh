#!/usr/bin/env bash
# PreToolUse guard for exec / write_to_process.
# Blocks commands that are never acceptable for an agent in this repo.
# Destructive or history-rewriting operations are always blocked; agents use the
# safe alternative named in each message (ADR-0050, ADR-0057).
cd "$(dirname "$0")/../.." 2>/dev/null || true
. "$(dirname "$0")/lib.sh"
read_stdin

cmd="$(json_get .tool_input.command)"
[ -z "$cmd" ] && cmd="$(json_get .tool_input.text_input)"
[ -z "$cmd" ] && exit 0
workdir="$(json_get .tool_input.workdir)"

lc="${cmd,,}"
lc="${lc//\\//}"
short="${cmd:0:160}"

deny() { block "BLOCKED (safety hook): $1 — command: ${short}"; }
match() { [[ "$lc" =~ $1 ]]; }

is_protected_doc_path() {
  local p
  [ "${THINHTHAN_AGENT_ROLE:-}" = "spec-owner" ] && return 1
  p="$(norm_path "$1")"
  p="${p,,}"
  case "$p" in
    docs/10_implementation|docs/10_implementation/*) return 1 ;;
    docs|docs/*) return 0 ;;
    *) return 1 ;;
  esac
}

clean_path_token() {
  local ref="$1"
  case "$ref" in *=*) ref="${ref#*=}" ;; esac
  ref="${ref#>}"; ref="${ref#>}"
  ref="${ref#\"}"; ref="${ref#\'}"; ref="${ref#(}"
  ref="${ref%\"}"; ref="${ref%\'}"; ref="${ref%)}"; ref="${ref%,}"
  ref="${ref%;}"; ref="${ref%&}"; ref="${ref%|}"
  printf '%s' "$ref"
}

has_protected_doc_ref() {
  local text="$1" ref
  local -a refs
  [[ "$text" == *docs* ]] || return 1
  read -r -a refs <<< "$text"
  for ref in "${refs[@]}"; do
    [[ "$ref" == *docs* ]] || continue
    ref="$(clean_path_token "$ref")"
    [ -n "$ref" ] && is_protected_doc_path "$ref" && return 0
  done
  return 1
}

is_mutating_command() {
  local re='(^|[[:space:]])(rm|rmdir|touch|mkdir|truncate|chmod|chown|mv|rename|set-content|add-content|clear-content|out-file|new-item|remove-item|move-item|rename-item)([[:space:]]|$)|git[[:space:]]+(rm|mv)([[:space:]]|$)|sed[[:space:]][^|;&]*-[a-z]*i|perl[[:space:]][^|;&]*-[a-z]*i|writefile|writeall(text|bytes)|open\([^)]*,[^)]*[wa]'
  [[ "$1" =~ $re ]]
}

# --- git history / state destruction -----------------------------------------
match 'git[[:space:]]+config([[:space:]]|$)' \
  && deny "git config mutation is forbidden by repo policy"
match 'git[[:space:]]+push[^|;&]*(--force|-f([[:space:]]|$)|--force-with-lease|--delete)' \
  && deny "force push or remote deletion is forbidden from an agent session"
match 'git[[:space:]]+commit[^|;&]*--amend' \
  && deny "commit --amend rewrites history — create a new commit instead"
match 'git[[:space:]]+commit[^|;&]*--no-verify' \
  && deny "--no-verify bypasses commit hooks — forbidden"
match 'git[[:space:]]+reset[^|;&]*--hard' \
  && deny "git reset --hard destroys uncommitted work — commit or stash instead"
match 'git[[:space:]]+clean[^|;&]*(--force|-[a-z]*f[a-z]*)' \
  && deny "git clean --force irreversibly deletes untracked files — delete specific files you created instead"
match 'git[[:space:]]+checkout[[:space:]]+(--|\.)' \
  && deny "git checkout --/. discards uncommitted changes"
match 'git[[:space:]]+checkout[^|;&]*[[:space:]]--[[:space:]]' \
  && deny "git checkout -- <path> discards uncommitted changes"
match 'git[[:space:]]+checkout[^|;&]*(--force|-[a-z]*f[a-z]*)' \
  && deny "git checkout --force can discard uncommitted changes"
match 'git[[:space:]]+(rebase|filter-branch|filter-repo)([[:space:]]|$)' \
  && deny "git history rewriting is forbidden — update task branches with git merge origin/main (ADR-0050)"
if match 'git[[:space:]]+restore'; then
  match 'git[[:space:]]+restore[^|;&]*--staged' \
    || deny "git restore without --staged discards uncommitted changes"
fi
match 'git[[:space:]]+stash[[:space:]]+(drop|clear)' \
  && deny "git stash drop/clear destroys stashed work"
match 'git[[:space:]]+branch[^|;&]*-d([[:space:]]|$)' \
  && deny "git branch deletion is forbidden for agents — merged branches are deleted by GitHub after auto-merge"
match 'git[[:space:]]+update-ref[[:space:]]+-d' \
  && deny "git update-ref -d deletes refs directly"
match '(^|[;&|[:space:]])(git[[:space:]]+apply|patch)([[:space:]]|$)' \
  && deny "patch application bypasses per-file write guards — use edit/write tools"
match '(^|[;&|[:space:]])(bash|sh|zsh|fish|powershell|pwsh|cmd)([[:space:]]*)$' \
  && deny "bare interactive shells bypass stateful path guards — run an explicit command instead"

# --- filesystem destruction ---------------------------------------------------
if match 'rm[[:space:]]+([^|;&]*[[:space:]])?(--recursive|--force|-[a-z]*[rf][a-z]*)'; then
  match 'rm[[:space:]]+[^|;&]*(--recursive|--force|-[a-z]*[rf][a-z]*)[^|;&]*(\.?\.?/)*(\.git|client|server|proto|docs|scripts|deploy|\.devin|\.github)(/|[[:space:]]|$)' \
    && deny "recursive deletion inside a protected repository path"
  match '[[:space:]](/\*?|~/?|\*|\$\{?home\}?|\.{1,2}/?)([[:space:]]|$)' \
    && deny "rm -rf on root/home/wildcard/current dir"
fi
match '(remove-item[^|;&]*-recurse|del[^|;&]*/s)[^|;&]*(\.git|client|server|proto|docs|scripts|deploy|\.devin|\.github)(/|[[:space:]]|$)' \
  && deny "recursive deletion inside a protected repository path"

# --- git internals ------------------------------------------------------------
match '(^|[[:space:]])(cat|rm|mv|cp|chmod|echo|printf|>>?)[^|;&]*\.git/(hooks|objects|refs|head|index|config)' \
  && deny "do not manipulate .git internals directly"

# --- secrets ------------------------------------------------------------------
# Single exemption (ADR-0072): the reviewer role may run exactly the policy-review
# script, which alone reads the App key named by THINHTHAN_POLICY_APP_KEY_FILE.
is_policy_review_invocation() {
  [ "${THINHTHAN_AGENT_ROLE:-}" = "reviewer" ] || return 1
  local re='^[[:space:]]*(thinhthan_policy_app_(id|key_file)=[^[:space:];&|<>`$]+[[:space:]]+){0,2}(pwsh|pwsh\.exe)[[:space:]]+-noprofile[[:space:]]+-file[[:space:]]+(\./)?\.devin/scripts/policy_review\.ps1([[:space:]]+[^;&|<>`$]*)?$'
  [[ "$lc" =~ $re ]]
}
if ! is_policy_review_invocation \
   && match '(^|[^a-z0-9._-])(\.env(\.[a-z0-9_-]+)?|id_(rsa|ed25519)|[a-z0-9._-]+\.(pem|key|pfx|p12))([^a-z0-9._-]|$)'; then
  match '\.env\.(example|sample|template)' \
    || deny "do not read, write, print, or commit secret material from shell commands"
fi

# --- database destruction -----------------------------------------------------
match '(drop[[:space:]]+(database|schema|table|index|type|extension)|dropdb|truncate[[:space:]]+(table[[:space:]]+)?[a-z_]|alter[[:space:]]+table[^;]*drop[[:space:]]+(column|constraint))' \
  && deny "destructive DB statement — schema changes go through approved numbered migrations only"
if match 'delete[[:space:]]+from[[:space:]]+[a-z_.]+'; then
  match 'delete[[:space:]]+from[^;]*[[:space:]]where[[:space:]]' \
    || deny "unscoped DELETE is forbidden; use a reviewed migration or an explicit bounded predicate"
fi

# --- OS-level hazards ----------------------------------------------------------
match '(^|[;&|[:space:]])(sudo|mkfs|shutdown|reboot|halt|poweroff|dd[[:space:]]+.*of=/dev|format[[:space:]]+[a-z]:)' \
  && deny "OS-level destructive/privileged command"

# --- spec docs are read-only (only docs/10_implementation/ is writable) -------
any_redir_re='>{1,2}'
if [ -n "$workdir" ] && is_protected_doc_path "$workdir" \
   && { is_mutating_command "$lc" || [[ "$lc" =~ $any_redir_re ]]; }; then
  deny "working directory is a protected docs path; only read-only commands are allowed there"
fi

if [[ "$lc" == *cd*docs* ]]; then
  cd_ref=""
  read -r -a command_parts <<< "$lc"
  for ((i=0; i<${#command_parts[@]}-1; i++)); do
    if [ "${command_parts[$i]}" = "cd" ]; then
      cd_ref="$(clean_path_token "${command_parts[$((i+1))]}")"
      break
    fi
  done
  if [ -n "$cd_ref" ] && is_protected_doc_path "$cd_ref"; then
    deny "changing shell state into a protected docs path could bypass write guards; read files by path instead"
  fi
fi

segments="${lc//&&/$'\n'}"
segments="${segments//;/$'\n'}"
segments="${segments//|/$'\n'}"
while IFS= read -r segment; do
  [ -z "$segment" ] && continue
  has_protected_doc_ref "$segment" || continue

  is_mutating_command "$segment" \
    && deny "spec docs are read-only for agents — only docs/10_implementation/ is writable"

  redir_re='>{1,2}[[:space:]]*[^a-z0-9./]*(\.\.?/)*docs/'
  [[ "$segment" =~ $redir_re ]] && deny "redirection targets a protected docs path"
  tee_re='tee([[:space:]]+-[a-z]+)*[[:space:]]+[^a-z0-9./]*(\.\.?/)*docs/'
  [[ "$segment" =~ $tee_re ]] && deny "tee targets a protected docs path"
  archive_re='tar[^|;&]*(--extract|-[a-z]*x[a-z]*)[^|;&]*(-c|--directory)[[:space:]]+[^a-z0-9./]*(\.\.?/)*docs(/|[[:space:]]|$)|unzip[^|;&]*-d[[:space:]]+[^a-z0-9./]*(\.\.?/)*docs(/|[[:space:]]|$)|7z[^|;&]*-o[[:space:]]*[^a-z0-9./]*(\.\.?/)*docs(/|[[:space:]]|$)'
  [[ "$segment" =~ $archive_re ]] && deny "archive extraction targets a protected docs path"

  copy_re='(^|[[:space:]])(cp|install|rsync)([[:space:]]|$)'
  if [[ "$segment" =~ $copy_re ]]; then
    dest="$(printf '%s' "$segment" | awk '{print $NF}' | sed -e "s/^[\"']//" -e "s/[\"']$//")"
    [ -n "$dest" ] && is_protected_doc_path "$dest" \
      && deny "copy/install destination is a protected docs path"
  fi
done <<< "$segments"

exit 0
