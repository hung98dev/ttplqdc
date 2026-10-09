// Package atlas persists Atlas journal progress: character_atlas page
// counters, tier promotion with atomic reward settlement (currency.special,
// presentation entitlement, LIFE_SKILL EXP), acknowledgement rows and the
// character_atlas_state revision feeding S2C_ATLAS_STATE (518).
//
// Invariants (docs/03_systems/atlas.md):
//   - Unlocks only on authoritative MONSTER_KILLED / SOUL_ACQUIRED /
//     BOSS_DEFEATED witness-relic / CHEST_OPENED / FISH_CAUGHT /
//     DISH_COOKED events; CHEST_SPOTTED and QUEST_CLUE never feed pages.
//   - Tier promotion auto-settles its bundle in the same transaction:
//     currency.special + presentation entitlement + one LIFE_SKILL EXP
//     per current act. There is no unclaimed reward state.
//   - Settlement is idempotent on the tier triple
//     atlas.tier.<character_id>.<atlas_page_id>.<tier> recorded as
//     reward_operation_id; replays never re-grant.
//   - C2S_ATLAS_CLAIM (504) acknowledges only: it writes acknowledged_at
//     once per page, grants nothing, and rejects tiers beyond the
//     reached tier with ATLAS_TIER_NOT_REACHED.
//   - Every committed counter/promotion/acknowledgement/milestone change
//     bumps character_atlas_state.revision exactly once.
package atlas
