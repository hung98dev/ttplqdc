package equipment

import "thinhthan/internal/durable/inventory"

// Canonical equipment layout (equipment.md § Slots/Loadouts — the spec
// is the single owner; durable mirrors it for committed mutation while
// sim/equipment mirrors it for runtime gates; the two never import each
// other across the sim/durable boundary).

// slots is the canonical 14-position set (BASIC 0-7, ADVANCED 8-13).
var slots = map[string]bool{
	"weapon": true, "head": true, "body": true, "hands": true,
	"legs": true, "feet": true, "necklace": true, "ring": true,
	"costume": true, "talisman": true, "jade": true, "seal": true,
	"relic": true, "charm": true,
}

// loadoutCount is the fixed roster (equipment.md invariant).
const loadoutCount = 3

// loadoutIDs is loadout_index (1-based) → wire loadout_id.
var loadoutIDs = [loadoutCount]string{
	"loadout.primary", "loadout.secondary_1", "loadout.secondary_2",
}

// isSlot reports whether slotID is one of the 14 canonical slots.
func isSlot(slotID string) bool { return slots[slotID] }

// loadoutID maps a 1-based index to the wire id (1..3 only).
func loadoutID(index int) (string, bool) {
	if index < 1 || index > loadoutCount {
		return "", false
	}
	return loadoutIDs[index-1], true
}

// equippedSlot returns the item_locations slot key for an equipped
// item: "<loadout_id>.<slot_id>" — inventory/slots.go owns the encoding
// (it also reads EQUIPPED rows for the 433 projection).
func equippedSlot(loadoutID, slotID string) string {
	return inventory.EquippedSlot(loadoutID, slotID)
}
