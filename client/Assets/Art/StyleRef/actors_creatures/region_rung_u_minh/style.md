# Style Pack: actors_creatures/region_rung_u_minh

Scope: IMP-104 creature art — monsters of zone `rung_u_minh`.

## §3.5 lighting/volume prompt scaffold (shared by every prompt)

- Painted-volume 2D chibi, Vietnamese countryside dark-fantasy folklore.
- One key light from top-front (upper-left, ~40 degrees above view plane).
- Exactly three value tiers per surface: lit plane, mid, shadow, plus a
  soft ambient-occlusion band in creases and under overlaps; no baked
  contact shadow on ground.
- Rim light ~2px along the top edge of the silhouette, warm neutral.
- Outline = darker tone of the object's local color (never black).
- 3/4 angled view facing slightly right; saturated warm vs cool contrast.
- Entities read more saturated than backgrounds.
- Background of generated input: flat `#FF00FF` (key), removed at finishing.

## Pack identity

`style_pack_id = actors_creatures/region_rung_u_minh` on every image row of the `actors_creatures`
provenance fragment. Palette lives in `palette.json` (Lab D65/2°); the
§3.8 gate requires >= 85% of silhouette pixels within DeltaE00 <= 8 of
the nearest listed palette color.

## Motifs

- Vietnamese countryside / Mekong-delta folk motifs only; checked against the forbidden list (torii, Qing robes, jiangshi hats, kimono, hanbok, modern religious/political symbols, meaningless Han/Nom characters) on every row.
