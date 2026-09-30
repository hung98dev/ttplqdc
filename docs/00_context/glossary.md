# Glossary
status: LOCKED

## Scope
Canonical project terminology. Domain specs own exact mechanics; this file only fixes shared meaning so AI agents do not invent synonyms or reinterpret common words.

## Identity / Player
**Account** — Persistent player identity root. Owns up to three characters. Shared value is only payment/IAP (`tiền nạp`) under ADR-0029. It does not own gameplay inventories or currencies. Exactly one live gameplay session at a time (ADR-0030).

**Character** — One playable persistent avatar owned by one account. Name is globally unique across the logical world. Class is permanent at launch. Gameplay resources are character-isolated; they never move to another character on the same account. At most one character per account is attached to the live session.

**Nạp / IAP entitlement** — Account-scoped real-money purchase record. Claiming it grants to the selected character only. Not a gameplay currency.

**Stable content ID** — Immutable ASCII lowercase identifier for authored definitions such as skills, maps, items, bosses, quests and currencies.

**Durable entity ID** — UUID v4 identity for persistent entities such as account, character, guild, item instance, listing, claim and operation.

**Display name** — Localized/user-facing text. Never authoritative identity.

## Authority / Runtime
**Authoritative** — Final server-owned truth. Unity may request/predict/present but does not decide the result.

**Intent** — Client request describing what the player wants to do, not proof that the action/result is valid.

**Prediction** — Client-side temporary simulation/presentation used for responsiveness and always correctable by server state.

**Simulation owner** — The single Go runtime owner allowed to mutate one live map/channel/instance partition.

**Session epoch** — Monotonic generation identifying the current controlling authenticated **account** gameplay session (ADR-0030). Older epochs are stale. A login on another device replaces this epoch.

**Ownership epoch** — Monotonic generation identifying the current simulation owner during transfer/recovery.

**Operation ID** — Stable UUID reused across retry for one logical value-changing operation.

**Idempotent** — Repeating the same logical operation produces at most one committed mutation and returns/reconstructs the same outcome.

## World
**Logical world** — The one persistent player world/economy/social universe. Channels do not create separate progression worlds.

**Zone / Region** — Authored progression grouping containing an anchor and field maps.

**Map** — Stable authored world-space definition.

**Map instance** — Runtime copy/ownership scope of a map.

**Channel / Khu vực** — Capacity partition for a normal-world map. Players in different channels of the same map remain in the same logical world. Exact channel count, per-channel capacity, and load thresholds are canonical in `../02_world/world_rules.md` (ADR-0020, ADR-0035).

**Safe/social anchor** — Non-adventure hub map for NPC/social/checkpoint activity.

**Field map** — Adventure/exploration map with hostile spawning. The launch budget has 18 field maps, excluding six safe anchors.

**Instance** — Isolated runtime activity such as dungeon, finale, PvP or Guild War.

**Checkpoint** — Persisted normal-world respawn/recovery anchor selected under map rules.

**Entry spawn** — Server-owned legal position used when entering/recovering into a map.

**Public boss generation** — One logical public-boss occurrence whose reward identity remains coherent across eligible channels.

## Combat / Build
**Basic attack** — Class baseline attack; one of four upgradeable basic skills per class and not an ACTIVE skill.

**Active skill** — One of five learned class skills that may occupy the five active hotbar slots when unlocked.

**Passive skill** — Learned class skill that remains active without occupying an active slot.

**Skill tag** — Canonical semantic selector such as DAMAGING, AREA, MOVEMENT or PROJECTILE; separate from execution/targeting mode.

**Status** — Canonical combat condition owned by status rules, for example BURN, SLOW, FREEZE, STUN or ROOT. Skill-private markers are not automatically global statuses.

**Loadout** — One 14-slot equipment configuration. Exactly one is ACTIVE and two may serve as SUPPORT.

**Support Signature** — At most one derived contribution from each SUPPORT loadout. It is not a second full set of stats/effects.

**Soul Contract** — Collectible Soul instance bound into an equipment/build slot under Soul rules.

**Spirit Meridian** — Resonance system over the eight BASIC equipment slots and their element relations.

**Formation** — Pattern/resonance system over the six ADVANCED equipment slots.

## Items / Economy
**Item definition** — Static content describing an item type.

**Item instance** — Persistent owned copy/stack with its own UUID and generated/binding/enhancement state.

**Binding** — Transfer restriction on an item instance: UNBOUND, CHARACTER_BOUND or ACCOUNT_BOUND under item rules. Binding may become stricter but never looser.

**Inventory** — Character-scoped slot container.

**Account Storage** — Account-level IAP entitlement panel under ADR-0029/0030. Not an item vault or bank; characters share no gameplay items.

**Escrow** — Server-owned temporary custody used by trade/Auction/state settlement so one asset cannot exist in two owners simultaneously.

**Reward Claim** — Persistent recovery/overflow record for a reward already earned but not deliverable. It is not player mail or spendable currency.

**Common currency** — Character-scoped, player-transferable circulating currency.

**Bound currency** — Character-scoped, non-transferable optional utility currency.

**Special currency** — Character-scoped, non-transferable cosmetic reward currency earned through gameplay. Not shared across same-account alts (ADR-0029).

**Faucet** — Authored source that creates currency/items/materials.

**Sink** — Authored operation that permanently consumes currency/items/materials.

**Bonus Books / Sách Tiềm Năng / Sách Kỹ Năng** — Two CHARACTER_BOUND consumable items (`item.book.potential` and `item.book.skill`) earned through level-milestone rewards from Level 25 to Level 60. Each `item.book.potential` grants +10 unspent potential points; each `item.book.skill` grants +1 unspent skill point. Exact schedule, idempotency flags, and binding rules are canonical in `../03_systems/items.md` and `../01_gameplay/progression.md` (ADR-0025).

**Linh Đan** — A consumable material item used as Linh Thú (Spirit Beast) food and rewarded from hidden field chests and certain content. Exact feeding rules and effect are canonical in `../03_systems/spirit_beasts.md`.

**Lucky Charm (Bùa May Mắn) / Insurance (Bùa Giữ Bậc)** — Enhancement support consumables. Lucky Charm improves the success probability of an enhancement attempt; Insurance preserves the current enhancement level when an attempt fails. Exact tier effects, caps, and interaction with other bonuses are canonical in `../03_systems/items.md` (ADR-0022).

**Guild Stone / Bia Đá Danh Vọng** — A non-power prestige board at each Safe Anchor (owner: `../03_systems/guild.md` § Guild Stone); only character inscription styles are purchasable with common currency. Inscriptions are visible to all players in the Safe Anchor, constituting a `KHOE` peak. Categories expand seasonally. Exact purchase rules, inscription types, and seasonal categories are canonical in `../03_systems/guild_progression.md` and `../07_content/economy_catalog.md` (ADR-0028).

**Morning Market / Chợ Phiên Sáng** — A rotating daily limited-stock trade event at Safe Anchors. Exact selection rule, reward table ID, reset boundary, and special offers are canonical in `../07_content/economy_catalog.md` (ADR-0028).

**Guild Blessing** — The single active weekly bonus granted to all current guild members following a completed Five-Element Ritual and vote. Effects are PvE utility only unless explicitly declared `context = PVP`. Exact blessing catalog and vote rules are canonical in `../03_systems/guild_progression.md`.

## Data / Content
**Static content** — Versioned authored gameplay definitions compiled/validated before activation.

**Runtime tuning** — Explicitly hot-reload-safe values that do not reinterpret persistent/in-flight state.

**Schema version** — Version of a data/wire/content structure.

**Content revision** — Immutable validated set of static content activated atomically.

**Finite expansion** — Build-time deterministic expansion of a bounded authored pattern into concrete stable IDs; runtime does not invent arbitrary content IDs.

**Migration** — Explicit versioned database/data transformation. Not the same as content activation.

**Tombstone / terminal state** — Durable lifecycle record retained so history/references remain valid after logical deletion.

## Reliability / Operations
**Baseline** — Full authoritative replicated state from which later deltas are interpreted.

**Delta** — Bounded authoritative change relative to an acknowledged baseline.

**AOI (Area of Interest)** — Server-owned relevance filter determining which nearby entities/state a client receives.

**Backpressure** — Rejecting/coalescing/bounding work when a queue/dependency is saturated instead of allowing unbounded memory/goroutine growth.

**RPO** — Maximum targeted durable-data loss window after disaster recovery.

## Social / Progression
**chivalry_points / Điểm Hiệp Nghĩa** — A non-spendable, non-currency lifetime progression score tracking a veteran character's mentorship of newcomers in lower-tier dungeons. Accumulates via qualifying dungeon completions; milestones grant cosmetic titles. Exact eligibility, daily cap, and milestone rewards are canonical in `../03_systems/social.md` (ADR-0023).

**bond_points / Khế Ước Tâm Giao** — A relationship metric between a character and their active Linh Thú, earned through shared combat. Bond level unlocks deeper companion abilities. Exact growth rate, cap, and payoff are canonical in `../03_systems/spirit_beasts.md`.

## Setting
**Vietnamese folklore fantasy** — Primary launch world/content identity. Material culture, creatures, environment and supernatural framing must read as Vietnamese before generic fantasy system tags are applied.

**Ngũ Hành** — Five-element gameplay/build substrate (KIM, MỘC, THỦY, HỎA, THỔ). It is not a requirement that every region, boss or story become generic elemental/xianxia fantasy.

**Just Guard** — A player-agency damage-mitigation window triggered by a correctly timed horizontal movement edge during an incoming hit. Not an iframe; does not cleanse status. Exact window duration, mitigation value, streak mechanics, and ICD are canonical in `../01_gameplay/combat.md` (ADR-0026, amended by ADR-0034).

**MA_AM (Âm Khí / Whispered Haunting)** — A folklore-exclusive debuff stack that reduces the target's damage output and amplifies DoT damage taken. Stackable; applies only from ELITE, boss, or folklore encounters. Exact stack cap, duration, per-stack penalty, and 3-stack payoff trigger (next BURN or POISON tick) are canonical in `../01_gameplay/status_effects.md` (ADR-0026; HARD_CONTROL and other DOT types do not trigger).

**Spirit Surge** — A periodic world event that floods a region with heightened-intensity spirit entities and provides a `WORLD_EVENT` EXP channel for participating characters. Cadence is canonical in `../02_world/world_rules.md`; region/field schedule, contribution and EXP rates in `../07_content/world_event_catalog.md`.

**BONFIRE_REST** — The non-combat resting state set when a character rests within range of an active village bonfire at a Safe Anchor. Exact range, benefit values, and duration are canonical in `../02_world/world_rules.md` (ADR-0023).

**Linh Thú** — Character-scoped companion. One active. Stats transfer. No same-account move.

**Atlas / Phát Hiện** — Non-power collection journal; first Seen may emit peak `ATLAS_SEEN`.

**PHAT_HIEN (Phát Hiện / Discovery)** — One of the three named peak-moment types. Triggered by first discovery events such as a visible hidden chest, a rare fishing catch, an Atlas first-Seen, or the first-session MAIN quest clue. Qualifying sources and first-15 SLO rules are canonical in `../00_context/vision.md` and `../07_content/progression_route.md`.

**CUU_NGUY (Cứu Nguy / Clutch Save)** — One of the three named peak-moment types. Triggered by a Just Guard window on an incoming hit (even if missed) or a Linh Thú Passive 2 success. Slow-motion juice is success-only and does not alter committed combat math. Canonical in `../01_gameplay/combat.md`.

**KHOE (Khoe / Show-off)** — One of the three named peak-moment types. Triggered by a visible prestige display to other players at a Safe Anchor: a weapon glow at enhancement +12 or higher, a title glow, or a Guild Stone inscription. Canonical in `../00_context/vision.md`.

**DROP_THROUGH / HEIGHT_BAND** — Field mystery types: drop through one-way platform / double-jump perch.

**Di Tích** — A boss-aftermath relic buff persisted at the owning region's safe anchor and active in the channel where the boss was defeated. Exact duration, persistence rules, and visual marker requirements are canonical in `../02_world/bosses.md` (persistence: `../06_data/data_model.md`).
