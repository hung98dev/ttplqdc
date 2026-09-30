# Text Normalization
status: LOCKED

## Scope
Defines authoritative Unicode validation, normalization, grapheme counting, and case-insensitive uniqueness keys for player-authored text.

Decision: ../11_decisions/0013-canonical-unicode-text-normalization.md.
Exact Go dependencies: ../00_context/technology_versions.md.

## Canonical Libraries
~~~
golang.org/x/text = v0.42.0
github.com/clipperhouse/uax29/v2 = v2.7.0
~~~

Use:
- golang.org/x/text/unicode/norm for NFC,
- golang.org/x/text/cases for default Unicode case folding,
- github.com/clipperhouse/uax29/v2/graphemes for extended grapheme segmentation/counting.

Do not substitute rune count, byte count, database lower(), Unity char count, or a different Unicode package.

# Canonical Display Value
For character/guild names and any namespace using canonical-name equality:

1. reject invalid UTF-8,
2. trim leading/trailing Unicode whitespace with Go strings.TrimSpace,
3. normalize with norm.NFC,
4. reject empty result,
5. count grapheme clusters with the pinned UAX #29 grapheme iterator,
6. apply owning minimum/maximum/content rules,
7. persist this normalized value as display_name/name.

Internal whitespace is preserved unless an owning spec explicitly says otherwise.

# Canonical Name Key
For case-insensitive uniqueness:
~~~
name_key = NFC(cases.Fold(display_value))
~~~

Properties:
- default language-neutral Unicode case folding,
- no accent removal,
- no transliteration,
- no NFKC/NFKD compatibility folding,
- Vietnamese đ/Đ folds by case but remains distinct from d/D,
- canonically equivalent NFC/NFD input resolves consistently.

PostgreSQL stores name_key explicitly and places the UNIQUE constraint/index on that key.

Do not rely on:
~~~
LOWER(name)
ILIKE
database/server locale
OS locale
ICU collation choice
Unity-only normalization
~~~
for authoritative equality.

# Grapheme Count
Player-facing "visible Unicode grapheme clusters" means extended grapheme clusters from UAX #29 as implemented by the pinned Unicode-17 library.

Limits are evaluated after trim + NFC.

Examples conceptually:
- a precomposed Vietnamese vowel and its canonically equivalent decomposed sequence count the same after NFC,
- an emoji ZWJ sequence counts according to one UAX #29 grapheme cluster when the standard defines it that way,
- byte length/rune length is still bounded separately for abuse/memory safety.

# Byte Safety
Every public text field also has a hard UTF-8 byte limit chosen by the owning network/domain contract.

If the owning feature only specifies a grapheme limit, implementation must choose a conservative protocol byte ceiling high enough for legal text but still bounded; that ceiling is transport/validation safety, not the user-visible length definition.

Reject malformed UTF-8 before normalization.

# Name Limits (ADR-0065)
Canonical limits for unique names (evaluated after trim + NFC; both limits apply):
```text
entity      graphemes  UTF-8 bytes (display)  display column   name_key column
character   1..16      <= 64                  VARCHAR(64)      VARCHAR(256)
guild       1..24      <= 96                  VARCHAR(96)      VARCHAR(256)
```
`name_key` is `VARCHAR(256)` because case folding may expand code points. A name over either limit is rejected with the owning `*_NAME_INVALID` error; nothing is truncated.

Reserved: a character or guild name whose `name_key` starts with `anonymized_` is rejected; that prefix belongs to server-generated erasure placeholders (`data_model.md` § Account Erasure), which are exempt from the player grapheme limit.

# Control / Invisible Characters
Names:
- reject Unicode control characters,
- reject line/paragraph separators,
- reject empty/whitespace-only output,
- reject leading/trailing whitespace through canonical trim,
- format/invisible characters that create spoofing risk may be rejected by the server's explicit name policy, but such rejection must be deterministic/tested.

Chat:
- follows social.md for supported newline/control behavior,
- still uses NFC and UAX #29 grapheme counting.

Do not silently delete arbitrary interior characters to make input valid. Reject with a stable validation reason instead.

# Profanity / Reserved Names
Profanity/reserved-name checks run on canonical normalized text and may additionally inspect the folded name_key.

Moderation filtering is policy, not identity normalization. Changing a profanity list must not change the stored identity key for existing legal names.

# Unity
Unity may run equivalent normalization/count checks to give immediate UX feedback.

The server:
- recomputes canonical display value/key,
- enforces final grapheme/content limit,
- handles race through PostgreSQL UNIQUE name_key constraint.

Client acceptance never proves uniqueness.

# Database
Recommended columns for a unique-name entity:
~~~
name       text NOT NULL
name_key   VARCHAR(256) NOT NULL  -- canonical per § Name Limits and physical_schema_contract.md
UNIQUE(name_key)
~~~

Persist name only after successful server canonicalization. Do not generate name_key with a database locale-dependent expression.

# Migration
If the canonical normalization algorithm/library Unicode behavior is intentionally upgraded:
1. update technology_versions.md + ADR/spec,
2. evaluate all existing names for key collisions under the candidate algorithm,
3. resolve collisions explicitly before enforcing new keys,
4. migrate in bounded batches,
5. never silently rename/delete player identities.

A Unicode dependency update is therefore more than a package bump when it changes canonical segmentation/folding behavior.

# Test Vectors
Required tests include:
- NFC vs canonically equivalent NFD Vietnamese text,
- case variants producing same key,
- distinct Vietnamese diacritics producing distinct keys,
- đ vs d remaining distinct,
- leading/trailing Unicode whitespace trimmed,
- combining-mark grapheme count,
- emoji ZWJ grapheme count,
- invalid UTF-8 rejection,
- concurrent same-key creation: exactly one winner,
- PostgreSQL locale change does not change server-computed key behavior.

# Invariants
~~~
authoritative Unicode logic = Go
display normalization = trim + NFC
unique key = NFC(default Unicode case fold(display))
grapheme limit = UAX #29 Unicode 17
Vietnamese diacritics remain significant
DB locale/lower is not identity
client validation is non-authoritative
~~~
