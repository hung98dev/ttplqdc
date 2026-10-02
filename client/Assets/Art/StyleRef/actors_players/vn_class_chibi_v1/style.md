# Style Pack `actors_players/vn_class_chibi_v1` — player class actors

Approved rendering contract for the five launch classes (`class.{kim,moc,thuy,hoa,tho}`), per `presentation_asset_manifest.md` §3.5/§3.8 and `classes.md` folklore presentation rules.

## Lighting / volume prompt frame (§3.5)
- Painted-volume 2D chibi: head ≈ 1/3 of standing height; readable silhouette first, detail second.
- Single key light, top-front-left; shadow falls down-right. Exactly three value tiers (light / mid / shade) plus occlusion shadows at contact points.
- Soft warm rim light on the shadow edge (opposite the key) to pop against dark backgrounds.
- Colored outline — dark warm brown (`outline`), never pure black.
- 3/4 view, character faces right; feet planted at the cell bottom.
- Flat hand-painted finish: no photoreal texture, no airbrush gradients wider than a tier step.

## Palette
`palette.json` — 18 authored colors in five L* tiers (light ≥78, mid-light ~55–70, mid ~40–55, dark ~25–40, ink <25). Every shipped actor pixel quantizes to the nearest palette color (ΔE00 = 0 by construction → §3.8 palette gate).

## Anchors / turnarounds
- `anchors/`: 10 approved anchor images — 5 per-class hero studies + 5 four-view turnaround strips.
- `turnarounds/`: reviewer-approved 4-view sheets (front, right side, back, 3/4 back) per class; approved before sprite production.

## Folklore presentation rules honored
- Allowed: village and travel clothing, woven cloth, bamboo, wood, bronze/iron, paper, cord, herbal bundles, talisman paper.
- Forbidden and absent: Chinese cultivation robes, onmyoji/priest robes, Western plate-mage, glowing crystal armor, oversized Hán ceremonial dress. Element motifs read as materials (bronze blade, water cords, paper talismans, earth-hide), not glowing VFX.
