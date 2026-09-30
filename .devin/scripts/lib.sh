#!/usr/bin/env bash
# Shared helpers for thinhthan Devin hooks and verification scripts.
# Usage: . "$(dirname "$0")/lib.sh"   (hook scripts read stdin BEFORE sourcing is fine too)
set -u

# --- Project root -------------------------------------------------------------
# Hooks receive DEVIN_PROJECT_DIR; fall back to git root, then cwd.
if [ -n "${DEVIN_PROJECT_DIR:-}" ] && [ -d "${DEVIN_PROJECT_DIR:-}" ]; then
  ROOT="$DEVIN_PROJECT_DIR"
else
  ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi
ROOT="${ROOT//\\//}"
ROOT="${ROOT%/}"
cd "$ROOT" 2>/dev/null || true

has_cmd() { command -v "$1" >/dev/null 2>&1; }

# Canonical wrappers are PowerShell 7 (`pwsh`) on Linux and Windows (ADR-0058).
ps_cmd() { if has_cmd pwsh; then printf 'pwsh'; elif has_cmd pwsh.exe; then printf 'pwsh.exe'; else return 1; fi; }
run_ps_script() { local script="$1" ps; shift; ps="$(ps_cmd)" || return 127; "$ps" -NoProfile -File "$script" "$@"; }

# Resolve the exact pinned Unity Editor; never confuse other `unity` CLIs with
# the editor binary. UNITY_EDITOR_PATH may override discovery only when its path
# still names the canonical editor version.
unity_editor_path() {
  local candidate normalized
  for candidate in \
    "${UNITY_EDITOR_PATH:-}" \
    "C:/Program Files/Unity/Hub/Editor/6000.6.1f1/Editor/Unity.exe" \
    "${HOME:-/nonexistent}/Unity/Hub/Editor/6000.6.1f1/Editor/Unity"; do
    [ -n "$candidate" ] || continue
    normalized="${candidate//\\//}"
    case "$normalized" in *6000.6.1f1*) ;; *) continue ;; esac
    [ -x "$candidate" ] || [ -f "$candidate" ] || continue
    printf '%s' "$candidate"
    return 0
  done
  return 1
}

# --- JSON stdin ---------------------------------------------------------------
# Call read_stdin once, then json_get / json_bool against the captured payload.
HOOK_INPUT=""
read_stdin() { HOOK_INPUT="$(cat 2>/dev/null || true)"; }

# json_get '.tool_input.command' -> string (empty when absent).
json_get() {
  local path="$1"
  [ -z "$HOOK_INPUT" ] && { printf ''; return 0; }
  if has_cmd jq; then
    printf '%s' "$HOOK_INPUT" | jq -r "$path // empty" 2>/dev/null || true
    return 0
  fi
  # Fallback: flat key extraction, sufficient for guardrails (fail-open).
  local key="${path##*.}"
  printf '%s' "$HOOK_INPUT" \
    | grep -o "\"$key\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" \
    | head -n1 \
    | sed 's/^[^"]*"[^"]*"[[:space:]]*:[[:space:]]*"\(.*\)"$/\1/' || true
}

# json_bool '.stop_hook_active' -> "true"/"false".
json_bool() {
  local path="$1"
  if has_cmd jq; then
    printf '%s' "$HOOK_INPUT" | jq -r "($path // false) | tostring" 2>/dev/null || printf 'false'
    return 0
  fi
  local key="${path##*.}"
  if printf '%s' "$HOOK_INPUT" | grep -q "\"$key\"[[:space:]]*:[[:space:]]*true"; then
    printf 'true'
  else
    printf 'false'
  fi
}

# --- Hook outputs -------------------------------------------------------------
# block <reason> — deny the tool call / the stop. JSON decision + exit 2 covers
# both control channels documented for command hooks.
block() {
  local reason="$1"
  if has_cmd jq; then
    jq -n --arg r "$reason" '{decision:"block", reason:$r}'
  else
    printf '{"decision":"block","reason":"%s"}\n' \
      "$(printf '%s' "$reason" | sed 's/\\/\\\\/g; s/"/\\"/g')"
  fi
  exit 2
}

# note <event> <text> — inject additionalContext, always exit 0.
note() {
  local event="$1" text="$2"
  if has_cmd jq; then
    jq -n --arg e "$event" --arg t "$text" \
      '{hookSpecificOutput:{hookEventName:$e, additionalContext:$t}}'
  else
    printf '{"hookSpecificOutput":{"hookEventName":"%s","additionalContext":"%s"}}\n' \
      "$event" "$(printf '%s' "$text" | sed 's/\\/\\\\/g; s/"/\\"/g')"
  fi
  exit 0
}

# --- Repo helpers -------------------------------------------------------------
# Repo-relative lexical normalization (Windows separators/root + . and ..).
norm_path() {
  local p="$1" root="$ROOT" part out=""
  local -a parts stack
  p="${p//\\//}"

  if has_cmd cygpath; then
    p="$(cygpath -am "$p" 2>/dev/null || printf '%s' "$p")"
    root="$(cygpath -am "$root" 2>/dev/null || printf '%s' "$root")"
  fi
  root="${root%/}"

  if [ "${p,,}" = "${root,,}" ]; then
    p=""
  elif [[ "${p,,}" == "${root,,}/"* ]]; then
    p="${p:${#root}+1}"
  fi

  IFS='/' read -r -a parts <<< "$p"
  for part in "${parts[@]}"; do
    case "$part" in
      ""|.) ;;
      ..)
        if [ "${#stack[@]}" -gt 0 ] && [ "${stack[${#stack[@]}-1]}" != ".." ]; then
          unset 'stack[${#stack[@]}-1]'
        else
          stack+=("..")
        fi
        ;;
      *) stack+=("$part") ;;
    esac
  done

  for part in "${stack[@]}"; do
    [ -n "$out" ] && out="$out/"
    out="$out$part"
  done
  printf '%s' "$out"
}

# changed_files -> repo-relative files differing from HEAD (incl. untracked).
changed_files() {
  {
    git diff --name-only HEAD 2>/dev/null
    git ls-files --others --exclude-standard 2>/dev/null
  } | sort -u | grep -v '^$' || true
}

# files_match <regex> — true if any changed file matches.
files_match() { changed_files | grep -Eq "$1"; }

# is_committed <path> — path exists in HEAD (newly staged files return false).
is_committed() { git cat-file -e "HEAD:$1" >/dev/null 2>&1; }
