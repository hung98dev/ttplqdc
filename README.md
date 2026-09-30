# thinhthan

AI-first MMORPG, built entirely by AI agents (no human in the merge loop, ADR-0050). Spec: `docs/`. Agent rules: `AGENTS.md`. Implementation has not started: the repository owner completes Owner Setup (`docs/10_implementation/audit_gates.md`), then `IMP-000` is the first PR.

## Agent start

1. Read `AGENTS.md`.
2. Read `docs/00_context/technology_versions.md` (exact pins).
3. Read the owning spec and ADRs whose Consequences name it.
4. Put code only at the path in `docs/10_implementation/repository_layout.md`.
5. Follow `docs/10_implementation/agent_execution_protocol.md`; verify with `pwsh -NoProfile -File scripts/verify.ps1 -LocalDeferMissing` (PowerShell 7 on Linux or Windows; created by IMP-000). CI runs on GitHub-hosted Linux + Windows runners (ADR-0058), materializes Unity files for you to commit and is authoritative (ADR-0072).

Do not implement from assumptions. Pins: Go `1.27.1`, Unity `6000.6.1f1`, PostgreSQL `18.6`.
