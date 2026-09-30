# Launch Economy Catalog
status: LOCKED

## Scope
Concrete launch currency faucets/sinks, reward bands, service prices, and economy-balance validation for the three currencies owned by `../03_systems/economy.md`.

This file owns launch-wide numeric currency bands that are reused by multiple reward systems. Domain catalogs still own their item/material/quest definitions and reference these bands instead of copying competing values.

# Design Targets
Launch economy should:
- let a first character progress and craft meaningful gear without Auction dependence,
- keep full-tier crafting and high enhancement as optional progression rather than story gates,
- keep `currency.common` circulating and meaningfully sinkable,
- give `currency.bound` a small non-tradable utility loop available from PvE and competitive play,
- keep `currency.special` cosmetic/account-only and sparse,
- avoid a daily-login power obligation,
- avoid currencies whose only purpose is to become another grind layer.

# `currency.common`
## Combat Faucet Bands
These are canonical guaranteed common-currency ranges used by `drop_tables.md`.

### NORMAL monster
| Tier | common |
|---|---:|
| T1 | 8..12 |
| T2 | 18..28 |
| T3 | 35..55 |
| T4 | 60..90 |
| T5 | 95..135 |
| T6 | 140..200 |

### ELITE monster
| Tier | common |
|---|---:|
| T1 | 40..60 |
| T2 | 90..130 |
| T3 | 180..260 |
| T4 | 320..460 |
| T5 | 500..700 |
| T6 | 750..1050 |

### Major boss
| Tier | common |
|---|---:|
| T1 | 300..450 |
| T2 | 700..1000 |
| T3 | 1400..2000 |
| T4 | 2600..3600 |
| T5 | 4200..5600 |
| T6 | 6200..8200 |

Normal dungeon repeat completion uses the owning tier's **boss-band minimum** as its guaranteed completion currency.

`ENDGAME_L60` dungeon completion uses:
```text
9000..12000 common
```
and combines the final-boss economy slot as defined by `drop_tables.md`; it must not additionally settle the old low-tier boss currency/material bundle.

Spirit Surge normal completion uses the owning tier's **elite-band minimum**.

Quest and Daily common rewards remain authored explicitly in `quest_catalog.md` because their values are pacing data, not reusable combat bands.

## Common Sinks
Launch sinks remain:
- deterministic equipment crafting,
- enhancement,
- recovery consumables,
- respec,
- inventory expansion,
- guild creation,
- safe-anchor travel,
- Auction listing fee and sale tax,
- direct trade fee (5% of common transferred),
- cosmetic common purchases (Guild Stone inscriptions, shrine variants, title glows).

No durability repair/rent/upkeep tax is added.

## `currency.common` Cosmetic Sink Catalog
Non-power, non-rent, one-time-unlock-per-variant cosmetics purchasable directly with `currency.common`. Each variant is a permanent character cosmetic. No premium key, no stamina, no fourth currency. Reason code: `COSMETIC_COMMON_SINK`.

### Guild Stone Inscriptions (Bia Đá Danh Vọng Kiểu)
Visual inscription style applied to the character's Guild Stone entry. Cosmetic only; no stat effect.
```text
cosmetic.guild_stone.inscription.trung_nguyen  cost 50,000 common
cosmetic.guild_stone.inscription.long_van      cost 80,000 common
cosmetic.guild_stone.inscription.phuong_vi     cost 120,000 common
cosmetic.guild_stone.inscription.ngoc_bich     cost 180,000 common
cosmetic.guild_stone.inscription.kim_bach      cost 250,000 common
cosmetic.guild_stone.inscription.thien_long    cost 400,000 common
```

### Shrine Variants (Miếu / Đàn Thờ)
Visual theme applied to the character's personal shrine display in safe anchors.
```text
cosmetic.shrine.lua_do     cost 60,000 common
cosmetic.shrine.thuy_ngoc  cost 100,000 common
cosmetic.shrine.moc_xanh   cost 150,000 common
cosmetic.shrine.tho_vang   cost 220,000 common
cosmetic.shrine.kim_trang  cost 320,000 common
cosmetic.shrine.linh_khoi  cost 500,000 common
```

### Title Glows (Hiệu Ứng Tước Hiệu)
Visual glow effect behind the character's displayed title text. No stat effect. Applied per character.
```text
cosmetic.title_glow.do_quang    cost 30,000 common
cosmetic.title_glow.xanh_nhat   cost 30,000 common
cosmetic.title_glow.vang_nhat   cost 30,000 common
cosmetic.title_glow.tim_nhat    cost 30,000 common
cosmetic.title_glow.bach_nhat   cost 30,000 common
cosmetic.title_glow.hong_phuc   cost 75,000 common
cosmetic.title_glow.linh_hoa    cost 75,000 common
cosmetic.title_glow.thien_van   cost 120,000 common
```

Total common cosmetic sink surface (all variants, one each):
```text
Guild Stone inscriptions: 50k+80k+120k+180k+250k+400k = 1,080,000 common
Shrine variants:          60k+100k+150k+220k+320k+500k = 1,350,000 common
Title glows:              30k*5+75k*2+120k = 420,000 common
Total cosmetic sink surface = 2,850,000 common per character
```
Reference income (lower bound, FIELD_COMBAT only): `REFERENCE_ENDGAME_COMMON_PER_HOUR = 76,500` = T6 NORMAL mean drop 170 common × 450 kills/hour. This is the single canonical reference-income constant; every hour estimate in other specs (`../03_systems/crafting.md`, `equipment_catalog.md`) divides by it. Quest, bounty and dungeon common add to this, so real income is higher. At the lower bound the full catalog takes about 37 hours of Lv60 play, keeping cosmetics aspirational but reachable.

# `currency.bound`
Bound currency is character-scoped and cannot transfer between players.

Its launch purpose is **optional utility acceleration**, never exclusive power.

## Sources
All grants are idempotent and require normal reward eligibility.

### SIDE quest completion
Each launch SIDE quest grants in addition to its existing rewards:
```text
bound = 10 * owning_tier
```
Therefore one side quest grants `10/20/30/40/50/60` from T1..T6.

### Dungeon completion
The **first eligible dungeon completion per character per UTC day, across all dungeons** grants bound currency once (`DAILY_FIRST` slot). The amount follows the completed run's tier/context:
```text
T1  5
T2 10
T3 15
T4 20
T5 25
ENDGAME_L60 30
idempotency key = dungeon.bound.daily.<utc_date>.<character_id>   (utc_date of the completion commit)
```
The first completion of the UTC day fixes the amount; later completions that day grant no bound currency, even in a higher tier. The EXP and item rewards of repeatable dungeons are unaffected.

Rationale: previously these grants were repeatable with no daily lockout, producing ~90+ bound/hour against a total sink surface of 975. The `daily_first` conversion brings the bound faucet into sustainable balance while preserving the dungeon's core EXP/item value.

### Ranked PvP
For the first five reward-eligible ranked match completions per character per UTC day **across all ranked modes combined**:
```text
20 bound per completed match
```
Win/loss does not change this completion amount. Rating remains the competitive reward signal; currency is not used to incentivize intentional stomps.

### Guild War
For the first three reward-eligible Guild War completions per character per Monday-00:00-UTC week **across guild membership changes**:
```text
50 bound per completed match
```
Win/loss does not change personal bound currency. Guild progression winner bonuses remain separate in `guild_war.md`.

### Daily MYSTERY Bounty
Daily MYSTERY bounty completion grants bound currency per tier (see `../07_content/quest_catalog.md` for full bounty definitions):
```text
T1:  5 bound
T2: 10 bound
T3: 15 bound
T4: 20 bound
T5: 25 bound
T6: 30 bound
```
One MYSTERY bounty per character per UTC day. Bound currency is awarded on quest completion, not on bounty reveal.

## Sinks
Additional common sinks owned elsewhere (counted in telemetry `inventory_expansion_sink` and guild sinks): inventory expansion prices (`../03_systems/inventory.md`), guild creation cost (`../03_systems/guild.md`).

One shared bound-utility offer set is available from the normal craft/service NPC:
```text
offer.bound.bua_may.so_cap       -> item.consumable.bua_may.so_cap       cost 25 bound
offer.bound.bua_may.trung_cap    -> item.consumable.bua_may.trung_cap    cost 50 bound
offer.bound.bua_may.cao_cap      -> item.consumable.bua_may.cao_cap      cost 120 bound
offer.bound.bua_may.sieu_cap     -> item.consumable.bua_may.sieu_cap     cost 300 bound
offer.bound.bua_giu_bac.so_cap   -> item.consumable.bua_giu_bac.so_cap   cost 60 bound
offer.bound.bua_giu_bac.trung_cap -> item.consumable.bua_giu_bac.trung_cap cost 120 bound
offer.bound.bua_giu_bac.cao_cap  -> item.consumable.bua_giu_bac.cao_cap  cost 300 bound
```
The same items remain craftable from PvE materials/common currency, so bound currency is an alternate convenience path rather than a required progression gate.

Total addressable bound sink surface (the offer set above is the complete launch bound offer set):
```text
Lucky Charm sinks:  25 + 50 + 120 + 300 = 495
Insurance sinks:    60 + 120 + 300 = 480
Total addressable sink surface = 975 bound
```

### Bound-Purchase Output Rule
Any item created by spending `currency.bound` must be non-transferable to other characters/accounts. For every bound offer above:
```text
source binding override = CHARACTER_BOUND
binding trigger = ON_ACQUIRE
```
This source override is owned operationally by `npc_shop_catalog.md` and applies only to the bound-purchased copy; normal crafted/dropped copies retain their item-definition binding.

The output cannot be traded, auctioned, moved through Guild Storage, or moved through account storage. It also cannot be merged with an incompatible UNBOUND stack in a way that loosens binding.

This prevents converting a non-transferable currency into tradable market value.

Bound currency cannot directly buy equipment, Souls, stat points, skill points, enhancement levels, or random loot boxes.

# `currency.special`
Special currency is CHARACTER-scoped, non-transferable, and cosmetic-only at launch (ADR-0029).

## One-Time PvE Sources
Each grant is once per **character** (CHARACTER-scoped per ADR-0029 — not ACCOUNT-scoped):
```text
character first eligible clear boss.thuong_luong      -> 2 special
character first eligible clear boss.ho_tinh_chin_duoi -> 3 special
character first eligible clear boss.than_trung         -> 5 special
character first progression.story.main.complete        -> 10 special
```
The above four sources grant `20` special per character. Additional sources that bring the per-character lifetime total to the 540 stated by `integration_validation.md` (520 from Atlas + 20 base PvE) are authored in other owning files (Atlas catalog). All sources must key on `character_id`, never `account_id`. A second character earns its own grants by playing.

Stable source operation namespaces:
```text
economy.special.first_boss.<boss_id>.<character_id>
economy.special.first_story_complete.<character_id>
```
A second character earns its own grants by playing. Same-account characters do not share special.

## Launch Cosmetic Sinks
The character's initial `20 special` may redeem exactly one of these 20-special cosmetic options:
```text
20 special -> cosmetic.appearance.ao_vai_hoa_van
20 special -> cosmetic.frame.nui_thieng
```
Extended special-currency cosmetic IDs (title glows, shrine_special, frames, nameplates, auras) are listed in `cosmetic_catalog.md` SPECIAL-CURRENCY SINKS. Prices stay here.

Both cosmetics also retain deterministic `item.material.vai_hoa_van` alternatives:
```text
20 fabric -> cosmetic.appearance.ao_vai_hoa_van
30 fabric -> cosmetic.frame.nui_thieng
```

This means the launch special balance provides a cosmetic choice while the unchosen cosmetic remains collectible through normal non-power event material.

If both entitlements were already earned through material redemption before the special balance is spent, the character keeps the currency for future explicitly authored cosmetic sinks. It is not silently deleted or auto-exchanged.

No launch combat power, equipment, Soul, enhancement, drop-rate, inventory size, economy efficiency, or matchmaking benefit is sold for `currency.special`.

## Extended Special Cosmetic Sink Catalog
To provide a sink surface matching the per-character faucet (540 per character per `integration_validation.md`: 520 Atlas + 20 base PvE), the following cosmetic catalog is added. All items are CHARACTER-scoped, non-transferable, non-power, and have no material alternative — they are `currency.special` exclusive cosmetics. No `currency.special` sink may buy anything not on this list or the base 20-special options.

### Title Glows — Special Tier (Hiệu Ứng Đặc Biệt Tước Hiệu)
```text
cosmetic.title_glow.kim_quang   cost 10 special   [KIM element gold shimmer]
cosmetic.title_glow.hoa_quang   cost 10 special   [HOA element flame shimmer]
cosmetic.title_glow.thuy_linh   cost 10 special   [THUY element water shimmer]
cosmetic.title_glow.tho_bach    cost 10 special   [THO earth shimmer]
cosmetic.title_glow.moc_xanh    cost 10 special   [MOC wood shimmer]
```

### Shrine Variants — Special Tier (Miếu Linh Đặc Biệt)
```text
cosmetic.shrine_special.dai_hong   cost 20 special   [crimson spirit shrine]
cosmetic.shrine_special.ngoc_bich  cost 20 special   [jade spirit shrine]
cosmetic.shrine_special.hong_tran  cost 20 special   [rose dust shrine]
cosmetic.shrine_special.cu_thach   cost 20 special   [ancient stone shrine]
cosmetic.shrine_special.bach_ngoc  cost 20 special   [white jade shrine]
```

### Portrait Frames (Khung Chân Dung)
```text
cosmetic.frame.rong_vang    cost 25 special   [golden dragon frame]
cosmetic.frame.phuong_hoang cost 25 special   [phoenix frame]
cosmetic.frame.bach_ho      cost 25 special   [white tiger frame]
cosmetic.frame.huyen_vu     cost 25 special   [black tortoise frame]
cosmetic.frame.lan_linh     cost 25 special   [qilin frame]
```

### Nameplates (Bảng Tên Đặc Biệt)
```text
cosmetic.nameplate.ky_luat_vang cost 50 special   [gold discipline nameplate]
cosmetic.nameplate.linh_thuyen  cost 50 special   [spirit vessel nameplate]
cosmetic.nameplate.son_ha       cost 50 special   [mountains and rivers nameplate]
```

### Character Auras (Hào Quang Nhân Vật)
```text
cosmetic.aura.quy_khi   cost 40 special   [devil aura]
cosmetic.aura.linh_khi  cost 40 special   [spirit aura]
```

Extended sink summary:
```text
5 title glows   * 10  =  50 special
5 shrine variants * 20 = 100 special
5 portrait frames * 25 = 125 special
3 nameplates    * 50  = 150 special
2 character auras * 40 =  80 special
Extended subtotal       = 505 special
Existing 2 base sinks   =  40 special (20+20)
Total special sink surface = 545 special per character
```
This provides 545 addressable sinks against the 540/character faucet, a margin of only **5 units** (0.9% surplus). This buffer is tight by design-safety standards: a single atlas tier recount or any minor faucet adjustment can eliminate the absorption margin entirely. The 5-unit surplus is accepted as the launch minimum floor, conditional on the faucet being confirmed frozen at 540 per character before activation. **Any future faucet increase requires a matching sink expansion before the revision activates.** A character that unlocks all cosmetics retains the 5-unit surplus toward future explicitly authored seasonal sinks.

# Affordability Targets
Balance validation uses normal authored play, not Auction purchase assumptions.

Expected launch targets:
- during an act, story + normal combat should cover several relevant crafted pieces without needing a full 14-piece recraft,
- a full current-tier 14-piece craft is optional and may require repeatable regional play,
- +6 is an early/low-cost enhancement target,
- +8 is a normal progression target,
- +10 is a late-normal target,
- +12 is a max-level/endgame target,
- +13..+16 is aspirational optimization and never baseline story/party eligibility.

A release candidate fails economy review when normal authored sources make these targets require Auction dependence or clearly exceed the intended progression-band/endgame playtime.

Daily/SIDE/competitive bonus rewards are accelerators. Baseline affordability review must not assume perfect completion of every daily board, every PvP bonus cap, or every Guild War reward window.

# Inflation / Deflation Telemetry
In addition to `economy.md` telemetry, track by source/sink and progression tier:
```text
common_created_per_active_hour
common_sunk_per_active_hour
bound_created_per_active_hour
bound_sunk_per_active_hour
auction_tax_sink
craft_sink
enhancement_sink
inventory_expansion_sink
median_common_by_level_band
p90_common_by_level_band
median_regional_material_by_tier
```

Alert conditions are telemetry signals, not automatic live nerfs. Economy tuning changes content revision data and goes through normal activation validation.

## Numeric Alert Thresholds
The following thresholds are MANDATORY release gates. **An unset threshold is a release-review rejection.** Alerts trigger an economy review; they are not automatic live nerfs.

### Daily Sink Ratio (per UTC day, all characters combined)
```text
REJECT (release blocker) if: sunk_common / created_common < 0.60
ALERT (escalation) if: sunk_common / created_common < 0.70
```
Rationale: At least 60% of created common must be destroyed each UTC day. Below this threshold inflation outpaces sinks and wallet balances grow without bound.

### p99 Wallet Balance Weekly Growth Ceiling
```text
REJECT (release blocker) if: (p99_common_balance_week_end - p99_common_balance_week_start)
                              / p99_common_balance_week_start > 0.15
```
Rationale: The 99th-percentile wallet must not grow faster than 15% per week in steady state. Faster growth indicates a faucet is uncapped or a sink is broken.

### Per-Tier Auction Median Price Index
```text
For each equipment tier T (T1..T6):
  compute: median_auction_price(T) = median sale price of all T-tier equipment sold in 7-day window
  where tier_price_floor(T) = {T1: 500, T2: 1500, T3: 4000, T4: 10000, T5: 25000, T6: 50000}
ALERT if: median_auction_price(T) < tier_price_floor(T) * 1.5
ALERT if: median_auction_price(T) > tier_price_floor(T) * 50
REJECT (release blocker) if any sale settles below tier_price_floor(T)   (enforcement bug)
```
Rationale: sales cannot legally settle below the floor, so a median pressing on the floor signals dumping or mule traffic; prices far above floor indicate scarcity-driven inflation.

### Per-Account Net Gold Outflow Percentile
```text
For each account, over a 30-day rolling window:
  net_gain = total_common_earned_from_system - total_common_sunk - total_common_fees
             (player-to-player transfers excluded; they are covered by anti_cheat.md signals)
ALERT if: p99(net_gain_per_account_30d) > 50_000_000     (extreme faucet outlier)
ALERT if: p1(net_gain_per_account_30d) < -50_000_000
          AND the same accounts received >= 50_000_000 via player transfers in the window
          (sinking far more than earned is funded externally -> possible RMT injection)
```
Rationale: detects extreme accumulation and sinks funded by transferred currency, without flagging players who legitimately spend savings.

### Bound Faucet-to-Sink Ratio (per UTC day)
```text
ALERT if: bound_created_daily / bound_sunk_daily > 3.0
```
Rationale: Prevents the bound faucet (now daily_first-gated) from re-entering the uncapped accumulation problem.

# Validation
Reject when:
- `drop_tables.md` copies a conflicting common combat band,
- a bound source has no idempotency/reward-eligibility scope,
- Ranked daily bound counter is implemented separately per mode,
- Guild War personal bound counter resets through guild switching,
- competitive result changes the bound completion amount,
- bound utility item loses its non-bound PvE acquisition path,
- item purchased with bound currency remains UNBOUND/ACCOUNT_BOUND/player-tradable,
- bound-purchased item can enter trade/Auction/Guild Storage/account storage,
- stack/merge operation can loosen a bound-purchased copy's binding,
- special currency buys combat/progression power,
- special grant source does not key on character_id (must never key on account_id),
- either launch special cosmetic price differs from 20,
- one cosmetic redemption consumes both material and special currency,
- full common-currency overflow silently deletes a sibling earned reward.

# Invariants
```text
common = transferable circulating economy
bound = non-transferable optional utility
bound purchase output cannot become tradable market value
special = CHARACTER-scoped cosmetic-only (not account-scoped; ADR-0029)
no currency exchange
no Auction dependency for baseline progression
PvP/Guild War bound reward does not depend on winning
ranked bound cap is character-wide across ranked modes
Guild War bound cap is character-wide across guild changes
special grants key on character_id, never account_id
special launch PvE supply per character (4 canonical sources) = 20
special launch sink total catalog >= 500 per character
bound sink surface = 975 (Lucky/Insurance offers only)
dungeon bound = one DAILY_FIRST grant per character per UTC day across all dungeons
```