# world_route_catalog.md
status: LOCKED

Fixture world-route catalog: one field map with bounds.

## Compiler Source Schema

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Map Metadata` / table `map_id,type,recommended,entry_spawn,first_discovery_exp` | world_map / `map_id` | map_id:id; type:enum; recommended:range; entry_spawn:id; first_discovery_exp:grouped_int | One FIELD map |
| `Canonical Bounds` / table `map_id,span (screens),bounds max (m),reference extent (px),layout_profile,required traversable topology` | space_geometry / `space_id` | map_id:id; span:pair; bounds:pair; extent:pair | same row count as Map Metadata |

## Map Metadata

| map_id | type | recommended | entry_spawn | first_discovery_exp |
|---|---|---|---|---|
| `map.fx1` | FIELD | 1-10 | `anchor.fx1` | 1,000 |

## Canonical Bounds

| map_id | span (screens) | bounds max (m) | reference extent (px) | layout_profile | required traversable topology |
|---|---|---|---|---|---|
| `map.fx1` | 12x8 | 220x160 | 1920x1080 | open | LAND |
