# Style Pack: instances/competitive_arena

Scope: IMP-105 competitive environment art —
`map.pvp.duel_court` (MIRRORED_DUEL_BOWL), `map.pvp.five_element_arena`
(TRI_ALTAR_CIRCUIT), `map.guild_war.five_seal_conflict` (FIVE_SEAL_BRAIDED_FRONT).
All three spaces are mirror-symmetric about `x = max_x/2` within 0.001m; every
visual element exists in mirrored pairs so parity is by construction.

## §3.5 lighting/volume prompt scaffold (shared by every prompt)

- Painted-volume 2D environment art, Vietnamese countryside fantasy, clean
  arena daylight-to-dusk range — readability over mood.
- One key light from top-front (upper-left, ~40 degrees above the view plane),
  identical across the mirror axis (no directional asymmetry).
- Exactly three value tiers per surface: lit plane, mid, shadow, plus a soft
  ambient-occlusion contact band; no baked cast/contact shadow on ground.
- Rim light 2-4 texture px on the shadow side edge, neutral.
- Outline = darker tone of the object's local color (never black), 2-4 px.
- 3/4 view for props; tiles read flat with material texture only.
- Material-distinct highlights (arena stone: matte; metal fittings: hard
  specular; cloth banners: low-sheen; element glows: translucent emissive).
- Entities read more saturated than parallax backgrounds.
- Cutout props: background of generated input is flat `#FF00FF` (key), removed
  at finishing. Tiles/parallax fill the whole canvas (no alpha).

## Pack identity

`style_pack_id = instances/competitive_arena` on every competitive image row
of the `instances` provenance fragment. Palette lives in `palette.json`
(Lab D65/2°); the §3.8 gate requires >= 85% of silhouette pixels within
ΔE00 ≤ 8 of the nearest listed palette color.

## Element color families (within palette)

- arena_neutral: blue-gray arena neutrals — platforms, courts, spectator walls.
- kim: gold/steel accents — altar filigree, metal seals, court inlays.
- moc: bamboo greens — wood altars, vine growth, bronze patina.
- thuy: river blues — water altars, mist, wave carvings.
- hoa: ember reds — fire altars, battle pennants (never floor telegraphs).
- tho: laterite/ochre — earth altars, clay court marks.

## Competitive-specific direction

- Tiles (TILE): seamless-tiling arena stone/court fills at exact 2x; identical
  material on both mirror halves — parity is checked by the coverage test,
  so no asymmetric wear inside tiles.
- Props (PROP): cutout artifacts — five-element altars (kim metal, moc wood,
  thuy water, hoa fire, tho earth), seal braziers, duel banners, guild drums;
  paired variants L/R are the same texture, never two generated versions.
- Parallax (BACKGROUND): full-canvas far/mid layers — stadium walls, banner
  rows, sky gradient; center-line composition must remain symmetric.
- Telegraph/readability: lane floors stay mid-value; altar glow sits behind
  the capture area, never inside silhouette edges.
</content>
