package inventory

import (
	"fmt"
	"strconv"
	"strings"
)

// Slot-string conventions (item_locations.slot VARCHAR(32)):
//
//	CHARACTER_INVENTORY -> "inv.<N>" where N is the wire zero-based
//	  slot number (inventory.md § "stable zero-based slots").
//	EQUIPPED -> "<loadout_id>.<slot_id>" where loadout_id is one of
//	  loadout.primary | loadout.secondary_1 | loadout.secondary_2 and
//	  slot_id is the wire "slot.<name>" id (equipment.md § Slots).
//	  The loadout dimension is required: three loadouts share the same
//	  slot ids, so the bare slot id alone cannot key the row.
//
// IMP-012 writes EQUIPPED rows; this package only reads them for the
// 433 projection and defines the shared convention it must follow.
const inventorySlotPrefix = "inv."

// InvSlot returns the DB slot string for wire slot n.
func InvSlot(n uint32) string { return inventorySlotPrefix + strconv.FormatUint(uint64(n), 10) }

// ParseSlot maps a CHARACTER_INVENTORY slot string to its wire number.
func ParseSlot(s string) (uint32, error) {
	if !strings.HasPrefix(s, inventorySlotPrefix) {
		return 0, fmt.Errorf("inventory: slot %q lacks %s prefix", s, inventorySlotPrefix)
	}
	n, err := strconv.ParseUint(s[len(inventorySlotPrefix):], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("inventory: slot %q: %w", s, err)
	}
	return uint32(n), nil
}

// EquippedSlot returns the DB slot string for a loadout slot.
func EquippedSlot(loadoutID, slotID string) string { return loadoutID + "." + slotID }

// splitEquippedSlot maps an EQUIPPED slot string back to (loadout_id,
// slot_id); an unrecognized string yields empty halves and the row is
// not projected (the reader drops it silently — a malformed row is not
// a snapshot-reader error).
func splitEquippedSlot(s string) (loadoutID, slotID string) {
	for _, id := range loadoutIDs {
		if strings.HasPrefix(s, id+".") {
			return id, s[len(id)+1:]
		}
	}
	return "", ""
}

// parkedSlot is the transient per-index slot a sorted row parks on while
// the permutation applies (VARCHAR(32); distinct from the 'inv.' space).
func parkedSlot(idx int) string { return "tmp." + strconv.Itoa(idx) }
