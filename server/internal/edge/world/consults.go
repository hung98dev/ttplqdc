// Package world binds the world/placement message ids on the edge:
// 103 (interact → interaction.npc_service), 104 (portal → placement.portal),
// 109 (channel switch → placement.channel) ride the ADR-0083 route —
// admission consult on the sim/world mailbox, JournalClientCommand Submit,
// durable commit, recorded client_result, post-commit world command for
// ids with a partition-side effect — and 208 (respawn) rides the ADR-0082
// router non-durable table straight into the same mailbox. Consults here
// is also the read-only port sibling edge/* packages declare; composition
// wires it in app/.
package world

import (
	"context"
	"errors"
	"strconv"
	"time"

	"thinhthan/internal/core/id"
	simworld "thinhthan/internal/sim/world"
	protocolv1 "thinhthan/internal/protocol/v1"
)

var (
	// ErrConsultTimeout: the bounded await elapsed with no reply —
	// admission fails closed (session/state-class error), never fabricated.
	ErrConsultTimeout = errors.New("world: consult timeout")
	// ErrConsultUnbound: no world runtime is wired — fail closed.
	ErrConsultUnbound = errors.New("world: consult surface unbound")
)

// Consults is the ADR-0083 read-only consult port. Replies are emitted by
// the authoritative site only (owning partition drain for partition state,
// world runtime for Director/channel state); a false OK or a transport
// error both reject admission — never a fabricated verdict.
type Consults interface {
	// PartitionTick returns the owning partition's tick for a member.
	// ok=false: no live partition admits the character.
	PartitionTick(ctx context.Context, characterID id.UUID) (tick uint64, ok bool, err error)
	// NpcServiceValid validates a dedicated NPC service admission
	// (session open + service in the NPC's allowed set + 2.5m range +
	// not dead + not in_combat). npcID is the wire npc_id/target_id.
	NpcServiceValid(ctx context.Context, characterID id.UUID, npcID, serviceID string) (simworld.ConsultReply, error)
	// TalkAdmission validates a TALK admission (range/dead/combat gates;
	// the world command opens the volatile NPC session post-commit).
	TalkAdmission(ctx context.Context, characterID id.UUID, npcID string) (simworld.ConsultReply, error)
	// PortalAdmission validates a 104 portal use (portal exists on the
	// current map, in range, level requirement, destination capacity).
	PortalAdmission(ctx context.Context, characterID id.UUID, portalID string) (simworld.ConsultReply, error)
	// ChannelAdmission validates a 109 channel switch (target exists,
	// not current, cooldown elapsed, destination below soft cap).
	ChannelAdmission(ctx context.Context, characterID id.UUID, targetChannel uint32) (simworld.ConsultReply, error)
}

// consultTimeout bounds every admission await.
const consultTimeout = 2 * time.Second

// runtimeConsults backs Consults with mailbox entries on the world
// runtime (ADR-0083 transport: second closed entry kind, FIFO with
// commands, single-shot reply channel).
type runtimeConsults struct {
	w       *simworld.Runtime
	timeout time.Duration
}

// NewConsults builds the consult port over the world runtime.
func NewConsults(w *simworld.Runtime) Consults {
	return &runtimeConsults{w: w, timeout: consultTimeout}
}

// npcEntity parses the wire npc_id into the sim's entity key; malformed
// ids answer TARGET_INVALID locally without consuming a mailbox slot.
func npcEntity(npcID string) (uint64, error) {
	return strconv.ParseUint(npcID, 10, 64)
}

func (rc *runtimeConsults) ask(ctx context.Context, c simworld.Consult) (simworld.ConsultReply, error) {
	if rc.w == nil {
		return simworld.ConsultReply{}, ErrConsultUnbound
	}
	reply := make(chan simworld.ConsultReply, 1)
	c.Reply = reply
	if err := rc.w.Consult(c); err != nil {
		return simworld.ConsultReply{}, err
	}
	select {
	case rep := <-reply:
		return rep, nil
	case <-ctx.Done():
		return simworld.ConsultReply{}, ctx.Err()
	case <-time.After(rc.timeout):
		return simworld.ConsultReply{}, ErrConsultTimeout
	}
}

func (rc *runtimeConsults) PartitionTick(ctx context.Context, characterID id.UUID) (uint64, bool, error) {
	rep, err := rc.ask(ctx, simworld.Consult{
		Kind:        simworld.ConsultPartitionTick,
		CharacterID: characterID,
	})
	if err != nil {
		return 0, false, err
	}
	return rep.Tick, rep.OK, nil
}

func (rc *runtimeConsults) NpcServiceValid(ctx context.Context, characterID id.UUID, npcID, serviceID string) (simworld.ConsultReply, error) {
	npc, err := npcEntity(npcID)
	if err != nil {
		return simworld.ConsultReply{OK: false, Code: protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID}, nil
	}
	return rc.ask(ctx, simworld.Consult{
		Kind:        simworld.ConsultNpcServiceValid,
		CharacterID: characterID,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:    npc,
		ServiceID:   serviceID,
	})
}

func (rc *runtimeConsults) TalkAdmission(ctx context.Context, characterID id.UUID, npcID string) (simworld.ConsultReply, error) {
	npc, err := npcEntity(npcID)
	if err != nil {
		return simworld.ConsultReply{OK: false, Code: protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID}, nil
	}
	return rc.ask(ctx, simworld.Consult{
		Kind:         simworld.ConsultNpcServiceValid,
		CharacterID:  characterID,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_TALK),
		TargetID:     npc,
	})
}

func (rc *runtimeConsults) PortalAdmission(ctx context.Context, characterID id.UUID, portalID string) (simworld.ConsultReply, error) {
	return rc.ask(ctx, simworld.Consult{
		Kind:        simworld.ConsultPlacementPortal,
		CharacterID: characterID,
		PortalID:    portalID,
	})
}

func (rc *runtimeConsults) ChannelAdmission(ctx context.Context, characterID id.UUID, targetChannel uint32) (simworld.ConsultReply, error) {
	return rc.ask(ctx, simworld.Consult{
		Kind:          simworld.ConsultPlacementChannel,
		CharacterID:   characterID,
		TargetChannel: targetChannel,
	})
}
