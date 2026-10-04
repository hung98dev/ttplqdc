package lockorder

// Priority is a lock-order priority encoded in tenths so the receipt
// tier is exact (25 = 2.5). Lower values lock first (database.md § Lock
// Order: "lock lower number first").
type Priority int

const (
	// SeasonFinalization serializes season boundary work via advisory lock.
	SeasonFinalization Priority = 0
	// Accounts covers accounts and their auth/erasure rows.
	Accounts Priority = 10
	// Characters covers characters and their projections.
	Characters Priority = 20
	// Receipts is the priority-2.5 durable_command_receipts tier: after
	// account/character locks, before value aggregates.
	Receipts Priority = 25
	// Currencies covers character_currencies.
	Currencies Priority = 30
	// Inventories covers character_inventories.
	Inventories Priority = 40
	// Items covers item_instances / item_locations.
	Items Priority = 50
	// Beasts covers character_beasts and beast rows.
	Beasts Priority = 60
	// Souls covers character_souls and soul collection/resonance.
	Souls Priority = 70
	// IAP covers entitlements, refund events and provider dedup/cursors.
	IAP Priority = 80
	// Cosmetics covers account/character cosmetic entitlements and equips.
	Cosmetics Priority = 90
	// Social covers friends, friend_requests, blocks, player_reports.
	Social Priority = 100
	// Guilds covers guilds + membership/invite/application/Stone/cosmetic rows.
	Guilds Priority = 110
	// GuildProgression covers progression, ritual cycles and blessing votes.
	GuildProgression Priority = 120
	// GuildStorage covers guild-storage item_locations rows and
	// guild_storage_claims/audit (priority 13).
	GuildStorage Priority = 130
	// Trade covers trade_settlement_records.
	Trade Priority = 140
	// Auction covers auction_listings and auction_proceeds.
	Auction Priority = 150
	// Competitive covers match admissions, PvP/Guild War ratings and
	// settlements, sanctions and frozen season awards.
	Competitive Priority = 160
	// Rewards covers reward claims, chest eligibility and boss settlements.
	Rewards Priority = 170
	// WorldBoss covers region_di_tich_markers (first), then
	// world_consequence_relics, then public_boss_schedules (single-row tx).
	WorldBoss Priority = 180
	// Feats covers character_feats, milestones, atlas and atlas_state.
	Feats Priority = 190
	// EconomyRollups covers the economy daily rollup tables.
	EconomyRollups Priority = 200
	// Operations is the trailing operations insert: database.md requires
	// operations rows to be inserted last in the committing transaction.
	Operations Priority = 1000
)

// TableEntry is one line of the canonical priority table. Tables are
// locked in listed order inside the priority (the priority-18
// marker-first exception is encoded by the list itself).
type TableEntry struct {
	Priority Priority
	Tables   []string
}

// CanonicalOrder is the literal mirror of database.md § Lock Order.
// TestLockOrderMatchesDatabaseMd re-derives it from the spec text so a
// spec edit without a matching registry edit fails CI.
//
// item_locations appears twice by design: priority 5 is the canonical
// row-lock placement; the priority-13 entry is the guild-storage subset
// (GUILD_STORAGE rows) reached through GuildStorageLock.
var CanonicalOrder = []TableEntry{
	{SeasonFinalization, []string{"competitive_season_finalizations"}},
	{Accounts, []string{"accounts", "account_password_credentials", "account_identities", "auth_session_families", "auth_refresh_credentials", "auth_revocations", "account_login_history", "erasure_intents"}},
	{Characters, []string{"characters", "character_activity", "character_attach_events", "character_chivalry", "character_chat_restrictions"}},
	{Receipts, []string{"durable_command_receipts"}},
	{Currencies, []string{"character_currencies"}},
	{Inventories, []string{"character_inventories"}},
	{Items, []string{"item_instances", "item_locations"}},
	{Beasts, []string{"character_beasts", "character_beast_food_daily", "beast_equipment_locations"}},
	{Souls, []string{"character_souls", "character_soul_collection", "character_soul_resonance"}},
	{IAP, []string{"account_iap_entitlements", "account_refund_consumed_events", "iap_notification_dedup", "iap_provider_cursors"}},
	{Cosmetics, []string{"account_cosmetic_entitlements", "account_entitlement_claims", "character_cosmetic_entitlements", "character_cosmetic_equips"}},
	{Social, []string{"friends", "friend_requests", "blocks", "player_reports"}},
	{Guilds, []string{"guilds", "guild_memberships", "guild_membership_history", "guild_member_contributions", "guild_invites", "guild_applications", "guild_stone_category_completions", "guild_stone_masteries", "guild_cosmetic_entitlements", "guild_cosmetic_selections"}},
	{GuildProgression, []string{"guild_progression", "guild_ritual_cycles", "guild_ritual_cycle_members", "guild_blessing_votes"}},
	{GuildStorage, []string{"item_locations", "guild_storage_claims", "guild_storage_audit"}},
	{Trade, []string{"trade_settlement_records"}},
	{Auction, []string{"auction_listings", "auction_proceeds"}},
	{Competitive, []string{"competitive_match_admissions", "pvp_ratings", "pvp_match_settlements", "pvp_sanctions", "guild_war_ratings", "guild_war_settlements", "competitive_season_frozen_awards"}},
	{Rewards, []string{"reward_claims", "reward_claim_lines", "reward_claim_contributions", "boss_chest_eligibility", "public_boss_reward_settlements"}},
	{WorldBoss, []string{"region_di_tich_markers", "world_consequence_relics", "public_boss_schedules"}},
	{Feats, []string{"character_feats", "character_feat_milestones", "character_atlas", "character_atlas_state"}},
	{EconomyRollups, []string{"economy_account_daily_rollups", "economy_character_daily_rollups"}},
}

// priorityIndex maps table -> (priority, rank within the priority line).
// First occurrence wins for duplicated tables (item_locations resolves
// to priority 5; the priority-13 placement is only reachable through a
// Lock with an explicit priority override).
type tableRank struct {
	priority Priority
	rank     int
}

var priorityIndex = func() map[string]tableRank {
	m := make(map[string]tableRank)
	for _, e := range CanonicalOrder {
		for i, t := range e.Tables {
			if _, dup := m[t]; !dup {
				m[t] = tableRank{priority: e.Priority, rank: i}
			}
		}
	}
	return m
}()

// PriorityOf returns the canonical priority of a registered table.
func PriorityOf(table string) (Priority, bool) {
	r, ok := priorityIndex[table]
	return r.priority, ok
}

// TablesAt returns the tables of one priority in canonical lock order.
func TablesAt(p Priority) []string {
	for _, e := range CanonicalOrder {
		if e.Priority == p {
			return append([]string(nil), e.Tables...)
		}
	}
	return nil
}
