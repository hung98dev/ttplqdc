# Constraints
status: LOCKED

## Product
- 2D side-scrolling MMORPG.
- Platforms: PC and mobile.
- Real-time action combat.
- One logical persistent online world; normal maps may use channels and instanced activities where required.
- Service target: 10,000+ concurrent users.
- Vietnamese folklore is the primary visible theme and content source.
- One account owns at most 3 characters. Characters share no gameplay resources. The only account-shared pool is payment/IAP (`tiền nạp`, ADR-0029). One account has exactly one live gameplay session on one device at a time; two characters cannot be played concurrently (ADR-0030).

## Gameplay
- Server authoritative for gameplay-critical/persistent state.
- Client must not decide damage, movement legality, rewards, inventory ownership, currency, quest progression, or PvP results.
- Gameplay must support keyboard/mouse and mobile touch controls.
- Core combat must remain usable on mobile without excessive simultaneous inputs or unreadable effects.
- Normal story progression must remain solo-viable; multiplayer is beneficial, not a mandatory trinity requirement.

## Content
- Primary creatures, locations, visual motifs, supernatural themes, props, architecture and quest identity must be rooted in Vietnamese folklore/environment language.
- Ngũ Hành is a gameplay/build substrate, not permission to make generic elemental/xianxia regions.
- Non-Vietnamese mythology must not become a primary theme without explicit design approval.
- Folklore may be adapted for gameplay; regional variants must not be presented as one definitive belief.
- Real sacred sites, living practices and historical artifacts should be fictionalized when literal use is unnecessary.
- Historical reconstruction accuracy is not mandatory.

## Art
- Stylized 2D side-scrolling presentation.
- Chibi/cute proportions with clear silhouettes.
- Strong Vietnamese visual identity.
- Combat telegraphs must remain readable on small mobile screens and must not depend only on color.
- Reference games are inspiration only; do not copy identifiable assets, maps, UI, animations, audio or designs.

## Progression / Economy
- Character max level at launch: 60.
- No normal-play stamina/energy gate.
- No mandatory login streak for permanent power.
- No infinite launch paragon/power ladder for combat stats; seasonal horizontal prestige (cosmetic/Atlas/Guild Stone/fishing collections) is explicitly allowed (see `vision.md`). Seasons are a real launch system with an 8-week cadence; see `../03_systems/seasons.md`.
- Story baseline must not require +16 equipment, perfect rolls, auction purchase, specific Boss Soul, Guild Blessing, or mandatory party composition.
- Exactly three gameplay currencies under the economy spec unless a future explicit architecture/design change replaces that rule.
- Equipment durability/repair is not part of launch.
- Bonus skill/potential books from Level 25 onward are part of the 1-60 progression curve (`../01_gameplay/progression.md`), granting at most 12 skill books (+1 skill point each) and 12 potential books (+10 potential each) by Level 60 on top of level-up rewards. Level 55 and Level 60 each additionally grant +2 milestone skill points (not a skill unlock), bringing the total skill points at Level 60 to 59 (level-up) + 12 (books) + 4 (milestones) = **75 out of 114**.

## Engineering
- Shared gameplay rules between PC and mobile where possible; platform UI/input may differ.
- Persistent state must survive reconnects and server restarts.
- One world runs as one `thinhthan-server` process on one host with one PostgreSQL database (ADR-0052); capacity grows by performance work or a larger host, never by extra processes or worlds.
- Static content uses immutable stable IDs and versioned atomic activation under `../06_data/config.md`.
- Server-side operations affecting ownership/value must be idempotent where retries are possible.
- Exact launch engine/toolchain/database/core-library versions are owned by `technology_versions.md`; implementation must not float or auto-select versions.
- Preview/beta/RC/nightly technology is not a production default merely because it is newer; production uses the explicitly pinned stable/LTS version.

## AI / Spec Discipline
- `docs/` is source of truth.
- Do not invent unresolved rules.
- Mark genuinely missing decisions as `TODO` and stop dependent implementation where required.
- One concept has one canonical owner; other docs reference it.
- Behavior changes update the owning spec and affected launch catalogs together.
- Architecture/data-contract changes require an ADR.
- A generic fantasy placeholder cannot silently become shipping content.
