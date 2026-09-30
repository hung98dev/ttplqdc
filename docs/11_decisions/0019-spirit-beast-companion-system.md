# ADR-0019: Spirit Beast Companion System (Linh Thú)
status: ACCEPTED

## Context
Character builds currently use equipment (14 slots), Spirit Meridian (8 slots), Soul Contracts (25 Souls), and Formations (6 slots). Players need an engaging, long-term companion progression system rooted in Vietnamese folklore that provides direct stat resonance, clutch passive combat triggers for PK, and a dedicated progression sink.

The system must avoid generic fantasy pets, standalone AI combat minions that overload 30Hz server tick loops, and unconstrained pay-to-win stat inflation.

## Decision
1. **System Name & Theme**:
   - Canonical Vietnamese name: **Linh Thú** (Spirit Beasts / Guardian Companions).
   - Machine namespace: `beast.*`.
   - Cultural identity: Grounded in Vietnamese folklore guardian beasts and spiritual companions (Nghê Đồng, Cóc Thần, Rái Cá Sông, Chim Lạc, Kim Quy, Hổ Vàng, Hươu Sao, Gà Thần, Hỏa Điệp, Trâu Đồng).

2. **Single Active Companion**:
   - Players may collect multiple Linh Thú (character-scoped collection).
   - A character may equip zero or one Linh Thú as `ACTIVE` (Xuất chiến) at any time.
   - The active Linh Thú travels beside the character in 2D space as a non-targetable satellite presentation entity; it does not possess independent pathfinding or separate health pools on the server.

3. **Stat Resonance**:
   - 100% of the active Linh Thú's base stats (scaling Lv1..60) plus its 3 equipped items are transferred directly to the owner's combat stat pool.

4. **Passive Skills & Clutch Counter Mechanics**:
   - Each Linh Thú has 2 unique passive skills:
     - Passive 1 (Combat / Stat Utility): Scales continuously with beast level.
     - Passive 2 (Clutch Counter): Unlocks at Lv20 and enhances at milestone levels (Lv40, Lv60) with high-impact conditional triggers (e.g., CC cleanse upon stun, emergency bubble shield when below 20% HP, healing suppression against bleeding targets).

5. **Progression & Dedicated Resource**:
   - Maximum beast level is `60` (`max_beast_level = 60`), capped by character level (`beast_level <= character_level`).
   - Upgrading consumes a dedicated non-currency item resource: **Linh Đan** (`item.material.linh_dan`) from hidden chests, elites, dungeons, and Spirit Surge (`drop_tables.md`).
   - First beast is granted on `quest.main.a1.dinh_lang_bo_hoang` complete (class Tương Sinh starter). Other beasts are drop tokens; duplicates become Linh Đan (`spirit_beasts.md`).
6. **Three Dedicated Equipment Slots**:
   - Each Linh Thú possesses exactly 3 equipment slots:
     1. `beast_slot.vong_co` (Necklace / Collar / Bell) -> ATTACK, CRIT_CHANCE, ACCURACY.
     2. `beast_slot.ao_giap` (Armor / Saddle / Vest) -> MAX_HP, DEFENSE, DAMAGE_REDUCTION.
     3. `beast_slot.linh_chau` (Spirit Bead / Jade Charm) -> DODGE_CHANCE, ATTACK_SPEED, PASSIVE_EFFECT_BONUS.
   - Launch beast equipment is inventory-storable and bound by level requirements; it has only its catalogued fixed stats and no enhancement or random-roll subsystem.

7. **Elemental Synergy (Ngũ Hành Tương Sinh)**:
   - Each Linh Thú belongs to an element (KIM, MOC, THUY, HOA, THO).
   - When the beast's element generates (Tương Sinh) the character's class element (Mộc sinh Hỏa, Hỏa sinh Thổ, Thổ sinh Kim, Kim sinh Thủy, Thủy sinh Mộc), the character receives **Linh Khí Tương Sinh**: +8% bonus to transferred beast stats and -10% beast passive internal cooldowns.

8. **Affection / Bond System (Khế Ước Tâm Giao)**:
   - Feeding traditional folk foods maintains beast affection (`bond_points`, 0..100); inactivity never decreases it.
   - High bond (>= 80) grants automatic item pickup within 4.0m radius and +5% out-of-combat movement speed.

## Consequences
- Adds deep, rewarding horizontal progression supporting the 2,000-hour Level 60 journey.
- Enhances PK tactility with clutch defensive/offensive companion procs without adding pet AI desync.
- Creates a stable sink for dungeon drops and world-event participation via `linh_dan` and beast equipment.
