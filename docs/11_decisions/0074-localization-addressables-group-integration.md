# ADR-0074: Localization Addressables Group Integration

status: ACCEPTED (2026-09-27)

## Context

`com.unity.localization` 1.5.12 auto-registers imported locales and string-table
collections at import time: `LocalizationAssetPostProcessor.OnPostprocessAllAssets`
resolves `AddressableAssetSettingsDefaultObject.GetSettings(true)` — which, before the
provisioner's injected slot exists, creates `AddressableAssetsData/DefaultObject.asset`
and repoints the `com.unity.addressableassets` EditorBuildSettings config slot at it —
then `AddressableGroupRules` moves the assets into auto-created `Localization-Locales`,
`Localization-Assets-Shared` and `Localization-String-Tables-{locale}` groups. On PR #43
(IMP-064) this deterministically failed the canonical-group-set, per-group budget, EBS
baseline and group-assignment validators: the groups are non-canonical, have no budgets,
and their package-assigned addresses (`Core_vi-VN`) and labels (`Locale`) cannot satisfy
the `asset.*` grammar yet are exactly what the Localization runtime resolves by.

## Decision

1. The canonical Addressables group set gains `localization.locales`,
   `localization.shared` and `localization.strings.<locale_key>` (one per canonical
   locale: `vi_vn`, `en_us`; lowercase code, `-` -> `_`). All are local player-built
   groups, resident from `BOOT` — login/error UI must localize without a download.
   Budgets: `presentation_asset_manifest.md` §1.
2. The IMP-063 provisioner converges package-created state after import: it moves
   entries out of every `Localization-*` group into the mapped canonical group
   (locales → `localization.locales`; string tables → `localization.strings.<key>`
   by the `_<code>` address suffix; anything else → `localization.shared`) preserving
   package addresses/labels, removes the empty package groups, restores the
   `com.unity.addressableassets` EBS slot to the canonical settings object and deletes
   `DefaultObject.asset`. A committed settings asset never contains a non-canonical
   group and `unity-materialized-*` never contains `DefaultObject.asset`.
3. Entries inside `localization.*` groups are package-managed: they are exempt from
   the `asset.*` key grammar and from `KeyGroupRule` assignment; single-group
   membership and the canonical-set rule still apply to them.

Alternatives rejected: whitelisting raw `Localization-*` package names as canonical
(display-name-derived names are opaque and leave the "canonical set" cosmetic);
forbidding localization entries or routing them into `bootstrap.local` under `asset.*`
addresses (breaks the package's own resolution path the runtime depends on).

## Consequences

- IMP-063: provisioner gains `RehomePackageGroups` + `RestoreEditorBuildSettingsSlot`;
  `AddressableGroups.CanonicalNames()` grows to 29; validators exempt `localization.*`
  entries from the key grammar.
- IMP-064 imports locales/tables without editor-side suppression; the committed state
  is canonical after the provisioner pass.
- Any future Unity package that auto-creates Addressables groups needs the same
  convergence treatment before it can pass the canonical-set validator.
