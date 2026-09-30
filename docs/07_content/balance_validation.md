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

Two separate fixtures are mandatory:
1. **Seven-channel calibration**: compute each channel from independent per-unit owning-catalog EXP × authored frequency × portfolio channel hours; reject >15% deviation for each original channel/act. BOUNTY calibrates active-time rate only; this is not a calendar simulation and grants no uncapped daily rewards. WORLD_EVENT uses one eligible completion/hour at `character_act`: region0 always, two rotating regions, highest unlocked active region, no access bypass.
2. **Zero-bounty continuous feasibility**: no Daily, Ranked PvP or Guild War EXP. Substitute FIELD/DUNGEON shares `48.275862%/21.724138%` per `progression_route.md`, leave all other shares unchanged, run actual accessible routes/ambient rates and require each act total/gate. Do not check zero BOUNTY against its calibration target or original FIELD/DUNGEON shares. Reject aggregate EXP or derived act-hours drift >15%, or calendar-reset/locked-map dependency.

Both fixtures use independently authored unit EXP, never `act_total * target_share` as measured output. Reject when:
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
Use the +8 T6 synthetic reference, no optional build layers. Exact selected active IDs in scheduling priority order:

| class | active_priority |
|---|---|
| KIM | `nhat_kiem_dinh_hon,kiem_tran,pha_giap,hoi_kiem,xuyen_phong` |
| MOC | `van_moc_hoi_sinh,van_doc,thanh_dang,moc_bo` |
| THUY | `thien_ha,han_trieu,trieu_quyen` |
| HOA | `cuu_hoa_lien,hoa_vuc,lien_bao,boc_bo` |
| THO | `thien_son_tran,dia_chan,thach_kich` |

All listed actives Lv10; equipped basic_1 Lv1, passives unlearned, remaining slots empty. Target is an immortal stationary boss hurtbox of the actual boss profile, center `(1.0m,0)` relative to stationary caster origin `(0,0)`; caster faces +X, positioned casts use target center. Neutral element, no outgoing boss attacks/control/dodge, player full HP/MP, cooldowns ready at tick0, no statuses. MP regen ticks at t=1000ms then every1000ms using in-combat reference rate; basic_1 connected primary restores2MP. Damage/status/zone/projectile resolution and all clocks use runtime formulas, including cast travel and expiry; never assume instantaneous projectile hits.

At each50ms tick resolve due effects and regen in canonical order, then if actor is free and current basic self-chain/deadlines permit, choose first ready affordable active from the fixed priority; otherwise choose basic if eligible; otherwise wait. An unaffordable active is skipped, not resource-borrowed; every accepted action spends actual MP, snapshots timing and starts cooldown once. Actives do not cancel recovery; same-basic recovery self-chain is the sole exception. Stop on first boss-HP zero commit. Five-player fixture has five identical actors and stable IDs1..5 sharing the boss, each initialized identically; actual multi-source DOTs coexist.

Emit tick/action-ID/accepted-ms/active-due/recovery-due/complete-due/next-accept-tick/MP-before/MP-cost/MP-after/cooldown-ready/committed-HP-damage trace. Same revision/input must produce byte-identical canonical trace and TTK; cost, priority or schedule mutation must change the appropriate trace, not a private alternative scheduler.

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
The 12-type secondary pool competes for unchanged per-item roll slots. The accepted 8→12 expansion may reduce expected power; it must not inflate it. Equality of raw ratios is not a common-unit power proof.

## Roll-Magnitude Budget Rule
Before activation of any content revision that adds or changes the magnitude of these secondary rolls:

1. Enumerate old8 (`ATTACK,DEFENSE,MAX_HP,MAX_MP,CRIT_CHANCE,ATTACK_SPEED,CAST_SPEED,COOLDOWN_REDUCTION`) and current12 (plus LIFESTEAL,REFLECT,ABSORB,HEAL_REDUCTION), preserving the old8 authored ranges as the comparison baseline.
2. Use exact common power units: `power(x)=10000*x/reference_stat` for flat stats (ATTACK724,DEFENSE426,MAX_HP4794,MAX_MP859); utility `power(x)=10000*x/global_PvE_cap` (CRIT_CHANCE0.60,ATTACK_SPEED0.50,CAST_SPEED0.50,COOLDOWN_REDUCTION0.35,LIFESTEAL0.08,REFLECT0.15,ABSORB0.10,HEAL_REDUCTION0.30). This is a declared budget-utilization metric, not a claim that one utility point equals DPS. Uniform inclusive integer/bp draws use exact rational midpoint; `E_item=K/N*sum(E_power(type))`. Compute per tier with K1/2; no caps/gear floors on this marginal metric. Old8 baseline uses the same denominators. Runtime-sensitive tests below remain independent.
3. Reject current expected power >old8*1.01 without an explicit approved balance note with numeric before/after; no note can waive runtime guardrails. Report decreases explicitly; do not call equal roll count a power proof.

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
Run two independent fixtures: (A) the no-roll reference unchanged, proving base-budget guardrails, and (B) a roll-sensitive marginal fixture. For B enumerate each type and every legal magnitude (integer/bp values inclusive), add that roll to each of the14 reference slots separately, average its actual runtime-derived damage/defense/resources/sustain contribution uniformly, and weight K/N per tier. Also run fixed deterministic stress loadouts with each type at maximum on every eligible slot (no duplicate type per item; other roll slot empty). Sustain scenario: boss hits reference every3s, player attacks per pinned scheduler, target has healing each3s so HEAL_REDUCTION is measurable; no stochastic RNG, expected crit and hit probabilities. Report per-type before/after outputs and reject the same hard combat/sustain windows, never widen them. Perturb an ATTACK-flat endpoint and each new utility endpoint by one legal unit and require its marginal output or budget metric to change; A intentionally remains unchanged. Only B/metric may be repaired by roll retuning.

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
