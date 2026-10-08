package equipment

import "fmt"

// Admission verdicts — the gate decisions the ADR-0083 consult answer
// and the durable executor share. Each carries the wire error_code the
// 402 error set prescribes (messages.md §402).
type Reject struct{ Code string }

func (r Reject) Error() string { return "equipment: " + r.Code }

var (
	// RejectInCombat: equip/unequip/switch rejected while in combat.
	RejectInCombat = Reject{"IN_COMBAT"}
	// RejectSwitchCooldown: SWITCH_ACTIVE inside the 3 s success window.
	RejectSwitchCooldown = Reject{"COOLDOWN_ACTIVE"}
	// RejectSlotMismatch: the item's declared slot differs from slot_id.
	RejectSlotMismatch = Reject{"SLOT_MISMATCH"}
	// RejectLevelTooLow: character level below the item's level_min.
	RejectLevelTooLow = Reject{"LEVEL_TOO_LOW"}
	// RejectInventoryFull: UNEQUIP/displacement found no free slot.
	RejectInventoryFull = Reject{"INVENTORY_FULL"}
	// RejectItemNotFound / RejectItemLocked / RejectStateConflict cover
	// custody and revision failures.
	RejectItemNotFound  = Reject{"ITEM_NOT_FOUND"}
	RejectItemLocked    = Reject{"ITEM_LOCKED"}
	RejectStateConflict = Reject{"STATE_CONFLICT"}
	// RejectSkillLoadout / RejectSoulContractLimit are the deterministic
	// closed-kind rejects for unregistered 402 kinds (SKILL_SET is
	// IMP-017's; SOUL_CONTRACT is IMP-031's — this packet registers only
	// EQUIP/UNEQUIP/SWITCH_ACTIVE).
	RejectSkillLoadout      = Reject{"SKILL_LOADOUT_INVALID"}
	RejectSoulContractLimit = Reject{"SOUL_CONTRACT_LIMIT_REACHED"}
	// RejectMalformed covers wire-shape failures (unknown loadout_id,
	// unknown slot_id, missing fields).
	RejectMalformed = Reject{"INVALID_STATE"}
)

// GateState is the runtime-fact bundle the admission gates read. The
// consult answerer fills it from live world state; the durable executor
// re-verifies only the durable-owned parts (combat is admission-side).
type GateState struct {
	InCombat     bool
	Alive        bool // dead characters cannot switch (equipment.md)
	Transferring bool
}

// CheckMutation returns nil when an EQUIP/UNEQUIP may be admitted.
func CheckMutation(st GateState) error {
	if st.InCombat {
		return RejectInCombat
	}
	return nil
}

// CheckSwitch returns nil when a SWITCH_ACTIVE may commit: alive,
// not in combat, not transferring (equipment.md § Loadout Switch).
func CheckSwitch(st GateState) error {
	if st.InCombat {
		return RejectInCombat
	}
	if !st.Alive || st.Transferring {
		return RejectStateConflict
	}
	return nil
}

// Err wraps a Reject with context for logs; the Code is the contract.
func (r Reject) Errf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", r, fmt.Sprintf(format, args...))
}
