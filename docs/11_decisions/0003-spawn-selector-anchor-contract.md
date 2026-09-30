# ADR-0003: Spawn Pools and Logical Map Anchors
status: ACCEPTED

## Context
The concrete launch spawn catalog uses compact monster pools and logical `anchor_id` references, while the original generic spawn schema required one `monster_id` and one direct `spawn_area`. Without a canonical expansion rule, the content catalog and runtime contract disagree.

## Decision
- A spawn group uses exactly one monster selector: one `monster_id` or one weighted/equal `monster_pool[]`.
- A spawn group uses exactly one locator: direct `spawn_area` or logical map-asset `anchor_id`.
- Pool selection is server-authoritative per spawn creation.
- Logical anchors must resolve during static-content validation; missing anchors prevent activation.
- Launch shorthand such as `max_alive` and `respawn_seconds` compiles deterministically into the canonical population/respawn fields.
- Runtime-expanded data stores full monster IDs and canonical fields; shorthand is authoring-only.

## Consequences
- `map_spawn_catalog.md` remains compact without creating a second runtime schema.
- Map artists can move geometry without changing stable content IDs as long as required anchors remain valid.
- Static validation gains selector/anchor/expansion checks.
