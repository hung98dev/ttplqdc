-- reward_claims.claim_kind: baseline 000001 declared VARCHAR(16), which
-- cannot hold the 17/18-char aggregate kinds 'ITEM_CONSOLIDATED' /
-- 'CURRENCY_AGGREGATE' (BLK-004, spec resolution #229). CHECK enum
-- unchanged; widening is metadata-only (no table rewrite on Postgres).

ALTER TABLE reward_claims
  ALTER COLUMN claim_kind TYPE VARCHAR(24);
