# ADR-0022: Multi-Tier Lucky and Insurance Charms Enhancement Contract
status: ACCEPTED

## Context
Equipment enhancement previously used a single generic Lucky Charm item and a single generic Insurance item valid only for +8..+11. With the hardcore progression curve anchored at 2% for +16 (ADR-0021), enhancement needs granular support items across early, mid, and endgame tiers.

Enhancement design requires:
1. Four distinct tiers of Lucky Charms (Bùa May Mắn) with explicit level eligibility and percentage bonuses.
2. Three distinct tiers of Insurance Charms (Bùa Giữ Bậc) preventing level downgrade on failure.
3. Strict single-charm-per-slot usage rules.

## Decision
1. **Four Lucky Charm Tiers (Bùa May Mắn)**:
   Increase success probability; consumed upon attempt execution:

| item_id | Name (vi-VN) | English | Success Bonus | Level Eligibility |
|---|---|---|:---:|---|
| `item.consumable.bua_may.so_cap` | Bùa May Mắn (Sơ Cấp) | Lesser Lucky Charm | **+5%** (+500 bp) | Current level `< +8` (attempts `+0->+1` .. `+7->+8`) |
| `item.consumable.bua_may.trung_cap` | Bùa May Mắn (Trung Cấp) | Medium Lucky Charm | **+3%** (+300 bp) | Current level `< +12` (attempts `+0->+1` .. `+11->+12`) |
| `item.consumable.bua_may.cao_cap` | Bùa May Mắn (Cao Cấp) | Greater Lucky Charm | **+1%** (+100 bp) | All levels (`+0->+1` .. `+15->+16`) |
| `item.consumable.bua_may.sieu_cap` | Bùa May Mắn (Siêu Cấp) | Supreme Lucky Charm | **+3%** (+300 bp) | All levels (`+0->+1` .. `+15->+16`) |

2. **Three Insurance Tiers (Bùa Giữ Bậc)**:
   Prevent level downgrade upon failure (`new_level = current_level`); consumed upon attempt execution:

| item_id | Name (vi-VN) | English | Function | Level Eligibility |
|---|---|---|---|---|
| `item.consumable.bua_giu_bac.so_cap` | Bùa Giữ Bậc (Sơ Cấp) | Lesser Insurance | Preserves level on failure | Current level `< +8` (attempts `+0->+1` .. `+7->+8`) |
| `item.consumable.bua_giu_bac.trung_cap` | Bùa Giữ Bậc (Trung Cấp) | Medium Insurance | Preserves level on failure | Current level `< +12` (attempts `+0->+1` .. `+11->+12`) |
| `item.consumable.bua_giu_bac.cao_cap` | Bùa Giữ Bậc (Cao Cấp) | Greater Insurance | Preserves level on failure | All levels (`+0->+1` .. `+15->+16`) |

3. **Single Charm Usage Contract**:
   - Each enhancement attempt accepts **at most 1 Lucky Charm** and **at most 1 Insurance Charm**.
   - Exactly **1 item** is consumed from each populated charm slot per executed attempt.
   - Players cannot stack multiple charms in the same slot or combine multiple tiers of the same charm type.

4. **Failure & Downgrade Behavior**:
   - Milestone floors: `0` (+0..+3), `4` (+4..+7), `8` (+8..+11), `12` (+12..+15), `16` (+16).
   - **Without Insurance**:
     `new_level = max(current_level - 1, enhancement_floor(current_level))`
     Failure drops the item by 1 level, clamped at the current milestone floor (e.g., failure at +13 drops to +12; failure at +7 drops to +6; failure at +5 drops to floor +4).
   - **With Insurance**:
     `new_level = current_level` (level is strictly preserved on failure).

5. **Level Eligibility Validation**:
   - The server validates charm level eligibility before executing any RNG roll or consuming resources. If an item exceeds the charm's allowed ceiling (e.g. attempting to use `so_cap` on a `+10` item), the request is rejected immediately with a validation error.

6. **Clamping Rule**:
   - Final success chance after Lucky Charm bonus is clamped at `min(base_rate + charm_bonus, 9500)` (95% maximum ceiling).

## Consequences
- Protects item progression across all tiers through deliberate economic choices.
- Elevates the value of high-tier materials and dungeon drops (Cao Cấp / Siêu Cấp charms).
- Provides deterministic validation boundaries that prevent client injection of stacked or tier-incompatible charms.
