package movement

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/collision"
	"thinhthan/internal/sim/spatial/geometry"
)

// stepEntity runs the §4.2 integration order for one entity: intent merge
// -> vx -> vy -> X sweep -> Y sweep -> one-way -> grounded recompute ->
// checkpoint + replication writes. All math is integer millimeters.
func (s *System) stepEntity(p *runtime.Partition, e *runtime.Entity, tc *runtime.TickContext) {
	st := s.state(e)
	cp := &e.Checkpoint
	if cp.RunSpeedMmS == 0 {
		// Effective parameters replicate the session's movement profile;
		// the only seam that may change them mid-run (MOVE_SPEED stats,
		// IMP-011) has not landed — a mid-run change emits 107
		// ILLEGAL_MOVE per contract §5.4.
		cp.RunSpeedMmS = RunSpeedMmS
		cp.FirstJumpMmS = FirstJumpMmS
		cp.SecondJumpMmS = SecondJumpMmS
		cp.GravityMmS2 = GravityMmS2
		cp.MaxFallMmS = MaxFallMmS
		cp.AirControlBp = AirControlBp
		cp.MaxStepHeightMm = MaxStepHeightMm
	}

	// Out-of-bounds input state: recover before integrating — an entity
	// already outside the space bounds can produce no legal move.
	if !s.inBounds(cp.X, cp.Y) {
		s.recover(p, e, st, tc, protocolv1.MovementCorrectionReason_MOVEMENT_CORRECTION_REASON_FORCED)
		return
	}

	// --- 1. Intent merge -------------------------------------------------
	// Held intent is the coalesced baseline; discrete edges refine it in
	// receive order (ADR-0038): PRESS/FLIP set the direction, RELEASE
	// clears a matching direction, JUMP/DROP are requests consumed below.
	dir := int32(0)
	switch cp.HeldHorizontalIntent {
	case protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT:
		dir = -1
	case protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT:
		dir = 1
	}
	jump := false
	drop := false
	for _, b := range e.PendingEdges() {
		if s.stamps != nil {
			if mono, recv, ok := s.stamps(e.ID); ok {
				s.evalStale(e, mono, recv, tc)
			}
		}
		s.noteEdge(e, b, tc)
		switch b {
		case EdgeJump:
			jump = true
		case EdgeDrop:
			drop = true
		case EdgePressLeft, EdgeFlipLeft:
			dir = -1
		case EdgePressRight, EdgeFlipRight:
			dir = 1
		case EdgeReleaseLeft:
			if dir == -1 {
				dir = 0
			}
		case EdgeReleaseRight:
			if dir == 1 {
				dir = 0
			}
		}
	}
	switch dir {
	case -1:
		cp.HeldHorizontalIntent = protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT
		cp.Facing = protocolv1.Facing_FACING_LEFT
	case 1:
		cp.HeldHorizontalIntent = protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT
		cp.Facing = protocolv1.Facing_FACING_RIGHT
	default:
		cp.HeldHorizontalIntent = protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_NONE
	}

	knockback := tc.Tick < st.knockUntilTk

	// --- 2. Horizontal velocity ------------------------------------------
	// Grounded: intent * run_speed. Airborne: same scaled by air control
	// (RoundDiv once at the parameter boundary, contract §2.3).
	var vx int64
	if knockback {
		vx = int64(cp.Vx) // forced velocity holds for the bound
	} else if cp.IsGrounded {
		vx = int64(dir) * int64(cp.RunSpeedMmS)
	} else {
		vx = geometry.RoundDiv(int64(dir)*int64(cp.RunSpeedMmS)*int64(cp.AirControlBp), 10000)
	}

	// --- 3. Vertical velocity ---------------------------------------------
	vy := int64(cp.Vy)
	if cp.IsGrounded && !knockback {
		vy = 0 // resting on ground skips gravity
	} else {
		vy -= geometry.RoundDiv(int64(cp.GravityMmS2), 20)
		if vy < -int64(cp.MaxFallMmS) {
			vy = -int64(cp.MaxFallMmS)
		}
	}
	if jump && !knockback {
		if cp.IsGrounded {
			vy = int64(cp.FirstJumpMmS)
			cp.JumpCount = 1
		} else if cp.JumpCount < MaxJumpCount {
			vy = int64(cp.SecondJumpMmS)
			cp.JumpCount++
		}
		// else: third jump press ignored, no state change
	}
	if drop && cp.IsGrounded && st.onOneWay {
		cp.DropIgnorePlatformID = uint64(st.groundSeg)
		cp.DropIgnoreUntilTick = tc.Tick + DropIgnoreTicks
	}

	// --- 4-6. Sweeps -------------------------------------------------------
	dx := geometry.RoundDiv(vx, 20)
	dy := geometry.RoundDiv(vy, 20)
	box := charBox(int64(cp.X), int64(cp.Y))
	res := s.world.ResolveMove(box, dx, dy, collision.MoveOpts{
		StepHeightMM:         int64(cp.MaxStepHeightMm),
		DropIgnorePlatformID: int64(cp.DropIgnorePlatformID),
		DropIgnoreUntilTick:  cp.DropIgnoreUntilTick,
		Tick:                 tc.Tick,
	})

	if reason, illegal := res.Correction(); illegal {
		// The input position already penetrated blocking geometry: reject
		// to the last safe state and tell the owner (contract §5.4).
		s.recover(p, e, st, tc, protocolv1.MovementCorrectionReason(reason))
		return
	}

	finalX := res.Final.MinX + CharHalfWidthMm
	finalY := res.Final.MinY
	if !s.inBounds32(finalX, finalY) {
		s.recover(p, e, st, tc, protocolv1.MovementCorrectionReason_MOVEMENT_CORRECTION_REASON_FORCED)
		return
	}

	if res.VxZeroed {
		vx = 0
	}
	if res.VyZeroed {
		vy = 0
	}
	// A one-way platform inside its drop-ignore window gives no support:
	// ResolveMove's ground re-eval keeps the contact flag for the sweep it
	// just performed, so the ignore applies at the grounded decision here.
	grounded := res.Grounded
	if grounded && res.OnOneWay &&
		cp.DropIgnorePlatformID != 0 &&
		uint64(res.GroundSegment) == cp.DropIgnorePlatformID &&
		tc.Tick < cp.DropIgnoreUntilTick {
		grounded = false
	}
	if grounded {
		vy = 0
		cp.JumpCount = 0
		st.lastSafeX = finalX
		st.lastSafeY = finalY
		st.hasLastSafe = true
	}
	st.onOneWay = res.OnOneWay
	if grounded && res.GroundSegment >= 0 {
		cp.PlatformID = uint64(res.GroundSegment)
		st.groundSeg = res.GroundSegment
	} else if !grounded {
		cp.PlatformID = 0
		st.groundSeg = -1
	}

	// Expired drop-ignore clears once its tick window passed.
	if cp.DropIgnoreUntilTick != 0 && tc.Tick >= cp.DropIgnoreUntilTick {
		cp.DropIgnorePlatformID = 0
		cp.DropIgnoreUntilTick = 0
	}

	// --- Transitions (movement.md § Transitions) --------------------------
	var state protocolv1.MovementState
	switch {
	case knockback:
		state = protocolv1.MovementState_MOVEMENT_STATE_KNOCKBACK
	case grounded:
		st.knockUntilTk = 0 // landing ends knockback early
		if vx != 0 {
			state = protocolv1.MovementState_MOVEMENT_STATE_RUN
		} else {
			state = protocolv1.MovementState_MOVEMENT_STATE_IDLE
		}
	default:
		if vy > 0 {
			state = protocolv1.MovementState_MOVEMENT_STATE_JUMP
		} else {
			state = protocolv1.MovementState_MOVEMENT_STATE_FALL
		}
	}

	// --- Writes ------------------------------------------------------------
	cp.X = int32(finalX)
	cp.Y = int32(finalY)
	cp.Vx = int32(vx)
	cp.Vy = int32(vy)
	cp.IsGrounded = grounded
	cp.MovementState = state
	e.Snap.Vx = int32(vx)
	e.Snap.Vy = int32(vy)
	e.Snap.Facing = cp.Facing
	e.Snap.MovementState = state
	_ = p.SetPosition(e.ID, int32(finalX), int32(finalY))
}

// inBounds reports whether the mid-feet point (x, y) lies inside the
// geometry bounds; the check uses the canonical space rectangle.
func (s *System) inBounds(x, y int32) bool {
	g := s.world.Geometry()
	return x >= 0 && y >= 0 && int64(x) <= g.BoundsMM.MaxX && int64(y) <= g.BoundsMM.MaxY
}

func (s *System) inBounds32(x, y int64) bool {
	g := s.world.Geometry()
	return x >= 0 && y >= 0 && x <= g.BoundsMM.MaxX && y <= g.BoundsMM.MaxY
}
