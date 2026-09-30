# Configuration
status: LOCKED

## Scope
Defines static game content, runtime tuning, schema validation, revisions, reload rules, and the launch-catalog activation contract.

Architecture/data-contract rationale is recorded by `../11_decisions/0001-content-revision-contract.md`.

## Categories
`STATIC_CONTENT` includes skills, items, equipment, monsters, bosses, drops, quests, dungeons, world events, NPCs, shops, Souls, Meridian, Formations, cosmetics, maps and spawns.

`RUNTIME_TUNING` contains only fields explicitly marked safe to change without reinterpreting persistent or in-flight state.

## Identity
Every persistent definition uses an immutable stable ID. Display/localization text and array indexes are never identity.

Every content bundle records:
```text
schema_version
content_revision
created_at
```

Stable runtime IDs use ASCII machine-readable identifiers. Localized Vietnamese diacritics belong in display/localization data, not identity.

## Validation
A revision cannot activate with:
- duplicate IDs
- unresolved required references
- invalid enum/ranges
- invalid probability/weight data
- undeclared currencies/ownership contexts
- references to unknown canonical stats/effects/status tags
- impossible Meridian/Formation matchers
- missing map spawn-area references
- unresolved entity `size_profile`
- missing/mismatched playable-space bounds, `layout_profile`, scene key, or geometry export
- launch skill primary-geometry count other than 45, invalid ADR-0047 role/envelope value, prose-only secondary geometry, or displacement tag/effect mismatch
- invalid quest prerequisite cycles
- content status that explicitly declares required external references unresolved

Validation reports errors; it never silently rewrites content.

# Launch Catalog Ownership
Mechanics live in gameplay/world/system specs. Concrete launch data currently lives under `../07_content/`.

| Domain | Concrete owner |
|---|---|
| classes / skill kits | `class_skill_catalog.md` + class identity rules in `../01_gameplay/classes.md` |
| region / encounter narrative identity | `encounter_catalog.md` |
| normal-world map metadata / checkpoints / portals / discovery | `world_route_catalog.md` |
| field spawn groups | `map_spawn_catalog.md` |
| non-boss monster runtime definitions | `monster_catalog.md` |
| major boss runtime definitions | `boss_catalog.md` |
| dungeon stage/runtime definitions | `dungeon_catalog.md` |
| recurring world event | `world_event_catalog.md` |
| quests | `quest_catalog.md` |
| equipment / sets | `equipment_catalog.md` |
| non-equipment items | `item_catalog.md` |
| deterministic recipes | `crafting_catalog.md` |
| economy faucets / sinks / affordability | `economy_catalog.md` |
| drop / reward tables | `drop_tables.md` |
| Souls | `soul_catalog.md` |
| Meridian / Formations | `build_catalog.md` |
| NPCs / shops | `npc_shop_catalog.md` |
| cosmetics | `cosmetic_catalog.md` |
| Atlas pages | `atlas_catalog.md` |
| Linh Thú / beast equipment | `spirit_beast_catalog.md` |

A rules document is never a substitute for required concrete launch definitions.

## Catalog Status
Recognized documentation readiness states:
```text
CONTRACT_ONLY
ROSTER_LOCKED
CORE_LOCKED
LOCKED
```

Interpretation:
- `CONTRACT_ONLY`: budget/rules exist but concrete launch roster is missing.
- `ROSTER_LOCKED`: identities/mechanics are concrete; named external acquisition/reference dependencies remain.
- `CORE_LOCKED`: core runtime sources are concrete; the file names the remaining catalog dependency needed for full closure.
- `LOCKED`: concrete for the file's declared scope.

A production launch bundle may include non-`LOCKED` documentation only when the file's own named unresolved dependencies are not required by the activated bundle. Normal launch activation should target all required catalogs as `LOCKED`.

## Finite Deterministic Expansion
A catalog may define a finite expansion instead of manually repeating hundreds of equivalent rows when all of the following are explicit:
```text
finite input domain
stable ID generation pattern
all gameplay values / formulas
ownership / binding rules
static expansion validation
```
Example:
```text
12 set keys x 14 canonical slots -> 168 concrete equipment item IDs
```
This is concrete content, not a wildcard runtime identity. The build pipeline expands/validates the full set before activation; runtime never invents new IDs from arbitrary strings.

## Probability
Gameplay probability uses integer basis points or integer weights. Server RNG owns outcomes. Combat/drop/enhancement streams use Go `math/rand/v2` PCG-64 per `../04_architecture/concurrency.md`. UUID/secrets remain `crypto/rand` in `ids.md`.

A committed random reward result is persistent transaction state and is never rerolled because of inventory overflow, reconnect, retry, or content revision.


## Versioning
Persisted instances retain required generated state. Breaking interpretation changes require migration rather than silent reinterpretation.

Definition removal must preserve enough historical/migrated state to interpret already-owned persistent instances or explicitly migrate them before activation.

## Reload
Default:
```text
STATIC_CONTENT -> controlled activation/restart
RUNTIME_TUNING -> hot reload only when field is explicitly hot_reload_safe
```
Do not hot reload fields that reinterpret:
- in-flight combat
- active encounter mechanics
- transactions
- reward settlement
- ownership/binding
- persistent identity
- quest prerequisite truth


## Map Geometry Compile
Server runtime collision, platforms, blocked volumes, and legal spawns are compiled from validated playable-space definitions and `anchor_id` references (`../04_architecture/physics_geometry_contract.md`, `../07_content/world_route_catalog.md`, `../07_content/dungeon_catalog.md`, `../03_systems/pvp.md`, `../03_systems/guild_war.md`). Unity scenes are authoring sources, not runtime authority.

Compile output is an immutable snapshot keyed by `content_revision + space_id`. It contains `space_kind`, exact bounds, `layout_profile`, logical anchors, and quantized collision segments. A remote Unity scene cannot redefine that snapshot. Missing or invalid geometry fails content activation rather than falling back to client meshes.

`1280x720` is the reference viewport, never a fallback map size. Normal-world spaces must compile to the exact `2.0..5.0`-screen bounds and distinct topology profiles in `world_route_catalog.md`.

## Atomic Activation
Validate one content revision as a dependency graph, then activate atomically.

Runtime encounters/transactions may pin the revision they started with. New sessions/encounters use the active revision after cutover according to owning-system rules.

## Rollback
Keep the previous validated revision available.

Rollback changes active definitions but never reverses committed player state, transaction results, quest completion, reward outcomes, Soul/equipment rolls, or currency operations.

### Schema-Coupled Content Revisions
A content revision is tagged **`schema-coupled`** when it changes a formula whose output is persisted to character rows — specifically: EXP threshold tables, level cap, or any other value stored in durable character state rather than recomputed on load. Examples: an EXP rescale that writes `current_exp` and character level to PostgreSQL.

**Who sets it**: the author of the content revision, confirmed during pre-activation review. Activation tooling must carry the `schema-coupled` tag through the deployment pipeline; a revision that meets the criteria but lacks the tag must not activate.

**Binary rollback of a schema-coupled revision is not permitted.** A content rollback that reverts EXP thresholds or level caps cannot undo committed character rows; characters would be re-evaluated against old thresholds and could land above the level cap or in an invalid progression bucket. When a schema-coupled revision must be reverted:
- a compensating migration that re-normalizes affected character rows is required, or
- a forward-only fix plan (new revision that repairs the invariant without reverting) must be prepared and deployed instead of a binary rollback.

The `schema-coupled` tag is recorded in the content revision metadata alongside `schema_version`, `content_revision`, and `created_at`. See `../08_scale_ops/deployment.md` for the deployment-side invariants.

## Client Contract
Clients may receive presentation-safe subsets and revision IDs.

Server remains authoritative for:
- RNG
- hidden reward weights
- prices at settlement
- combat
- spawn decisions
- quest progression
- ownership/binding
- reward validation

A client content file is never authority merely because it contains the same static values.

## Cross-Catalog Validation Examples
Activation rejects examples such as:
```text
Soul source monster missing
quest target map missing
spawn group monster missing
shop offer item missing
equipment recipe output missing
boss reward references unknown Soul
Meridian/Formation relation impossible
starter checkpoint missing
```

## Invariants
```text
persistent IDs immutable
activation atomic
unresolved required reference -> activation failure
finite expansion -> pre-expanded/validated before activation
static hot reload = opt-in
rules spec != launch content catalog
client config != authority
schema-coupled revision tag required when formula output is persisted to character rows
binary rollback of schema-coupled revision not permitted without compensating migration or forward-only fix
```
