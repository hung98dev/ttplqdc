# PvP (fixture)
status: LOCKED

Minimal CAT-006 spec-section source stub for content-compiler fixtures.
Exercises the competitive-space grammar: a geometry table row without
logical anchors and one with a declared anchor set.

# Competitive Space Geometry

| modes | space_id | span (screens) | bounds max (m) | reference extent (px) | layout_profile | required topology |
|---|---|---|---|---|---|---|
| `TEST_DUEL` | `map.pvp.test_court` | `1.00x1.00` | `25.6x14.4` | `1280x720` | `TEST_DUEL_BOWL` | mirrored central floor |
| `TEST_ARENA` | `map.pvp.test_arena` | `2.00x1.00` | `51.2x14.4` | `2560x720` | `TEST_ALTAR_CIRCUIT` | single altar route |

# Five Element Arena

The arena contains:

```text
altar.test
```

# Compiler Source Schema

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Competitive Space Geometry` / table `modes, space_id, span (screens), bounds max (m), reference extent (px), layout_profile, required topology` | space geometry / `space_id` | modes:set(enum(TEST_DUEL, TEST_ARENA)); space_id:id; span:pair(decimal); bounds max:pair(decimal); reference extent:pair(int); layout_profile:enum token; required topology:string | `space_kind = PVP` constant |
| `Five Element Arena` / `text` fence `altar.*` lines | space anchors / `map.pvp.test_arena` | ordered `id` tokens | the space's declared logical anchor set for export parity; `map.pvp.test_court` declares no logical anchors |
