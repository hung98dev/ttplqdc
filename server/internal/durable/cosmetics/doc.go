// Package cosmetics persists the cosmetic entitlement/equip surface
// (03_systems/cosmetics.md + 07_content/cosmetic_catalog.md):
// character-scoped play entitlements in character_cosmetic_entitlements
// (one row per grant source — ownership survives while any row
// remains), account-entitled IAP cosmetics in
// account_cosmetic_entitlements (equippable on any account character,
// first_equipped_at written once), per-slot equips in
// character_cosmetic_equips, the server-owned Folklore Feat counters
// and milestone rows in character_feats/character_feat_milestones, and
// the guild-owned entitlement/selection state in
// guild_cosmetic_entitlements/guild_cosmetic_selections plus
// guilds.guild_cosmetic_revision.
//
// Executors own three durable families: cosmetic.redeem (422 → 423,
// the per-attempt dual-route redemption — exactly one route is
// validated and consumed, the other becomes a permanent no-op once the
// entitlement exists), cosmetic.equip (424 → 425) and client.656
// (C2S_GUILD_COSMETIC_EQUIP → S2C_GUILD_RESULT 649; the 628 fan-out is
// emitted by the guild composition reading GuildCosmeticView). All
// cosmetics are non-power: nothing here touches combat, economy rates,
// or progression.
package cosmetics
