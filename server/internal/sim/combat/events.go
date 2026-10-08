package combat

import (
	"google.golang.org/protobuf/proto"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/replication"
	"thinhthan/internal/sim/runtime"
)

// emitStarted emits S2C_ACTION_STARTED (203) with the full timing
// contract of messages.md.
func (s *System) emitStarted(src *runtime.Entity, act *Action, tc *runtime.TickContext) {
	msg := &protocolv1.S2CActionStarted{
		ActionInstanceId:   act.ID,
		SourceEntityId:     src.ID,
		SkillId:            act.SkillID,
		ClientSeq:          act.ClientSeq,
		ServerTick:         tc.Tick,
		Facing:             act.Facing,
		TargetEntityId:     act.Target,
		AreaCenterXMm:      act.AreaX,
		AreaCenterYMm:      act.AreaY,
		CastMs:             act.CastMs,
		CooldownEndsAtTick: act.CooldownEnd,
		MpAfter:            src.Private.CurrentMP,
		ActiveStartsAtTick: act.ActiveStart,
		ActiveEndsAtTick:   act.ActiveEnd,
		RecoveryEndsAtTick: act.RecoveryEnd,
		MotionEndsAtTick:   act.MotionEnd,
	}
	s.enqueue(src.ID, idS2CActionStarted, msg)
}

// reject emits S2C_ACTION_REJECTED (204): client_seq echo,
// request_message_id, operation_id (208 only), skill_id, error_code.
func (s *System) reject(src *runtime.Entity, req Request, code protocolv1.ErrorCode) {
	msg := &protocolv1.S2CActionRejected{
		ClientSeq:        req.ClientSeq,
		RequestMessageId: req.RequestWireID,
		OperationId:      req.OperationID,
		SkillId:          req.SkillID,
		ErrorCode:        code,
	}
	s.enqueue(src.ID, idS2CActionRejected, msg)
}

// emitCombat emits S2C_COMBAT_EVENT (304), delivery class
// AUTHORITATIVE.
func (s *System) emitCombat(p *runtime.Partition, ev *protocolv1.S2CCombatEvent) {
	// Events fan to both parties' viewers; the replication layer owns
	// interest management — combat enqueues to source and target.
	s.enqueue(ev.SourceEntityId, idS2CCombatEvent, ev)
	if ev.TargetEntityId != ev.SourceEntityId {
		s.enqueue(ev.TargetEntityId, idS2CCombatEvent, ev)
	}
}

// emitDeath emits S2C_DEATH (206): respawn_available_at_tick carries
// the normal-world RESPAWN_DELAY floor (server-driven content may
// override through the respawn lane).
func (s *System) emitDeath(e *runtime.Entity, killer uint64, tc *runtime.TickContext) {
	msg := &protocolv1.S2CDeath{
		EntityId:               e.ID,
		KillerEntityId:         killer,
		ServerTick:             tc.Tick,
		RespawnAvailableAtTick: tc.Tick + respawnDelayTks,
	}
	s.enqueue(e.ID, idS2CDeath, msg)
}

// enqueue routes a replication message through the injected Outbound
// port; a nil port runs the sim headless (tests).
func (s *System) enqueue(to uint64, id uint32, msg proto.Message) {
	if s.cfg.Outbound == nil {
		return
	}
	_ = s.cfg.Outbound.Enqueue(runtime.Outbound{
		To:        to,
		MessageID: id,
		Class:     replication.DeliveryAuthoritativeEvent,
		Msg:       msg,
	})
}
