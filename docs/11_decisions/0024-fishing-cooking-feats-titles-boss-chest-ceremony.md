# ADR-0024: Folk Fishing, Hearth Cooking, Folklore Feats/Titles, and World Boss Chest Ceremony
status: ACCEPTED

## Context
A long-term 2,000-hour MMORPG requires relaxing life-skill activities, meaningful non-combat social loops, prestigious cosmetic goals for bragging rights, and euphoric celebration moments upon conquering major encounters.

Three complementary systems are designed:
1. Folk Fishing & Hearth Cooking: an interactive gathering loop feeding directly into Linh Thú affection and village bonfire buffs.
2. Folklore Feats & Glowing Titles: prestigious milestone cosmetic titles reflecting mastery across combat, gathering, crafting, and PvP.
3. World Boss Gold Chest Ceremony: an interactive, celebratory reward ritual when major bosses fall, preserving personal loot security while providing thrilling visual feedback.

## Decision
1. **Folk Fishing (Câu Cá Dân Gian)**:
   - Designated water access points (riverbanks, wooden ferry piers, village ponds) feature interactive fishing spots (`fishing_spot.<map_id>.<index>`).
   - Equipped with a **Bamboo Fishing Rod** (`item.tool.can_cau_tre`) and consuming **Earthworm Bait** (`item.consumable.moi_cau`), players engage in a simple 2D rhythm tension mini-game.
   - Yields `COMMON_CATCH` (*Cá Bống, Cá Rô Đồng, Cá Chép, Tôm Sông*) plus launch `RARE_CATCH` *Cá Chép Hóa Rồng* (`item.material.ca_chep_hoa_rong`, 100 bp / 1% in `fishing.catch.default`). Rare catch is `PHAT_HIEN`; it is never a cooking input.
   - Strictly bounded: maximum 50 successful catches per character per UTC day to prevent botting/macro abuse.

2. **Hearth Cooking (Bếp Lửa Làng Quê)**:
   - Safe Anchors feature a communal cooking hearth (`cooking_hearth.<map_id>`).
   - Combines caught fish and regional herbs into Linh Thú companion delicacies (*Cá Bống Kho Tộ*, *Cá Chép Nướng Mộc*, *Tôm Nướng Than*) and Village Wine (*Rượu Nếp Làng*).
   - Cooked delicacies serve as the primary source for increasing Linh Thú affection (`bond_points`, up to daily cap) and kindling the village bonfire.

3. **Folklore Feats and Glowing Titles (Thành Tựu & Danh Hiệu Phát Sáng)**:
   - Feats track lifetime milestone achievements across combat, gathering, crafting, and competitive modes.
   - Unlocks exclusive non-power titles rendered above character names with distinct typography, colors, and particle glows:
     - *Khắc Tinh Ma Da*: Slay 1,000 Ma Da monsters (Cyan river mist glow).
     - *Dũng Sĩ Trừ Hổ*: Slay Boss Hổ Tinh 10 times (Striped amber tiger glow).
     - *Ngư Ông Bến Đò*: Catch 200 fish (Water ripple aura).
     - *Bàn Tay Thần*: Enhance any equipment piece to +12 (Golden spark aura).
     - *Tuyệt Đỉnh Thần Binh*: Enhance any equipment piece to +16 (Radiant celestial golden dragon glow).
     - *Thiên Hạ Đệ Nhất*: Season champion in Ranked Duel (Mythic flaming crimson glow).
   - Titles are purely cosmetic and grant zero combat stats, strictly adhering to the non-power cosmetic invariant.

4. **World Boss Gold Chest Ceremony (Lễ Hội Rương Vàng Sau Khi Diệt Boss)**:
   - When a major world boss is slain, the server triggers an in-channel victory celebration:
     1. The boss collapses and a massive **Gilded Dragon Chest** (`chest.world_boss.<boss_id>`) descends at the arena center amidst festive firecrackers and celebratory music.
     2. The chest remains active for 3 minutes before fading.
     3. Every eligible participant (who met the authoritative combat contribution threshold under `bosses.md`) approaches and interacts with the chest to trigger their personal reward settlement.
   - **Personal Loot Invariant**: Interacting with the chest executes that specific player's personal drop table roll. One player claiming their reward never depletes or interferes with another player's loot. Full inventory safeguards divert overflow into Reward Claims as normal.

## Consequences
- Bridges life-skills directly with the companion bond system without adding a complex profession skill tree (non-goal preserved).
- Inspires player pride and flexing through visible glowing titles reflecting genuine in-game accomplishments.
- Replaces dry, instant loot delivery with an exhilarating communal victory ceremony after intense boss fights.
