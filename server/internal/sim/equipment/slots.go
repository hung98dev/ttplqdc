package equipment

// Slots is the canonical 14-position order (equipment.md § Slots +
// equipment_catalog.md § Canonical Slot Order). Ordinal identity is
// the position in this list — BASIC 0-7, ADVANCED 8-13.
var Slots = [14]string{
	"weapon", "head", "body", "hands", "legs", "feet",
	"necklace", "ring",
	"costume", "talisman", "jade", "seal", "relic", "charm",
}

var slotSet = func() map[string]bool {
	m := make(map[string]bool, len(Slots))
	for _, s := range Slots {
		m[s] = true
	}
	return m
}()

// IsSlot reports whether slotID is one of the 14 canonical slots.
func IsSlot(slotID string) bool { return slotSet[slotID] }

// LoadoutCount is the fixed loadout roster size (equipment.md invariant).
const LoadoutCount = 3

// LoadoutIDs is the canonical loadout_index → loadout_id mapping:
// index 1 = loadout.primary, 2..3 = loadout.secondary_*.
var LoadoutIDs = [LoadoutCount]string{
	"loadout.primary", "loadout.secondary_1", "loadout.secondary_2",
}

var loadoutSet = func() map[string]int {
	m := make(map[string]int, LoadoutCount)
	for i, id := range LoadoutIDs {
		m[id] = i + 1 // loadout_index is 1-based
	}
	return m
}()

// LoadoutIndex resolves a wire loadout_id to its 1-based loadout_index;
// ok=false for any other id (the closed roster above).
func LoadoutIndex(loadoutID string) (index int, ok bool) {
	i, ok := loadoutSet[loadoutID]
	return i, ok
}

// LoadoutID maps a 1-based loadout_index to its wire loadout_id;
// ok=false outside 1..3.
func LoadoutID(index int) (string, bool) {
	if index < 1 || index > LoadoutCount {
		return "", false
	}
	return LoadoutIDs[index-1], true
}

// EquippedSlot builds the item_locations slot key for an equipped item:
// "<loadout_id>.<slot_id>" (data_model.md § Item Locations — the loadout
// dimension is required because all three loadouts share the 14 names).
func EquippedSlot(loadoutID, slotID string) string { return loadoutID + "." + slotID }
