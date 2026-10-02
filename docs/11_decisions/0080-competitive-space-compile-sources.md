# ADR-0080: Competitive-Space Compile Sources
status: ACCEPTED

## Context

`physics_geometry_contract.md` §6.1 enumerates five `space_kind` values (`WORLD | DUNGEON | FINALE | PVP | GUILD_WAR`), `config.md` § Map Geometry Compile names `../03_systems/pvp.md` and `../03_systems/guild_war.md` as playable-space compile sources, `content_authoring_contract.md` §4.5 resolves a playable-space index of 33 spaces (24 world + 5 dungeon + finale + duel + arena + Guild War), and `integration_validation.md` requires every listed competitive `space_id` to resolve exact bounds + layout profile + export. `IMP-062` Acceptance commits collision scenes for three competitive spaces and `TestAnchorSetMatchesCatalog`/`TestCompetitiveMirrorParity` parity.

Yet the contract's compile-input set covered only the 24 catalogs in `docs/07_content/`: the three competitive spaces were declared only in `03_systems/` system specs with no `Compiler Source Schema` registry, so the compiler could not ingest them. The wave-3 audit flagged the resulting 30-vs-33 space coverage gap (finding F-2.2).

## Decision

- The competitive-space geometry declarations in `../03_systems/pvp.md` (§ Competitive Space Geometry table and § Five Element Arena anchor fence) and `../03_systems/guild_war.md` (§ Map geometry and objective-ID fences) are **registered spec-section compile sources** under `content_authoring_contract.md` §1 grammar. Each file declares a `Compiler Source Schema` registry binding those sections to `space_geometry` (`space_kind` constants `PVP` and `GUILD_WAR` respectively) and to their declared logical anchors.
- These spec sections remain the single source of truth for their data; they are not a 25th gameplay catalog, and the 24-catalog set in contract §3 is unchanged.
- `SPARRING_RING` has no separate scene and is excluded from the space set.
- `map.pvp.duel_court` declares no logical anchors; `map.pvp.five_element_arena` anchors `altar.left|center|right`; `map.guild_war.five_seal_conflict` anchors `guild_war.seal.{moc,hoa,tho,kim,thuy}`.
- Registered-source parity wording replaces "catalog" where the physics contract refers to space declarations and required anchor sets.
- Measurable contract: `CAT-006` (`content_authoring_contract.md` § Requirement IDs); regression coverage via `TestCompetitiveSpaceGeometryIndex` in `IMP-003` `## Tests`, consumable parity via `IMP-062`.

## Consequences

- `docs/06_data/content_authoring_contract.md` — §Scope, §1, §3, §4.5 and `CAT-006` extend the compile-input set to the two registered spec sections.
- `docs/03_systems/pvp.md`, `docs/03_systems/guild_war.md` — gain `Compiler Source Schema` registries.
- `docs/04_architecture/physics_geometry_contract.md` — space-declaration parity references "registered compile source" instead of "catalog".
- `docs/10_implementation/task_queue.md` — `IMP-003` specs/contract_inputs/Acceptance/Tests register `CAT-006`; executable compiler support (two added compile inputs, `PVP`/`GUILD_WAR` `space_kind`) lands as a gatefix on `IMP-003` owned paths, dispatched by the coordinator.
- `docs/10_implementation/spec_traceability.md` — `CAT-006` row maps the requirement to `IMP-003, IMP-062`.
- `docs/06_data/config.md`, `docs/07_content/integration_validation.md` — already enumerated these sources/spaces; unchanged.
