# Đèo Mây environment style pack — `world/vn_env_deo_may_v1`

Fragment `world`, region `deo_may` (ADR-0076). Anchors in this folder are the
approved visual lock for every `Art/World/deo_may/` deliverable.

## Lighting frame (manifest §3.5)

cold alpine top light; violet shadow fill; frost rim; slate-violet outline.

- One top-front key light, symmetric L/R; 3 value tiers + ambient occlusion.
- Rim light on silhouette edge; outline is the pack `outline` color, never
  pure black.
- 3/4 view for props and structures; no painted ground shadow (runtime owns
  the contact shadow).
- Environment layers: L1 gameplay carries the highest contrast and value
  range; contrast and saturation decrease through L2 -> L3 -> L4.

## Palette gate

`palette.json` is the canonical color lock. Every shipped image quantized to
this palette (validator: >=85% of silhouette pixels within ΔE00 <= 8).

## Banned motifs (manifest §5)

No torii, qing robes, jiangshi hats, kimono, hanbok, modern
religious/political symbols, meaningless han/nom characters. Motifs must be
Vietnamese folk-material: cloud pass, stone ridges, pine groves, tiger-spirit territory.
