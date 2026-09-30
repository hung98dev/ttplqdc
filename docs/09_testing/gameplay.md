# Gameplay Tests
status: LOCKED

## Scope
Mandatory deterministic unit/integration/regression tests for gameplay, world, systems, and launch-content contracts. Load/network/security-specific suites remain in their owning testing specs.

# Formula Tests
Automate exact-value vectors for:
- Level EXP cumulative curve (`exp_required(L) = 10000 * L^2`, `cumulative_exp_to_reach(L)`): exact boundary tests (`0` EXP = Lv1, `9,999` = Lv1, `10,000` = Lv2, `702,099,999` = Lv59, `702,100,000` = Lv60), single-grant multi-level jumps (e.g. `0 -> 50,000` advances Lv1 -> Lv3 with atomic grant of +2 skill points and +8 potential points), excess EXP discard above `702,100,000` at Lv60, and client level progress display mapping.
- Potential 60% per-stat cap and respec validation.
- Class level-growth stats for representative Levels `1,10,30,60`.
- Modifier order `BASE -> FLAT_ADD -> PERCENT_ADD -> FINAL_MULTIPLY -> CLAMP`.
- Full damage pipeline stage floors, dodge/critical order, elemental multiplier, defense/PENETRATE rounding (Kim-only `skill.kim.basic.vo_song_kiem` ratio 0.15; other basic_4 have neither the PENETRATE tag nor a ratio), damage-taken modifiers, minimum damage, and Just Guard before shield absorption.
- crit chance/damage caps.
- speed/CDR caps.
- monster stat/EXP deterministic expansion for NORMAL/ELITE rows using Channel EXP Rate References (not ADR-0031 `round(400+5L)`).
- boss HP `floor(10000 + 300*L + 16*L*L)`, boss ATTACK/DEFENSE/EXP formulas, and PUBLIC/PARTY scaling vectors.
- Level-60 boss baseline exactly `85600 HP / 335 ATTACK / 170 DEFENSE`.
- ENDGAME_L60 one-member boss HP exactly `149800` before PARTY_DEFAULT.

Every formula test uses integer/floor behavior exactly as specified; no approximate float assertions for committed integer results.

## Peak Opportunity Telemetry Tests
- an eligible session is exactly the `vision.md` predicate (>=15 minutes and >=30 accepted non-idle gameplay intents); excluded sessions never enter the denominator,
- each presented peak opportunity emits `session_id`, character level/map, type, `source`, timestamp, `outcome`, and content revision exactly once,
- first-15 numerator accepts only `ATLAS_SEEN`, `CHEST_SPOTTED`, `QUEST_CLUE`, `JUST_GUARD_WINDOW`,
- `FISH_RARE` / `CHEST_HIDDEN` / Just Guard success / clutch / `KHOE` do not enter the first-15 numerator,
- `FISH_RARE` and Atlas Seen from the same rare catch emit one `PHAT_HIEN`, not two; `CHEST_HIDDEN` follows the same one-peak rule with Atlas Seen,
- `JUST_GUARD_WINDOW` emits on connected eligible hits even when `just_guard_triggered=false`; `DODGED` and hard-control/ICD skip emit none,
- `CUU_NGUY` slow-mo starts only on `just_guard_triggered` or `beast_passive2_success`,
- the weekly SLO calculation is `eligible_sessions_with_first_15m_opportunity / eligible_sessions`, with target `>=0.85`,
- an SLO miss emits an operational alert/review record only; it never grants a reward or modifies combat, EXP, RNG, or economy state.

## First-Session Act I Tests
A scripted Level-1 character on `quest.main.a1.duong_vao_lang` with empty inventory (no `chia_khoa_co`, no fishing rod) must, within 15 minutes of wall time and >=30 accepted intents:
- `REACH` `dinh_lang` then `bo_ruong`,
- `INTERACT` `quest_object.a1_duong_vao_lang.moc_tre` exactly once and emit `PHAT_HIEN` `source=QUEST_CLUE`,
- be able to fire `CHEST_SPOTTED` on `chest.hidden.map.lang_da.bo_ruong.01` from a path-legal standing point without a key or double-jump,
- on first connected eligible hit, set `just_guard_window=true` and `just_guard_hint=true` once (`progression.first_session.just_guard_hint`); hint does not change damage,
- after first qualifying kill, emit `ATLAS_SEEN` unless already counted with another same-settlement peak,
- complete this path with at least one first-15 qualifying source without rare fish, chest open, Linh Thú, or enhancement glow.
Retry/reconnect must not duplicate `QUEST_CLUE`, `CHEST_SPOTTED`, or the Just Guard hint.

# Combat Result Tests
Required cases:
1. post-mitigation damage is computed before shield absorption.
2. multiple shields absorb by earliest expiry -> lexical effect ID -> creation sequence.
3. same source/effect shield reapply keeps max(current remaining,new amount) and refreshes expiry without emitting break.
4. hostile damage that is fully absorbed still enters/refreshes `in_combat`.
5. HP-threshold trigger does not fire when a shield prevents threshold crossing.
6. `SHIELD_BROKEN`, `SHIELD_EXPIRED`, and `SHIELD_REMOVED` are mutually correct.
7. percent-MAX_HP heal/shield and percent-MAX_MP restore use typed ratio data and clamp to maxima.
8. trigger recursion depth >3 is suppressed/logged.
9. same effect ID default trigger count and explicit multi-hit behavior are deterministic.
10. target modes accepted by combat exactly match `skills.md`.

11. On respawn: character restores exactly `40% MAX_HP` and `40% MAX_MP`; `current_hp` and `current_mp` after respawn equal `floor(MAX_HP * 0.40)` and `floor(MAX_MP * 0.40)` respectively.
12. During respawn invulnerability window: outgoing damage from the respawned character is `0`; incoming damage is also `0`; the window ends on first accepted action-start or timer expiry, whichever comes first.
13. Just Guard juice (70ms hitstop, 220ms 0.35× slow-mo) does not change the 0.60 damage multiplier, ICD, or `in_combat`.
14. `beast_passive2_success` juice (90ms hitstop, 350ms 0.30× slow-mo) does not apply on ICD reject or failed predicate.
15. Predicted Just Guard/`CUU_NGUY` before `S2C_COMBAT_EVENT` is a test failure.
16. `just_guard_window=true` and `just_guard_triggered=false` does not apply the 0.60 multiplier or slow-mo.
17. `just_guard_hint` is true on the first window only; a second eligible hit has `just_guard_hint=false`.

# Status Tests
Cover BURN, BLEED, POISON, CHILL, SLOW, FREEZE, STUN, ROOT, VULNERABLE, CRIT_MARK, HEAL_REDUCTION, RESIST_SHRED, WEAKEN, AIRBORNE:
- default reapply/explicit STACK behavior,
- CHILL max 3 and source-aware ownership,
- SLOW multiplicative combination and 0.40 floor,
- control movement/action behavior,
- immunity before instance creation,
- dispel priority ordering,
- death/map-transfer/disconnect expiry.

Regression: another player's CHILL must not satisfy a caster-owned THUY freeze combo unless explicitly shared.

# Skill / Class Tests
For each of 5 classes:
- exactly 4 basic + 5 active + 3 passive definitions (12 skills per class, 60 total),
- unlock milestones match `skills.md` (Lv1 B1, Lv4 B2, Lv8 A1, Lv11 P1, Lv14 A2, Lv18 B3, Lv22 A3, Lv27 P2, Lv32 A4, Lv36 B4, Lv45 A5, Lv50 P3),
- exactly 1 learned basic attack equipped in the dedicated basic slot; active loadout count is `0..min(4, learned active count)` (five actives are learned, four may be equipped at a time — ADR-0033),
- held basic input is rate-limited by server cooldown and cannot create duplicate accepted actions,
- 75 Level-60 skill points (59 level-up + 12 books + 4 Lv55/Lv60 bonus — ADR-0033) cannot spend >75; full respec restores exactly spent points,
- movement skills cannot bypass blocked geometry/portals,
- passive/Soul/set proc cannot self-recurse accidentally,
- class remains solo-capable in scripted baseline encounter fixtures.

## Skill Runtime Data Tests
For all 20 basics and 25 ACTIVE skills (45 combat actions total):
- execution/targeting/tags resolve from canonical enums,
- `startup_ms/active_ms/recovery_ms` are present and non-negative,
- `timing_speed_stat` is `NONE|ATTACK_SPEED|CAST_SPEED`,
- one typed geometry variant resolves with positive required parameters,
- authoritative skill origin is exactly `caster_anchor_y + 0.9m` for every geometry and ignores animation sockets,
- each row passes its launch role band; projectile envelope is `<=8.8m` and positioned-circle outer reach is `<=11.0m`,
- hits at the exact authoritative hurtbox boundary or `0.001m` contact epsilon succeed; a quantized `0.002m` surface gap fails,
- projectile range/speed/radius are server-owned,
- movement distance/duration stops at authoritative collision,
- `luu_bo` sweeps contact only along its collision-resolved `MOVE_CONTACT_LINE`; `son_bich` validates and creates its grounded `BARRIER_POSITION` AABB,
- every secondary spatial effect resolves from typed data with deterministic selection/collision/cap rules,
- `DISPLACEMENT` is present if and only if the skill can force target position or apply canonical `AIRBORNE`,
- `boc_bo` ember trail creates no persistent zone or second hit/status query,
- launch ACTIVE startup <=700ms,
- basic base total action time remains `500..1000ms`.

Speed regression vectors prove:
```text
ATTACK_SPEED/CAST_SPEED -> startup/recovery only
```
and do not change:
```text
active_ms
projectile range/speed
forced movement distance/duration
zone/status lifetime
cooldown
```

Launch cost/cooldown defaults compile exactly:
```text
basic cost = 0
basic cost_timing = ON_START
ACTIVE MP cost_timing = ON_START
cooldown_start = ON_START
```
An accepted ON_START skill interrupted afterward still consumes cost/starts cooldown once; a pre-acceptance rejection consumes neither.

## Skill-Level Tests
For `S=1..12`:
```text
ACTIVE damage_scale     = 1 + 0.035 * (S-1)
ACTIVE support_scale    = 1 + 0.025 * (S-1)
ACTIVE cooldown_seconds = base_cooldown * (1 - 0.030 * (S-1))
```
Validate every active cooldown row (including `luu_bo` and `son_bich`) and all fifteen passive scaling rows from `class_skill_catalog.md` exactly.

Required regressions:
- basic attacks upgrade up to Level 12 with cooldown reduction and scaling status effect proc rate,
- active skills upgrade up to Level 12 with power scaling and cooldown reduction,
- passive skills upgrade up to Level 6,
- basic attack Lv1 and Lv12 cooldowns lie inside the class bands of `skills.md`, and `proc_bp(12) = max_proc` exactly,
- every class-skill damage component uses the owning class element; KHAC detonation consumes remaining scheduled ticks x stacks at `1.50x` (`classes.md`),
- `han_khi` consumes only the caster's own 3 CHILL stacks and respects its 5,000ms per-target lockout,
- every upgradeable level changes at least one gameplay number,
- no level silently adds a tag/mechanic/targeting/execution type,
- MOC poison and HOA burn tick counts/total coefficients match the catalog,
- basic DOT templates snapshot source ATTACK, tick at the authored 1,000ms boundaries, use no crit/dodge, and honor reapply/stack/dispel flags; `burn_true_3s` rejects normal cleanse,
- `area_splash_50` excludes primary, applies its 1.20m/50% payload, and cannot exceed the basic action's resolved target cap,
- MOC `hoi_xuan` base includes `0.12 target MAX_HP + 0.25 source ATTACK` before support scaling,
- THUY freeze-immune CHILL resolution applies the authored 25% SLOW/2s fallback.
- only `skill.kim.basic.vo_song_kiem` has PENETRATE + ratio 0.15; other class basic_4 rows have no PENETRATE tag and no ratio.

# Equipment / Build Tests
- exactly 3 loadouts; one ACTIVE, two SUPPORT.
- same item instance cannot occupy two positions.
- ACTIVE-only normal stats/set/Soul/Meridian/Formation contribution.
- each SUPPORT emits <=1 signature; character <=2.
- support signature ignores enhancement/rarity/roll magnitude/Soul level.
- 168 generated equipment IDs unique.
- enhancement rate/floor/failure vectors for all `+0..+16` transitions.
- Lucky/Insurance validity and consumption.
- explicit enhancement material/common multiplier tables match `crafting.md`.
- expected cumulative units remain approximately `27.26/192.02/275.35/525.35/785781.85` material at `+6/+8/+10/+12/+16` and corresponding common-base references under ADR-0021; the +13..+16 expectation traverses the persisted per-target soft-pity state machine rather than treating pity as absent or permanently maxed.
- base rolls never reroll from enhancement/reconnect/retry.
- Meridian relation direction and wrap-around matching.
- at most 3 active Meridian effects and one Formation.

# Soul Tests
- 25 definitions and `15 NORMAL + 7 ELITE + 3 BOSS` count.
- collection unlimited; same Soul ID max one contract per loadout.
- one item max one Soul; loadout max 3 Souls/1 BOSS Soul.
- ACTIVE Soul EXP only; thresholds `0,100,300,700,1500`.
- contracted equipment transfer blocked until contract resolves.
- Boss Soul FIRST_CLEAR grant is lifetime/idempotent; repeat roll does not replace guarantee.

# Spawn Tests
For all 54 persistent launch groups:
- selector expands to known monster(s),
- locator anchor exists in map fixture,
- shorthand expands to canonical population/respawn fields,
- max population is never exceeded under concurrent death/retry events,
- pool result is server-owned and stable across one creation retry,
- no hostile persistent group resolves inside safe/social map or portal/checkpoint safety radius.

Spawn density assertions (ADR-0035):
- NORMAL spawn groups: `max_alive = 20` per group; at least two NORMAL groups per field map yields `>= 40` alive NORMAL monsters per channel,
- ELITE spawn groups: `max_alive = 2` per group,
- NORMAL respawn band: `10..14s` — assert `respawn_min = 10`, `respawn_max = 14` (ADR-0062),
- ELITE respawn band: `35..60s` — assert `respawn_min = 35`, `respawn_max = 60` (ADR-0062); ELITE floor `2 × 3600 / 60 = 120/hour >= 22 × 5 = 110/hour`,
- NIGHT_RARE authored groups: respawn band `240..360s` — assert `respawn_min = 240`, `respawn_max = 360`; NIGHT_RARE group is only active during night phase and not selectable as a daytime spawn,
- total theoretical supply across two NORMAL groups (40 alive, 10..14s respawn): conservative planning floor (slowest 14s) ≈ 10,286/hour; expected operating (avg 13s) ≈ 11,077/hour. Demand is sized at `FORCED_PLACEMENT_HARD_CAP = 22`: 22 × 450 = 9,900/hour (ratio ≥ 1.04× floor, ≈ 1.12× expected); at `MAX_PLAYERS_PER_CHANNEL = 18`: 8,100/hour (≥ 1.27× floor). Assert both constants and that 9,900/hour does not exceed the conservative floor (ADR-0062).

Sleep/wake and restart reconstruct population without granting rewards.

# Monster / Boss Tests
- all 58 launch non-boss definitions (46 NORMAL + 12 ELITE) plus 6 Season-0 variants (total 64 non-boss monster rows in `monster_catalog.md`) compile and expand into required runtime fields (HP, ATTACK, DEFENSE, element, movement, combat profile, base_exp, concrete drop_table_id).
- 6 Season 0 variants (`dom_dom_nguyen`, `bup_lua`, `tinh_buoi`, `co_lua`, `vong_bien`, `hon_gao`) are selectable in their Act I pools only while `season_region_index = 0` (`map_spawn_catalog.md` § Logical Pool Resolution) and never otherwise; group `max_alive` is unchanged.
- EXP and drop acceptance for 6 seasonal variants: base_exp matches Act I NORMAL band (545..590), drop tables resolve to valid items without power inflation, and kills register toward seasonal Atlas pages (`atlas.page.season.0.*`).
- fixed monster level never follows player level.
- loot/EXP death settlement happens once.
- ordinary party reward range requires same instance + <=30m unless bounded override.
- all 8 boss HP values expand from the current doubled durability formula.
- PUBLIC boss HP percentage preserved when participant scaling changes.
- same PUBLIC generation cannot reward one character twice through channel hopping.
- boss add/summon default gives no EXP/loot.
- hard CC converts to boss stagger by default.
- `boss.than_trung` first-progression-clear EXP `1,091,400` settles once/character independently of base EXP, repeat loot, Soul first-clear, and account-special reward.

## Channel EXP Rate Vectors
Assert authored per-unit EXP exactly (progression_route Channel EXP Rate References). NORMAL base_exp:
- Act I target 570: `dom_dom_ma` 545, `bu_nhin_rom` 545, `coc_thanh_tinh` 560, `quy_nhap_trang` 565, `vong_hon` 575, `hon_ma_co_thu` 585, `hon_do_trang` 590, `vong_hon_gia` 595
- Act II target 736: `ma_rung` 707, `dom_lua` 714, `bong_nguoi` 730, `tinh_cay` 745, `dai_tinh_cay` 753, `vong_rung_sau` 768
- Act III target 563: `ma_da` 546, `ca_tinh` 551, `quy_song_dem` 551, `thuong_luong` 556, `bong_nuoc_ma` 562, `hon_chet_duoi` 572, `ca_tinh_gia` 578, `nguoi_song_co` 588
- Act IV target 627: `ma_tranh` 609, `khi_nui` 615, `ma_van_dem` 615, `ho_tinh` 620, `ho_con_tinh` 626, `vong_rung` 636, `ho_tinh_lon` 642, `vong_nui_gia` 653
- Act V target 840: `tuong_da` 814, `hon_binh` 821, `ma_co` 834, `oan_hon_dem` 841, `qua_tinh` 848, `hon_tran_linh` 854, `qua_tinh_lon` 868
- Act VI target 933: `vong_linh` 911, `tinh_thu` 918, `than_rung_dem` 918, `ngu_tinh` 925, `ma_nui` 932, `than_trung` 938, `hon_binh_co` 945, `dai_vong_linh` 952, `tinh_nui_gia` 959
ELITE base_exp (act average = target): I 51333 `hon_xo_non` 49800 / `ma_xo` 52866; II 66267 `ma_tranh` 64500 / `moc_tinh` 68034; III 50654 `ma_da_gia` 49300 / `thuy_quai` 52008; IV 56378 `ma_tranh_gia` 54800 / `ho_tinh_ve` 57956; V 75582 `thach_ve` 73500 / `hon_tuong` 77664; VI 83954 `linh_ve` 81700 / `bong_vong` 86208.
Boss kill EXP: PUBLIC `ma_da_chua` 168846, `ngu_tinh` 279846; every INSTANCED boss (`quy_nhap_trang`, `moc_tinh_da`, `thuong_luong`, `ho_tinh`, `ho_tinh_chin_duoi`, `than_trung`) grants 0 kill EXP.
Dungeon EXP: trash/ELITE/boss kills inside a dungeon grant 0; completion grants `dungeon_repeat_exp(min(character_act, dungeon_tier_act + 1))` (e.g. Act VI character in T5 → 139923; Act VI character in T4 → 125970).
ELITE_BOSS split: Acts III and VI budget 6% ELITE + 2% PUBLIC boss; Acts I, II, IV, V budget 8% ELITE.
Spirit Surge: completion EXP uses act `min(character_act, region_act + 1)`; an Act IV character in the Act III region earns the Act IV rate, in the Act I region the Act II rate.
WORLD_EVENT Spirit Surge completion grants character EXP (not inventory) equal to WORLD_EVENT per-unit: I 256667, II 331333, III 253269, IV 282111, V 377909, VI 419769. Key `surge.completion.exp.<utc_hour>.<character_id>`.
LIFE_SKILL: each successful `FISH_CAUGHT`, `DISH_COOKED`, and atlas tier-up grants LIFE_SKILL per-unit for the character's current act: I 6417, II 8283, III 6332, IV 7047, V 9448, VI 10494.
BOUNTY: `bounty_set_exp` at 1.5 sets/hour: I 171111, II 220889, III 168846, IV 188074, V 251939, VI 279846. Still 3 completions = 1 set; 3-cap/UTC-day unchanged.

# Dungeon Tests
Run each of five launch dungeons with party sizes `1` and `5`:
- correct stage ordering/checkpoints/final boss,
- sampled encounter scaling `1.00x/3.20x HP` and `1.00x/1.16x damage`,
- active encounter does not live-rescale on disconnect/reconnect,
- wipe resamples next attempt,
- REPEAT always available,
- FIRST_CLEAR commits once lifetime,
- progression first-clear EXP exact values `15,400/99,400/263,400/507,400/831,400` (= 0.4% of each act's budget),
- DAILY_FIRST (when configured) scopes by UTC day,
- no reward gate becomes entry lockout,
- MAIN `DUNGEON` objective completes in one successful run without second boss kill.

For every ENDGAME_L60 run:
- repeat reward settles exactly one `.endgame` combined table,
- baseline dungeon repeat and final-boss repeat tables do not also settle,
- unearned lifetime first-clear slots may still settle once,
- final boss one-member base before party scaling is `149800 HP`,
- authored mechanic remix is present; a stat-only endgame copy fails validation.

# Quest / Progression Tests
- 24 MAIN chain has no prerequisite cycle.
- full MAIN path contains zero standalone PUBLIC-boss wait requirement.
- each act's MAIN EXP rewards total exactly 3.0% of the authored act budget (ADR-0032).
- 24 normal-world maps each own exactly one character first-discovery EXP slot.
- each act's safe-anchor first discovery = 0.2% of act budget and each of three FIELD discoveries = 0.2% of act budget, total 0.8% (not 7%).
- each act's first dungeon/finale major clear = 0.4% of act budget.
- STORY_ONCE budget = 5.0% of act budget per act (MAIN 3.0% + SIDE 0.8% + anchor 0.2% + fields 0.6% + first-clear 0.4%).
- all discovery and first-progression-clear operations are character-scoped and idempotent.
- 12 SIDE quests never gate MAIN.
- each of 24 MAIN and 12 SIDE declares exactly one `mystery_type` in {LIGHT_ORDER,FALSE_TRAIL,FORBIDDEN_GROUND,DROP_THROUGH,HEIGHT_BAND,WATER_GATE} and exactly one `mystery_owner`,
- DAILY templates and the Spirit Surge event quest declare no mystery beat,
- dungeon/boss-owned MAIN owners resolve (`dinh_lang_bo_hoang`, `mieu_ba_trong_rung`, `xom_chim`, `hang_ma_tranh`, `den_tran`, `boss.than_trung`),
- `WATER_GATE` appears only with `mystery_owner = dungeon.xom_chim`,
- field quests never use `TIMING_WINDOW` or animated flood,
- FORBIDDEN_GROUND hazards are static decal+collision with no extra spawn,
- DROP_THROUGH completes only after down+jump through a one-way platform,
- HEIGHT_BAND does not complete from the main-path volume,
- LIGHT_ORDER field uses ≤2 authentic idle sprites (except `zone.nui_thieng` which may use 3 per `quests.md`), spatial while moving; dungeon-owned LIGHT_ORDER may use that dungeon's 3-lamp/3-drum stage,
- no MAIN/SIDE uses `EVIDENCE` fetch or `LISTEN` dwell,
- `quest.main.a1.duong_vao_lang` INTERACT `moc_tre` remains `QUEST_CLUE` with FORBIDDEN_GROUND jump-in,
- same-account characters cannot transfer items, common/bound/special, Linh Đan, or beasts (ADR-0029); only IAP is account-shared,
- `quest.main.a1.dinh_lang_bo_hoang` complete grants exactly one starter beast; retry cannot duplicate,
- every TOWN has bonfire, hearth, sparring ring; every `has_water=true` map has ≥1 fishing_spot,
- party EXP N=5 is 80% of each member's solo EXP, not ~0.28×,
- Just Guard requires a horizontal movement-intent EDGE (press, release, or direction flip) in the 150ms window with no other movement edge in the preceding 400ms; a held direction does NOT qualify (ADR-0034); `bu_nhin_rom` rush has ≥400ms telegraph,
- PvP PREPARING snapshot includes active Linh Thú; swap during match rejected,
- Daily board = 6 choices, max 3 completions, <=2 same objective family, targets accessible content only.
- reset 00:00 UTC expires unfinished Daily without power penalty.
- quest completion/reward retry is idempotent.
- full inventory routes earned items to Reward Claims rather than loss/reroll.

Conservative progression simulation from `../07_content/balance_validation.md` must satisfy both channel-deviation reject rules: independently authored per-unit channel EXP (authored monster/dungeon/event EXP × unit frequency × channel hours, per "Channel EXP Rate References" in `../07_content/progression_route.md`) must not deviate from the target channel EXP by more than 15% for any channel in any act; and |derived_act_hours − target_act_hours| / target_act_hours ≤ 0.15 for any act. Daily/PvP/Guild War must not be injected into this simulation.

# Reward / Economy Tests
- currency caps fail atomically; never clamp silently.
- direct trade/AH escrow state and idempotency.
- auction pending proceeds cannot duplicate and claim respects common-currency cap.
- all random reward choices are committed before inventory insertion; retry/full inventory never rerolls selected slot/quantity/Soul.
- HUNT modifies only explicitly eligible regional material entries.
- no equipment/Soul/cosmetic jackpot receives HUNT bonus.
- bound-purchased Bùa output is `CHARACTER_BOUND ON_ACQUIRE` and cannot be laundered via trade/Auction/storage/stack merge.
- special character-first launch sources total exactly 20 once/character (ADR-0029; currency.special is CHARACTER-scoped, not account-scoped).
- both 20-special cosmetic alternatives exist; spending one does not consume/award the other entitlement.

# Balance Regression Tests
Run the deterministic synthetic build described by `../07_content/balance_validation.md` for all five classes at Levels `10,20,30,40,50,60`.

Hard gates:
```text
same-level NORMAL basic-only TTK = 2..6s
same-level ELITE basic-only TTK  = 8.5..24s
major boss solo basic-only TTK   = 55..130s
major boss 5-player basic-only   = 35..85s
normal readable 1.40x boss heavy = 6..18% synthetic reference MAX_HP
mandatory-boss class basic-TTK max/min spread <= 2.20
channel-deviation reject: per-unit cross-check > 15% deviation from canonical target per channel per act (see progression_route.md Channel EXP Rate References)
act-hours reject: |derived_act_hours − target_act_hours| / target_act_hours <= 0.15 for all acts
```

Lv60 active-rotation regression review windows:
```text
ordinary Lv60 boss solo ~73..111s
ordinary Lv60 boss 5p   ~47..71s
ENDGAME_L60 solo        ~129..194s
ENDGAME_L60 5p          ~82..124s
```
A >15% drift outside those broad rotation windows requires an intentional balance note/review; hard-gate failure rejects activation.

Simulator output must include the active content revision and derived reference stats. The test must not maintain a second hidden runtime formula set.


# Fishing Tests
- legal catch IDs are `COMMON_CATCH ∪ RARE_CATCH ∪` IDs of the active seasonal table (at most one extra `SEASONAL_CATCH`); default table remains closed to `COMMON_CATCH ∪ RARE_CATCH`; any other item_id fails compile,
- `C2S_INTERACT` `interact_kind=CAST` starts CASTING; `HOOK_WINDOW` accepts only `interact_kind=HOOK`,
- `fishing.catch.default` weights are positive integers summing to 10000 with `item.material.ca_chep_hoa_rong` = 100,
- `item.material.ca_chep_hoa_rong` is CHARACTER_BOUND, not a cooking input, not tradable,
- `item.material.ca_ro_dong` exists as COMMON_CATCH,
- a seeded PCG-64 (`math/rand/v2`) roll that selects `ca_chep_hoa_rong` settles one item, one `PHAT_HIEN` `source=FISH_RARE`, and at most one Atlas Seen grant,
- retry of the same `fishing.<character_id>.<utc_date>.<cast_sequence>` cannot reroll or refund bait,
- 51st successful catch in a UTC day is rejected,
- `fishing.catch.season.{0,1,2,5}` each sum to 10000; seasons 3–4 have no catch table,
- `SEASON_EPOCH_UTC_SECONDS = 1799020800`; `season_number` at that instant is 0.

# Hearth / Kindling Tests
- each of 5 `recipe.food.*` hearth recipes grants `extra_output = 1 item.material.cui_lua_trai`,
- `COOK` is the kindling faucet; no shop row or price exists for `cui_lua_trai`,
- `KINDLE` / `COOK` / `BONFIRE_REST` use `C2S_INTERACT` `interact_kind`,
- full inventory routes the kindling extra_output to Reward Claims rather than loss.

# Spirit Surge Tests
For representative UTC hour indexes:
- exactly **three concurrent regions** are active per UTC hour: indices `H mod 6`, `(H+2) mod 6`, `(H+4) mod 6` (deterministic from server UTC hour H; no randomness),
- a given region is active in exactly 3 of every 6 consecutive UTC hours (50% availability per region),
- the three active region indices for H and H+6 are identical (period = 6 hours),
- same three regions survive server restart without reroll for the current UTC hour; a restart at minute < 15 reactivates them until `HH:15:00` only, a restart at minute >= 15 activates nothing until the next hour (`../04_architecture/service_boundaries.md` § Spirit Surge Scheduling),
- a surge deactivates at `HH:15:00`, and a channel partition started inside the window receives the activation with the same end time,
- safe anchor is never selected as an event field,
- <=2 event groups per active region, <=4 alive/group,
- event attack tell >=0.80s,
- presence-only character earns zero completion reward,
- >=10 valid contribution points + wave-03 presence qualifies,
- daily-first once/day; repeat baseline remains available,
- a character in a non-active region receives no Spirit Surge event spawns or participation UI.

# New-Stat Combat Tests (ADR-0037)
Test cases for LIFESTEAL, REFLECT, ABSORB, and HEAL_REDUCTION behaviors.

## Lifesteal Throttle
1. Lifesteal per-second throttle: when committed LIFESTEAL heal in a 1.0s rolling window exceeds `LIFESTEAL_HPS_CAP = 0.015 * attacker MAX_HP`, the excess is discarded. It is NOT banked, queued, or deferred to the next window. Assert the healed HP equals exactly the throttle cap, not the raw `hp_damage * LIFESTEAL` value.
2. Throttle window resets are rolling: a second burst within the same second discards the overage; the first-second cap is not refilled until the window rolls past.
3. Lifesteal against AoE: the third and subsequent targets in one skill hit each contribute `hp_damage * LIFESTEAL * 0.30` toward the throttle bucket. Assert the 0.30 multiplier applies before throttle comparison.
4. Lifesteal against DoT ticks: each periodic tick contribution uses `tick_damage * LIFESTEAL * 0.30`. Assert the multiplier.
5. Lifesteal never triggers from: a `DODGED` result, reflected damage instances (tagged `NO_LIFESTEAL`), or damage the character deals to itself.

## Reflect Range Gate and Per-Hit Cap
6. Reflect 3.0m melee gate: a melee-range hit (originating component within 3.0m of the defender) triggers REFLECT. A ranged or DoT component that originated beyond 3.0m does NOT trigger REFLECT even if the projectile's endpoint is within 3.0m. Assert the gate uses component origin, not hit position.
7. Per-hit cap: `reflected = min(floor(hp_damage * REFLECT), floor(0.03 * attacker MAX_HP))`. Assert both the floor operations and the min clamp are applied in this order.
8. Reflect recursion impossible: the reflected instance is tagged `NO_REFLECT`. Assert no secondary REFLECT fires from the reflected damage instance regardless of the original attacker's own REFLECT stat.
9. Reflected damage carries all four tags: `NO_CRIT | NO_REFLECT | NO_LIFESTEAL | NO_PROC`. Assert each individually: cannot roll crit, cannot trigger REFLECT, cannot trigger LIFESTEAL, cannot trigger on-hit Soul/Formation/skill procs.
10. Reflected damage grants zero EXP, zero loot ownership, and zero threat to the defending character. A monster that dies from reflected damage settles rewards to the defender under normal contribution rules only.

## Absorb vs Heal Reduction
11. ABSORB is NOT reduced by HEAL_REDUCTION: apply maximum HEAL_REDUCTION (floor 0.40 remaining, i.e., 60% reduction) to a target who also generates absorb shields. Assert the shield magnitude is calculated from raw `hp_damage * ABSORB` (before HEAL_REDUCTION), not reduced by it.
12. LIFESTEAL IS reduced by HEAL_REDUCTION: apply HEAL_REDUCTION status to the lifesteal caster and assert that the lifesteal heal result passes through the caster's `HEALING_RECEIVED` stat at `(1 - HEAL_REDUCTION)`, producing a reduced heal.
13. HEAL_REDUCTION floor holds at maximum stacking: with multiple independent HEAL_REDUCTION sources applied multiplicatively, the combined HEAL_REDUCTION product must never fall below `0.40` (60% maximum reduction from HEAL_REDUCTION statuses). Assert `max(0.40, product)` clamps correctly, then PvP multipliers apply on top and the final value is never below `0.00`.

## Heal Reduction Floor at Maximum Stacking
14. Three simultaneous HEAL_REDUCTION sources each at maximum cap: `(1-0.30) * (1-0.30) * (1-0.30) = 0.343`, which is below the `0.40` floor. Assert the floor clamping applies and the effective multiplier is `0.40`, not `0.343`.

# PvP Determinism Tests
- Five Element Arena capture uses signed `capture_units` and the exact `10000/15000/20000` per-second contributor rates; contest freezes and absence decay starts only after 3s at the stored last rate,
- score target, normal-timer winner, simultaneous-target tie, overtime, and terminal `VOID` paths settle exactly one result/rating/reward operation,
- ranked meaningful-participation predicates use only authoritative counters; reconnect preserves counters, early surrender can settle a loss but not an unearned bound grant, and AFK/ABANDONED never qualifies.

# Content Compile Tests
Run `../07_content/integration_validation.md` and `../07_content/balance_validation.md` against every candidate revision. Activation test must prove:
```text
invalid revision -> rejected as a whole
previous revision remains active
no partial catalog becomes visible
```

# Regression Seeds
Maintain fixed server-RNG seeds for:
- enhancement success/failure edge cases,
- item secondary rolls,
- spawn-pool choice,
- drop-table choice,
- boss scaling resample,
- fishing.catch.default choice,

Gameplay RNG is Go `math/rand/v2` PCG-64. Tests assert deterministic results for a given content revision + seed.

# Required CI Result
Gameplay/content test suite must pass before merge/deploy of any behavior or launch-data change. A changed canonical formula, ID graph, state transition, reward slot, skill timing/geometry/scaling value, or content expansion requires updating/adding a regression vector in the same change.
