# .devin — Agent Governance Layer

Devin CLI configuration for this repository. Everything here is agent-facing tooling; the product spec lives in `docs/`.

## Layout

```text
config.json           permissions (allow workspace ops; deny protected docs/secrets/generated/.git)
hooks.v1.json         lifecycle hooks -> scripts/*.sh
HANDBOOK.md           agent onboarding doc
rules/                *.md — always_on or glob-triggered rules
skills/<name>/SKILL.md
agents/<name>.md      custom subagent profiles
scripts/              hook + verification scripts (Bash 5; Git Bash on Windows; canonical verify/codegen are PowerShell 7 `pwsh`, ADR-0058)
```

## Conventions

- Rules use `trigger: always_on`, or `trigger: glob` with `globs:` as a YAML sequence. Keep always-on rules short because they consume context every session.
- Skills trigger on `user` + `model` by default. Add `allowed-tools` when a skill must be read-only.
- Agents are plain `*.md` profiles; `name:` must not collide with built-in profiles. `model:` is intentionally unset so profiles inherit Devin's default subagent model.
- Hook scripts must be deterministic and fast (<5s except the Stop gate). Safety guards block explicit policy violations; heavy verification runs only at Stop/checkpoints.
- Scripts parse hook JSON with `jq` when present and use a minimal fallback otherwise.
- `docs/**` is readable by all. Implementer sessions may not edit protected docs, `.devin/`, `AGENTS.md` or `README.md`; sessions started with `THINHTHAN_AGENT_ROLE=spec-owner` may edit protected specs/ADRs (spec-change PRs, `policy-review`). The rule is enforced by `pre_write_guard.sh` and `pre_exec_guard.sh` because `config.json` deny rules cannot depend on the role.
- Secret material stays unreadable to every agent tool (`config.json` denies `*.pem`/`*.key`). The only exception is role-scoped in `pre_exec_guard.sh`: a session with `THINHTHAN_AGENT_ROLE=reviewer` may run exactly `pwsh -NoProfile -File .devin/scripts/policy_review.ps1 ...`, which reads the App key named by `THINHTHAN_POLICY_APP_KEY_FILE` in-process and posts the `policy-review` check run (ADR-0072).

## Adding a new agent / skill / rule

1. Create the file at the path above with valid frontmatter and a focused body.
2. Check existing entries first; do not create overlapping workflows.
3. Reference canonical `docs/` instead of copying contracts.
4. Validate with:

```bash
jq empty .devin/config.json .devin/hooks.v1.json
bash -n .devin/scripts/*.sh
devin doctor --json
devin rules list
devin skills list
```

`devin rules list` must contain no `Errors:` section.

## Testing hooks manually

```bash
echo '{"tool_input":{"command":"git push --force"}}' | bash .devin/scripts/pre_exec_guard.sh
echo '{"tool_input":{"command":"pwsh -NoProfile -File .devin/scripts/policy_review.ps1 -Repo o/r"}}' | THINHTHAN_AGENT_ROLE=reviewer bash .devin/scripts/pre_exec_guard.sh   # allowed
echo '{"tool_input":{"command":"cat key.pem"}}' | THINHTHAN_AGENT_ROLE=reviewer bash .devin/scripts/pre_exec_guard.sh                                        # blocked
echo '{"tool_input":{"file_path":"x.pb.go"}}' | bash .devin/scripts/pre_write_guard.sh
echo '{"tool_input":{"file_path":"docs/05_network/messages.md"}}' | bash .devin/scripts/pre_write_guard.sh      # blocked (implementer)
echo '{"tool_input":{"file_path":"docs/05_network/messages.md"}}' | THINHTHAN_AGENT_ROLE=spec-owner bash .devin/scripts/pre_write_guard.sh   # allowed
echo '{"tool_input":{"file_path":"docs/10_implementation/task_queue.md"}}' | bash .devin/scripts/pre_write_guard.sh
echo '{"stop_hook_active":false}' | bash .devin/scripts/stop_verify_gate.sh
echo '{}' | bash .devin/scripts/session_context.sh
```
