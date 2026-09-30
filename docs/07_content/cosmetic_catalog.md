# Launch Cosmetic Catalog
status: LOCKED

## Scope
Concrete non-power cosmetic entitlements for launch. Ownership/equip/redemption rules remain canonical in `../03_systems/cosmetics.md`.

All cosmetics below are presentation-only. They never modify stats, collision, rewards, matchmaking, drop rates, quest progress, or economy efficiency.

# TITLE — 20 Core + 107 Atlas = 127
Default scope: CHARACTER (ADR-0029). Guild cosmetics stay guild-scoped. IAP store cosmetics (`../03_systems/monetization.md`) are account-entitled.

Core launch titles (20):

| cosmetic_id | Display | Unlock source |
|---|---|---|
| `cosmetic.title.lang_da` | Người Qua Làng Đa | `progression.story.a1.complete` |
| `cosmetic.title.u_minh` | Người Đi U Minh | `progression.story.a2.complete` |
| `cosmetic.title.ben_nuoc` | Người Qua Bến Nước | `progression.story.a3.complete` |
| `cosmetic.title.deo_may` | Người Qua Đèo Mây | `progression.story.a4.complete` |
| `cosmetic.title.thanh_co` | Người Nghe Trống Thành | `progression.story.a5.complete` |
| `cosmetic.title.canh_cuoi` | Người Khép Canh Cuối | `progression.story.main.complete` |
| `cosmetic.title.tho_san_thuong_luong` | Người Săn Thuồng Luồng | first eligible clear of `boss.thuong_luong` |
| `cosmetic.title.doi_mat_than_trung` | Người Đối Mặt Thần Trùng | first eligible clear of `boss.than_trung` |
| `cosmetic.title.khac_tinh_ma_da` | Khắc Tinh Ma Da | Slay 1,000 Ma Da monsters (ADR-0024) |
| `cosmetic.title.dung_si_tru_ho` | Dũng Sĩ Trừ Hổ | Slay Boss Hổ Tinh 10 times (ADR-0024) |
| `cosmetic.title.ngu_ong_ben_do` | Ngư Ông Bến Đò | Catch 200 fish via Folk Fishing (ADR-0024) |
| `cosmetic.title.ban_tay_than` | Bàn Tay Thần | Enhance any equipment to +12 (ADR-0024) |
| `cosmetic.title.tuyet_dinh_than_binh` | Tuyệt Đỉnh Thần Binh | Enhance any equipment to +16 (ADR-0024, Golden Glow) |
| `cosmetic.title.thien_ha_de_nhat` | Thiên Hạ Đệ Nhất | Rank 1 season champion in Ranked Duel (ADR-0024, Flame Glow) |
| `cosmetic.title.trang_si_giup_doi` | Tráng Sĩ Giúp Đời | 500 Chivalry Points (ADR-0023) |
| `cosmetic.title.dai_hiep_lang_que` | Đại Hiệp Làng Quê | 2,000 Chivalry Points (ADR-0023) |
| `cosmetic.title.hiep_nghia_vo_song` | Hiệp Nghĩa Vô Song | 5,000 Chivalry Points (ADR-0023) |
| `cosmetic.title.tam_giao_vien_man` | Tâm Giao Viên Mãn | Linh Thú bond 100 |
| `cosmetic.title.pvp.than_thoai` | Thần Thoại Đấu Trường | PvP season settlement, `pvp.tier.mythic` (§ Competitive Season Rewards) |
| `cosmetic.title.guild_war.hung_binh` | Hùng Binh Ngũ Ấn | Guild War season settlement, guild MMR >= 1800 (§ Competitive Season Rewards) |

Atlas titles (107) — concrete roster owned by `atlas_catalog.md`:

| Count | Pattern | Example |
|---|---|---|
| 58 | authored per page (`Title (T3)` column of `atlas_catalog.md`; usually `cosmetic.title.atlas.<quai_dam_key>`) | `cosmetic.title.atlas.dom_dom_ma` — Master that monster's T3 kill count |
| 25 | `cosmetic.title.atlas.hon_*` | `cosmetic.title.atlas.hon_coc` — Reach Soul Lv5 (tier 3) |
| 8 | `cosmetic.title.atlas.di_*` | `cosmetic.title.atlas.di_than_trung` — Witness 10 boss relics (tier 3) |
| 13 | `cosmetic.title.atlas.<co_vat_key>` | `cosmetic.title.atlas.ca_chep_hoa_rong` — Catch 10 Cá Chép Hóa Rồng (tier 3) |
| 3 | Milestone | `cosmetic.title.atlas.nha_suu_tam` (20 mastered), `cosmetic.title.atlas.hoc_gia_dan_gian` (50), `cosmetic.title.atlas.bach_khoa_dan_gian` (104 mastered) |

All atlas titles are `CHARACTER` scope (ADR-0029; `atlas.md` is the canonical owner of Atlas scoping), purely cosmetic, glowing per `atlas_catalog.md` tier 3, idempotent per `atlas.tier.<character_id>.<atlas_page_id>.<tier>` (`../03_systems/atlas.md`).

Story and feat titles are granted idempotently when their triggering condition or progression flag commits.

# PROFILE_FRAME — 9
Default scope: CHARACTER (ADR-0029). Play-earned frames are character-scoped.

| cosmetic_id | Display | Unlock source |
|---|---|---|
| `cosmetic.frame.ben_da` | Khung Bến Đa | complete `quest.side.a1.chiec_non_ben_da` |
| `cosmetic.frame.ho_tinh` | Khung Hồ Tinh | first eligible clear of `boss.ho_tinh_chin_duoi` |
| `cosmetic.frame.nui_thieng` | Khung Núi Thiêng | one-time alternative redemption defined below |
| `cosmetic.frame.pvp.bac` | Khung Bạc Đấu Trường | PvP season settlement, `pvp.tier.silver` |
| `cosmetic.frame.pvp.vang` | Khung Vàng Đấu Trường | PvP season settlement, `pvp.tier.gold` |
| `cosmetic.frame.pvp.ngoc` | Khung Ngọc Đấu Trường | PvP season settlement, `pvp.tier.jade` |
| `cosmetic.frame.pvp.linh` | Khung Linh Đấu Trường | PvP season settlement, `pvp.tier.spirit` |
| `cosmetic.frame.pvp.than_thoai` | Khung Thần Thoại Đấu Trường | PvP season settlement, `pvp.tier.mythic` |
| `cosmetic.frame.guild_war.chien_ky` | Khung Chiến Kỳ | Guild War season settlement, eligible member |

## `cosmetic.frame.nui_thieng`
Display: **Khung Núi Thiêng**

The final-story completion no longer grants this frame directly. Instead it contributes to the character `currency.special` supply in `economy_catalog.md`, and the player may choose this frame as one cosmetic sink.

Material route:
```text
required_item_id = item.material.vai_hoa_van
required_quantity = 30
```

Special-currency route:
```text
required_currency_id = currency.special
required_amount = 20
```

Both routes grant the same character entitlement and follow the alternative-redemption semantics in `../03_systems/cosmetics.md`.

# CHARACTER_APPEARANCE — 4
Default scope: CHARACTER (ADR-0029). Play-earned appearances are character-scoped.

These are overlay/appearance entitlements. They do not occupy the gameplay `costume` equipment slot and do not alter equipment element/Formation state.

## `cosmetic.appearance.non_la_moc`
Display: **Nón Lá Mộc**  
Unlock: complete `quest.side.a1.luy_tre_keu_dem`.

Visual direction: simple stylized leaf hat silhouette; avoid logos/modern branding.

## `cosmetic.appearance.ao_toi_la`
Display: **Áo Tơi Lá**  
Unlock: complete `quest.side.a2.nguoi_di_rung_muon`.

Visual direction: stylized rain cape/leaf covering appropriate to the fictional wet-forest setting.

## `cosmetic.appearance.khan_ben_nuoc`
Display: **Khăn Bến Nước**  
Unlock: complete `quest.side.a3.den_ben_do`.

Visual direction: simple cloth head/neck accessory; do not imitate a specific ethnic-community ceremonial garment without separate cultural review.

## `cosmetic.appearance.ao_vai_hoa_van`
Display: **Áo Vải Hoa Văn**

Material route:
```text
required_item_id = item.material.vai_hoa_van
required_quantity = 20
```

Special-currency route:
```text
required_currency_id = currency.special
required_amount = 20
```

The player chooses one route. Settlement consumes exactly that route's input and grants the same character entitlement atomically under `../03_systems/cosmetics.md`.

If the entitlement already exists, both redemption routes become no-op/already-owned operations and consume nothing. There is no automatic material/currency exchange.

Visual direction: fictional Vietnamese-inspired woven/floral/geometric motifs. Do not copy protected modern textile designs or present one minority/community's ceremonial clothing as generic fantasy costume.

# Special-Currency Choice
The base launch PvE route exposes `20 currency.special` per character under `economy_catalog.md`. Atlas adds up to `520` more per character (104 pages × T1=1 + T2=2 + T3=2) toward the character cap `1,000,000` (ADR-0029). Total character faucet = 540, within the ~545 sink surface.

At launch the initial 20 special alone may unlock **one** of:
```text
20 special -> cosmetic.appearance.ao_vai_hoa_van
20 special -> cosmetic.frame.nui_thieng
```

The other entitlement remains obtainable through its Vải Hoa Văn material route. Therefore spending special currency creates a cosmetic choice, not an inaccessible collection gap:
```text
Ao Vai Hoa Van -> 20 fabric
Khung Nui Thieng -> 30 fabric
```

If a player earns both cosmetics through material first, the character may retain its 20 special for future explicitly authored cosmetic sinks. Currency is never auto-converted or deleted merely because current launch entitlements are already owned.

# FOLKLORE FEATS CATALOG
Feat tracking rules and schema owned by `../03_systems/cosmetics.md`. This section owns the concrete feat definitions.

Default scope: CHARACTER (ADR-0029).

| feat_id | Feat type | Threshold | Tracked event | Cosmetic reward |
|---|---|---:|---|---|
| `feat.combat.slay_ma_da` | `KILL_COUNT` | 1,000 | `MONSTER_KILLED` where `monster_family = ma_da` (all ma_da variants) | `cosmetic.title.khac_tinh_ma_da` |
| `feat.combat.slay_ho_tinh_boss` | `BOSS_KILL_COUNT` | 10 | `BOSS_DEFEATED` where `boss_id = boss.ho_tinh` | `cosmetic.title.dung_si_tru_ho` |
| `feat.life.catch_fish` | `GATHER_COUNT` | 200 | `FISH_CAUGHT` (any fish via Folk Fishing) | `cosmetic.title.ngu_ong_ben_do` |
| `feat.craft.enhance_12` | `ENHANCEMENT_FLAG` | +12 | `ENHANCEMENT_COMPLETED` where `result_level = 12` | `cosmetic.title.ban_tay_than` |
| `feat.craft.enhance_16` | `ENHANCEMENT_FLAG` | +16 | `ENHANCEMENT_COMPLETED` where `result_level = 16` | `cosmetic.title.tuyet_dinh_than_binh` |
| `feat.pvp.rank1_season` | `PVP_RANK_FLAG` | Rank 1 | `PVP_SEASON_SETTLED` where `rank = 1` and mode = ranked_duel | `cosmetic.title.thien_ha_de_nhat` |

## Feat Idempotency Keys
```text
character_id + feat_id + milestone_threshold
```
Server increments `counter_value` on each qualifying authoritative event. When `counter_value` reaches a milestone `threshold`, the server inserts the `character_feat_milestones` row and emits the cosmetic grant under the idempotency key in the same transaction. A retry when the row exists is a no-op.

## Feat Counter Rules
- `KILL_COUNT` and `GATHER_COUNT` counters are cumulative across all sessions and seasons; they are never reset.
- `ENHANCEMENT_FLAG` and `PVP_RANK_FLAG` are set once and never cleared.
- `feat.combat.slay_ma_da` counts kills of all `ma_da` family members: `monster.ben_nuoc_den.ma_da`, `monster.ben_nuoc_den.ma_da_gia`, and `boss.ma_da_chua` kills (each boss kill = 1 toward the counter).

# CURRENCY.COMMON COSMETIC SINKS
Non-power cosmetic items purchasable with `currency.common`. No fourth currency is introduced. All items are character-scoped (ADR-0029). Prices are authoritative per `economy_catalog.md`.

Note: the prior draft of this section contained 3+3+3=9 entries with TODO prices and different IDs. Those entries are superseded by the 6+6+8=20 entries below, which are reconciled with the `economy_catalog.md` authoritative price list. The IDs in the prior draft (e.g. `hoa_van_lang`) were placeholder names that did not match the canonical IDs in `economy_catalog.md` (e.g. `trung_nguyen`); the entries below use the canonical IDs.

## Guild Stone Inscriptions (Bia Đá Danh Vọng Kiểu)
Visual inscription style applied to the character's Guild Stone entry. Cosmetic only; no stat effect.

| cosmetic_id | Display (vi-VN) | `currency.common` cost | Unlock condition |
|---|---|---:|---|
| `cosmetic.guild_stone.inscription.trung_nguyen` | Trung Nguyên | 50,000 | Purchase with `currency.common` |
| `cosmetic.guild_stone.inscription.long_van` | Long Vân | 80,000 | Purchase with `currency.common` |
| `cosmetic.guild_stone.inscription.phuong_vi` | Phượng Vĩ | 120,000 | Purchase with `currency.common` |
| `cosmetic.guild_stone.inscription.ngoc_bich` | Ngọc Bích | 180,000 | Purchase with `currency.common` |
| `cosmetic.guild_stone.inscription.kim_bach` | Kim Bạch | 250,000 | Purchase with `currency.common` |
| `cosmetic.guild_stone.inscription.thien_long` | Thiên Long | 400,000 | Purchase with `currency.common` |

## Shrine Variants (Miếu / Đàn Thờ)
Visual theme applied to the character's personal shrine display in safe anchors. Non-power presentation only.

| cosmetic_id | Display (vi-VN) | `currency.common` cost | Description |
|---|---|---:|---|
| `cosmetic.shrine.lua_do` | Miếu Lửa Đỏ | 60,000 | Flame-red shrine motif |
| `cosmetic.shrine.thuy_ngoc` | Miếu Thủy Ngọc | 100,000 | Water-jade shrine motif |
| `cosmetic.shrine.moc_xanh` | Miếu Mộc Xanh | 150,000 | Forest-green shrine motif |
| `cosmetic.shrine.tho_vang` | Miếu Thổ Vàng | 220,000 | Earth-gold shrine motif |
| `cosmetic.shrine.kim_trang` | Miếu Kim Trắng | 320,000 | Metal-white shrine motif |
| `cosmetic.shrine.linh_khoi` | Miếu Linh Khói | 500,000 | Spirit-smoke shrine motif |

## Title Glows (Hiệu Ứng Tước Hiệu)
Visual glow effect behind the character's displayed title text. No stat effect. Applied per character.

| cosmetic_id | Display (vi-VN) | `currency.common` cost | Glow description |
|---|---|---:|---|
| `cosmetic.title_glow.do_quang` | Ánh Đỏ Quang | 30,000 | Red glow |
| `cosmetic.title_glow.xanh_nhat` | Ánh Xanh Nhạt | 30,000 | Light-blue glow |
| `cosmetic.title_glow.vang_nhat` | Ánh Vàng Nhạt | 30,000 | Pale-gold glow |
| `cosmetic.title_glow.tim_nhat` | Ánh Tím Nhạt | 30,000 | Soft-purple glow |
| `cosmetic.title_glow.bach_nhat` | Ánh Bạch Nhạt | 30,000 | White shimmer |
| `cosmetic.title_glow.hong_phuc` | Ánh Hồng Phúc | 75,000 | Rose-blessing glow |
| `cosmetic.title_glow.linh_hoa` | Ánh Linh Hoa | 75,000 | Spirit-flower glow |
| `cosmetic.title_glow.thien_van` | Ánh Thiên Vân | 120,000 | Celestial-cloud glow |

## Common Sink Redemption Rules
- Purchase is a server-authoritative atomic operation: validate entitlement absent + validate `currency.common` balance + debit + grant entitlement.
- Idempotency key: `common_cosmetic.<cosmetic_id>.<character_id>`.
- If entitlement already owned, operation is a no-op; no currency is consumed.
- No randomized outcome; each purchase grants exactly the listed cosmetic.

# SEASONAL COSMETICS
Cosmetics granted by the seasonal track (see `../03_systems/seasons.md`). Free-track cosmetics are character-scoped (ADR-0029). Paid-track cosmetics are unlocked under an `ACCOUNT_SCOPED_ACCESS` entitlement; each character claims their own character-scoped reward instance (ADR-0041). All are purely cosmetic.

Season 0 (Làng Đa) FREE track:
| cosmetic_id | Display (vi-VN) | Unlock | scope |
|---|---|---|---|
| `cosmetic.title.season.0.lang_da_ky_ghe` | Kỳ Ghé Làng Đa | Complete all 10 Season 0 Atlas pages at T1 | CHARACTER |
| `cosmetic.frame.season.0` | Khung Mùa Làng Đa | Master all 10 Season 0 Atlas pages at T3 | CHARACTER |
| `cosmetic.shrine.season.0` | Miếu Mùa Làng Đa | Reach T2 on all 10 Season 0 Atlas pages | CHARACTER |

Season 0 PAID track (unlocked per character via `ACCOUNT_SCOPED_ACCESS` entitlement; extra cosmetic tiers only; no power):
| cosmetic_id | Display (vi-VN) | Unlock | scope |
|---|---|---|---|
| `cosmetic.title.season.0.paid.dem_lang` | Đêm Làng | Paid season 0 track (account-scoped access) | CHARACTER |
| `cosmetic.frame.season.0.paid` | Khung Đêm Làng | Paid season 0 track (account-scoped access) | CHARACTER |
| `cosmetic.emote.season.0.paid.chap_tay` | Chắp Tay | Paid season 0 track (account-scoped access) | CHARACTER |
Seasonal FREE/PAID tracks for seasons 1–5:

| n | free title | free frame | free shrine | paid title | paid frame | paid emote |
|---|---|---|---|---|---|---|
| 1 | `cosmetic.title.season.1.u_minh_suong` | `cosmetic.frame.season.1` | `cosmetic.shrine.season.1` | `cosmetic.title.season.1.paid` | `cosmetic.frame.season.1.paid` | `cosmetic.emote.season.1.paid` |
| 2 | `cosmetic.title.season.2.ben_den` | `cosmetic.frame.season.2` | `cosmetic.shrine.season.2` | `cosmetic.title.season.2.paid` | `cosmetic.frame.season.2.paid` | `cosmetic.emote.season.2.paid` |
| 3 | `cosmetic.title.season.3.deo_suong` | `cosmetic.frame.season.3` | `cosmetic.shrine.season.3` | `cosmetic.title.season.3.paid` | `cosmetic.frame.season.3.paid` | `cosmetic.emote.season.3.paid` |
| 4 | `cosmetic.title.season.4.thanh_mua` | `cosmetic.frame.season.4` | `cosmetic.shrine.season.4` | `cosmetic.title.season.4.paid` | `cosmetic.frame.season.4.paid` | `cosmetic.emote.season.4.paid` |
| 5 | `cosmetic.title.season.5.nui_mua` | `cosmetic.frame.season.5` | `cosmetic.shrine.season.5` | `cosmetic.title.season.5.paid` | `cosmetic.frame.season.5.paid` | `cosmetic.emote.season.5.paid` |

Free track = CHARACTER. Paid track = CHARACTER (per-character claim under ACCOUNT_SCOPED_ACCESS entitlement). Displays: U Minh Sương / Bến Đen / Đèo Sương / Thành Mùa / Núi Mùa.

Cycle rollover rule: seasons repeat across 6 regional cycles (`cycle = floor(season_number / 6)`, `region_index = season_number mod 6`). Season 6 and later re-use the established stable IDs for `region_index` (Season 6 re-uses Season 0 IDs, Season 7 re-uses Season 1, etc. per `seasons.md`). Unlocks are permanent and idempotent; no duplicate database rows are created on repeat cycles.

## Seasonal Atlas T3 titles — 60
Finite expansion: every `atlas.page.season.<n>.<key>` T3 title column in `atlas_catalog.md` is `cosmetic.title.season.<n>.<key>`, CHARACTER, unlock = Master that page. Compiler expands `10 pages × seasons 0..5 = 60` IDs. Missing title ID fails activation.

# IAP STORE COSMETICS — 13
Account-entitled (`account_cosmetic_entitlements`). All 13 launch store cosmetic products (and the bundle) are **store exclusive** with no in-game faucet or currency equivalent. They are account-wide wardrobe unlocks equippable on any character of the account. They grant zero combat power, zero stats, and zero progression efficiency. Prices owned by `economy_catalog.md` / monetization products. Namespace `cosmetic.iap.*`.

| product_id | cosmetic_id | Display | Exclusivity |
|---|---|---|---|
| `product.cosmetic.character_skin.co_tam_truyen` | `cosmetic.iap.appearance.co_tam_truyen` | Cổ Tâm Truyện | store exclusive |
| `product.cosmetic.character_skin.co_tien` | `cosmetic.iap.appearance.co_tien` | Cô Tiên | store exclusive |
| `product.cosmetic.character_skin.vo_quan_thanh_co` | `cosmetic.iap.appearance.vo_quan_thanh_co` | Võ Quan Thành Cổ | store exclusive |
| `product.cosmetic.character_skin.nu_tuong_trong_dong` | `cosmetic.iap.appearance.nu_tuong_trong_dong` | Nữ Tướng Trống Đồng | store exclusive |
| `product.cosmetic.weapon_trail.phuong_hoang_vu` | `cosmetic.iap.trail.phuong_hoang_vu` | Phượng Hoàng Vũ | store exclusive |
| `product.cosmetic.weapon_trail.bao_gam` | `cosmetic.iap.trail.bao_gam` | Báo Gấm | store exclusive |
| `product.cosmetic.weapon_trail.long_hoa` | `cosmetic.iap.trail.long_hoa` | Long Hoa | store exclusive |
| `product.cosmetic.emote.bai_chao_lang` | `cosmetic.iap.emote.bai_chao_lang` | Bái Chao Làng | store exclusive |
| `product.cosmetic.emote.vo_tay_thang_tran` | `cosmetic.iap.emote.vo_tay_thang_tran` | Vỗ Tay Thắng Trận | store exclusive |
| `product.cosmetic.emote.ngoi_thien_dinh` | `cosmetic.iap.emote.ngoi_thien_dinh` | Ngồi Thiền Đình | store exclusive |
| `product.cosmetic.portrait_frame.thien_long_store` | `cosmetic.iap.frame.thien_long` | Thiên Long | store exclusive |
| `product.cosmetic.nameplate.hun_thuoc_co` | `cosmetic.iap.nameplate.hun_thuoc_co` | Hun Thuốc Cỏ | store exclusive |
| `product.cosmetic.title_glow.long_nhan_store` | `cosmetic.iap.title_glow.long_nhan` | Long Nhan | store exclusive |

`product.cosmetic.bundle.nguoi_hung_lang_da` is a store-exclusive bundle granting the three: `cosmetic.iap.appearance.co_tam_truyen` + `cosmetic.iap.frame.thien_long` + `cosmetic.iap.emote.bai_chao_lang`. Bundle is not a 14th cosmetic_id.
# SPECIAL-CURRENCY SINKS — 20
CHARACTER play sinks. Prices owned by `economy_catalog.md`. Do not invent `item.cosmetic.*`.

| cosmetic_id | kind |
|---|---|
| `cosmetic.title_glow.kim_quang` | title_glow |
| `cosmetic.title_glow.hoa_quang` | title_glow |
| `cosmetic.title_glow.thuy_linh` | title_glow |
| `cosmetic.title_glow.tho_bach` | title_glow |
| `cosmetic.title_glow.moc_xanh` | title_glow |
| `cosmetic.shrine_special.dai_hong` | shrine |
| `cosmetic.shrine_special.ngoc_bich` | shrine |
| `cosmetic.shrine_special.hong_tran` | shrine |
| `cosmetic.shrine_special.cu_thach` | shrine |
| `cosmetic.shrine_special.bach_ngoc` | shrine |
| `cosmetic.frame.rong_vang` | frame |
| `cosmetic.frame.phuong_hoang` | frame |
| `cosmetic.frame.bach_ho` | frame |
| `cosmetic.frame.huyen_vu` | frame |
| `cosmetic.frame.lan_linh` | frame |
| `cosmetic.nameplate.ky_luat_vang` | nameplate |
| `cosmetic.nameplate.linh_thuyen` | nameplate |
| `cosmetic.nameplate.son_ha` | nameplate |
| `cosmetic.aura.quy_khi` | aura |
| `cosmetic.aura.linh_khi` | aura |

`cosmetic.appearance.ao_vai_hoa_van` and `cosmetic.frame.nui_thieng` also have special-currency routes (already counted in APPEARANCE_PLAY / PROFILE_FRAME_PLAY).

# GUILD-SCOPED COSMETICS — 5
These entitlements belong to the guild, not a member account. They are visual rewards for the existing Ritual streak and Guild War season placement; no new guild currency/system is added.

## `cosmetic.guild.crest.ritual_4`
Category: `GUILD_CREST_ACCENT`  
Display: **Nét Tre**  
Unlock: guild reaches a `4` consecutive Ritual-cycle streak.

## `cosmetic.guild.banner.ritual_8`
Category: `GUILD_BANNER`  
Display: **Cờ Hoa Văn**  
Unlock: guild reaches an `8` consecutive Ritual-cycle streak.

## `cosmetic.guild.shrine.ritual_12`
Category: `GUILD_SHRINE_VISUAL`  
Display: **Đèn Hội Tụ**  
Unlock: guild reaches a `12` consecutive Ritual-cycle streak.

## `cosmetic.guild.shrine.guild_war_top10`
Category: `GUILD_SHRINE_VISUAL`  
Display: **Đàn Ngũ Ấn**  
Unlock: Guild War season settlement, guild leaderboard rank 1..10 (§ Competitive Season Rewards).

## `cosmetic.guild.banner.guild_war_champion`
Category: `GUILD_BANNER`  
Display: **Cờ Bá Chủ Ngũ Ấn**  
Unlock: Guild War season settlement, guild leaderboard rank 1.

## Competitive Season Rewards
Settled once per season at season end (`../03_systems/pvp.md` § Rewards, `../03_systems/guild_war.md` § Season Rewards). Every ID is a stable launch ID; a later season re-grants nothing new (duplicate source = success, no replacement).
```text
PvP (CHARACTER scope; requires >= 10 reward-eligible ranked completions in the season, any ranked mode)
  tier = highest final season_rating tier across ranked modes at settlement; a tier grants its row and every lower row
  pvp.tier.silver  -> cosmetic.frame.pvp.bac
  pvp.tier.gold    -> cosmetic.frame.pvp.vang
  pvp.tier.jade    -> cosmetic.frame.pvp.ngoc
  pvp.tier.spirit  -> cosmetic.frame.pvp.linh
  pvp.tier.mythic  -> cosmetic.frame.pvp.than_thoai + cosmetic.title.pvp.than_thoai
  Ranked Duel season rank 1 -> cosmetic.title.thien_ha_de_nhat (feat.pvp.rank1_season)
Guild War (eligibility = guild_war.md § Season Rewards)
  every eligible member                         -> cosmetic.frame.guild_war.chien_ky (CHARACTER)
  eligible member, guild final MMR >= 1800      -> cosmetic.title.guild_war.hung_binh (CHARACTER)
  guild leaderboard rank 1..10, >= 10 matches   -> cosmetic.guild.shrine.guild_war_top10 (GUILD)
  guild leaderboard rank 1, >= 10 matches       -> cosmetic.guild.banner.guild_war_champion (GUILD)
```

Ritual-streak cosmetics remain cosmetic-only; missing/breaking a streak never removes already unlocked guild cosmetic entitlement unless the guild itself is deleted under canonical guild lifecycle.

# Source / Duplicate Semantics
Direct source grants use stable operation keys:
```text
cosmetic.story.<cosmetic_id>.<character_id>
cosmetic.quest.<quest_id>.<cosmetic_id>.<character_id>
cosmetic.boss.<boss_id>.<cosmetic_id>.<character_id>
cosmetic.guild.ritual.<guild_id>.<streak_threshold>.<cosmetic_id>
cosmetic.pvp.season.<season_id>.<cosmetic_id>.<character_id>
cosmetic.guild_war.season.<season_id>.<cosmetic_id>.<character_id | guild_id>
cosmetic.iap.<cosmetic_id>.<account_id>
```

Redemption uses one stable character-scoped entitlement operation identity plus the selected input route. A retry cannot consume both routes.

If the entitlement already exists, a duplicate source completes successfully without granting currency/material replacement.

# Spirit Surge Cosmetic Material
`drop_tables.md` may award `item.material.vai_hoa_van` from the optional Spirit Surge daily-first reward and from the Weekly Highlight Bonus (at most one per character per week). Its two launch sinks are now concrete:
```text
20 -> cosmetic.appearance.ao_vai_hoa_van
30 -> cosmetic.frame.nui_thieng
```
Neither sink creates gameplay power.

# Cultural Review
Character/guild cosmetic concepts that directly reference a specific living community, religious vestment, sacred symbol, military insignia, or historical regalia require explicit cultural/reference review before art production.

Generic village/travel/craft motifs may be stylized, but should still read Vietnamese rather than generic East-Asian fantasy.

Paid cosmetics never depict a named historical person or a deified figure; store skins use archetypes (e.g. `nu_tuong_trong_dong`, `vo_quan_thanh_co`). Motifs from non-Vietnamese mythology (e.g. Tam Muội Chân Hỏa, snow leopard) are not used for shipping IDs.

# Validation
Reject:
- unknown quest/boss/progression/material/currency source,
- cosmetic effect containing gameplay stat/reward modifier,
- appearance changing gameplay equipment element/slot state,
- material/currency redemption with randomized result,
- special-currency amount differing from `economy_catalog.md`,
- `cosmetic.frame.nui_thieng` also being directly granted by story completion,
- one redemption attempt consuming both material and special currency,
- guild-scoped entitlement treated as personal tradable asset,
- duplicate entitlement converted into currency automatically.

# Invariants
```text
TITLE_PLAY = 127 (20 core + 107 atlas)
PROFILE_FRAME_PLAY = 9
APPEARANCE_PLAY = 4
GUILD = 5
PLAY_PLUS_GUILD = 145
COMMON_SINKS = 20
SPECIAL_CURRENCY_SINKS = 20
SEASONAL_ATLAS_TITLES = 60
SEASON_FREE = 18
SEASON_PAID = 18
IAP_STORE_IDS = 13
TOTAL_STABLE_COSMETIC_IDS = 294
cosmetic power = 0
play frames/appearances/titles/common sinks/season free = CHARACTER (ADR-0029)
IAP store cosmetics = ACCOUNT (account_cosmetic_entitlements); season paid = CHARACTER (claimed via ACCOUNT_SCOPED_ACCESS)
guild crest/banner/shrine = GUILD
play source keys use <character_id>
IAP source keys use <account_id>
```
