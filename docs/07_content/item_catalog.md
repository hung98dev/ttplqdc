# Launch Item Catalog
status: LOCKED

## Scope
Concrete non-equipment launch items needed by crafting, enhancement, recovery, cosmetic redemption, and reward tables. Equipment item definitions are owned by `equipment_catalog.md`; Soul instances are not inventory items.

All stable IDs are ASCII lowercase. Display text is localization content.

# Regional Craft / Enhancement Materials — 6
All six are:
```text
type = MATERIAL
rarity = COMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 9999
discard_allowed = true
HUNT_eligible = true
```

| item_id | Display | Tier | Identity |
|---|---|---:|---|
| `item.material.lang_da.manh_dong` | Mảnh Đồng Làng | T1 | old bronze/metal fragments recovered around Làng Đa |
| `item.material.u_minh.vo_cay` | Vỏ Cây U Minh | T2 | supernatural bark from fictional U Minh-region growths |
| `item.material.ben_nuoc.da_song` | Đá Sông | T3 | smooth mineral pieces from Bến Nước Đen waterways |
| `item.material.deo_may.da_voi` | Mảnh Đá Vôi | T4 | limestone fragments from Đèo Mây caves and paths |
| `item.material.thanh_co.gach_co` | Mảnh Gạch Cổ | T5 | fragments from fictional ruined structures; not real artifacts |
| `item.material.nui_thieng.da_suong` | Đá Sương Núi | T6 | fictional mineral formed along the high-mountain route |

These six items are the only normal launch base material consumed by equipment crafting and enhancement. This deliberately avoids a large material-bag taxonomy.

# Enhancement Support Items
## `item.consumable.bua_may.so_cap`
Display: **Bùa May Mắn (Sơ Cấp)**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
shared_cooldown_group = NONE
```
Use: Grants `+5%` (+500 bp) enhancement success bonus for equipment below +8 under ADR-0022.

## `item.consumable.bua_may.trung_cap`
Display: **Bùa May Mắn (Trung Cấp)**
```text
type = CONSUMABLE
rarity = UNCOMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
shared_cooldown_group = NONE
```
Use: Grants `+3%` (+300 bp) enhancement success bonus for equipment below +12 under ADR-0022.

## `item.consumable.bua_may.cao_cap`
Display: **Bùa May Mắn (Cao Cấp)**
```text
type = CONSUMABLE
rarity = RARE
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
shared_cooldown_group = NONE
```
Use: Grants `+1%` (+100 bp) enhancement success bonus for all equipment levels (+0..+16) under ADR-0022.

## `item.consumable.bua_may.sieu_cap`
Display: **Bùa May Mắn (Siêu Cấp)**
```text
type = CONSUMABLE
rarity = EPIC
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
shared_cooldown_group = NONE
```
Use: Grants `+3%` (+300 bp) enhancement success bonus for all equipment levels (+0..+16) under ADR-0022.

Copies purchased from `shop.utility.bound` use `CHARACTER_BOUND ON_ACQUIRE`. Visual direction: small fictional paper charms with traditional Vietnamese decorative motifs.
## `item.consumable.bua_giu_bac.so_cap`
Display: **Bùa Giữ Bậc (Sơ Cấp)**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
shared_cooldown_group = NONE
```
Use: Prevents level downgrade on failed enhancement for equipment below +8 under ADR-0022.

## `item.consumable.bua_giu_bac.trung_cap`
Display: **Bùa Giữ Bậc (Trung Cấp)**
```text
type = CONSUMABLE
rarity = RARE
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
shared_cooldown_group = NONE
```
Use: Prevents level downgrade on failed enhancement for equipment below +12 under ADR-0022.

## `item.consumable.bua_giu_bac.cao_cap`
Display: **Bùa Giữ Bậc (Cao Cấp)**
```text
type = CONSUMABLE
rarity = EPIC
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
shared_cooldown_group = NONE
```
Use: Prevents level downgrade on failed enhancement for all equipment levels (+0..+16) under ADR-0022.

As with Bùa May, normal drop/craft copies use the base definition; `shop.utility.bound` copies are created `CHARACTER_BOUND ON_ACQUIRE` and cannot merge with UNBOUND stacks.
# Recovery Consumables
## `item.consumable.nuoc_la`
Display: **Nước Lá**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 200
shared_cooldown_group = HP
```
Effect:
```text
restore = 20% MAX_HP
```
Does not revive. Content that disables healing consumables rejects use before consumption.

## `item.consumable.tra_sen`
Display: **Trà Sen**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 200
shared_cooldown_group = MP
```
Effect:
```text
restore = 20% MAX_MP
```

Recovery items are convenience sustain, not primary combat rotation requirements. Normal field monsters may drop them at low rates; shops may sell them using common currency.

# Engagement & Exploration Items
## `item.consumable.chia_khoa_co`
Display: **Chìa Khóa Cổ**
```text
type = CONSUMABLE
rarity = UNCOMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 99
shared_cooldown_group = NONE
```
Use: Consumed to unlock Hidden Folklore Chests (`chest.hidden.*`) on adventure field maps under ADR-0023. Drops rarely from field monsters and elites.

## `item.consumable.ruou_nep`
Display: **Rượu Nếp Làng**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 99
shared_cooldown_group = BUFF
```
Use: Consumed while in `BONFIRE_REST`. Buff `buff.ruou_nep_am_long` (+5% ATTACK, 30 minutes) **begins on consumption** (`world_rules.md`). Does not stack or refresh.

## `item.material.cui_lua_trai`
Display: **Củi Lửa Trại**
```text
type = MATERIAL
rarity = COMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
HUNT_eligible = false
```
Use: Kindles or extends communal village bonfire gatherings at Safe Anchors under ADR-0023.

## `item.tool.can_cau_tre`
Display: **Cần Câu Tre**
```text
type = TOOL
rarity = COMMON
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 1
```
Use: Equipped in fishing spots (`fishing_spot.*`) along riverbanks and piers to catch folk aquatic resources under ADR-0024.

## `item.consumable.moi_cau`
Display: **Mồi Câu Giun Đất**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 999
```
Use: Consumed on each fishing cast at a fishing spot under ADR-0024. Purchased at `shop.recovery.common`. No field-dig gather at launch.

## `item.material.rau_ram`
Display: **Rau Răm**
```text
type = MATERIAL
rarity = COMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
HUNT_eligible = false
```
Cooking herb. Shop `shop.recovery.common` only at launch.

## `item.material.gung_lang`
Display: **Gừng Làng**
```text
type = MATERIAL
rarity = COMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
HUNT_eligible = false
```
Cooking herb. Shop `shop.recovery.common` only at launch.
## `item.material.ca_bong`
Display: **Cá Bống**
```text
type = MATERIAL
rarity = COMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
HUNT_eligible = false
```
Use: Caught at fishing spots; cooking ingredient for Linh Thú companion food under ADR-0024.

## `item.material.ca_ro_dong`
Display: **Cá Rô Đồng**
```text
type = MATERIAL
rarity = COMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
HUNT_eligible = false
```
Use: Caught at fishing spots as `COMMON_CATCH`. Input of `recipe.food.ca_ro_kho` (output `item.consumable.food.ca_bong_kho`).

## `item.material.ca_chep`
Display: **Cá Chép**
```text
type = MATERIAL
rarity = UNCOMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
HUNT_eligible = false
```
Use: Caught at fishing spots; cooking ingredient for Linh Thú companion food under ADR-0024.

## `item.material.ca_chep_hoa_rong`
Display: **Cá Chép Hóa Rồng**
```text
type = MATERIAL
rarity = LEGENDARY
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 99
HUNT_eligible = false
discard_allowed = true
```
Use: Sole launch `RARE_CATCH` on `fishing.catch.default`. Not a cooking input, not tradable/auctionable, not enhancement material. `FISH_CAUGHT` of this item is `PHAT_HIEN` (`source=FISH_RARE`) under `../02_world/world_rules.md`.

## `item.material.tom_song`
Display: **Tôm Sông**
```text
type = MATERIAL
rarity = COMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
HUNT_eligible = false
```
Use: Caught at fishing spots; cooking ingredient for Linh Thú companion food under ADR-0024.

## `item.material.ca_linh_giang`
Display: **Cá Linh Giang**
```text
type = MATERIAL
rarity = RARE
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 99
HUNT_eligible = false
discard_allowed = true
```
Use: Season-0 seasonal catch only (`fishing.catch.season.0`). Not a cooking input. Not on `fishing.catch.default`.

## `item.material.ca_sam_u_minh`
Display: **Cá Sặt U Minh**
```text
type = MATERIAL
rarity = RARE
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 99
HUNT_eligible = false
discard_allowed = true
```
Use: Season-1 seasonal catch only (`fishing.catch.season.1`). Not a cooking input.

## `item.material.ca_bong_den`
Display: **Cá Bống Đen**
```text
type = MATERIAL
rarity = RARE
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 99
HUNT_eligible = false
discard_allowed = true
```
Use: Season-2 seasonal catch only (`fishing.catch.season.2`). Not a cooking input.

## `item.material.ca_suong_ho`
Display: **Cá Sương Hồ**
```text
type = MATERIAL
rarity = RARE
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 99
HUNT_eligible = false
discard_allowed = true
```
Use: Season-5 seasonal catch only (`fishing.catch.season.5`). Not a cooking input.

## `fishing.catch.default`
Launch catch table for every `fishing_spot.*` unless a spot row names another legal table. Weights are basis points summing to `10000`.

| item_id | set | weight_bp |
|---|---|---:|
| `item.material.ca_bong` | COMMON_CATCH | 4000 |
| `item.material.ca_ro_dong` | COMMON_CATCH | 2500 |
| `item.material.tom_song` | COMMON_CATCH | 2500 |
| `item.material.ca_chep` | COMMON_CATCH | 900 |
| `item.material.ca_chep_hoa_rong` | RARE_CATCH | 100 |

Reject activation when weights are not positive integers, do not sum to 10000, omit `RARE_CATCH`, or include any ID outside `COMMON_CATCH ∪ RARE_CATCH`. Default table is closed; seasonal extra IDs live only on the active seasonal table.

## Seasonal catch tables
Shared weights except the 100 bp seasonal row. Sum `10000`. Used by `has_water` spots of the featured region while `season_region_index` matches; otherwise those spots use `fishing.catch.default`. No tables for seasons 3–4.

| table_id | region spots | extra SEASONAL_CATCH | extra bp | ca_bong bp |
|---|---|---|---:|---:|
| `fishing.catch.season.0` | `fishing_spot.map.lang_da.*` | `item.material.ca_linh_giang` | 100 | 3900 |
| `fishing.catch.season.1` | `fishing_spot.map.rung_u_minh.*` | `item.material.ca_sam_u_minh` | 100 | 3900 |
| `fishing.catch.season.2` | `fishing_spot.map.ben_nuoc_den.*` | `item.material.ca_bong_den` | 100 | 3900 |
| `fishing.catch.season.5` | `fishing_spot.map.nui_thieng.*` | `item.material.ca_suong_ho` | 100 | 3900 |

Other rows identical to default: `ca_ro_dong` 2500, `tom_song` 2500, `ca_chep` 900, `ca_chep_hoa_rong` 100.

## `item.consumable.food.ca_bong_kho`
Display: **Cá Bống Kho Tộ**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 99
shared_cooldown_group = FOOD
```
Use: Fed to any owned Linh Thú via `C2S_BEAST_FEED` to grant `+5 bond_points` (canonical value; daily cap and clamp in `../03_systems/spirit_beasts.md`, ADR-0024).

## `item.consumable.food.ca_chep_nuong`
Display: **Cá Chép Nướng Mộc**
```text
type = CONSUMABLE
rarity = UNCOMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 99
shared_cooldown_group = FOOD
```
Use: Fed to any owned Linh Thú via `C2S_BEAST_FEED` to grant `+8 bond_points` (canonical value; daily cap and clamp in `../03_systems/spirit_beasts.md`, ADR-0024).

## `item.consumable.food.tom_nuong`
Display: **Tôm Nướng Than**
```text
type = CONSUMABLE
rarity = COMMON
binding = UNBOUND
binding_trigger = ON_USE
stack_limit = 99
shared_cooldown_group = FOOD
```
Use: Fed to any owned Linh Thú via `C2S_BEAST_FEED` to grant `+10 bond_points` (canonical value; daily cap and clamp in `../03_systems/spirit_beasts.md`, ADR-0024).

# Cosmetic Redemption Material
## `item.material.vai_hoa_van`
Display: **Vải Hoa Văn**
```text
type = MATERIAL
rarity = UNCOMMON
binding = ACCOUNT_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 999
HUNT_eligible = false
```

Launch deterministic cosmetic sinks:
```text
20 item.material.vai_hoa_van
-> cosmetic.appearance.ao_vai_hoa_van

30 item.material.vai_hoa_van
-> cosmetic.frame.nui_thieng
```
through account cosmetic redemption in `cosmetic_catalog.md` / `../03_systems/cosmetics.md`.

Each entitlement also has an explicit alternative `20 currency.special` route. There is no material/currency exchange and one redemption never consumes both.

This material grants no combat power and is excluded from Guild HUNT weighting.

# Bonus Progression Books
## `item.book.potential`
Display: **Sách Tiềm Năng**
```text
type = CONSUMABLE
rarity = RARE
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 99
shared_cooldown_group = NONE
```
Use: Consumed to grant `+10` unspent potential points (counts toward 60% per-stat cap denominator; total by 60 = 356). Granted via `progression.book.potential.<level>` flags at Lv25,30,35,40 (+1 each) and Lv45,50,55,60 (+2 each) — total 12 by 60. Idempotent operation per level flag.

## `item.book.skill`
Display: **Sách Kỹ Năng**
```text
type = CONSUMABLE
rarity = EPIC
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
stack_limit = 99
shared_cooldown_group = NONE
```
Use: Consumed to grant `+1` unspent skill point (total by 60 = 75/114; 59 from levels + 12 from books + 4 from Lv55/Lv60 milestone bonus per ADR-0033). Same schedule and idempotency as potential book, via `progression.book.skill.<level>`. Never tradable/auctionable/storage.

Both books are CHARACTER_BOUND and never enter trade/Auction/Guild Storage/account_storage. See `../01_gameplay/progression.md` and `../03_systems/items.md`.

# Spirit Beast Materials (Linh Đan)
## `item.material.linh_dan.so_cap`
Display: **Linh Đan (Sơ Cấp)**
```text
type = MATERIAL
rarity = UNCOMMON
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
```
Use: Dedicated upgrade material for Linh Thú companion leveling from Level 1 to 20 under `../03_systems/spirit_beasts.md`.

## `item.material.linh_dan.trung_cap`
Display: **Linh Đan (Trung Cấp)**
```text
type = MATERIAL
rarity = RARE
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
```
Use: Dedicated upgrade material for Linh Thú companion leveling from Level 21 to 40 under `../03_systems/spirit_beasts.md`.

## `item.material.linh_dan.cao_cap`
Display: **Linh Đan (Cao Cấp)**
```text
type = MATERIAL
rarity = EPIC
binding = UNBOUND
binding_trigger = NONE
stack_limit = 999
```
Use: Dedicated upgrade material for Linh Thú companion leveling from Level 41 to 60 under `../03_systems/spirit_beasts.md`.

# Binding / Stack Validation
Static/runtime validation must preserve the distinction between base item binding and stricter source binding:
- `UNBOUND` crafted/dropped Bùa and `CHARACTER_BOUND` bound-shop Bùa cannot merge,
- split keeps the source stack's exact effective binding,
- using a bound-shop Bùa is legal for its owning character,
- bound-shop Bùa cannot enter trade, Auction, Guild Storage, or account storage,
- ACCOUNT_BOUND items cannot be transferred between characters on the same account (ADR-0029).

# Invariants
```text
regional power-crafting material types = 6
bonus progression book types = 2 (item.book.potential +10, item.book.skill +1, total 12 each by 60)
no hidden material quality tiers
Soul != inventory item
currency != item
Lucky/Insurance behavior owned by crafting.md
bound-shop utility copies are CHARACTER_BOUND ON_ACQUIRE
bonus books are CHARACTER_BOUND ON_ACQUIRE and never tradable/auctionable
Vải Hoa Văn has exactly two concrete launch cosmetic sinks
real sacred/religious objects are not represented as lootable historical artifacts
fishing.catch.default sums to 10000 bp and includes RARE_CATCH 100
ca_chep_hoa_rong is CHARACTER_BOUND and not a cooking input
```