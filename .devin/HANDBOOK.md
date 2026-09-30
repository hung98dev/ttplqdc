# AI Engineering Handbook — thinhthan

Onboarding for coding agents. Dense facts only — read `AGENTS.md` + `docs/` for the actual contracts.

## What this is

AI-first MMORPG ("Thỉnh Thần", Vietnamese folklore fantasy), built entirely by AI agents with no human in the merge loop (ADR-0050). The repository is currently **docs-only**: no `IMP-*` task has started; implementation begins at `IMP-000` (`docs/10_implementation/wave_execution_prompts.md`). `docs/` is the source of truth. Implementers may not edit protected docs (only status/claim fields, blocker entries and their own evidence under `docs/10_implementation/`); the `spec-owner` role (`THINHTHAN_AGENT_ROLE=spec-owner`) edits protected specs/ADRs through spec-change PRs.

## Repository layout (planned; each path is created by its owning IMP task)

```text
proto/thinhthan/v1/     wire source of truth (IMP-061)
server/                 module thinhthan, Go 1.27.1
  cmd/verify            canonical Q0-Q6 entrypoint (IMP-000)
  cmd/server            the only production binary thinhthan-server (IMP-006/IMP-069)
  cmd/compiler|migrate  build/operator tools (IMP-003/IMP-005)
  internal/core/id      stable IDs/revisions/operations (IMP-001)
  internal/protocol/v1  generated Go protobuf (never hand-edit)
  internal/{edge,sim,durable,global,config,conformance,stackpin}
  migrations/           000001_baseline_schema (IMP-005)
client/                 Unity 6000.6.1f1 (URP 2D Renderer)
deploy/prod/            systemd unit + pinned CA bundle (IMP-048)
scripts/                verify.ps1, codegen.ps1 (PowerShell 7, Linux + Windows)
docs/                   numbered specs 00..11
.devin/                 agent governance (rules/skills/hooks/agents/scripts)
```

## Architecture invariants

```text
one process = Edge + Sim + Durable + Global (thinhthan-server, ADR-0044)
sim: no SQL, no edge import         durable: no sim import
edge: routes intent only            protocol: generated, domain-free
global: in-process single-writer (party/chat/matchmaking/Spirit Surge)
single-owner primitives: combat engine / items / currency / reward claims / rng
client = untrusted; sends intent; server owns every result
one world = one process = one PostgreSQL (ADR-0052); login queue at WORLD_CCU_CAP
channel cap 18 (ADR-0035); map cap 540; tick 20 Hz; p95 tick < 35 ms
```

## Toolchain pins (canonical: `docs/00_context/technology_versions.md`)

```text
Go 1.27.1 | Unity 6000.6.1f1 (C# 9.0) | PostgreSQL 18.6 | protoc 36.2
pgx/v5 5.11.0 | coder/websocket 1.8.15 | golang-migrate 4.20.1 | x/crypto 0.57.0 (Argon2id)
protobuf go 1.36.12 | Google.Protobuf 3.36.2 | slog | x/text | uax29
NO: routers, ORMs, zap/logrus/zerolog, Redis/Kafka/NATS, gRPC, math/rand v1
```

## Commands

| Purpose | Command |
|---|---|
| Scoped iteration checks | `bash .devin/scripts/verify_delta.sh` |
| Full local checkpoint / Stop equivalent | `bash .devin/scripts/verify_delta.sh --full` |
| Canonical clean-tree Q0-Q6 | `pwsh -NoProfile -File scripts/verify.ps1` locally with `-LocalDeferMissing` (CI: GitHub-hosted Linux + Windows jobs without the switch, ADR-0058, ADR-0072) |
| Diff risk classification | `bash .devin/scripts/diff_scope.sh` |
| Regenerate protobuf + Unity metadata | `pwsh -NoProfile -File scripts/codegen.ps1` |
| Regenerate golden fixtures | `cd server && go test ./internal/testing/protocol -run TestBinaryEncodingParity -update-golden` |
| Go suite | `go -C server test ./...` / affected sim/edge/durable/global with `-race` (needs cgo + C compiler; CI runs `-race` on Linux only) |
| Unity EditMode | `"$UNITY_EDITOR_PATH" -batchmode -projectPath client -runTests -testPlatform EditMode -quit` |

`.devin` auto-discovers the pinned Unity Hub editor path on Windows or Linux; a machine without it gets `WARN` and defers Unity to CI, which also materializes editor-generated files into artifact `unity-materialized-<os>` for you to commit (`agent_execution_protocol.md` §4b). Set `UNITY_EDITOR_PATH` only to another binary whose path contains `6000.6.1f1`. Codegen manages protoc under ignored `tools/`; global `protoc` is optional.

## Read order for any task

```text
AGENTS.md -> docs/README.md -> 00_context/technology_versions.md
-> owning spec (+ ADRs whose Consequences name it)
-> known_blockers.md -> task packet -> definition_of_done.md
```

## Never touch by hand

`docs/**` except `docs/10_implementation/**` (implementer role), `server/internal/protocol/**`, `client/Assets/Scripts/Protocol/**`, generated `client/Assets{,/Scripts{,/Protocol}}.meta`, `proto/testdata/golden/**`, `tools/**`, Unity generated folders, `server/vendor/`, `.git/`, committed migrations.

## Open blockers (check before claiming)

Canonical live list: `docs/10_implementation/known_blockers.md` (currently empty). Implementers add entries there and stop the dependent task; the `spec-owner` agent resolves them.

## DONE bar

`docs/10_implementation/definition_of_done.md` is canonical. Implementation and tests must match the owning spec; protected spec/ADR changes are made only by the `spec-owner` agent in spec-change PRs that pass `policy-review`. Before commit, `verify_delta.sh --full` requires all functional canonical gates plus scoped supplements; it may defer only Q6 clean-tree when the worktree contains exactly the reviewed local diff and verification creates no new drift. A DONE/evidence claim still requires canonical `scripts/verify.*` PASS from a clean tested source. Every PR needs the independent `reviewer` and the App check run `policy-review` (`.devin/scripts/policy_review.ps1`). Merges are serialized by the merge slot; merge sequence: `docs/10_implementation/agent_execution_protocol.md` §5a (canonical). Before IMP-000 exists, local checks report `SKIP(bootstrap)`.

## Governance layer map

| Piece | Path | Job |
|---|---|---|
| Always-on rules | `.devin/rules/00*,01*` | engineering discipline, risk+DoD |
| Scoped rules | `.devin/rules/10..15` | Go / Unity / proto / docs / SQL / art assets (glob-triggered) |
| Skills | `.devin/skills/<name>/` | workflows — invoke as `/name` |
| Subagents | `.devin/agents/<name>.md` | specialized profiles |
| Hooks | `.devin/hooks.v1.json` + `scripts/*` | SessionStart context, exec/write guards, post-edit nudges, Stop full-verification gate |
| Permissions | `.devin/config.json` | allow workspace ops; deny protected docs/secrets/generated/.git |

## Skill entry points

| Skill | Use when |
|---|---|
| `/run-wave` | owner says "làm wave N": coordinate one wave from claims to report |
| `/run-imp-task` | the default entry point: one IMP task from readiness to auto-merge |
| `/repo-architecture` | before unfamiliar, multi-module, or contract work; produces a read-only change map |
| `/implement-backend-feature` | implementing task-owned Go server behavior |
| `/implement-unity-feature` | implementing task-owned client/presentation behavior without wire changes |
| `/client-server-feature` | both ends or protobuf contract are affected |
| `/fix-backend-bug` | deterministic Go repro → regression test → root-cause fix |
| `/fix-unity-bug` | Unity lifecycle/scene/serialization repro and fix |
| `/network-debugging` | disconnect, desync, duplicate, ordering, latency, or serialization symptoms |
| `/database-change` | the protected data spec already defines the schema change (`server/migrations/`) |
| `/code-review` | severity-ranked independent diff review before DONE |
| `/produce-art-asset` | art/audio for IMP-070..076, IMP-104, IMP-105: 2x finish, cutout + volume gates, provenance, in-game review |

## Subagent profiles

| Profile | Scope |
|---|---|
| `backend-engineer` | Go/server only; stops on wire/client contract needs |
| `unity-engineer` | Unity/client only; no generated protocol or server contract edits |
| `integration-engineer` | approved proto + both consumers + compatibility |
| `debugger` | evidence-first root-cause analysis and regression fix |
| `reviewer` | read-only conformance verdict; posts the `policy-review` check run via `.devin/scripts/policy_review.ps1` (`THINHTHAN_AGENT_ROLE=reviewer`) |
| `verifier` | read-only diff-to-test-matrix execution |
| `spec-owner` | Contract Owner: resolves blockers, edits protected specs/ADRs via spec-change PRs (`THINHTHAN_AGENT_ROLE=spec-owner`) |
| `coordinator` | claims ready tasks (status-only PRs), grants the merge slot, records/resolves OPS entries (`ops/` PRs), unclaims stale claims, keeps concurrency within the limit of 5 tasks (2 with `client/`) |
| `asset-producer` | art/audio production within asset task owned paths; never gameplay/server/spec |
