-- 000001_baseline_schema.up.sql
-- Complete launch baseline: every table declared in docs/06_data/data_model.md
-- plus ADR-0060/0061/0065/0070/0079 and PRIV-004..006 constraints.
-- FK policy (physical_schema_contract.md §4): ON DELETE RESTRICT by default,
-- ON UPDATE NO ACTION; the only CASCADE is auth_refresh_credentials ->
-- auth_session_families and the only DEFERRABLE pair is on
-- account_entitlement_claims. Immutable once merged.

-- ==========================================================================
-- Account / Authentication
-- ==========================================================================

CREATE TABLE accounts (
  account_id                 UUID PRIMARY KEY,
  status                     VARCHAR(32) NOT NULL DEFAULT 'ACTIVE'
    CHECK (status IN ('ACTIVE', 'SUSPENDED_PAYMENT_RECONCILIATION', 'BANNED', 'PENDING_DELETION', 'TOMBSTONE_ERASED')),
  deletion_requested_at      TIMESTAMPTZ NULL,
  erasure_started_at         TIMESTAMPTZ NULL,
  erased_at                  TIMESTAMPTZ NULL,
  credential_guard_until     TIMESTAMPTZ NULL,
  economy_review_flagged_at  TIMESTAMPTZ NULL,
  created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- iap_refund_consumed_score is deliberately absent: derived-only from
-- account_refund_consumed_events (ADR-0060).
-- Baseline seeds the reserved tombstone owner (data_model.md).
INSERT INTO accounts (account_id, status, created_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'TOMBSTONE_ERASED', NOW());

CREATE TABLE account_login_history (
  account_id        UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  observed_at       TIMESTAMPTZ NOT NULL,
  device_id_hash    BYTEA NOT NULL,
  ip_prefix16_hash  BYTEA NOT NULL,
  is_new_origin     BOOLEAN NOT NULL,
  PRIMARY KEY (account_id, observed_at)
);
CREATE INDEX account_login_history_observed_at_idx ON account_login_history (observed_at);

CREATE TABLE account_password_credentials (
  account_id         UUID PRIMARY KEY REFERENCES accounts(account_id) ON DELETE RESTRICT,
  username_key       VARCHAR(20) NOT NULL UNIQUE,
  email              VARCHAR(254) NOT NULL,
  email_key          VARCHAR(254) NOT NULL UNIQUE,
  password_hash      TEXT NOT NULL,
  params_version     SMALLINT NOT NULL,
  created_at         TIMESTAMPTZ NOT NULL,
  updated_at         TIMESTAMPTZ NOT NULL
);

CREATE TABLE account_identities (
  provider_id        VARCHAR(16) NOT NULL CHECK (provider_id IN ('apple', 'google', 'steam')),
  provider_subject   VARCHAR(255) NOT NULL,
  account_id         UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  linked_at          TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (provider_id, provider_subject),
  UNIQUE (account_id, provider_id)
);

CREATE TABLE auth_session_families (
  session_family_id   UUID PRIMARY KEY,
  account_id          UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  provider_id         VARCHAR(16) NOT NULL CHECK (provider_id IN ('password', 'apple', 'google', 'steam')),
  client_platform     VARCHAR(16) NOT NULL CHECK (client_platform IN ('WINDOWS', 'ANDROID')),
  app_version         VARCHAR(32) NOT NULL,
  device_model_class  VARCHAR(32) NULL,
  device_id_hash      BYTEA NOT NULL,
  absolute_expires_at TIMESTAMPTZ NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL,
  last_refreshed_at   TIMESTAMPTZ NOT NULL,
  expires_at          TIMESTAMPTZ NOT NULL,
  revoked_at          TIMESTAMPTZ NULL,
  revoke_reason       VARCHAR(32) NULL CHECK (revoke_reason IS NULL OR revoke_reason IN
    ('LOGOUT', 'REUSE_DETECTED', 'ACCOUNT_REVOKE', 'PASSWORD_CHANGE', 'TAKEOVER_RULE', 'ERASURE_REQUEST', 'ADMIN'))
);
CREATE INDEX auth_session_families_account_id_idx ON auth_session_families (account_id);
CREATE INDEX auth_session_families_expires_at_idx ON auth_session_families (expires_at);

CREATE TABLE auth_refresh_credentials (
  credential_hash     BYTEA PRIMARY KEY,
  session_family_id   UUID NOT NULL REFERENCES auth_session_families(session_family_id) ON DELETE CASCADE,
  generation          INTEGER NOT NULL,
  issued_at           TIMESTAMPTZ NOT NULL,
  expires_at          TIMESTAMPTZ NOT NULL,
  first_presented_at  TIMESTAMPTZ NULL,
  rotated_at          TIMESTAMPTZ NULL,
  UNIQUE (session_family_id, generation)
);

CREATE TABLE auth_revocations (
  revocation_id       UUID PRIMARY KEY,
  scope               VARCHAR(16) NOT NULL CHECK (scope IN ('SESSION_FAMILY', 'ACCOUNT', 'PROVIDER_LINK')),
  account_id          UUID NULL,   -- no FK: purged with the account at erasure
  session_family_id   UUID NULL,
  provider_id         VARCHAR(16) NULL,
  not_before          TIMESTAMPTZ NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL,
  expires_at          TIMESTAMPTZ NOT NULL
);
CREATE INDEX auth_revocations_account_id_idx ON auth_revocations (account_id);

-- ==========================================================================
-- IAP / Monetization
-- ==========================================================================

CREATE TABLE account_iap_entitlements (
  entitlement_id       UUID PRIMARY KEY,
  account_id           UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  product_id           VARCHAR(64) NOT NULL,
  entitlement_type     VARCHAR(24) NOT NULL
    CHECK (entitlement_type IN ('ONE_SHOT', 'ACCOUNT_SCOPED_ACCESS', 'DIRECT_ACCOUNT_COSMETIC')),
  grant_state          VARCHAR(20) NOT NULL
    CHECK (grant_state IN ('PENDING', 'GRANTED', 'REJECTED', 'REFUNDED', 'REFUNDED_CONSUMED')),
  reject_reason        VARCHAR(24) NULL
    CHECK (reject_reason IS NULL OR reject_reason IN ('RECEIPT_INVALID', 'PRODUCT_MISMATCH', 'SEASON_TRACK_DUPLICATE')),
  platform             VARCHAR(16) NOT NULL CHECK (platform IN ('GOOGLE_PLAY', 'APP_STORE', 'STEAM')),
  platform_receipt     VARCHAR(512) NOT NULL UNIQUE,
  created_at           TIMESTAMPTZ NOT NULL,
  granted_at           TIMESTAMPTZ NULL,
  ended_at             TIMESTAMPTZ NULL,
  season_number        INTEGER NULL,
  claim_deadline_at    TIMESTAMPTZ NULL,
  UNIQUE (entitlement_id, account_id, entitlement_type),
  CHECK ((grant_state = 'REJECTED') = (reject_reason IS NOT NULL)),
  CHECK ((season_number IS NULL) = (claim_deadline_at IS NULL))
);
-- One live season track per real account per season; the tombstone is
-- excluded so erasure re-pointing never collides (ADR-0065).
CREATE UNIQUE INDEX account_iap_entitlements_live_season_idx
  ON account_iap_entitlements (account_id, season_number)
  WHERE season_number IS NOT NULL AND grant_state IN ('PENDING', 'GRANTED')
    AND account_id <> '00000000-0000-0000-0000-000000000001';
CREATE INDEX account_iap_entitlements_pending_idx
  ON account_iap_entitlements (grant_state, created_at)
  WHERE grant_state = 'PENDING';

CREATE TABLE account_refund_consumed_events (
  event_id            UUID PRIMARY KEY,
  account_id          UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  entitlement_id      UUID NOT NULL REFERENCES account_iap_entitlements(entitlement_id) ON DELETE RESTRICT,
  occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX account_refund_consumed_events_account_idx
  ON account_refund_consumed_events (account_id, occurred_at);

CREATE TABLE iap_notification_dedup (
  provider           VARCHAR(16) NOT NULL CHECK (provider IN ('APP_STORE', 'GOOGLE_PLAY', 'STEAM')),
  notification_key   VARCHAR(160) NOT NULL,
  received_at        TIMESTAMPTZ NOT NULL,
  processed_at       TIMESTAMPTZ NULL,
  PRIMARY KEY (provider, notification_key)
);
CREATE INDEX iap_notification_dedup_received_at_idx ON iap_notification_dedup (received_at);

CREATE TABLE iap_provider_cursors (
  provider           VARCHAR(16) PRIMARY KEY,
  cursor_value       VARCHAR(64) NOT NULL,
  updated_at         TIMESTAMPTZ NOT NULL
);

CREATE TABLE account_cosmetic_entitlements (
  account_id           UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  cosmetic_id          TEXT NOT NULL,
  entitlement_id       UUID NOT NULL REFERENCES account_iap_entitlements(entitlement_id) ON DELETE RESTRICT,
  granted_at           TIMESTAMPTZ NOT NULL,
  first_equipped_at    TIMESTAMPTZ NULL,
  PRIMARY KEY (account_id, cosmetic_id, entitlement_id)
);

-- ==========================================================================
-- Characters (root) + owned projections
-- ==========================================================================

CREATE TABLE characters (
  character_id          UUID PRIMARY KEY,
  account_id            UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  name                  VARCHAR(64) NOT NULL,
  name_key              VARCHAR(256) NOT NULL UNIQUE,
  class_id              VARCHAR(64) NOT NULL
    CHECK (class_id IN ('class.kim', 'class.moc', 'class.thuy', 'class.hoa', 'class.tho')),
  level                 INTEGER NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 60),
  current_exp           INTEGER NOT NULL DEFAULT 0 CHECK (current_exp >= 0 AND current_exp <= 702100000),
  unspent_skill_points  INTEGER NOT NULL DEFAULT 0 CHECK (unspent_skill_points >= 0),
  unspent_potential_points INTEGER NOT NULL DEFAULT 0 CHECK (unspent_potential_points >= 0),
  appearance            JSONB NOT NULL DEFAULT '{}'::jsonb,
  checkpoint_id         VARCHAR(64) NOT NULL DEFAULT 'checkpoint.lang_da.dinh_lang',
  map_id                VARCHAR(64) NOT NULL DEFAULT 'map.lang_da.dinh_lang',
  fishing_utc_date      DATE NULL,
  fishing_catch_count   INTEGER NOT NULL DEFAULT 0 CHECK (fishing_catch_count >= 0),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- explicit composite unique: FK target for account_entitlement_claims
  UNIQUE (character_id, account_id)
);

CREATE TABLE character_cosmetic_entitlements (
  character_id           UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  cosmetic_id            VARCHAR(64) NOT NULL,
  source_kind            VARCHAR(16) NOT NULL CHECK (source_kind IN ('PLAY', 'REDEMPTION', 'SEASON_TRACK')),
  source_entitlement_id  UUID NULL REFERENCES account_iap_entitlements(entitlement_id) ON DELETE RESTRICT,
  source_ref             VARCHAR(64) NOT NULL,
  grant_operation_id     UUID NOT NULL,
  granted_at             TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (character_id, cosmetic_id, source_ref),
  CHECK ((source_kind = 'SEASON_TRACK') = (source_entitlement_id IS NOT NULL))
);
CREATE INDEX character_cosmetic_entitlements_source_idx
  ON character_cosmetic_entitlements (source_entitlement_id)
  WHERE source_entitlement_id IS NOT NULL;

CREATE TABLE character_cosmetic_equips (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  slot           VARCHAR(32) NOT NULL,
  cosmetic_id    VARCHAR(64) NOT NULL,
  PRIMARY KEY (character_id, slot)
);

CREATE TABLE character_feats (
  character_id       UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  feat_id            TEXT NOT NULL,
  counter_value      INTEGER NOT NULL DEFAULT 0 CHECK (counter_value >= 0),
  PRIMARY KEY (character_id, feat_id)
);

CREATE TABLE character_feat_milestones (
  character_id        UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  feat_id             TEXT NOT NULL,
  milestone_threshold INTEGER NOT NULL,
  completed_at        TIMESTAMPTZ NOT NULL,
  reward_operation_id UUID NOT NULL UNIQUE,
  PRIMARY KEY (character_id, feat_id, milestone_threshold)
);

CREATE TABLE account_entitlement_claims (
  account_entitlement_id   UUID NOT NULL,
  account_id               UUID NOT NULL,
  entitlement_type         VARCHAR(24) NOT NULL CHECK (entitlement_type = 'ACCOUNT_SCOPED_ACCESS'),
  character_id             UUID NOT NULL,
  reward_tier_id           VARCHAR(64) NOT NULL,
  claimed_at               TIMESTAMPTZ NOT NULL,
  claim_operation_id       UUID NOT NULL,
  PRIMARY KEY (account_entitlement_id, character_id, reward_tier_id),
  FOREIGN KEY (account_entitlement_id, account_id, entitlement_type)
    REFERENCES account_iap_entitlements(entitlement_id, account_id, entitlement_type)
    DEFERRABLE INITIALLY IMMEDIATE,
  FOREIGN KEY (character_id, account_id)
    REFERENCES characters(character_id, account_id)
    DEFERRABLE INITIALLY IMMEDIATE
);

CREATE TABLE character_activity (
  character_id       UUID PRIMARY KEY REFERENCES characters(character_id) ON DELETE RESTRICT,
  last_attached_at   TIMESTAMPTZ NULL,
  last_detached_at   TIMESTAMPTZ NULL,
  session_active     BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE character_attach_events (
  character_id  UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
  attached_at   TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (character_id, session_epoch)
);
CREATE INDEX character_attach_events_attached_idx
  ON character_attach_events (character_id, attached_at);

CREATE TABLE character_currencies (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  currency_id    VARCHAR(32) NOT NULL
    CHECK (currency_id IN ('currency.common', 'currency.bound', 'currency.special')),
  balance        BIGINT NOT NULL CHECK (balance >= 0),
  revision       BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (character_id, currency_id)
);

CREATE TABLE character_skill_levels (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  skill_id       VARCHAR(64) NOT NULL,
  level          SMALLINT NOT NULL CHECK (level BETWEEN 0 AND 12),
  PRIMARY KEY (character_id, skill_id)
);

CREATE TABLE character_potential_allocations (
  character_id      UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  potential_id      VARCHAR(64) NOT NULL,
  allocated_points  INTEGER NOT NULL DEFAULT 0 CHECK (allocated_points >= 0),
  PRIMARY KEY (character_id, potential_id)
);

CREATE TABLE character_progression_flags (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  flag_key       VARCHAR(128) NOT NULL,
  set_at         TIMESTAMPTZ NOT NULL,
  operation_id   UUID NULL,
  PRIMARY KEY (character_id, flag_key)
);

CREATE TABLE character_discoveries (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  kind           VARCHAR(24) NOT NULL CHECK (kind IN ('MAP', 'ZONE', 'FIRST_CLEAR', 'CHEST')),
  content_key    VARCHAR(64) NOT NULL,
  discovered_at  TIMESTAMPTZ NOT NULL,
  operation_id   UUID NULL,
  PRIMARY KEY (character_id, kind, content_key)
);

CREATE TABLE character_quests (
  character_id         UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  quest_id             VARCHAR(64) NOT NULL,
  state                VARCHAR(24) NOT NULL
    CHECK (state IN ('LOCKED', 'AVAILABLE', 'ACTIVE', 'READY_TO_COMPLETE', 'COMPLETED', 'FAILED', 'EXPIRED')),
  cycle_id             VARCHAR(16) NULL,
  choice_index         SMALLINT NULL,
  progress             JSONB NOT NULL DEFAULT '{}'::jsonb,
  accepted_at          TIMESTAMPTZ NULL,
  updated_at           TIMESTAMPTZ NOT NULL,
  completed_at         TIMESTAMPTZ NULL,
  reward_operation_id  UUID NULL,
  PRIMARY KEY (character_id, quest_id)
);

-- ==========================================================================
-- Items / inventory / loadouts / enhancement pity
-- ==========================================================================

CREATE TABLE item_instances (
  item_instance_id   UUID PRIMARY KEY,
  item_id            VARCHAR(64) NOT NULL,
  quantity           INTEGER NOT NULL CHECK (quantity > 0),
  effective_binding  VARCHAR(24) NOT NULL
    CHECK (effective_binding IN ('UNBOUND', 'ACCOUNT_BOUND', 'CHARACTER_BOUND')),
  enhancement_level  SMALLINT NOT NULL DEFAULT 0 CHECK (enhancement_level BETWEEN 0 AND 16),
  item_state         JSONB NOT NULL DEFAULT '{}'::jsonb,
  content_revision   CHAR(64) NULL CHECK (content_revision IS NULL OR content_revision ~ '^[0-9a-f]{64}$'),
  created_at         TIMESTAMPTZ NOT NULL
);

CREATE TABLE guilds (
  guild_id               UUID PRIMARY KEY,
  name                   VARCHAR(96) NOT NULL,
  name_key               VARCHAR(256) NOT NULL UNIQUE,
  state                  VARCHAR(16) NOT NULL CHECK (state IN ('ACTIVE', 'DISBANDING', 'DISBANDED')),
  recruitment_mode       VARCHAR(16) NOT NULL CHECK (recruitment_mode IN ('CLOSED', 'APPLICATIONS')),
  leader_character_id    UUID NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  motd                   VARCHAR(1024) NOT NULL DEFAULT '',
  guild_revision         BIGINT NOT NULL,
  guild_storage_revision BIGINT NOT NULL,
  guild_cosmetic_revision BIGINT NOT NULL DEFAULT 0,
  created_at             TIMESTAMPTZ NOT NULL,
  disbanded_at           TIMESTAMPTZ NULL,
  CHECK (state <> 'ACTIVE' OR leader_character_id IS NOT NULL)
);

CREATE TABLE auction_listings (
  listing_id            UUID PRIMARY KEY,
  seller_character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  seller_account_id     UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  item_instance_id      UUID NOT NULL REFERENCES item_instances(item_instance_id) ON DELETE RESTRICT,
  item_id               VARCHAR(64) NOT NULL,
  quantity              INTEGER NOT NULL CHECK (quantity > 0),
  price_common          BIGINT NOT NULL CHECK (price_common BETWEEN 100 AND 2000000000),
  listing_fee_common    BIGINT NOT NULL CHECK (listing_fee_common >= 10),
  state                 VARCHAR(16) NOT NULL
    CHECK (state IN ('ACTIVE', 'SOLD', 'CANCELLED', 'EXPIRED', 'RECLAIMED', 'MOVED_TO_CLAIM')),
  listed_at             TIMESTAMPTZ NOT NULL,
  expires_at            TIMESTAMPTZ NOT NULL,
  ended_at              TIMESTAMPTZ NULL,
  buyer_character_id    UUID NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  revision              BIGINT NOT NULL,
  CHECK ((state = 'ACTIVE') = (ended_at IS NULL))
);
CREATE UNIQUE INDEX auction_listings_escrow_item_idx
  ON auction_listings (item_instance_id)
  WHERE state IN ('ACTIVE', 'CANCELLED', 'EXPIRED');
CREATE INDEX auction_listings_expiry_idx
  ON auction_listings (expires_at) WHERE state = 'ACTIVE';
CREATE INDEX auction_listings_seller_idx
  ON auction_listings (seller_character_id, state);
CREATE INDEX auction_listings_search_idx
  ON auction_listings (item_id, price_common, listing_id) WHERE state = 'ACTIVE';

CREATE TABLE item_locations (
  item_instance_id       UUID PRIMARY KEY REFERENCES item_instances(item_instance_id) ON DELETE RESTRICT,
  location_kind          VARCHAR(24) NOT NULL
    CHECK (location_kind IN ('CHARACTER_INVENTORY', 'EQUIPPED', 'BEAST_EQUIPMENT_SLOT', 'GUILD_STORAGE', 'AUCTION_ESCROW')),
  character_id           UUID NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  guild_id               UUID NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  listing_id             UUID NULL,   -- no FK: breaks the listings<->locations cycle; ownership stays via item_instance_id
  slot                   VARCHAR(32) NULL,
  depositor_character_id UUID NULL,
  depositor_account_id   UUID NULL,
  updated_at             TIMESTAMPTZ NOT NULL,
  CHECK (location_kind <> 'CHARACTER_INVENTORY' OR (character_id IS NOT NULL AND slot IS NOT NULL)),
  CHECK (location_kind <> 'EQUIPPED' OR (character_id IS NOT NULL AND slot IS NOT NULL)),
  CHECK (location_kind <> 'BEAST_EQUIPMENT_SLOT' OR (character_id IS NOT NULL AND slot IS NOT NULL)),
  CHECK (location_kind <> 'GUILD_STORAGE' OR
         (guild_id IS NOT NULL AND slot IS NOT NULL AND depositor_character_id IS NOT NULL AND depositor_account_id IS NOT NULL)),
  CHECK (location_kind <> 'AUCTION_ESCROW' OR listing_id IS NOT NULL)
);
-- Slot uniqueness inside a container (data_model.md): one live item per
-- character slot and per guild-storage section slot.
CREATE UNIQUE INDEX item_locations_character_slot_idx
  ON item_locations (character_id, location_kind, slot)
  WHERE location_kind IN ('CHARACTER_INVENTORY', 'EQUIPPED', 'BEAST_EQUIPMENT_SLOT');
CREATE UNIQUE INDEX item_locations_guild_slot_idx
  ON item_locations (guild_id, slot)
  WHERE location_kind = 'GUILD_STORAGE';
CREATE INDEX item_locations_character_idx ON item_locations (character_id);

CREATE TABLE enhancement_pity (
  item_instance_id  UUID NOT NULL REFERENCES item_instances(item_instance_id) ON DELETE RESTRICT,
  target_level      SMALLINT NOT NULL CHECK (target_level BETWEEN 13 AND 16),
  pity_fail_count   SMALLINT NOT NULL DEFAULT 0 CHECK (pity_fail_count BETWEEN 0 AND 9),
  updated_at        TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (item_instance_id, target_level)
);

CREATE TABLE character_inventories (
  character_id   UUID PRIMARY KEY REFERENCES characters(character_id) ON DELETE RESTRICT,
  capacity       INTEGER NOT NULL CHECK (capacity BETWEEN 60 AND 120),
  revision       BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE character_loadouts (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  -- 1 = loadout.primary, 2..3 = loadout.secondary_* (equipment.md)
  loadout_index  SMALLINT NOT NULL CHECK (loadout_index BETWEEN 1 AND 3),
  role           VARCHAR(16) NOT NULL CHECK (role IN ('ACTIVE', 'SUPPORT')),
  revision       BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (character_id, loadout_index)
);

-- ==========================================================================
-- Souls / builds
-- ==========================================================================

CREATE TABLE character_souls (
  soul_instance_id             UUID PRIMARY KEY,
  character_id                 UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  soul_id                      VARCHAR(64) NOT NULL,
  level                        SMALLINT NOT NULL CHECK (level BETWEEN 1 AND 5),
  current_soul_exp             INTEGER NOT NULL CHECK (current_soul_exp >= 0),
  contracted_item_instance_id  UUID NULL UNIQUE REFERENCES item_instances(item_instance_id) ON DELETE RESTRICT
);

CREATE TABLE character_soul_collection (
  character_id   UUID PRIMARY KEY REFERENCES characters(character_id) ON DELETE RESTRICT,
  revision       BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0)
);

CREATE TABLE character_soul_resonance (
  character_id            UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  soul_id                 VARCHAR(64) NOT NULL,
  memory_resonance_count  INTEGER NOT NULL DEFAULT 0 CHECK (memory_resonance_count >= 0),
  sheen_unlocked_at       TIMESTAMPTZ NULL,
  PRIMARY KEY (character_id, soul_id)
);

-- ==========================================================================
-- Spirit beasts
-- ==========================================================================

CREATE TABLE character_beasts (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  beast_id       VARCHAR(64) NOT NULL,
  level          SMALLINT NOT NULL CHECK (level >= 1),
  bond_points    INTEGER NOT NULL DEFAULT 0 CHECK (bond_points >= 0),
  is_active      BOOLEAN NOT NULL DEFAULT FALSE,
  created_at     TIMESTAMPTZ NOT NULL,
  updated_at     TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (character_id, beast_id)
);
CREATE UNIQUE INDEX character_beasts_one_active_idx
  ON character_beasts (character_id) WHERE is_active;

CREATE TABLE character_beast_food_daily (
  character_id        UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  utc_date            DATE NOT NULL,
  food_points_gained  SMALLINT NOT NULL DEFAULT 0 CHECK (food_points_gained BETWEEN 0 AND 20),
  PRIMARY KEY (character_id, utc_date)
);

CREATE TABLE beast_equipment_locations (
  character_id       UUID NOT NULL,
  beast_id           VARCHAR(64) NOT NULL,
  slot_id            VARCHAR(24) NOT NULL CHECK (slot_id IN ('vong_co', 'ao_giap', 'linh_chau')),
  item_instance_id   UUID NOT NULL UNIQUE REFERENCES item_instances(item_instance_id) ON DELETE RESTRICT,
  PRIMARY KEY (character_id, beast_id, slot_id),
  FOREIGN KEY (character_id, beast_id) REFERENCES character_beasts(character_id, beast_id) ON DELETE RESTRICT
);

-- ==========================================================================
-- Atlas
-- ==========================================================================

CREATE TABLE character_atlas (
  character_id         UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  atlas_page_id        VARCHAR(64) NOT NULL,
  tier                 SMALLINT NOT NULL DEFAULT 0 CHECK (tier >= 0),
  seen_count           INTEGER NOT NULL DEFAULT 0 CHECK (seen_count >= 0),
  completed_at         TIMESTAMPTZ NULL,
  reward_operation_id  UUID NULL,
  acknowledged_at      TIMESTAMPTZ NULL,
  PRIMARY KEY (character_id, atlas_page_id)
);

CREATE TABLE atlas_milestones (
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  milestone_id   VARCHAR(64) NOT NULL,
  completed_at   TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (character_id, milestone_id)
);

CREATE TABLE character_atlas_state (
  character_id   UUID PRIMARY KEY REFERENCES characters(character_id) ON DELETE RESTRICT,
  revision       BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0)
);

-- ==========================================================================
-- Reward claims (ADR-0065, ADR-0079)
-- ==========================================================================

CREATE TABLE reward_claims (
  reward_claim_id      UUID PRIMARY KEY,
  owner_character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  -- canonical enum: reward_claims.md § Claim Creation (ADR-0060)
  source_type          VARCHAR(32) NOT NULL
    CHECK (source_type IN ('MONSTER', 'BOSS', 'BOSS_CHEST', 'DUNGEON', 'QUEST', 'WORLD_EVENT',
                           'ATLAS', 'FEAT', 'LEVEL_MILESTONE', 'PVP', 'GUILD_WAR', 'GUILD',
                           'FISHING', 'HIDDEN_CHEST', 'AUCTION_ESCROW_EXPIRY', 'ADMIN_COMPENSATION')),
  source_reference     VARCHAR(160) NOT NULL,
  reward_slot          VARCHAR(64) NOT NULL,
  claim_kind           VARCHAR(16) NOT NULL CHECK (claim_kind IN ('SINGLE', 'ITEM_CONSOLIDATED', 'CURRENCY_AGGREGATE')),
  consolidation_key    VARCHAR(192) NULL,
  state                VARCHAR(16) NOT NULL CHECK (state IN ('PENDING', 'CLAIMING', 'CLAIMED', 'EXPIRED')),
  created_at           TIMESTAMPTZ NOT NULL,
  updated_at           TIMESTAMPTZ NOT NULL,
  claimed_at           TIMESTAMPTZ NULL,
  claim_operation_id   UUID NULL,
  revision             BIGINT NOT NULL,
  CHECK ((claim_kind = 'SINGLE') = (consolidation_key IS NULL))
);
CREATE UNIQUE INDEX reward_claims_pending_consolidation_idx
  ON reward_claims (owner_character_id, claim_kind, consolidation_key)
  WHERE state = 'PENDING' AND consolidation_key IS NOT NULL;
CREATE INDEX reward_claims_owner_state_idx ON reward_claims (owner_character_id, state);

CREATE TABLE reward_claim_lines (
  reward_claim_id      UUID NOT NULL REFERENCES reward_claims(reward_claim_id) ON DELETE RESTRICT,
  line_no              SMALLINT NOT NULL,
  line_kind            VARCHAR(16) NOT NULL CHECK (line_kind IN ('ITEM', 'CURRENCY')),
  item_id              VARCHAR(64) NULL,
  quantity             NUMERIC(38,0) NULL,
  effective_binding    VARCHAR(24) NULL,
  item_state           JSONB NOT NULL DEFAULT '{}'::jsonb,
  content_revision     CHAR(64) NULL CHECK (content_revision IS NULL OR content_revision ~ '^[0-9a-f]{64}$'),
  currency_id          VARCHAR(32) NULL,
  amount               NUMERIC(38,0) NULL,
  delivered_quantity   NUMERIC(38,0) NOT NULL DEFAULT 0,
  delivered_amount     NUMERIC(38,0) NOT NULL DEFAULT 0,
  PRIMARY KEY (reward_claim_id, line_no),
  CHECK (delivered_quantity >= 0 AND delivered_amount >= 0),
  CHECK ((line_kind = 'ITEM' AND delivered_quantity <= quantity AND delivered_amount = 0)
      OR (line_kind = 'CURRENCY' AND delivered_amount <= amount AND delivered_quantity = 0)),
  CHECK ((line_kind = 'ITEM' AND item_id IS NOT NULL AND quantity > 0 AND effective_binding IS NOT NULL
          AND content_revision IS NOT NULL AND currency_id IS NULL AND amount IS NULL)
      OR (line_kind = 'CURRENCY' AND currency_id IS NOT NULL AND amount > 0 AND item_id IS NULL
          AND quantity IS NULL AND effective_binding IS NULL AND item_state = '{}'::jsonb))
);

CREATE TABLE reward_claim_contributions (
  source_reward_operation_id  UUID NOT NULL,
  owner_character_id          UUID NOT NULL,
  reward_slot                 VARCHAR(64) NOT NULL,
  reward_claim_id             UUID NOT NULL REFERENCES reward_claims(reward_claim_id) ON DELETE RESTRICT,
  quantity_or_amount          NUMERIC(38,0) NOT NULL CHECK (quantity_or_amount > 0),
  created_at                  TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (source_reward_operation_id, owner_character_id, reward_slot)
);

-- ==========================================================================
-- Auction / trade settlements / economy rollups (ADR-0065)
-- ==========================================================================

CREATE TABLE auction_proceeds (
  proceeds_id               UUID PRIMARY KEY,
  settled_at                TIMESTAMPTZ NOT NULL,
  seller_character_id       UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  seller_account_id         UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  buyer_character_id        UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  buyer_account_id          UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  proceeds_amount           BIGINT NOT NULL CHECK (proceeds_amount > 0),
  item_id                   VARCHAR(64) NOT NULL,
  quantity                  INTEGER NOT NULL CHECK (quantity > 0),
  listing_id                UUID NOT NULL UNIQUE REFERENCES auction_listings(listing_id) ON DELETE RESTRICT,
  state                     VARCHAR(16) NOT NULL CHECK (state IN ('PENDING', 'CLAIMED')),
  claimed_at                TIMESTAMPTZ NULL,
  claim_operation_id        UUID NULL
);
CREATE INDEX auction_proceeds_pending_idx
  ON auction_proceeds (seller_character_id) WHERE state = 'PENDING';
CREATE INDEX auction_proceeds_buyer_idx
  ON auction_proceeds (buyer_character_id, settled_at);
CREATE INDEX auction_proceeds_seller_account_idx
  ON auction_proceeds (seller_account_id, settled_at);
CREATE INDEX auction_proceeds_buyer_account_idx
  ON auction_proceeds (buyer_account_id, settled_at);

CREATE TABLE trade_settlement_records (
  settlement_id                          UUID PRIMARY KEY,
  settled_at                             TIMESTAMPTZ NOT NULL,
  initiator_character_id                 UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  initiator_account_id                   UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  counterpart_character_id               UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  counterpart_account_id                 UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  common_sent_by_initiator               BIGINT NOT NULL CHECK (common_sent_by_initiator >= 0),
  common_sent_by_counterpart             BIGINT NOT NULL CHECK (common_sent_by_counterpart >= 0),
  trade_id                               UUID NOT NULL UNIQUE,
  settlement_operation_id                UUID NOT NULL,
  item_transfers                         JSONB NOT NULL DEFAULT '[]'::jsonb
);
CREATE INDEX trade_settlement_initiator_account_idx
  ON trade_settlement_records (initiator_account_id, settled_at);
CREATE INDEX trade_settlement_counterpart_account_idx
  ON trade_settlement_records (counterpart_account_id, settled_at);
CREATE INDEX trade_settlement_initiator_character_idx
  ON trade_settlement_records (initiator_character_id, settled_at);
CREATE INDEX trade_settlement_counterpart_character_idx
  ON trade_settlement_records (counterpart_character_id, settled_at);

CREATE TABLE economy_account_daily_rollups (
  account_id       UUID NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  utc_day          DATE NOT NULL,
  common_outflow   BIGINT NOT NULL DEFAULT 0 CHECK (common_outflow >= 0),
  common_inflow    BIGINT NOT NULL DEFAULT 0 CHECK (common_inflow >= 0),
  updated_at       TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (account_id, utc_day)
);

CREATE TABLE economy_character_daily_rollups (
  character_id            UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  utc_day                 DATE NOT NULL,
  common_outflow          BIGINT NOT NULL DEFAULT 0 CHECK (common_outflow >= 0),
  common_inflow           BIGINT NOT NULL DEFAULT 0 CHECK (common_inflow >= 0),
  trade_partner_volumes   JSONB NOT NULL DEFAULT '{}'::jsonb,
  item_partner_counts     JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at              TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (character_id, utc_day)
);

-- ==========================================================================
-- Erasure (ADR-0070, ADR-0079; PRIV-006)
-- ==========================================================================

CREATE TABLE erasure_intents (
  operation_id     UUID PRIMARY KEY,
  account_id       UUID NULL,   -- no FK; only while pending, cleared on erasure
  account_id_hash  BYTEA NOT NULL CHECK (octet_length(account_id_hash) = 32),
  prepared_at      TIMESTAMPTZ NOT NULL,
  completed_at     TIMESTAMPTZ NULL,
  CHECK ((completed_at IS NULL) = (account_id IS NOT NULL)),
  CHECK (completed_at IS NULL OR completed_at >= prepared_at)
);
CREATE UNIQUE INDEX erasure_intents_pending_account_idx
  ON erasure_intents (account_id) WHERE completed_at IS NULL;
CREATE INDEX erasure_intents_pending_prepared_idx
  ON erasure_intents (prepared_at) WHERE completed_at IS NULL;
CREATE INDEX erasure_intents_completed_idx
  ON erasure_intents (completed_at) WHERE completed_at IS NOT NULL;

-- ==========================================================================
-- Boss aftermath relic / markers (ADR-0040, ADR-0053, ADR-0061, ADR-0070)
-- ==========================================================================

CREATE TABLE world_consequence_relics (
  map_id                VARCHAR(64) NOT NULL,
  channel_id            SMALLINT NOT NULL CHECK (channel_id BETWEEN 1 AND 30),
  relic_id              VARCHAR(64) NOT NULL,
  source_id             VARCHAR(64) NOT NULL,
  relic_active          BOOLEAN NOT NULL,
  buff_effect_id        VARCHAR(64) NOT NULL,
  spawned_at            TIMESTAMPTZ NOT NULL,
  expires_at            TIMESTAMPTZ NOT NULL CHECK (expires_at = spawned_at + INTERVAL '60 minutes'),
  PRIMARY KEY (map_id, channel_id, relic_id)
);
CREATE INDEX world_consequence_relics_active_idx
  ON world_consequence_relics (relic_id, expires_at) WHERE relic_active = TRUE;
CREATE INDEX world_consequence_relics_expiry_idx
  ON world_consequence_relics (expires_at) WHERE relic_active = TRUE;

CREATE TABLE region_di_tich_markers (
  region_id                          VARCHAR(64) NOT NULL,
  boss_id                            VARCHAR(64) NOT NULL,
  last_defeated_utc                  TIMESTAMPTZ NULL,
  last_defeated_encounter_instance_id UUID NULL,
  last_defeated_participant_count    INTEGER NULL CHECK (last_defeated_participant_count IS NULL OR last_defeated_participant_count >= 1),
  relic_active_in_region             BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (region_id, boss_id),
  CHECK ((last_defeated_utc IS NULL) = (last_defeated_participant_count IS NULL)
     AND (last_defeated_utc IS NULL) = (last_defeated_encounter_instance_id IS NULL))
);

-- ==========================================================================
-- Guild (ADR-0065, ADR-0079)
-- ==========================================================================

CREATE TABLE guild_memberships (
  character_id          UUID PRIMARY KEY REFERENCES characters(character_id) ON DELETE RESTRICT,
  guild_id              UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  role                  VARCHAR(16) NOT NULL CHECK (role IN ('LEADER', 'VICE_LEADER', 'OFFICER', 'MEMBER')),
  joined_at             TIMESTAMPTZ NOT NULL,
  membership_id         UUID NOT NULL UNIQUE
);
CREATE UNIQUE INDEX guild_memberships_one_leader_idx
  ON guild_memberships (guild_id) WHERE role = 'LEADER';
CREATE INDEX guild_memberships_guild_idx ON guild_memberships (guild_id);

CREATE TABLE guild_membership_history (
  membership_id   UUID PRIMARY KEY,
  guild_id        UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  character_id    UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  joined_at       TIMESTAMPTZ NOT NULL,
  left_at         TIMESTAMPTZ NULL,
  CHECK (left_at IS NULL OR left_at >= joined_at)
);
CREATE UNIQUE INDEX guild_membership_history_open_idx
  ON guild_membership_history (character_id) WHERE left_at IS NULL;
CREATE INDEX guild_membership_history_guild_idx
  ON guild_membership_history (guild_id, joined_at, left_at);

CREATE TABLE guild_member_contributions (
  guild_id              UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  character_id          UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  lifetime_contribution BIGINT NOT NULL DEFAULT 0 CHECK (lifetime_contribution >= 0),
  cycle_id              VARCHAR(16) NOT NULL,
  cycle_contribution    BIGINT NOT NULL DEFAULT 0 CHECK (cycle_contribution >= 0),
  PRIMARY KEY (guild_id, character_id)
);

CREATE TABLE guild_invites (
  invite_id              UUID PRIMARY KEY,
  guild_id               UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  inviter_character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  target_character_id    UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  state                  VARCHAR(16) NOT NULL
    CHECK (state IN ('PENDING', 'ACCEPTED', 'DECLINED', 'CANCELLED', 'EXPIRED')),
  created_at             TIMESTAMPTZ NOT NULL,
  expires_at             TIMESTAMPTZ NOT NULL CHECK (expires_at = created_at + INTERVAL '10 minutes'),
  resolved_at            TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX guild_invites_pending_idx
  ON guild_invites (guild_id, target_character_id) WHERE state = 'PENDING';

CREATE TABLE guild_applications (
  application_id         UUID PRIMARY KEY,
  guild_id               UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  applicant_character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  state                  VARCHAR(16) NOT NULL
    CHECK (state IN ('PENDING', 'ACCEPTED', 'REJECTED', 'CANCELLED', 'EXPIRED')),
  created_at             TIMESTAMPTZ NOT NULL,
  expires_at             TIMESTAMPTZ NOT NULL CHECK (expires_at = created_at + INTERVAL '7 days'),
  resolved_at            TIMESTAMPTZ NULL,
  resolver_character_id  UUID NULL
);
CREATE UNIQUE INDEX guild_applications_pending_idx
  ON guild_applications (guild_id, applicant_character_id) WHERE state = 'PENDING';
CREATE INDEX guild_applications_applicant_pending_idx
  ON guild_applications (applicant_character_id) WHERE state = 'PENDING';

CREATE TABLE guild_progression (
  guild_id              UUID PRIMARY KEY REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  guild_exp             BIGINT NOT NULL DEFAULT 0 CHECK (guild_exp >= 0),
  guild_level           SMALLINT NOT NULL CHECK (guild_level >= 1),
  ritual_streak         INTEGER NOT NULL DEFAULT 0 CHECK (ritual_streak >= 0),
  active_blessing_id    VARCHAR(64) NULL,
  blessing_expires_at   TIMESTAMPTZ NULL,
  revision              BIGINT NOT NULL
);

CREATE TABLE guild_ritual_cycles (
  guild_id                  UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  cycle_id                  VARCHAR(16) NOT NULL,
  m_effective               SMALLINT NOT NULL,
  required_points_per_element INTEGER NOT NULL,
  points_kim                INTEGER NOT NULL DEFAULT 0,
  points_moc                INTEGER NOT NULL DEFAULT 0,
  points_thuy               INTEGER NOT NULL DEFAULT 0,
  points_hoa                INTEGER NOT NULL DEFAULT 0,
  points_tho                INTEGER NOT NULL DEFAULT 0,
  rotation_pointer          VARCHAR(4) NOT NULL,
  completed_at              TIMESTAMPTZ NULL,
  candidate_blessing_ids    VARCHAR(64)[] NULL,
  vote_closes_at            TIMESTAMPTZ NULL,
  finalized_blessing_id     VARCHAR(64) NULL,
  PRIMARY KEY (guild_id, cycle_id)
);

CREATE TABLE guild_ritual_cycle_members (
  guild_id               UUID NOT NULL,
  cycle_id               VARCHAR(16) NOT NULL,
  character_id           UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  account_id             UUID NULL,   -- severed at erasure, character_id stays
  membership_joined_at   TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (guild_id, cycle_id, character_id),
  FOREIGN KEY (guild_id, cycle_id) REFERENCES guild_ritual_cycles(guild_id, cycle_id) ON DELETE RESTRICT
);

CREATE TABLE guild_blessing_votes (
  guild_id       UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  cycle_id       VARCHAR(16) NOT NULL,
  account_id     UUID NOT NULL,
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  blessing_id    VARCHAR(64) NOT NULL,
  voted_at       TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (guild_id, cycle_id, account_id)
);

CREATE TABLE guild_storage_claims (
  claim_id                 UUID PRIMARY KEY,
  guild_id                 UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  requester_character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  item_instance_id         UUID NOT NULL,
  quantity                 INTEGER NOT NULL CHECK (quantity > 0),
  state                    VARCHAR(16) NOT NULL
    CHECK (state IN ('PENDING', 'APPROVED', 'REJECTED', 'CANCELLED', 'EXPIRED', 'COMPLETED')),
  created_at               TIMESTAMPTZ NOT NULL,
  approved_at              TIMESTAMPTZ NULL,
  approver_character_id    UUID NULL,
  expires_at               TIMESTAMPTZ NOT NULL,
  resolved_at              TIMESTAMPTZ NULL
);
CREATE INDEX guild_storage_claims_guild_state_idx
  ON guild_storage_claims (guild_id, state);

CREATE TABLE guild_storage_audit (
  audit_id                 UUID PRIMARY KEY,
  guild_id                 UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  operation_id             UUID NOT NULL,
  actor_character_id       UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  action                   VARCHAR(24) NOT NULL CHECK (action IN
    ('DEPOSIT', 'WITHDRAW', 'MOVE', 'CLAIM_REQUEST', 'CLAIM_APPROVE', 'CLAIM_REJECT',
     'CLAIM_DELIVER', 'CLAIM_CANCEL', 'CLAIM_EXPIRE')),
  section                  VARCHAR(8) NOT NULL CHECK (section IN ('COMMON', 'RESERVE')),
  item_id                  VARCHAR(64) NOT NULL,
  quantity                 INTEGER NOT NULL CHECK (quantity > 0),
  receiver_character_id    UUID NULL,
  source_character_id      UUID NULL,
  before_quantity          INTEGER NOT NULL CHECK (before_quantity >= 0),
  after_quantity           INTEGER NOT NULL CHECK (after_quantity >= 0),
  occurred_at              TIMESTAMPTZ NOT NULL
);
CREATE INDEX guild_storage_audit_guild_idx
  ON guild_storage_audit (guild_id, occurred_at);
CREATE INDEX guild_storage_audit_receiver_idx
  ON guild_storage_audit (receiver_character_id, occurred_at)
  WHERE action IN ('WITHDRAW', 'CLAIM_DELIVER');

CREATE TABLE guild_stone_category_completions (
  guild_id        UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  category_id     TEXT NOT NULL,
  season_number   INTEGER NOT NULL,
  completed_at    TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (guild_id, category_id, season_number)
);

CREATE TABLE guild_stone_masteries (
  season_id      INTEGER NOT NULL CHECK (season_id >= 0),
  character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  atlas_page_id  VARCHAR(64) NOT NULL,
  guild_id       UUID NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  mastered_at    TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (season_id, character_id, atlas_page_id)
);
CREATE INDEX guild_stone_masteries_guild_idx
  ON guild_stone_masteries (guild_id, season_id) WHERE guild_id IS NOT NULL;

CREATE TABLE guild_cosmetic_entitlements (
  guild_id            UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  cosmetic_id         VARCHAR(64) NOT NULL,
  grant_operation_id  UUID NOT NULL,
  source_reference    VARCHAR(192) NOT NULL,
  acquired_at         TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (guild_id, cosmetic_id)
);

CREATE TABLE guild_cosmetic_selections (
  guild_id     UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  slot_id      VARCHAR(8) NOT NULL CHECK (slot_id IN ('SHRINE', 'BANNER', 'CREST')),
  cosmetic_id  VARCHAR(64) NULL,
  PRIMARY KEY (guild_id, slot_id),
  FOREIGN KEY (guild_id, cosmetic_id)
    REFERENCES guild_cosmetic_entitlements(guild_id, cosmetic_id) ON DELETE RESTRICT
);

-- ==========================================================================
-- PUBLIC boss generation / eligibility (ADR-0053, ADR-0061, ADR-0079)
-- ==========================================================================

CREATE TABLE boss_chest_eligibility (
  character_id                     UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  public_boss_spawn_generation_id  UUID NOT NULL,
  boss_id                          TEXT NOT NULL,
  copy_map_id                      VARCHAR(64) NOT NULL,
  copy_channel_id                  SMALLINT NOT NULL,
  copy_defeated                    BOOLEAN NOT NULL DEFAULT FALSE,
  eligible_until                   TIMESTAMPTZ NULL,
  reward_content_revision          CHAR(64) NULL,
  claim_operation_id               UUID NULL,
  PRIMARY KEY (character_id, public_boss_spawn_generation_id, copy_map_id, copy_channel_id),
  CHECK (copy_defeated = (eligible_until IS NOT NULL)),
  CHECK ((copy_defeated AND reward_content_revision IS NOT NULL AND reward_content_revision ~ '^[0-9a-f]{64}$')
      OR (NOT copy_defeated AND reward_content_revision IS NULL))
);
CREATE INDEX boss_chest_eligibility_copy_idx
  ON boss_chest_eligibility (public_boss_spawn_generation_id, copy_map_id, copy_channel_id);

CREATE TABLE public_boss_reward_settlements (
  character_id                    UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  public_boss_spawn_generation_id UUID NOT NULL,
  reward_slot                     VARCHAR(64) NOT NULL,
  source_copy_map_id              VARCHAR(64) NOT NULL,
  source_copy_channel_id          SMALLINT NOT NULL,
  settlement_operation_id         UUID NOT NULL,
  settled_at                      TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (character_id, public_boss_spawn_generation_id, reward_slot)
);

CREATE TABLE public_boss_schedules (
  boss_id                          TEXT PRIMARY KEY,
  state                            TEXT NOT NULL CHECK (state IN ('SCHEDULED', 'OPEN')),
  public_boss_spawn_generation_id  UUID NULL,
  opened_at                        TIMESTAMPTZ NULL,
  next_spawn_at                    TIMESTAMPTZ NULL,
  revision                         BIGINT NOT NULL,
  CHECK ((state = 'OPEN' AND public_boss_spawn_generation_id IS NOT NULL AND opened_at IS NOT NULL AND next_spawn_at IS NULL)
      OR (state = 'SCHEDULED' AND public_boss_spawn_generation_id IS NULL AND opened_at IS NULL AND next_spawn_at IS NOT NULL))
);

-- ==========================================================================
-- Social / party / moderation (PRIV-004)
-- ==========================================================================

CREATE TABLE friends (
  character_low_id      UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  character_high_id     UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  created_at            TIMESTAMPTZ NOT NULL,
  created_operation_id  UUID NOT NULL,
  PRIMARY KEY (character_low_id, character_high_id),
  CHECK (character_low_id < character_high_id)
);
CREATE INDEX friends_high_idx ON friends (character_high_id);

CREATE TABLE friend_requests (
  friend_request_id        UUID PRIMARY KEY,
  requester_character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  target_character_id      UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  state                    VARCHAR(16) NOT NULL
    CHECK (state IN ('PENDING', 'ACCEPTED', 'DECLINED', 'CANCELLED', 'EXPIRED')),
  created_at               TIMESTAMPTZ NOT NULL,
  expires_at               TIMESTAMPTZ NOT NULL CHECK (expires_at = created_at + INTERVAL '7 days'),
  resolved_at              TIMESTAMPTZ NULL,
  create_operation_id      UUID NOT NULL,
  resolve_operation_id     UUID NULL,
  CHECK (requester_character_id <> target_character_id),
  CHECK ((state = 'PENDING') = (resolved_at IS NULL))
);
CREATE UNIQUE INDEX friend_requests_pending_pair_idx
  ON friend_requests (LEAST(requester_character_id, target_character_id),
                      GREATEST(requester_character_id, target_character_id))
  WHERE state = 'PENDING';
CREATE INDEX friend_requests_target_idx ON friend_requests (target_character_id, state);
CREATE INDEX friend_requests_requester_idx ON friend_requests (requester_character_id, state);
CREATE INDEX friend_requests_expiry_idx
  ON friend_requests (expires_at) WHERE state = 'PENDING';

CREATE TABLE blocks (
  blocker_character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  blocked_character_id   UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  created_at             TIMESTAMPTZ NOT NULL,
  operation_id           UUID NOT NULL,
  PRIMARY KEY (blocker_character_id, blocked_character_id),
  CHECK (blocker_character_id <> blocked_character_id)
);
CREATE INDEX blocks_blocked_idx ON blocks (blocked_character_id);

CREATE TABLE character_chivalry (
  character_id      UUID PRIMARY KEY REFERENCES characters(character_id) ON DELETE RESTRICT,
  lifetime_points   BIGINT NOT NULL DEFAULT 0 CHECK (lifetime_points >= 0),
  day_utc           DATE NULL,
  day_points        SMALLINT NOT NULL DEFAULT 0 CHECK (day_points BETWEEN 0 AND 100),
  revision          BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE character_chat_restrictions (
  character_id          UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  channel               VARCHAR(16) NOT NULL
    CHECK (channel IN ('WORLD', 'LOCAL', 'PARTY', 'GUILD', 'WHISPER')),
  expires_at            TIMESTAMPTZ NOT NULL,
  imposed_by_operator_id UUID NULL,
  imposed_at            TIMESTAMPTZ NOT NULL,
  reason_code           VARCHAR(64) NOT NULL,
  PRIMARY KEY (character_id, channel)
);
CREATE INDEX character_chat_restrictions_expiry_idx
  ON character_chat_restrictions (expires_at);

CREATE TABLE player_reports (
  report_id              UUID PRIMARY KEY,
  operation_id           UUID NOT NULL,
  reporter_account_id    UUID NOT NULL,   -- no FK (Category H)
  reporter_character_id  UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  target_character_id    UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  reason                 VARCHAR(32) NOT NULL CHECK (reason IN
    ('SPAM', 'HARASSMENT', 'HATE_OR_ABUSE', 'CHEATING', 'SCAM', 'INAPPROPRIATE_NAME', 'OTHER')),
  chat_message_id        UUID NULL,       -- no FK (chat_messages rolls off)
  reporter_notes         TEXT NULL,
  created_at             TIMESTAMPTZ NOT NULL,
  status                 VARCHAR(16) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'RESOLVED', 'DISMISSED')),
  resolved_at            TIMESTAMPTZ NULL,
  handled_by_operator_id UUID NULL,       -- no FK
  resolution_code        VARCHAR(64) NULL,
  CHECK ((status = 'OPEN') = (resolved_at IS NULL)),
  UNIQUE (reporter_account_id, operation_id)
);
CREATE INDEX player_reports_status_idx ON player_reports (status, created_at);
CREATE INDEX player_reports_reporter_idx ON player_reports (reporter_account_id, created_at);
CREATE INDEX player_reports_target_idx ON player_reports (target_character_id, created_at);
CREATE INDEX player_reports_created_idx ON player_reports (created_at);

-- ==========================================================================
-- PvP / Guild War / competitive seasons
-- ==========================================================================

CREATE TABLE pvp_ratings (
  character_id          UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  pvp_mode_id           VARCHAR(48) NOT NULL
    CHECK (pvp_mode_id IN ('pvp.mode.ranked_duel', 'pvp.mode.five_element_arena')),
  season_id             INTEGER NOT NULL CHECK (season_id >= 0),
  pvp_mmr               INTEGER NOT NULL,
  season_rating         INTEGER NOT NULL CHECK (season_rating >= 0),
  ranked_games_played   INTEGER NOT NULL DEFAULT 0 CHECK (ranked_games_played >= 0),
  lifetime_mode_games   INTEGER NOT NULL DEFAULT 0 CHECK (lifetime_mode_games >= 0),
  wins                  INTEGER NOT NULL DEFAULT 0 CHECK (wins >= 0),
  losses                INTEGER NOT NULL DEFAULT 0 CHECK (losses >= 0),
  draws                 INTEGER NOT NULL DEFAULT 0 CHECK (draws >= 0),
  updated_at            TIMESTAMPTZ NOT NULL,
  revision              BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (character_id, pvp_mode_id, season_id),
  CHECK (wins + losses + draws = ranked_games_played)
);
CREATE INDEX pvp_ratings_leaderboard_idx
  ON pvp_ratings (pvp_mode_id, season_id, season_rating DESC);

CREATE TABLE pvp_match_settlements (
  pvp_match_id              UUID NOT NULL,
  character_id              UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  pvp_mode_id               VARCHAR(48) NOT NULL
    CHECK (pvp_mode_id IN ('pvp.mode.ranked_duel', 'pvp.mode.five_element_arena')),
  season_id                 INTEGER NOT NULL,
  match_state               VARCHAR(12) NOT NULL CHECK (match_state IN ('COMPLETED', 'VOID')),
  result                    VARCHAR(8) NULL CHECK (result IS NULL OR result IN ('WIN', 'LOSS', 'DRAW')),
  participation             VARCHAR(12) NOT NULL CHECK (participation IN ('NORMAL', 'AFK', 'ABANDONED')),
  mmr_before                INTEGER NOT NULL,
  mmr_after                 INTEGER NOT NULL,
  season_rating_before      INTEGER NOT NULL,
  season_rating_after       INTEGER NOT NULL,
  abandon_penalty           SMALLINT NOT NULL DEFAULT 0 CHECK (abandon_penalty IN (0, 20)),
  reward_eligible           BOOLEAN NOT NULL,
  ranked_bound_utc_date     DATE NULL,
  ranked_bound_slot         SMALLINT NULL CHECK (ranked_bound_slot IS NULL OR ranked_bound_slot BETWEEN 1 AND 5),
  settlement_operation_id   UUID NOT NULL,
  settled_at                TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (pvp_match_id, character_id),
  CHECK ((match_state = 'VOID') = (result IS NULL)),
  CHECK (match_state = 'COMPLETED' OR (mmr_after = mmr_before AND season_rating_after = season_rating_before
                                       AND NOT reward_eligible AND abandon_penalty = 0)),
  CHECK (participation = 'NORMAL' OR NOT reward_eligible),
  CHECK ((ranked_bound_slot IS NULL) = (ranked_bound_utc_date IS NULL)),
  CHECK (ranked_bound_slot IS NULL OR reward_eligible)
);
CREATE UNIQUE INDEX pvp_match_settlements_daily_bound_idx
  ON pvp_match_settlements (character_id, ranked_bound_utc_date, ranked_bound_slot)
  WHERE ranked_bound_slot IS NOT NULL;
CREATE INDEX pvp_match_settlements_character_idx
  ON pvp_match_settlements (character_id, settled_at);

CREATE TABLE pvp_sanctions (
  sanction_id           UUID PRIMARY KEY,
  character_id          UUID NOT NULL REFERENCES characters(character_id) ON DELETE RESTRICT,
  sanction_kind         VARCHAR(24) NOT NULL CHECK (sanction_kind IN ('QUEUE_RESTRICTION')),
  reason                VARCHAR(12) NOT NULL CHECK (reason IN ('ABANDON', 'AFK')),
  source_match_kind     VARCHAR(12) NOT NULL CHECK (source_match_kind IN ('PVP', 'GUILD_WAR')),
  source_match_id       UUID NOT NULL,
  ladder_step           SMALLINT NOT NULL CHECK (ladder_step IN (1, 2, 3)),
  starts_at             TIMESTAMPTZ NOT NULL,
  ends_at               TIMESTAMPTZ NOT NULL,
  operation_id          UUID NOT NULL,
  CHECK (ends_at = starts_at + CASE ladder_step WHEN 1 THEN INTERVAL '15 minutes'
                                                WHEN 2 THEN INTERVAL '30 minutes'
                                                ELSE INTERVAL '2 hours' END),
  UNIQUE (character_id, source_match_kind, source_match_id)
);
CREATE INDEX pvp_sanctions_starts_idx ON pvp_sanctions (character_id, starts_at);
CREATE INDEX pvp_sanctions_ends_idx ON pvp_sanctions (character_id, ends_at);

CREATE TABLE guild_war_ratings (
  guild_id                 UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  season_id                INTEGER NOT NULL CHECK (season_id >= 0),
  guild_war_mmr            INTEGER NOT NULL CHECK (guild_war_mmr >= 0),
  guild_war_games_played   INTEGER NOT NULL DEFAULT 0 CHECK (guild_war_games_played >= 0),
  guild_war_wins           INTEGER NOT NULL DEFAULT 0 CHECK (guild_war_wins BETWEEN 0 AND guild_war_games_played),
  lifetime_rated_wars      INTEGER NOT NULL DEFAULT 0 CHECK (lifetime_rated_wars >= 0),
  updated_at               TIMESTAMPTZ NOT NULL,
  revision                 BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (guild_id, season_id)
);
CREATE INDEX guild_war_ratings_leaderboard_idx
  ON guild_war_ratings (season_id, guild_war_mmr DESC, guild_war_wins DESC, guild_war_games_played ASC);

CREATE TABLE guild_war_settlements (
  guild_war_match_id        UUID NOT NULL,
  settlement_type           VARCHAR(24) NOT NULL
    CHECK (settlement_type IN ('GUILD_RATING', 'GUILD_PROGRESSION', 'PERSONAL_REWARD', 'SEASON_PARTICIPATION')),
  recipient_id              UUID NOT NULL,
  guild_id                  UUID NOT NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  season_id                 INTEGER NOT NULL,
  match_state               VARCHAR(12) NOT NULL CHECK (match_state IN ('COMPLETED', 'VOID')),
  result                    VARCHAR(8) NULL CHECK (result IS NULL OR result IN ('WIN', 'LOSS')),
  mmr_before                INTEGER NULL,
  mmr_after                 INTEGER NULL,
  participation             VARCHAR(12) NULL CHECK (participation IS NULL OR participation IN ('NORMAL', 'AFK', 'ABANDONED')),
  reward_eligible           BOOLEAN NULL,
  bound_week_monday         DATE NULL,
  bound_slot                SMALLINT NULL CHECK (bound_slot IS NULL OR bound_slot BETWEEN 1 AND 3),
  settlement_operation_id   UUID NOT NULL,
  settled_at                TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (guild_war_match_id, settlement_type, recipient_id),
  CHECK ((match_state = 'VOID') = (result IS NULL)),
  CHECK ((settlement_type = 'GUILD_RATING') = (mmr_before IS NOT NULL AND mmr_after IS NOT NULL)),
  CHECK (settlement_type NOT IN ('GUILD_RATING', 'GUILD_PROGRESSION') OR recipient_id = guild_id),
  CHECK ((settlement_type IN ('PERSONAL_REWARD', 'SEASON_PARTICIPATION')) = (participation IS NOT NULL)),
  CHECK ((settlement_type = 'PERSONAL_REWARD') = (reward_eligible IS NOT NULL)),
  CHECK ((bound_slot IS NULL) = (bound_week_monday IS NULL)),
  CHECK (bound_slot IS NULL OR (settlement_type = 'PERSONAL_REWARD' AND reward_eligible)),
  CHECK (match_state = 'COMPLETED' OR (mmr_after IS NOT DISTINCT FROM mmr_before AND bound_slot IS NULL))
);
CREATE UNIQUE INDEX guild_war_settlements_weekly_bound_idx
  ON guild_war_settlements (recipient_id, bound_week_monday, bound_slot)
  WHERE bound_slot IS NOT NULL;
CREATE INDEX guild_war_settlements_season_participation_idx
  ON guild_war_settlements (recipient_id, season_id, guild_id)
  WHERE settlement_type = 'SEASON_PARTICIPATION' AND match_state = 'COMPLETED';

CREATE TABLE competitive_season_finalizations (
  scope                VARCHAR(24) NOT NULL
    CHECK (scope IN ('RANKED_DUEL', 'FIVE_ELEMENT_ARENA', 'GUILD_WAR')),
  season_id            INTEGER NOT NULL CHECK (season_id >= 0),
  cutoff_at            TIMESTAMPTZ NOT NULL,
  state                VARCHAR(16) NOT NULL CHECK (state IN ('CLOSING', 'FINALIZED')),
  finalized_at         TIMESTAMPTZ NULL,
  content_revision     CHAR(64) NOT NULL CHECK (content_revision ~ '^[0-9a-f]{64}$'),
  PRIMARY KEY (scope, season_id),
  CHECK ((state = 'CLOSING') = (finalized_at IS NULL))
);

CREATE TABLE competitive_match_admissions (
  scope                VARCHAR(24) NOT NULL
    CHECK (scope IN ('RANKED_DUEL', 'FIVE_ELEMENT_ARENA', 'GUILD_WAR')),
  match_id             UUID NOT NULL,
  season_id            INTEGER NOT NULL CHECK (season_id >= 0),
  admitted_at          TIMESTAMPTZ NOT NULL,
  deadline_at          TIMESTAMPTZ NOT NULL,
  state                VARCHAR(16) NOT NULL
    CHECK (state IN ('PREPARING', 'ACTIVE', 'RESOLVING', 'COMPLETED', 'VOID', 'CANCELLED')),
  terminal_at          TIMESTAMPTZ NULL,
  PRIMARY KEY (scope, match_id),
  CHECK (state NOT IN ('COMPLETED', 'VOID', 'CANCELLED') OR terminal_at IS NOT NULL)
);
CREATE INDEX competitive_match_admissions_scope_idx
  ON competitive_match_admissions (scope, season_id, state);

CREATE TABLE competitive_season_frozen_awards (
  scope                VARCHAR(24) NOT NULL
    CHECK (scope IN ('RANKED_DUEL', 'FIVE_ELEMENT_ARENA', 'GUILD_WAR')),
  season_id            INTEGER NOT NULL CHECK (season_id >= 0),
  owner_kind           VARCHAR(16) NOT NULL CHECK (owner_kind IN ('CHARACTER', 'GUILD')),
  owner_id             UUID NOT NULL,
  cosmetic_id          VARCHAR(64) NOT NULL,
  frozen_at            TIMESTAMPTZ NOT NULL,
  source_operation_id  UUID NOT NULL,
  guild_id             UUID NULL REFERENCES guilds(guild_id) ON DELETE RESTRICT,
  membership_id        UUID NULL REFERENCES guild_membership_history(membership_id) ON DELETE RESTRICT,
  delivered_at         TIMESTAMPTZ NULL,
  PRIMARY KEY (scope, season_id, owner_kind, owner_id, cosmetic_id)
);

-- ==========================================================================
-- Idempotency / audit (ADR-0065, ADR-0079)
-- ==========================================================================

CREATE TABLE operations (
  operation_family     VARCHAR(48) NOT NULL,
  owner_kind           VARCHAR(16) NOT NULL CHECK (owner_kind IN ('ACCOUNT', 'CHARACTER', 'GUILD', 'WORLD')),
  owner_id             UUID NOT NULL,
  operation_id         UUID NOT NULL,
  request_fingerprint  BYTEA NOT NULL,
  outcome              JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at           TIMESTAMPTZ NOT NULL,
  completed_at         TIMESTAMPTZ NOT NULL,
  replay_until         TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (operation_family, owner_id, operation_id)
);
CREATE INDEX operations_replay_until_idx ON operations (replay_until);

CREATE TABLE durable_command_receipts (
  operation_family       VARCHAR(48) NOT NULL,
  owner_kind             VARCHAR(16) NOT NULL CHECK (owner_kind IN ('ACCOUNT', 'CHARACTER')),
  owner_id               UUID NOT NULL,
  operation_id           UUID NOT NULL,
  request_fingerprint    BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
  admitted_at            TIMESTAMPTZ NOT NULL,
  issued_at              TIMESTAMPTZ NOT NULL,
  replay_until           TIMESTAMPTZ NOT NULL,
  state                  VARCHAR(24) NOT NULL
    CHECK (state IN ('ADMITTED', 'COMMITTED', 'EXPIRED_UNCOMMITTED', 'REJECTED')),
  outcome_schema_version INTEGER NULL,
  outcome                BYTEA NULL,
  completed_at           TIMESTAMPTZ NULL,
  disposition_ack_at     TIMESTAMPTZ NULL,
  PRIMARY KEY (operation_family, owner_id, operation_id),
  CHECK (replay_until = issued_at + INTERVAL '180 days'),
  CHECK (issued_at <= admitted_at + INTERVAL '60 seconds' AND admitted_at < replay_until),
  CHECK ((state = 'ADMITTED' AND outcome_schema_version IS NULL AND outcome IS NULL AND completed_at IS NULL)
      OR (state <> 'ADMITTED' AND outcome_schema_version IS NOT NULL AND outcome_schema_version = 1
          AND outcome IS NOT NULL AND completed_at IS NOT NULL)),
  CHECK (outcome IS NULL OR octet_length(outcome) BETWEEN 1 AND 1048576),
  CHECK (disposition_ack_at IS NULL OR (state <> 'ADMITTED' AND disposition_ack_at >= completed_at))
);
CREATE INDEX durable_command_receipts_purge_idx
  ON durable_command_receipts (replay_until, operation_family, owner_id, operation_id)
  WHERE disposition_ack_at IS NOT NULL;
CREATE INDEX durable_command_receipts_open_idx
  ON durable_command_receipts (state, admitted_at)
  WHERE disposition_ack_at IS NULL;

CREATE TABLE audit_events (
  audit_event_id        UUID PRIMARY KEY,
  occurred_at           TIMESTAMPTZ NOT NULL,
  actor_kind            VARCHAR(16) NOT NULL CHECK (actor_kind IN ('PLAYER', 'OPERATOR', 'SYSTEM')),
  actor_id              UUID NULL,
  subject_account_id    UUID NULL,
  subject_character_id  UUID NULL,
  action                VARCHAR(48) NOT NULL,
  reason                VARCHAR(512) NULL,
  ticket_id             VARCHAR(64) NULL,
  operation_id          UUID NULL,
  payload               JSONB NOT NULL DEFAULT '{}'::jsonb,
  system_notification_key BYTEA NULL
    CHECK (system_notification_key IS NULL OR octet_length(system_notification_key) = 32)
);
CREATE UNIQUE INDEX audit_events_notification_idx
  ON audit_events (system_notification_key) WHERE system_notification_key IS NOT NULL;
CREATE INDEX audit_events_subject_account_idx ON audit_events (subject_account_id, occurred_at);
CREATE INDEX audit_events_subject_character_idx ON audit_events (subject_character_id, occurred_at);
CREATE INDEX audit_events_occurred_idx ON audit_events (occurred_at);

-- ==========================================================================
-- Operators (PRIV-005) / chat
-- ==========================================================================

CREATE TABLE operators (
  operator_id            UUID PRIMARY KEY,
  login_key              VARCHAR(32) NOT NULL UNIQUE,
  password_hash          TEXT NULL,
  totp_secret_encrypted  BYTEA NULL,
  role                   VARCHAR(16) NOT NULL CHECK (role IN ('SUPPORT', 'MODERATOR', 'ECONOMY', 'ADMIN')),
  status                 VARCHAR(16) NOT NULL CHECK (status IN ('ACTIVE', 'DISABLED')),
  created_at             TIMESTAMPTZ NOT NULL,
  last_login_at          TIMESTAMPTZ NULL,
  disabled_at            TIMESTAMPTZ NULL,
  CHECK ((status = 'ACTIVE' AND password_hash IS NOT NULL AND totp_secret_encrypted IS NOT NULL AND disabled_at IS NULL)
      OR (status = 'DISABLED' AND password_hash IS NULL AND totp_secret_encrypted IS NULL AND disabled_at IS NOT NULL))
);
CREATE INDEX operators_disabled_idx
  ON operators (disabled_at) WHERE status = 'DISABLED';

CREATE TABLE chat_messages (
  message_id           UUID PRIMARY KEY,
  sender_account_id    UUID NOT NULL,
  sender_character_id  UUID NOT NULL,
  channel              VARCHAR(16) NOT NULL CHECK (channel IN ('WORLD', 'PARTY', 'GUILD', 'WHISPER', 'LOCAL')),
  scope_id             TEXT NULL,
  content              TEXT NOT NULL,
  created_at           TIMESTAMPTZ NOT NULL
);
CREATE INDEX chat_messages_sender_idx ON chat_messages (sender_account_id, created_at);
CREATE INDEX chat_messages_created_idx ON chat_messages (created_at);

-- ==========================================================================
-- Rate limiting / auth backoff (external_integrations.md §3, ADR-0064)
-- ==========================================================================

CREATE TABLE rate_limit_counters (
  key_hash         BYTEA PRIMARY KEY,
  window_seconds   INTEGER NOT NULL,
  window_start     TIMESTAMPTZ NOT NULL,
  current_count    INTEGER NOT NULL CHECK (current_count >= 0),
  previous_count   INTEGER NOT NULL DEFAULT 0 CHECK (previous_count >= 0)
);

CREATE TABLE auth_failure_backoff (
  key_hash               BYTEA PRIMARY KEY,
  consecutive_failures   INTEGER NOT NULL CHECK (consecutive_failures >= 0),
  locked_until           TIMESTAMPTZ NULL,
  last_failure_at        TIMESTAMPTZ NOT NULL
);
