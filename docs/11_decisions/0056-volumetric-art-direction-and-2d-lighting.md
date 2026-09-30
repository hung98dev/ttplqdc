# ADR-0056: Volumetric Art Direction and URP 2D Lighting
status: ACCEPTED

## Context
The only art rule was "stylized 2D chibi". AI-generated sprites and backgrounds tend to look flat: even fills, no consistent light, weak value range, backgrounds with the same contrast as the gameplay layer, entities that do not separate from the scene. Nothing in the specs described lighting, value structure, depth layers or a runtime lighting setup, so a flat-painting result passed every gate.

## Decision
- Art direction is **painted-volume 2D chibi**: one shared key light from above-front (left/right symmetric so `flipX` stays correct), three value tiers plus occlusion, rim light for separation, coloured outlines, 3/4 view for actors, material-specific highlights. Canonical rules and measurable thresholds live in `../07_content/presentation_asset_manifest.md` §3.5–3.6.
- Environments use five depth layers with parallax factors and atmospheric perspective; the gameplay layer and entities have the highest local contrast.
- Runtime uses the URP `17.6.0` **2D Renderer** (already pinned): `Sprite-Lit-Default` material, a per-map Global Light2D driven by the day/night cycle, point Light2D for lanterns/bonfires/VFX, a runtime soft contact shadow under every actor. No normal maps and no `ShadowCaster2D` at launch (memory/performance); light budget 8 in view on mobile, 16 on desktop.
- An automated **Volume & Depth Gate** and an extended Visual Review are added to the asset gates of ADR-0055.

## Consequences
- `presentation_asset_manifest.md`, `../04_architecture/client.md` and `../02_world/world_rules.md` (day/night presentation) are updated.
- IMP-101 owns the rendering setup (`client/Assets/Settings/Rendering/`, `client/Assets/Scripts/Core/Rendering/`); IMP-070 implements the gate; IMP-071..075, IMP-104, IMP-105 must pass it; IMP-076 re-runs it.

## Amendment — Light budget per quality preset (ADR-0071)
The active point Light2D budget is set only by the quality preset (`LOW` 4, `MEDIUM` 8, `HIGH` 16; `../04_architecture/client_performance.md` § Platforms and Device Tiers), not by platform. The "8 on mobile, 16 on desktop" wording above is superseded.
