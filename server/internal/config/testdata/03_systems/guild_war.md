# Guild War (fixture)
status: LOCKED

Minimal CAT-006 spec-section source stub for content-compiler fixtures.
Exercises the `Map` canonical-geometry fence + required-topology bullets +
Stable objective IDs anchor set.

## Map

Canonical geometry:

```text
space_id = map.guild_war.test_conflict
span = 2.00 x 1.00 reference screens
bounds = (0,0)..(51.2m,14.4m)
reference_extent = 2560x720px at 50 px/m
layout_profile = TEST_SEAL_FRONT
```

Required topology:
- two seal plazas lie left-to-right in the declared order on the main front;
- guild spawns mirror about `x=25.6m` within `0.001m`.

Stable objective IDs:

```text
guild_war.seal.test_moc
guild_war.seal.test_kim
```

## Compiler Source Schema

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Map` / `text` fence `space_id, span, bounds, reference_extent, layout_profile` | space geometry / `space_id` | space_id:id; span:pair(decimal); bounds:range corner pair(decimal) m, min constant `(0,0)`; reference_extent:pair(int); layout_profile:enum token | `space_kind = GUILD_WAR` constant; the required-topology bullets are normative validation text |
| `Map` / `text` fence "Stable objective IDs" | space anchors / `map.guild_war.test_conflict` | ordered `id` tokens | the space's declared logical anchor set for export parity |
