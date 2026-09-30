---
description: Unity client rules — C# 9.0, asmdefs, authority boundary, hot-path hygiene
trigger: glob
globs:
  - "client/**/*.cs"
  - "client/**/*.asmdef"
  - "client/**/*.unity"
  - "client/**/*.prefab"
  - "client/**/*.asset"
  - "client/**/*.meta"
---

# Unity Client Rules

Canonical: `engineering_conventions.md` §2, `04_architecture/client.md`, `04_architecture/client_performance.md` § Smoothness by Construction (ADR-0059). Unity `6000.6.1f1`, C# 9.0, .NET Standard 2.1, IL2CPP release builds.

## Style

- Allman braces, 4 spaces. PascalCase types/methods, camelCase locals/params, `_camelCase` private fields. C# 9.0 language level — do not use newer syntax.
- `csc.rsp` makes every warning an error and enables nullable; one top-level type per file, namespace = assembly + folder. The verifier's style check (Q4) enforces this — see `engineering_conventions.md` §2.7.

## Assemblies (acyclic)

```text
ThinhThan.Protocol   generated protobuf only, references nothing
ThinhThan.Core       math, IDs, text, pure domain models
ThinhThan.Net        WSS session, transport, serialization
ThinhThan.Systems    gameplay presentation, interpolation, controllers
ThinhThan.UI         HUD, menus, input overlays
ThinhThan.App        composition root (creates FrameLoop + services); nothing references it
ThinhThan.Tests.*    EditMode / PlayMode
```

Never create or edit an asmdef or `client/ProjectSettings/` entry: IMP-000 authors all 13 asmdefs and the ProjectSettings baseline (`docs/10_implementation/repository_layout.md` § Mandatory Assemblies, § ProjectSettings Baseline; ADR-0068); only IMP-095 edits `QualitySettings.asset`. A missing reference is a `BLK-xxx`. `Assets/Scripts/Protocol/` is generated — never hand-edit.

## Authority boundary (hard rule)

Client sends **intent**, never results. Never let client code decide: position legality, damage/heal/shield, hit confirmation, cooldowns, RNG, loot, balances, quest completion, trade/auction ownership, PvP rating, content activation. Predicted/presentation values are provisional until the server event arrives; Just Guard/clutch/reflect/lifesteal/absorb render only after `S2C_COMBAT_EVENT`. Scene state is not persistent authority.

## Layering

```text
Input/UI -> gameplay intent -> network session -> replicated state -> presentation
```

MonoBehaviours bridge Unity lifecycle and bind scene objects — no God Objects, no SQL-shaped or persistence-mutating constructs, no direct socket calls from UI. Business logic lives in plain C# classes where reasonable. Pair event subscriptions and resource handles with lifecycle cleanup; no fire-and-forget async work without cancellation/ownership.

## Frame model and hot path (ADR-0059)

- Only `FrameLoop` has Unity frame callbacks. Your code is an `IFrameSystem` ticked in its phase: `Input -> NetReceive -> Prediction -> Interpolation -> Presentation -> UI -> Camera`. Read time from `FrameTime`.
- Frame code allocates 0 bytes: no LINQ, boxing, string concat/interpolation, capturing lambdas, `params`, or collection growth. Entity views are index-based arrays with cached components.
- Non-urgent work (instantiate, list/UI population, Addressables completion, decode) goes through `FrameBudget` (≤ 2 ms per gameplay frame). Transient visuals come from `Pool<T>`, which is pre-sized at map load.
- Forbidden APIs and their replacements: `engineering_conventions.md` §2.5 (Q4 fence). One implementation per concern: §2.6 — never write a second pool, scheduler, logger or frame driver.
- UI: set dirty flags that are applied once in the UI phase; use separate static and dynamic Canvases; use `TMP_Text.SetText` for numbers; set `raycastTarget = false` on non-interactive graphics.
- Rendering: use `sharedMaterial` and `SpriteRenderer.color`, never `.material` or `new Material`; materials must be SRP-Batcher compatible; never sort in script.

## Serialization & assets

- Never rename a serialized field without `[FormerlySerializedAs]`; never break prefab/scene references.
- ScriptableObjects for static/shared config — not mutable global state.
- Do not touch scenes/prefabs outside task scope; avoid unrelated YAML churn.
- Addressables keys are stable, non-localized presentation IDs; no `Resources`/raw-AssetBundle shortcuts.
- Localization: authored text via Unity Localization keys, `vi-VN` + `en-US` required; gameplay identity never parses localized strings.

## Input

- Input System `1.20.0` only. Movement edges send `C2S_MOVEMENT_EDGE` (108, `DISCRETE_INTENT`) with `PRESS | RELEASE | FLIP` — no `STOP` exists; `client_mono_ms` is advisory, server clamps ≤80ms.

## Tests

- EditMode for pure logic/data/validation; PlayMode for WSS session flow and component lifecycle. Don't use PlayMode where EditMode suffices.
