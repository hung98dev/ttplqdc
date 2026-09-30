# Crafting
status: LOCKED

## Scope
Defines recipes, atomic crafting, probability, batch crafting, and equipment enhancement `+0..+16`.

## Authority / Access
Recipes, costs, RNG and mutation are server-authoritative. Normal crafting/enhancement uses configured NPC/service stations and is rejected while `in_combat`. Portable crafting is disabled.

## Recipes
Every `recipe_id` defines requirements, inputs, outputs, success mode, and optional currency cost. Normal item recipes are GUARANTEED. Recipe unlocks may be always, quest, progression, NPC service, or temporary event.

## Atomic Craft
Validate all inputs/currency/requirements/locks/output capacity before consumption. Success commits all; failure commits none. Equipment output gets one new instance, one persistent stat roll, and +0 unless explicit.

Guaranteed batch size = `1..99`, all-or-nothing. Probabilistic batch crafting is disabled.

## Probability
Use integer basis points `0..10000`; RNG is server-owned.

# Enhancement
Range `+0..+16`; success always gains exactly +1.

## Milestone Floors
```text
0..3 -> 0
4..7 -> 4
8..11 -> 8
12..15 -> 12
16 -> 16
```

## Base Success Rates
| Attempt | Success Rate (%) | Basis Points (`bp`) |
|---|---:|---:|
| +0 -> +1 | 100% | 10,000 |
| +1 -> +2 | 100% | 10,000 |
| +2 -> +3 | 85% | 8,500 |
| +3 -> +4 | 70% | 7,000 |
| +4 -> +5 | 55% | 5,500 |
| +5 -> +6 | 45% | 4,500 |
| +6 -> +7 | 35% | 3,500 |
| +7 -> +8 | 25% | 2,500 |
| +8 -> +9 | 20% | 2,000 |
| +9 -> +10 | 15% | 1,500 |
| +10 -> +11 | 10% | 1,000 |
| +11 -> +12 | 8% | 800 |
| +12 -> +13 | 6% | 600 |
| +13 -> +14 | 4% | 400 |
| +14 -> +15 | 3% | 300 |
| +15 -> +16 | 2% | 200 |
## Failure
On failure without insurance:
```text
new_level = max(current_level - 1, enhancement_floor(current_level))
```
Failure drops the item by 1 level, clamped at its current milestone floor (Floor 0: 0..3; Floor 4: 4..7; Floor 8: 8..11; Floor 12: 12..15). In particular, a failed `+13 -> +14`, `+14 -> +15`, or `+15 -> +16` returns to the preceding level; only a failed `+12 -> +13` remains at its +12 milestone floor.

On failure with insurance:
```text
new_level = current_level
```
Enhancement level is strictly preserved.

## Lucky Charm (Bùa May Mắn)
Players may optionally use a Lucky Charm to boost enhancement success rate under ADR-0022. Exactly **one lucky charm of one type** may be consumed per attempt; stacking multiple lucky charms, or using a charm outside its level eligibility, is rejected with `CHARM_INELIGIBLE` before anything is consumed:

| Charm Tier | Item ID | Bonus | Level Eligibility |
|---|---|:---:|---|
| Sơ Cấp | `item.consumable.bua_may.so_cap` | **+5%** (+500 bp) | Current level `< +8` (attempts `+0->+1` .. `+7->+8`) |
| Trung Cấp | `item.consumable.bua_may.trung_cap` | **+3%** (+300 bp) | Current level `< +12` (attempts `+0->+1` .. `+11->+12`) |
| Cao Cấp | `item.consumable.bua_may.cao_cap` | **+1%** (+100 bp) | All levels (`+0->+1` .. `+15->+16`) |
| Siêu Cấp | `item.consumable.bua_may.sieu_cap` | **+3%** (+300 bp) | All levels (`+0->+1` .. `+15->+16`) |

Final success chance after lucky bonus and guild blessing is clamped at:
```text
min(base_rate + blessing_bonus + charm_bonus, 9500)
```
Where `blessing_bonus` (from Guild Blessing, see below) is applied before `charm_bonus`. Consumed upon attempt execution regardless of outcome.

## Insurance (Bùa Giữ Bậc)
Players may optionally use an Insurance Charm to prevent level downgrade upon failure under ADR-0022. Exactly **one insurance charm of one type** may be consumed per attempt:

| Insurance Tier | Item ID | Function | Level Eligibility |
|---|---|---|---|
| Sơ Cấp | `item.consumable.bua_giu_bac.so_cap` | Preserves level on failure | Current level `< +8` (attempts `+0->+1` .. `+7->+8`) |
| Trung Cấp | `item.consumable.bua_giu_bac.trung_cap` | Preserves level on failure | Current level `< +12` (attempts `+0->+1` .. `+11->+12`) |
| Cao Cấp | `item.consumable.bua_giu_bac.cao_cap` | Preserves level on failure | All levels (`+0->+1` .. `+15->+16`) |

An enhancement attempt may combine at most 1 eligible Lucky Charm and 1 eligible Insurance item. Consumed on attempt execution regardless of outcome.

## Guild Blessing (Phúc Lành Hội)
When an active Guild Blessing buff is in effect for the character at attempt time:
```text
blessing_bonus = +300 bp
```
`blessing_bonus` is applied AFTER `base_rate` and BEFORE `charm_bonus` in the canonical success clamp. The full clamp order for pity-eligible attempts is:
```text
min(base_rate + blessing_bonus + charm_bonus + pity, 9500)
```
For non-pity attempts (`+0..+12`):
```text
min(base_rate + blessing_bonus + charm_bonus, 9500)
```
Guild Blessing is a guild-server-side buff flag; it is not a consumable item and is not deducted from any currency. It cannot be stacked with a second simultaneous Guild Blessing. The `blessing_bonus` cannot push the final rate above 9500 bp on its own.

## Soft Pity for +13..+16 (Bao Ho Mem)
For attempts targeting `+13`, `+14`, `+15`, `+16` (current level 12..15), server tracks consecutive failures per `item_instance_id + target_level` (canonical key per ADR-0028 §Pity; `character_id` is not part of the pity key):

- After `5` consecutive failures at the same target level, the next attempt at that target level receives `+1%` (+100 bp) pity bonus.
- Pity stacks up to `+5%` (+500 bp) after 9 consecutive failures (5→+1%, 6→+2%, 7→+3%, 8→+4%, 9→+5%).
- Pity bonus is added after `base_rate + blessing_bonus + charm_bonus` before the 95% clamp, so max final rate is `min(base_rate + blessing_bonus + charm_bonus + pity, 9500)`.
- Pity resets to `0` immediately after a success at that target level; failures at other target levels do not affect this counter.
- Pity state is persisted as four independently addressable records keyed by `item_instance_id + target_level` (`target_level in {13,14,15,16}`), each with `pity_fail_count` 0..9; `pity_bonus_bp` is derived, never independently mutable. Pity state moves with the item through permitted ownership transfer and cannot be reset by trade/auction.
- Pity does not apply to `+0..+12` attempts.

### Enchanting Stat Multiplier by Enhancement Level
The following table shows the enchanting-stat multiplier applied at each level. Only explicit `enhanceable_stats` use this multiplier; base rolls never reroll.

```text
+0 0%, +1 2%, +2 4%, +3 6%, +4 8%, +5 11%, +6 14%, +7 17%, +8 20%,
+9 24%, +10 28%, +11 32%, +12 36%, +13 41%, +14 46%, +15 51%, +16 56%
```

## Costs
Each equipment tier defines:
```text
base_enhancement_material_units
base_enhancement_common
```
Attempt cost is based on current enhancement level `L` using the following **explicit tables**, not exponential formulas.

### Material multiplier
| Current +L | multiplier |
|---:|---:|
| 0 | 1 |
| 1 | 1 |
| 2 | 2 |
| 3 | 2 |
| 4 | 3 |
| 5 | 3 |
| 6 | 4 |
| 7 | 5 |
| 8 | 6 |
| 9 | 8 |
| 10 | 10 |
| 11 | 12 |
| 12 | 15 |
| 13 | 18 |
| 14 | 22 |
| 15 | 28 |

### Common-currency multiplier
| Current +L | multiplier |
|---:|---:|
| 0 | 1 |
| 1 | 2 |
| 2 | 3 |
| 3 | 4 |
| 4 | 6 |
| 5 | 8 |
| 6 | 12 |
| 7 | 16 |
| 8 | 22 |
| 9 | 30 |
| 10 | 40 |
| 11 | 55 |
| 12 | 75 |
| 13 | 100 |
| 14 | 135 |
| 15 | 180 |

Attempt cost:
```text
material_cost = base_enhancement_material_units * material_multiplier[L]
common_cost   = base_enhancement_common * common_currency_multiplier[L]
```

Attempt costs scale aggressively at higher tiers to create a durable, healthy economic sink for materials and currency without making early-game progression prohibitive.

### Expected-Cost Reference
With Insurance applied for attempts +8..+11 **and the canonical per-target soft-pity state machine applied for +13..+16**, expected cumulative multiplier units from +0 are approximately:

| Target | material units | common-base units | Notes |
|---:|---:|---:|---|
| +6 | 27.26 | 56.28 | no Insurance assumed |
| +8 | 192.02 | 488.53 | Insurance for +8..+11 assumed |
| +10 | 275.35 | 798.53 | Insurance for +8..+11 assumed |
| **+11** | **375.35** | **1,198.53** | Insurance; 10 expected attempts at L=10 |
| +12 | 525.35 | 1,886.03 | Insurance for +8..+11 assumed |
| **+13** | **701.15** | **2,765.03** | no Insurance; pity applied (+12 floor); ~11.72 expected attempts |
| +16 | 785,781.85 | 3,979,515.15 | pity applied to +13..+16; **TERMINAL GOLD DESTINATION** |

These values are validation references, not persisted runtime state. Tooling computes expectation in full precision and compares each displayed value after rounding to two decimal places; a displayed reference differs only when absolute full-precision drift exceeds `0.005` multiplier units.

The **+16 enhancement of a T6 item is the official terminal destination for `currency.common`**. At `REFERENCE_ENDGAME_COMMON_PER_HOUR` (76,500, canonical in `../07_content/economy_catalog.md`) the expected full T6+16 cost (994,878,788 common, approximately 0.995B; derived as 3,979,515.15 common-base units × T6 base 250 = 994,878,787.5, rounded) represents approximately 13,000 hours of field income (994,878,788 / 76,500 = 13,005) — a genuine lifetime goal, not a casual target. Its role is to act as a permanent, deep, unambiguous gold sink that removes excess currency without creating a pay-to-win advantage.

Derivation arithmetic (T6, +12→+13 with pity):
```text
Pity Markov: P(success) at fail_count f = min(0.06 + max(0, f-4)*0.01, 0.11)
E[attempts from pity=0]:
  E_9 = 1/(0.11)               = 9.09
  E_8 = 1 + 0.90 * E_9         = 9.18
  E_7 = 1 + 0.91 * E_8         = 9.35
  E_6 = 1 + 0.92 * E_7         = 9.60
  E_5 = 1 + 0.93 * E_6         = 9.93
  E_4 = 1 + 0.94 * E_5         = 10.33
  E_3 = 1 + 0.94 * E_4         = 10.71
  E_2 = 1 + 0.94 * E_3         = 11.07
  E_1 = 1 + 0.94 * E_2         = 11.41
  E_0 = 1 + 0.94 * E_1         = 11.72 expected attempts
Expected cost +12->+13: 11.72 * 75 = 879.00 common-base units (material: 11.72 * 15 = 175.80)
```

### Pacing Guardrail
Launch tuning intent:
```text
+0..+6   = normal story/progression investment
+7..+8   = meaningful but routine build investment
+9..+10  = late-normal optimization
+11..+12 = max-level/endgame optimization
+13      = first aspirational tier; first serious gold sink beyond normal endgame
+14..+15 = long-term character investment; well into post-cap play
+16      = lifetime terminal goal; official terminal destination for currency.common
```

The +10→+11→+12→+13 staircase forms a continuous intermediate sink ladder that absorbs meaningful gold before the dramatic jump to the +16 terminal goal. A player who can afford +12 endgame gear will naturally encounter the +13 gate as the next sink without needing an artificial transition cliff. Enhancement above +16 is not defined; +13..+16 each have soft pity (Bảo Hộ Mềm) as defined in §Soft Pity — no additional shortcut mechanics apply beyond pity.

Balance validation must evaluate **expected cost including failures/downgrades**, not only the price of one successful attempt. Do not tune story/endgame around every one of 14 ACTIVE pieces reaching +12/+16.

A content revision should be rejected for release review when simulated expected cost from a tier's normal reward sources makes its intended enhancement band materially unreachable without Auction dependence or repetitive farming far beyond that progression band's target playtime.

## Equipped Enhancement
Enhancement may target owned equipment in any loadout when not in combat and not transaction-locked. This exception does not permit normal crafting consumption of equipped items.

## Visual Prestige & Broadcast
Enhancing equipment to high milestones unlocks authoritative visual prestige and server notifications under ADR-0023:
- **Weapon Glow & Aura (Client Presentation)**:
  - `+8..+9`: Soft cyan/blue sheen (Lam Quang).
  - `+10..+11`: Pulsing mystical purple aura (Tử Quang).
  - `+12..+13`: Radiant golden beam (Hoàng Kim Quang) signaling endgame mastery.
  - `+14..+15`: Crackling elemental arcs orbiting the weapon profile.
  - `+16`: Mythic Dragon/Lotus aura (Thần Binh Kim Hộ Thể) enveloping the entire character model.
- **World Broadcast**:
  - When an item successfully achieves `+16`, the server emits an authoritative global announcement banner across all world channels (`loc.notice.enhancement_plus_16_broadcast`).

## Idempotency
Every attempt uses stable `craft_operation_id`; one accepted attempt executes one RNG result exactly once. No timed queue, post-commit cancellation, or stat reroll system initially.

## Invariants
```text
0 <= enhancement_level <= 16
success -> +1
failure -> floor-bounded downgrade unless insured
soft pity +1% per fail after 5 fails at same +13..+16 target, max +5%
success rate clamp = min(base_rate + blessing_bonus + charm_bonus + pity, 9500)
blessing_bonus applied before charm_bonus, pity applied last
equipment never destroyed
attempt-cost multipliers are explicit tables
cost review includes failure probability, not just per-attempt price
+13..+16 never required for baseline content
+16 T6 = official terminal gold destination (~13,000 hours at REFERENCE_ENDGAME_COMMON_PER_HOUR; ~994,878,788 common)
one operation -> one RNG result
```
