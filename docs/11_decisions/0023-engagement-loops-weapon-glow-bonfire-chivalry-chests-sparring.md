# ADR-0023: Engagement Loops: Visual Prestige, Village Bonfire, Chivalry Score, Hidden Chests, and Sparring Ring
status: ACCEPTED

## Context
A 2,000-hour progression MMORPG requires compelling visual feedback, cozy social hubs, altruistic mentor loops, environmental exploration rewards, and low-friction friendly competitive outlets alongside its core combat and progression systems.

To maximize player retention, community cohesion, and visual excitement without introducing economy exploits or server lag, five complementary engagement mechanics are added to the specification.

## Decision
1. **Weapon Glow and Full-Body Mythic Aura (+8..+16)**:
   - Client presentation displays distinct visual prestige effects tied to the character's active weapon enhancement level:
     - `+8..+9`: Faint blue elemental sheen (Lam Quang).
     - `+10..+11`: Mystical purple particle pulse (Tử Quang).
     - `+12..+13`: Brilliant golden radiance (Hoàng Kim Quang) — visible endgame mark.
     - `+14..+15`: Crackling elemental arcs orbiting the weapon blade/staff.
     - `+16`: Full-body Mythic Dragon/Lotus aura (Thần Binh Kim Hộ Thể) enveloping the character.
   - **World Broadcast**: Achieving `+16` enhancement triggers an authoritative server-wide celebratory notice banner (`loc.notice.enhancement_plus_16_broadcast`) across all channels.

2. **Village Bonfire Gathering (Lửa Trại Đình Làng)**:
   - Each of the 6 Safe Anchors features a communal gathering bonfire.
   - Players resting within 6.0m of an active bonfire receive:
     - Exact active-rest, EXP, bonding, kindling, and wine state contracts are canonical in `02_world/world_rules.md`.

3. **Chivalry System (Điểm Hiệp Nghĩa)**:
   - High-level characters (`Level >= 40`) completing lower-tier dungeons (Act I/II, T1/T2) in a party with at least one novice player (`Level <= 25`) earn **Chivalry Points** (`chivalry_points`, non-currency progression counter, capped at 100 points/day).
   - Chivalry Points are a non-spendable lifetime score; milestone cosmetic entitlements are granted directly. They cannot be exchanged for items or currency.
   - Maintains a vibrant newcomer-helper loop without violating the three-currency economy rule.

4. **Hidden Folklore Chests (Rương Cổ Bí Ẩn Dã Ngoại)**:
   - Each of the 18 adventure field maps conceals exactly `2` hidden chests perched on elevated terrain (banyan tree branches, cave rock ledges, abandoned shrine roofs) requiring 2D platforming and double-jumps.
   - **First-session chest exception**: one designated chest per map (`chest.hidden.<map_id>.bo_ruong.01`) is path-visible without requiring a double-jump and opens without an Old Key. It serves the first-15-minute `CHEST_SPOTTED` SLO peak (ADR-0025). Its reward table is a reduced introductory subset; it is not counted toward the elevated double-jump platforming requirement.
   - All other chests require both a double-jump to reach the elevated terrain and an **Old Key** (`item.consumable.chia_khoa_co`), dropped rarely by field monsters and elites.
   - Rewards (non-first-session chests): Guaranteed Linh Đan, regional materials, and a chance at Lucky Charms.
   - Personal availability, 30-minute cooldown, channel-hop prevention, operation identity, and reward settlement are canonical in `02_world/maps_zones.md`.

5. **Open Sparring Ring (Lôi Đài Tỷ Thí Tự Do)**:
   - Safe Anchors feature an open wooden sparring platform in the town center.
   - Any two players may step onto the platform and issue a sparring request (`C2S_SPARRING_REQUEST`).
   - The match executes in full public view of all players in that channel.
   - When either participant reaches 1 HP, the match terminates instantly with a victory banner, restoring both players to 100% HP and MP.
   - Sparring causes zero persistent loss, consumes no ranked MMR, and provides a risk-free micro-PK testing arena.

## Consequences
- Elevates weapon enhancement from a numeric spreadsheet into visible in-game prestige.
- Creates warm, living village atmospheres that encourage social bonding and trading.
- Eliminates low-tier dungeon abandonment by rewarding veterans for assisting newcomers.
- Rewards 2D platforming mastery and open-world exploration with tangible crafting assets.
- Provides immediate, friction-free PvP practice in safe social hubs.
