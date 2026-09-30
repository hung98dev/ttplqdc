# ADR-0015: Unity Localization Presentation Layer
status: ACCEPTED

## Context
Gameplay/content specifications already require stable machine IDs and treat Vietnamese display text as localization data. NPC dialogue also references localization keys. Without one client localization system, AI agents could hard-code Vietnamese strings into MonoBehaviours/ScriptableObjects, invent custom dictionaries, or choose different packages.

The launch world identity is Vietnamese folklore, so Vietnamese text quality and diacritics must be first-class. Localization remains presentation only and must never redefine authoritative IDs/gameplay values.

## Decision
- Unity Localization is the canonical client localization system.
- Exact package version is `com.unity.localization 1.5.12`, pinned in `../00_context/technology_versions.md`.
- Required launch locales are `vi-VN` (primary/default) and `en-US` (English).
- Both `vi-VN` and `en-US` are required for release-scope content to support Vietnamese players and international audiences.
- String-table keys are stable ASCII lowercase localization IDs. UI/gameplay code references keys, not hard-coded shipping prose, for release-scope user-facing content.
- Static gameplay IDs and localization keys are distinct namespaces.
- Server error/domain messages use stable codes/keys plus structured arguments. Unity selects localized display text; server free-text is not gameplay identity.
- Runtime player-authored text (character/guild names/chat) is **not** translated and follows the authoritative Unicode rules in `../06_data/text.md`.
- Localized assets, when used, follow Addressables/asset authority rules from ADR-0014.
- Smart Strings may format presentation arguments/plurals but cannot compute authoritative gameplay values.
- Missing required `vi-VN` or `en-US` key is a release validation failure for release-scope content. Development may show a conspicuous key placeholder; production must not silently fall back to a gameplay ID.
- Locale selection is a client presentation preference and never changes matchmaking, world/economy, server authority, or persisted gameplay state.

## Consequences
- Vietnamese source strings are centralized and testable.
- Future language support does not require renaming gameplay IDs.
- Client code is less likely to accumulate hard-coded shipping text.
- Localization tables become part of presentation asset/version validation and must be available before content requiring them is released.

## Invariants
```text
Unity Localization = 1.5.12
required launch locales = vi-VN (default), en-US
gameplay ID != localization key
player-authored text is never machine-translated
Smart Strings != gameplay calculation
missing required vi-VN or en-US key = release failure
server sends stable code/key + args, not authoritative localized prose
```
