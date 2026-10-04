package collision

// CorrectionReason mirrors the wire MovementCorrectionReason enum
// (proto/thinhthan/v1/movement.proto, S2CMovementCorrection / ID 107). Values
// are identical to the generated enum so the edge can convert with a plain
// cast; this package must not import the protocol package.
//
// Per ADR-0069 and contract §5.4 an ordinary prediction error or ordinary
// collision resolution (wall stop, floor landing, step-up) never produces a
// 107 correction — only illegal or forced moves do. This API exposes a
// verdict only; deciding when to emit the message is edge/IMP-013 scope.
type CorrectionReason int

const (
	// CorrectionNone is MOVEMENT_CORRECTION_REASON_UNSPECIFIED (0).
	CorrectionNone CorrectionReason = iota
	// CorrectionIllegalMove is MOVEMENT_CORRECTION_REASON_ILLEGAL_MOVE (1).
	CorrectionIllegalMove
	// CorrectionKnockback is MOVEMENT_CORRECTION_REASON_KNOCKBACK (2).
	CorrectionKnockback
	// CorrectionPortal is MOVEMENT_CORRECTION_REASON_PORTAL (3).
	CorrectionPortal
	// CorrectionRespawn is MOVEMENT_CORRECTION_REASON_RESPAWN (4).
	CorrectionRespawn
	// CorrectionForced is MOVEMENT_CORRECTION_REASON_FORCED (5).
	CorrectionForced
)

// Correction is the legality verdict for a resolved move. It returns
// (CorrectionIllegalMove, true) only when the input position already
// penetrated blocking geometry — the deterministic sign that a predicted
// client state was impossible and must be corrected. Every ordinary
// resolution (blocked velocity, landings, steps, one-way drops) returns
// (CorrectionNone, false): they are physics, not corrections.
//
// Forced moves (knockback, portal, respawn, operator-forced) are not
// geometry-derived: the caller emits the corresponding reason itself; this
// method never invents one.
func (r Result) Correction() (CorrectionReason, bool) {
	if r.illegal {
		return CorrectionIllegalMove, true
	}
	return CorrectionNone, false
}
