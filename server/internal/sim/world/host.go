package world

import (
	"context"
	"sync/atomic"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// worldPlayer is the host-side bookkeeping of one member: entity handle,
// death/respawn progress and the last committed respawn outcome for
// retry-replay (the committed-outcome rule).
type worldPlayer struct {
	EntityID       uint64
	Frozen         bool
	RespawnAtTick  uint64 // dead players respawn at/after this tick
	RespawnEmitted bool
	Respawn        *protocolv1.S2CRespawn // committed outcome for retry
}

// npcInst is one spawned NPC instance on this channel.
type npcInst struct {
	EntityID uint64
	Def      NpcDef
}

// ChannelHost owns the *runtime.Partition of one running channel plus the
// mailbox, sessions and member bookkeeping its drain applies. The drain
// runs as the PhaseExternalResults system (commands + consults in FIFO
// order) inside the partition's tick — never on edge goroutines.
type ChannelHost struct {
	w     *Runtime
	mapID string
	ch    uint32

	part   *runtime.Partition
	mb     *Mailbox
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // closed when the partition loop exits

	players   map[id.UUID]*worldPlayer
	npcs      map[uint64]*npcInst
	sessions  *npcSessions
	respawned map[[16]byte]*protocolv1.S2CRespawn // committed 207 by operation_id

	pendingDurable atomic.Int64
}

// Mailbox is the intake edge/world posts commands and consults into.
func (h *ChannelHost) Mailbox() *Mailbox { return h.mb }

// Partition exposes the owning partition (loader, source events, tick).
func (h *ChannelHost) Partition() *runtime.Partition { return h.part }

// drainMailbox is the PhaseExternalResults system: pulls every queued
// entry in FIFO order and applies it inside the partition tick.
func (h *ChannelHost) drainMailbox(p *runtime.Partition, tc *runtime.TickContext) {
	for {
		e, ok := h.mb.Take()
		if !ok {
			return
		}
		if e.Fn != nil {
			e.Fn(p, tc)
			continue
		}
		if e.Con != nil {
			h.answerConsult(e.Con, p)
			continue
		}
		h.applyCommand(e.Cmd, p, tc)
	}
}

// emit sends one world-control message to a character's session — the
// session-level send is host-agnostic, so it routes through the runtime.
func (h *ChannelHost) emit(characterID id.UUID, msgID uint32, msg proto.Message) {
	h.w.emit(characterID, msgID, msg)
}

// applyCommand dispatches one drained mailbox command.
func (h *ChannelHost) applyCommand(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
	switch cmd.Kind {
	case CmdInteract:
		h.applyInteract(cmd, p, tc)
	case CmdPortal:
		h.applyPortal(cmd, p, tc)
	case CmdChannelSwitch:
		h.applyChannelSwitch(cmd, p, tc)
	case CmdRespawn:
		h.applyRespawn(cmd, p, tc)
	case CmdTransferStart:
		h.applyTransferStart(cmd, p, tc)
	case CmdPresentationReady:
		h.w.onPresentationReady(h, cmd, p, tc)
	case CmdPlayerLeave:
		h.removePlayer(cmd.CharacterID, p)
	}
}

// removePlayer despawns the member entity, closes sessions, and frees the
// director membership/occupancy.
func (h *ChannelHost) removePlayer(characterID id.UUID, p *runtime.Partition) {
	if pl, ok := h.players[characterID]; ok {
		_ = p.Remove(pl.EntityID)
		delete(h.players, characterID)
	}
	h.sessions.CloseCharacter(characterID)
	h.w.director.OnPlayerLeave(characterID)
}

// answerConsult resolves one ADR-0083 read-only consult inside the drain.
// Consults never mutate and never journal; every answer is a single reply.
func (h *ChannelHost) answerConsult(con *Consult, p *runtime.Partition) {
	switch con.Kind {
	case ConsultPartitionTick:
		con.Reply <- ConsultReply{OK: true, Tick: p.TickN()}
	case ConsultNpcServiceValid:
		h.answerNpcService(con, p)
	case ConsultPlacementPortal:
		h.answerPortal(con, p)
	case ConsultPlacementChannel:
		h.answerChannel(con, p)
	}
}

// playerEntity resolves a member's live entity for consult reads.
func (h *ChannelHost) playerEntity(characterID id.UUID, p *runtime.Partition) (*worldPlayer, *runtime.Entity, bool) {
	pl, ok := h.players[characterID]
	if !ok {
		return nil, nil, false
	}
	e, err := p.Entity(pl.EntityID)
	if err != nil {
		return nil, nil, false
	}
	return pl, e, true
}
