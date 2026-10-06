package movement

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// ApplyKnockback displaces one entity with forced velocity for
// durationTicks ticks (movement.md § Forced Movement, F-06: the caller —
// combat — owns the bound). While active the forced velocity wins over
// intent-driven integration; landing or expiry ends it. Every accepted
// displacement emits a 107 KNOCKBACK correction.
func (s *System) ApplyKnockback(p *runtime.Partition, tc *runtime.TickContext, id uint64, vxMmS, vyMmS int64, durationTicks uint64) {
	e, err := p.Entity(id)
	if err != nil {
		return
	}
	st := s.state(e)
	cp := &e.Checkpoint
	st.knockUntilTk = tc.Tick + durationTicks
	cp.Vx = int32(vxMmS)
	cp.Vy = int32(vyMmS)
	cp.IsGrounded = false
	cp.MovementState = protocolv1.MovementState_MOVEMENT_STATE_KNOCKBACK
	s.emit107(p, e, tc, protocolv1.MovementCorrectionReason_MOVEMENT_CORRECTION_REASON_KNOCKBACK)
}

// ForceMove places an entity at (x, y) for a forced-placement reason —
// PORTAL, RESPAWN or FORCED (contract §5.4; ordinary collisions never
// reach this path). Velocity clears and the recovery anchor resets: the
// teleported position is the new ground truth.
func (s *System) ForceMove(p *runtime.Partition, tc *runtime.TickContext, id uint64, x, y int64, reason protocolv1.MovementCorrectionReason) {
	e, err := p.Entity(id)
	if err != nil {
		return
	}
	if !s.inBounds32(x, y) {
		return
	}
	st := s.state(e)
	st.knockUntilTk = 0
	st.hasLastSafe = true
	st.lastSafeX = x
	st.lastSafeY = y
	cp := &e.Checkpoint
	cp.X = int32(x)
	cp.Y = int32(y)
	cp.Vx = 0
	cp.Vy = 0
	cp.IsGrounded = true
	cp.JumpCount = 0
	cp.MovementState = protocolv1.MovementState_MOVEMENT_STATE_IDLE
	cp.DropIgnorePlatformID = 0
	cp.DropIgnoreUntilTick = 0
	e.Snap.Vx = 0
	e.Snap.Vy = 0
	e.Snap.MovementState = protocolv1.MovementState_MOVEMENT_STATE_IDLE
	_ = p.SetPosition(id, int32(x), int32(y))
	s.emit107(p, e, tc, reason)
}
