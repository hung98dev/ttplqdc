package world

import (
	"encoding/hex"
	"strconv"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
)

// applyInteract is the post-commit world effect of a durable 103: TALK
// opens the gameplay-capable session; NPC_SERVICE refreshes it; CAST and
// HOOK route to the channel's fishing delegates (inert until IMP-058
// binds them). The durable write itself (set_checkpoint) already
// committed — the partition effect is domain dispatch only (ADR-0083).
func (h *ChannelHost) applyInteract(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
	switch protocolv1.InteractKind(cmd.InteractKind) {
	case protocolv1.InteractKind_INTERACT_KIND_TALK:
		if _, ok := h.npcs[cmd.TargetID]; ok {
			h.sessions.Open(cmd.TargetID, cmd.CharacterID, tc.Tick)
		}
	case protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE:
		h.sessions.Touch(cmd.TargetID, cmd.CharacterID, tc.Tick)
	case protocolv1.InteractKind_INTERACT_KIND_CAST:
		h.fishing.Cast(cmd, p, tc)
	case protocolv1.InteractKind_INTERACT_KIND_HOOK:
		h.fishing.Hook(cmd, p, tc)
	}
}

// applyPortal is the post-commit effect of a durable 104: freeze the
// member, resolve the destination under forced semantics (the client
// verdict already committed), emit 105, and register the transfer.
func (h *ChannelHost) applyPortal(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
	pl, pe, ok := h.playerEntity(cmd.CharacterID, p)
	if !ok {
		return
	}
	pd, ok := h.w.cfg.Maps.Portal(cmd.PortalID)
	if !ok || pd.SourceMap != h.mapID {
		return
	}
	req := PlacementRequest{
		CharacterID:   cmd.CharacterID,
		MapID:         pd.DestMap,
		Kind:          PlaceForced,
		ForcedReason:  ForcedReattach,
		SpawnAnchorID: pd.DestSpawn,
	}
	res, pw, err := h.w.director.Select(req)
	if err != nil || pw != nil {
		if pw != nil {
			pw.RequestMessageID = 0 // server-driven pending
			pw.Reason = PendingReconnect
			h.w.registerPending(pw, func(w *Runtime, r PlacementResult) {
				w.startTransfer(cmd.CharacterID, cmd.OperationID,
					protocolv1.TransferReason_TRANSFER_REASON_PORTAL,
					pd.DestMap, r.ChannelIndex, pd.DestSpawn, h.mapID, h.ch)
			})
			h.emitPending(cmd.CharacterID, pw)
		}
		return
	}
	pl.Frozen = true
	_ = pe
	h.w.startTransfer(cmd.CharacterID, cmd.OperationID,
		protocolv1.TransferReason_TRANSFER_REASON_PORTAL,
		pd.DestMap, res.ChannelIndex, pd.DestSpawn, h.mapID, h.ch)
}

// applyChannelSwitch is the post-commit effect of a durable 109: same
// map, resolved target channel, CHANNEL_SWITCH reason.
func (h *ChannelHost) applyChannelSwitch(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
	pl, _, ok := h.playerEntity(cmd.CharacterID, p)
	if !ok {
		return
	}
	req := PlacementRequest{
		CharacterID:  cmd.CharacterID,
		MapID:        h.mapID,
		Kind:         PlaceForced,
		Preferred:    cmd.TargetChannel,
		ForcedReason: ForcedReattach,
	}
	res, pw, err := h.w.director.Select(req)
	if err != nil || pw != nil {
		if pw != nil {
			pw.Reason = PendingReconnect
			h.w.registerPending(pw, func(w *Runtime, r PlacementResult) {
				w.startTransfer(cmd.CharacterID, cmd.OperationID,
					protocolv1.TransferReason_TRANSFER_REASON_CHANNEL_SWITCH,
					h.mapID, r.ChannelIndex, "", h.mapID, h.ch)
			})
			h.emitPending(cmd.CharacterID, pw)
		}
		return
	}
	pl.Frozen = true
	h.w.startTransfer(cmd.CharacterID, cmd.OperationID,
		protocolv1.TransferReason_TRANSFER_REASON_CHANNEL_SWITCH,
		h.mapID, res.ChannelIndex, "", h.mapID, h.ch)
}

// startTransfer freezes bookkeeping on the director and emits the 105 on
// the character's session. Transfer-id minting is deterministic from the
// durable operation id (replay-safe).
func (w *Runtime) startTransfer(characterID id.UUID, opID [16]byte,
	reason protocolv1.TransferReason, dstMap string, dstCh uint32,
	dstSpawn string, srcMap string, srcCh uint32) {
	tid := id.ServerJobOperationID("sim.transfer",
		hex.EncodeToString(opID[:]), dstMap, strconv.Itoa(int(dstCh)))
	w.director.beginTransfer(characterID, id.UUID(tid), reason,
		dstMap, dstCh, dstSpawn)
	msg := &protocolv1.S2CTransferPrepare{
		TransferId:      tid[:],
		Reason:          reason,
		MapId:           dstMap,
		ChannelIndex:    dstCh,
		ContentRevision: w.cfg.ContentRevision,
		ReadyDeadlineMs: uint32(TransferBudgetWorld.Milliseconds()),
	}
	// The 105 rides the character's session — never the source host's —
	// so it is emitted even when the source channel is already gone or
	// the destination has not started (forced re-placement path).
	w.emit(characterID, MsgS2CTransferPrepare, msg)
}

// emitPending sends the S2C_PLACEMENT_PENDING (15) notice.
func (h *ChannelHost) emitPending(characterID id.UUID, pw *PendingWait) {
	h.emit(characterID, MsgS2CPlacementPending, &protocolv1.S2CPlacementPending{
		RequestMessageId: pw.RequestMessageID,
		Reason:           protocolv1.PlacementReason(pw.Reason),
		RetryAfterMs:     pw.RetryAfterMs,
	})
}

// applyTransferStart admits the member entity into this partition at the
// resolved spawn. Used for first attach, transfer completion and forced
// placement resolution; a non-nil Respawn payload emits the committed
// 207 and queues the sim.checkpoint durable write.
func (h *ChannelHost) applyTransferStart(cmd *Command, p *runtime.Partition, tc *runtime.TickContext) {
	if _, exists := h.players[cmd.CharacterID]; exists {
		return
	}
	eid, err := p.Admit(runtime.ClassPlayer)
	if err != nil {
		return // occupancy accounting guarantees the cap cannot be hit
	}
	e, err := p.Entity(eid)
	if err != nil {
		return
	}
	x, y := cmd.SpawnX, cmd.SpawnY
	anchorID := cmd.SpawnAnchorID
	if anchorID == "" {
		if rec, ok := h.w.cfg.Maps.Lookup(h.mapID); ok {
			anchorID = rec.EntrySpawn
		}
	}
	if a, ok := h.w.anchorFor(h.mapID, anchorID); anchorID != "" && ok {
		x, y = int32(a.X), int32(a.Y)
	}
	_ = p.SetPosition(eid, x, y)
	e.Snap.CharacterID = cmd.CharacterID
	e.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_PLAYER
	if cmd.Vitals.MaxHP > 0 {
		e.Snap.MaxHP = cmd.Vitals.MaxHP
	}
	if cmd.Vitals.HP > 0 {
		e.Snap.HP = cmd.Vitals.HP
	} else if e.Snap.MaxHP > 0 {
		e.Snap.HP = e.Snap.MaxHP
	}
	if cmd.Vitals.MaxMP > 0 {
		e.Private.MaxMP = cmd.Vitals.MaxMP
	}
	if cmd.Vitals.MP > 0 {
		e.Private.CurrentMP = cmd.Vitals.MP
	}
	h.players[cmd.CharacterID] = &worldPlayer{EntityID: eid}
	h.w.director.Admitted(cmd.CharacterID, h.mapID, h.ch)
	if t, ok := h.w.director.transferFor(cmd.CharacterID); ok {
		// Transfer completion moves the member's resident map — the
		// durable write rides sim.checkpoint under the transfer id (the
		// receipt the placement portal already committed points here).
		tid := [16]byte(t.TransferID)
		h.w.commitCheckpointWrite(p, tid, &CheckpointWrite{
			CharacterID:     cmd.CharacterID,
			SafeMapID:       h.mapID,
			EntrySpawnID:    cmd.SpawnAnchorID,
			TransferID:      &t.TransferID,
			SourceMapID:     t.SrcMap,
			SourceChannelID: t.SrcChannel,
			MembershipState: "world",
			RecordedAtMs:    h.w.cfg.Now().UnixMilli(),
			SourceTick:      tc.Tick,
		})
	}
	h.w.director.completeTransfer(cmd.CharacterID)
	if cmd.Respawn != nil {
		h.finishRespawn(cmd, p, tc, eid, e)
	}
}

// anchorFor resolves a named anchor on a map.
func (w *Runtime) anchorFor(mapID, anchorID string) (geometry.Anchor, bool) {
	rec, ok := w.cfg.Maps.Lookup(mapID)
	if !ok {
		return geometry.Anchor{}, false
	}
	return rec.Anchor(anchorID)
}
