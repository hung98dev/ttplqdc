-- 000006_character_rest_daily.up.sql
-- Bonfire rest-EXP daily tick counter per (character_id, utc_date),
-- cap 180/day — data_model.md § Character; world_rules.md
-- § Passive Rest EXP.

CREATE TABLE character_rest_daily (
  character_id       UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  utc_date           DATE NOT NULL,
  rest_ticks_gained  SMALLINT NOT NULL DEFAULT 0 CHECK (rest_ticks_gained BETWEEN 0 AND 180),
  PRIMARY KEY (character_id, utc_date)
);
