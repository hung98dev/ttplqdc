# Launch Encounter Catalog
status: LOCKED

## Scope
Concrete launch progression zones, field identities, enemy families, dungeons, and major bosses.

The visible world is **Vietnamese folklore fantasy**. Ngũ Hành remains a gameplay system underneath the setting; it must not turn the world into generic xianxia/elemental fantasy.

## Cultural Pillars
The launch world should be recognizable through Vietnamese spaces before the player reads any text:
```text
đình làng / cây đa / bến nước / lũy tre / ruộng lúa / đường đất
miếu nhỏ / am cũ / mộ hoang / rừng rậm / đầm lầy
xóm chài / bến đò / ghe thuyền / bãi lau / nhà ngập
đèo núi / hang đá vôi / bản ven rừng / đường mòn
thành cổ hư cấu / đền cổ / trống / cổng đá / núi thiêng
```

Supernatural content draws from Vietnamese folk and legendary motifs: ma da, ma trành, ma xó, quỷ nhập tràng, tinh cây, hổ tinh, hồ tinh, ngư tinh, thuồng luồng, thần trùng, wandering spirits, local taboos, abandoned shrines, and oral ghost-story motifs.

Folklore has regional variants. Game adaptations are fictionalized and must not claim one version is the definitive belief.

## Forbidden Drift
Do not default to:
```text
Chinese cultivation sects / immortal realms / generic Hán-fantasy place names
Japanese yokai structure
Western skeleton/zombie/orc fantasy
fire zone / water zone / earth zone merely because Ngũ Hành exists
generic crystal temples, magic academies, floating fantasy cities
```

Hán-Việt vocabulary is allowed where natural in Vietnamese language/history, but names must feel like Vietnamese places, stories, occupations, rituals, or objects rather than imported cultivation fantasy.

## Stable-ID Rule
All runtime IDs are ASCII lowercase snake_case. Vietnamese diacritics belong in display/localization text only.

## Launch Budget
```text
6 progression regions
18 adventure field maps
5 normal dungeons
8 major bosses
```

# Progression Route

| Act | Levels | zone_id | Display region | Folk identity | Dungeon |
|---|---:|---|---|---|---|
| I | 1-10 | `zone.lang_da` | Làng Đa | village, đình, banyan, rice fields, old graves | `dungeon.dinh_lang_bo_hoang` |
| II | 11-20 | `zone.rung_u_minh` | Rừng U Minh | wet forest, abandoned huts, wandering spirits, tree spirits | `dungeon.mieu_ba_trong_rung` |
| III | 21-30 | `zone.ben_nuoc_den` | Bến Nước Đen | ferry, fishing hamlet, reeds, drowned spirits, river monsters | `dungeon.xom_chim` |
| IV | 31-40 | `zone.deo_may` | Đèo Mây | mountain paths, tiger-spirit territory, ma trành, caves | `dungeon.hang_ma_tranh` |
| V | 41-50 | `zone.thanh_co` | Thành Cổ | fictional old fortification, temple ruins, guardian legends | `dungeon.den_tran` |
| VI | 51-60 | `zone.nui_thieng` | Núi Thiêng | sacred mountain, old boundary stones, convergence of prior omens | finale |

These are fictional game regions. They may borrow environmental vocabulary from real Vietnam but are not literal reconstructions of living communities or sacred sites.

# ACT I — Làng Đa
Purpose: establish Vietnamese identity immediately and teach core movement/combat.

## Fields
```text
map.lang_da.bo_ruong     Lv1-4  — rice paddies, irrigation banks, scarecrows
map.lang_da.ben_da       Lv4-7  — banyan, ferry steps, riverbank
map.lang_da.go_ma        Lv7-10 — old graves, bamboo edge, abandoned field shrine
```

Safe/social anchor: `map.lang_da.dinh_lang`.

## Enemy Family
```text
monster.lang_da.dom_dom_ma      NORMAL — drifting spirit light, simple contact burst
monster.lang_da.bu_nhin_rom     NORMAL — straw scarecrow rush
monster.lang_da.coc_thanh_tinh  NORMAL — hop + visible tongue line
monster.lang_da.quy_nhap_trang  NORMAL — possession shimmer wind-up, WEAKEN on control hit
monster.lang_da.vong_hon        NORMAL — slow committed strike, incorporeal phase-flicker visual
monster.lang_da.hon_ma_co_thu   NORMAL — extended-range rush (7.0m), standard telegraph
monster.lang_da.hon_do_trang    NORMAL — slow committed strike, incorporeal phase-flicker visual
monster.lang_da.hon_xo_non      ELITE  — marked forbidden ground zone, no hard CC
monster.lang_da.ma_xo           ELITE  — marks one forbidden ground patch before appearing
monster.lang_da.vong_hon_gia    NORMAL — float-lane melee, incorporeal phase-flicker visual
monster.lang_da.dom_dom_nguyen  NORMAL — season-0 variant spirit light, delayed burst
monster.lang_da.bup_lua         NORMAL — season-0 fire-spirit drifter, delayed burst
monster.lang_da.tinh_buoi       NORMAL — season-0 tree-spirit melee
monster.lang_da.co_lua          NORMAL — season-0 scarecrow rush
monster.lang_da.vong_bien       NORMAL — season-0 wanderer melee
monster.lang_da.hon_gao         NORMAL — season-0 grave-rice spirit melee
```

## Dungeon — Đình Làng Bỏ Hoang
`dungeon.dinh_lang_bo_hoang`, target `15-18 min`.

Stages:
1. relight three old lamps around the đình courtyard
2. cross banyan roots and rear garden while avoiding marked spirit ground
3. defeat `boss.quy_nhap_trang`

The abandoned đình is a fictional location affected by a supernatural incident; the game does not portray communal worship itself as evil.

## Boss — Quỷ Nhập Tràng
`boss.quy_nhap_trang`

Stylized from the Vietnamese folk motif of a corpse unnaturally rising/moving; no gore.

Tests: telegraph dodge + positioning.

```text
BAT_DAY  -> stiff forward lunge after 0.9s tell
DUOI_DEN -> marks one lantern lane, extinguishes it, then charges through it
50% HP   -> two sequential lane marks; one safe route always remains
```

# ACT II — Rừng U Minh
Purpose: deep wet-forest ghost stories, getting lost, strange lights, tree spirits, and wandering ma trành.

## Fields
```text
map.rung_u_minh.loi_tram       Lv11-14 — wet trail and old wooden walkways
map.rung_u_minh.rung_sau       Lv14-17 — dense forest, black water pools
map.rung_u_minh.mieu_bo_hoang  Lv17-20 — abandoned shrine clearing
```

Safe anchor: `map.rung_u_minh.xom_rung`.

## Enemy Family
```text
monster.rung_u_minh.ma_rung        NORMAL — visible brush ambush rush
monster.rung_u_minh.dom_lua        NORMAL — lure marker, delayed float burst
monster.rung_u_minh.bong_nguoi     NORMAL — short delayed echo strike
monster.rung_u_minh.tinh_cay       NORMAL — ranged control, ROOT 1.0s on hit
monster.rung_u_minh.dai_tinh_cay   NORMAL — ranged control, ROOT 1.5s + vine-pull visual
monster.rung_u_minh.vong_rung_sau  NORMAL — dual-track rush tell, only one lane active
monster.rung_u_minh.ma_tranh       ELITE  — leap/ambush with recovery opening
monster.rung_u_minh.moc_tinh       ELITE  — one root zone + nearby plant-spirit support
```

## Dungeon — Miếu Bà Trong Rừng
`dungeon.mieu_ba_trong_rung`, target `17-20 min`.

Stages:
1. follow three visible trail markers through an authored lost-path sequence
2. clear supernatural roots around an abandoned miếu
3. defeat `boss.moc_tinh_da`

## Boss — Mộc Tinh Cây Đa
`boss.moc_tinh_da`

An old banyan spirit wrapped around the fictional abandoned shrine.

Tests: safe-zone movement + add priority.

```text
RE_GIA   -> marked root sections
MAM_AM   -> at most 2 growth adds
THU_THAN -> trunk guarded while growths remain
```

# ACT III — Bến Nước Đen
Purpose: river and ferry folklore: drowned spirits, ma da, fishing life, reeds, floods, and creatures beneath the water.

## Fields
```text
map.ben_nuoc_den.bai_lau     Lv21-24 — reeds and muddy riverbank
map.ben_nuoc_den.duong_ngap  Lv24-27 — flooded paths and stilted homes
map.ben_nuoc_den.ben_do_cu   Lv27-30 — abandoned ferry landing
```

Safe anchor: `map.ben_nuoc_den.cho_ben`.

## Enemy Family
```text
monster.ben_nuoc_den.ma_da          NORMAL — water-edge grab with visible reach, PULL 1.5m
monster.ben_nuoc_den.ca_tinh        NORMAL — lane dash, SONG_TRA_NGAN trailing water zone, SLOW 10%
monster.ben_nuoc_den.quy_song_dem   NORMAL — dark lure visual, arcing projectile from above-target
monster.ben_nuoc_den.thuong_luong   NORMAL — lane dash, CUON_XA: pull 1.5m toward origin on connect, SLOW 15%
monster.ben_nuoc_den.bong_nuoc_ma   NORMAL — floating delayed burst, 1.0s fuse
monster.ben_nuoc_den.hon_chet_duoi  NORMAL — slow water projectile, SLOW 15% for 2s
monster.ben_nuoc_den.ca_tinh_gia    NORMAL — lane dash, SONG_TRA_NGAN trailing water zone, SLOW 10%
monster.ben_nuoc_den.nguoi_song_co  NORMAL — ranged SONG_CUNG: two sequential shots, 0.5s gap
monster.ben_nuoc_den.ma_da_gia      ELITE  — pull then telegraphed slam (PULL 2m + 1.20 ATTACK slam)
monster.ben_nuoc_den.thuy_quai      ELITE  — rotating water opening, no unavoidable damage
```

## Dungeon — Xóm Chìm
`dungeon.xom_chim`, target `18-22 min`.

Stages:
1. raise two old sluice gates while water changes platform lanes
2. cross roofs and half-submerged homes
3. defeat `boss.thuong_luong`

## Boss — Thuồng Luồng
`boss.thuong_luong`

Large river creature inspired by Vietnamese thuồng-luồng traditions.

Tests: platform movement + directional telegraphs.

```text
QUET_DUOI -> horizontal lane preview
NUOC_DANG -> temporarily floods lowest platform tier
CUON_SONG -> three ordered water strikes
```

Phase 2 starts below 50% HP (`boss_catalog.md`); below 40% HP two mechanics may overlap; at least one reachable safe platform must remain.

## Public Boss — Ma Da Chúa
`boss.ma_da_chua`, recommended Lv30.

A large fictional manifestation of the ma-da motif, not a deity.

Alternates authored left/right flood surges and marked grabs. Contribution/reward/channel rules remain canonical in `../02_world/bosses.md`.

# ACT IV — Đèo Mây
Purpose: mountain-road fear, tiger-spirit stories, ma trành, caves, and travelers being led astray.

## Fields
```text
map.deo_may.duong_rung  Lv31-34 — forest road and bamboo bridges
map.deo_may.khe_da      Lv34-37 — stream, rocks, limestone openings
map.deo_may.rung_cam    Lv37-40 — forbidden old hunting forest
```

Safe anchor: `map.deo_may.ban_chan_deo`.

## Enemy Family
```text
monster.deo_may.ma_tranh       NORMAL — flanking rush after visible trail sign
monster.deo_may.khi_nui        NORMAL — thrown-object arcing projectile
monster.deo_may.ma_van_dem     NORMAL — fog-veil float burst, mist visual matches burst timing
monster.deo_may.ho_tinh        NORMAL — BIEN_HOA: feint direction change mid-rush
monster.deo_may.ho_con_tinh    NORMAL — pounce line, SAT_BO: BLEED after rush lands
monster.deo_may.vong_rung      NORMAL — SLOW 15% zone; FLOAT_LANE
monster.deo_may.ho_tinh_lon    NORMAL — pounce line, SAT_BO: BLEED after rush lands
monster.deo_may.vong_nui_gia   NORMAL — LOAN_VUNG: expanding zone 1.5m→3.0m, SLOW 15% inside
monster.deo_may.ho_tinh_ve     ELITE  — two authored rushes max, 1.5s recovery opening
monster.deo_may.ma_tranh_gia   ELITE  — one false-trail danger lane
```

## Dungeon — Hang Ma Trành
`dungeon.hang_ma_tranh`, target `18-22 min`.

Stages:
1. follow visible trail charms across forest/cave boundary
2. survive authored ambush lanes without losing route readability
3. defeat `boss.ho_tinh`

## Boss — Hổ Tinh
`boss.ho_tinh`

Draws from Vietnamese tiger-spirit and ma-trành motifs, not a generic elemental tiger.

Tests: burst window + pattern recognition.

```text
VET_VUOT -> two delayed claw lanes
VO_MOI   -> three previewed pounce lanes
UY_SON   -> 4s roar channel; damage/stagger weakens following pressure wave
```

Failure to fully suppress `UY_SON` is survivable pressure, never instant wipe.

# ACT V — Thành Cổ
Purpose: move from village ghost stories to legend-scale ruins while remaining recognizably Vietnamese.

The city is fictional; architecture uses Vietnamese fortification/temple visual language without claiming to reconstruct a specific dynasty or monument.

## Fields
```text
map.thanh_co.duong_da   Lv41-44 — old road, brick walls, gate remnants
map.thanh_co.hao_can    Lv44-47 — dry moat, broken watch posts
map.thanh_co.den_tran   Lv47-50 — sealed temple approach
```

Safe anchor: `map.thanh_co.cong_ngoai`.

## Enemy Family
```text
monster.thanh_co.tuong_da      NORMAL — slow armored strike, strong hit-reaction resistance
monster.thanh_co.hon_binh      NORMAL — committed weapon lane presentation
monster.thanh_co.ma_co         NORMAL — delayed ground mark (ZONE)
monster.thanh_co.oan_hon_dem   NORMAL — OAN_HON: cursed inscription wind-up, WEAKEN 15% ATTACK on control hit
monster.thanh_co.qua_tinh      NORMAL — KHUC_XA ricochet projectile off authored terrain
monster.thanh_co.hon_tran_linh NORMAL — committed overhead strike, clear downward swing visual
monster.thanh_co.qua_tinh_lon  NORMAL — KHUC_XA ricochet projectile off authored terrain (larger variant)
monster.thanh_co.thach_ve      ELITE  — frontal GUARD + PHAN_CHIEU RUSH counter after guard
monster.thanh_co.hon_tuong     ELITE  — SUMMON_ECHO: one temporary echo mirrors combat pattern
```

## Dungeon — Đền Trấn
`dungeon.den_tran`, target `20-25 min`.

Stages:
1. sound three old seal drums while spectral guards control lanes
2. defeat stone guardians through recovery windows
3. cross a broken-wall sequence with readable falling debris
4. defeat `boss.ho_tinh_chin_duoi`

## Boss — Hồ Tinh Chín Đuôi
`boss.ho_tinh_chin_duoi`

Inspired by Hồ Tinh motifs found in Vietnamese legendary tradition. Visual/narrative direction must distinguish it from generic East-Asian nine-tailed-fox templates by grounding the encounter in this game's Vietnamese landscape and story.

Tests: pattern recognition + positioning.

```text
CUU_ANH      -> tail shadows preview lanes; only 2 become dangerous
LUA_MA       -> spirit-fire movement markers with preserved safe ground
LO_CHAN_THAN -> selected resolved patterns expose the true body for 5s
```

At 30% HP exposure becomes 4s; telegraph time never shrinks.

Phase 2 (below 50% HP): every 10s the boss selects one active dangerous shadow lane to absorb. That lane flares with a distinct inward-spiral pattern for 2.5s — a shape tell that does not depend on colour. Players can destroy the selected lane before the 2.5s window expires to interrupt the absorb entirely (no stack, no window reduction, safe ground restored normally). If the lane is not destroyed in time it is absorbed: one stack accrues (max 3), and each stack reduces the LO_CHAN_THAN exposure window by 1.0s. This is the learnable action grounded in the Vietnamese tail/shadow motif: the fox spirit tries to reclaim its cast shadow; the player can sever that shadow before it is drawn back.

# ACT VI — Núi Thiêng
Purpose: culmination at a fictional sacred mountain where the disturbances from previous regions converge. No celestial palace, cultivation sect, immortal realm, or new power system appears.

## Fields
```text
map.nui_thieng.rung_may    Lv51-54 — cloud forest, old path shrines
map.nui_thieng.suon_da     Lv54-57 — cliffs, boundary stones, caves, mountain-fed deep lake (has_water = true; fishing_spot required; anchors boss.ngu_tinh)
map.nui_thieng.cong_co     Lv57-60 — old mountain gate and ritual path
```

Safe anchor: `map.nui_thieng.chan_nui`.

## Enemy Family
```text
monster.nui_thieng.vong_linh    NORMAL — destination-marker phase-step rush
monster.nui_thieng.tinh_thu     NORMAL — clearly displayed elemental projectile
monster.nui_thieng.than_rung_dem NORMAL — rush with 0.8s telegraph before dash
monster.nui_thieng.ngu_tinh     NORMAL — LUONG_LONG: two simultaneous projectiles, authored safe gap ≥0.8m
monster.nui_thieng.ma_nui       NORMAL — lane-pressure ground-mark ZONE
monster.nui_thieng.than_trung   NORMAL — TAI_HOA: omen ZONE, VULNERABLE escalates on repeated exit
monster.nui_thieng.hon_binh_co  NORMAL — committed weapon sweep
monster.nui_thieng.dai_vong_linh NORMAL — BIEN_HOA: feint direction change mid-rush
monster.nui_thieng.tinh_nui_gia NORMAL — TRAM_DOC: VULNERABLE -15% DEFENSE 3.0s on zone exit
monster.nui_thieng.linh_ve      ELITE  — PHAN_CHIEU: alternates attack/guard, RUSH counter after guard
monster.nui_thieng.bong_vong    ELITE  — HON_CUOP: CONTROL applies STUN 0.5s micro-interrupt + self-shield drain
```

## Public Boss — Ngư Tinh
`boss.ngu_tinh`, recommended Lv55.

Ngư Tinh is moved here from the earlier forest concept: it appears at a mountain-fed deep lake connected to the final region's waterways. This avoids forcing a sea/river legend into an unrelated forest biome.

Tests: add priority + positioning. At 70% and 40% HP, at most three spirit fragments appear; clearing them opens safe ground and a damage window.

# Final Boss — Thần Trùng
`boss.than_trung`, recommended Lv60 instanced finale.

A fictional game interpretation inspired by Vietnamese folk beliefs around ominous repeated death/misfortune. It is not presented as a definitive religious account.

Tests: remembering and combining mechanics learned across the journey.

## Phase 1 — Dấu Cũ (`100-70%`)
One familiar regional omen at a time:
```text
LANG_DA       -> committed lane strike
RUNG_U_MINH   -> growth/add priority
BEN_NUOC_DEN  -> moving water wave
DEO_MAY       -> ordered ambush lanes
THANH_CO      -> guard/recovery window
```

## Phase 2 — Trùng Điệp (`70-35%`)
Exactly two authored motifs overlap. Pair order is deterministic for the encounter revision; unfair random combinations are forbidden.

### Pair 1 — Làng Rừng Gặp Nhau
Motifs: LANG_DA ∥ RUNG_U_MINH

LANG_DA committed lane strike fires simultaneously with a RUNG_U_MINH growth add emergence. The growth add's eruption tell (≥0.80s) and the lane strike arrow tell appear together; the two tells are shape-distinct (eruption ground marker vs. horizontal sweep arrow; neither depends on colour). If the growth add survives until the lane strike resolves, a secondary lane strike fires 0.75s later targeting the growth add's position. The two lane strikes together respect the Phase 2 two-hit limit (no more than two damaging resolutions to one target within 0.8s; primary and secondary strikes target different authored positions so both may connect to the same player only if standing on the eruption point). Primary lane strike coefficient: 1.35 ATTACK. Secondary strike coefficient: 1.50 ATTACK (Phase 2 heavy cap). Growth add at normal motif coefficient (0.80). Safe resolution: destroy the growth add inside its eruption window → secondary strike does not fire; then dodge the primary lane.

### Pair 2 — Sóng Cổ Vùng Dậy
Motifs: BEN_NUOC_DEN ∥ THANH_CO

BEN_NUOC_DEN moving water wave sweeps while a THANH_CO guard/recovery spectral panel appears at the authored center-ground position. The guard panel's 1.2s pulsing-border tell (shape: rectangular flickering frame, not colour-dependent) precedes the wave's arrival. Sheltering behind the panel blocks the wave hit entirely; the panel shatters on wave contact, opening a 3.5s boss damage-taken 1.20 window. Players not behind the panel survive via the authored elevated platform safe zone above the wave path (always present; tell timing ≥ Phase 1 water wave value). Water wave coefficient: 1.20 ATTACK. Panel provides full wave absorption for characters in the authored 1.5m shelter zone behind it. The 3.5s window is shorter than Phase 3's 5.0s window to maintain difficulty escalation.

## Phase 3 — Canh Cuối (`35-0%`)
Three authored combination patterns rotate. Arena lighting becomes darker but gameplay telegraphs remain high contrast. Resolving a pattern opens a 5s boss damage-taken 1.20 window. All three patterns respect the max-two-simultaneous-mechanic-families guardrail.

### Pattern 1 — Đèo Mây Thành Cổ
Motifs: DEO_MAY ∥ THANH_CO

Four DEO_MAY ambush lanes announce in authored order (standard lane-preview telegraph) while a THANH_CO guard/recovery panel appears at one authored safe pocket. Players must reach the guard pocket within the 3.0s ambush window. The final ambush lane targets the panel position; surviving inside the pocket causes the panel to shatter, opening the 5s damage window. Coefficient per ambush lane: 1.30 ATTACK. Final lane (targeting guard pocket): 1.40 ATTACK.

### Pattern 2 — Rừng Sóng Dậy Thôn
Motifs: RUNG_U_MINH ∥ BEN_NUOC_DEN

Two RUNG_U_MINH growth adds emerge at authored mid-arena positions while BEN_NUOC_DEN water wave sweeps between them. Clearing both growth adds within the wave's 2.0s travel window converts each add's location to an authored safe platform for 1.5s; the wave does not damage a character standing on a cleared-add safe platform. Clearing both adds before wave resolution opens the 5s damage window. Water wave coefficient: 1.25 ATTACK. Growth adds at normal motif coefficient (0.80).

### Pattern 3 — Làng Đa Vào Núi Thiêng
Motifs: LANG_DA ∥ NUI_THIENG

NUI_THIENG is a sixth motif introduced only in Phase 3, representing the sacred mountain's own convergence: boundary-stone omen markers appear at arena edges and contract an authored safe strip from 3.0m to 1.5m width over 2.5s. Simultaneously, a LANG_DA committed lane strike fires through the center of the contracting strip. The strip's remaining width at resolve time (≥1.5m) is always sufficient for a standing character. Players who exit the strip before the lane strike resolves receive WEAKEN 10% ATTACK for 2.0s (the boundary omen's price for straying). Tell for the contracting strip: double-ring ground markers converging inward, shape-distinct from the lane strike sweep arrow; neither depends on colour. Lane strike resolves, strip locks for 0.5s, then the 5s damage window opens. Lane strike coefficient: 1.40 ATTACK. WEAKEN is a debuff, not a damage hit.

No folklore quiz, instant-kill knowledge check, or mandatory five-element party composition.

# Canonical Eight Major Bosses
```text
1 boss.quy_nhap_trang       Act I dungeon
2 boss.moc_tinh_da          Act II dungeon
3 boss.thuong_luong         Act III dungeon
4 boss.ma_da_chua           Act III public
5 boss.ho_tinh              Act IV dungeon
6 boss.ho_tinh_chin_duoi    Act V dungeon
7 boss.ngu_tinh             Act VI public
8 boss.than_trung           Act VI finale
```

# Lv60 Endgame Reuse
The five launch dungeons may expose explicit Lv60 reward variants with selected authored mechanic remixes.

Rules:
```text
no generic difficulty ladder
no stat-only HP sponge mode
no entry key/currency
no mandatory daily clear
reward variant visible before entry
normal dungeon death/reconnect/membership rules unchanged
```

## Weekly Highlighted Dungeon Rotation
One dungeon per week is designated the weekly highlight and adds a cosmetic/material bonus reward table on top of its normal drops. All other dungeons remain fully playable and rewarding at their normal rates; the highlight designation does not lock out non-highlighted dungeons.

### Selection Rule
The weekly highlighted dungeon is derived deterministically:
```text
utc_week_number = floor((server_utc_seconds - 345600) / (7 * 86400))   -- 345600 s = 4 days: Unix epoch was Thursday, so weeks start Monday 00:00 UTC
highlight_index  = utc_week_number mod 5
```

| highlight_index | featured_dungeon_id |
|:---:|---|
| 0 | `dungeon.dinh_lang_bo_hoang` |
| 1 | `dungeon.mieu_ba_trong_rung` |
| 2 | `dungeon.xom_chim` |
| 3 | `dungeon.hang_ma_tranh` |
| 4 | `dungeon.den_tran` |

The selection is deterministic from `utc_week_number`; no stored highlight row is required.

### Reset Boundary
```text
weekly highlight resets: Monday 00:00 UTC
```
A run started before the reset and completing after does not retroactively change its reward table; the table is resolved at run-start using the `utc_week_number` at the moment the dungeon instance is created.

### Reward Tables
The weekly highlight bonus tables (`reward_table.weekly.*`) and their grant rule are owned by `drop_tables.md` § Weekly Highlight Bonus. The bonus is granted **once per character per `utc_week_number`**, on the first eligible completion of the highlighted dungeon.

No weekly highlight reward grants combat stats, enhancement levels, or exclusive permanent power.

# Readability Guardrails
For 2D side-scroll/mobile:
```text
max simultaneous major boss mechanic families = 2
normal boss dangerous adds target <= 4
persistent danger must preserve reachable safe ground
telegraph contrast cannot depend only on color
camera must show required reaction space before a mechanic resolves
```

# Cultural Review Gate
Before a named folklore creature, ritual, sacred object, historical figure, or real location is promoted to production content:
1. record the source/inspiration used by the content team
2. distinguish documented motif from game invention
3. check regional/religious sensitivity where applicable
4. prefer fictional villages/temples/characters when a real sacred site is unnecessary
5. verify visual references are Vietnamese rather than generic East-Asian substitutes

The goal is not museum reconstruction. The goal is a fantasy MMORPG whose silhouettes, environments, stories, creatures, props, and language unmistakably grow from Vietnamese folklore.