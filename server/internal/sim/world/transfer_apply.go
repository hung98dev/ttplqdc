package world

import (
	"context"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// onPresentationReady handles the client's 106: the frozen entity leaves
// the source partition and the destination admits it — the new channel's
// baseline carries the client back to IN_WORLD.
func (w *Runtime) onPresentationReady(h *ChannelHost, cmd *Command,
	p *runtime.Partition, tc *runtime.TickContext) {
	t, ok := w.director.transferFor(cmd.CharacterID)
	if !ok || [16]byte(t.TransferID) != cmd.TransferID {
		return // stale 106 for an unknown or superseded transfer
	}
	kind := t.Kind
	h.removeMemberKeepLedger(cmd.CharacterID, p)
	w.admitPlayer(cmd.CharacterID, PlacementResult{
		MapID:        t.DstMap,
		ChannelIndex: t.DstChannel,
	}, &Command{
		Kind:          CmdTransferStart,
		CharacterID:   cmd.CharacterID,
		OperationID:   cmd.OperationID,
		SpawnAnchorID: t.DstSpawn,
	})
	if kind == protocolv1.TransferReason_TRANSFER_REASON_CHANNEL_SWITCH {
		w.director.MarkSwitch(cmd.CharacterID)
	}
	_ = tc
}

// recoverTransfer runs source recovery on an expired transfer: the
// member is re-placed at its checkpoint (checkpoint respawn of
// placement, forced order) and a 105 FORCED re-arms the client-side
// transfer machine.
func (w *Runtime) recoverTransfer(t *transfer) {
	charID := t.CharacterID
	src, hasSrc := w.hostFor(t.SrcMap, t.SrcChannel)
	cp, err := w.LoadCheckpoint(context.Background(), charID)
	if err != nil {
		return // durable read failed; retry on next tick
	}
	w.director.completeTransfer(charID)
	if hasSrc {
		src.removeMemberKeepLedgerQueued(charID)
	}
	res, pw, err := w.director.Select(PlacementRequest{
		CharacterID:   charID,
		MapID:         cp.MapID,
		Kind:          PlaceForced,
		ForcedReason:  ForcedRecovery,
		SpawnAnchorID: cp.AnchorID,
	})
	if err != nil {
		return
	}
	if pw != nil {
		pw.RequestMessageID = 0
		w.registerPending(pw, func(w *Runtime, r PlacementResult) {
			w.forceAdmit(charID, r, cp.AnchorID)
		})
		w.emitPending(charID, pw)
		return
	}
	w.forceAdmit(charID, res, cp.AnchorID)
}

// forceAdmit emits a 105 FORCED so the client-side transfer machine arms,
// then admits the member at the resolved spawn.
func (w *Runtime) forceAdmit(characterID id.UUID, res PlacementResult, spawnAnchorID string) {
	var zero [16]byte
	w.startTransfer(characterID, zero,
		protocolv1.TransferReason_TRANSFER_REASON_FORCED,
		res.MapID, res.ChannelIndex, spawnAnchorID, res.MapID, res.ChannelIndex)
	// The 105 is delivered on the character's session; admission proceeds
	// immediately (the client ack cycle reuses the transfer machine).
	w.admitPlayer(characterID, res, &Command{
		Kind:          CmdTransferStart,
		CharacterID:   characterID,
		SpawnAnchorID: spawnAnchorID,
	})
}
