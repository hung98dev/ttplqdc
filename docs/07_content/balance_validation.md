# Launch Balance Validation
status: LOCKED

## Scope
Deterministic launch sanity checks for progression gaps, field/elite kill time, boss durability, party scaling, and incoming boss damage.

This file does **not** create hidden runtime scaling. It defines synthetic validation vectors computed from canonical stats/content so future content revisions cannot accidentally turn normal progression into a grind, make bosses evaporate before mechanics resolve, or create one-shot damage spikes.

Owning runtime/content rules remain in:
- `../01_gameplay/stats.md`
- `../01_gameplay/progression.md`
- `class_skill_catalog.md`
- `monster_catalog.md`
- `boss_catalog.md`
- `equipment_catalog.md`
- `progression_route.md`
- `../02_world/dungeons.md`

# Progression-Gap Benchmark
Act totals under `exp_required(L) = 10000 * L * L` (×100 scale):

| Act | act_exp_total | target_hours | FIELD_COMBAT 40% | DUNGEON_REPEAT 18% | WORLD_EVENT 12% | BOUNTY_REPEAT 12% | ELITE_BOSS 8% | LIFE_SKILL 5% | STORY_ONCE 5% |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| I | 3,850,000 | 15 | 1,540,000 | 693,000 | 462,000 | 462,000 | 308,000 | 192,500 | 192,500 |
| II | 24,850,000 | 75 | 9,940,000 | 4,473,000 | 2,982,000 | 2,982,000 | 1,988,000 | 1,242,500 | 1,242,500 |
| III | 65,850,000 | 260 | 26,340,000 | 11,853,000 | 7,902,000 | 7,902,000 | 5,268,000 | 3,292,500 | 3,292,500 |
| IV | 126,850,000 | 450 | 50,740,000 | 22,833,000 | 15,222,000 | 15,222,000 | 10,148,000 | 6,342,500 | 6,342,500 |
| V | 207,850,000 | 550 | 83,140,000 | 37,413,000 | 24,942,000 | 24,942,000 | 16,628,000 | 10,392,500 | 10,392,500 |
| VI | 272,850,000 | 650 | 109,140,000 | 49,113,000 | 32,742,000 | 32,742,000 | 21,828,000 | 13,642,500 | 13,642,500 |

Each row sums exactly to its act_exp_total; the six acts sum to 702,100,000.

Guardrails:
```text
REJECT if the independently computed channel EXP — derived from per-unit values in
          progression_route.md "Channel EXP Rate References" × unit frequency × channel hours
          (authored monster/dungeon/event EXP from owning catalogs) — deviates from
          the target channel EXP by more than 15% for any channel in any act.
          (The 2-percentage-point channel-share check is NOT a valid substitute: any value
          computed as act_total × channel_percentage satisfies it trivially and catches
          only authoring typos, not wrong EXP targets.)
REJECT if | derived_act_hours - target_act_hours | / target_act_hours > 0.15
```

Daily, Ranked PvP, and Guild War never count toward this benchmark.

Reject a revision when:
- any channel's EXP computed from independently authored per-unit values (see "Channel EXP Rate References" in `progression_route.md`) deviates from the target channel EXP by more than 15% in any act,
- any act's derived hours deviate from the target by more than 15%,
- a next-act level gate can only be reached by Daily/PvP/Guild War,
- a MAIN route introduces a rare-drop/slow-public-boss wait gate to fill EXP,
- optional repeat monster grinding becomes the only practical way to satisfy the next region's level requirement.

# Synthetic PvE Reference Build
Combat sanity tests use one deterministic reference vector per class/tier. This is a validation vector, not a required legal player build and not a gear-score gate.

For tier endpoint Levels `10/20/30/40/50/60`:
```text
potential earned = 4 * (level - 1) + bonus_potential(level)
bonus_potential(level) = 0 below 25
  Lv25 = 10, Lv30 = 20, Lv35 = 30, Lv40 = 40, Lv45 = 60, Lv50 = 80, Lv55 = 100, Lv60 = 120
offensive primary = floor(50% of earned)
VIT               = floor(25% of earned)
AGI               = remainder
```
At tier endpoints: Lv10=36, Lv20=76, Lv30=136, Lv40=196, Lv50=276, Lv60=356.
where offensive primary is:
```text
STR for KIM/THO
INT for MOC/THUY/HOA
```

Equipment contribution uses all 14 current-tier fixed base-stat lines from `equipment_catalog.md`, but deliberately excludes:
```text
secondary rolls
set bonuses
Support Signatures
Souls
Meridian
Formation
Guild Blessings
consumables
```
This isolates the stable base budget and avoids making lucky rolls/build collections mandatory for validation.

Reference enhancement:
```text
T1 +4
T2 +5
T3 +6
T4 +6
T5 +7
T6 +8
```
These are below aspirational enhancement and inside the intended story/normal progression bands.

Expected critical damage uses expectation rather than RNG:
```text
expected_crit_multiplier = 1 + CRIT_CHANCE * (CRIT_DAMAGE - 1)
```
Element multiplier is neutral `1.00` for benchmark purposes.

# Reference Basic Attack
Every basic-only benchmark and every basic used to fill rotation downtime uses the class `basic_1` (`kiem_thuc`, `linh_diep`, `thuy_tien`, `hoa_phu`, `tran_quyen`) at `skill_level = 1`: the reference vector spends no skill points on basics, so these benchmarks are a conservative floor. Its proc effects are ignored for TTK (no DoT ticks, no status multipliers). The "Current expected" ranges in this document are informative and are recomputed by the balance validator from this pin; only the "Release guardrail" ranges gate activation.

# Field NORMAL Benchmark
Use only the reference basic attack against a same-level NORMAL generated from `monster_catalog.md`.

All authoritative basic timings come from `class_skill_catalog.md`; ATTACK_SPEED applies normally.

Current expected endpoint range across all five classes and six tiers:
```text
~2.8s .. 4.8s basic-only TTK
```

Release guardrail:
```text
2.0s <= NORMAL same-level reference TTK <= 6.0s
```
This keeps ordinary enemies readable without turning traversal into damage-sponge play.

# ELITE Benchmark
Use the same basic-only vector against same-level ELITE stats.

Current expected endpoint range:
```text
~8.97s .. 20.60s basic-only TTK
```

Release guardrail:
```text
8.5s <= ELITE same-level reference TTK <= 24s
```
Elite mechanics therefore have time to appear while remaining short field encounters. The `8.5s` floor replaces the earlier rounded `9s` estimate so the canonical Level-10 KIM endpoint (`8.97s`) passes without changing the authored ELITE stat vector.

# Major-Boss Durability Benchmark
Use each tier's authored boss Level/DEFENSE and `boss_catalog.md` HP formula:
```text
MAX_HP = floor(10000 + 300*L + 16*L*L)
```

## Solo basic-only
Current expected range across tier-end reference classes:
```text
~57.7s .. 126.5s
```
Release guardrail:
```text
55s <= solo basic-only boss TTK <= 130s
```

This is intentionally a conservative floor test. Normal active-skill use should reduce TTK without collapsing the encounter to a few seconds.

This range replaces the earlier pre-cadence estimate of `~72s .. 121s`. The corrected window uses the pinned synthetic reference build, authored per-class basic cooldowns, and the canonical damage pipeline. It is a baseline-contract correction, not permission to widen guardrails when secondary-roll power exceeds its budget.

## Five-player PARTY_DEFAULT
For five identical reference players, canonical PARTY HP scaling is:
```text
boss HP = 3.20x solo HP
party outgoing throughput = 5x one-player throughput
```
so basic-only time is approximately:
```text
solo_TTK * 3.20 / 5 = solo_TTK * 0.64
```
Expected range:
```text
~36.9s .. 81.0s
```
Release guardrail:
```text
35s <= five-player basic-only boss TTK <= 85s
```

A boss whose ordinary five-player reference time falls below the floor is at risk of skipping authored mechanic cadence. Prefer boss-specific mechanic/HP correction over increasing player damage taken.

# Level-60 Rotation Benchmark
For an offensive-throughput check at Level 60:
- equip the five highest useful single-target damage actives available to that class; if fewer than five directly damage, use all damaging actives; fill all downtime with the reference basic attack,
- set those damaging actives to `skill_level = 10`, which costs at most `45` of the `75` earned Level-60 skill points (59 level-up + 12 books + 4 bonus),
- use authored cooldown/startup/active/recovery values,
- use `+8` T6 synthetic reference gear,
- exclude optional build-system damage bonuses.

Approximate launch reference windows after the boss-durability correction:
```text
boss.than_trung / ordinary Lv60 boss baseline:
  solo rotation TTK ~73s .. 111s
  five-player equivalent ~47s .. 71s

ENDGAME_L60 boss at 1.75x Lv60 boss HP:
  solo rotation TTK ~129s .. 194s
  five-player equivalent ~82s .. 124s
```

These are regression windows, not promises of exact player clear time. Movement, mechanics, missed uptime, defensive skills, build bonuses, class composition, and player skill change real encounters.

Release review should investigate when the deterministic simulator leaves these broad windows by more than `15%` without an intentional balance note.

# Incoming Boss-Damage Benchmark
Use the same synthetic reference build and a readable heavy boss hit of:
```text
1.40 * boss ATTACK
```
through canonical defense mitigation, before optional shields/DR.

Current tier-end reference result across all classes:
```text
~7.7% .. 10.5% MAX_HP per heavy hit
```

Launch guardrail for a normal readable heavy mechanic:
```text
6% .. 18% reference MAX_HP
```

The existing `boss_catalog.md` hard review remains stronger:
```text
>35% expected geared HP from one attack requires explicit readable tell/counterplay review
```
No baseline boss mechanic should become an unavoidable one-shot to make a durable encounter feel difficult.

# Class Spread Guardrail
Different classes intentionally trade damage, sustain, control, mobility, and defense. Validation should therefore check spread rather than force identical DPS.

For the same reference content:
```text
max basic-only boss TTK / min basic-only boss TTK <= 2.20
```
A larger spread requires explicit review because solo progression would otherwise punish a class choice through substantially longer mandatory fights. The `2.20` ceiling preserves the authored class cadence split while still rejecting drift beyond the current canonical result of approximately `2.0`.

Damage-role differences may remain visible; this guardrail is not a class ranking and does not require a trinity.

# New-Stat Power-Budget Validation (ADR-0037)
`LIFESTEAL`, `REFLECT`, `ABSORB`, `HEAL_REDUCTION`, and `HEALING_RECEIVED` enter the **existing** equipment secondary-roll pool and compete for existing roll slots. The number of secondary rolls per item does not increase. Total expected secondary-roll power per item must remain unchanged.

## Roll-Magnitude Budget Rule
Before activation of any content revision that adds or changes the magnitude of these secondary rolls:

1. Enumerate the updated secondary-roll pool entries and their authored magnitude ranges.
2. Compute the pre-change and post-change expected secondary-roll power-unit total per item using the same synthetic reference metric used for existing rolls (e.g., weighted average contribution to the reference build's effective DPS/survivability).
3. **Reject** if post-change expected total exceeds pre-change total by more than `1%` without an explicit intentional balance note that documents the approved deviation and records the revised expected values.

The guardrail numbers (TTK windows, survivability windows) are **never** to be widened to accommodate a roll-magnitude overshoot. If a window moves outside its guardrail after adding these stats, the roll magnitudes are wrong — fix the magnitudes.

## Sustain Benchmark — Lifesteal at the Pinned Reference
Pinned reference values (do not re-derive):
```text
reference_lv60_max_hp = 4794
HP_REGEN in combat    = 9.08 HP/s
LIFESTEAL_HPS_CAP     = 0.015 * attacker MAX_HP / s = 0.015 * 4794 ≈ 71.9 HP/s
heavy boss hit range  = 7.7% .. 10.5% MAX_HP = ~369 .. ~503 HP
boss DPS reference    = assume boss attacks approximately every 3.0s at 1.40x ATTACK
```

At full PvP-exempt PvE cap (`LIFESTEAL = 0.08`), against a capped 6,900 DPS synthetic KIM reference (from change order §3), the per-second heal before throttle would be `6900 * 0.08 = 552 HP/s`. The `LIFESTEAL_HPS_CAP` throttle cuts this to `~72 HP/s`, roughly 8× in-combat `HP_REGEN`. Combined total sustain: `~72 + 9.08 ≈ 81 HP/s`.

Against the documented heavy boss hitting for 7.7%–10.5% MAX_HP (≈369–503 HP) every ~3.0s, the boss's average DPS is approximately `123–168 HP/s`. Total lifesteal + regen sustain of `~81 HP/s` offsets roughly **48–66% of incoming boss DPS at the documented hit interval** — meaningful but not sufficient to become net-immortal.

**Lifesteal immortality guardrail:**
```text
REJECT if: (LIFESTEAL_HPS_CAP + reference_combat_HP_REGEN) >= documented_boss_average_DPS
           at the pinned reference (reference_lv60_max_hp = 4794, HP_REGEN = 9.08/s,
           boss heavy hit 7.7%–10.5% MAX_HP at canonical hit interval)
```
This guardrail must never be satisfied by a real build. If combined lifesteal+regen sustain reaches or exceeds the documented boss average DPS at the pinned reference, the roll magnitudes or `LIFESTEAL_HPS_CAP` is wrong — fix the source parameter, not the guardrail.

## TTK and Survivability Window Preservation Rule
After any content revision that grants or changes `LIFESTEAL`, `REFLECT`, `ABSORB`, or `HEAL_REDUCTION` values, the validation suite must re-run the full synthetic reference build (§ "Synthetic PvE Reference Build") with these stats set to zero (the reference build deliberately excludes secondary rolls). The TTK and survivability windows must still fall within their existing guardrails.

**Explicit reject rule — effective sustain or mitigation inflation:**
```text
REJECT UNCONDITIONALLY if adding LIFESTEAL/REFLECT/ABSORB/HEAL_REDUCTION to the
secondary-roll pool causes the synthetic reference build's effective sustain
(HP/s offset from incoming DPS) or effective mitigation (fraction of incoming damage
negated) to move outside the NORMAL TTK (2–6s), ELITE TTK (8.5–24s), boss solo TTK
(55–130s), boss five-player TTK (35–85s), or heavy-hit survivability (6–18% MAX_HP)
windows when measured at the reference build level.

If a window moves outside its guardrail:
  - the roll magnitudes are wrong — fix the magnitudes
  - the guardrail must NEVER be widened to accommodate them
  - this reject is not overridable by a content note
```

# Skill Reach and Camera-Readability Gate

ADR-0047 uses the ADR-0046 reference viewport (`25.6m x 14.4m`) and authoritative entity colliders. The activation tool must enumerate all 45 launch basic/active primary geometries and emit one row per skill with its role, authored dimensions, outer envelope, allowed band, and result.

Hard rejects:

```text
primary geometry row count != 45
projectile max_range_m + hit_radius_m > 8.8m
AREA_POSITION cast_range_m + radius_m > 11.0m
any primary dimension outside the role band in skills.md
minimum ranged-basic projectile range / maximum hostile melee reach < 2.50
any secondary spatial effect lacking typed geometry or deterministic resolution fields
any skill tag/effect displacement mismatch, including AIRBORNE
```

Launch audit reference:

```text
minimum ranged-basic projectile range = 7.5m
maximum hostile melee/single-target reach = 2.6m
separation ratio = 7.5 / 2.6 = 2.8846...
maximum positioned outer reach = 11.0m
camera half-width margin = 12.8 - 11.0 = 1.8m
```

The tool must also run exact-boundary fixtures for every ADR-0046 hurtbox profile (`CHARACTER`, normal/elite ground/floating, and boss profiles): intersection or a `0.001m` surface gap hits under the physics contact epsilon; a quantized `0.002m` gap misses. Sprite pixels, transparent padding, pivot placement, and render scale never change the result.

# Validation Output
Automated tooling should emit at least:
```text
revision
class_id
tier/level
reference ATTACK/DEFENSE/MAX_HP/CRIT/ATTACK_SPEED
NORMAL TTK
ELITE TTK
boss solo TTK
boss five-player TTK
Lv60 rotation TTK where applicable
heavy-hit HP ratio
progression remaining-gap ratio by act
LIFESTEAL secondary-roll expected magnitude (if present in pool)
REFLECT secondary-roll expected magnitude (if present in pool)
ABSORB secondary-roll expected magnitude (if present in pool)
HEAL_REDUCTION secondary-roll expected magnitude (if present in pool)
secondary-roll pool expected power delta vs pre-change baseline
lifesteal+regen sustain vs boss average DPS ratio at pinned reference
skill geometry row count/pass count
skill reach role/envelope/band/result per launch action
minimum ranged/basic-to-hostile-melee reach ratio
maximum positioned outer reach and reference-camera margin
hurtbox boundary fixture result per entity profile
```

A failed balance gate rejects activation until the content change is intentionally retuned or the owning guardrail is deliberately revised with documentation.

# Invariants
```text
balance validation creates no runtime auto-scaling
channel share reject: per-unit cross-check > 15% deviation from target per channel per act (see progression_route.md Channel EXP Rate References)
act-hours reject: deviation > 15% from target act hours
NORMAL basic-only TTK target = 2..6s
ELITE basic-only TTK target = 8.5..24s
boss solo basic-only TTK target = 55..130s
boss five-player basic-only TTK target = 35..85s
normal heavy boss hit target = 6..18% reference HP
class mandatory-boss basic-TTK spread <= 2.20
Lv60 rotation benchmark uses 75 total skill points (59 + 12 + 4)
Daily/PvP/Guild War not required for leveling or PvE affordability
new-stat secondary rolls enter existing pool; roll count per item unchanged
secondary-roll pool expected power delta <= +1% without intentional balance note
guardrails are never widened to accommodate roll-magnitude overshoot
lifesteal immortality reject: (LIFESTEAL_HPS_CAP + combat_HP_REGEN) < boss_avg_DPS at pinned ref
effective sustain/mitigation inflation -> roll magnitudes are wrong -> reject
pinned reference: reference_lv60_max_hp = 4794, HP_REGEN = 9.08/s, heavy boss hit 7.7..10.5% MAX_HP
skill primary geometry rows = 45 and every row passes its role band
projectile envelope <= 8.8m; positioned-circle outer reach <= 11.0m
ranged-basic minimum / hostile-melee maximum >= 2.50
reference-camera horizontal telegraph margin >= 1.8m
range resolves against authoritative hurtbox intersection, never sprite/pivot/client distance
```
