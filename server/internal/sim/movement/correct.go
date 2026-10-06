package movement

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/replication"
	"thinhthan/internal/sim/runtime"
)

// emit107 sends S2C_MOVEMENT_CORRECTION to the entity's owner through the
// injected OutboundPort. The replication delivery tables cover only ids
// 300-308, so corrections emit a literal MessageID with
// DeliveryAuthoritativeEvent ordering (F-03 — deliberate, no runtime
// edits). A nil port disables emission (sim-only tests).
func (s *System) emit107(p *runtime.Partition, e *runtime.Entity, tc *runtime.TickContext, reason protocolv1.MovementCorrectionReason) {
	if s.outbound == nil {
		return
	}
	msg := &protocolv1.S2CMovementCorrection{
		LastProcessedClientSeq: s.seqOf(e.ID),
		ServerTick:             tc.Tick,
		Checkpoint:             checkpointPB(&e.Checkpoint),
		Reason:                 reason,
	}
	_ = s.outbound.Enqueue(runtime.Outbound{
		To:        e.ID,
		MessageID: 107,
		Class:     replication.DeliveryAuthoritativeEvent,
		Msg:       msg,
	})
}

// checkpointPB materializes the wire MovementCheckpoint from the flat
// snapshot. Called only on correction paths — never inside the per-tick
// hot loop — so the allocation is off the zero-alloc budget.
func checkpointPB(c *replication.CheckpointSnapshot) *protocolv1.MovementCheckpoint {
	return &protocolv1.MovementCheckpoint{
		XMm:                  c.X,
		YMm:                  c.Y,
		VxMmS:                c.Vx,
		VyMmS:                c.Vy,
		Facing:               c.Facing,
		MovementState:        c.MovementState,
		PlatformId:           c.PlatformID,
		IsGrounded:           c.IsGrounded,
		JumpCount:            c.JumpCount,
		DropIgnorePlatformId: c.DropIgnorePlatformID,
		DropIgnoreUntilTick:  c.DropIgnoreUntilTick,
		HeldHorizontalIntent: c.HeldHorizontalIntent,
		EffectiveParameters: &protocolv1.EffectiveMovementParameters{
			RunSpeedMmS:     c.RunSpeedMmS,
			FirstJumpMmS:    c.FirstJumpMmS,
			SecondJumpMmS:   c.SecondJumpMmS,
			GravityMmS2:     c.GravityMmS2,
			MaxFallMmS:      c.MaxFallMmS,
			AirControlBp:    c.AirControlBp,
			MaxStepHeightMm: c.MaxStepHeightMm,
		},
	}
}
