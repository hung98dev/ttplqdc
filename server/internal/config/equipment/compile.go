package equipment

import "strings"

// CanonicalSlotOrder is the ordered 14-slot list the expansion and
// slot_ordinal use (equipment_catalog.md § Canonical Slot Order).
var CanonicalSlotOrder = []string{
	"weapon", "head", "body", "hands", "legs", "feet",
	"necklace", "ring", "costume", "talisman", "jade", "seal", "relic", "charm",
}

// TierNames is the closed tier set T1..T6 (§ Tier Budget).
var TierNames = []string{"T1", "T2", "T3", "T4", "T5", "T6"}

// ElementNames is the closed element enum (§ Element Layouts).
var ElementNames = map[string]bool{
	"KIM": true, "MOC": true, "THUY": true, "HOA": true, "THO": true,
}

// RollPool is the closed 12-ID secondary roll pool (§ Secondary Roll
// Pool; 8 original + 4 added by ADR-0037).
var RollPool = []string{
	"roll.attack_flat", "roll.defense_flat", "roll.max_hp_flat", "roll.max_mp_flat",
	"roll.crit_chance", "roll.attack_speed", "roll.cast_speed", "roll.cooldown_reduction",
	"roll.lifesteal", "roll.reflect", "roll.absorb", "roll.heal_reduction",
}

// FlatRollStats maps each flat roll ID to the stat its unit range
// resolves (§ flat-range fence); every other pool member is utility.
var FlatRollStats = map[string]string{
	"roll.attack_flat":  "ATTACK",
	"roll.defense_flat": "DEFENSE",
	"roll.max_hp_flat":  "MAX_HP",
	"roll.max_mp_flat":  "MAX_MP",
}

// StatStages is the Typed Set-Effect Convention's stage vocabulary
// (§ Typed Set-Effect Convention). A stat-modifier term in a Bonus or
// Support payload must end in one of these stages.
var StatStages = []string{"PERCENT_ADD", "FLAT_ADD", "SOURCE_ADDITIVE"}

// StatNames is the stat-prefix set the convention types.
var StatNames = map[string]bool{
	"MAX_HP": true, "MAX_MP": true, "ATTACK": true, "DEFENSE": true,
	"CRIT_CHANCE": true, "ATTACK_SPEED": true, "CAST_SPEED": true,
	"MOVE_SPEED": true, "DAMAGE_REDUCTION": true, "HEALING_RECEIVED": true,
	"LIFESTEAL": true, "REFLECT": true, "ABSORB": true, "HEAL_REDUCTION": true,
}

// SetThresholds is the closed fence set (§ Launch Shape: 2/4/6 only).
var SetThresholds = []int64{2, 4, 6}

// ItemID renders the finite expansion ID `item.eq.<tier>.<key>.<slot>`.
func ItemID(tier, setKey, slot string) string {
	return "item.eq." + strings.ToLower(tier) + "." + setKey + "." + slot
}
