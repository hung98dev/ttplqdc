---
name: produce-art-asset
description: Create or source a release art/audio asset — 2x authoring, cutout cleanup, volume/lighting, provenance, quality gates, in-game visual review. Use for IMP-070..076, IMP-104, IMP-105 and any client/Assets/Art change.
---

# Produce an Art Asset

Canonical: `docs/07_content/presentation_asset_manifest.md` §3–§7 (sizes, §3.1a gate scope, §3.2 Cutout Gate, §3.3 Visual Review, §3.5 art direction, §3.6 Volume & Depth Gate, §3.7 animation, §3.8 Style Pack, §3.9–§3.11 tile/9-slice/VFX/hitbox/atlas), ADR-0055, ADR-0056, ADR-0076, `docs/04_architecture/physics_geometry_contract.md`, cultural rules in `docs/07_content/cosmetic_catalog.md`.

## Workflow

1. **Target.** Resolve catalog ID → `asset_class` and either `size_profile` (Linh Thú = `SPIRIT_BEAST`; all 42 NPCs assigned IMP-104 = `NPC_HUMANOID`) or declared `cell_ref` (PROP, VFX) or fixed §3.1 size → exact 2x texture, PPU (100; UI 200; PARALLAX_FAR 1x 50), mesh type, compression, canonical group/key. NPC coverage includes prefab/one-hop alias for every roster ID, UI-required portrait, idle/interact clips and truthful cultural/provenance mapping; no collider derived from art.
   **Brief:** catalog ID, `folklore_card` (source tales, regional variants, forbidden motifs §5), `style_pack_id`, region palette, clip list from §3.7.
   **Turnaround first:** for classes and bosses, produce a 4-view turnaround with the Style Pack anchors and same seed; the reviewer approves it before any sprite is produced.
2. **Generate/source.** Before final-art claim, resolve manifest §5's actual provider/API/tool/model/version, reproducible explicit seed, terms snapshot/rights and output capability record in Owner Setup; generic in-session access is not proof. Demonstrate compliant actor PNG/layered PSB/rig import or the selected audio route; image generation alone does not supply rigs/audio. If seed/rights/capability are unavailable, stop that asset or select another compliant in-session route, never fabricate or ship placeholder. AI image: native alpha or `#FF00FF` key background, §3.5 lighting/volume prompt, Style Pack input, poses/parts generated separately. Free-licensed: CC0-1.0 / CC-BY-4.0, OFL-1.1 only fonts, original source page. Audio may be generated with a proven tool or permitted FREE_LICENSED. Keep no external desktop GUI dependency. Record actual model, seed, parameters, inputs and terms snapshot; workflow/model hashes null only when not exposed.
3. **Finish at target size.** Downscale once (area/Lanczos) to the exact 2x size, then clean edges, remove specks, dilate alpha ≥ 4 px, sharpen. Never ship raw "remove background" output; never let Unity resize.
4. **Gates.** Run the Unity EditMode Cutout + Volume & Depth validators (`bash .devin/scripts/verify_delta.sh --full`). Zero violations; fix the art, never the thresholds. Includes palette gate, frame consistency, tile seam / 9-slice / VFX limits, hitbox–silhouette alignment and the post-compression fringe check.
5. **In game.** Register media and stable keys using only the exact packet grants in `repository_layout.md` § Addressables Append Registry Grants: settings asset + named group assets and implied `.meta`, append your own disjoint entries only; no schema/profile or existing-entry changes. Do **not** add assets to IMP-070's `Scenes/Review/` files: its data-driven renderer reads catalog/keys/aliases/provenance and real map layers, failing missing contexts. `Unity (Windows)` renders the §3.3 resolutions/day-night/zooms, LOW motion and Style Pack contact sheets only after actual WARP/URP/RFloat capability proof (§3.3a). `-force-d3d11` alone never proves WARP; never write unconditional `renderer=warp`. Reference the `visual-review` CI run in manifest evidence; no local captures/committed screenshots.
6. **Provenance.** Add fragment row with source/final hashes, license/terms/inputs and `review_state = PENDING`. Every image has top-level `style_pack_id`; pure FREE_LICENSED has `generation_record = null`. No legacy nested style field or made-up AI metadata.
7. **Review.** A different agent (`reviewer`) checks the `visual-review` artifact against §3.3/§3.5 and the cultural rules, writes the verdict in a PR review comment and sets `APPROVED` or `REJECTED`. The reviewer scores the §3.3 rubric (0/1/2 per criterion; no 0, total >= 80%) and checks the `folklore_card`.

## Acceptance
- Exact 2x size, correct import settings, both gates pass, screenshots approved, provenance row `APPROVED`, no placeholder, no historical person or non-Vietnamese mythology motif.
