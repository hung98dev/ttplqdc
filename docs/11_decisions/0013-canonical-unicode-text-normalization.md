# ADR-0013: Canonical Unicode Text Normalization
status: ACCEPTED

## Context
Character names, guild names, chat, and other player-authored text allow Vietnamese diacritics and use Unicode grapheme-cluster limits. Existing specs say "Unicode NFC" and "case-insensitive" but do not define one exact implementation for case folding, uniqueness keys, or grapheme counting.

Without a canonical algorithm, Unity, Go, and PostgreSQL could disagree about whether two visually/equivalently encoded names are the same, how long a string is, or which characters are allowed. Database collation-dependent lowercasing would also make behavior environment-dependent.

## Decision
- Authoritative player-text validation runs in Go.
- Unicode normalization/case folding uses pinned `golang.org/x/text v0.42.0`.
- Grapheme segmentation/counting uses pinned `github.com/clipperhouse/uax29/v2 v2.7.0`, which implements UAX #29 grapheme boundaries for Unicode 17.
- Canonical name display value:
  1. require valid UTF-8,
  2. trim leading/trailing Unicode whitespace with Go `strings.TrimSpace`,
  3. normalize to NFC,
  4. apply owning length/content rules.
- Canonical case-insensitive uniqueness key:
  1. start from the canonical display value,
  2. apply language-neutral Unicode default case folding with `cases.Fold()`,
  3. normalize the result to NFC again.
- PostgreSQL stores both display value and server-computed `name_key`. UNIQUE constraints/indexes compare the stored key; PostgreSQL locale/collation-specific `lower()` is not the canonical equality algorithm.
- Vietnamese accents/diacritics remain significant. Case folding does not strip accents, transliterate, or apply compatibility normalization.
- Do not use NFKC/NFKD for canonical player-name identity at launch.
- Grapheme limits are measured after trim + NFC normalization using the pinned UAX #29 implementation.
- Unity may mirror validation for UX, but the Go result is authoritative.

## Consequences
- Canonically equivalent NFC/NFD spellings resolve to the same stored representation/key.
- Upper/lower-case variants conflict where the owning namespace is case-insensitive.
- Distinct Vietnamese diacritics remain distinct.
- Name uniqueness is stable across PostgreSQL host locales.
- Two small Unicode dependencies are explicitly approved and pinned rather than letting each AI agent choose a different text library.

## Invariants
```text
authoritative normalization = Go
normalization = NFC
uniqueness = Unicode default case fold + NFC
compatibility normalization = disabled
accent stripping/transliteration = disabled
grapheme segmentation = UAX #29 Unicode 17
DB lower()/locale collation != canonical name key
Unity validation = UX mirror only
```
