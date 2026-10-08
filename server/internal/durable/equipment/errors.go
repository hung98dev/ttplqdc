package equipment

import "errors"

// Deterministic reject causes — each maps to one wire error_code of the
// 402 error set (messages.md §402) via codeOf. The runtime-side gate
// rejects live in sim/equipment; these are the durable verdicts.
var (
	// errMalformed covers wire-shape failures (unknown loadout_id,
	// unknown slot_id, missing oneof).
	errMalformed = errors.New("equipment: malformed request") // INVALID_STATE
	// errItemNotFound: instance absent or not in this character's
	// inventory/slot.
	errItemNotFound = errors.New("equipment: item not found") // ITEM_NOT_FOUND
	// errItemLocked: the item is trade/session-locked.
	errItemLocked = errors.New("equipment: item locked") // ITEM_LOCKED
	// errSlotMismatch: the item's declared slot differs from slot_id.
	errSlotMismatch = errors.New("equipment: slot mismatch") // SLOT_MISMATCH
	// errLevelTooLow: character level below the item's level_min.
	errLevelTooLow = errors.New("equipment: level too low") // LEVEL_TOO_LOW
	// errInventoryFull: UNEQUIP/displacement found no free slot.
	errInventoryFull = errors.New("equipment: inventory full") // INVENTORY_FULL
	// errSkillLoadout / errSoulContractLimit are the deterministic
	// closed-kind rejects for unregistered 402 kinds (SKILL_SET is
	// IMP-017's; SOUL_CONTRACT is IMP-031's).
	errSkillLoadout      = errors.New("equipment: skill loadout kind unregistered") // SKILL_LOADOUT_INVALID
	errSoulContractLimit = errors.New("equipment: soul contract kind unregistered") // SOUL_CONTRACT_LIMIT_REACHED
	errStateConflict     = errors.New("equipment: state conflict")                  // STATE_CONFLICT
)
