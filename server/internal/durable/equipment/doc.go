// Package equipment is the durable executor for the loadout.change
// family (C2S_LOADOUT_CHANGE 402 -> S2C_LOADOUT_RESULT 403): EQUIP,
// UNEQUIP and SWITCH_ACTIVE commit atomically under the character lock;
// SKILL_SET (IMP-017) and SOUL_CONTRACT (IMP-031) are unregistered kinds
// and reject deterministically from the 402 error set.
//
// Durable rules (equipment.md, ADR-0060, data_model.md § Loadouts):
//   - exactly 3 character_loadouts rows per character, exactly 1 ACTIVE
//   - 2 SUPPORT; loadout_revision = SUM(revision) on 403/433 and every
//     committed mutation increments at least one row's revision by 1;
//   - one item_instance_id occupies at most one position across all
//     loadouts (EQUIPPED slot "<loadout_id>.<slot_id>");
//   - EQUIP moves CHARACTER_INVENTORY -> EQUIPPED and displaces the
//     current occupant to a free inventory slot (INVENTORY_FULL aborts);
//   - UNEQUIP requires inventory capacity and returns the item's
//     contracted Soul to Collection atomically (character_souls
//     .contracted_item_instance_id -> NULL), reported on 403;
//   - SWITCH_ACTIVE flips roles without items passing through inventory.
//
// Admission-time runtime gates (in_combat/alive/transferring, the 3 s
// switch cooldown) live in sim/equipment and are evaluated by the
// ADR-0083 consult; this executor owns only the durable invariants.
package equipment
