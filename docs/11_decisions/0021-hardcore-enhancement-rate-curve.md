# ADR-0021: Hardcore Enhancement Success Curve Anchored at 2% for +16
status: ACCEPTED

## Context
The initial equipment enhancement rates were relatively generous (30% at +15->+16, 45% at +11->+12), making high enhancement tiers easily attainable and reducing the long-term prestige, economic sink value, and excitement of achieving +16 endgame gear in a 2,000-hour progression MMORPG.

Enhancement design requires a steeper, hardcore progression curve anchored at exactly 2% success rate for +15->+16, with corresponding downward scaling across intermediate levels.

## Decision
1. **Canonical Base Success Rates (+0..+16)**:
   Integer basis points (`bp`, 0..10000):

| Transition | Success Rate (%) | Basis Points (`bp`) |
|---|---:|---:|
| `+0 -> +1` | 100% | 10,000 |
| `+1 -> +2` | 100% | 10,000 |
| `+2 -> +3` | 85% | 8,500 |
| `+3 -> +4` | 70% | 7,000 |
| `+4 -> +5` | 55% | 5,500 |
| `+5 -> +6` | 45% | 4,500 |
| `+6 -> +7` | 35% | 3,500 |
| `+7 -> +8` | 25% | 2,500 |
| `+8 -> +9` | 20% | 2,000 |
| `+9 -> +10` | 15% | 1,500 |
| `+10 -> +11` | 10% | 1,000 |
| `+11 -> +12` | 8% | 800 |
| `+12 -> +13` | 6% | 600 |
| `+13 -> +14` | 4% | 400 |
| `+14 -> +15` | 3% | 300 |
| `+15 -> +16` | 2% | 200 |
2. **Escalating Attempt Cost Multipliers**:
   Attempt cost increases more aggressively with level `L` to provide a deep, healthy resource and currency sink:

   - **Material Multipliers (`material_multiplier[L]`)**:
     - `+0..+1`: 1
     - `+2..+3`: 2
     - `+4..+5`: 3
     - `+6`: 4, `+7`: 5
     - `+8`: 6, `+9`: 8, `+10`: 10, `+11`: 12
     - `+12`: 15, `+13`: 18, `+14`: 22, `+15`: 28

   - **Common-Currency Multipliers (`common_currency_multiplier[L]`)**:
     - `+0`: 1, `+1`: 2, `+2`: 3, `+3`: 4
     - `+4`: 6, `+5`: 8, `+6`: 12, `+7`: 16
     - `+8`: 22, `+9`: 30, `+10`: 40, `+11`: 55
     - `+12`: 75, `+13`: 100, `+14`: 135, `+15`: 180

3. **Milestone Floor Invariant**:
   Milestone floors remain locked to protect baseline player progress against catastrophic loss:
   - Floor 0: `+0..+3 -> 0`
   - Floor 4: `+4..+7 -> 4`
   - Floor 8: `+8..+11 -> 8`
   - Floor 12: `+12..+15 -> 12`
   - Floor 16: `+16 -> 16`
   On failure without insurance:
   - Levels `+0..+11`: `new_level = max(current_level - 1, enhancement_floor(current_level))`.
   - Levels `+12..+15`: use the same floor-bounded transition. A failed `+12 -> +13` remains at +12; failures from +13/+14/+15 downgrade by one level, never below +12.

4. **Lucky Charm & Insurance**: Exact tiers, eligibility and consumption are canonical in `../03_systems/crafting.md`: one eligible Lucky and one eligible Insurance may each be consumed on an executed attempt. Lucky is Sơ Cấp `+500bp` below +8, Trung Cấp `+300bp` below +12, Cao Cấp `+100bp` all levels, or Siêu Cấp `+300bp` all levels; Insurance preserves the current level on failure at its authored eligibility. No alternate ADR-local charm table exists.

5. **Expected Cumulative Multiplier Units (Reference)**: Values include Insurance on +8..+11 and the canonical per-target soft-pity state machine at +13..+16; they are not a no-pity or permanently-max-pity estimate.
   - Target +6: material ~27.26, common-base ~56.28
   - Target +8: material ~192.02, common-base ~488.53
   - Target +10: material ~275.35, common-base ~798.53
   - Target +12: material ~525.35, common-base ~1,886.03
   - Target +16: material ~785,781.85, common-base ~3,979,515.15
## Consequences
- Makes +12 a respected endgame milestone and +16 a legendary, aspirational achievement.
- Deepens the economic sink for materials and `currency.common` across the 2,000-hour progression lifecycle.
- Fully respects the baseline rule: story and normal content never require +16 equipment.
