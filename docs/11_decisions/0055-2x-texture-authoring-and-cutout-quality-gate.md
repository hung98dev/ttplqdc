# ADR-0055: 2x Texture Authoring and Cutout Quality Gate
status: ACCEPTED

## Context
ADR-0046 fixes presentation at `ART_PIXELS_PER_METER = 50` reference pixels for a `1280x720` viewport and imported sprites at 50 PPU (1x). Players run 1920x1080 desktops, 1440p/4K monitors and phones with 1080..1440 px short edges, so 1x textures are magnified 1.5..3x and look soft. Assets are produced by AI agents; AI background removal commonly leaves colour halos, semi-transparent smears, stray specks and stair-stepped (aliased) edges, and AI images are often generated at an arbitrary size and resampled carelessly.

## Decision
- Reference geometry is unchanged: all px limits in specs (silhouette, cell, collider) are **reference px** at 50 px/m.
- Every gameplay/UI raster is authored and shipped at **2x** (`TEXTURE_SCALE = 2`): texture px = 2 × reference px, imported at **PPU 100**. World size, colliders and camera are unchanged.
- Each final texture is drawn/cleaned at its exact 2x target size. Generation or painting may happen larger, but the final pass (downscale with an area/Lanczos filter, then line/edge cleanup and sharpening) is done at the target size; no runtime or import-time resizing.
- A mandatory automated **Cutout Quality Gate** plus an in-game visual review gate apply to every alpha-bearing texture (`../07_content/presentation_asset_manifest.md` §3.2–3.4).
- Compression: characters, monsters, bosses, Linh Thú, UI, icons and fonts use ASTC 4x4 (mobile) / BC7 (desktop); backgrounds/parallax use ASTC 6x6 / BC7. Gameplay sprites have mipmaps off and bilinear filtering.

## Consequences
- `../07_content/presentation_asset_manifest.md` owns sizes, quality checks and review evidence; `../04_architecture/physics_geometry_contract.md` notes reference px vs texture px.
- Texture memory is ~4x per sprite versus 1x; Addressables budgets in the manifest stay binding and IMP-063/IMP-076 must measure them.
- IMP-063 (validator), IMP-070 (register/validator), IMP-071..075, IMP-104, IMP-105 (production) and IMP-076 (release audit) implement and evidence the gate.

## Amendment — UI and far-parallax PPU (ADR-0071)
UI sprites are 2x textures imported at **PPU 200** (Canvas Reference PPU 100) so they render at reference size; `PARALLAX_FAR` layers authored at 1x import at **PPU 50**. All other gameplay rasters stay 2x at PPU 100. Canonical: `../07_content/presentation_asset_manifest.md` §3.
