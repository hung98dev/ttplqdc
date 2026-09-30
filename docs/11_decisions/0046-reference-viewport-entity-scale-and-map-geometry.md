# ADR-0046: Reference Viewport, Entity Scale, and Map Geometry
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0068)**: item 6 geometry is exported from collision-only authoring scenes `client/Assets/Scenes/Collision/<space_id>.unity` by `ThinhThan.Core.Geometry.Editor.GeometryExporter` (both IMP-062); visual scenes carry no `ServerGeometry` colliders.

> **AMENDMENT NOTICE (ADR-0055)**: px values remain reference px at 50 px/m; textures are authored at 2x and imported at 100 PPU, with a mandatory cutout quality gate.

## Context

The client contract used `1920x1080`, while the intended game presentation is a `1280x720` reference viewport with a player silhouette about `96px` high. Entity catalogs did not resolve every monster and boss to a concrete visual/collision size. World, dungeon, PvP, and Guild War specs named playable spaces but did not give implementation-ready bounds or a required layout shape.

Without one conversion rule, an agent could treat pixels as physics units, resize colliders from art, or incorrectly build every map at screen size. Without per-space layout profiles, an agent could also satisfy rectangular bounds with repetitive horizontal corridors.

## Decision

1. The logical reference viewport is `1280x720` at `16:9`. It is a camera/UI design surface, not a map size or a restriction on the physical display resolution.
2. World physics remains `1 Unity unit = 1m`. Presentation uses `ART_PIXELS_PER_METER = 50`, so the reference camera shows `25.6m x 14.4m` and has orthographic size `7.2m`.
3. The baseline character silhouette is at most `64x96px` in idle/run/jump reference frames. Its collider remains `0.8m x 1.8m` (`40x90px` at reference scale). Weapon trails and skill VFX are separate presentation bounds.
4. Every non-player entity resolves to exactly one canonical profile: `MONSTER_SMALL`, `MONSTER_MEDIUM`, `MONSTER_ELITE`, `BOSS_LARGE`, or `WORLD_BOSS`. Catalogs own the exact resolution; runtime may not infer it from textures, names, or Transform scale.
5. Every playable space owns a rectangular outer bounds envelope and a topology profile. Normal-world maps are `2.0..5.0` reference viewports wide; all 24 use distinct width-height span pairs and distinct layout profiles. The full rectangle is not implicitly walkable: exported collision, platforms, branches, loops, and vertical tiers implement the named topology.
6. Geometry files use a generic `space_id` and `space_kind`, because the same exporter serves normal maps, dungeons, the finale, PvP, and Guild War. Output is `server/internal/sim/spatial/maps/<space_id>.geom.json`, exported from collision-only authoring scenes (ADR-0068).
7. The default desktop player window is `1280x720`. Larger native/fullscreen resolutions are supported without changing world-space scale or gameplay simulation.

## Consequences

- `docs/04_architecture/physics_geometry_contract.md` owns pixel/world conversion, camera extent, entity profiles, and geometry schema.
- `docs/04_architecture/client_experience_contract.md` owns reference viewport, scaling, and aspect-ratio behavior.
- `docs/02_world/maps_zones.md`, `monsters.md`, `bosses.md`, and `dungeons.md` consume stable bounds/profile fields without duplicating their values.
- `docs/06_data/config.md` and `content_authoring_contract.md` compile geometry by `space_id` and validate profile resolution.
- `docs/07_content/presentation_asset_manifest.md` owns sprite cell/silhouette import limits.
- `docs/07_content/monster_catalog.md` and `boss_catalog.md` resolve every launch entity to a profile.
- `docs/07_content/world_route_catalog.md` and `dungeon_catalog.md` own PvE bounds and unique topology profiles.
- `docs/03_systems/pvp.md` and `guild_war.md` own competitive-space bounds and topology.
- `docs/10_implementation/task_queue.md` makes IMP-013, IMP-018, IMP-019, IMP-024, IMP-025, IMP-040, IMP-041, IMP-042, IMP-062, IMP-063, IMP-065, IMP-066, and IMP-067 consume this decision where relevant.
- Existing logical spawn anchors remain authoritative under ADR-0003; this decision does not replace anchors with hard-coded coordinates.
- A monolithic map bitmap is not required. Pixel extents express reference coverage; tilemaps, reusable props, and parallax layers remain the delivery method.
