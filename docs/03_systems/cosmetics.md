# Cosmetics
status: LOCKED

## Scope
Defines ownership/equip semantics for non-power rewards such as titles, frames, banners, crests, shrine visuals, and appearance cosmetics.

## Principle
Cosmetics never modify combat stats, reward rates, matchmaking, drop rates, economy efficiency, or progression power.

## Identity / Ownership
Stable `cosmetic_id`. Launch gameplay cosmetics (story, feat, atlas, chivalry, bond, appearance from play) are **character-scoped** (ADR-0029):
```text
character_id + cosmetic_id
```
Grants are stored one row per grant source in `character_cosmetic_entitlements` (`../06_data/data_model.md`); the cosmetic is owned while any row exists, so revoking one source (e.g. a refunded season track) keeps ownership granted by another source. Guild crest/banner/shrine stay guild-scoped. Payment/IAP cosmetics are account-entitled and may be equipped on any character of that account without moving items.

Duplicate entitlement grants are idempotent and do not automatically create a tradable token/currency.

## Categories
```text
TITLE                 slot: title            scope: character/account-IAP
TITLE_GLOW            slot: title_glow
PROFILE_FRAME         slot: frame
NAMEPLATE             slot: nameplate
CHARACTER_APPEARANCE  slot: appearance
WEAPON_TRAIL          slot: weapon_trail
AURA                  slot: aura
EMOTE                 no slot; every owned emote is usable (emote wheel)
CHARACTER_SHRINE      slot: character_shrine  (personal shrine display at Safe Anchors)
GUILD_STONE_INSCRIPTION slot: guild_stone_inscription  (style of the character's own name on the Guild Stone, guild.md; character-scoped)
GUILD_SHRINE_VISUAL   guild-scoped; set by guild LEADER/VICE_LEADER
GUILD_BANNER          guild-scoped
GUILD_CREST_ACCENT    guild-scoped
```
Each slot holds one equipped cosmetic. `cosmetic.guild_stone.inscription.*` are `GUILD_STONE_INSCRIPTION`. Seasonal and currency shrines (`cosmetic.shrine.*`) are `CHARACTER_SHRINE`; guild shrine visuals are a separate guild-scoped category.

## Equip
Equipping/unequipping is presentation state. The first equip of an account IAP cosmetic by any character sets its `first_equipped_at` once (refund classification, `monetization.md`). One active item per mutually exclusive cosmetic slot unless a definition says otherwise.

Invalid/removed entitlement falls back to none/default. Cosmetic equip can never alter gameplay collision/hitboxes or hide required combat telegraphs.

## Folklore Feats and Glowing Titles
Prestigious cosmetic titles are unlocked through lifetime milestone achievements across combat, exploration, gathering, crafting, and competitive modes under ADR-0024:
- Titles display prominently above character names in 2D world presentation.
- High-milestone titles feature distinctive typography, particle effects, and color glows (e.g. golden dragon radiance for +16 enhancement, water ripples for fishing master, crimson fire for PvP champions).
- Purely cosmetic: titles grant zero stats, zero combat multipliers, and zero hidden perks.

## Reward Sources
PvE story/achievements, PvP, Guild, Guild War, world events, seasonal Atlas chapters, and seasonal Guild Stone categories may grant direct cosmetic entitlements.

Do not introduce mode-specific cosmetic currencies merely to deliver these rewards. A cosmetic priced in currency uses exactly one of: `currency.special` (special redemption list) or `currency.common` (common-priced cosmetics such as Guild Stone inscription styles, character shrines and title glows in `../07_content/economy_catalog.md`). Every price is authored explicitly; `currency.bound` and real money are never redemption currencies (store items use IAP, `monetization.md`).

## Seasonal Cosmetic Track
Each 8-week season exposes a cosmetic track. Free track = 3 cosmetics with no paid gate and no battle-pass power. Paid track (`product.service.season_track.<id>`) adds extra cosmetic tiers only. Seasonal free-track cosmetics are character-scoped (ADR-0029). The free track consists of:
- One seasonal title, granted for completing all 10 seasonal Atlas pages at Seen (T1) tier.
- One seasonal profile frame, granted for mastering all 10 seasonal Atlas pages (T3 tier).
- One seasonal shrine visual, granted for reaching T2 (Studied) on all 10 seasonal Atlas pages.

Season 0 free: `cosmetic.title.season.0.lang_da_ky_ghe`, `cosmetic.frame.season.0`, `cosmetic.shrine.season.0`.
Season 0 paid (account-scoped access; per-character claim): `cosmetic.title.season.0.paid.dem_lang`, `cosmetic.frame.season.0.paid`, `cosmetic.emote.season.0.paid.chap_tay`.

Seasonal cosmetic rules (binding):
- No seasonal cosmetic grants combat stats, reward rates, matchmaking influence, or economy efficiency.
- Missing a season causes no permanent loss: seasonal cosmetics re-enter on region-rotation repeat.
- Free-track IDs: `cosmetic.title.season.<n>.<key>`, `cosmetic.frame.season.<n>`, `cosmetic.shrine.season.<n>`.
- Grants are idempotent: `season.<n>.<cosmetic_id>.<character_id>`.
- The complete seasonal cosmetic schedule and concrete entitlement roster are owned by `seasons.md` and `../07_content/cosmetic_catalog.md`.

## Folklore Feats Tracking
Folklore Feats are lifetime milestone achievements (ADR-0024) that grant cosmetic titles. This section owns feat counter schema, persistence, and idempotency; concrete feat definitions are owned by `../07_content/cosmetic_catalog.md`.

### Feat Types

| Feat type | Counter model | Example condition |
|---|---|---|
| `KILL_COUNT` | Cumulative integer counter per `(character_id, feat_id)` | Slay 1,000 Ma Da |
| `BOSS_KILL_COUNT` | Cumulative integer counter per `(character_id, feat_id)` | Defeat Hổ Tinh 10 times |
| `GATHER_COUNT` | Cumulative integer counter per `(character_id, feat_id)` | Catch 200 fish |
| `ENHANCEMENT_FLAG` | Boolean flag set on first reaching the enhancement level | Enhance to +12 |
| `PVP_RANK_FLAG` | Boolean flag set on ranked season settlement | Rank 1 season champion |

### Persistence Schema
```text
character_feats(
  character_id       uuid,
  feat_id            text,       -- stable ID, e.g. feat.combat.slay_ma_da
  counter_value      integer,    -- current count for KILL/GATHER; 0/1 for flags
  PRIMARY KEY (character_id, feat_id)
)
character_feat_milestones(
  character_id        uuid,
  feat_id             text,
  milestone_threshold integer,   -- authored threshold; single-milestone feats use 1
  completed_at        timestamp,
  reward_operation_id uuid,      -- idempotency key for the cosmetic grant
  PRIMARY KEY (character_id, feat_id, milestone_threshold)
)
```
A feat with several milestones stores one milestone row per reached threshold.

### Idempotency
Feat milestone grants are idempotent per:
```text
character_id + feat_id + milestone_threshold
```
Retrying a grant when the milestone row already exists is a safe no-op that does not duplicate the cosmetic entitlement.

### Authority
Server owns all feat counters. The server increments counters on authoritative game events (`MONSTER_KILLED`, `FISH_CAUGHT`, `BOSS_DEFEATED`, `ENHANCEMENT_COMPLETED`, `PVP_SEASON_SETTLED`). Counter values are never accepted from the client.

### Rules
- Feat counters persist across season boundaries and are never reset.
- Reaching a feat milestone more than once (by re-running the same content) does not grant the cosmetic more than once; the idempotency key prevents duplication.
- Feat cosmetic titles grant zero stats, zero multipliers, and zero hidden perks.

## Currency.Common Cosmetic Sinks
Guild Stone inscriptions, shrine variants, and title glows are purchasable with `currency.common`. These are optional cosmetic personalizations with no power, no durability, and no rent cost.

Rules:
- Sinks use only `currency.common`; no fourth currency is introduced.
- All items are non-power: no stat, reward-rate, or economy-efficiency modifier.
- Purchases are character-scoped entitlements (ADR-0029), not tradable items.
- Purchase is a server-authoritative atomic operation: validate entitlement absent + validate balance + debit + grant.
- Idempotency key: `common_cosmetic.<cosmetic_id>.<character_id>`.
- Concrete IDs and `currency.common` amounts are owned by `../07_content/cosmetic_catalog.md`.

The recurring common sink does not create a new currency and is compliant with `../00_context/non_goals.md`.

## Material Redemption
An explicit cosmetic may define a one-time material redemption:
```text
cosmetic_id
required_item_id
required_quantity
```

Redemption is a server-authoritative atomic operation:
1. validate entitlement is not already owned,
2. validate item ownership/quantity and transaction locks,
3. consume configured material quantity,
4. grant entitlement,
5. commit once under stable `cosmetic_redemption_operation_id`.

Failure consumes nothing. Inventory overflow is irrelevant because output is an entitlement, not an item.

## Special-Currency Redemption
An explicit cosmetic may instead define:
```text
cosmetic_id
required_currency_id = currency.special
required_amount
```

Settlement is atomic:
```text
validate entitlement absent for character
+ validate character special balance
+ debit configured special amount
+ grant character entitlement
```

The operation is character-scoped and idempotent under ADR-0029. If the entitlement already exists on the character, the operation succeeds as already-owned/no-op and consumes no currency.

A cosmetic may explicitly expose **alternative** material and special-currency redemption routes. When it does:
- the client chooses one route before submission,
- exactly one route is validated/consumed,
- both routes grant the same entitlement ID,
- acquiring the entitlement by one route permanently makes the other route a no-op,
- there is no automatic exchange between material and currency.

## Redemption Guardrails
- redemption is cosmetic-only,
- no randomized cosmetic outcome,
- no real-money implication,
- no power/reward-rate/economy-efficiency effect,
- no separate mode-specific redemption currency,
- currency redemption uses only the explicit `currency.special` or `currency.common` price authored for that cosmetic,
- duplicate entitlement cannot repeatedly consume material/currency.

These narrow actions let optional event material and character-scoped `currency.special` rewards have a direct use without creating a crafting profession or fourth currency.

## Trading
Initial cosmetic entitlements are non-tradable/non-auctionable.

A material used for cosmetic redemption follows its own item binding rules; entitlement itself never becomes tradable because its input was tradable/bound.

## Guild-Scoped Cosmetics
Guild banner/crest/shrine definitions may be guild-scoped rather than character-scoped.

Guild-scoped unlock/equip requires canonical guild permission/state and disappears from personal equip availability after leaving that guild unless a separate character or IAP account entitlement was also granted.

Guild cosmetic state never modifies Guild War or Guild progression power.

## Persistence
Persist:
```text
character cosmetic entitlements (character_cosmetic_entitlements: one row per grant source; ADR-0029)
equipped character cosmetic IDs
account IAP cosmetic entitlement set (account_cosmetic_entitlements incl. first_equipped_at; payment spec in monetization.md)
guild-scoped cosmetic state where owned by guild
```
Server validates ownership; client never grants entitlement.

## Launch Counts
```text
TITLE_PLAY = 127
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
```
Play-earned cosmetics are character-scoped. IAP store cosmetics are account-entitled.

## Audit
Audit entitlement grants, material/currency redemption, administrative changes, and guild-scoped cosmetic changes with stable operation/source references.

## Invariants
```text
cosmetic -> no gameplay power
duplicate unlock -> no duplicate asset/currency
material redemption -> atomic + deterministic + cosmetic-only
special redemption -> atomic + deterministic + cosmetic-only
common redemption -> atomic + deterministic + cosmetic-only
alternative redemption consumes exactly one route
default gameplay cosmetic ownership = character scoped (ADR-0029)
IAP cosmetic entitlement = account scoped (account_cosmetic_entitlements)
mode-specific cosmetic token currencies = disabled
cosmetic visuals never alter authoritative hitboxes
feat counter -> server-authoritative, never reset across seasons
feat grant -> idempotent per character_id + feat_id + milestone_threshold
seasonal cosmetic -> cosmetic-only, no combat stats, no permanent loss from missing a season
currency.common cosmetic sinks -> non-power, no fourth currency
```