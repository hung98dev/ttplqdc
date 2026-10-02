-- 000001_baseline_schema.down.sql
-- Local/staging rollback only (14-sql-migrations): drops the full baseline
-- in reverse dependency order. Never run destructively in production.

DROP TABLE IF EXISTS auth_failure_backoff;
DROP TABLE IF EXISTS rate_limit_counters;

DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS operators;
DROP TABLE IF EXISTS audit_events;

DROP TABLE IF EXISTS durable_command_receipts;
DROP TABLE IF EXISTS operations;

DROP TABLE IF EXISTS competitive_season_frozen_awards;
DROP TABLE IF EXISTS competitive_match_admissions;
DROP TABLE IF EXISTS competitive_season_finalizations;
DROP TABLE IF EXISTS guild_war_settlements;
DROP TABLE IF EXISTS guild_war_ratings;
DROP TABLE IF EXISTS pvp_sanctions;
DROP TABLE IF EXISTS pvp_match_settlements;
DROP TABLE IF EXISTS pvp_ratings;

DROP TABLE IF EXISTS player_reports;
DROP TABLE IF EXISTS character_chat_restrictions;
DROP TABLE IF EXISTS character_chivalry;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS friend_requests;
DROP TABLE IF EXISTS friends;

DROP TABLE IF EXISTS public_boss_schedules;
DROP TABLE IF EXISTS public_boss_reward_settlements;
DROP TABLE IF EXISTS boss_chest_eligibility;

DROP TABLE IF EXISTS guild_cosmetic_selections;
DROP TABLE IF EXISTS guild_cosmetic_entitlements;
DROP TABLE IF EXISTS guild_stone_masteries;
DROP TABLE IF EXISTS guild_stone_category_completions;
DROP TABLE IF EXISTS guild_storage_audit;
DROP TABLE IF EXISTS guild_storage_claims;
DROP TABLE IF EXISTS guild_blessing_votes;
DROP TABLE IF EXISTS guild_ritual_cycle_members;
DROP TABLE IF EXISTS guild_ritual_cycles;
DROP TABLE IF EXISTS guild_progression;
DROP TABLE IF EXISTS guild_applications;
DROP TABLE IF EXISTS guild_invites;
DROP TABLE IF EXISTS guild_member_contributions;
DROP TABLE IF EXISTS guild_membership_history;
DROP TABLE IF EXISTS guild_memberships;

DROP TABLE IF EXISTS region_di_tich_markers;
DROP TABLE IF EXISTS world_consequence_relics;

DROP TABLE IF EXISTS erasure_intents;

DROP TABLE IF EXISTS economy_character_daily_rollups;
DROP TABLE IF EXISTS economy_account_daily_rollups;
DROP TABLE IF EXISTS trade_settlement_records;
DROP TABLE IF EXISTS auction_proceeds;

DROP TABLE IF EXISTS reward_claim_contributions;
DROP TABLE IF EXISTS reward_claim_lines;
DROP TABLE IF EXISTS reward_claims;

DROP TABLE IF EXISTS character_atlas_state;
DROP TABLE IF EXISTS atlas_milestones;
DROP TABLE IF EXISTS character_atlas;

DROP TABLE IF EXISTS beast_equipment_locations;
DROP TABLE IF EXISTS character_beast_food_daily;
DROP TABLE IF EXISTS character_beasts;

DROP TABLE IF EXISTS character_soul_resonance;
DROP TABLE IF EXISTS character_soul_collection;
DROP TABLE IF EXISTS character_souls;

DROP TABLE IF EXISTS character_loadouts;
DROP TABLE IF EXISTS character_inventories;
DROP TABLE IF EXISTS enhancement_pity;
DROP TABLE IF EXISTS item_locations;
DROP TABLE IF EXISTS auction_listings;
DROP TABLE IF EXISTS item_instances;

-- guilds before characters: guilds.leader_character_id references characters
DROP TABLE IF EXISTS guilds;

DROP TABLE IF EXISTS character_quests;
DROP TABLE IF EXISTS character_discoveries;
DROP TABLE IF EXISTS character_progression_flags;
DROP TABLE IF EXISTS character_potential_allocations;
DROP TABLE IF EXISTS character_skill_levels;
DROP TABLE IF EXISTS character_currencies;
DROP TABLE IF EXISTS character_attach_events;
DROP TABLE IF EXISTS character_activity;
DROP TABLE IF EXISTS account_entitlement_claims;
DROP TABLE IF EXISTS character_feat_milestones;
DROP TABLE IF EXISTS character_feats;
DROP TABLE IF EXISTS character_cosmetic_equips;
DROP TABLE IF EXISTS character_cosmetic_entitlements;
DROP TABLE IF EXISTS characters;

DROP TABLE IF EXISTS account_cosmetic_entitlements;
DROP TABLE IF EXISTS iap_provider_cursors;
DROP TABLE IF EXISTS iap_notification_dedup;
DROP TABLE IF EXISTS account_refund_consumed_events;
DROP TABLE IF EXISTS account_iap_entitlements;

DROP TABLE IF EXISTS auth_revocations;
DROP TABLE IF EXISTS auth_refresh_credentials;
DROP TABLE IF EXISTS auth_session_families;
DROP TABLE IF EXISTS account_identities;
DROP TABLE IF EXISTS account_password_credentials;
DROP TABLE IF EXISTS account_login_history;

DROP TABLE IF EXISTS accounts;
