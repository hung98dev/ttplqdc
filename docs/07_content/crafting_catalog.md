# Launch Crafting Catalog
status: LOCKED

## Scope
Concrete guaranteed launch recipes needed to make every equipment set reachable without auction/RNG dependency. Mechanical crafting/enhancement behavior remains canonical in `../03_systems/crafting.md`.

No timed crafting queue, profession level, recipe rarity, or crafting skill tree is added.

# Equipment Recipe Expansion
Every one of the 168 equipment items from `equipment_catalog.md` has exactly one guaranteed crafting recipe:
```text
item:   item.eq.<tier>.<set_key>.<slot>
recipe: recipe.eq.<tier>.<set_key>.<slot>
```
This finite expansion is normative and creates exactly 168 equipment recipes.

All equipment recipes:
```text
success_mode = GUARANTEED
output_quantity = 1
enhancement_level = +0
binding on created item = definition default (UNBOUND until ON_EQUIP)
```
Output capacity is validated before inputs/currency are consumed.

# Tier Material Mapping
| Tier | material_id | minimum level | material base | common-currency base |
|---|---|---:|---:|---:|
| T1 | `item.material.lang_da.manh_dong` | 1 | 3 | 100 |
| T2 | `item.material.u_minh.vo_cay` | 11 | 4 | 250 |
| T3 | `item.material.ben_nuoc.da_song` | 21 | 5 | 600 |
| T4 | `item.material.deo_may.da_voi` | 31 | 6 | 1200 |
| T5 | `item.material.thanh_co.gach_co` | 41 | 8 | 2200 |
| T6 | `item.material.nui_thieng.da_suong` | 51 | 10 | 3500 |

The old `500/1200/2500/5000/9000/15000` common bases were too high relative to authored quest/combat faucets. They made even deterministic crafting feel like an Auction/grind gate despite the catalog's stated purpose.

# Slot Cost Weight
| slot | weight |
|---|---:|
| weapon | 5 |
| head | 3 |
| body | 5 |
| hands | 3 |
| legs | 4 |
| feet | 3 |
| necklace | 3 |
| ring | 2 |
| costume | 4 |
| talisman | 3 |
| jade | 3 |
| seal | 4 |
| relic | 3 |
| charm | 2 |

For an equipment recipe:
```text
material_quantity = tier_material_base * slot_weight
common_currency_cost = tier_common_currency_base * slot_weight
```
No other ingredient is required.

Examples:
```text
recipe.eq.t1.dinh_lang.weapon
  -> 15 item.material.lang_da.manh_dong + 500 common

recipe.eq.t6.dau_cu.ring
  -> 20 item.material.nui_thieng.da_suong + 7000 common
```

The 14 canonical slot weights sum to `47`. Therefore crafting one complete 14-piece set from scratch costs:

| Tier | regional material | common |
|---|---:|---:|
| T1 | 141 | 4,700 |
| T2 | 188 | 11,750 |
| T3 | 235 | 28,200 |
| T4 | 282 | 56,400 |
| T5 | 376 | 103,400 |
| T6 | 470 | 164,500 |

A full set is an optional reference budget, not an expected act requirement. Direct drops and first-clear pieces intentionally reduce real player cost.

# Recipe Availability
Recipes become available at the tier's minimum level. No dungeon first-clear is required to craft baseline gear.

Direct dungeon/elite/boss equipment drops are accelerators and alternate acquisition, not the only path.

T6 crafting therefore begins at Level 51; the story/finale does not require a pre-existing `boss.than_trung` drop to become viable.

# Lucky Charm Recipes
Recipes produce tier-appropriate Lucky Charms under ADR-0022:
- `t1..t2` produces `item.consumable.bua_may.so_cap`
- `t3..t4` produces `item.consumable.bua_may.trung_cap`
- `t5` produces `item.consumable.bua_may.cao_cap`
- `t6` produces `item.consumable.bua_may.sieu_cap`

Recipe ID:
```text
recipe.utility.bua_may.<tier>
```
Input:
```text
6 * mapped regional material
common currency = tier_common_currency_base * 2
```
Success: GUARANTEED.
# Insurance Recipes
Recipes produce tier-appropriate Insurance Charms under ADR-0022:
- `t1..t2` produces `item.consumable.bua_giu_bac.so_cap`
- `t3..t4` produces `item.consumable.bua_giu_bac.trung_cap`
- `t5..t6` produces `item.consumable.bua_giu_bac.cao_cap`

Recipe ID:
```text
recipe.utility.bua_giu_bac.<tier>
```
Input:
```text
10 * mapped regional material
common currency = tier_common_currency_base * 4
```
Success: GUARANTEED.

The alternate `currency.bound` utility offers in `economy_catalog.md` never replace these recipes; both paths coexist.
# Recovery Consumables
`item.consumable.nuoc_la` and `item.consumable.tra_sen` are primarily shop/field-drop items and intentionally have no launch crafting recipe. This keeps equipment crafting as the only persistent **gear** crafting loop players must learn.

# Hearth Cooking
Guaranteed hearth recipes (`success_mode = GUARANTEED`). Station = `cooking_hearth.<map_id>`. Rejected in combat. Not a profession tree.
| recipe_id | inputs | output | extra_output |
|---|---|---|---|
| `recipe.food.ca_bong_kho` | 1 `item.material.ca_bong` + 1 `item.material.rau_ram` | `item.consumable.food.ca_bong_kho` | 1 `item.material.cui_lua_trai` |
| `recipe.food.ca_chep_nuong` | 1 `item.material.ca_chep` + 1 `item.material.gung_lang` | `item.consumable.food.ca_chep_nuong` | 1 `item.material.cui_lua_trai` |
| `recipe.food.tom_nuong` | 1 `item.material.tom_song` + 1 `item.material.rau_ram` | `item.consumable.food.tom_nuong` | 1 `item.material.cui_lua_trai` |
| `recipe.food.ca_ro_kho` | 1 `item.material.ca_ro_dong` + 1 `item.material.gung_lang` | `item.consumable.food.ca_bong_kho` | 1 `item.material.cui_lua_trai` |
| `recipe.food.ruou_nep` | 2 `item.material.ca_chep` + 1 `item.material.gung_lang` | `item.consumable.ruou_nep` | 1 `item.material.cui_lua_trai` |

`RARE_CATCH` is never an input. `ca_ro_kho` reuses `ca_bong_kho` output so rô đồng is not vendor trash. Kindling faucet: every successful cook grants `extra_output = 1 item.material.cui_lua_trai` (no shop row).

LIFE_SKILL: each successful `DISH_COOKED` is 1 action granting LIFE_SKILL per-unit for the character's current act (`progression_route.md`): I 6417, II 8283, III 6332, IV 7047, V 9448, VI 10494. Key `life_skill.cook.<recipe_id>.<character_id>.<operation_id>`.

# Economy Guardrail
Equipment crafting is a deterministic sink for:
```text
regional material + common currency
```
It is not intended to outcompete direct loot in time-to-first-upgrade. Target acquisition pacing:
- a player naturally completing an act should afford several meaningful crafted pieces,
- a full 14-piece tier replacement normally requires optional combat/dungeon/crafting play,
- the player never needs a rare random recipe drop,
- normal crafting does not require PvP/Guild War/bound currency,
- the player does not need Auction purchases to complete a build-defining set threshold.

# Validation
Static validation expands all recipe IDs and rejects:
- missing output item,
- missing tier material,
- invalid slot weight,
- output tier/recipe tier mismatch,
- negative/zero input quantity,
- non-guaranteed normal equipment recipe,
- duplicate output recipe ID,
- full-set common/material reference total inconsistent with the 47 slot-weight sum.

# Invariants
```text
168 equipment items -> 168 guaranteed equipment recipes
recipe discovery RNG = none
profession grind = none
auction dependency = none
craft output starts at +0
full-set slot-weight sum = 47
```