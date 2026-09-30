# ADR-0076: AI Art Pipeline — Consistency, Animation and Gate Corrections
status: ACCEPTED

## Context
All art is produced by AI agents with an owner-provided generation tool. A review of the art specs against 2025–2026 practice found: three gates that reject valid art (4-corner alpha rule applied to tiles/9-slice/far parallax; flat-region metric merging smooth gradients; top-light rule failed by dark hair), no animation contract (required clips were undefined, so coverage tests were not deterministic), no style-consistency mechanism across hundreds of assets, no review at the LOW render scale where unmipped sprites minify ~2x, no tile/9-slice/VFX/hitbox/atlas rules, an inconsistent base-install figure (82 vs 92 MB), no Vietnamese glyph set, cultural review limited to cosmetics, and incomplete provenance for reproducibility and terms.

## Decision
1. Gate corrections: corner rule scoped to cell-based classes; flat regions = 8-connected components of equal Lab bins (L* step 3, a*/b* step 6); top light = area-weighted median per (a*, b*) hue cluster >= 4; each with pass/fail fixtures (`presentation_asset_manifest.md` §3.2, §3.6).
2. Animation (§3.7): skeletal via the pinned PSD Importer 15.0.0 + 2D Animation 16.0.0 for `CHARACTER` (one shared skeleton, fixed PSB layer names, Sprite Library for cosmetics), `MONSTER_MEDIUM`+ and bosses; frame-by-frame for `MONSTER_SMALL`, `SPIRIT_BEAST` and VFX; clip table, frame/fps minimums, frame-consistency and pivot gates.
3. Style (§3.8): locked by reference images/adapters and per-packet Style Packs (`client/Assets/Art/StyleRef/<fragment>/`) with turnarounds approved before sprite production and a palette gate; no LoRA training at launch unless the first batch fails the palette gate (via ADR).
4. Review: 960x540 LOW render with motion; if shimmer is visible, enable exactly one mip level for `ACTOR` sprites and recompute §1 RAM budgets in the same spec change. Rubric 0/1/2 per criterion plus a contact sheet beside Style Pack anchors.
5. New rules: tile seams, 9-slice borders, VFX flipbook/blend/instance limits, hitbox–silhouette alignment, atlas padding >= 4 px with a fringe check on decompressed ASTC/BC7, upscale rule, Vietnamese glyph set + NFC + stacked-diacritic clipping test.
6. Provenance: `generation_record` gains model id/hash, seed, parameters, workflow hash, Style Pack id, reference hashes, terms snapshot hash (snapshot stored per fragment) and a C2PA flag; entities of cultural origin carry a `folklore_card` checked against a forbidden-motif list (initial list; spec-owner extends it).
7. No normal or mask maps at launch (keeps ADR-0056). The project accepts that purely AI-generated art may not be copyright-protectable in some markets; store AI-content disclosure is a release-checklist item of `IMP-067`.
8. Unverified thresholds (palette 85% within ΔE00 8, frame ΔE00 3 / 8 px, collider ratio 0.5..0.9) are calibrated on the first production batch through a gate-ratchet ADR.

## Consequences
- `../07_content/presentation_asset_manifest.md` (§3.1–§3.11, §5, §6, §7, Invariants, Requirement IDs `ART-001..012`), `../04_architecture/client_localization.md` (§ Fonts), `../04_architecture/client_performance.md` (item 6), `../00_context/technology_versions.md` (§ Content production tools), `../10_implementation/repository_layout.md` (StyleRef and terms ownership), `../10_implementation/task_queue.md` (`IMP-067`, `IMP-070..076`, `IMP-104`, `IMP-105`), `../10_implementation/spec_traceability.md` and `.devin` art skill/agent/rule are updated.
- Amends ADR-0055 (gate scope and metrics) and ADR-0056 (style locking, animation).
- `IMP-070` validator scope grows (new gates and fixtures); final-art packets gain Style Pack, animation, folklore and review obligations.
