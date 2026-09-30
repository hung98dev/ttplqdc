# Specification Index

This directory is the source of truth for implementation.

Project state: docs-only; no `IMP-*` task has started (first task `IMP-000`, `10_implementation/wave_execution_prompts.md`). The project is built entirely by AI agents with no human in the merge loop (ADR-0050). Protected specs and ADRs change only through spec-change PRs by the `spec-owner` agent role; implementer agents write only `10_implementation/` and record gaps in `10_implementation/known_blockers.md` (`../AGENTS.md`).

Exact engine/toolchain/database/core-dependency versions are canonical in `00_context/technology_versions.md`. AI agents must read that file before creating manifests, projects, generated code, CI, migrations, or infrastructure.

Read order (strictly aligned with `AGENTS.md`):
1. `docs/README.md` — specification index and status taxonomy
2. `00_context/` — product constraints, vision, vocabulary, technology versions
3. `11_decisions/` — ADRs; before reading or modifying any spec, search ADRs whose Consequences section names that spec
4. Relevant domain specs in dependency order:
   - `01_gameplay/` — core character/combat/progression rules
   - `02_world/` — world/runtime content rules
   - `03_systems/` — player-facing persistent systems
   - `04_architecture/` — runtime architecture
   - `05_network/` — wire/sync contracts
   - `06_data/` — persistence/config/content-bundle contracts
   - `07_content/` — concrete launch rosters, IDs, values, rewards and content production data
   - `07_security/` — trust and validation
   - `08_scale_ops/` — capacity and operations
   - `09_testing/` — verification
5. `10_implementation/` — execution order, task queue, and Definition of Done
The duplicated numeric prefix `07_*` is historical directory naming only; it does not imply content/security share ownership.

## Status Values
General spec status:
```text
TODO
DRAFT
LOCKED
```

Concrete content catalogs may additionally use:
```text
CONTRACT_ONLY
ROSTER_LOCKED
CORE_LOCKED
```
Their meaning is canonical in `07_content/README.md` and activation behavior is canonical in `06_data/config.md`.

ADRs in `11_decisions/` use `ACCEPTED`; amendments are recorded as notices at the top of the amended ADR and in the index (`11_decisions/README.md`).

`LOCKED` means implementation may depend on the document for its declared scope. It does not mean every other dependent catalog is automatically complete.

## Launch Gameplay / World / Systems
Canonical rule indexes:
- `01_gameplay/README.md`
- `02_world/README.md`
- `03_systems/README.md`

Concrete launch content index:
- `07_content/README.md`

Implementation agents should read:
```text
context -> owning rules -> owning launch catalog -> implementation/testing docs
```
Do not infer launch values from a generic mechanic document when a concrete content catalog owns them.

## Scope Guardrail
`LOCKED` does not mean the game can never evolve. It means new work must not silently invent or duplicate rules.

When adding a feature:
1. identify the owning canonical spec
2. update that spec first
3. keep one concept = one source of truth
4. avoid adding another mandatory power/economy/social subsystem when an existing system can express the design
5. use an ADR for architecture/data-contract changes required by the feature
6. update concrete launch catalogs when the rule change affects content IDs/values/references

## Setting Guardrail
The launch game's visible identity is Vietnamese folklore fantasy. Generic East-Asian/xianxia, Japanese-yokai, or Western-fantasy placeholder content must not silently become shipping content.

Ngũ Hành is a mechanical substrate; it is not permission to replace Vietnamese world/creature/prop identity with generic elemental fantasy.

The launch design intentionally favors a smaller number of deep, readable systems over feature accumulation.
