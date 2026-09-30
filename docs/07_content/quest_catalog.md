# Launch Quest Catalog
status: LOCKED

## Scope
Concrete launch quest roster for the six progression acts, optional folklore side stories, Daily bounty generation, and Spirit Surge event participation.

Quest mechanics remain canonical in `../02_world/quests.md`. Launch MAIN/SIDE each have exactly one folklore mystery beat. No branching power rewards, no rare-drop blockers, no public-boss/slow-respawn wait gate in MAIN, and no mandatory daily checklist.

## Quest-Local Objects
Quest-only world interactions may use IDs under:
```text
quest_object.<quest_id_suffix>.<object_key>
```
They exist only while the owning quest/objective is active and are not persistent inventory items. This avoids creating fake permanent item/NPC catalogs for one-use story props.

# EXP Budget
Act budgets (×100 corrected, `exp_required(L) = 10000·L²`):
| Act | Level band | act_exp_total |
|---|---|---:|
| I | 1-10 | 3,850,000 |
| II | 11-20 | 24,850,000 |
| III | 21-30 | 65,850,000 |
| IV | 31-40 | 126,850,000 |
| V | 41-50 | 207,850,000 |
| VI | 51-60 | 272,850,000 |

Under the seven-channel EXP portfolio (`STORY_ONCE` = 5% of act budget), the MAIN and SIDE quest sub-allocations are:
```text
MAIN quests (all 4)  = 3.0% of act budget
SIDE quests (all 2)  = 0.8% of act budget  ->  each SIDE = 0.4%
first dungeon/finale = 0.4% of act budget  (FIRST_CLEAR in dungeon_catalog.md)
```

Within the 3.0% MAIN allocation, relative weights are preserved at 12:14:16:18 (sum = 60):
```text
MAIN 1 = 12/60 × 3.0% = 0.6% of act budget
MAIN 2 = 14/60 × 3.0% = 0.7% of act budget
MAIN 3 = 16/60 × 3.0% = 0.8% of act budget
MAIN 4 = 18/60 × 3.0% = 0.9% of act budget
```

Repeatable content (FIELD_COMBAT 40%, DUNGEON_REPEAT 18%, WORLD_EVENT 12%, BOUNTY_REPEAT 12%, ELITE_BOSS 8%, LIFE_SKILL 5%) fills the remaining 95% from `progression_route.md`.

## Bounty Set EXP — BOUNTY_REPEAT Channel
Daily bounty sets now grant character EXP via the `BOUNTY_REPEAT` channel (12% of act budget, 240 of the 2,000 target hours). This channel was previously zero.

Formula: `bounty_set_exp(act) = act_exp_total(act) / (target_hours(act) × 1.5)`  (1.5 bounty sets/hour; 3 completions = 1 set; 3-cap/UTC-day unchanged).

| Act | act_exp_total | target_hours | bounty_set_exp |
|---:|---:|---:|---:|
| I | 3,850,000 | 15 | 171,111 |
| II | 24,850,000 | 75 | 220,889 |
| III | 65,850,000 | 260 | 168,846 |
| IV | 126,850,000 | 450 | 188,074 |
| V | 207,850,000 | 550 | 251,939 |
| VI | 272,850,000 | 650 | 279,846 |

Settlement Semantics:
Each daily bounty grants its character EXP settlement immediately upon completion:
- Standard DAILY bounty: `daily_bounty_exp(act) = floor(bounty_set_exp(act) / 3)`
- MYSTERY daily bounty: `mystery_bounty_exp(act) = floor(daily_bounty_exp(act) * 1.15)` plus the tier-specific `currency.bound` grant.
- Reaching the 3-cap: completing the 3rd daily bounty of the cycle additionally awards any integer remainder `remainder = bounty_set_exp(act) - 3 * daily_bounty_exp(act)` (0, 1, or 2 EXP), ensuring the full set total is credited.
- Settlement is per-quest and order-independent: whether the MYSTERY bounty is finished 1st, 2nd, or 3rd, its +15% modifier applies directly to its own completion settlement and commits idempotently under `reward.quest.<quest_id>.<character_id>`. No modifier buffering or cross-quest state machine is required.
# Shared Main-Quest Rules
```text
type = MAIN
repeatability = ONCE
completion_mode = AUTO_COMPLETE unless final objective says TURN_IN
objective_order = SEQUENTIAL
abandon = false
```
Every MAIN/SIDE declares exactly one `mystery_type` + `mystery_owner` per `../02_world/quests.md`. DAILY/EVENT do not. Rewards and EXP shares are unchanged. Wrong lamp/decoy/hazard never fails the quest.

All MAIN reward equipment/material delivery is idempotent and uses Reward Claims if inventory is full.

For a dungeon whose mandatory completion already requires its final boss, the MAIN quest uses the `DUNGEON` objective only. Do not append the same run's final `BOSS` objective after it under sequential ordering; the boss event occurs before dungeon-completion settlement and would otherwise force an unintended second run.

Binary resolution choice (six act-closing MAIN quests, ADR-0061): while the quest is `ACTIVE` and its `branch_flag` is unset, the character sets it once with `C2S_STORY_BRANCH_CHOOSE` (`../05_network/messages.md`); the value is immutable afterwards. Entering that act's dungeon (or the finale) is rejected with `STORY_CHOICE_REQUIRED` only while that act-closing MAIN quest is `ACTIVE` for the entering character and its flag is unset; the client then shows the choice. A character who has not reached the quest, or has already resolved it, enters under the normal dungeon rules (party help, daily objectives, repeats). The flag is presentation/lore only.

Standalone PUBLIC bosses are optional world content and are not MAIN progression gates unless an always-available instanced story equivalent is explicitly defined.

Ordinary field ELITE respawn groups are also not MAIN gates. If future MAIN content needs a named elite fight, it must use an always-available quest-owned encounter or dungeon encounter rather than waiting for the shared `35..60s` field respawn.

# ACT I — Làng Đa
Regional material: `item.material.lang_da.manh_dong`

## `quest.main.a1.duong_vao_lang` — Đường Vào Làng
Prerequisite: character Level 1.
`quest_giver = npc.lang_da.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FORBIDDEN_GROUND`. `mystery_owner = quest`.
Objectives:
1. `REACH` `map.lang_da.dinh_lang`
2. `REACH` `map.lang_da.bo_ruong`
3. `INTERACT` `quest_object.a1_duong_vao_lang.moc_tre`
4. `KILL` any 4 of `monster.lang_da.bu_nhin_rom|monster.lang_da.dom_dom_ma`

Jump the marked earth patch `quest_hazard.a1_duong_vao_lang.vet_cam` to reach the bamboo marker. Hazard does not fail the quest. INTERACT `moc_tre` is one `PHAT_HIEN` (`source=QUEST_CLUE`); no extra EXP/currency; retry cannot duplicate the peak.

Rewards:
```text
EXP 23,100
common 300
2 item.material.lang_da.manh_dong
progression.story.a1.01 = true
```

## `quest.main.a1.ben_da_lanh` — Bến Đa Lạnh
Prerequisite: `progression.story.a1.01`.
`quest_giver = npc.lang_da.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FALSE_TRAIL`. `mystery_owner = quest`.
Objectives:
1. `REACH` `map.lang_da.ben_da`
2. `INTERACT` `quest_object.a1_ben_da_lanh.dau_thuyen_cu`
3. `KILL` 3 `monster.lang_da.coc_thanh_tinh`

Authentic mooring post `dau_thuyen_cu`. Decoy post `dau_thuyen_gia` on a second pier plank. Static sprites only. Cóc are aftermath.

Rewards:
```text
EXP 26,950
common 400
3 item.material.lang_da.manh_dong
progression.story.a1.02 = true
```

## `quest.main.a1.go_ma_thuc_giac` — Gò Mả Thức Giấc
Prerequisite: `progression.story.a1.02`.
`quest_giver = npc.lang_da.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FORBIDDEN_GROUND`. `mystery_owner = quest`.
Objectives:
1. `REACH` `map.lang_da.go_ma`
2. `INTERACT` `quest_object.a1_go_ma_thuc_giac.den_dat`
3. `KILL` 4 `monster.lang_da.vong_hon`

Hazard volumes `quest_hazard.a1_go_ma.ma_xo_01` and `.02` are static marked ground (decal + chip). No extra spawn, no particle flood. `den_dat` is only reachable on the unmarked path.

Rewards:
```text
EXP 30,800
common 500
4 item.material.lang_da.manh_dong
progression.story.a1.03 = true
```

## `quest.main.a1.dinh_lang_bo_hoang` — Đêm Ở Đình Bỏ Hoang
Prerequisite: `progression.story.a1.03`.
`quest_giver = npc.lang_da.nguoi_dan_duong`. `completion_mode = TURN_IN` (return to `npc.lang_da.nguoi_dan_duong`).
`mystery_type = LIGHT_ORDER`. `mystery_owner = dungeon.dinh_lang_bo_hoang` (relight courtyard lamps).
Objective: `DUNGEON` complete `dungeon.dinh_lang_bo_hoang`.

Binary resolution — choose before entering the dungeon:
```text
branch_flag = progression.story.a1.resolution
options = appease | banish
appease:  ambient NPC dialogue in lang_da shows gratitude variant;
          di_tich.lang_da relic flavour = "The spirit was appeased."
          Atlas di_tich page variant = calm-spirit ink wash illustration (cosmetic/lore only)
banish:   ambient NPC dialogue in lang_da shows relief variant;
          di_tich.lang_da relic flavour = "The spirit was banished."
          Atlas di_tich page variant = exorcism-seal ink wash illustration (cosmetic/lore only)
No stat difference between branches.
```

Rewards:
```text
EXP 34,650
common 800
6 item.material.lang_da.manh_dong
progression.story.a1.complete = true
beast_grant starter (class Tương Sinh, spirit_beasts.md)
```
Dungeon first-clear equipment remains owned by `drop_tables.md` and is not duplicated here. Beast grant key `beast.grant.starter.<character_id>` is independent of inventory slots.

# ACT II — Rừng U Minh
Material: `item.material.u_minh.vo_cay`
Prerequisite for first quest: `progression.story.a1.complete` and Level 11.

## `quest.main.a2.loi_vao_u_minh` — Lối Vào U Minh
`quest_giver = npc.rung_u_minh.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = HEIGHT_BAND`. `mystery_owner = quest`.
Objectives: `REACH map.rung_u_minh.xom_rung`; `REACH map.rung_u_minh.loi_tram`; double-jump perch `quest_object.a2_loi_vao_u_minh.mieng_rung` on the banyan limb; `KILL` 3 `monster.rung_u_minh.ma_rung`.
Main-path volume does not complete. Static sprite.
Rewards: `EXP 149,100`, common `800`, material `2`, flag `progression.story.a2.01`.

## `quest.main.a2.dom_lua_dan_duong` — Đốm Lửa Dẫn Đường
Prerequisite: a2.01.
`quest_giver = npc.rung_u_minh.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FALSE_TRAIL`. `mystery_owner = quest`.
Objectives: `INTERACT quest_object.a2_dom_lua_dan_duong.moc_that`; `REACH map.rung_u_minh.rung_sau`; `KILL` 3 `monster.rung_u_minh.dom_lua`.
Authentic: `moc_that`. Decoys: `moc_duong_01`, `moc_duong_02` (lure lights; interact does not progress).
Rewards: `EXP 173,950`, common `1000`, material `3`, flag `progression.story.a2.02`.

## `quest.main.a2.re_da_quan_mieu` — Rễ Đa Quấn Miếu
Prerequisite: a2.02.
`quest_giver = npc.rung_u_minh.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FORBIDDEN_GROUND`. `mystery_owner = quest`.
Objectives: `REACH map.rung_u_minh.mieu_bo_hoang`; `INTERACT quest_object.a2_re_da_quan_mieu.re_chan_that`; `KILL` 4 `monster.rung_u_minh.tinh_cay`.
Hazard volumes `quest_hazard.a2_re_da.re_cam_01..02` are static marked ground. No extra spawn, no particle flood.
Rewards: `EXP 198,800`, common `1200`, material `4`, flag `progression.story.a2.03`.

## `quest.main.a2.mieu_ba_trong_rung` — Miếu Bà Trong Rừng
Prerequisite: a2.03.
`quest_giver = npc.rung_u_minh.nguoi_dan_duong`. `completion_mode = TURN_IN` (return to `npc.rung_u_minh.nguoi_dan_duong`).
`mystery_type = FALSE_TRAIL`. `mystery_owner = dungeon.mieu_ba_trong_rung` (lost-path trail markers).
Objective: `DUNGEON` complete `dungeon.mieu_ba_trong_rung`.

Binary resolution — choose before entering the dungeon:
```text
branch_flag = progression.story.a2.resolution
options = appease | banish
appease:  ambient NPC dialogue in rung_u_minh shows offering-accepted variant;
          di_tich.rung_u_minh relic flavour = "Miếu Bà accepted the offering."
          Atlas di_tich page variant = offerings-at-shrine illustration (cosmetic/lore only)
banish:   ambient NPC dialogue in rung_u_minh shows clearing variant;
          di_tich.rung_u_minh relic flavour = "The shrine's hold was broken."
          Atlas di_tich page variant = broken-root seal illustration (cosmetic/lore only)
No stat difference between branches.
```

Rewards: `EXP 223,650`, common `1800`, material `6`, flag `progression.story.a2.complete`.

# ACT III — Bến Nước Đen
Material: `item.material.ben_nuoc.da_song`
Prerequisite first quest: a2.complete and Level 21.

## `quest.main.a3.bai_lau_co_tieng_goi` — Bãi Lau Có Tiếng Gọi
`quest_giver = npc.ben_nuoc_den.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FALSE_TRAIL`. `mystery_owner = quest`.
Objectives: `REACH map.ben_nuoc_den.bai_lau`; `INTERACT quest_object.a3_bai_lau_co_tieng_goi.coc_ben`; `KILL` 3 `monster.ben_nuoc_den.ma_da`.
Authentic post `coc_ben`. Decoy reed clump `coc_gia` on another platform. Static sprites.
Rewards: `EXP 395,100`, common `1800`, material `2`, flag `progression.story.a3.01`.

## `quest.main.a3.duong_ngap` — Đường Ngập
Prerequisite: a3.01.
`quest_giver = npc.ben_nuoc_den.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = DROP_THROUGH`. `mystery_owner = quest`.
Objectives: `REACH map.ben_nuoc_den.duong_ngap`; drop-through one-way walkway `platform.one_way.a3_duong_ngap.van_mat`; `REACH quest_object.a3_duong_ngap.bac_thang`; `KILL` 3 `monster.ben_nuoc_den.ca_tinh`.
Walking the solid-looking top does not complete. Painted water, no animated flood.
Rewards: `EXP 460,950`, common `2200`, material `3`, flag `progression.story.a3.02`.

## `quest.main.a3.ben_do_cu` — Bến Đò Cũ
Prerequisite: a3.02.
`quest_giver = npc.ben_nuoc_den.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = LIGHT_ORDER`. `mystery_owner = quest`.
Objectives: `REACH map.ben_nuoc_den.ben_do_cu`; hit/INTERACT `quest_object.a3_ben_do_cu.den_01` then `den_03` left-to-right while moving along the pier; `KILL` 3 `monster.ben_nuoc_den.ma_da`.
`den_02` is a decoy on the same lane; hitting it resets. No stand-still memory, no "leave one off".
Rewards: `EXP 526,800`, common `2600`, material `4`, flag `progression.story.a3.03`.

## `quest.main.a3.xom_chim` — Xóm Chìm
Prerequisite: a3.03.
`quest_giver = npc.ben_nuoc_den.nguoi_dan_duong`. `completion_mode = TURN_IN` (return to `npc.ben_nuoc_den.nguoi_dan_duong`).
`mystery_type = WATER_GATE`. `mystery_owner = dungeon.xom_chim` (sluice gates).
Objective: `DUNGEON` complete `dungeon.xom_chim`.

Binary resolution — choose before entering the dungeon:
```text
branch_flag = progression.story.a3.resolution
options = appease | banish
appease:  ambient NPC dialogue in ben_nuoc_den shows the river-spirit-calmed variant;
          di_tich.ben_nuoc_den relic flavour = "The drowned village found rest."
          Atlas di_tich page variant = submerged-lantern mourning scene (cosmetic/lore only)
banish:   ambient NPC dialogue in ben_nuoc_den shows the waters-cleared variant;
          di_tich.ben_nuoc_den relic flavour = "The spirit's grip was severed."
          Atlas di_tich page variant = broken-chain waterway illustration (cosmetic/lore only)
No stat difference between branches.
```

Rewards: `EXP 592,650`, common `3500`, material `6`, flag `progression.story.a3.complete`.

# ACT IV — Đèo Mây
Material: `item.material.deo_may.da_voi`
Prerequisite first quest: a3.complete and Level 31.

## `quest.main.a4.duong_rung` — Đường Rừng
`quest_giver = npc.deo_may.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = DROP_THROUGH`. `mystery_owner = quest`.
Objectives: `REACH map.deo_may.duong_rung`; drop-through one-way planks `platform.one_way.a4_duong_rung.van_mat`; `REACH map.deo_may.khe_da`.
Rewards: `EXP 761,100`, common `3500`, material `2`, flag `progression.story.a4.01`.

## `quest.main.a4.vet_chan_ho` — Vết Chân Hổ
Prerequisite: a4.01.
`quest_giver = npc.deo_may.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FALSE_TRAIL`. `mystery_owner = quest`.
Objectives: `INTERACT quest_object.a4_vet_chan_ho.dau_chan_that`; `REACH map.deo_may.rung_cam`; `KILL` 3 `monster.deo_may.ho_con_tinh`.
Authentic print `dau_chan_that`. Decoy platforms `dau_chan_gia_01..02` do not progress. No carry-item fetch.
Rewards: `EXP 887,950`, common `4500`, material `3`, flag `progression.story.a4.02`.

## `quest.main.a4.loi_sai` — Lối Sai
Prerequisite: a4.02.
`quest_giver = npc.deo_may.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = HEIGHT_BAND`. `mystery_owner = quest`.
Objectives: double-jump perch `INTERACT quest_object.a4_loi_sai.dau_duong_that`; `KILL` 3 `monster.deo_may.ma_tranh`.
Main path does not complete. Static sprite.
Rewards: `EXP 1,014,800`, common `5500`, material `4`, flag `progression.story.a4.03`.

## `quest.main.a4.hang_ma_tranh` — Hang Ma Trành
Prerequisite: a4.03.
`quest_giver = npc.deo_may.nguoi_dan_duong`. `completion_mode = TURN_IN` (return to `npc.deo_may.nguoi_dan_duong`).
`mystery_type = FALSE_TRAIL`. `mystery_owner = dungeon.hang_ma_tranh` (trail charms).
Objective: `DUNGEON` complete `dungeon.hang_ma_tranh`.

Binary resolution — choose before entering the dungeon:
```text
branch_flag = progression.story.a4.resolution
options = appease | banish
appease:  ambient NPC dialogue in deo_may shows mountain-spirit-pacified variant;
          di_tich.deo_may relic flavour = "The pass-spirit accepted the pact."
          Atlas di_tich page variant = tiger-spirit pact calligraphy (cosmetic/lore only)
banish:   ambient NPC dialogue in deo_may shows safe-roads variant;
          di_tich.deo_may relic flavour = "The haunting of Hang Ma Trành was ended."
          Atlas di_tich page variant = broken-trail-charm ruin illustration (cosmetic/lore only)
No stat difference between branches.
```

Rewards: `EXP 1,141,650`, common `7000`, material `6`, flag `progression.story.a4.complete`.

# ACT V — Thành Cổ
Material: `item.material.thanh_co.gach_co`
Prerequisite first quest: a4.complete and Level 41.

## `quest.main.a5.cong_ngoai` — Cổng Ngoài
`quest_giver = npc.thanh_co.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = LIGHT_ORDER`. `mystery_owner = quest`.
Objectives: `REACH map.thanh_co.cong_ngoai`; hit/INTERACT `quest_object.a5_cong_ngoai.den_trai` then `den_phai` left-to-right on the gate lane; `REACH map.thanh_co.duong_da`.
Rewards: `EXP 1,247,100`, common `6500`, material `2`, flag `progression.story.a5.01`.

## `quest.main.a5.hao_can` — Hào Cạn
Prerequisite: a5.01.
`quest_giver = npc.thanh_co.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = FORBIDDEN_GROUND`. `mystery_owner = quest`.
Objectives: `REACH map.thanh_co.hao_can`; `REACH quest_object.a5_hao_can.doi_canh`; `KILL` 3 `monster.thanh_co.hon_binh`.
Hazard volumes `quest_hazard.a5_hao_can.vet_ma_01..02` do not fail the quest.
Rewards: `EXP 1,454,950`, common `8000`, material `3`, flag `progression.story.a5.02`.

## `quest.main.a5.tieng_trong_tran` — Tiếng Trống Trấn
Prerequisite: a5.02.
`quest_giver = npc.thanh_co.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = LIGHT_ORDER`. `mystery_owner = quest`.
Objectives: `REACH map.thanh_co.den_tran`; hit/INTERACT `quest_object.a5_tieng_trong_tran.trong_trai` then `trong_phai` left-to-right on the drum lane; `KILL` 3 `monster.thanh_co.hon_binh`.
Idle drum sprites only; no pulse particle loop.
Rewards: `EXP 1,662,800`, common `9500`, material `4`, flag `progression.story.a5.03`.

## `quest.main.a5.den_tran` — Đền Trấn
Prerequisite: a5.03.
`quest_giver = npc.thanh_co.nguoi_dan_duong`. `completion_mode = TURN_IN` (return to `npc.thanh_co.nguoi_dan_duong`).
`mystery_type = LIGHT_ORDER`. `mystery_owner = dungeon.den_tran` (three seal drums).
Objective: `DUNGEON` complete `dungeon.den_tran`.

Binary resolution — choose before entering the dungeon. The moral question here is preservation versus cleansing: should the fortress's supernatural covenant be maintained as a living heritage, or severed and left to settle as ruin?
```text
branch_flag = progression.story.a5.resolution
options = seal | cleanse
seal:    ambient NPC dialogue in thanh_co shows covenant-preserved variant;
         di_tich.thanh_co relic flavour = "The pact of Thành Cổ was sealed back into the stone."
         Atlas di_tich page variant = nine-tail covenant seal restored calligraphy (cosmetic/lore only)
         On return visit: spectral lanterns flicker at the fortress gate arches (visual prop state change, cosmetic only)
cleanse: ambient NPC dialogue in thanh_co shows ruins-quieted variant;
         di_tich.thanh_co relic flavour = "The old citadel's bond was dissolved. The stones remember nothing."
         Atlas di_tich page variant = cracked-gate rubble illustration (cosmetic/lore only)
         On return visit: additional rubble/crack decal layer appears at the fortress gate (visual prop state change, cosmetic only)
No stat difference between branches.
```

Rewards: `EXP 1,870,650`, common `12000`, material `6`, flag `progression.story.a5.complete`.

# ACT VI — Núi Thiêng
Material: `item.material.nui_thieng.da_suong`
Prerequisite first quest: a5.complete and Level 51.

## `quest.main.a6.rung_may` — Rừng Mây
`quest_giver = npc.nui_thieng.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = TESTIMONY`. `mystery_owner = quest`.
Objectives:
1. `REACH map.nui_thieng.rung_may`
2. `TALK` any 3 of `talk_pool = [npc.nui_thieng.cho_1, npc.nui_thieng.cho_2, npc.nui_thieng.dem_1, npc.nui_thieng.dem_2, npc.nui_thieng.nguoi_dan_duong]`
3. `INTERACT` `implicated_id = quest_object.a6_rung_may.da_ranh_that` (the true boundary-stone marker)

Wrong markers `quest_object.a6_rung_may.da_gia_01..02` reset without failing. NPC dialogue pool: cho_1 references drowned-village omens seen at Bến Nước Đen returning to the mountain (explicit callback to Act III resolution); cho_2 describes the fortress spirits of Thành Cổ having moved north; dem_1 recalls tiger-trail signs matching Đèo Mây; dem_2 mentions strange lights identical to Rừng U Minh; nguoi_dan_duong names all five prior di_tich as one converging pattern. The true boundary stone bears faint marks of all five regional omens.

Aftermath: `REACH map.nui_thieng.suon_da`.

Rewards: `EXP 1,637,100`, common `10000`, material `2`, flag `progression.story.a6.01`.

## `quest.main.a6.da_ranh` — Đá Ranh
Prerequisite: a6.01.
`quest_giver = npc.nui_thieng.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = HEIGHT_BAND`. `mystery_owner = quest`.
Objectives: `INTERACT quest_object.a6_da_ranh.da_goc` on the cliff-lip perch; `REACH map.nui_thieng.cong_co`.
Main path does not complete. Static sprite.
Rewards: `EXP 1,909,950`, common `13000`, material `3`, flag `progression.story.a6.02`.

## `quest.main.a6.dau_cu` — Dấu Cũ
Prerequisite: a6.02.
`quest_giver = npc.nui_thieng.nguoi_dan_duong`. `completion_mode = AUTO_COMPLETE`.
`mystery_type = LIGHT_ORDER`. `mystery_owner = quest`.

Objectives:
1. `REACH map.nui_thieng.cong_co`
2. hit/INTERACT `quest_object.a6_dau_cu.da_01` then `da_02` then `da_03` in left-to-right authored order along the ritual path

Three boundary stones must be activated in sequence while moving forward along the ritual path. `da_gia` decoy stone resets the sequence without failing. No standing still required; no particle telegraph. The three-stone sequence is harder than the two-lamp sequences in Acts III and V — the authoring space is longer, the decoy placement is tighter, and the third stone is at the path's highest elevation requiring the HEIGHT_BAND from a6.da_ranh to have been practiced.

Aftermath: `KILL` 3 `monster.nui_thieng.ma_nui`.

Rewards: `EXP 2,182,800`, common `16000`, material `4`, flag `progression.story.a6.03`.

`boss.ngu_tinh` remains an optional PUBLIC world boss. MAIN progression never waits for its `30m..45m` generation cooldown.

## `quest.main.a6.than_trung` — Canh Cuối
Prerequisite: a6.03.
`quest_giver = npc.nui_thieng.nguoi_dan_duong`. `completion_mode = TURN_IN` (return to `npc.nui_thieng.nguoi_dan_duong`).
`mystery_type = FORBIDDEN_GROUND`. `mystery_owner = boss.than_trung` (safe lanes in the finale).
Objective: `BOSS boss.than_trung`.

Binary resolution — choose before engaging the boss. The moral question here is not appeasement or destruction but recognition: should the pattern of misfortune be acknowledged as part of the world's living memory, or severed so that it cannot return?
```text
branch_flag = progression.story.a6.resolution
options = remember | sever
remember: ambient NPC dialogue in nui_thieng shows pattern-acknowledged variant;
          di_tich.nui_thieng relic flavour = "Thần Trùng was named and witnessed. The pattern is part of the world."
          Atlas di_tich page variant = boundary-stones glowing warmly, all five regional omen marks visible on the central stone (cosmetic/lore only)
          On return visit: boundary stones along cong_co emit a soft-light aura; NPC dialogue notes the omens are still present but no longer hidden (cosmetic only)
sever:    ambient NPC dialogue in nui_thieng shows chain-broken variant;
          di_tich.nui_thieng relic flavour = "The chain of misfortune was cut. The mountain is quieter now — and emptier."
          Atlas di_tich page variant = cracked boundary stones, dark and still (cosmetic/lore only)
          On return visit: boundary stones along cong_co are visibly cracked; NPC dialogue notes the omens stopped but something in the mountain's presence is absent (cosmetic only)
No stat difference between branches.
```

Rewards: `EXP 2,455,650`, common `20000`, material `8`, flag `progression.story.main.complete`.

The final boss first-clear Soul is granted by `drop_tables.md`, not duplicated by the quest. The character-scoped first-story-completion `currency.special` grant is owned by `economy_catalog.md` and is likewise not duplicated in the character quest bundle.

# Optional SIDE Quests — 12
Each SIDE quest is ONCE, abandonable, does not gate MAIN progression, and receives the deterministic reward bundle for its owning act below.

Each SIDE quest EXP = 0.4% of act budget (STORY_ONCE sub-split; 2 SIDE quests per act × 0.4% = 0.8% total SIDE per act).

| Act/Tier | EXP = 0.4% act budget | common | regional material | bound side grant |
|---|---:|---:|---:|---:|
| I / T1 | 15,400 | 300 | 2 | 10 |
| II / T2 | 99,400 | 700 | 2 | 20 |
| III / T3 | 263,400 | 1,400 | 2 | 30 |
| IV / T4 | 507,400 | 2,500 | 2 | 40 |
| V / T5 | 831,400 | 4,000 | 2 | 50 |
| VI / T6 | 1,091,400 | 6,000 | 2 | 60 |

The common amount is owned here as quest pacing data. The bound amount is numerically owned by `economy_catalog.md` as `10 * owning_tier`; this table is a compiled reward preview and must match it.

Each completion uses one idempotent quest settlement with independent stable slots for character EXP, common, material, bound, and any account cosmetic entitlement trigger. A capped currency/material delivery issue cannot duplicate another slot.

SIDE `mystery_owner = quest`. Rewards use no unique power item and no rare random objective. `quest.side.a1.chiec_non_ben_da` additionally grants one starter `item.tool.can_cau_tre` and 5 `item.consumable.moi_cau` to introduce the village fishing and cooking loop.

| quest_id | Display | mystery_type | quest_giver | Objectives |
|---|---|---|---|---|
| `quest.side.a1.chiec_non_ben_da` | Chiếc Nón Ở Bến Đa | FALSE_TRAIL | `npc.lang_da.cho_1` | REACH Bến Đa; INTERACT `ghe_cu`. Decoy boat `ghe_gia`. Aftermath: KILL 2 vong_hon. Introduces Folk Fishing with starter rod + bait. |
| `quest.side.a1.luy_tre_keu_dem` | Lũy Tre Kêu Đêm | TESTIMONY | `npc.lang_da.cho_2` | TALK any 3 of `talk_pool = [npc.lang_da.cho_1, npc.lang_da.cho_2, npc.lang_da.dem_1, npc.lang_da.dem_2, npc.lang_da.nguoi_dan_duong]`; INTERACT `implicated_id = quest_object.a1_luy_tre.huong_an_that` (the true incense altar). Wrong objects `quest_object.a1_luy_tre.cot_tre_gia_*` reset without failing. Aftermath: KILL 3 bu_nhin_rom. |
| `quest.side.a2.nguoi_di_rung_muon` | Người Đi Rừng Muộn | FALSE_TRAIL | `npc.rung_u_minh.cho_1` | INTERACT `moc_that`; decoys `moc_gia_01..03`. Aftermath: KILL 1 ma_tranh. |
| `quest.side.a2.goc_da_co` | Gốc Đa Cũ | HEIGHT_BAND | `npc.rung_u_minh.cho_2` | Double-jump `moc_goc` on the banyan limb. Main path does not complete. |
| `quest.side.a3.den_ben_do` | Đèn Bến Đò | LIGHT_ORDER | `npc.ben_nuoc_den.cho_1` | Hit/INTERACT `den_a` then `den_c` left-to-right while running the pier. `den_b` decoy resets. Aftermath: KILL 3 ma_da. |
| `quest.side.a3.tieng_go_tren_mai` | Tiếng Gõ Trên Mái | DROP_THROUGH | `npc.ben_nuoc_den.dem_1` | Drop-through one-way roof `platform.one_way.a3_mai.van_mat`; REACH under-roof `mai_go`. Aftermath: KILL 3 bong_nuoc_ma. |
| `quest.side.a4.chiec_mong_sat` | Chiếc Móng Sắt | FALSE_TRAIL | `npc.deo_may.cho_1` | INTERACT `dau_chan_that`; decoy platforms `dau_gia_*`. Aftermath: KILL 3 ho_con_tinh. |
| `quest.side.a4.cau_tre_gay` | Cầu Tre Gãy | DROP_THROUGH | `npc.deo_may.cho_2` | Drop-through one-way `platform.one_way.a4_cau.van_mat`; REACH far bank. Walking the top does not complete. Aftermath: KILL 3 khi_nui. |
| `quest.side.a5.vien_gach_co` | Viên Gạch Cổ | LIGHT_ORDER | `npc.thanh_co.cho_1` | Hit/INTERACT wall marks `gach_1` then `gach_2` left-to-right on the brick lane. Aftermath: KILL 3 ma_co. |
| `quest.side.a5.tieng_buoc_tren_tuong` | Tiếng Bước Trên Tường | TESTIMONY | `npc.thanh_co.cho_2` | TALK any 3 of `talk_pool = [npc.thanh_co.cho_1, npc.thanh_co.cho_2, npc.thanh_co.dem_1, npc.thanh_co.dem_2, npc.thanh_co.nguoi_dan_duong]`; INTERACT `implicated_id = quest_object.a5_tuong.bia_da_that` (the inscribed watch-post slab). Wrong objects `quest_object.a5_tuong.gach_gia_*` reset without failing. Aftermath: KILL 3 hon_binh. |
| `quest.side.a6.duong_may_nguoc` | Đường Mây Ngược | FALSE_TRAIL | `npc.nui_thieng.cho_1` | INTERACT `moc_nguoc_that`; decoys `moc_may_gia_*`. Aftermath: KILL 3 vong_linh. |
| `quest.side.a6.vet_tren_da_suong` | Vết Trên Đá Sương | FALSE_TRAIL | `npc.nui_thieng.dem_1` | INTERACT `vet_that`; decoy stones `vet_gia_*`. Aftermath: KILL 3 tinh_thu. |

Quest-local IDs for SIDE use `quest_object.<quest_id_suffix>.*` as in the MAIN section. Decoy/hazard objects are quest-local and exist only while that SIDE is ACTIVE.

# DAILY Bounty Template Pool — 12 standard + 1 mystery meta
Daily board generation selects six currently accessible templates with concrete target substitution from the character's unlocked act. Slot 6 is always the `daily.mystery` MYSTERY meta-template; slots 1–5 are drawn from the 12 standard objective templates. Maximum three completions per reset remains canonical.

Each generated quest ID is:
```text
quest.daily.<utc_date>.<character_id>.<slot_1_to_6>
```
with persisted template ID and resolved targets so rerolls/reconnects cannot change it.

Templates:
| template_id | Objective family | Requirement | Reward |
|---|---|---|---|
| `daily.hunt_small` | KILL | 8 NORMAL in one unlocked field | common + 2 regional material |
| `daily.hunt_varied` | KILL | 4 NORMAL each from 2 families | common + 2 regional material |
| `daily.elite_watch` | KILL | 1 eligible ELITE | common + 3 regional material |
| `daily.dungeon_path` | DUNGEON | complete one accessible NORMAL dungeon | common + 3 regional material |
| `daily.field_route` | REACH | visit 3 authored points in one region | common + 2 regional material |
| `daily.old_marks` | INTERACT | inspect 4 non-random field markers | common + 2 regional material |
| `daily.river_or_trail` | REACH | cross two field-map endpoints | common + 2 regional material |
| `daily.spirit_cleanup` | KILL | defeat 6 supernatural-tag NORMAL enemies | common + 2 regional material |
| `daily.guardian` | KILL | defeat 2 eligible ELITEs in any unlocked region | common + 4 regional material |
| `daily.dungeon_help` | DUNGEON | complete one dungeon with any eligible party size | common + 3 regional material |
| `daily.explore_quiet` | REACH | enter one optional side area and return | common + 2 regional material |
| `daily.surge_if_active` | EVENT | contribute to one active Spirit Surge; only generated when schedule/access permits | common + event completion reward only |
| `daily.mystery` | MYSTERY | objective, area, and reward hidden (`???`) until revealed by reaching the area or interacting with starter NPC; drawn from the same pool as above with a `+15% EXP` bonus; grants `currency.bound` in addition to the standard common/material reward | `common × 1.15 (rounded)` + `bound` (see tier table below) + regional material |

The `daily.mystery` template is always the 6th slot (the one MYSTERY slot per board). It is flagged `mystery: true`; server assigns one of the 12 standard objective templates with variable-ratio weighting but withholds title/objective/reward text until reveal. The bound grant and +15% EXP modifier are applied at reveal/settlement, not at board generation.

Daily `currency.bound` grant from MYSTERY bounty by current tier:
```text
T1  5
T2  10
T3  15
T4  20
T5  25
T6  30
```

Daily common reward by current tier:
```text
T1 300
T2 700
T3 1400
T4 2500
T5 4000
T6 6000
```
No Daily template grants exclusive equipment, Soul, skill point, potential point, or permanent stat.

Daily payouts are optional accelerators and are not included in the baseline affordability assumptions in `economy_catalog.md`.

Board constraints:
- slot 6 is always `daily.mystery`; its underlying objective is drawn from the non-MYSTERY pool
- no more than 2 templates from the same objective family (the MYSTERY slot's underlying family counts toward this limit), except the slot-6 fallback in § Board Generation
- do not generate `daily.surge_if_active` unless an eligible Surge can be entered during the current board resolution window
- do not target a dungeon/field the character has not unlocked
- objective counts never require rare random drops
- MYSTERY template always grants `currency.bound` at settlement; this makes DAILY a valid `currency.bound` source

## Board Generation (ADR-0061)
The board is generated on the character's first board request of the UTC day and persisted; generation is a pure function of the inputs below.
```text
digest   = SHA-256("thinhthan.daily.v1|" || character_id (16 raw bytes) || utc_date "YYYY-MM-DD")
rng      = math/rand/v2 PCG(seed1 = digest[0:8], seed2 = digest[8:16]) (big-endian uint64)
draw(W)  = r = rng.Uint64N(sum(W)); first entry, in table order, whose cumulative weight > r
target region R = region of the character's highest unlocked act (tier = that act)
eligible = standard templates whose target resolves (DUNGEON: >= 1 unlocked dungeon; surge: rule above)
candidates(capped) = eligible templates not yet on the board and, when capped, whose family has < 2 on the board
slots 1..5: each slot = draw(standard weights of candidates(capped = true)); one rng call per slot, no rejection loop
slot 6:     draw(mystery weights of candidates(capped = true)); if that set is empty, draw(mystery weights of
            candidates(capped = false)) — the only case where a 3rd template of one family is allowed
targets:    resolved slot 1..6 in order with the same rng, using the rules below
```
Eligibility always contains the 9 non-DUNGEON, non-EVENT templates (every region has ELITEs, markers and route points), so every slot has a non-empty candidate set. Below Level 8 (no dungeon) and without an active Surge the capped set is exhausted after slot 5 (KILL 2 + REACH 2 + INTERACT 1) and slot 6 uses the uncapped draw.

Golden vectors (template selection only; `character_id` given as UUID, hashed as its 16 raw bytes):
```text
case A  character_id 01920000-0000-7000-8000-000000000001, utc_date 2026-10-01, no unlocked dungeon, no Surge
        digest[0:16] 4f7a4430e2b9f1fb2cf6d902d7312f32   slot 6 uncapped = true
        1 daily.old_marks  2 daily.spirit_cleanup  3 daily.elite_watch  4 daily.field_route
        5 daily.river_or_trail  6 daily.guardian
case B  character_id 01920000-0000-7000-8000-000000000002, utc_date 2026-10-01, >= 1 unlocked dungeon, Surge eligible
        digest[0:16] c07c3ffea4db31dd4cafff80315df897   slot 6 uncapped = false
        1 daily.spirit_cleanup  2 daily.field_route  3 daily.dungeon_path  4 daily.surge_if_active
        5 daily.explore_quiet  6 daily.elite_watch
```
| template_id | standard weight | mystery weight |
|---|---:|---:|
| `daily.hunt_small` | 12 | 6 |
| `daily.hunt_varied` | 10 | 8 |
| `daily.elite_watch` | 8 | 12 |
| `daily.dungeon_path` | 8 | 10 |
| `daily.field_route` | 10 | 8 |
| `daily.old_marks` | 10 | 8 |
| `daily.river_or_trail` | 8 | 8 |
| `daily.spirit_cleanup` | 10 | 10 |
| `daily.guardian` | 6 | 12 |
| `daily.dungeon_help` | 6 | 10 |
| `daily.explore_quiet` | 8 | 8 |
| `daily.surge_if_active` | 4 | 6 |

Target resolution (region `R`, its three FIELD maps in `world_route_catalog.md` order; "draw a map" = uniform over them):
```text
hunt_small      draw a map; any 8 NORMAL kills on it
hunt_varied     monster family = combat_profile column of monster_catalog.md; draw 2 distinct families present
                among NORMAL monsters of R's ALWAYS spawn pools; 4 kills of each family anywhere in R
elite_watch     1 kill of any ELITE of R
dungeon_path    draw one unlocked dungeon (uniform); complete it
field_route     for each of R's 3 fields draw p1|p2; reach the 3 points (any order)
old_marks       draw 4 of R's 6 field markers without replacement; interact with each
river_or_trail  draw a map; reach its p1 and p2 (either order)
spirit_cleanup  supernatural = monster element NONE; 6 kills of NORMAL element-NONE monsters in R
guardian        2 ELITE kills in any unlocked region
dungeon_help    any unlocked dungeon completion, any party size
explore_quiet   draw a map; enter its side area, then reach its p1 or p2
surge_if_active 1 eligible contribution to any active Spirit Surge
```
Daily objective anchors — every FIELD map provides (`<map_key>` = `map_id` without `map.`; static activation fails when any is missing, overlaps a portal/checkpoint safety radius, or lies outside the walkable geometry):
```text
anchor.daily.<map_key>.p1      west-end route point (REACH radius 3m)
anchor.daily.<map_key>.p2      east-end route point (REACH radius 3m)
marker.daily.<map_key>.1|2     inspectable old marker (C2S_INTERACT target, range 2m, not random, no cooldown)
area.daily.<map_key>.side      optional side-area volume; "enter" = character position inside the volume
```
Totals: 36 route points, 36 markers, 18 side areas. Validation also rejects a region whose ALWAYS NORMAL pools contain fewer than 2 combat profiles or no element-NONE NORMAL.

# Spirit Surge Event Quest
`quest.event.spirit_surge.contribute`

Availability derives from active `SPIRIT_SURGE` only.
Objectives:
1. contribute to configured event enemies/objectives above participation threshold
2. complete the local elite event chain

Reward is entirely delegated to:
```text
drop.event.spirit_surge.<tier>.completion
```
and the daily-first table where eligible. The quest itself adds no extra permanent-power reward.

# Bonus Books Note
Bonus progression books (`item.book.potential` +10, `item.book.skill` +1) are **not** direct skill/potential grants and therefore do not violate the `direct skill/potential reward` validation rule. They are level-milestone item grants via `progression.book.<type>.<level>` flags (Lv25,30,35,40 +1 each; Lv45,50,55,60 +2 each; total 12 each by 60) and are consumed as CHARACTER_BOUND consumables per `../01_gameplay/progression.md` and `item_catalog.md`. No quest grants a book; the point is granted only after item consumption.

# Validation
Static validation rejects:
- MAIN prerequisite cycle,
- quest target map/monster/boss/dungeon absent from content catalogs,
- sequential `DUNGEON` then final `BOSS` objective for the same mandatory dungeon run,
- standalone PUBLIC boss as a required MAIN objective without an always-available instanced equivalent,
- ordinary shared-spawn ELITE as a required MAIN objective without a quest-owned guaranteed encounter,
- reward material not matching the quest's region/tier,
- SIDE EXP/common/material reward not matching the deterministic owning-act row,
- SIDE bound preview differing from `economy_catalog.md` `10 * owning_tier`,
- direct skill/potential reward (bonus books are items, not direct points, so they are allowed),
- Daily board with >2 same primary objective family, except the single 3rd template placed by the slot-6 uncapped fallback of § Board Generation,
- Daily target outside character access,
- quest-local object referenced by another unrelated quest,
- duplicate completion idempotency key,
- MAIN or SIDE missing exactly one `mystery_type` and `mystery_owner`,
- DAILY or EVENT declaring a mystery beat,
- `mystery_owner` dungeon/boss ID that does not resolve,
- `mystery_owner = quest` with zero INTERACT/REACH/LIGHT/DROP/HEIGHT payload,
- `TIMING_WINDOW` or field `WATER_GATE` (`mystery_owner = quest`),
- `EVIDENCE` fetch or `TIMING_WINDOW` loop mystery beat (LISTEN is permitted),
- field (`mystery_owner = quest`) LIGHT_ORDER that requires standing still, extra particle telegraph, or more than two authentic objects — exception: `zone.nui_thieng` field LIGHT_ORDER may use up to three authentic objects as the authored finale-region escalation (dungeon/boss-owned LIGHT_ORDER uses that stage as-is),
- FORBIDDEN_GROUND that spawns extra monsters or particle floods,
- DROP_THROUGH whose platform is not one-way (`movement.md` drop-through),
- HEIGHT_BAND completable from the main-path standing volume,
- a second field puzzle on a dungeon/boss-owned MAIN,
- TESTIMONY with `talk_pool` count ≠ 5,
- TESTIMONY `implicated_id` that does not resolve in quest-local or existing authored world objects,
- MAIN or SIDE missing a `quest_giver` NPC id that resolves in `npc_shop_catalog.md`,
- act-closing MAIN (MAIN 4) with `completion_mode ≠ TURN_IN`,
- act-closing MAIN missing a `branch_flag` and `branch_resolution` block,
- MYSTERY daily template missing `mystery: true` flag, `+15% EXP` modifier, or `currency.bound` grant,
- Daily board slot 6 not assigned `daily.mystery` template.

# Invariants
```text
MAIN quests = 24
SIDE quests = 12
Daily templates = 12 standard + 1 mystery_meta; board = 6
Spirit Surge event quest = 1
MAIN reward per act = 3.0% act EXP (sub-shares 12/60, 14/60, 16/60, 18/60 of 3.0%)
SIDE reward per quest = 0.4% act EXP; 2 SIDE per act = 0.8% total SIDE
SIDE reward bundle is concrete by tier
SIDE bound amount owner = economy_catalog.md
MAIN 4 per act has binary branch resolution, cosmetic/lore only
Acts I-IV: options = appease | banish
Act V: options = seal | cleanse (visible prop state change on return visit, cosmetic only)
Act VI: options = remember | sever (visible prop state change on return visit, cosmetic only)
act-closing MAIN (MAIN 4) uses TURN_IN completion mode
all MAIN/SIDE bound to a quest_giver NPC from npc_shop_catalog.md
MAIN public-boss wait gates = 0
MAIN shared-ELITE wait gates = 0
no rare-drop blocker
no exclusive Daily power
Daily economy bonus not required for baseline affordability
MYSTERY daily template = slot 6, always present; mystery: true; +15% EXP; grants currency.bound
DAILY mystery bounty is a valid currency.bound source
Vietnamese folklore/local mystery focus
MAIN/SIDE each have exactly one mystery beat
DAILY/EVENT mystery beat = none
mystery_type set includes TESTIMONY
TESTIMONY talk_pool = 4 ambient NPCs + region guide; wrong INTERACT resets, does not fail
LISTEN is a permitted objective type
```
