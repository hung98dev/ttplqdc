// Package crafting owns the durable settlement of C2S_CRAFT (404) and
// C2S_ENHANCE (406): frozen JournalCraftSnapshot / JournalEnhanceResult
// payloads written at admission by PlanCraft / PlanEnhance, replayed
// verbatim through the durable queue (crafting.md § Atomic Craft /
// § Enhancement, save_rules.md § atomic groups craft/enhance,
// protobuf_conventions.md §7 families `craft.create` / `enhance.apply`).
//
// Crafting is a GUARANTEED capacity-prevalidated all-or-nothing
// inventory creation — never a Reward Claim source. Enhancement is a
// bounded RNG attempt on one owned equipment instance applying the
// canonical rate table, milestone floors, charm/insurance rules, guild
// blessing order and the `item_instance_id + target_level` soft-pity
// state machine; the frozen outcome is replayed, never rerolled.
package crafting
