# Seasons
status: LOCKED

## Scope
Defines the live-ops season cadence, themed region rotation, seasonal Atlas chapters, seasonal Guild Stone categories, seasonal cosmetic track, server schedule derivation, and retention invariants. Seasons are load-bearing infrastructure delivering the "seasonal horizontal prestige (cosmetic/Atlas/Guild Stone/fishing collections)" promised in `../00_context/vision.md` and `../00_context/constraints.md`.

At ~2,000 hours to Level 60, the game spans two to four real-world years per player. Seasons are not optional enrichment; they are the scheduled cadence that replenishes horizontal collection goals for long-term players.

Decision: `../11_decisions/0036-seasons-as-launch-infrastructure.md`.

## Season Cadence
```text
season_duration_weeks = 8
season_duration_seconds = 8 * 7 * 86400 = 4,838,400
SEASON_EPOCH_UTC_SECONDS = 1799020800   # 2027-01-04T00:00:00Z Monday; commercial launch must not precede this instant. Slipping launch requires a content revision that replaces this constant with another Monday 00:00 UTC only.
```

Season number is derived deterministically and is restart-safe:
```text
season_number = floor((server_utc_seconds - SEASON_EPOCH_UTC_SECONDS) / season_duration_seconds)
season_region_index = season_number mod 6
```

No season metadata row needs to be stored at the top level. An outage gap or server restart within a season period does not advance the season counter; the computation is stateless.

## Alignment with Ranked PvP Seasons
The season boundary is co-incident with the ranked PvP season reset defined in `pvp.md` (also 8 weeks). Both resets fire on the same UTC Monday 00:00 boundary. The PvP rating soft-reset (`pvp.md`) and the seasonal Atlas/cosmetic rotation are driven by the same `season_number` derivation. No duplicate boundary event is emitted.

## Themed Region Rotation
One region is "featured" per season, derived from `season_region_index`:

| region_index | featured_zone_id | Display name (vi-VN) |
|:---:|---|---|
| 0 | `zone.lang_da` | Làng Đa |
| 1 | `zone.rung_u_minh` | Rừng U Minh |
| 2 | `zone.ben_nuoc_den` | Bến Nước Đen |
| 3 | `zone.deo_may` | Đèo Mây |
| 4 | `zone.thanh_co` | Thành Cổ |
| 5 | `zone.nui_thieng` | Núi Thiêng |

### Cycle Rollover and Stable-ID Identity Rule
The 6-region rotation repeats continuously:
```text
cycle_number        = floor(season_number / 6)
season_region_index = season_number mod 6
```
- **Stable ID Reuse**: Content rosters for Atlas pages, seasonal monsters, Guild Stone categories, and cosmetic reward tiers are pinned to the 6 canonical regions (`season_region_index = 0..5`). When `season_number >= 6` (Cycle 1 and later), the runtime re-uses the established stable IDs corresponding to `season_region_index`:
  - Season 6 re-runs Season 0 (`atlas.page.season.0.*`, `cosmetic.*.season.0.*`, `guild_stone.season.0.*`, and Season 0 monster variants).
  - Season 7 re-runs Season 1, and so forth for subsequent cycles.
- **Anti-FOMO / Catch-Up**: New players or characters who missed an earlier season can earn those Atlas pages and cosmetics when the rotation returns to that region.
- **Idempotency across Cycles**: Atlas pages and cosmetics are permanent once unlocked. If a character already unlocked an item in an earlier cycle, the grant operation is a clean idempotent no-op and creates no duplicate database row.
- **Paid Season Pass**: Purchasing `product.service.season_track.<season_number>` creates an entitlement whose claims stay open until `claim_deadline_at` = season end + 14 days. If a character on the account already owns a cosmetic tier from a prior cycle, the client displays it as already claimed; characters that do not yet own it may claim it.
- **Future Content Expansions**: If future live-ops authors entirely new cosmetic assets for subsequent cycles instead of rotating existing folklore cosmetics, a new content revision will introduce new catalog IDs and an ADR before that cycle activates.

Being in or visiting the featured region makes the seasonal Atlas chapter available and activates the seasonal Guild Stone category. The region's maps, monsters, and dungeon are always accessible regardless of season; the featured status adds collection content only and never gates progression.
## Seasonal Atlas Chapter
Each season adds one Atlas chapter of exactly 10 pages tied to the featured region's folklore themes. Seasonal Atlas pages use the namespace:
```text
atlas.page.season.<season_region_index>.<page_key>
```
Where `season_region_index = season_number mod 6` in `[0..5]`. Stable content catalog IDs are strictly bound to `0..5`; `season_number` is the temporal sequence counter.

Seasonal Atlas page rules:
- Character-scoped, identical to launch Atlas pages (`atlas.md`).
- Same three-tier model (Seen / Studied / Mastered); seasonal tiers grant cosmetics only and **0 `currency.special`** (`../07_content/atlas_catalog.md` § Seasonal Special Budget).
- **Permanent once unlocked**: seasonal Atlas progress is never removed at season end or at any future boundary.
- A character who misses a season may earn those Atlas pages when the rotation returns to that region (every 6 seasons ≈ 48 weeks).
- Unlock triggers follow the same server-authoritative event model as launch Atlas pages: `MONSTER_KILLED`, `FISH_CAUGHT`, `CHEST_OPENED`, `DISH_COOKED`, `BOSS_DEFEATED`.
- Seasonal pages are counted separately from the 104 launch pages (see `atlas.md` and `atlas_catalog.md`).

Per-season page breakdown:
```text
seasonal pages per season = 10
breakdown: 6 quai_dam + 2 co_vat + 2 di_tich = 10
```

Concrete page rosters per season are owned by `../07_content/atlas_catalog.md`.

## Seasonal Guild Stone Category
Each season introduces one themed Guild Stone inscription category linked to the featured region. The Guild Stone and category completion rule are owned by `guild.md` § Guild Stone; this file lists only the category IDs.

Seasonal inscription IDs:
```text
guild_stone.season.0.lang_da
guild_stone.season.1.rung_u_minh
guild_stone.season.2.ben_nuoc_den
guild_stone.season.3.deo_may
guild_stone.season.4.thanh_co
guild_stone.season.5.nui_thieng
```

Rules:
- Non-power guild prestige entry contributing to the guild's Guild Stone count (`guild.md`).
- Grants no character cosmetic; seasonal character cosmetics come only from the seasonal Atlas track.
- The category is a permanent addition; a guild's total Guild Stone count never decreases at season end.
- Inscription is available during the season it was introduced and on every repeat of that region.

## Paid Season Track Access
The paid cosmetic season track (`product.service.season_track.<season_number>`) is defined and owned by `monetization.md`. The purchase creates one **account-scoped access entitlement** covering all characters on the purchasing account until `claim_deadline_at` (season end + 14 days). Each character claims their own reward tiers independently; the access entitlement is not consumed by the first character to claim. See `monetization.md` (Season Track) and `account_storage.md` (Account-Scoped Access Entitlements) for the authoritative claim model and composite idempotency key.

Paid track adds extra cosmetic tiers only. It is not a power gate. Free track remains the 3 cosmetics below.

## Seasonal Cosmetic Track
Each season exposes a free cosmetic track (3 cosmetics, no paid gate, no battle-pass power) plus optional paid extra tiers:

| Cosmetic | ID Schema | Unlock |
|---|---|---|
| Seasonal title (free) | `cosmetic.title.season.<season_region_index>.<key>` | Complete all 10 seasonal Atlas pages (T1 Seen) |
| Seasonal profile frame (free) | `cosmetic.frame.season.<season_region_index>` | Master all 10 seasonal Atlas pages (T3) |
| Seasonal shrine visual (free) | `cosmetic.shrine.season.<season_region_index>` | Reach T2 (Studied) on all 10 seasonal Atlas pages |
| Paid extra title | `cosmetic.title.season.<season_region_index>.paid[.<key>]` (key only in season 0) | Paid track claim |
| Paid extra frame | `cosmetic.frame.season.<season_region_index>.paid` | Paid track claim |
| Paid extra emote | `cosmetic.emote.season.<season_region_index>.paid[.<key>]` (key only in season 0) | Paid track claim |

Where `season_region_index = season_number mod 6` in `[0..5]` for all content catalog item definitions. The live purchase product `product.service.season_track.<season_number>` and grant audit log preserve the temporal `season_number`.
Season 0:
```text
reward_tier.season.0.free.title  -> cosmetic.title.season.0.lang_da_ky_ghe
reward_tier.season.0.free.frame  -> cosmetic.frame.season.0
reward_tier.season.0.free.shrine -> cosmetic.shrine.season.0
reward_tier.season.0.paid.title  -> cosmetic.title.season.0.paid.dem_lang
reward_tier.season.0.paid.frame  -> cosmetic.frame.season.0.paid
reward_tier.season.0.paid.emote  -> cosmetic.emote.season.0.paid.chap_tay
```

Seasons 1–5 (same free/paid shape; IDs owned by `../07_content/cosmetic_catalog.md`):
```text
reward_tier.season.1.free.title  -> cosmetic.title.season.1.u_minh_suong
reward_tier.season.1.free.frame  -> cosmetic.frame.season.1
reward_tier.season.1.free.shrine -> cosmetic.shrine.season.1
reward_tier.season.1.paid.title  -> cosmetic.title.season.1.paid
reward_tier.season.1.paid.frame  -> cosmetic.frame.season.1.paid
reward_tier.season.1.paid.emote  -> cosmetic.emote.season.1.paid
reward_tier.season.2.free.title  -> cosmetic.title.season.2.ben_den
reward_tier.season.2.free.frame  -> cosmetic.frame.season.2
reward_tier.season.2.free.shrine -> cosmetic.shrine.season.2
reward_tier.season.2.paid.title  -> cosmetic.title.season.2.paid
reward_tier.season.2.paid.frame  -> cosmetic.frame.season.2.paid
reward_tier.season.2.paid.emote  -> cosmetic.emote.season.2.paid
reward_tier.season.3.free.title  -> cosmetic.title.season.3.deo_suong
reward_tier.season.3.free.frame  -> cosmetic.frame.season.3
reward_tier.season.3.free.shrine -> cosmetic.shrine.season.3
reward_tier.season.3.paid.title  -> cosmetic.title.season.3.paid
reward_tier.season.3.paid.frame  -> cosmetic.frame.season.3.paid
reward_tier.season.3.paid.emote  -> cosmetic.emote.season.3.paid
reward_tier.season.4.free.title  -> cosmetic.title.season.4.thanh_mua
reward_tier.season.4.free.frame  -> cosmetic.frame.season.4
reward_tier.season.4.free.shrine -> cosmetic.shrine.season.4
reward_tier.season.4.paid.title  -> cosmetic.title.season.4.paid
reward_tier.season.4.paid.frame  -> cosmetic.frame.season.4.paid
reward_tier.season.4.paid.emote  -> cosmetic.emote.season.4.paid
reward_tier.season.5.free.title  -> cosmetic.title.season.5.nui_mua
reward_tier.season.5.free.frame  -> cosmetic.frame.season.5
reward_tier.season.5.free.shrine -> cosmetic.shrine.season.5
reward_tier.season.5.paid.title  -> cosmetic.title.season.5.paid
reward_tier.season.5.paid.frame  -> cosmetic.frame.season.5.paid
reward_tier.season.5.paid.emote  -> cosmetic.emote.season.5.paid
```

Free-track progression is character-scoped (ADR-0029). Free-track cosmetics are earned through seasonal Atlas progress only (T1 all pages → title, T2 all pages → shrine, T3 all pages → frame), so guildless players can finish the free track. No separate seasonal currency exists. Paid extra tiers require the ACCOUNT_SCOPED_ACCESS entitlement and are claimed per character.

Cosmetic grants are idempotent. The idempotency key is a UUID v5 derived from the project content-grant namespace and the canonical grant-scope string (per `../06_data/ids.md`):
```text
grant_scope_string         = "season." + season_number + "." + cosmetic_id + "." + character_id
cosmetic_grant_operation_id = UUID v5(CONTENT_GRANT_NAMESPACE_UUID, grant_scope_string)
```
`CONTENT_GRANT_NAMESPACE_UUID` is the pinned constant defined in `../00_context/technology_versions.md`. The resulting UUID is stored in a PostgreSQL `uuid` column and satisfies all operation ID requirements. The grant-scope string is identical to the previous concatenated form; the derivation method changes to UUID v5 so the key fits a `uuid` column.

A retry when the entitlement already exists is a safe no-op.

## Anti-FOMO Invariants
The spec's non-decay discipline applies unconditionally to all seasonal content:
```text
no streak mechanic across seasons
no decay of progress at season boundary
no missed-cycle penalty: all seasonal Atlas pages and cosmetics re-enter when region repeats
seasonal cosmetics are cosmetic only (no combat stats, no economy efficiency, no reward-rate modifier)
no seasonal reward is permanently exclusive
no battle-pass power
no seasonal reward grants combat stats
```

Missing a season causes no permanent loss. There is no exclusive "first cycle only" cosmetic.

## Authority & Persistence
Server derives `season_number` and `featured_zone_id` at every relevant request from live `server_utc_seconds`. No season-state row is stored.

Seasonal Atlas progress uses the existing `character_atlas` table defined in `atlas.md`:
```text
character_atlas(character_id, atlas_page_id, tier, seen_count, completed_at, reward_operation_id)
```

Seasonal Guild Stone entries use the Guild Stone rules in `guild.md`.

Seasonal cosmetic entitlements are rows of `character_cosmetic_entitlements` (`cosmetics.md` § Persistence); a paid-track row records the granting `ACCOUNT_SCOPED_ACCESS` entitlement as its source.

## Invariants
```text
SEASON_EPOCH_UTC_SECONDS = 1799020800
season_duration = 8 weeks (co-incident with pvp.md ranked PvP season)
region_rotation_cycle = 6 zones, deterministic, derived from season_number mod 6
seasonal_atlas_pages_per_season = 10 (separate from 104 launch pages)
seasonal_cosmetic = cosmetic-only, no combat stats
missing a season = no permanent loss (content re-enters on rotation repeat)
no battle-pass power
no streak mechanic
no progress decay at season boundary
seasonal cosmetic grant idempotency key = UUID v5(CONTENT_GRANT_NAMESPACE_UUID, "season.<season_number>.<cosmetic_id>.<character_id>") stored in uuid column
server derives season_number without stored season-state row
```
