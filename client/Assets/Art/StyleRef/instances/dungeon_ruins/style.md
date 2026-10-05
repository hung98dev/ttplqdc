# Style Pack: instances/dungeon_ruins

Scope: IMP-105 dungeon instance environment art — `dungeon.dinh_lang_bo_hoang`
(abandoned village hall, lamp alcoves), `dungeon.mieu_ba_trong_rung` (forest
shrine of Bà, root maze), `dungeon.xom_chim` (sunken hamlet, sluice gates and
rooftops above black water), `dungeon.hang_ma_tranh` (haunted warrior cave,
predator trail), `dungeon.den_tran` (temple of the seal, drum wings and stone
guardians).

## §3.5 lighting/volume prompt scaffold (shared by every prompt)

- Painted-volume 2D environment art, Vietnamese countryside fantasy, dusk-dim
  dungeon interiors.
- One key light from top-front (upper-left, ~40 degrees above the view plane);
  lamps/embers may add a local warm accent on props.
- Exactly three value tiers per surface: lit plane, mid, shadow, plus a soft
  ambient-occlusion contact band; no baked cast/contact shadow on ground.
- Rim light 2-4 texture px on the shadow side edge, warm neutral.
- Outline = darker tone of the object's local color (never black), 2-4 px.
- 3/4 view for props; tiles read flat with material texture only.
- Material-distinct highlights (stone: broad matte; brass/bronze: hard specular;
  wood/clay: soft; water: translucent emissive; moss/root: low-sheen).
- Entities read more saturated than parallax backgrounds.
- Cutout props: background of generated input is flat `#FF00FF` (key), removed
  at finishing. Tiles/parallax fill the whole canvas (no alpha).

## Pack identity

`style_pack_id = instances/dungeon_ruins` on every dungeon image row of the
`instances` provenance fragment. Palette lives in `palette.json` (Lab D65/2°);
the §3.8 gate requires >= 85% of silhouette pixels within ΔE00 ≤ 8 of the
nearest listed palette color.

## Element color families (within palette)

- neutral_stone: ink/stone warm grays — platforms, walls, carved drums.
- earth_moss: moss and root olives — forest floor, root maze overgrowth.
- lamp_amber: lamp oil ambers — alcove lamps, ember light, gilded seal marks.
- seal_crimson: lacquer-seal reds — seal glyphs, drum skins, omen cloth.
- water_murky: dark floodwater teals — sunken hamlet waterline, cave pools.

## Dungeon-specific direction

- Tiles (TILE): seamless-tiling ground/platform fills authored at exact 2x;
  edge-to-edge material (stone, packed earth, moss, waterline mud); no props
  baked into tiles; seams pass the § TILE seam check.
- Props (PROP): cutout artifacts grounded on their bottom edge — lamps, drums,
  carved seals, shrine roofs, sluice wheels, root clusters, tiger totems;
  ≤4px float gap.
- Parallax (BACKGROUND): full-canvas far/mid layers — silhouetted ridges,
  roofline canopies, misted forest, temple walls; lower saturation, no focal
  detail that competes with gameplay silhouettes.
- Telegraph/readability: floor under walkable lanes stays mid-value; hazard
  telegraph reds are reserved and never used in fills.
</content>
