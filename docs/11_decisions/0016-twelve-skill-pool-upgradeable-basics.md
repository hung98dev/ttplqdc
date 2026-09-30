# ADR-0016: Twelve-Skill Pool and Upgradeable Basic Attacks
status: ACCEPTED
amended_by: [ADR-0033]

> **AMENDMENT NOTICE**: Skill unlock levels and skill-point budgets were amended by **ADR-0033** (`docs/01_gameplay/progression.md`). The levels listed in §1 of this ADR (basics Lv1/10/30/50, actives Lv20/35/40/55/60, passives Lv15/25/45) are **historical**. Current authoritative unlock schedule: basics Lv1/4/18/36, actives Lv8/14/22/32/45, passives Lv11/27/50. Current Lv60 skill-point total is 75 (ADR-0033).

## Context
The original skill model provided one unlevelled basic attack, six active skills (max level 10), and two passive skills (max level 10) per class. That structure limited basic attack variation, left auto-attacks detached from skill point investment, and offered limited build divergence for auto-attack versus burst playstyles in real-time PvP.

Combat design requires basic attacks to be integrated into character progression, allowing distinct basic attack variants across character leveling, skill point investment, cooldown reduction scaling down to class-specific speed caps, and status effect procs to deepen PK dynamics.

## Decision
1. **Skill Pool Structure**:
   Each class owns exactly 12 upgradeable skills:
   - 4 Basic Attack skills (unlocked at character levels 1, 10, 30, 50).
   - 5 Active skills (unlocked at character levels 20, 35, 40, 55, 60).
   - 3 Passive skills (unlocked at character levels 15, 25, 45).
   Across five classes, the launch content contains exactly 60 upgradeable skills.

2. **Skill Level Limits**:
   - Basic Attack skills: maximum `skill_level = 12`.
   - Active skills: maximum `skill_level = 12`.
   - Passive skills: maximum `skill_level = 6`.
   - Upgrading one skill level costs exactly `1` skill point.

3. **Leveling and Scaling Dynamics**:
   - Upgrading any skill increases its primary power (damage multiplier, flat damage, heal/shield value, or stat bonus) and reduces its cooldown (`cooldown_ms`).
   - Passive skills scale trigger values, stat bonuses, or duration across levels 1..6.

4. **Basic Attack Cooldown Speed Bounds**:
   Basic attack cooldown scales down with skill level, reaching class-specific minimum cooldowns at `skill_level = 12`:
   - `class.kim` (Kiếm Khách): 0.20s (200ms) at max level (Lv1: 0.50s -> Lv12: 0.20s).
   - `class.thuy` (Thủy Sư): 0.30s (300ms) at max level (Lv1: 0.65s -> Lv12: 0.30s).
   - `class.moc` (Dược Sư): 0.35s (350ms) at max level (Lv1: 0.70s -> Lv12: 0.35s).
   - `class.hoa` (Phù Sư): 0.40s (400ms) at max level (Lv1: 0.80s -> Lv12: 0.40s).
   - `class.tho` (Hộ Pháp): 0.50s (500ms) at max level (Lv1: 0.95s -> Lv12: 0.50s).

5. **Basic Attack Status Effect Procs**:
   Every basic attack includes a percentage chance to inflict an elemental or combat status effect on hit. The proc chance scales upward with skill level (baseline 5-8% at Lv1 scaling to 18-25% at Lv12):
   - `class.kim`: BLEED / VULNERABLE.
   - `class.moc`: POISON.
   - `class.thuy`: CHILL / SLOW / FREEZE.
   - `class.hoa`: BURN.
   - `class.tho`: STUN / ROOT / WEAKEN.

6. **Loadout**:
   - 1 dedicated Basic Attack slot: character equips 1 of the 4 learned basic attacks.
   - 5 Active Skill slots: character equips active skills into the hotbar.
   - 3 Passive Skills: all learned passives are active simultaneously without slot restrictions.

7. **Skill Point Economy**:
   - A Level-60 character earns 59 skill points from leveling (1 point per level from Lv2 to Lv60).
   - Fully maxing all 12 skills requires 114 skill points (4*11 + 5*11 + 3*5 = 44 + 55 + 15 = 114).
   - Players must specialize their builds, choosing between basic-attack-centric, active-burst, or passive-sustain/control configurations.

## Consequences
- Basic attack is an active participant in build optimization and respec choices.
- Total launch upgradeable skills increases from 40 to 60.
- Class skill kits and timing tables in `class_skill_catalog.md` are updated to provide concrete level 1..12 and level 1..6 data.
- PvP combat gains depth through auto-attack weaving, proc timing, and cooldown reduction.

## Amendment — ADR-0033

> **Superseded in part**: The skill unlock levels in §1 were revised by **ADR-0033** (Skill Unlock Schedule Remap). The original levels — Basic 1/10/30/50, Active 20/35/40/55/60, Passive 15/25/45 — are historical and no longer authoritative. The new schedule is: Basic 1/4/18/36, Active 8/14/22/32/45, Passive 11/27/50. The 4/5/3 pool shape (twelve skills per class) is unchanged.
>
> The Lv60 skill-point total in §7 (`59 from leveling`) is also historical. Lv55 and Lv60 no longer grant skill unlocks; each grants +2 bonus skill points. Combined with 12 bonus skill books (ADR-0025), the Lv60 total is `59 (level-ups) + 12 (books) + 4 (Lv55/Lv60 milestone) = **75**` out of 114 required to max everything. All current specs use `75`.
