# Character
status: LOCKED

## Identity
- One account may own up to `3` characters.
- Each character has immutable `character_id`.
- Character rename is not supported initially.

## Resource Isolation (ADR-0029)
Characters on one account do not share gameplay resources: inventory, equipment, Souls, Linh Thú, quests, atlas, EXP/level, `currency.common`, `currency.bound`, `currency.special`, enhancement, or gameplay cosmetics.

The only account-shared pool is payment/IAP entitlements. Claiming IAP grants to the currently selected character (CHARACTER_BOUND item or character cosmetic). After claim, that grant cannot move to another character.

Same-account direct trade, Auction buy of own listing, and gameplay item vault transfer are rejected.

## Deletion
Character deletion is not supported. Once created, a character is permanent and cannot be deleted.
## Name
Canonical Unicode processing and uniqueness-key construction are owned by `../06_data/text.md`.

- `1..16` visible Unicode grapheme clusters and at most 64 UTF-8 bytes after canonical trim + NFC (`../06_data/text.md` § Name Limits).
- Names whose canonical key starts with `anonymized_` are reserved (erasure placeholders) and rejected.
- Vietnamese diacritics are allowed and significant.
- Global uniqueness uses the server-computed canonical `name_key`, not database locale/lowercasing.
- Control characters and empty names are rejected.
- A concurrent duplicate-name race is resolved by the PostgreSQL UNIQUE `name_key` constraint; client availability checks are advisory only.

## Creation
Character creation selects:
- `character_name` (globally unique canonical name)
- `class_id` (`class.kim`, `class.moc`, `class.thuy`, `class.hoa`, `class.tho`)

Appearance upon creation is fixed according to class:
- Player customization of hair, skin, face, or body preset is disabled during character creation.
- Each class has one canonical authored visual silhouette and default appearance.
- Visual changes after creation occur solely through non-power cosmetic entitlements (`../03_systems/cosmetics.md`).
Creation initializes:
- `level = 1`
- `current_exp = 0`
- class from `classes.md`
- base/potential stats from `stats.md`
- starter skills from `skills.md`
- empty equipment loadout except explicit starter equipment content
- starter checkpoint `checkpoint.lang_da.dinh_lang`
- starter map `map.lang_da.dinh_lang`
- no party or guild membership

Class is permanent after creation.

## Persistent State
Persist at minimum:
- identity/name/class/appearance
- lifecycle timestamps (creation timestamp; character deletion is a non-goal per `non_goals.md`, so no deletion timestamp exists)
- level and EXP
- potential allocation
- skills and skill levels
- equipment/loadouts
- inventory
- currencies
- quest/progression flags including `progression.first_session.just_guard_hint`
- active world checkpoint
- social/guild state references
- build-system progression

## Runtime State
Runtime state includes:
- current HP/MP
- map instance and position
- movement/combat state
- active statuses
- cooldowns
- current target
- party runtime state

## Lifecycle
Canonical lifecycle:

```text
CREATE -> ACTIVE -> OFFLINE -> ACTIVE
```

Runtime states may include:

```text
DEAD
DISCONNECTED
TRANSFERRING_MAP
```

## Login
1. authenticate account (one live gameplay session per account, ADR-0030)
2. if another connection of this account is live, it is replaced (`SESSION_REPLACED`)
3. select owned ACTIVE character (exactly one attached at a time)
4. load persistent state
5. validate/repair invariant-safe state
6. resolve valid map/checkpoint
7. restore runtime state allowed by reconnect rules
8. enter world

A second client of the same account cannot attach a different character while the first remains in world. Switching character requires detach / character select first; the previous character is `OFFLINE` before the next attach.

## Logout and Disconnect
- Normal logout persists required state before world removal.
- Disconnect never grants healing, movement, rewards, or state rollback.
- Persistent writes must be idempotent and duplication-safe.

## Authority
Client may request actions but cannot authoritatively modify character identity, progression, inventory, currency, position, skills, or equipment.

## Invariants
```text
characters_per_account <= 3
character_name is globally unique (case-folded name_key)
character is permanent (no deletion)
class is permanent
creation appearance is fixed by class (no customizer)
one account -> one live gameplay session (ADR-0030)
at most one attached character per account
```
