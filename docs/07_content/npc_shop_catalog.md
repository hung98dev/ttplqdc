# Launch NPC and Shop Catalog
status: LOCKED

## Scope
Concrete launch NPC/service/shop definitions for the six safe/social anchors. Runtime interaction authority remains canonical in `../02_world/npcs.md`; economy behavior remains in `../03_systems/economy.md`; launch currency amounts are owned by `economy_catalog.md`.

NPCs are fictional inhabitants of the game world. Names/titles are ordinary Vietnamese-language presentation, not depictions of real religious or historical persons.

# Shared Service Shape
Each progression safe anchor has exactly three persistent service NPCs:
```text
nguoi_dan_duong -> STORY, QUEST, SERVICE(set_checkpoint, travel, respec)
tho_nghe        -> SHOP, SERVICE(crafting, enhancement)
hang_quan       -> SHOP, SERVICE(storage.account, auction)
```
This keeps critical services predictable without filling towns with dozens of functional NPCs.

Default:
```text
movement_mode = STATIC
interaction_range = 2.5m equivalent
schedule = ALWAYS
```
No launch critical service is night-only or event-only.

# Regional NPCs — 18
| map_id | Guide / checkpoint | Craft / enhance | Shop / account / auction |
|---|---|---|---|
| `map.lang_da.dinh_lang` | `npc.lang_da.nguoi_dan_duong` — Người Dẫn Đường Làng | `npc.lang_da.tho_nghe` — Bác Thợ Làng | `npc.lang_da.hang_quan` — Hàng Nước Làng |
| `map.rung_u_minh.xom_rung` | `npc.rung_u_minh.nguoi_dan_duong` — Người Dẫn Đường Rừng | `npc.rung_u_minh.tho_nghe` — Thợ Xóm Rừng | `npc.rung_u_minh.hang_quan` — Quán Ven Rừng |
| `map.ben_nuoc_den.cho_ben` | `npc.ben_nuoc_den.nguoi_dan_duong` — Người Giữ Bến | `npc.ben_nuoc_den.tho_nghe` — Thợ Bến | `npc.ben_nuoc_den.hang_quan` — Quán Chợ Bến |
| `map.deo_may.ban_chan_deo` | `npc.deo_may.nguoi_dan_duong` — Người Gác Đèo | `npc.deo_may.tho_nghe` — Thợ Chân Đèo | `npc.deo_may.hang_quan` — Quán Chân Đèo |
| `map.thanh_co.cong_ngoai` | `npc.thanh_co.nguoi_dan_duong` — Người Giữ Cổng | `npc.thanh_co.tho_nghe` — Thợ Ngoài Thành | `npc.thanh_co.hang_quan` — Quán Ngoài Thành |
| `map.nui_thieng.chan_nui` | `npc.nui_thieng.nguoi_dan_duong` — Người Dẫn Đường Núi | `npc.nui_thieng.tho_nghe` — Thợ Chân Núi | `npc.nui_thieng.hang_quan` — Quán Chân Núi |
Display labels may be replaced later by named fictional characters without changing `npc_id` or services.

# Ambient NPCs — 24
Each safe anchor has 4 ambient NPCs (`capabilities = DIALOGUE, QUEST, DECORATIVE`; no SHOP/SERVICE, ADR-0061). They grant nothing outside quest transactions. A NIGHT_ONLY or DAY_ONLY NPC can be talked to, give a quest or accept a turn-in only while present; its SIDE quest can be accepted only then, and progress already made persists while it is absent.

| map_id | DAY_ONLY | NIGHT_ONLY | PATROL |
|---|---|---|---|
| `map.lang_da.dinh_lang` | `npc.lang_da.cho_1`, `npc.lang_da.cho_2` | `npc.lang_da.dem_1`, `npc.lang_da.dem_2` | — |
| `map.rung_u_minh.xom_rung` | `npc.rung_u_minh.cho_1`, `npc.rung_u_minh.cho_2` | `npc.rung_u_minh.dem_1`, `npc.rung_u_minh.dem_2` | — |
| `map.ben_nuoc_den.cho_ben` | `npc.ben_nuoc_den.cho_1`, `npc.ben_nuoc_den.cho_2` | `npc.ben_nuoc_den.dem_1`, `npc.ben_nuoc_den.dem_2` | — |
| `map.deo_may.ban_chan_deo` | `npc.deo_may.cho_1`, `npc.deo_may.cho_2` | `npc.deo_may.dem_1`, `npc.deo_may.dem_2` | — |
| `map.thanh_co.cong_ngoai` | `npc.thanh_co.cho_1`, `npc.thanh_co.cho_2` | `npc.thanh_co.dem_1`, `npc.thanh_co.dem_2` | — |
| `map.nui_thieng.chan_nui` | `npc.nui_thieng.cho_1`, `npc.nui_thieng.cho_2` | `npc.nui_thieng.dem_1`, `npc.nui_thieng.dem_2` | — |

Dialogue: 1–2 nodes, localization only. Critical services remain the 18 ALWAYS NPCs.

# Guide Service Contract
Every `*.nguoi_dan_duong` supports:
```text
service.set_checkpoint
service.travel
service.respec
```

## Checkpoint
Sets that region's safe-anchor checkpoint:
```text
checkpoint.lang_da.dinh_lang
checkpoint.rung_u_minh.xom_rung
checkpoint.ben_nuoc_den.cho_ben
checkpoint.deo_may.ban_chan_deo
checkpoint.thanh_co.cong_ngoai
checkpoint.nui_thieng.chan_nui
```

## Travel
Travel is only between discovered safe/social anchors. No field-map teleport destination is offered.

Travel common-currency cost by destination tier:
```text
T1 0
T2 100
T3 200
T4 350
T5 550
T6 800
```
Returning to `map.lang_da.dinh_lang` costs `0` common currency so a character is never economically stranded.

Travel validates destination discovery, story access, `in_combat=false`, and normal transfer rules.

## Respec
Opens the canonical skill/potential respec actions from `../01_gameplay/progression.md`. The NPC never owns a second respec price table.

# Craft / Enhancement Service
Every `*.tho_nghe` supports:
```text
service.crafting
service.enhancement
shop_id = shop.utility.bound
```
and references the recipes from `crafting_catalog.md`.

The service does not sell equipment directly and does not provide repair/durability actions.

## Bound Utility Shop — `shop.utility.bound`
Unlimited NPC stock; server-authoritative fixed prices owned by `economy_catalog.md`.

| offer_id | item_id | currency | price | source binding override |
|---|---|---|---:|---|
| `offer.bound.bua_may.so_cap` | `item.consumable.bua_may.so_cap` | `currency.bound` | 25 | `CHARACTER_BOUND` |
| `offer.bound.bua_may.trung_cap` | `item.consumable.bua_may.trung_cap` | `currency.bound` | 50 | `CHARACTER_BOUND` |
| `offer.bound.bua_may.cao_cap` | `item.consumable.bua_may.cao_cap` | `currency.bound` | 120 | `CHARACTER_BOUND` |
| `offer.bound.bua_may.sieu_cap` | `item.consumable.bua_may.sieu_cap` | `currency.bound` | 300 | `CHARACTER_BOUND` |
| `offer.bound.bua_giu_bac.so_cap` | `item.consumable.bua_giu_bac.so_cap` | `currency.bound` | 60 | `CHARACTER_BOUND` |
| `offer.bound.bua_giu_bac.trung_cap` | `item.consumable.bua_giu_bac.trung_cap` | `currency.bound` | 120 | `CHARACTER_BOUND` |
| `offer.bound.bua_giu_bac.cao_cap` | `item.consumable.bua_giu_bac.cao_cap` | `currency.bound` | 300 | `CHARACTER_BOUND` |
These are optional convenience alternatives. Both items retain their normal regional-material/common-currency crafting recipes, so PvP/Guild War is never required for enhancement progression.

### Bound-Currency Anti-Laundering
The base item definitions remain `UNBOUND` for normal drop/crafting sources, but a purchase paid with `currency.bound` applies this source override at creation:
```text
binding = CHARACTER_BOUND
binding_trigger = ON_ACQUIRE
```

A bound-purchased copy therefore cannot enter direct trade, Auction, Guild Storage, or account cross-character storage. It may only be consumed by the owning character under the normal enhancement rules.

The binding override is part of the atomic purchase transaction and cannot be removed by stacking, split/merge, reconnect, refund, or moving the item between ordinary character containers. Bound-purchased copies may stack only with compatible copies that have the same binding state.

This prevents converting non-transferable `currency.bound` into tradable Bùa items. Normal crafted/dropped copies keep their original item-definition binding and remain independent assets.

No equipment, Soul, direct enhancement level, skill point, or potential point is sold here.

# Shop / Account / Auction Service
Every `*.hang_quan` supports:
```text
shop_id = shop.recovery.common
service.storage.account
service.auction
```

Feature access still follows character milestones:
- account storage: service exists but normal storage rules determine eligible items
- auction: Level 15 minimum from progression
- normal shop: always available outside combat

# Shop — `shop.recovery.common`
Unlimited NPC stock; server-authoritative fixed price.

| offer_id | item_id | currency | buy_price | sell_back_price |
|---|---|---|---:|---:|
| `shop.recovery.nuoc_la` | `item.consumable.nuoc_la` | `currency.common` | 80 | 16 |
| `shop.recovery.tra_sen` | `item.consumable.tra_sen` | `currency.common` | 80 | 16 |
| `shop.recovery.can_cau_tre` | `item.tool.can_cau_tre` | `currency.common` | 200 | 40 |
| `shop.recovery.moi_cau` | `item.consumable.moi_cau` | `currency.common` | 5 | 1 |
| `shop.recovery.rau_ram` | `item.material.rau_ram` | `currency.common` | 8 | 1 |
| `shop.recovery.gung_lang` | `item.material.gung_lang` | `currency.common` | 8 | 1 |

`sell_back_price` is the authoritative common received when selling the item back to any NPC shop. It equals `floor(buy_price × 0.20)` for consumables and `floor(buy_price × 0.20)` for the rod; bait and herbs use a floor of `1`. These values are explicit; parsers must not recompute them from `buy_price` alone.

`can_cau_tre` purchase is CHARACTER_BOUND on acquire; a character who already owns one is rejected without debit.

Regional crafting materials, equipment, Souls, Bùa May, and Bùa Giữ Bậc are not sold for `currency.common` at launch.

# Quest References
MAIN/SIDE quests are bound to NPC quest-givers from this catalog. Every MAIN quest in an act uses the region's `nguoi_dan_duong` as quest-giver; SIDE quests use the region's ambient NPCs (`cho_*` / `dem_*`). Act-closing MAIN quests (MAIN 4 of each act) use `TURN_IN` completion mode, returning to the region's `nguoi_dan_duong`. Full quest-giver bindings are authored in `quest_catalog.md`; this file is the NPC authority.

Quest state/reward logic remains solely in `quest_catalog.md` / `../02_world/quests.md`.

A dialogue node cannot grant power outside an explicit quest/service transaction.

# Visual Direction
NPC silhouettes should read as Vietnamese rural/travel/craft characters appropriate to each fictional region through clothing, props, stall architecture, baskets, tools, boats, bamboo/wood furniture, and environment language.

Do not default service NPCs to generic cultivation-sect robes, Japanese shrine attendants, or Western fantasy merchants.

# Validation
Reject:
- NPC placement on missing map,
- service not supported by canonical NPC system,
- shop offer item/currency missing from owning catalog,
- bound utility price differing from `economy_catalog.md`,
- bound utility item lacking its normal PvE crafting path,
- `currency.bound` purchase output that is not `CHARACTER_BOUND` on acquire,
- bound-purchased utility entering trade/Auction/Guild Storage/account storage,
- stack merge that mixes the bound-purchase copy with an incompatible UNBOUND copy,
- travel destination not a safe/social anchor,
- auction access that bypasses Level 15,
- personal general-purpose bank service,
- repair/durability service,
- direct equipment/Soul/stat/skill-point sale through bound utility shop.

# Invariants
```text
18 persistent launch service NPCs
24 ambient NPCs (DIALOGUE, QUEST, DECORATIVE; no SHOP/SERVICE)
1 shared recovery-common shop
1 shared bound-utility shop
3 predictable service NPCs per safe anchor
bound utility != exclusive progression path
bound-currency purchase output = CHARACTER_BOUND
bound currency cannot be laundered into tradable item value
no general personal bank
no repair service
travel only between discovered safe anchors
critical services always have an available equivalent
```