# Style Pack: instances/finale_omen

Scope: IMP-105 finale instance environment art —
`instance.finale.than_trung` (OMEN_CONVERGENCE_ARENA): the final convergence
arena where omen gates open and the thần trùng (sovereign centipede) surfaces
through a cracked seal field.

## §3.5 lighting/volume prompt scaffold (shared by every prompt)

- Painted-volume 2D environment art, Vietnamese countryside fantasy, night
  omen atmosphere — void indigo sky, corruption glow from below.
- One key light from top-front (upper-left, ~40 degrees above the view plane);
  corruption red uplight accents allowed inside seal cracks only.
- Exactly three value tiers per surface: lit plane, mid, shadow, plus a soft
  ambient-occlusion contact band; no baked cast/contact shadow on ground.
- Rim light 2-4 texture px on the shadow side edge, cool neutral.
- Outline = darker tone of the object's local color (never black), 2-4 px.
- 3/4 view for props; tiles read flat with material texture only.
- Material-distinct highlights (bone: matte broad; void stone: soft; spirit
  glow: translucent emissive).
- Entities read more saturated than parallax backgrounds.
- Cutout props: background of generated input is flat `#FF00FF` (key), removed
  at finishing. Tiles/parallax fill the whole canvas (no alpha).

## Pack identity

`style_pack_id = instances/finale_omen` on every finale image row of the
`instances` provenance fragment. Palette lives in `palette.json` (Lab D65/2°);
the §3.8 gate requires >= 85% of silhouette pixels within ΔE00 ≤ 8 of the
nearest listed palette color.

## Element color families (within palette)

- void_indigo: night-void indigos — sky, deep arena walls, omen haze.
- corruption_red: seal-bleed reds — glowing cracks, corruption veins, omen
  glyphs (telegraph red stays reserved: corruption glow lives BELOW the
  floor line, never inside walkable silhouette edges).
- bone_pale: weathered bone/chalk — ancient seal masonry, ritual stones.
- spirit_cyan: spirit-fire cyans — omen flames, gate seams, soul wisps.

## Finale-specific direction

- Tiles (TILE): seamless-tiling void-stone/cracked-seal fills at exact 2x;
  cracks glow beneath the top surface edge so lanes stay readable.
- Props (PROP): cutout artifacts grounded on their bottom edge — broken seal
  pillars, omen gates, bone monoliths, spirit braziers, centipede-spoor
  carvings; ≤4px float gap.
- Parallax (BACKGROUND): full-canvas far/mid layers — void horizon, collapsed
  temple silhouette, drifting omen shards; low saturation, dark.
</content>
