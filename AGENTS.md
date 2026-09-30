# AI Project Rules

Purpose: this repository is specified and implemented entirely by AI agents (ADR-0050, ADR-0057, ADR-0058). The only human role is the repository owner: one-time Owner Setup and fixing `OPS-xxx` environment failures (`docs/10_implementation/audit_gates.md`).

Rules:
- Treat `docs/` as the project specification and source of truth.
- Do not invent missing rules. An implementer that finds a gap or contradiction records it in `docs/10_implementation/known_blockers.md`, sets the task `BLOCKED` and stops; the `spec-owner` agent resolves it.
- Prefer deterministic rules, explicit IDs, schemas, state transitions, formulas, limits, and error cases.
- Keep docs short. No essays, marketing text, repeated explanation, or technology comparisons unless a decision is still open.
- One concept has one canonical document. Other docs reference it instead of duplicating it.
- Behavior changes start with the spec: the `spec-owner` agent lands the spec/ADR change in a spec-change PR first; implementation follows in the task PR.
- Implementation must satisfy `docs/10_implementation/definition_of_done.md`; the execution workflow is `docs/10_implementation/agent_execution_protocol.md`.
- Decision changes that affect architecture or data contracts require an ADR in `docs/11_decisions/`.
- Technology/toolchain/dependency versions are canonical in `docs/00_context/technology_versions.md`. Use the exact pins; never choose `latest`, a floating range, a prerelease, or an unlisted substitute.
- Do not add a new runtime/framework/infrastructure dependency to solve a local task until its exact version and ownership are approved in the canonical version matrix and relevant spec.

## Owner commands
When the owner says "làm wave N", "chạy wave N", "do wave N" (or only the wave number), you are the coordinator for wave N: execute the **Wave Prompt** in `docs/10_implementation/wave_execution_prompts.md` with `<N>` = that number (skill `/run-wave`). Do not ask for confirmation; finish with the wave report defined there.

Read order:
1. `docs/README.md`
2. `docs/00_context/`
3. `docs/11_decisions/` — before modifying any spec, search ADRs whose Consequences section names that spec; if one exists, read it
4. relevant domain specs
5. `docs/10_implementation/`

## Contract-change rule

**When you change a contract — a formula, a message, a stat, an ID scheme, a durable key, a constant, or a data-model field — you must identify and check every spec that consumes it, not only the one that defines it.** Use grep across `docs/` for the symbol or constant name before editing. Changing only the defining file is a defect.

## Spec changes and cross-directory changes

Protected specs (`docs/**` outside `docs/10_implementation/`, plus the policy files listed in `docs/10_implementation/audit_gates.md`) change only through a **spec-change PR** made by the `spec-owner` agent role (the Contract Owner in `docs/10_implementation/agent_execution_protocol.md`). Implementer agents never edit protected specs.

A change touching files in more than two numbered directories must list the complete grep-derived consumer set in its change packet and pass `policy-review` (independent AI reviewer). Cross-directory changes have caused silent defects (e.g. a mechanic made non-functional by a coalescing rule defined two directories away; a durable table keyed on a runtime identity defined in a different directory).

## Concept-to-owner index

| Concept | Canonical file |
|---------|---------------|
| EXP curve and level budgets | `01_gameplay/progression.md`, `00_context/constraints.md` |
| Skill unlock schedule and skill-point totals | `01_gameplay/progression.md` (ADR-0033, ADR-0025) |
| Just Guard (trigger, streak, latency model, wire protocol) | `01_gameplay/combat.md` (ADR-0034, ADR-0038) |
| Combat damage pipeline (resolution order, caps) | `01_gameplay/combat.md`, `01_gameplay/stats.md` |
| Item ownership and binding (CHARACTER_BOUND, ACCOUNT_SCOPED_ACCESS) | `03_systems/items.md`, `03_systems/account_storage.md` (ADR-0029, ADR-0041) |
| IAP entitlements and season track | `03_systems/monetization.md`, `03_systems/account_storage.md` (ADR-0041, ADR-0053) |
| Guild Stone | `03_systems/guild.md` |
| Channel capacity and routing | `02_world/world_rules.md`, `02_world/maps_zones.md` (ADR-0020, ADR-0035) |
| World topology (one world, one process, login queue) | `04_architecture/system_overview.md`, `07_security/session.md` (ADR-0044, ADR-0052) |
| Wire message registry, envelope and delivery classes | `05_network/messages.md`, `05_network/protocol.md` (ADR-0054) |
| Error codes (protocol, session, domain) | `05_network/errors.md` |
| Durable aggregate keys | `06_data/data_model.md` (ADR-0040, ADR-0053) |
| Aggregate lock order | `06_data/database.md` |
| Login providers, password, operator auth | `07_security/auth.md` (ADR-0051) |
| Personal data, retention, erasure | `07_security/data_protection.md` |
| Atlas page roster and currency.special faucet | `07_content/atlas_catalog.md` (ADR-0042) |
| Dungeon / Spirit Surge EXP rules | `07_content/dungeon_catalog.md`, `07_content/world_event_catalog.md` (ADR-0032) |
| Content catalog status values | `07_content/README.md` |
| Asset size, cutout, volume and review gates | `07_content/presentation_asset_manifest.md` (ADR-0055, ADR-0056) |
| CI, bootstrap, evidence, merge and review policy | `10_implementation/audit_gates.md`, `10_implementation/agent_execution_protocol.md` (ADR-0050, ADR-0057, ADR-0058, ADR-0068, ADR-0072) |
| Client performance and smoothness | `04_architecture/client_performance.md` |
| Client smoothness by construction (FrameLoop, FrameBudget, governor) / code-quality gates | `04_architecture/client_performance.md` § Smoothness by Construction, `10_implementation/engineering_conventions.md` §1.1, §1.7, §2.3–§2.7 (ADR-0059) |
| Roles (spec-owner, coordinator, implementers, reviewer) | `10_implementation/README.md` |

## Implementation execution

Before writing code, read in order:
1. `docs/00_context/technology_versions.md` — exact pins
2. owning spec + ADRs whose Consequences name it
3. `docs/10_implementation/repository_layout.md` — path must already exist or be owned by the task
4. `docs/10_implementation/definition_of_done.md`

| Code | Path |
|---|---|
| Go IDs / UUID | `server/internal/core/id/` |
| Content compile / activation | `server/internal/config/` |
| Realtime sim | `server/internal/sim/` |
| Persistence | `server/internal/durable/` |
| Session / WSS / HTTPS auth | `server/internal/edge/` |
| Operator admin API | `server/internal/edge/admin/` |
| Party / WORLD chat / matchmaking / Spirit Surge / boss generation | `server/internal/global/` |
| Generated Go proto | `server/internal/protocol/v1/` |
| Generated C# proto | `client/Assets/Scripts/Protocol/` |
| Client rendering / lighting | `client/Assets/Scripts/Core/Rendering/` |
| Wire schema | `proto/thinhthan/v1/` |

`proto/` is the wire source of truth. Never hand-edit generated protocol outputs. Regenerate with `scripts/codegen.ps1`.

Forbidden — do not add, do not invent:
```text
Redis / Kafka / NATS / distributed cache / extra server binaries / microservices
second world / Kubernetes / container runtime in production (ADR-0052)
Gin Chi Echo Fiber gorilla/mux gorilla/websocket gRPC
GORM sqlx zap logrus zerolog
math/rand          -> math/rand/v2 or crypto/rand
account item vault -> account_storage.md is IAP panel only (ADR-0029)
open-world PK, swim stamina, mail, 4th currency, mounts, invented shop prices
C2S_AUCTION_BID    -> FIXED_PRICE buy only
CHARACTER_ALREADY_ACTIVE on replace login -> SESSION_REPLACED (ADR-0030)
latest / floating / prerelease versions
Bash verify/codegen wrappers (ADR-0050); self-hosted / GPU / larger runners, owner-managed VM, `*-latest` runner image (ADR-0058)
fork PRs; job-level `if` that skips a required check; secrets before the fork guard (ADR-0058)
git rebase / force-push / direct push to main / self-review; auto-merge without the merge slot (ADR-0050, ADR-0072)
client runtime: Update/FixedUpdate/LateUpdate/OnGUI outside FrameLoop, coroutines, LINQ, Find*/SendMessage,
  Resources.Load, async void, Debug.Log outside Log, runtime material instances (ADR-0059; full list engineering_conventions.md §2.5)
second pool / scheduler / logger / frame driver implementation (engineering_conventions.md §2.6)
C# compiler warnings; //lint:file-ignore; unpinned linters or .NET SDK/Roslyn tooling in CI (ADR-0059)
```

Invariants:
```text
one world = one process = one PostgreSQL database (ADR-0052)
one binary = thinhthan-server; one process = Edge + Sim + Durable + Global
Global = in-process single-writer; not Redis; not Kafka; no global_leader_lease
no production --role split; no extra server binaries
simulation 20 Hz; p95 tick < 35 ms; channel cap = 18 (ADR-0035); map = 540
C# 9.0 Allman; Go tabs; proto indent 2
Go toolchain = 1.27.1
textures authored at 2x, imported at 100 PPU (UI 200) (ADR-0055)
public repo; CI = GitHub-hosted ubuntu-24.04 + windows-2022 only, no GPU (ADR-0058)
client: one FrameLoop (Input -> NetReceive -> Prediction -> Interpolation -> Presentation -> UI -> Camera); FrameBudget <= 2 ms/frame (ADR-0059)
csc.rsp -warnaserror+ -nullable:enable; LF line endings; gofmt + go vet + staticcheck 2026.2.1 clean
hot-path allocations are exact gates; timing on hosted CI = median of 3; ns/op report-only
```

Done:
- `DONE` is defined only by `docs/10_implementation/definition_of_done.md`; the merge sequence only by `docs/10_implementation/agent_execution_protocol.md` §5a
- spec changes land first in a spec-change PR; code + tests + status + CI-produced evidence land in the task PR
- verify: `pwsh -NoProfile -File scripts/verify.ps1` (locally with `-LocalDeferMissing`; CI without it on GitHub-hosted Linux + Windows jobs on every PR); Go: `go -C server ...`; `-race` runs on the Linux job only
- CI materializes Unity editor output as artifact `unity-materialized-<os>`; commit it byte-for-byte (`agent_execution_protocol.md` §4b); no local Unity editor is required
- merges are serialized by the coordinator's merge slot (label `merge-slot`); only the slot holder updates from `main`, adds evidence and enables auto-merge; the PR merges (squash) when `Q0-Q6 verify (Linux)`, `Q0-Q6 verify (Windows)` and the App check run `policy-review` are green (ADR-0072)
- blockers reach `main` through status-only `block/` (implementer) or `ops/` (coordinator) PRs; the owner only fixes the environment and closes `ops-blocked` issues
- each role session uses its own agent token with the permissions listed in `docs/10_implementation/audit_gates.md` § Owner Setup; the reviewer posts `policy-review` only via `.devin/scripts/policy_review.ps1`
- verify fail = not done
