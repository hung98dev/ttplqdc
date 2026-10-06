package movement

import (
	"strings"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// recover restores an entity whose authoritative position is invalid —
// a penetrated resolve (107 ILLEGAL_MOVE) or an out-of-bounds state (107
// FORCED) — through the canonical ladder of movement.md § Out-of-Bounds
// Recovery: last safe ground -> map fallback spawn -> world checkpoint.
// Recovery never kills, scores or penalizes the entity.
func (s *System) recover(p *runtime.Partition, e *runtime.Entity, st *entityState, tc *runtime.TickContext, reason protocolv1.MovementCorrectionReason) {
	x, y, ok := st.lastSafeX, st.lastSafeY, st.hasLastSafe
	if ok && !s.inBounds32(x, y) {
		ok = false
	}
	if !ok {
		x, y, ok = s.anchor("spawn.")
	}
	if !ok {
		x, y, _ = s.anchor("checkpoint.")
	}
	cp := &e.Checkpoint
	cp.X = int32(x)
	cp.Y = int32(y)
	cp.Vx = 0
	cp.Vy = 0
	cp.IsGrounded = false // re-derived by the next resolve
	cp.JumpCount = 0
	cp.DropIgnorePlatformID = 0
	cp.DropIgnoreUntilTick = 0
	cp.MovementState = protocolv1.MovementState_MOVEMENT_STATE_FALL
	e.Snap.Vx = 0
	e.Snap.Vy = 0
	e.Snap.MovementState = cp.MovementState
	_ = p.SetPosition(e.ID, int32(x), int32(y))
	s.emit107(p, e, tc, reason)
}

// anchor returns the first anchor whose id carries the given prefix
// ("spawn." = map fallback spawn, "checkpoint." = world checkpoint) in
// geometry-declared order — anchors are the canonical recovery points of
// physics_geometry_contract.md §6.
func (s *System) anchor(prefix string) (x, y int64, ok bool) {
	for _, a := range s.world.Geometry().Anchors {
		if strings.HasPrefix(a.ID, prefix) {
			return a.X, a.Y, true
		}
	}
	return 0, 0, false
}
