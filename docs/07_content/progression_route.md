# Lv1-60 Progression Route
status: LOCKED

## Target Pace
First-character Level 60 pacing estimate:
```text
2,000 hours of long-term gameplay
```
This is a balance/telemetry forecast from the authored activity mix, never a per-character constraint. The server does not use played hours to block EXP, level-up, access, or rewards.
Progression is designed for long-term depth and community bonding, combining story milestones with persistent field mob farming, party grinding, dungeon crawling, and world events.

Estimated time investment per act:
```text
Act I   (Lv 1-10):  ~15 hours   (onboarding, early skills, first party dungeon)
Act II  (Lv 11-20): ~75 hours   (auction, Meridian, Soul collection, guild creation)
Act III (Lv 21-30): ~260 hours  (Formations, ranked PvP introduction, mid-tier gear)
Act IV  (Lv 31-40): ~450 hours  (mastery, high-tier field hunting, team farming)
Act V   (Lv 41-50): ~550 hours  (harder mechanics, final regions, advanced sets)
Act VI  (Lv 51-60): ~650 hours  (culmination, endgame dungeons, Lv60 milestone)
---------------------------------------------------------------------------------
Total:              ~2,000 hours to reach maximum character Level 60
```
## Acts
| Levels | Purpose | New system |
|---|---|---|
| 1-10 | onboarding | crafting 5; party dungeon/guild join 10 |
| 11-20 | build/economy | auction 15; guild creation/Meridian/Soul 20 |
| 21-30 | build combinations | Formation 25; ranked PvP 30 |
| 31-40 | mastery/remixing | none |
| 41-50 | harder mechanics/final region | none |
| 51-60 | culmination/endgame prep | none |

# First Session — Act I (`first_session.act1`)
Canonical 0–15 minute onboarding. No key, rare fish, Linh Thú clutch, or +12 glow is required. First-15 SLO sources are only `ATLAS_SEEN`, `CHEST_SPOTTED`, `QUEST_CLUE`, `JUST_GUARD_WINDOW` (`../00_context/vision.md`).

| minute | Beat | map / object | peak | source |
|---|---|---|---|---|
| 0–2 | Spawn + `REACH` Đình Làng; see `bonfire.map.lang_da.dinh_lang` | `map.lang_da.dinh_lang` | none (village seen) | — |
| 2–4 | `REACH` ruộng | `map.lang_da.bo_ruong` | none | — |
| 4–8 | Inspect bamboo marker | `quest_object.a1_duong_vao_lang.moc_tre` | PHAT_HIEN | `QUEST_CLUE` |
| 5–12 | First connected hit with Just Guard window vs telegraphed `bu_nhin_rom` | field combat | CUU_NGUY opportunity | `JUST_GUARD_WINDOW` |
| 6–12 | First qualifying kill | `bu_nhin_rom` / `dom_dom_ma` | PHAT_HIEN | `ATLAS_SEEN` |
| 8–15 | Spot first-session chest from the path | `chest.hidden.map.lang_da.bo_ruong.01` | PHAT_HIEN | `CHEST_SPOTTED` |

Seeing the đình bonfire is presentation-only (`VILLAGE_SEEN`). It is **not** a first-15 SLO source.

Cutscenes/hints must not block input longer than `3s`. No extra combat power.

## Zone Contract
Exactly six main progression zones, one per act. Normally each has one safe/social anchor, three field maps, and one major dungeon/finale path. Use Vietnamese folklore/environment identity; exact names/lore are authored content.

# EXP Budget
The previous `75-85%` pacing statement had no concrete first-discovery/first-completion EXP source, which would have forced undocumented grinding. Launch now has an explicit deterministic budget.

Act totals under the canonical level curve (`exp_required(L) = 10000 * L * L`, ×100 scale; Act VI capped at L < 60):
| Act | Level band | act_exp_total |
|---|---|---:|
| I | 1-10 | 3,850,000 |
| II | 11-20 | 24,850,000 |
| III | 21-30 | 65,850,000 |
| IV | 31-40 | 126,850,000 |
| V | 41-50 | 207,850,000 |
| VI | 51-60 | 272,850,000 |
| **TOTAL** | | **702,100,000** |

```text
Verification: 3,850,000 + 24,850,000 + 65,850,000 + 126,850,000 + 207,850,000 + 272,850,000 = 702,100,000 ✓
```

## Seven-Channel EXP Source Portfolio

The old 75/25 one-time/repeatable split is replaced by a seven-channel portfolio. Under a 2,000-hour target the one-time authored content covers ~100 hours; repeatable channels must carry the remaining ~1,900.

| Channel | % of act budget | Target hours (of 2,000) |
|---|---:|---:|
| `FIELD_COMBAT` — NORMAL monster kills | **40%** | 800 |
| `DUNGEON_REPEAT` — repeatable dungeon completion | **18%** | 360 |
| `WORLD_EVENT` — Spirit Surge participation | **12%** | 240 |
| `BOUNTY_REPEAT` — daily bounty sets | **12%** | 240 |
| `ELITE_BOSS` — ELITE kills + major boss kills | **8%** | 160 |
| `LIFE_SKILL` — fishing / cooking / Atlas tier milestones | **5%** | 100 |
| `STORY_ONCE` — MAIN + SIDE quests, first discovery, first clear | **5%** | 100 |
| **TOTAL** | **100%** | **2,000** |

Bonfire Rest EXP (`../02_world/world_rules.md`, up to 180,000 EXP/day) is a minor social dwell bonus outside this seven-channel portfolio. It is excluded from the 2,000-hour baseline progression calculation and does not perturb the 100% portfolio budget.

Sub-split of `ELITE_BOSS`: in Act III and Act VI (acts with an open-world PUBLIC boss: `boss.ma_da_chua`, `boss.ngu_tinh`) ELITE = 6%, PUBLIC boss = 2%; in Acts I, II, IV, V ELITE = 8%. Instanced dungeon/finale bosses grant no kill EXP (their EXP is inside the dungeon completion settlement and STORY_ONCE first clear), so a dungeon run is never counted twice.

Sub-split of `STORY_ONCE` (all percentages are of the act budget):
```text
MAIN quests                      3.0%
SIDE quests (both)               0.8%   (0.4% each)
first safe-anchor discovery      0.2%
3 first field-map discoveries    0.6%   (0.2% each)
first major dungeon/finale clear 0.4%
─────────────────────────────────────
STORY_ONCE total                 5.0%
```

Per-act channel EXP allocation (each row must sum to act_exp_total):

| Channel | Act I | Act II | Act III | Act IV | Act V | Act VI |
|---|---:|---:|---:|---:|---:|---:|
| `FIELD_COMBAT` 40% | 1,540,000 | 9,940,000 | 26,340,000 | 50,740,000 | 83,140,000 | 109,140,000 |
| `DUNGEON_REPEAT` 18% | 693,000 | 4,473,000 | 11,853,000 | 22,833,000 | 37,413,000 | 49,113,000 |
| `WORLD_EVENT` 12% | 462,000 | 2,982,000 | 7,902,000 | 15,222,000 | 24,942,000 | 32,742,000 |
| `BOUNTY_REPEAT` 12% | 462,000 | 2,982,000 | 7,902,000 | 15,222,000 | 24,942,000 | 32,742,000 |
| `ELITE_BOSS` 8% | 308,000 | 1,988,000 | 5,268,000 | 10,148,000 | 16,628,000 | 21,828,000 |
| `LIFE_SKILL` 5% | 192,500 | 1,242,500 | 3,292,500 | 6,342,500 | 10,392,500 | 13,642,500 |
| `STORY_ONCE` 5% | 192,500 | 1,242,500 | 3,292,500 | 6,342,500 | 10,392,500 | 13,642,500 |
| **act_exp_total** | **3,850,000** | **24,850,000** | **65,850,000** | **126,850,000** | **207,850,000** | **272,850,000** |

```text
Verification (Act I):  1,540,000+693,000+462,000+462,000+308,000+192,500+192,500 = 3,850,000 ✓
Verification (Act VI): 109,140,000+49,113,000+32,742,000+32,742,000+21,828,000+13,642,500+13,642,500 = 272,850,000 ✓
```

`STORY_ONCE` sub-split per act:

| Sub-channel | Act I | Act II | Act III | Act IV | Act V | Act VI |
|---|---:|---:|---:|---:|---:|---:|
| MAIN quests 3.0% | 115,500 | 745,500 | 1,975,500 | 3,805,500 | 6,235,500 | 8,185,500 |
| SIDE quests 0.8% | 30,800 | 198,800 | 526,800 | 1,014,800 | 1,662,800 | 2,182,800 |
| safe anchor 0.2% | 7,700 | 49,700 | 131,700 | 253,700 | 415,700 | 545,700 |
| 3 fields 0.6% | 23,100 | 149,100 | 395,100 | 761,100 | 1,247,100 | 1,637,100 |
| first clear 0.4% | 15,400 | 99,400 | 263,400 | 507,400 | 831,400 | 1,091,400 |
| **STORY_ONCE 5.0%** | **192,500** | **1,242,500** | **3,292,500** | **6,342,500** | **10,392,500** | **13,642,500** |

### Baseline Leveling Pace and Daily/PvP Decoupling
- **Continuous Leveling Feasibility**: Baseline first-character leveling to Level 60 is strictly designed to be continuous and achievable through uncapped ambient channels (`FIELD_COMBAT`, `DUNGEON_REPEAT`, `ELITE_BOSS`, `LIFE_SKILL`, `WORLD_EVENT`, `STORY_ONCE`). A player who plays in extended or marathon sessions without logging in across hundreds of calendar days is never progress-locked by daily reset timers.
- **BOUNTY_REPEAT Role**: The `BOUNTY_REPEAT` allocation (12% of act budget, 240 target hours) models the structured daily bounty loop (1 set = 3 bounties per active day, ~40 minutes/set at 1.5 sets/hour equivalent rate, capped at 1 set/UTC day per `02_world/quests.md`). Over an expected multi-month journey (~360 active days), this channel provides a high-efficiency progression accelerator. If a player bypasses daily bounties, the equivalent EXP is readily obtainable via uncapped ambient `FIELD_COMBAT` and `DUNGEON_REPEAT` at standard channel rates without causing progression failure or balance rejection.
- **Ranked PvP and Guild War**: These modes remain zero-EXP progression channels and are never required for character leveling.

## Act III Efficiency Note
Implied EXP/hour by act: I 256,667 / II 331,333 / III 253,269 / IV 282,111 / V 377,909 / VI 419,769. Act III shows a ~24% efficiency drop from Act II — the sharpest single-act decline in the arc. This is intentional: Act III introduces two complex systems (Formation at Lv25; ranked PvP at Lv30). System-introduction overhead — time spent experimenting with Formation element combinations, understanding PvP queues and rulesets, and adjusting gear layouts for new mechanics — reduces effective farming rate relative to Act II's mature loop. The 260-hour budget provides room for this learning phase; EXP/hour rises steadily from Act IV onward as those systems become routine. No adjustment to Act III hours is made; this note is the binding design rationale required by the 15% act-hours audit.

## Concrete First-Discovery EXP
Exactly four normal-world first-discovery EXP slots exist per act: the safe anchor and three adventure fields. All are part of the `STORY_ONCE` channel (see sub-split table above).

| Act | safe anchor 0.2% | each field 0.2% | three fields 0.6% | discovery total 0.8% |
|---|---:|---:|---:|---:|
| I | 7,700 | 7,700 | 23,100 | 30,800 |
| II | 49,700 | 49,700 | 149,100 | 198,800 |
| III | 131,700 | 131,700 | 395,100 | 526,800 |
| IV | 253,700 | 253,700 | 761,100 | 1,014,800 |
| V | 415,700 | 415,700 | 1,247,100 | 1,662,800 |
| VI | 545,700 | 545,700 | 1,637,100 | 2,182,800 |

Character first-discovery key:
```text
reward.discovery.<map_id>.<character_id>
```

Rules:
- reward commits only on first successful authoritative entry to the map,
- reconnect/re-entry cannot repeat it,
- travel service cannot grant discovery unless the destination was already discovered, so it cannot skip forward rewards,
- discovery EXP is character progression, not an inventory reward and never creates Reward Claims,
- at Level 60 normal character EXP remains ignored under `../01_gameplay/progression.md`.

Concrete map ownership is in `world_route_catalog.md`.

## Concrete Major First-Completion EXP
Each act has exactly one progression-path first-completion EXP slot equal to `0.4%` of that act's EXP budget (within the `STORY_ONCE` channel):

| Act | source | first-completion EXP |
|---|---|---:|
| I | `dungeon.dinh_lang_bo_hoang` | 15,400 |
| II | `dungeon.mieu_ba_trong_rung` | 99,400 |
| III | `dungeon.xom_chim` | 263,400 |
| IV | `dungeon.hang_ma_tranh` | 507,400 |
| V | `dungeon.den_tran` | 831,400 |
| VI | `boss.than_trung` finale | 1,091,400 |

Character idempotency namespace:
```text
reward.first_progression_clear.<source_id>.<character_id>
```

For Acts I-V this EXP is part of the existing dungeon `FIRST_CLEAR` settlement alongside its equipment/material rewards. For Act VI it is an independent first-finale-clear progression slot on `boss.than_trung`.

This bonus is not a lockout: repeat dungeon/boss rewards remain governed by their owning specs; only this character EXP slot is one-time.

## Optional SIDE Contribution
Two SIDE quests per act each grant `0.4%` of that act's EXP budget (sub-slot within `STORY_ONCE`), as concretely defined in `quest_catalog.md`.

Completing both therefore contributes another optional `0.8%` of the act budget; this is part of the `STORY_ONCE` channel and allows players to substitute authored folklore side stories for some ambient combat/repeat content.

## Overlevel / Underlevel Guardrail
Recommended map level is guidance, not hidden scaling. EXP rewards above are fixed content values.

If optional play causes a character to overlevel an act:
- later discovery/quest rewards are not reduced merely for being overlevel,
- normal access gates still require story flags,
- Level 60 ignores further character EXP instead of converting it into another progression currency.

If telemetry shows the median first character repeatedly hits a next-act level gate only after substantial unplanned monster grinding, adjust authored EXP source values/reward pacing rather than introducing an invisible EXP multiplier.

# Channel EXP Rate References
Canonical per-unit EXP values back-derived from target act-hours and channel allocations. These are the independent reference values the 15%-deviation reject rule checks against. Tooling must cross-check authored content (monster EXP in `monster_catalog.md`, dungeon completion EXP in `dungeon_catalog.md`, event EXP in `world_event_catalog.md`) against these figures; a mechanical derivation from act_total × channel_percentage always yields zero deviation and does not constitute a real check.

## Assumed Unit Frequencies
```text
FIELD_COMBAT:   600 kills/hour  peak-optimal (one kill every 6s sustained)
                450 kills/hour  realistic sustained (travel, loot, death, idle counted)
                Derivation uses 450/hour; 600/hour is the cap for performance telemetry.
DUNGEON_REPEAT: 3 runs/hour     (~20-minute average run across all tiers)
WORLD_EVENT:    1 event/hour    (every hour one of the character's own region or the region one act below is active; EXP act rule in world_event_catalog.md)
BOUNTY_REPEAT:  1.5 sets/hour   (~40 minutes per bounty set including travel)
ELITE kills:    5 kills/hour    (encounters ~60s each including travel; sub-channel of ELITE_BOSS)
Major boss:     1.5 kills/hour  (PUBLIC bosses only; 30..45 min respawn + fight ≈ 40 min cycle; sub-channel of ELITE_BOSS in Acts III and VI)
LIFE_SKILL:     40 actions/hour (fishing casts, cooking steps, atlas interaction)
STORY_ONCE:     per-item EXP values are explicitly authored; no per-unit rate needed.
```

## Per-Unit EXP Back-Derived at 450 Kills/Hour

Derivation: `EXP_per_unit = act_exp_total / (act_hours × units_per_hour)`
(channel % cancels — the per-unit rate is the same formula for every channel because each channel's time share equals its EXP share)

| Channel | Formula basis | Act I | Act II | Act III | Act IV | Act V | Act VI |
|---|---|---:|---:|---:|---:|---:|---:|
| FIELD_COMBAT | EXP/kill (450/h) | 570 | 736 | 563 | 627 | 840 | 933 |
| DUNGEON_REPEAT | EXP/run (3/h) | 85,556 | 110,444 | 84,423 | 93,963 | 125,970 | 139,923 |
| WORLD_EVENT | EXP/event (1/h) | 256,667 | 331,333 | 253,269 | 282,111 | 377,909 | 419,769 |
| BOUNTY_REPEAT | EXP/set (1.5/h) | 171,111 | 220,889 | 168,846 | 188,074 | 251,939 | 279,846 |
| ELITE kill | EXP/kill (5/h) | 51,333 | 66,267 | 50,654 | 56,378 | 75,582 | 83,954 |
| Major boss (PUBLIC) | EXP/kill (1.5/h) | — | — | 168,846 | — | — | 279,846 |
| LIFE_SKILL | EXP/action (40/h) | 6,417 | 8,283 | 6,332 | 7,047 | 9,448 | 10,494 |

Cross-check against authored sources: `monster_catalog.md` NORMAL kill EXP and ELITE kill EXP must match the FIELD_COMBAT and ELITE kill columns within 15% per act. `dungeon_catalog.md` completion reward EXP must match the DUNGEON_REPEAT column within 15%. `world_event_catalog.md` Spirit Surge completion EXP must match WORLD_EVENT within 15%. Any deviation > 15% triggers the channel reject rule below.

STORY_ONCE per-unit values are explicitly defined in the sub-split tables above and in `integration_validation.md`; they are not repeated here.

# Endgame
At Lv60 reuse existing systems: repeatable endgame dungeon/boss rewards, equipment optimization, Souls, Meridian/Formations, Guild Ritual, ranked PvP/Guild War, cosmetics/prestige. Do not add an infinite launch power level.

# Anti-FOMO
No mandatory daily chain. First-clear/discovery bonuses are permanent one-time route rewards, not timed appointments. Repeatable baseline rewards remain available where owning specs allow them.

# Validation
Reject when:
- any channel's EXP computed from independently authored per-unit values (see Channel EXP Rate References above: authored monster/dungeon/event EXP × unit frequency × channel hours) deviates from the target channel EXP by more than 15% in any act,
- `| derived_act_hours - target_act_hours | / target_act_hours > 0.15` for any act,
- MAIN quest EXP no longer equals 3.0% of act budget (within STORY_ONCE) without intentional budget change,
- one act has other than 1 safe + 3 field first-discovery EXP slots,
- discovery reward percentage differs from 0.2% safe / 0.2% each field,
- first major progression clear differs from 0.4% of act budget,
- a discovery/first-clear key is account-scoped instead of character-scoped,
- repeat map entry or repeat dungeon completion can regrant the one-time EXP slot,
- baseline first-character leveling route creates an inescapable hard gate requiring daily reset cycles, calendar appointments, or PvP/Guild-War participation (route must remain completable via uncapped ambient channels),
- a hidden player-level EXP scaler is introduced.

# Invariants
```text
first-character Level-60 pacing estimate = ~2,000h
EXP curve = 10000 * L * L (×100 scale); total 1->60 = 702,100,000
seven-channel portfolio sums to 100% per act
FIELD_COMBAT = 40% / DUNGEON_REPEAT = 18% / WORLD_EVENT = 12% / BOUNTY_REPEAT = 12%
ELITE_BOSS = 8% / LIFE_SKILL = 5% / STORY_ONCE = 5%
STORY_ONCE sub-split: MAIN 3.0%, SIDE 0.8%, anchor 0.2%, 3 fields 0.6%, first-clear 0.4%
first normal-world discovery share = 0.8% per act (0.2% anchor + 0.6% fields)
first major progression clear share = 0.4% per act
SIDE optional EXP = 0.8% per act if both completed
channel share reject: per-unit cross-check deviation > 15% from canonical target (see Channel EXP Rate References; 2pp share check is insufficient — it cannot fail when cells are mechanically derived)
act-hours reject threshold = 15% deviation from target
Daily/PvP/Guild War hard progress gate = none (ambient channels ensure continuous completability)
Lv60 character EXP converts to nothing
```
