// Package equipment carries the pure equipment/loadout rule surface of
// 03_systems/equipment.md: the canonical 14-slot/3-loadout layout,
// the EQUIP/UNEQUIP/SWITCH_ACTIVE admission gates (in_combat, 3 s
// switch cooldown), the SUPPORT signature selection (at most one per
// support loadout, two per character, coarse build facts only), and
// the crafting.md enhancement tables (success curve, milestone floors,
// charm/insurance eligibility) that both the durable executor and the
// sim-side callers evaluate identically.
//
// This package holds no SQL, no world state, and no I/O: callers pass
// the facts; the functions return deterministic verdicts. The durable
// executor (durable/equipment) owns the committed mutation; the world
// consult answerer (edge/world wiring, ADR-0083) owns admission-time
// recomputation of the runtime gates.
package equipment
