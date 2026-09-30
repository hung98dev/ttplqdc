# Atlas — Hon Giam & Quai Dam
status: LOCKED

## Scope
Defines the folklore collection atlas (Hon Giam — Soul Mirror, Quai Dam — Monster Tales) for Vietnamese folklore creatures, Souls, and world mysteries. Atlas is a **non-power collection system** owned by `atlas.md`.

## Design Goals
- Turn Vietnamese folklore into collectible identity, not just monster reskins.
- Provide completion-driven retention (100% compulsion) without combat power creep.
- Every atlas page tells a researched folklore snippet (not a definitive religious claim) and rewards curiosity, not grind.

## Atlas Pages
Stable IDs: `atlas.page.<category>.<key>` (e.g. `atlas.page.quai_dam.ma_da`, `atlas.page.hon_giam.ho_tinh`).

Categories:

| Category | Source | Unlock trigger | Example |
|---|---|---|---|
| `quai_dam` | Monsters (NORMAL/ELITE) | First kill + 10 + 100 kills (night-only pages 5 + 30; `../07_content/atlas_catalog.md`) | Ma Da, Ma Tranh, Ho Tinh |
| `hon_giam` | Souls (`soul_contracts.md`) | First acquisition of that `soul_id` | Hon Ma Xó, Hon Thuong Luong |
| `di_tich` | Boss aftermath relics | Witness boss aftermath relic (`bosses.md`) within 60m | Di Tich Thuong Luong |
| `co_vat` | Hidden chests, fishing, cooking | Open hidden chest, catch rare fish, cook first delicacy | Ruong Co Bich An, Ca Chep Hoa Rong |

Total **launch** pages: **104** (58 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat). The exact roster is owned by `../07_content/atlas_catalog.md`.

**Seasonal pages** are additional and counted separately from the 104 launch pages. Each season contributes 10 pages (6 quai_dam + 2 co_vat + 2 di_tich) keyed to the featured region. Seasonal page IDs follow the namespace `atlas.page.season.<season_region_index>.<page_key>` (`season_number mod 6`; repeat cycles reuse the IDs). Seasonal roster, cadence, and rules are owned by `seasons.md` and `../07_content/atlas_catalog.md`.

## Unlock Rules
- Server-authoritative: unlock on authoritative `MONSTER_KILLED`, `SOUL_ACQUIRED`, `BOSS_DEFEATED` (witness relic), `CHEST_OPENED`, `FISH_CAUGHT`, `DISH_COOKED` events. First **Seen** (tier 1) commit per page is one `PHAT_HIEN` (`source=ATLAS_SEEN`) unless that same settlement already emitted `source=FISH_RARE` or `source=CHEST_HIDDEN`. `CHEST_SPOTTED` and `QUEST_CLUE` never unlock Atlas pages.
- Each page has 1-3 tiers (Seen / Studied / Mastered) with counters persisted per character.
- Unlock is character-scoped, not account-scoped, but account-wide view aggregates all characters for display.
- No Atlas page grants combat stats, damage multipliers, or enhancement bonuses.

## Rewards (Non-Power)
Each page tier grants at most one immutable `atlas_reward_bundle`, concretely authored by `atlas_catalog.md`. A launch-page bundle may contain one `currency.special` credit (1 or 2 per `../07_content/atlas_catalog.md`; seasonal pages grant 0; cosmetic-only, **character-scoped**) and one non-economic presentation entitlement (cosmetic title, lore illustration, or card frame). Atlas completion milestones may similarly bundle one glowing title plus one portrait frame, never gear or points.

Tier progress and its entitlement key are character-scoped: `atlas.tier.<character_id>.<atlas_page_id>.<tier>` (the single idempotency key for promotion, EXP, `currency.special` and entitlement of that tier; stored as `reward_operation_id`). A `currency.special` result credits that `character_id`. Launch maximum Atlas total (`520`) is below the `currency.special` character cap (`1,000,000`). (Recomputed after roster expansion to 104 pages with T3 = 2 special to remain within the ~545 sink surface: 104×(1+2+2) = 520.) Cosmetic title/frame from Atlas is character-scoped (ADR-0029). Neither reward may be rerolled on retry.

## Claim
Every tier reward auto-settles in the same transaction as the tier promotion (no unclaimed Atlas reward state exists; a player who never opens the journal loses nothing). `C2S_ATLAS_CLAIM` (504) with `operation_id`, `atlas_page_id`, `tier` (`../05_network/messages.md`) only acknowledges a settled tier for the journal UI: `S2C_ATLAS_CLAIM_RESULT` (505) returns the existing grant and sets `acknowledged_at` once; a tier not yet reached is rejected with `ATLAS_TIER_NOT_REACHED` and changes nothing. It never creates a grant. Do not reuse 408.

Each atlas tier-up is 1 LIFE_SKILL action granting character EXP for the current act (`../07_content/progression_route.md`): I 6417, II 8283, III 6332, IV 7047, V 9448, VI 10494. The EXP grant is atomic with the tier promotion.

## Folklore Presentation
- Each page shows Vietnamese name, folk region, 2-3 sentence adapted tale, and stylized chibi/footprint art. Text is `vi-VN`/`en-US` via `client_localization.md`; display never affects identity.
- Must not claim one regional version as definitive historical truth; use "Theo loi ke dan gian..." framing.

## Discovery vs Access
Unlocking an Atlas page does not unlock combat content, maps, or dungeons. It is a parallel collection journal.

## Persistence
Persist per character:
```
character_atlas(character_id, atlas_page_id, tier, seen_count, completed_at, reward_operation_id, acknowledged_at NULL)
atlas_milestones(character_id, milestone_id, completed_at)
```
Plus idempotent reward operation IDs for each tier grant.

## Authority
Server owns all unlock counters, tier promotion, and reward grants. Client only renders the journal UI and sends inspect requests.

## UI
Atlas Journal in Safe Anchors and menu: grid of pages, completion %, lore viewer, "new reward" marker cleared by `C2S_ATLAS_CLAIM` acknowledgement. No trading of pages.

## Invariants
```text
launch atlas pages = 104 (58 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat)
seasonal atlas pages = 10 per season, counted separately from 104 launch pages
atlas is non-power (no stats, no damage, no enhancement)
atlas progress is character-scoped
one page tier + character -> at most one reward grant
atlas completion never gates MAIN progression
server-authoritative unlock only
Seen tier-1 peak = PHAT_HIEN unless same settlement already emitted FISH_RARE or CHEST_HIDDEN
CHEST_SPOTTED and QUEST_CLUE never unlock Atlas pages
missing a season causes no permanent atlas loss (seasonal pages re-enter on region rotation repeat)
```
