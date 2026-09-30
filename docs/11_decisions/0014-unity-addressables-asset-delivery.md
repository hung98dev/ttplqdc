# ADR-0014: Unity Addressables Asset Delivery
status: ACCEPTED

## Context
The Unity PC/mobile client needs a deterministic way to package and load maps, sprites, animations, VFX, audio, UI presentation, and later content without forcing every presentation asset into the base executable.

AI implementation agents must not independently choose Resources folders, raw AssetBundle APIs, custom downloaders, or different Addressables package versions. Asset delivery also must not become a second source of gameplay truth: authoritative gameplay content remains the versioned server/static-content contract.

## Decision
- Unity Addressables is the canonical launch asset-loading/content-delivery system.
- Exact package version is `com.unity.addressables 2.11.2`, pinned in `../00_context/technology_versions.md`.
- Direct `Resources.Load` is not a general game-content pipeline. A tiny bootstrap-only Resources use requires an explicit documented exception.
- Direct raw AssetBundle APIs are not used by gameplay/presentation code; Addressables owns bundle build, dependency resolution, catalog lookup, caching, and async loading.
- Asset keys are presentation identifiers and never replace canonical gameplay IDs.
- Gameplay/server content revision and Unity Addressables catalog/build identity are separate dimensions. The server remains authoritative for gameplay values/eligibility; remote asset delivery may only change compatible presentation assets/data explicitly classified as client presentation.
- The base install contains everything required to start the app, authenticate, display update/error/reconnect UI, and fetch/verify compatible presentation catalogs. Launch content may be split into local and remote Addressables groups according to the client asset spec.
- Entering a map/instance is presentation-gated until the client has all required compatible assets for that destination. Failure to fetch assets produces a client/update/download failure path; it never causes the server to accept a different map/state or client-supplied fallback.
- Remote catalog/content updates must be immutable/versioned, hash-verified through Addressables, published before the server/client build begins requiring them, and never mutate an already-published bundle under the same immutable identity.
- CDN/object-storage vendor is deployment configuration, not an SDK dependency. No vendor-specific Unity SDK is selected by this ADR.
- Addressable handles have explicit ownership/release; memory/resource lifetime is part of client correctness on mobile.
- A gameplay/content-schema breaking change still follows protocol/content-version rules and cannot be bypassed by downloading a new presentation catalog.

## Consequences
- One supported asset path works on PC and mobile.
- Remote presentation updates and staged content downloads do not require inventing a custom patcher.
- Addressables adds catalog/bundle/version management that must be tested and monitored.
- Server gameplay truth remains independent from Unity bundle contents.
- If a later platform needs a different delivery mechanism, it must preserve the same compatibility/authority boundaries and requires an architecture update.

## Invariants
```text
Unity asset system = Addressables 2.11.2
general Resources.Load content pipeline = disabled
direct raw AssetBundle gameplay pipeline = disabled
asset key != gameplay stable ID
remote assets != gameplay authority
required destination assets loaded before presentation attach
published bundle identity is immutable
asset handles are released explicitly
vendor-specific CDN SDK = none
```
