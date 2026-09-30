---
description: Art/audio asset rules — 2x authoring, cutout and volume gates, provenance
trigger: glob
globs:
  - "client/Assets/Art/**"
  - "client/Assets/Audio/**"
  - "client/Assets/Scenes/**"
---

# Art & Audio Asset Rules

Canonical: `docs/07_content/presentation_asset_manifest.md` (ADR-0055, ADR-0056). Workflow: `/produce-art-asset`.

- Textures are authored and finished at exactly 2x reference px; import PPU 100 (UI 200); far parallax L3/L4 may be 1x at PPU 50; Linh Thú use `SPIRIT_BEAST`, PROP/VFX a declared `cell_ref`. No import/runtime resizing.
- Every file declares its `asset_class`; the Cutout Gate and Volume & Depth Gate apply per §3.1a. A violation is fixed in the art, never by loosening thresholds (gate ratchet, ADR-0050).
- Painted-volume 2D chibi: one top-front key light, three value tiers + occlusion, rim light, coloured outline, 3/4 view. Flat illustration fails review.
- Vietnamese folklore identity: no named historical/deified person on paid items, no non-Vietnamese mythology motifs.
- Every shipped media file has a provenance row with correct hashes and `review_state = APPROVED` by a different agent; no placeholder in release scope.
- Binary art/audio goes through Git LFS (`.gitattributes`); working files (PSD/PSB/AI originals) stay outside Addressables.
