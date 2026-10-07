package world

import (
	"context"
	"math"

	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/runtime"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// applyRespawn is the ADR-0082 208 drain: only a DEAD member whose
// respawn delay elapsed may respawn; success settles via
// QueueCommand/EMIT_DURABLE_COMMANDS (sim.checkpoint preserving the
// client operation_id) plus the committed 207 — a same-operation_id
// retry re-sends the committed outcome. A dead member pending respawn
// stays DEAD until its forced placement resolves (test
// TestPendingRespawnStaysDead).
func (h *ChannelHost) applyRespawn(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
	charID := cmd.CharacterID
	// Committed-outcome rule: a retry with the same operation_id after
	// success re-sends the committed 207.
	if committed, ok := h.respawned[cmd.OperationID]; ok {
		h.emit(charID, MsgS2CRespawn, committed)
		return
	}
	pl, e, ok := h.playerEntity(charID, p)
	reject := func(code protocolv1.ErrorCode) {
		h.emit(charID, MsgS2CActionRejected, &protocolv1.S2CActionRejected{
			RequestMessageId: 208,
			OperationId:      cmd.OperationID[:],
			ErrorCode:        code,
		})
	}
	if !ok || !(e.Dead || e.Snap.HP <= 0) {
		reject(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	if tc.Tick < pl.RespawnAtTick {
		reject(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	cp, err := h.w.LoadCheckpoint(context.Background(), charID)
	if err != nil {
		reject(protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE)
		return
	}
	// Restored vitals: 40% of max, floored (death_respawn.md).
	hpAfter := int64(math.Floor(float64(e.Snap.MaxHP) * 0.4))
	mpAfter := int64(math.Floor(float64(e.Private.MaxMP) * 0.4))
	if hpAfter < 1 {
		hpAfter = 1
	}
	if mpAfter < 0 {
		mpAfter = 0
	}
	preferred := uint32(0)
	if cp.MapID == h.mapID {
		preferred = h.ch
	}
	res, pw, serr := h.w.director.Select(PlacementRequest{
		CharacterID:   charID,
		MapID:         cp.MapID,
		Kind:          PlaceForced,
		Preferred:     preferred,
		ForcedReason:  ForcedRespawn,
		SpawnAnchorID: cp.AnchorID,
	})
	if serr != nil {
		reject(protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE)
		return
	}
	if pw != nil {
		pw.RequestMessageID = 208
		out := &RespawnOutcome{CheckpointID: cp.CheckpointID, HPAfter: hpAfter, MPAfter: mpAfter}
		h.w.registerPending(pw, func(w *Runtime, r PlacementResult) {
			w.respawnAt(charID, cmd.OperationID, r, cp, out, h.mapID, h.ch)
		})
		h.emitPending(charID, pw)
		return
	}
	h.w.respawnAt(charID, cmd.OperationID, res, cp,
		&RespawnOutcome{CheckpointID: cp.CheckpointID, HPAfter: hpAfter, MPAfter: mpAfter},
		h.mapID, h.ch)
}

// respawnAt moves the dead member to the resolved checkpoint channel and
// completes the respawn there (207 + sim.checkpoint durable write).
func (w *Runtime) respawnAt(characterID id.UUID, opID [16]byte, res PlacementResult,
	cp CheckpointDef, out *RespawnOutcome, srcMap string, srcCh uint32) {
	cmd := &Command{
		Kind:          CmdTransferStart,
		CharacterID:   characterID,
		OperationID:   opID,
		MapID:         res.MapID,
		SpawnAnchorID: cp.AnchorID,
		Respawn:       out,
	}
	// Leaving the source channel frees its occupancy; the destination
	// occupancy was reserved by Select.
	if src, ok := w.hostFor(srcMap, srcCh); ok {
		src.removeMemberKeepLedgerQueued(characterID)
	}
	w.admitPlayer(characterID, res, cmd)
}

// removeMemberKeepLedgerQueued despawns the member inside this channel's
// tick — callers on the runtime goroutine post through the mailbox so
// partition state never mutates off-goroutine; drain callers run inline.
func (h *ChannelHost) removeMemberKeepLedgerQueued(characterID id.UUID) {
	if h.inDrain.Load() {
		h.removeMemberKeepLedger(characterID, h.part)
		return
	}
	_ = h.mb.Post(Entry{Fn: func(p *runtime.Partition, _ *runtime.TickContext) {
		h.removeMemberKeepLedger(characterID, p)
	}})
}

// removeMemberKeepLedger despawns the member entity and closes sessions
// while the director keeps the membership row until the destination
// admits (the occupancy free happens here).
func (h *ChannelHost) removeMemberKeepLedger(characterID id.UUID, p *runtime.Partition) {
	if pl, ok := h.players[characterID]; ok {
		_ = p.Remove(pl.EntityID)
		delete(h.players, characterID)
	}
	h.sessions.CloseCharacter(characterID)
	h.w.director.OnPlayerLeave(characterID)
}

// finishRespawn emits the committed 207 and queues the checkpoint durable
// write (sim.checkpoint preserving the client operation_id) once the
// destination partition has admitted the player.
func (h *ChannelHost) finishRespawn(cmd *Command, p *runtime.Partition,
	tc *runtime.TickContext, entityID uint64, e *runtime.Entity) {
	charID := cmd.CharacterID
	out := cmd.Respawn
	e.Snap.HP = out.HPAfter
	e.Private.CurrentMP = out.MPAfter
	msg := &protocolv1.S2CRespawn{
		EntityId:             entityID,
		MapId:                h.mapID,
		CheckpointId:         out.CheckpointID,
		XMm:                  e.Snap.X,
		YMm:                  e.Snap.Y,
		HpAfter:              out.HPAfter,
		MpAfter:              out.MPAfter,
		InvulnerableUntilTick: tc.Tick + InvulnTicks,
		ServerTick:           tc.Tick,
	}
	h.emit(charID, MsgS2CRespawn, msg)
	h.respawned[cmd.OperationID] = msg
	h.w.commitCheckpointWrite(p, cmd.OperationID, &CheckpointWrite{
		CharacterID:     charID,
		CheckpointID:    out.CheckpointID,
		SafeMapID:       h.mapID,
		EntrySpawnID:    cmd.SpawnAnchorID,
		SourceMapID:     h.mapID,
		SourceChannelID: h.ch,
		MembershipState: "respawn",
		RecordedAtMs:    h.w.cfg.Now().UnixMilli(),
		SourceTick:      tc.Tick,
	})
}
