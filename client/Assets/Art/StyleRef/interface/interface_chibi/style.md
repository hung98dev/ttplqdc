# Style Pack: interface/interface_chibi

Scope: IMP-073 interface art — UI chrome (9-slice, buttons, HUD frames), item /
equipment / skill / status icons, skill VFX and telegraphs.

## §3.5 lighting/volume prompt scaffold (shared by every prompt)

- Painted-volume 2D chibi, Vietnamese countryside fantasy.
- One key light from top-front (upper-left, ~40 degrees above the view plane).
- Exactly three value tiers per surface: lit plane, mid, shadow, plus a soft
  ambient-occlusion contact band; no baked cast/contact shadow on ground.
- Rim light 2-4 texture px on the shadow side edge, warm neutral.
- Outline = darker tone of the object's local color (never black), 2-4 px.
- 3/4 view; material-distinct highlights (metal: hard specular; wood/clay:
  broad soft highlight; cloth/leather: low-sheen; water/ice/fire/energy:
  translucent emissive).
- Entities read more saturated than backgrounds.
- Background of generated input: flat `#FF00FF` (key), removed at finishing.

## Pack identity

`style_pack_id = interface/interface_chibi` on every image row of the
`interface` provenance fragment. Palette lives in `palette.json` (Lab D65/2°);
the §3.8 gate requires >= 85% of silhouette pixels within ΔE00 ≤ 8 of the
nearest listed palette color.

## Element color families (within palette)

- KIM (metal): warm gold/steel — highlight cream, mid brass, shadow umber.
- MOC (wood): leaf/bamboo greens — highlight lime, mid leaf, shadow deep moss.
- THUY (water): river/teal blues — highlight ice, mid azure, shadow indigo.
- HOA (fire): ember reds — highlight apricot, mid vermilion, shadow maroon.
- THO (earth): laterite/clay — highlight sand, mid terracotta, shadow loam.

## UI chrome

- Panels/frames: aged parchment + dark lacquered wood + brass corners;
  neutral warm grays; status separation by shape (buff = round, debuff =
  triangle) and never by color alone.
- Telegraph decals: high-contrast hatched edge + fill band; readable without
  hue information (danger = dense double-outline + inner hatch band).
