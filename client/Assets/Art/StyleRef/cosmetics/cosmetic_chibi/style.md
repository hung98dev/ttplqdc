# Style Pack: cosmetics/cosmetic_chibi

Scope: IMP-074 cosmetic presentation art — profile frames, character shrines
(miếu), title glows, nameplates, auras, weapon trails, emote icons, guild
banner/crest/shrine visuals and character appearances (Sprite Library parts).

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

`style_pack_id = cosmetics/cosmetic_chibi` on every image row of the
`cosmetics` provenance fragment. Palette lives in `palette.json` (Lab D65/2°);
the §3.8 gate requires >= 85% of silhouette pixels within ΔE00 ≤ 8 of the
nearest listed palette color.

## Element color families (within palette)

- neutral: ink/bone warm grays — inscription plates, stone, paper.
- kim_metal: warm gold/steel — frame filigree, crest brass, shrine bells.
- moc: leaf/bamboo greens — leaves, rain cape, bamboo banner poles.
- thuy: river/teal blues — water motifs, dew, azure silks.
- hoa: ember reds — lacquer red, ember glows, vermilion trims.
- tho: laterite/clay — shrine bricks, terracotta, earth tones.
- jade: jade greens — ngọc bích jewels, spirit jade.
- lacquer: deep plum/wine — lacquered wood, noble silks, dusk motifs.
- skin_hair: warm skin/hair browns — appearance character art.

## Cosmetic-specific direction

- Frames (UI_ART): ornate but readable border ring on a 256x256 ref canvas;
  center must stay empty (portrait shows through); the motif carries the
  theme (dragon coils, river reeds, arena metal, night festival lanterns).
- Shrines (PROP): small personal miếu — a tiny roofed shrine with an offering
  plate or lantern; bottom edge sits at cell bottom (grounded), ≤4px gap.
- Title glows / auras / trails (VFX_SOFT): soft additive-feel gradients with
  declared `soft_edges`; wide horizontal band (glow/trail) or round field
  (aura); no hard silhouette required.
- Nameplates (UI_ART): horizontal banner strip 320x64 ref with a left emblem
  space and motif along the strip.
- Emotes (ITEM_ICON): a single gesture prop or hand sign, chibi scale, reads
  at 64x64 ref.
- Guild (PROP/UI_ART): banners hang on a pole with ngũ ấn ribbon marks; crest
  accent is a small circular emblem; guild shrine is a communal stone altar.
- Appearances (COSMETIC_APPEARANCE): full-body chibi clothing variants sliced
  onto the shared §3.7 layer names (head, hair, torso, arm_front, arm_back,
  leg_front, leg_back, weapon, accessory_*); body proportions follow the
  shared skeleton — they change clothes, never anatomy/equipment identity.
