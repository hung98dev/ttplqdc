# Client Localization
status: LOCKED

## Scope
Defines Unity localization package usage, key format, required launch locale, content integration, formatting, fallback, and authority boundaries.

Decision: `../11_decisions/0015-unity-localization.md`.
Exact package: `../00_context/technology_versions.md`.

# Canonical System
```text
com.unity.localization = 1.5.12
required launch locales = vi-VN (default), en-US
```

The game ships with full bilingual localization: Vietnamese (`vi-VN`) and English (`en-US`). Additional locales are optional future presentation content and do not change gameplay identity.

# What Is Localized
Release-scope user-facing authored presentation should use localization keys:
- class/skill/item/equipment/set/Soul/Formation names and descriptions,
- map/zone/dungeon/boss/monster display names,
- NPC names/dialogue,
- quest titles/descriptions/objective presentation,
- UI labels/tooltips/errors,
- PvP/guild display labels,
- cosmetic names/descriptions,
- system notices intended for players.

Player-authored names/chat are never localized.

# Localization Keys
Canonical format:
```text
loc.<domain>.<stable_segments>
```

Examples:
```text
loc.class.kim.name
loc.skill.kim.kiem_quang.name
loc.map.lang_da.dinh_lang.name
loc.item.material.lang_da.tre_gia.name
loc.quest.main.a1.01.title
loc.error.inventory.full
```

Rules:
- ASCII lowercase,
- deterministic/stable,
- no Vietnamese diacritics in the key,
- display prose is table value, not key,
- renaming Vietnamese prose does not rename the key,
- duplicate key fails presentation-content validation.

Localization key does not have to be byte-identical to a gameplay ID, but should include the relevant stable ID segments where useful.

# Required Locales
Both `vi-VN` and `en-US` are required for launch release-scope content:
- `vi-VN`: Default and primary language preserving Vietnamese folklore authenticity, mythology motifs, and diacritics.
- `en-US`: Full English localization covering all user-facing presentation for international accessibility.

A required key is valid only when:
- the key exists in the canonical table,
- both `vi-VN` and `en-US` have non-empty values,
- Smart String arguments compile when used in both locales,
- referenced localized asset exists when the entry is asset-localized.

Development builds may visibly display:
```text
[MISSING:loc.foo.bar]
```
to expose defects.

Production release validation rejects missing required strings in either `vi-VN` or `en-US` instead of silently showing stable gameplay IDs to players.
# Server / Client Boundary
Server sends:
- stable gameplay/error/event IDs,
- localization key where protocol/domain contract exposes one,
- typed formatting arguments,
- authoritative numeric values/results separately.

Client chooses localized presentation text.

Forbidden:
- server trusts localized text returned by Unity,
- client parses localized prose to infer gameplay state,
- localized string decides price/reward/damage/access,
- server free-text is used as a durable client decision code.

# Formatting Arguments
Smart Strings may format:
- names,
- counts,
- percentages,
- durations,
- server-supplied values,
- plural/select presentation.

Arguments are typed and ordered/named by the localization contract.

The localized expression never recomputes an authoritative formula. Example: server sends the final cost value; localization formats it.

# Vietnamese Text
Authoring:
- UTF-8,
- proper Vietnamese diacritics,
- no ASCII-diacritic stripping for convenience,
- terminology follows Vietnamese folklore/content guardrails,
- Hán-Việt vocabulary may be used naturally but must not drift into generic xianxia naming.

Player-authored Unicode normalization remains owned by `../06_data/text.md`; localization tables are authored presentation data, not identity keys.

# Assets
Localized assets use Unity Localization + Addressables under `client_assets.md`.

Rules:
- do not duplicate large assets per locale unless presentation genuinely differs,
- asset localization cannot change server gameplay collision/spawn/stats,
- missing required localized asset follows the same release-failure rule,
- remote localization asset/catalog publishing obeys Addressables publish order.

# Locale Selection
Initial client locale:
1. use saved supported presentation preference (`vi-VN` or `en-US`) when valid,
2. if no saved preference, map device OS locale: any `en-*` (`en-US`, `en-GB`, `en-AU`, …) -> `en-US`; otherwise `vi-VN`.

No OS-locale auto-selection may switch to an unsupported locale.

Changing locale:
- is local presentation preference,
- may reload string/asset tables asynchronously,
- never changes account/character/world state,
- never requires reconnect unless a future platform constraint explicitly says so.

# Persistence
Locale preference is **local client presentation preference only** at launch. It is not account-synced and is not gameplay-critical state.

Failure to persist the local preference does not affect gameplay authority.


# Fonts / Glyph Coverage
The shipping font/fallback chain must cover all required Vietnamese characters/diacritics and punctuation used by `vi-VN`.

Release validation includes:
- glyph coverage scan over all required strings,
- no missing-glyph tofu for canonical launch content,
- UI layout checks on target mobile resolutions.

Do not ship a font fallback that materially changes Vietnamese legibility.

# Content Pipeline
Localization export/import may use Unity-supported formats/tools, but generated/imported tables are reviewed and committed/versioned like presentation content.

Do not permit spreadsheet row order to become identity. Keys remain the join identity.

# Tests
Required:
- every release-scope key has non-empty `vi-VN` and `en-US`,
- duplicate/malformed key rejection,
- Smart String argument schema validation for both locales,
- Vietnamese and English glyph coverage,
- representative narrow mobile UI overflow checks in both languages,
- locale switch while menus/content loaded,
- missing key visibly fails in development and blocks release validation,
- client cannot alter gameplay result by editing local localization table,
- player-authored names/chat remain unlocalized.

# Invariants
```text
package = com.unity.localization 1.5.12
required launch locales = vi-VN (default), en-US
localization key = ASCII stable presentation identity
gameplay ID/result never derived from localized text
player-authored text is not translated
missing required vi-VN or en-US = release failure
Vietnamese glyph coverage = required
```
