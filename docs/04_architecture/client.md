# Client
status: LOCKED

## Technology
Launch client:
```text
engine    = Unity
language  = C# 9.0
api       = .NET Standard 2.1
platforms = PC + mobile
```

Exact Unity editor, language/API profile, production scripting backend, Unity packages, and Protocol Buffer runtime are pinned in `../00_context/technology_versions.md`. The project must match those pins through `ProjectVersion.txt`, `Packages/manifest.json`, and `Packages/packages-lock.json`; a locally installed newer editor/package is not permission to upgrade the project.

Rendering uses the URP 2D Renderer (ADR-0056): `Sprite-Lit-Default`, one Global Light2D per map driven by the day/night cycle (`../02_world/world_rules.md`), point Light2D for lanterns/bonfires/skill VFX (active-in-view budget per quality preset only: `LOW` 4, `MEDIUM` 8, `HIGH` 16; `client_performance.md` § Platforms and Device Tiers), a runtime soft contact shadow under every actor, no normal maps and no `ShadowCaster2D` at launch. Art rules: `../07_content/presentation_asset_manifest.md` §3.5.

Launch presentation uses Unity `6000.6.1f1`, URP `17.6.0`, SRP Core `17.6.0`, the pinned Input System for shared PC/mobile controls, and IL2CPP for production player builds under the version matrix. Shader Graph, when used, remains pinned to `17.6.0`.

The stack decision is recorded in `../11_decisions/0006-unity-go-postgresql-stack.md`; version discipline is ADR-0010. Presentation asset packaging/loading/remote delivery is canonical in `client_assets.md` / ADR-0014 using Addressables 2.11.2. User-facing authored strings/assets follow `client_localization.md` / ADR-0015 using Unity Localization 1.5.12 with required launch locales `vi-VN` (default) and `en-US`.

## Responsibilities
Unity owns:
- keyboard/mouse and mobile-touch input collection,
- camera, rendering, 2D animation, VFX, audio, UI and accessibility presentation,
- client-side action/movement prediction where explicitly allowed,
- interpolation/extrapolation of replicated remote presentation,
- local asset/content lookup by stable IDs,
- local non-authoritative UX state such as menus, selected tabs and transient indicators,
- reconnect/resume presentation and client-side retry of idempotent requests.

Unity sends **intent**, never authoritative gameplay results.

## Forbidden Authority
The client never decides or persists authoritative:
```text
position legality
damage / healing / shields
hit confirmation
status application
cooldowns / resource spend
RNG result
loot / rewards
inventory ownership
equipment enhancement result
currency balances
quest/progression completion
guild/trade/Auction ownership
PvP result/rating
content revision activation
```
A client-computed value may be used for immediate presentation only and is corrected from server state.

## Local Player Prediction
For responsive side-scrolling movement, the local client may predict input-driven movement using the same published movement parameters as the server.

Prediction rules:
- input is sequence-numbered,
- client stores unacknowledged input history,
- server snapshot/ack is authoritative,
- on mismatch the client rewinds to authoritative state and reapplies still-valid local inputs,
- portal transfer, death, forced displacement, knockback, server correction, and content locks can force a hard reconciliation,
- client prediction never bypasses collision, map bounds, portal rules, or server validation.

Combat presentation may start immediately after local input, but damaging/healing/status results are displayed as provisional until authoritative resolution is received. No client hit result is trusted. Hitstop/slow-mo/`CUU_NGUY` juice and `just_guard_hint` start only after `S2C_COMBAT_EVENT`; the client must not predict Just Guard, clutch, the first-session hint, reflect damage, lifesteal heal, or absorb shield — these secondary results are rendered only from the authoritative combat event. Presentation clocks may freeze or slow independently of the simulation clock and never write HP, shields, status, or cooldown.

## Remote Entities
Remote players, monsters, bosses and projectiles are rendered from replicated server state using interpolation (delay, extrapolation and correction values: `client_performance.md` § Network Smoothness). The client may smooth visual motion but must not silently alter the server-owned logical position used for gameplay UI such as hit confirmation or targeting validity.

## Content
Unity consumes validated versioned static content compiled from `../06_data/config.md` and `../07_content/`. Presentation assets are resolved through the separately versioned Addressables contract in `client_assets.md`.

Rules:
- gameplay identity uses immutable IDs, never localized display strings,
- client knows the active content revision supplied by the server,
- incompatible schema/content revision fails connection or requests the supported content update path,
- art/presentation assets may map many visuals to one gameplay ID, but visuals never define gameplay values,
- Addressable keys/catalogs are presentation identity and never replace server stable gameplay IDs/content authority,
- Vietnamese localized names/diacritics are presentation data, not runtime identity,
- shipping authored prose uses stable localization keys rather than gameplay IDs/hard-coded MonoBehaviour strings,
- player-authored names/chat are not localized and remain governed by server Unicode rules.

## Scene / World Ownership
Unity scenes/maps are presentation and collision-source assets, not persistent world authorities.

Authoritative server-owned data includes:
- current logical map/map-instance/channel,
- accepted spawn/portal destination,
- character transform used for gameplay,
- active encounter membership,
- monster/boss lifecycle,
- world-event state.

The client cannot choose an arbitrary destination coordinate during normal transfer.

## Networking Boundary
Network code exposes typed request/response/event models to gameplay presentation code. Gameplay MonoBehaviours must not construct ad-hoc persistence mutations or SQL-shaped requests.

Recommended layering:
```text
Input/UI
  -> Client Gameplay Intent
  -> Network Session
  -> Replicated Client State
  -> Presentation/View
```
Do not make scene objects the canonical copy of persistent character/account state.

## Performance
Player-facing performance targets (device tiers, frame pacing, GC, load times, input responsiveness, network smoothness, mobile sustained performance), their gates, and the runtime architecture that enforces them (single `FrameLoop`, `FrameBudget`, pre-warm, adaptive quality governor, network receive path; ADR-0059) are canonical in `client_performance.md` § Smoothness by Construction.

## PC / Mobile
Gameplay rules are shared. Platform-specific differences are limited to input, UI layout, device integration, graphics/performance settings, and platform services.

Core combat must remain usable on mobile without requiring more simultaneous actions than the canonical hotbar/control design supports.

## Security / Robustness
Treat the Unity process as fully user-controlled/untrusted.

At minimum:
- never embed server/database credentials,
- never trust local save files for online progression,
- validate server certificates/secure transport according to network specs,
- bound packet/event queues to avoid client memory growth,
- malformed/unknown messages fail safely,
- reconnect does not duplicate actions or rewards,
- logs must not contain authentication secrets.

## Invariants
```text
engine = Unity
language = C#
PC/mobile gameplay rules are shared
client sends intent, not result
juice never mutates combat math; CUU_NGUY starts only after S2C_COMBAT_EVENT
server state wins every reconciliation
scene state != persistent authority
client never connects directly to PostgreSQL
```
