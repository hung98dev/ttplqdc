-- 000005_character_beast_bond_daily.up.sql
-- Bonfire rest-EXP Linh Thú bond counter per (character_id, beast_id,
-- utc_date), cap 6/day — data_model.md § Spirit Beasts;
-- world_rules.md § Linh Thú Bonding. Separate from the food counter
-- (character_beast_food_daily).

CREATE TABLE character_beast_bond_daily (
  character_id          UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  beast_id              VARCHAR(64) NOT NULL,
  utc_date              DATE NOT NULL,
  bonfire_points_gained SMALLINT NOT NULL DEFAULT 0 CHECK (bonfire_points_gained BETWEEN 0 AND 6),
  PRIMARY KEY (character_id, beast_id, utc_date)
);
