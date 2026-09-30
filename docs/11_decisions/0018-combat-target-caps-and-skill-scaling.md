# ADR-0018: Combat Target Caps and Skill Level Target Scaling
status: ACCEPTED
amended_by: [ADR-0033]

> **AMENDMENT NOTICE**: The active-skill unlock levels in §3 ("Wide AoE / Signatures unlocked Lv55, Lv60") were superseded by **ADR-0033**. The target caps (`MAX_MONSTER_TARGETS = 4`, `MAX_PLAYER_TARGETS = 3`) remain authoritative. **Amendment (2026-09-24)**: the per-level basic-attack target bands are canonical in `../01_gameplay/skills.md` § Target Count Scaling (`basic_3` = 4 monsters / 3 players at Lv8..12 as the AoE basic; `basic_4` = 1/1 at all levels as the single-target finisher). Any basic band written in this ADR that differs is superseded.

## Context
In side-scrolling action combat, unbounded AoE target counts degrade server simulation performance, cause extreme mob clustering exploits in PvE, and make group PvP unreadable and uncompetitive. Clear, hard target limits are required.

Furthermore, skills should not grant full AoE potential immediately upon unlock; target count should participate in skill point investment so upgrading skills visibly expands combat versatility.

## Decision
1. **Global Target Ceilings**:
   - Every damaging combat action (basic attacks and active skills) is subject to hard server target caps:
     - `MAX_MONSTER_TARGETS = 4` (PvE)
     - `MAX_PLAYER_TARGETS = 3` (PvP)
   - No skill may damage more than 4 monsters or 3 players in one resolution tick.

2. **Basic Attack Target Ceilings by Unlock Level**:
   - `basic_1` (unlocked at Lv1):
     - Maximum 1 target (1 monster / 1 player).
     - Does not scale target count with level (`skill_level 1..12`: 1 monster / 1 player). Focuses on pure single-target speed and bleed/proc pressure.
   - `basic_2` (unlocked at Lv10):
     - At `skill_level 1..5`: 1 monster / 1 player.
     - At `skill_level 6..12`: 2 monsters / 1 player.
   - `basic_3` (unlocked at Lv30):
     - At `skill_level 1..3`: 1 monster / 1 player.
     - At `skill_level 4..7`: 2 monsters / 1 player.
     - At `skill_level 8..12`: 3 monsters / 1 player.
   - `basic_4` (unlocked at Lv50):
     - At `skill_level 1..3`: 2 monsters / 1 player.
     - At `skill_level 4..7`: 3 monsters / 2 players.
     - At `skill_level 8..12`: 4 monsters / 3 players.

3. **Active Skills Target Balancing**:
   - **Single-Target / Dash Attacks** (e.g. `nhat_kiem_dinh_hon`, `thach_kich`):
     - `skill_level 1..12`: 1 monster / 1 player.
   - **Directional Cleave / Small AoE** (unlocked Lv20, Lv35, Lv40):
     - At `skill_level 1..5`: 2 monsters / 1 player.
     - At `skill_level 6..12`: 3 monsters / 2 players.
   - **Wide AoE / Ground Zones / Signatures** (unlocked Lv55, Lv60):
     - At `skill_level 1..4`: 2 monsters / 1 player.
     - At `skill_level 5..8`: 3 monsters / 2 players.
     - At `skill_level 9..12`: 4 monsters / 3 players.

4. **Target Selection Order**:
   When more valid entities overlap geometry than the skill's current target cap:
   1. Authoritative locked/primary target first (if inside geometry).
   2. Distance to effect origin (closest first).
   3. Stable entity ID ascending (deterministic tie-breaker).

## Consequences
- Prevents mob-train grouping exploits and stabilizes 20 Hz / 50ms server collision costs (ADR-0007).
- Skill upgrades provide tangible utility (hitting additional targets) beyond numeric damage scaling.
- Group PvP remains tactical, readable, and tightly focused around 3-player hit limits.

## Amendment — ADR-0033

> **Superseded in part**: §3 classifies "Wide AoE / Ground Zones / Signatures" as unlocked at Lv55 and Lv60. **ADR-0033** (Skill Unlock Schedule Remap) moved Active 5 (SIGNATURE) to Lv45; Lv55 and Lv60 no longer grant skill unlocks — they grant +2 bonus skill points each. The tier label "Signatures (unlocked Lv55, Lv60)" in §3 is historical. Current active-skill unlock levels are Lv8/14/22/32/45.
>
> The target caps themselves (`MAX_MONSTER_TARGETS = 4`, `MAX_PLAYER_TARGETS = 3`) remain authoritative. All unlock levels written in §§1–3 (basics Lv10/30/50; actives Lv20/35/40/55/60) are historical — current levels are basics Lv1/4/18/36 and actives Lv8/14/22/32/45 — and per-level basic target bands are canonical in `../01_gameplay/skills.md` (2026-09-24 amendment above).
