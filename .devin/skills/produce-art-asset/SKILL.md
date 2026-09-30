---
name: produce-art-asset
description: Create or source a release art/audio asset — 2x authoring, cutout cleanup, volume/lighting, provenance, quality gates, in-game visual review. Use for IMP-070..076, IMP-104, IMP-105 and any client/Assets/Art change.
---

# Produce an Art Asset

Canonical: `docs/07_content/presentation_asset_manifest.md` §3–§7 (sizes, §3.1a gate scope, §3.2 Cutout Gate, §3.3 Visual Review, §3.5 art direction, §3.6 Volume & Depth Gate, §3.7 animation, §3.8 Style Pack, §3.9–§3.11 tile/9-slice/VFX/hitbox/atlas), ADR-0055, ADR-0056, ADR-0076, `docs/04_architecture/physics_geometry_contract.md`, cultural rules in `docs/07_content/cosmetic_catalog.md`.

## Workflow

1. **Target.** Resolve the catalog ID → `asset_class` and either `size_profile` (actors; every Linh Thú = `SPIRIT_BEAST`) or declared `cell_ref` (PROP, VFX) or the fixed §3.1 size (icons 64x64 ref, UI, tiles) → exact 2x texture size, PPU (100; UI 200; PARALLAX_FAR 1x = 50), mesh type (Tight when the long side ≥ 256 texture px with transparent margins), compression class, Addressables group and key (`docs/04_architecture/client_assets.md` § Grouping / § Stable Asset Keys).
   **Brief:** catalog ID, `folklore_card` (source tales, regional variants, forbidden motifs §5), `style_pack_id`, region palette, clip list from §3.7.
   **Turnaround first:** for classes and bosses, produce a 4-view turnaround with the Style Pack anchors and same seed; the reviewer approves it before any sprite is produced.
2. **Generate/source.** AI: native alpha or flat `#FF00FF` key background; prompt contains the §3.5 directives (top-front key light, 3 value tiers + occlusion, rim light, coloured outline, 3/4 view, material highlights). Free-licensed: only CC0-1.0 / CC-BY-4.0 / OFL-1.1 (fonts) from the original source page. Lock style with the Style Pack reference images/adapter (no LoRA at launch); generate each pose/part separately, never a whole sprite grid in one image. Animation: skeletal PSB with the fixed layer names for CHARACTER/MONSTER_MEDIUM+/boss, frame-by-frame for small actors, Spirit Beasts and VFX (§3.7). Record seed, model, parameters, workflow hash and a terms snapshot.
3. **Finish at target size.** Downscale once (area/Lanczos) to the exact 2x size, then clean edges, remove specks, dilate alpha ≥ 4 px, sharpen. Never ship raw "remove background" output; never let Unity resize.
4. **Gates.** Run the Unity EditMode Cutout + Volume & Depth validators (`bash .devin/scripts/verify_delta.sh --full`). Zero violations; fix the art, never the thresholds. Includes palette gate, frame consistency, tile seam / 9-slice / VFX limits, hitbox–silhouette alignment and the post-compression fringe check.
5. **In game.** Add the asset to its `client/Assets/Scenes/Review/` scene (IMP-070; real map layers of its region/instance). The Linux CI job renders 1280x720, 1920x1080, 2400x1080, day and night, 100% and 200% under xvfb + llvmpipe and uploads artifact `visual-review` (also the 960x540 LOW render with a 2 s motion clip and a contact sheet beside the Style Pack anchors); reference that CI run in the evidence manifest. Never capture locally, commit screenshots or treat them as evidence.
6. **Provenance.** Add the register fragment row (hashes of source and final, tool/version/terms, prompt, inputs, `review_state = PENDING`).
7. **Review.** A different agent (`reviewer`) checks the `visual-review` artifact against §3.3/§3.5 and the cultural rules, writes the verdict in a PR review comment and sets `APPROVED` or `REJECTED`. The reviewer scores the §3.3 rubric (0/1/2 per criterion; no 0, total >= 80%) and checks the `folklore_card`.

## Acceptance
- Exact 2x size, correct import settings, both gates pass, screenshots approved, provenance row `APPROVED`, no placeholder, no historical person or non-Vietnamese mythology motif.
