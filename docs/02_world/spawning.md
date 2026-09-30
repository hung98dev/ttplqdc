# Spawning
status: LOCKED

## Scope
Defines authoritative runtime spawning for normal/elite monsters and other explicitly configured world entities.

Monster behavior belongs in `monsters.md`; boss lifecycle belongs in `bosses.md`. Concrete persistent launch field groups are owned only by `../07_content/map_spawn_catalog.md`.

## Identity
Stable definition:
```text
spawn_group_id
```
Runtime entities receive unique instance IDs and belong to exactly one:
```text
map_instance_id
```

## Spawn Group
Canonical runtime-expanded data:
```text
spawn_group_id
map_id
monster_selector
spawn_locator
population.target
population.max
respawn.mode
respawn.delay/range
conditions[]
```

### Monster Selector
Exactly one selector form is allowed:
```text
monster_id
```
or:
```text
monster_pool[] = { monster_id, weight }
```

Pool rules:
- server selects one monster definition independently for each spawn creation
- missing weight means equal weight among entries
- all weights are positive integers
- selection result is committed before entity creation retry and is not client-visible authority
- a pool does not allow the group to exceed `population.max`

### Spawn Locator
Exactly one locator form is allowed:
```text
spawn_area
```
or:
```text
anchor_id
```

`anchor_id` is a stable logical reference into the owning map asset. The map asset resolves it to legal fixed points/area geometry before content activation. Missing or invalid anchors fail static activation.

## Launch Authoring Shorthand
The launch catalog may use these deterministic compile-time aliases:
```text
max_alive        -> population.target = population.max = max_alive
respawn_seconds  -> respawn.mode = FIXED_DELAY; respawn.delay = respawn_seconds
```

Catalog-local short monster keys may compile to full `monster_id` only when the catalog explicitly defines the namespace-expansion rule. Runtime data stores full IDs.

Validation occurs after expansion. Shorthand never creates a second runtime schema.

## Population Invariant
```text
alive_population <= population.max
```

## Initial Population
When a map instance activates:
```text
initial_population = population.target
```
unless the spawn group explicitly defines a smaller initial population for encounter presentation.

Creation is staggered across up to `3s` to avoid one-tick spikes.

## Respawn Modes
Supported:
```text
FIXED_DELAY
RANDOM_DELAY
WAVE
```

Default for ordinary field monsters:
```text
RANDOM_DELAY
12s..18s NORMAL
45s..75s ELITE
```
Content may override these values. The launch persistent groups intentionally override them through `../07_content/map_spawn_catalog.md`; these generic defaults must not be used to replace the authored launch respawn values.

Respawn timer starts from authoritative death or configured permanent removal.

Despawn is not death and grants no kill rewards.

## Population Recovery
Spawner maintains target population, not one replacement timer per client-observed death.

If population is below target, it may schedule enough replacements to recover toward target, but never above `population.max`.

No explicit minimum population is required.

## Spawn Position Modes
Resolved locators may produce:
```text
FIXED_POINT
RANDOM_POINTS
RANDOM_AREA
```

Every chosen position must validate:
- correct map instance
- configured area/point
- map bounds
- legal ground/platform if required
- no blocked geometry

Position selection retries at most:
```text
8 attempts
```
If all fail, the spawn attempt is deferred by `2s`; it does not create an invalid entity.

## Player Proximity
Default minimum spawn distance from a visible player:
```text
6 meters-equivalent world distance
```
This rule may be disabled for scripted ambush/event content.

Spawn points outside the player's current camera are preferred when multiple legal positions exist.

## Sleep/Wake
To reduce CPU cost, ordinary spawn groups may sleep when no player is within the map instance's configured simulation-interest range for `30s`.

While sleeping:
- no new ordinary monsters are spawned
- already existing monsters may be frozen/despawned according to map-instance optimization policy
- no rewards/progression are generated

On wake:
- population is reconstructed toward target over `0-3s`
- previous runtime monster instances do not need persistence

Boss/dungeon encounter state follows its owning spec and cannot use this generic sleep rule to erase committed progress.

## Event Spawns
World-event spawn groups activate/deactivate from authoritative event state.

When an event ends:
- no new event entities spawn
- living event entities may be removed after a `10s` cleanup grace unless the event defines completion cleanup sooner
- cleanup despawn grants no death reward

`SPIRIT_SURGE` event timing belongs in `world_rules.md`.

## Wave Spawns
A wave definition has:
```text
wave_id
entries[]
start_condition
completion_condition
next_wave_delay
```
Default next-wave delay:
```text
3s
```
Waves are used only for explicit encounters/events, not ordinary field population maintenance.

## Multiple Instances
Each `map_instance_id` owns an independent population.

Population is never shared across channels merely because the `map_id` is the same.

## Restart Recovery
Normal world monsters are reconstructed from spawn definitions after restart.

Do not persist individual ordinary monster runtime instances.

Persistent encounter entities require an explicit owning-system rule.

## Capacity and Idempotency
Spawn creation uses stable server-side operation identity where retries are possible.

Retries, duplicate death events, lag, or recovery must never exceed configured `population.max`.

## Authority
Server owns:
- selector resolution
- activation conditions
- spawn timing
- entity creation/removal
- coordinates
- population counts
- randomness

Client cannot request arbitrary spawn creation, select a pool result, or accelerate timers.

## Design Guardrails
- Keep field density high enough that players spend more time fighting than waiting.
- Avoid instant respawns directly on top of players.
- ELITE respawns are slower to preserve discovery value.
- Do not use long timers as the primary retention mechanic.
- World-event density may rise temporarily but must remain bounded.

## Invariants
```text
selector = one monster_id OR one monster_pool
locator = one spawn_area OR one anchor_id
alive_population <= population.max
normal despawn != death
normal monster runtime instances are not persistent
client never selects authoritative spawn coordinate/timer/pool result
```