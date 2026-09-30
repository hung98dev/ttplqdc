# World Rules
status: LOCKED

## Scope
Defines shared-world topology, channels, instancing, global event cadence, persistence boundaries, and world-wide gameplay invariants.

## World Model
The game uses one logical persistent world divided into maps and map instances.

Stable identities:

```text
world_id
map_id
map_instance_id
```

`map_id` identifies content. `map_instance_id` identifies one running copy.

## Channels (Khu Vực)
Every normal open-world map (safe anchors and adventure fields) has exactly **30 channels** (`CHANNELS_PER_MAP = 30`), numbered `1..30` under ADR-0020.

Rules:
- channels contain identical map topology and spawn definitions
- character progression, inventory, quests, guild, party, auction, and chat are globally shared across all channels
- monsters, dynamic spawns, and dropped personal loot are channel-local
- changing channel is a server-authoritative transfer
- channel selection cannot be used while `in_combat` or under a build lock
- channel transfer enforces a `10s` cooldown after successful switch

## Capacity
Player capacity per channel:
```text
MAX_PLAYERS_PER_CHANNEL = 18   ← reduced from 20 (ADR-0035 resolved)
SOFT_THRESHOLD_CHANNEL  = 18   (hard cap now aligns with the soft threshold; no grey zone)
TOTAL_MAP_CAPACITY      = 540  (30 channels × 18 players)
```

ADR-0035 rationale: 18 was already the soft threshold; aligning the hard cap eliminates the 18–20 grey zone so routing logic, the Full display state, and the hard reject all agree on the same number. Binary rollback to cap=20 is not permitted without first resolving spawn-balance at the higher density.

**World capacity and 10k CCU:** The launch world contains 24 normal open-world maps (18 adventure FIELD maps + 6 safe/social TOWN anchors), each with 30 channels × 18 players = 540 per map. Combined open-world capacity = 24 × 540 = **12,960 players**. Instanced content (dungeons, PvP, boss arenas) adds further headroom. 10,000 CCU is comfortably accommodated.

**Spawn supply/demand model (per channel, per hour — both figures from ADR-0035):**
- Demand — realistic sustained: **450 kills/hour** once travel, looting, death, and idle time are counted. 600 kills/hour is the peak-optimal rate (one kill every 6 seconds, continuously) and must not be used as the planning average.
- Supply — conservative planning floor (slowest 14s respawn, ADR-0062): ~40 alive monsters × (3,600 / 14) ≈ **10,286 kills/hour**. Demand is sized at `FORCED_PLACEMENT_HARD_CAP`: 22 players × 450 = 9,900/hour → ratio ≈ 1.04× (18 players: 8,100/hour → 1.27×).
- Supply — expected operating figure (average 13s respawn): ~40 × (3,600 / 13) ≈ **11,077 kills/hour**, giving a ratio of ~1.37× against realistic demand. The difference between 10,286 and 11,077 comes solely from respawn cadence assumptions; 10,286 is the planning floor and 11,077 is the expected operating supply.

Load status thresholds:
- `1..11 players`: Normal (Bình thường)
- `12..17 players`: Busy (Đông)
- `18 players`: Full (Đầy)

When a channel reaches 18 players, player-initiated arrivals and transfer requests into that channel are rejected. Automatic placement routes new players to the **most populated running** channel with `player_count < 18`; if no running channel is below 18, it starts the lowest-index stopped channel (`../08_scale_ops/sharding.md` § Channel Partition Lifecycle, ADR-0070). If all 30 channels are running at 18, reject with `MAP_CAPACITY_FULL` as below.

When all 30 channels are at capacity, player-initiated normal-map entry (portal, travel service, map selection) and channel-transfer requests are rejected with `MAP_CAPACITY_FULL`; the server does not queue, evict, or silently route the character to another map. The response includes `retry_after_ms = 5000`. A retry uses the same entry intent but a new operation ID after that interval.

### Forced Placement (ADR-0061)
Server-initiated placements cannot be refused by the player and never return `MAP_CAPACITY_FULL`: checkpoint respawn, dungeon/finale exit or closing transfer, PvP/Guild War return, reconnect fallback (`maps_zones.md` § Reconnect) and first login to the starter map.
```text
FORCED_PLACEMENT_HARD_CAP = 22 players per channel (MAX_PLAYERS_PER_CHANNEL + 4)
1 preferred channel if player_count < 22
    respawn: current channel if the checkpoint is on the same map (else none); instance exit/return: the
    recorded entry channel; reconnect: previous channel; first login: none
2 else the running channel with the lowest player_count < 18 (tie -> lowest channel index)
3 else the lowest-index stopped channel (started for this placement, ../08_scale_ops/sharding.md)
4 else (all 30 channels running and >= 18) the running channel with the lowest player_count < 22
    (tie -> lowest channel index)
5 else (all 30 channels at 22): placement pending; the server sends S2C_PLACEMENT_PENDING
    {reason : RESPAWN | INSTANCE_RETURN | RECONNECT | FIRST_LOGIN, retry_after_ms = 5000} and repeats
    steps 1..5 every 5s until placed; the normal transfer/respawn/attach messages then follow
```
While placement is pending (ADR-0062):
```text
RESPAWN          character stays dead at the death position; UI/chat allowed; no further penalty
INSTANCE_RETURN  character stays in the closing/FAILED instance; the instance stays open (no combat,
                 no empty-instance timeout) until its last pending member is placed
RECONNECT        session attached, character in no world partition; client shows the loading screen
FIRST_LOGIN      same as RECONNECT
```
Placement pending is not the CCU login queue of `../07_security/session.md` (that queue applies only at `WORLD_CCU_CAP` before a session attaches); a placement-pending session counts toward CCU. Disconnecting while pending leaves the character at its last authoritative position (reconnect rules apply).
A channel above 18 shows Full and keeps rejecting player-initiated arrivals until it drops below 18. No player is evicted to restore 18.
## Party Cohesion
When entering a normal world map together, party members should be routed to the same map instance when capacity allows.

Party membership does not force teleportation and does not continuously migrate members across instances.

## Safe Zones
A map or zone may declare:

```text
safe_zone = true
```

Inside a safe zone:
- hostile PvE actors cannot acquire new player targets unless content explicitly overrides it
- player damage is disabled
- normal NPC, trade, auction, crafting, and social interactions are allowed
- logout is immediate after the normal session save completes

## Village Bonfire Gathering (Lửa Trại Đình Làng)
Every Safe Anchor features a central communal bonfire under ADR-0023:
- **Resting Radius and Initiation**: Characters within 6.0m of an active bonfire initiate `BONFIRE_REST` by sending `C2S_INTERACT` (103) with `interact_kind = BONFIRE_REST` and `target_id = bonfire.<map_id>` (e.g. `bonfire.map.lang_da.dinh_lang`, `../07_content/world_route_catalog.md`). The server validates that the bonfire is active (kindled), the character is stationary within the 6.0m radius, and has no active combat state.
- **Active state**: `BONFIRE_REST` begins on server acceptance (character enters resting/sitting presentation); it ends immediately on movement input, combat action, map transfer, disconnect, explicit stand-up interaction, or leaving the radius. It does not persist across reconnect.
- **Passive Rest EXP**: Each completed 10s tick grants `rest_exp = 1,000` (×100 scale, ADR-0031), up to 180 ticks (30 active minutes = 180,000 EXP maximum) per `character_id + utc_date`. The cap is checked and committed with the tick operation; no tick is granted after the cap. This rest EXP is an ambient social accelerator outside the seven-channel progression portfolio (`../07_content/progression_route.md`) and is strictly excluded from baseline leveling pace calculations.
- **Linh Thú Bonding**: Each completed 300s active-rest interval grants the active beast `+1 bond_point`, up to 6 points per `character_id + beast_id + utc_date` from bonfire rest. This is additional to, and uses a separate counter from, the food cap in `spirit_beasts.md`.
- **Kindling**: A character not in combat may consume one `item.material.cui_lua_trai` at an inactive bonfire. This activates it for 30 minutes; each successful kindling during activation extends expiry by 30 minutes, capped at 120 minutes after server time. The operation is idempotent; no item is consumed if the cap would prevent an extension.
- **Village Wine (Rượu Nếp)**: Consuming `item.consumable.ruou_nep` while in `BONFIRE_REST` grants `buff.ruou_nep_am_long` (+5% ATTACK) for 30 minutes. It **begins on consumption**, remains through map transfer, and does not stack or refresh: a use while the buff is active is rejected without consumption.
## Open Sparring Ring (Lôi Đài Tỷ Thí Tự Do)
Full specification (including level gate and consumables ban) is canonical in `../03_systems/pvp.md`.


## Folk Fishing (Câu Cá Dân Gian)
Designated water access points (riverbanks, piers, village ponds) feature interactive fishing spots under ADR-0024:
- Requires a Bamboo Fishing Rod (`item.tool.can_cau_tre`) and Earthworm Bait (`item.consumable.moi_cau`).
- Concrete `fishing_spot.*` IDs and `has_water` map flags are owned by `../07_content/world_route_catalog.md`. A map with `has_water = false` has no spots and does not receive Di Tích rare-fish weight.
- `IDLE -> CASTING -> HOOK_WINDOW -> RESOLVING -> IDLE`. Cast and hook intents are `C2S_INTERACT` (103) with `interact_kind = CAST` or `HOOK` (`../05_network/messages.md`). `CASTING` consumes one bait only after rod, range, capacity, and the 50-success cap validate. `HOOK_WINDOW` starts 2s after cast and accepts exactly one server-received `HOOK` intent in `[0.40s, 1.20s]` after window start; any other `interact_kind` is rejected. Otherwise it resolves as failure with no catch and the bait remains consumed.
- On a valid hook intent, the server rolls the spot catch table exactly once using `fishing.<character_id>.<utc_date>.<cast_sequence>`; the client only presents the timing UI. Legal catch IDs are `COMMON_CATCH ∪ RARE_CATCH ∪` IDs of the **active seasonal table** (at most one extra `SEASONAL_CATCH`):
  ```text
  COMMON_CATCH   = item.material.ca_bong | item.material.ca_ro_dong | item.material.ca_chep | item.material.tom_song
  RARE_CATCH     = item.material.ca_chep_hoa_rong
  SEASONAL_CATCH = IDs exclusive to the active seasonal table:
    season 0 item.material.ca_linh_giang
    season 1 item.material.ca_sam_u_minh
    season 2 item.material.ca_bong_den
    season 5 item.material.ca_suong_ho
    seasons 3 and 4 = none (featured region has no has_water maps)
  ```
  Default table `fishing.catch.default` (`../07_content/item_catalog.md`) is closed: only `COMMON_CATCH ∪ RARE_CATCH`. A default-table row with any other item_id fails content activation.
  While the featured region has `has_water=true` fishing spots, those spots use `fishing.catch.season.<season_number mod 6>`; otherwise `fishing.catch.default`. Seasons 3 and 4 have no seasonal catch table. At most one seasonal table is active. Seasonal weights are owned by `item_catalog.md` (each table sums to 10000).
- Di Tích rare-fish buff (`bosses.md`): if the map `has_water = true` and `buff.di_tich.*` is active, `item.material.ca_chep_hoa_rong` weight is `100 -> 110` bp and the largest COMMON weight is reduced by `10` bp so the table still sums to `10000`.
- A successful catch increments `character_id + utc_date` atomically with reward settlement; maximum is 50. Full inventory uses Reward Claims. Disconnect during `CASTING` or `HOOK_WINDOW` resolves as failure; retrying the same operation returns that result and cannot reroll or refund bait. Each successful `FISH_CAUGHT` is 1 LIFE_SKILL action granting character EXP for the current act (`../07_content/progression_route.md`): I 6417, II 8283, III 6332, IV 7047, V 9448, VI 10494.
- **Rare catch peak**: settling `item_id ∈ RARE_CATCH` is one `PHAT_HIEN` opportunity (`../00_context/vision.md`). If that settlement also promotes Atlas Seen for `atlas.page.co_vat.ca_chep_hoa_rong`, emit **one** peak (`source=FISH_RARE`), not two. Presentation starts only after the authoritative settlement: 1200ms carp-dragon splash, `loc.peak.phat_hien.rare_fish`, atlas ping when Seen is new. Spectators in the same `map_instance_id` within 15m see the splash on the catcher. Presentation does not pause simulation, grant power, or reroll.

## Village Hearth Cooking (Bếp Lửa Làng Quê)
Safe Anchors feature a communal cooking hearth (`cooking_hearth.<map_id>`) under ADR-0024:
- Combines `COMMON_CATCH` fish and regional herbs into Linh Thú delicacies (*Cá Bống Kho Tộ*, *Cá Chép Nướng Mộc*, *Tôm Nướng Than*) and Village Wine (*Rượu Nếp Làng*). `RARE_CATCH` is never a cooking input.
- Guaranteed recipes without RNG failure or crafting skill levels.
- Serves as the primary source of food for raising Linh Thú affection (`bond_points`) and kindling village bonfires.
- Recipe inputs/outputs and bond values are concrete content data in `item_catalog.md`; cooking is an atomic guaranteed craft under `../03_systems/crafting.md`, rejected in combat, and uses one stable operation ID.

## Open World PvP
Open-world PK is not enabled.

Player-versus-player damage is valid only inside PvP content defined by `../03_systems/pvp.md` or instanced Guild War defined by `../03_systems/guild_war.md`.

## Day and Night
The world has a cosmetic day/night cycle:

```text
WORLD_DAY_DURATION = 120 real minutes
DAY = 80 minutes
NIGHT = 40 minutes
```
Time-of-day alters presentation without hidden stat inflation:
- **Day (80 min)**: Daylight palette, rural market audio, normal bird calls.
- **Night (40 min)**: Safe Anchors illuminate communal lanterns (`den_long_dinh_lang`); ambient audio transitions to evening crickets, cicadas, and distant night-watch gongs (`tieng_mo_dem`); adventure fields reveal drifting will-o'-the-wisp motes and activate night-only rare spawns (`map_spawn_catalog.md`).

Presentation lighting: the map's Global Light2D interpolates between the authored day and night colour/intensity over a 5-minute dusk/dawn transition; lanterns and bonfires use point Light2D at night (`../04_architecture/client.md`). Night must keep actors and telegraphs readable (`../07_content/presentation_asset_manifest.md` §3.3).

Time-of-day must not apply hidden global combat multipliers.
## Spirit Surge
The recurring open-world event is `SPIRIT_SURGE`.

Cadence:

```text
starts at every whole UTC hour
duration = 15 minutes
```

Each surge activates **three concurrent eligible outdoor regions**, each with its own field and element selection. Region and field assignment rotate deterministically from server UTC time (see `../07_content/world_event_catalog.md`). Each region is active approximately 50% of all hours, so the 12% EXP channel (`WORLD_EVENT`) is reachable without forcing players to chase a specific region.

Elements in use per surge:

```text
KIM | MOC | THUY | HOA | THO
```

Effects in each active region:
- up to 2 temporary surge groups are added to the selected field; the 54 persistent groups are unchanged (`../07_content/world_event_catalog.md`)
- one elite event chain at a time per selected map channel
- event monsters have visibly telegraphed elemental mechanics
- eligible kills and objectives may award configured surge materials
- completing the event chain awards one first-completion bonus per character per UTC day

The event is optional. Baseline farming and character progression remain available when no surge is active.

## Event Participation
A character participates only through authoritative contribution events such as:
- valid damage
- healing/protection credited to eligible participants
- objective capture/interact
- configured support action

Being present in the map is insufficient by itself.

Reward thresholds are content data and must prevent trivial one-hit tagging.

## Event Scaling
Open-world event enemies may scale encounter durability from eligible nearby participant count.

Scaling may modify:
- max HP
- stagger/poise threshold
- spawn count

It must not scale reward count linearly per participant.

## Rare World Encounters
A map may define hidden or conditional encounters based on explicit server-owned conditions such as:
- time of day
- Spirit Surge element
- quest state
- previous local event completion

These encounters must be discoverable through environmental clues. They must not require external real-money purchases or random paid keys.

## Map Persistence
Persistent:
- character map/checkpoint when required by character save rules
- quest/world progression explicitly marked persistent
- globally scheduled event state required for restart recovery

Runtime-only unless another spec says otherwise:
- normal monster instances
- temporary ground effects
- local aggro
- ordinary dropped visual pickups

## Restart Recovery
After server restart:
- characters resume from authoritative persisted position/checkpoint rules
- scheduled world events are reconstructed from server time, not duplicated from stale timers
- completed one-time rewards remain completed
- expired runtime monster/event entities are not recreated blindly
- **Di Tích world-consequence state persists across restart** and is reloaded from durable storage; relic markers and visible anchor state follow `../02_world/bosses.md`

## Fast Travel
There is no unrestricted map teleport menu.

Travel uses:
- map portals
- unlocked checkpoints where a content/NPC service explicitly allows travel
- dungeon/PvP entry flows

Fast travel is rejected while `in_combat`.

## In-Combat Canonical Rule
The canonical definition of:

```text
in_combat
```

belongs in `../01_gameplay/combat.md`.

For normal world/system gating, the current launch rule therefore clears after:

```text
6 seconds
```

without a hostile combat event, provided no unresolved hostile action/channel or content-specific combat lock remains.

World systems must not maintain a second independent combat-lock timer.

This shared flag gates equipment/loadout changes, Soul Contract mutation, direct trade, channel change, normal fast travel, respec, and other systems that explicitly reference `in_combat`.

PvP/Guild War content may keep combat locked for a whole round/match where their own spec requires it.

## Anti-Friction Rules
- No stamina/energy gate for normal world play.
- No mandatory daily login streak for permanent power.
- First-clear rewards may accelerate progress but repeat play still gives baseline rewards.
- World events should create reasons to meet other players, not punish players who miss a specific hour.

## Authority
The server owns:
- map-instance assignment
- channel capacity
- world time
- event schedule/state
- event participation
- world rewards
- safe-zone state
- combat gating

Clients render state and send intent only.

## Invariants
```text
map_id != map_instance_id
open-world PK = disabled
world event presence != automatic reward eligibility
channel transfer while in_combat = rejected
in_combat source of truth = combat.md
normal gameplay progression is not energy-gated
server time is authoritative
channels_per_map = 30
max_players_per_channel = 18 (player-initiated); forced placement hard cap = 22 (ADR-0061)
spirit_surge active regions per hour = 3 (region coverage ~50%)
spirit_surge selection = deterministic from server UTC time
Di_Tich world-consequence state = persistent across restart
fishing catch IDs = COMMON_CATCH ∪ RARE_CATCH ∪ active seasonal table IDs (at most one extra SEASONAL_CATCH); default table remains closed
RARE_CATCH settlement = one PHAT_HIEN source=FISH_RARE
open sparring ring spec owner = pvp.md
```
