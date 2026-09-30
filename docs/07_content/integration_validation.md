# Launch Content Integration Validation
status: LOCKED

## Scope
Cross-catalog validation required before a content revision may activate. Domain catalogs own their mechanics; this file owns only reference/count/integration checks between them.

Validation is deterministic and runs after finite authoring expansions and shorthand compilation but before atomic activation from `../06_data/config.md`.

# Identity / Syntax
Reject when:
- any persistent/runtime content ID contains non-ASCII characters or uppercase letters where the namespace requires lowercase IDs,
- duplicate IDs exist after deterministic expansion,
- a display/localization string is used as identity,
- an expanded generated ID collides with an explicitly authored ID.

# Launch Count Assertions
```text
classes = 5
upgradeable class skills = 60
basic attacks = 20
active skills = 25
passive skills = 15
world regions = 6
adventure field maps = 18
safe/social anchors = 6
normal-world checkpoints = 6
normal-world first-discovery EXP slots = 24
authored entry portals = 52
NORMAL monsters = 46
ELITE monsters = 12
season_0 variant NORMAL = 6
persistent field spawn groups = 54
NORMAL dungeons = 5
major progression first-clear EXP slots = 6
major bosses = 8
Souls = 25
spirit_beasts = 10
beast_equipment = 18
Meridian resonances = 15
Formations = 12
equipment sets = 12
set equipment item IDs = 168
bonus progression book types = 2 (item.book.potential, item.book.skill) — 12 each by Lv60
MAIN quests = 24
SIDE quests = 12
Daily templates = 12 standard + 1 mystery_meta; board = 6
Spirit Surge recurring event definitions = 1
currencies = 3
atlas pages launch = 104 (58 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat)
atlas pages seasonal first-cycle = 60 (10 × seasons 0..5)
cosmetics TOTAL_STABLE = 294
  TITLE_PLAY=127 PROFILE_FRAME_PLAY=9 APPEARANCE_PLAY=4 GUILD=5 PLAY_PLUS_GUILD=145
  COMMON_SINKS=20 SPECIAL_CURRENCY_SINKS=20 SEASONAL_ATLAS_TITLES=60 SEASON_FREE=18 SEASON_PAID=18 IAP_STORE_IDS=13
```

Any count change requires updating its owning catalog/budget intentionally; silent drift fails activation.

Atlas reward validation rejects a character-scoped Atlas tier whose `currency.special` delivery uses a Reward Claim. Its immutable character-tier entitlement key must settle exactly one **character-scoped** special credit; the authored launch maximum (`520` per character from atlas, `540` total including 20 base PvE sources) must remain below the character cap `1,000,000` (ADR-0029).

# Presentation Scale and Spatial Geometry

Compile and activation must enforce ADR-0046 across catalogs and exported geometry:

```text
reference viewport = 1280x720
ART_PIXELS_PER_METER = 50
reference world view = 25.6m x 14.4m
normal-world maps = 24 exact bounds rows, each width 2.0..5.0 screens
dungeons = 5 exact bounds rows
finale = instance.finale.than_trung exact bounds row
competitive spaces = map.pvp.duel_court, map.pvp.five_element_arena,
                     map.guild_war.five_seal_conflict
```

Reject when:
- any active monster/boss resolves to zero or multiple `size_profile` values,
- any sprite profile PPU/pivot/cell/silhouette does not match `presentation_asset_manifest.md`,
- a playable `space_id` is missing bounds, `layout_profile`, Addressable scene key, or geometry export,
- catalog bounds and `.geom.json` bounds differ after millimetre quantization,
- normal-world width is outside `2.0..5.0` screens or equals the viewport because an agent treated `1280x720` as map size,
- two normal-world maps reuse either the same width-height span pair or the same `layout_profile`,
- exported topology misses a required main route, tier, branch, loop, stage area, objective or anchor,
- a logical spawn/portal/objective/boss/checkpoint/camera region lies outside bounds or on no legal `CHARACTER` path,
- ranked PvP or Guild War geometry fails its `0.001m` mirror-parity rule.

Reference pixel extent validates coverage only; it must not require a monolithic bitmap. Tilemap/prop/parallax composition is valid when its scene and exported authoritative geometry pass.

# Skill Graph
The canonical runtime shape is owned by `../01_gameplay/skills.md`; the concrete mapping, scaling, timing, and geometry are owned by `class_skill_catalog.md`. The data-contract decision is ADR-0005.

Launch assertions per class:
```text
basic attacks = 4
active skills = 5
passive skills = 3
SIGNATURE-tagged active skills = 1
```

For each basic/active skill:
- `execution_type` is a canonical enum,
- `targeting_mode` is a canonical enum,
- `skill_tags[]` contains only canonical tags,
- air profile expands to explicit grounded/jumping/falling booleans,
- `startup_ms`, `active_ms`, and `recovery_ms` resolve to non-negative integers,
- `timing_speed_stat` is exactly `NONE|ATTACK_SPEED|CAST_SPEED`,
- one canonical typed geometry variant resolves with valid positive parameters,
- the complete launch set contains exactly 45 primary geometry rows (20 basics, 25 actives),
- the geometry fits its role band from `../01_gameplay/skills.md`,
- projectile envelope is `max_range_m + hit_radius_m <= 8.8m`,
- positioned-circle outer reach is `cast_range_m + radius_m <= 11.0m`,
- closest-hurtbox and shape/hurtbox intersection semantics use the ADR-0046 collider profile rather than pivot, sprite, alpha, or client-reported distance,
- the authoritative origin is exactly `caster_anchor_y + 0.9m`; no animation socket may override it,
- every secondary spatial effect resolves from `secondary_geometries[]` with origin, shape/distance, collision rule, stable selection order, and target-cap interaction,
- `MOVEMENT` tag implies an authored authoritative caster displacement and launch movement behavior `FORCED`,
- `PROJECTILE` tag resolves a server-owned projectile path/effect plus projectile geometry,
- `HEAL`, `SHIELD`, `STATUS_APPLY`, and `DISPLACEMENT` tags each have a matching effect result,
- every forced-position or canonical `AIRBORNE` result has `DISPLACEMENT`, while a skill without either cannot carry it,
- `BASIC_ATTACK` appears only on the twenty basic attacks,
- passive skills have stable trigger/effect definitions but no execution/targeting/action-geometry fields.

Launch cost/cooldown timing resolves deterministically from the canonical defaults:
```text
basic attack cost = 0
basic attack cost_timing = ON_START
ACTIVE MP skill cost_timing = ON_START
cooldown_start = ON_START
```
No launch class skill owns an implicit interruption refund.

Timing validation also asserts:
- ATTACK_SPEED/CAST_SPEED changes startup/recovery only,
- ACTIVE window does not shrink from those speed stats,
- projectile range/speed, forced-movement distance/duration, zone duration, and status duration do not inherit timing-speed scaling,
- no launch ACTIVE startup exceeds `700ms`,
- basic base action time remains inside the authored `500..1000ms` guardrail.

Skill-level compilation asserts:
```text
BASIC damage_scale(S)   = 1 + 0.040 * (S-1)   for S=1..12
ACTIVE damage_scale(S)  = 1 + 0.035 * (S-1)   for S=1..12
ACTIVE support_scale(S) = 1 + 0.025 * (S-1)   for S=1..12
ACTIVE cooldown(S)      = base_cooldown * (1 - 0.030 * (S-1)) for S=1..12
```
plus the basic attack cooldown and status proc scaling tables, and all fifteen passive scaling rows from `class_skill_catalog.md`.

Reject:
- an upgradeable skill level that changes no gameplay number,
- a hidden global skill-level damage/support multiplier outside the catalog,
- a skill-level upgrade that adds a mechanic/tag/targeting/execution type,
- geometry inferred from animation/VFX instead of data,
- `MOVE_CONTACT_LINE` without a swept contact effect or `BARRIER_POSITION` without valid grounded blocking geometry,
- `skill.hoa.active.boc_bo` producing a persistent ember-trail zone or any second target/damage/status resolution.

Cross-system skill selectors in equipment, Souls, Meridian, and Formation must use stable tags such as:
```text
DAMAGING
AREA
PROJECTILE
MOVEMENT
HEAL
SHIELD
STATUS_APPLY
DISPLACEMENT
PENETRATE
```
Reject selectors based on prose/localized categories such as `movement-type`, `area-type`, display name, animation name, or the legacy human-readable `type:` line in `class_skill_catalog.md`.

Reject a `PENETRATE` damage component unless `defense_penetration_ratio` is present and in `0.00..0.50`; reject that field on a component without `PENETRATE`.

For every launch basic reference to BLEED, BURN, POISON, or AREA_SPLASH, reject prose-only effect data. It must resolve to a `class_skill_catalog.md` canonical effect template with stable `effect_id`; DOT templates require element, total/per-tick coefficient, snapshot rule, duration, tick interval, reapply/stack rule, and dispellable flag. AREA_SPLASH requires radius, coefficient, primary-target rule, deterministic selection order, and target-cap interaction. `effect.basic.burn_true_3s` is the only launch basic BURN template with `dispellable=false`.

# World Route Graph
For every map row in `world_route_catalog.md`:
- map exists in `encounter_catalog.md`,
- map type is supported by `../02_world/maps_zones.md`,
- recommended level range is inside the owning progression band,
- canonical `spawn.entry.*` exists in the map asset,
- exactly one character first-discovery EXP value exists,
- TOWN safe anchors have no hostile persistent spawn group.

For every generated or explicit portal:
- source exists,
- destination map/dungeon/encounter exists,
- destination entry/return spawn resolves,
- stable ID is ASCII lowercase,
- normal transfer never accepts client coordinates.

Route invariants:
```text
6 checkpoint IDs -> 6 safe anchors
18 undirected intra-region edges -> 36 directional portals
5 undirected cross-region edges -> 10 directional portals
5 dungeon-entry portals
1 finale-entry portal
36 + 10 + 5 + 1 = 52 authored entry portals
```

First-discovery assertions per act:
```text
safe anchor = 0.2% owning act EXP
field 1     = 0.2%
field 2     = 0.2%
field 3     = 0.2%
act discovery total = 0.8%
```
All 24 discovery keys use:
```text
reward.discovery.<map_id>.<character_id>
```
and commit only after successful authoritative first entry.

Reject when:
- an intra-region forward edge lacks its reverse edge,
- a cross-region forward edge lacks the matching persistent story flag + level gate,
- a backward cross-region route requires replaying completed story content instead of destination discovery,
- a MAIN dungeon/finale portal is time-gated or RNG-gated,
- a standalone PUBLIC boss gates normal region traversal,
- normal portal arrival overlaps a hostile spawn safety envelope,
- launch starter entry differs from `spawn.entry.lang_da.dinh_lang`,
- travel can create an undiscovered-map reward,
- discovery reward is account-scoped or repeatable.

# Progression Graph
Act totals must match `exp_required(L) = 10000 * L * L` (×100 scale; Act VI capped at L < 60):
```text
Act I       3,850,000
Act II     24,850,000
Act III    65,850,000
Act IV    126,850,000
Act V     207,850,000
Act VI    272,850,000
TOTAL     702,100,000
```

For every act the seven-channel EXP source portfolio is exactly:
```text
FIELD_COMBAT    40%    DUNGEON_REPEAT  18%    WORLD_EVENT     12%
BOUNTY_REPEAT   12%    ELITE_BOSS       8%    LIFE_SKILL       5%
STORY_ONCE       5%
─────────────────────────────────────────────────────────────────
TOTAL          100%
```

`STORY_ONCE` sub-split (percentages of act budget):
```text
MAIN quests                      3.0%
SIDE quests (both)               0.8%   (0.4% each)
first safe-anchor discovery      0.2%
3 first field-map discoveries    0.6%   (0.2% each)
first major dungeon/finale clear 0.4%
─────────────────────────────────────
STORY_ONCE total                 5.0%
```

The six first-progression-clear slots resolve exactly (0.4% of each act's budget):
```text
dungeon.dinh_lang_bo_hoang  =    15,400 EXP
dungeon.mieu_ba_trong_rung  =    99,400 EXP
dungeon.xom_chim            =   263,400 EXP
dungeon.hang_ma_tranh       =   507,400 EXP
dungeon.den_tran            =   831,400 EXP
boss.than_trung             = 1,091,400 EXP
```
with character-scoped idempotency:
```text
reward.first_progression_clear.<source_id>.<character_id>
```

The progression balance audit in `balance_validation.md` validates against the two channel-deviation reject rules.

Reject when:
- any channel's EXP computed from independently authored per-unit values (authored monster/dungeon/event EXP from owning catalogs × unit frequency × channel hours, per "Channel EXP Rate References" in `progression_route.md`) deviates from the target channel EXP by more than 15% in any act (the superseded 2-percentage-point channel-share check is not a valid substitute: any value derived as `act_total × channel_percentage` satisfies it trivially and detects only authoring typos, not wrong EXP targets),
- `| derived_act_hours - target_act_hours | / target_act_hours > 0.15` for any act,
- MAIN quest EXP no longer equals 3.0% of act budget (within STORY_ONCE) without an intentional budget revision,
- discovery no longer equals 0.2% safe + 0.2% per field (0.8% total) per act,
- first major progression clear no longer equals 0.4% of act budget,
- Daily, Ranked PvP, or Guild War income/EXP is required by the baseline first-character progression simulation,
- Level-60 ignored EXP is silently converted into another currency/progression value.

# Monster / Spawn / Drop Graph
For every `monster_id` in `monster_catalog.md`:
1. exactly one matching `drop_table_id` exists,
2. every spawn-pool reference expands to a known monster,
3. NORMAL monsters use NORMAL reward tables and ELITEs use ELITE reward tables,
4. fixed level lies within its authored progression-region band,
5. generated `attacks[]` use unique stable IDs.

For the canonical launch spawn catalog `map_spawn_catalog.md`:
- exactly `18` FIELD maps are covered,
- every FIELD has exactly `2 NORMAL + 1 ELITE` persistent groups,
- total persistent groups = `54`,
- every spawn-group ID is unique,
- no second launch spawn catalog may define competing persistent groups.

For every persistent spawn group:
- map exists and is FIELD,
- logical anchor resolves in the map asset,
- selector resolves to at least one valid monster,
- NORMAL group resolves only NORMAL monsters,
- ELITE group resolves only ELITE monsters,
- `max_alive = 20` for launch NORMAL groups and `2` for launch ELITE groups,
- any INSTANCED space granting kill EXP, a dungeon completion granting EXP other than `dungeon_repeat_exp(min(character_act, dungeon_tier_act + 1))`, an ELITE_BOSS PUBLIC-boss share outside Acts III/VI, a Spirit Surge grant not using `min(character_act, region_act + 1)`, or a seasonal Atlas tier granting `currency.special`,
- launch NORMAL respawn resolves to `10..14s`,
- launch ELITE respawn resolves to `35..60s`,
- launch NIGHT_RARE respawn resolves to `240..360s` (authored NIGHT_RARE band; monsters in this band appear only during server night cycle),
- population and expanded runtime fields satisfy `spawning.md`.

Safe/social maps must resolve to zero hostile persistent spawn groups.

# Boss Graph
For every `boss_id`:
- boss exists in both narrative encounter roster and `boss_catalog.md`,
- mode/scaling pair is valid,
- one repeat `drop_table_id` exists,
- Boss Soul first-clear source, if any, points back to that exact boss,
- PUBLIC placement resolves to a valid FIELD boss anchor,
- INSTANCED boss is owned by a dungeon/finale definition,
- boss-created adds default to no reward table,
- base HP equals `floor(10000 + 300*L + 16*L*L)`,
- ATTACK/DEFENSE/EXP resolve from the owning boss formulas.

Exactly two launch major bosses are PUBLIC and six are INSTANCED.

Level-60 launch boss baseline must resolve:
```text
MAX_HP  = 85600
ATTACK  = 335
DEFENSE = 170
```

Combat-regression gates are owned by `balance_validation.md` and must pass during release validation:
```text
NORMAL same-level basic-only TTK = 2..6s
ELITE same-level basic-only TTK  = 8.5..24s
boss solo basic-only TTK         = 55..130s
boss five-player basic-only TTK  = 35..85s
normal readable heavy hit        = 6..18% synthetic reference MAX_HP
mandatory-boss class basic-TTK max/min spread <= 2.20
```
A numeric content revision outside these gates requires intentional retuning or an explicit owning-guardrail revision; activation may not silently accept the drift.

# Dungeon Graph
For each of five launch dungeons:
- all mandatory `stage_id` values are unique,
- final boss exists and is INSTANCED,
- NORMAL repeat and FIRST_CLEAR tables exist,
- first progression clear EXP matches the exact Progression Graph value,
- Level-60 `.endgame` reward table exists,
- MAIN quest uses a `DUNGEON` objective only for mandatory dungeon completion and does not append the same run's final boss sequentially,
- owning dungeon-entry portal exists in `world_route_catalog.md`,
- party size remains `1..5` and difficulty enum remains NORMAL.

For every `ENDGAME_L60` tagged run:
- minimum level is 60,
- trash stats resolve from the explicit fixed Level-60 profile in `dungeon_catalog.md`, not player gear,
- final boss baseline is `85600 HP / 335 ATTACK / 170 DEFENSE`,
- endgame boss HP multiplier is `1.75`, producing `149800` one-member HP before PARTY_DEFAULT,
- exactly one named mechanic remix exists for that dungeon,
- telegraph floors remain at least their baseline values,
- repeat settlement is exactly one owning `.endgame` combined table,
- normal dungeon repeat and baseline final-boss repeat tables do not additionally settle,
- unearned independent lifetime first-clear operations may still settle once,
- no key/stamina/weekly entry gate exists.

An ENDGAME_L60 variant that changes only numbers and has no named mechanic remix fails validation.

# Quest Graph
Reject:
- missing prerequisite target or prerequisite cycle,
- MAIN quest that requires a standalone PUBLIC boss generation without an always-available story instance,
- target map/monster/elite/dungeon/boss absent from owning catalog,
- reward item/material absent from owning item/equipment catalog,
- direct permanent skill/potential reward,
- rare RNG collection requirement with no deterministic fallback,
- Daily template targeting inaccessible content,
- more than two generated Daily choices from one primary objective family, except the single 3rd template placed by the slot-6 uncapped fallback (`quest_catalog.md` § Board Generation; ADR-0062).

Every act-complete flag used as a cross-region gate must be produced by exactly one MAIN quest and consumed by the matching forward route gate.

Every SIDE quest must compile to the exact owning-act preview from `quest_catalog.md`:
```text
T1 EXP 15400   common 300  material 2  bound 10
T2 EXP 99400   common 700  material 2  bound 20
T3 EXP 263400  common 1400 material 2  bound 30
T4 EXP 507400  common 2500 material 2  bound 40
T5 EXP 831400  common 4000 material 2  bound 50
T6 EXP 1091400 common 6000 material 2  bound 60
```
The common/EXP/material values are quest pacing data. The bound value must equal the `economy_catalog.md` rule:
```text
bound = 10 * owning_tier
```
Each SIDE settlement uses independent stable slots so one overflow/cap condition cannot duplicate another slot.

Daily payouts are optional accelerators and must not be assumed by baseline economy-affordability or leveling validation.

# Equipment / Crafting Graph
For every generated `item.eq.<tier>.<set_key>.<slot>`:
- set/tier/slot exists,
- exactly one guaranteed normal crafting recipe exists,
- regional crafting material exists,
- set element layout resolves one element,
- enhancement base-cost tier exists,
- no generated item occupies two canonical slots.

For every 2/4/6 set effect:
- referenced stat/effect/status exists,
- decimals are typed as FLAT_ADD/PERCENT_ADD/ratio/multiplier/etc under `stats.md`,
- shield/heal/resource percentages use typed effect components,
- skill selectors use canonical `skill_tags[]`,
- Support Signature does not depend on enhancement, rarity, rolled stat magnitude, or Soul level.

Equipment recipe validation asserts canonical slot-weight sum:
```text
14 canonical slot weights = 47
```
and verifies the full-set material/common reference totals in `crafting_catalog.md` from that sum.

Enhancement validation uses the explicit attempt-cost tables from `../03_systems/crafting.md` and base costs from `equipment_catalog.md`. It simulates the Markov transition process including success, downgrade, floor, and high-end no-downgrade failure.

With Insurance applied for +8..+11 and the canonical per-target soft-pity state machine for +13..+16, expected cumulative multiplier references from +0 under ADR-0021/0028 must remain approximately:
```text
target +6  -> material 27.26, common-base 56.28
target +8  -> material 192.02, common-base 488.53
target +10 -> material 275.35, common-base 798.53
target +12 -> material 525.35, common-base 1,886.03
target +16 -> material 785,781.85, common-base 3,979,515.15
```
The validator solves the reachable `(level, pity_13, pity_14, pity_15, pity_16)` Markov states in full precision and compares displayed values after two-decimal rounding; drift greater than `0.005` multiplier units rejects activation.
The reference is the expectation over the full pity state machine, not an artificial permanently-max-pity rate and not a no-pity baseline. Tooling must use the persisted `item_instance_id + target_level` failure counters, their reset-on-success rule, and floor-bounded downgrade transitions; it must not publish an unsupported fixed-percent expected-cost reduction.
Small numerical tolerance is allowed only for floating-point/tooling representation; authored transition/cost drift requires an intentional review.

Every tier must use:
```text
base_enhancement_material_units = 1
```
and the tier common bases from `equipment_catalog.md`.

Reject a release candidate whose expected-cost simulation makes the tier's intended band in `crafting.md` materially unreachable from normal authored reward sources without Auction dependence or farming far beyond that progression-band playtime. Do not validate cost by multiplying only successful per-attempt prices.

Baseline affordability simulation must exclude optional Daily, ranked PvP, and Guild War bonus income. These systems may accelerate progression but cannot be required to make deterministic gear/enhancement targets viable.

# Economy Graph
Canonical launch numeric combat currency bands belong only to `economy_catalog.md`.

Validation asserts:
```text
currency.common  -> CHARACTER + transferable
currency.bound   -> CHARACTER + non-transferable
currency.special -> CHARACTER + non-transferable + cosmetic-only at launch
```

## Common
Reject when:
- `drop_tables.md` contains a second conflicting numeric NORMAL/ELITE/boss band table,
- deterministic equipment recipes use common bases different from `crafting_catalog.md`,
- guild creation cost differs from `10,000 common`,
- post-Level-20 respec differs from `25 * level^2 common`,
- Auction/trade allows bound or special currency,
- an earned common-cap failure blocks an independently earned item/Soul/material sibling slot,
- an NPC offer has `sell_back >= buy_price`,
- a future NPC-sellable crafted output creates deterministic positive common from buy/craft-and-sell without consuming a scarcer finite input.

## Bound
Required launch sources:
```text
SIDE completion -> 10 * tier
NORMAL dungeon completion -> 5/10/15/20/25 for T1..T5
ENDGAME_L60 dungeon completion -> 30
first 5 reward-eligible ranked matches/day -> 20 each
first 3 reward-eligible Guild Wars/week -> 50 each
```

Required launch sinks:
```text
offer.bound.bua_may.so_cap        = 25
offer.bound.bua_may.trung_cap     = 50
offer.bound.bua_may.cao_cap       = 120
offer.bound.bua_may.sieu_cap      = 300
offer.bound.bua_giu_bac.so_cap    = 60
offer.bound.bua_giu_bac.trung_cap = 120
offer.bound.bua_giu_bac.cao_cap   = 300
```

Competitive scope assertions:
- Ranked daily cap is one character-wide counter across all ranked PvP modes, not five per mode.
- Guild War personal bound cap is one character-wide Monday-UTC weekly counter across guild membership changes.
- Guild War per-character bound cap is independent from the separate per-guild Guild EXP/contribution cap.
- `VOID`, AFK, abandoned, or otherwise reward-ineligible matches consume no personal bound slot.
- win/loss/draw/eligible surrender does not change the configured bound amount.

Bound-purchase anti-laundering assertions:
```text
currency.bound item purchase -> source binding CHARACTER_BOUND ON_ACQUIRE
```
Reject when:
- a bound utility offer becomes the only acquisition path for its item,
- bound directly buys equipment/Soul/skill point/potential point/instant enhancement/random loot box,
- a player can trade/auction bound currency,
- an item bought with bound currency remains UNBOUND or becomes ACCOUNT_BOUND,
- a bound-purchased output can enter trade, Auction, Guild Storage, or account storage,
- stack/split/merge/container movement can loosen its CHARACTER_BOUND state,
- PvP/Guild War win/loss changes the bound completion amount,
- Guild War introduces a mode-specific cosmetic-token currency instead of direct cosmetic entitlements.

## Special
Character-scoped one-time PvE sources before atlas must resolve exactly:
```text
boss.thuong_luong character first clear      = 2
boss.ho_tinh_chin_duoi character first clear = 3
boss.than_trung character first clear         = 5
progression.story.main.complete character first = 10
base PvE total per character (pre-atlas) = 20
```
Atlas adds per-character special (1/2/2 per tier — T3 reduced to 2 per ADR-0042 to stay within sink surface, max 520) via `atlas.page.<id> + tier` idempotency. `currency.special` is character-scoped (ADR-0029). Base 20 is per character, not shared across alts.

The launch sinks are explicit alternative redemptions:
```text
cosmetic.appearance.ao_vai_hoa_van = 20 special OR 20 item.material.vai_hoa_van
cosmetic.frame.nui_thieng           = 20 special OR 30 item.material.vai_hoa_van
```
The character may spend its initial 20 special on at most one of these until another explicitly authored special source exists. Material routes remain deterministic non-power alternatives.

Reject when:
- special source repeats on the same character,
- special buys combat/progression/economy-efficiency power,
- same-account characters share special, Linh Đan, or inventory,
- special buys combat/progression/economy-efficiency power,
- one cosmetic redemption consumes both routes,
- duplicate entitlement consumes input again,
- the two launch special sinks use different special prices without intentional economy update,
- any automatic common/bound/special exchange exists.

# Soul Graph
For all 25 Souls:
- source monster/elite/boss exists,
- corresponding repeat/first-clear acquisition reference exists in `drop_tables.md`,
- rank matches source contract,
- BOSS Soul first eligible clear is guaranteed exactly once per character,
- no Soul is trade/auction enabled,
- trigger uses canonical event/status/skill-tag terminology.

Soul progression validation also asserts the launch `soul_exp_reward` loop from `../03_systems/soul_contracts.md`:
```text
NORMAL monster = 1
ELITE monster = 6
major boss = 20
NORMAL dungeon completion = 15
ENDGAME_L60 dungeon completion = 25
Spirit Surge completion = 10
```

Each eligible settlement applies the value equally to ACTIVE contracted Souls, never split; one source settlement may credit one `soul_instance_id` at most once. Soul EXP is not an inventory reward and never enters Reward Claims.

# Meridian / Formation Graph
Shared relation IDs are exactly:
```text
SINH_OUT
SINH_IN
KHAC_OUT
KHAC_IN
DONG_HE
```

## Meridian Reachability
Meridian validation uses the concrete BASIC element domain from `equipment_catalog.md`. Every launch tier exposes two element choices per BASIC slot, so compiler enumerates:
```text
2 choices per BASIC slot
8 slots
2^8 = 256 legal sequences
```

For each of the 15 launch resonances:
1. matcher primitive is one of `ELEMENT_COUNT|RELATION_CHAIN|RELATION_COUNT|RELATION_SUBSEQUENCE|FULL_RING`,
2. all element/relation/count predicates are valid,
3. at least one of the 256 legal sequences matches,
4. after coexistence-group priority + lexical tie-break, at least one legal sequence selects that `resonance_id` inside its group,
5. total selected meaningful Meridian effects never exceeds 3.

Reject:
- an 8-slot sequence matcher with the wrong length,
- relation chain/count threshold outside `0..8`,
- element count impossible for the concrete equipment domain,
- syntactically valid resonance with zero legal equipment witnesses,
- resonance that matches but is always shadowed inside its coexistence group,
- FULL_RING resonance that does not require all eight BASIC slots,
- reintroduction of the old unreachable launch assumptions such as `DONG_HE_CHAIN >= 3` or the retired explicit subsequences unless the equipment catalog changes and validation proves a legal selected witness.

Validation tooling stores at least one selected-group witness bitstring for every launch resonance.

## Formation Reachability
Formation validation uses the concrete ADVANCED element domain from `equipment_catalog.md`. Every launch tier exposes two element choices per ADVANCED slot, so compiler enumerates:
```text
2 choices per ADVANCED slot
6 slots
2^6 = 64 legal sequences
```

For each of the 12 launch Formations:
1. matcher primitive is one of `ELEMENT_COUNT_PATTERN|RELATION_COUNT_PATTERN|EXPLICIT_SEQUENCE`,
2. all element/relation predicates use valid IDs/counts,
3. at least one of the 64 legal same-tier sequences matches,
4. after actual priority + lexical tie-break selection, at least one legal sequence selects that `formation_id` as the winner.

Reject:
- >1 active Formation after selection,
- EXPLICIT_SEQUENCE with length != 6,
- matcher requiring an element count impossible for the equipment domain,
- syntactically valid Formation with zero legal equipment witnesses,
- Formation that matches some sequences but is always shadowed by higher-priority definitions,
- reintroduction of the old unreachable assumptions: six identical elements, literal full five-element cycle, or exact two-element alternation unless the equipment catalog is first changed so a legal witness exists.

Validation tooling stores one selected-winner witness sequence for every launch Formation.

# Atlas Graph
For all 104 atlas pages in `atlas_catalog.md`:
- page ID matches `atlas.page.<category>.<key>` lowercase contract,
- category is one of `quai_dam|hon_giam|di_tich|co_vat`,
- `quai_dam` 58 entries each resolve to a `monster.*` in `monster_catalog.md` (46 NORMAL + 12 ELITE — expanded to cover the full 58-monster roster per ADR-0042),
- `hon_giam` 25 entries each resolve to a `soul.*` in `soul_catalog.md`,
- `di_tich` 8 entries each resolve to a `boss.*` in `boss_catalog.md`,
- `co_vat` 13 entries each resolve to an `item.*` or system source in `item_catalog.md`/`world_rules.md`, including `item.material.ca_chep_hoa_rong`,
- `fishing.catch.default` weights sum to 10000, include `RARE_CATCH` weight 100, and contain only `COMMON_CATCH ∪ RARE_CATCH`,
- `fishing.catch.season.{0,1,2,5}` each sum to 10000 with extra SEASONAL_CATCH 100; no tables for seasons 3–4,
- every `cosmetic.title.atlas.*` and every seasonal T3 title `cosmetic.title.season.<n>.<key>` resolves in `cosmetic_catalog.md` (finite expansion),
- no duplicate atlas_page_id,
- atlas never grants combat stats, skill/potential points, gear, or bound efficiency.

Seasonal first-cycle (60 pages, seasons 0..5): every Source ID resolves to `monster.*` / `chest.hidden.season.*` / `item.material.*` or `DISH_COOKED` item / `relic.season.*`. Relics exist in `bosses.md`. Chests exist in `world_route_catalog.md`.

Atlas special rewards use the same idempotency as other special sources: `atlas.tier.<character_id>.<atlas_page_id>.<tier>`. Atlas is non-power and never gates MAIN.

# Bonus Book Graph
For `item.book.potential` and `item.book.skill` in `item_catalog.md`:
- exactly two book types exist, both CHARACTER_BOUND ON_ACQUIRE, stack 99,
- schedule resolves to 12 grants each by Lv60 per `../01_gameplay/progression.md` (25,30,35,40 = +1 each; 45,50,55,60 = +2 each),
- flags `progression.book.potential.<level>` and `progression.book.skill.<level>` are distinct and idempotent,
- consumption grants +10 potential / +1 skill via operation_id and respects 60% cap (potential) and 75/114 limit (skill),
- books never enter trade/Auction/Guild Storage/account_storage.

# Reward Graph
All reward item/Soul/equipment references must resolve before commit.

Drop/event rewards that declare independent stable slots must allow sibling slots to settle independently. In particular:
- capped `currency.common` cannot block an earned item/material/equipment/Soul slot,
- an undeliverable item slot may become its own Reward Claim without moving already-delivered siblings back to pending,
- high-frequency currency overflow uses safe aggregate claims rather than producing unbounded PENDING rows,
- aggregate currency contribution keys remain idempotent and append-only.

Reward random selection is committed before inventory insertion; Reward Claims preserve the selected result. A retry must not reroll:
- equipment slot,
- Soul roll,
- quantity,
- weighted choice.

`FIRST_CLEAR`, `DAILY_FIRST`, PUBLIC generation reward, discovery reward, progression-first-clear reward, quest completion, account-special grant, competitive bound grant, and cosmetic entitlement use distinct idempotency scopes.

Direct player trade is not a reward flow: a receiver common-currency cap failure must reject the entire trade and must not create a Reward Claim. Auction seller proceeds continue using their dedicated pending-proceeds escrow.

Quest rewards are owned by `quest_catalog.md`; `drop_tables.md` owns combat/dungeon/world-event loot. A duplicated `drop.quest.*` reward for an already-inline quest bundle is an error.

# World Event Graph
For Spirit Surge:
- **three regions** are active simultaneously each hour; selection is deterministic from UTC hour index (ADR-0035),
- a given region is active approximately 50% of hours across the rotation,
- safe/social maps cannot be selected,
- temporary group count <=2 and max_alive <=4,
- event attack telegraph >=0.80s,
- contribution threshold is required for completion reward,
- daily-first has no exclusive permanent power.

# New-Stat Integration Assertions (ADR-0037)
These assertions run alongside the existing cross-catalog checks.

## Resolution Stage
```text
ASSERT LIFESTEAL resolves at Global Effect Resolution Order stage 7 (ON_HIT/ON_HEAL/ON_STATUS TRIGGERS)
ASSERT REFLECT   resolves at Global Effect Resolution Order stage 7
ASSERT ABSORB    resolves at Global Effect Resolution Order stage 7
ASSERT HEAL_REDUCTION resolves at Global Effect Resolution Order stage 7
REJECT if any of the four stats fires at stages 1..6 (before PRIMARY_RESULT_COMMIT)
```

## Stat Caps
```text
ASSERT stats.md declares LIFESTEAL_CAP = 0.08
ASSERT stats.md declares REFLECT_CAP   = 0.15
ASSERT stats.md declares ABSORB_CAP    = 0.10
ASSERT stats.md declares HEAL_REDUCTION_CAP = 0.30
ASSERT pvp.md declares LIFESTEAL PvP cap  = 0.05
ASSERT pvp.md declares REFLECT PvP cap   = 0.08
ASSERT pvp.md declares ABSORB PvP cap    = 0.06
ASSERT pvp.md declares HEAL_REDUCTION PvP cap = 0.25
REJECT if any PvP cap exceeds the corresponding PvE cap
```

## Reflected Damage Tags
```text
ASSERT every reflected damage instance carries tags: NO_CRIT | NO_REFLECT | NO_LIFESTEAL | NO_PROC
REJECT if reflected damage can roll critical
REJECT if reflected damage can itself trigger REFLECT (recursion prevention)
REJECT if reflected damage can trigger LIFESTEAL on the original attacker
REJECT if reflected damage can trigger on-hit procs (Soul, skill, Formation effects)
```

## Authorised Sources Only
```text
REJECT if any content grants LIFESTEAL outside equipment secondary rolls, set bonuses,
         Soul effects, Spirit Meridian resonances, or Formation bonuses
REJECT if any content grants REFLECT outside the same sanctioned sources
REJECT if any content grants ABSORB outside the same sanctioned sources
REJECT if any content grants HEAL_REDUCTION outside the same sanctioned sources
REJECT if LIFESTEAL/REFLECT/ABSORB/HEAL_REDUCTION appears as a potential-derived stat
REJECT if LIFESTEAL/REFLECT/ABSORB/HEAL_REDUCTION appears on the Spirit Beast transferred-stat list
REJECT if any secondary roll in the pool adds a new roll slot (pool size must be unchanged)
```

## Interlock Assertions
```text
ASSERT HEAL_REDUCTION applies to LIFESTEAL heal results
ASSERT HEAL_REDUCTION does NOT apply to ABSORB shield creation
ASSERT ABSORB shield is reduced by target_shield_received_multiplier, not by HEAL_REDUCTION
ASSERT lifesteal heal floor <= LIFESTEAL_HPS_CAP (excess discarded, not banked)
ASSERT ABSORB uses the existing shield system: max(current_remaining, new_amount) reapplication
```

## Reflected Damage Reward Isolation
```text
ASSERT reflected damage grants zero EXP to the defending character
ASSERT reflected damage generates zero loot ownership for the defending character
ASSERT reflected damage generates zero threat attribution to the defending character
ASSERT a monster killed solely by reflected damage settles rewards to the defender
       under the normal contribution rules (not as a kill-credit bonus to the defender)
```

# Balance Simulation Gate
`balance_validation.md` is mandatory release-validation input, not advisory prose.

The simulator must derive values from active content revision data and emit at least:
```text
revision
class_id
tier/level
synthetic reference stats
NORMAL TTK
ELITE TTK
boss solo TTK
boss five-player TTK
Lv60 rotation TTK where applicable
heavy-hit HP ratio
progression remaining-gap ratio by act
```

It must not contain an independent hidden copy of gameplay formulas. Tests read/compile the canonical formulas/values or compare generated data against the locked references.

A failed hard gate rejects revision activation. Broad Lv60 rotation windows are regression-review gates and may be intentionally revised only with documented content balance changes.

# Cultural / Presentation Gate
Before art/content export:
- folklore-named creature/ritual/real-place inspiration has a source/reference note,
- game invention is distinguishable from documented motif,
- real sacred/historical site is not presented as a literal evil dungeon unless intentionally reviewed,
- visual references are recognizably Vietnamese rather than generic Chinese-xianxia, Japanese-yokai, or Western-fantasy substitutes,
- class-skill display/VFX language follows the Vietnamese-grounded presentation guardrail in `class_skill_catalog.md`,
- runtime IDs remain neutral machine identity; localized Vietnamese display text may use diacritics.

# Activation Result
If any required check fails:
```text
revision_activation = REJECTED
previous_validated_revision remains active
no partial catalog activation
```

Warnings may exist for balance telemetry recommendations, but missing IDs, invalid counts, schema conflict, unresolved ownership, route softlock, unreachable build pattern/progression/economy loop, semantic-tag ambiguity, transfer-value laundering, action-geometry ambiguity, progression-gap failure, combat-balance hard-gate failure, or idempotency ambiguity are errors.

# Invariants
```text
cross-catalog reference validation runs after expansion
all active/basic skills have canonical execution + targeting + tags + timing + geometry
skill upgrades have concrete deterministic numeric outcomes
class-skill MP cost/cooldown timing resolves to ON_START unless explicitly overridden
cross-system skill selectors use stable tags
normal-world first-discovery slots = 24
reference viewport = 1280x720 at 50 px/m; character silhouette <= 64x96px
normal-world map widths = 2.0..5.0 screens; bounds are not assumed fully walkable
24 normal-world maps use 24 distinct width-height span pairs and 24 distinct layout_profile values
all playable spaces resolve exact bounds + scene key + geometry export
all monsters/bosses resolve exactly one size_profile
seven-channel portfolio = 100% per act; channel share reject: per-unit cross-check deviation > 15% from canonical target (see progression_route.md Channel EXP Rate References); act-hours reject = 15%
STORY_ONCE = 5% per act (MAIN 3.0%, SIDE 0.8%, anchor 0.2%, fields 0.6%, first-clear 0.4%)
bonus books = 2 types, 12 each by 60, CHARACTER_BOUND, idempotent per level flag; total Lv60 skill points = 75
no dangling power/reward/reference route
no MAIN public-boss wait gate
no route progression RNG gate
Soul EXP has reachable launch sources
enhancement review uses expected failure-inclusive cost with canonical pity state transitions
all 15 Meridian resonances have legal selected-group witnesses
all 12 Formations have legal selected-winner witnesses
boss HP formula = 10000 + 300L + 16L^2
ENDGAME_L60 one-member boss HP = 149800 before PARTY_DEFAULT
atlas pages = 104, all sources resolve, non-power, fishing.catch.default legal
combat balance hard gates pass
common combat bands have one owner
new stats (LIFESTEAL/REFLECT/ABSORB/HEAL_REDUCTION) resolve at stage 7 only
new stat PvP caps exist in pvp.md and do not exceed PvE caps
reflected damage carries NO_CRIT|NO_REFLECT|NO_LIFESTEAL|NO_PROC
new stats sourced only from equipment/Soul/Meridian/Formation; no potential/beast transfer
ABSORB not affected by HEAL_REDUCTION; LIFESTEAL IS affected by HEAL_REDUCTION
reflected damage grants no EXP/loot/threat
baseline affordability excludes optional Daily/PvP/Guild-War bonus income
bound has PvE + competitive sources and non-exclusive sinks
bound currency cannot be laundered into tradable output
competitive bound caps are character-scoped across modes/guild changes
special base PvE = 20 per character, atlas adds per-character tiers on top (max 520 from atlas), two launch 20-special sinks remain
capped currency cannot block independent earned item slots
quest rewards have one owner
ENDGAME_L60 repeat reward settles once through combined table
content activation is all-or-nothing
validation never silently rewrites authored data
```
