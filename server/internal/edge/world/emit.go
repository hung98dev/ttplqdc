package world

import (
	"context"
	"time"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	durableworld "thinhthan/internal/durable/world"
	"thinhthan/internal/sim/runtime"
	simworld "thinhthan/internal/sim/world"
)

// Emitter is the EmitPort adapter the composition root installs on the
// world runtime: it translates the partition's flat durable commands into
// JournalClientCommand-free records. IMP-018's only emitted family is
// sim.checkpoint (ProducerCheckpoint — JournalOutcome.command.checkpoint
// field 25); the typed payload is fetched from the runtime's ledger and
// dropped only after the record is accepted.
type Emitter struct {
	w   *simworld.Runtime
	q   *queue.Queue
	now func() time.Time
}

// NewEmitter builds the adapter over the world runtime + durable queue.
func NewEmitter(w *simworld.Runtime, q *queue.Queue) *Emitter {
	return &Emitter{w: w, q: q, now: func() time.Time { return time.Now().UTC() }}
}

// Emit implements runtime.EmitPort. An error means not accepted — the
// partition keeps the command pending and retries; payloads that cannot
// be materialized are dropped so the partition is never wedged on a
// write whose ledger entry is already gone (restart).
func (e *Emitter) Emit(ctx context.Context, cmd runtime.DurableCommand) error {
	if cmd.Kind != runtime.CmdCheckpoint || cmd.Family != simworld.FamilySimCheckpoint {
		return nil // world partitions emit only sim.checkpoint
	}
	op := [16]byte(cmd.OperationID)
	cp, ok := e.w.CheckpointPayload(op)
	if !ok {
		return nil // ledger entry already consumed (replay after restart)
	}
	rec, err := durableworld.CheckpointRecord(
		id.UUID(cmd.OperationID), cp.CharacterID, cmd.Fingerprint,
		cmd.SourceEvent, cmd.ContentRevision,
		checkpointJournal(cp), e.now())
	if err != nil {
		return nil // unrecoverable translate — drop rather than retry forever
	}
	if err := e.q.Submit(ctx, rec); err != nil {
		return err
	}
	e.w.DropCheckpointPayload(op)
	return nil
}

// checkpointJournal converts the sim-side value into the durable payload.
func checkpointJournal(cp *simworld.CheckpointWrite) *journalv1.JournalCheckpoint {
	j := &journalv1.JournalCheckpoint{
		CharacterId:     cp.CharacterID[:],
		OwnershipEpoch:  cp.OwnershipEpoch,
		CheckpointId:    cp.CheckpointID,
		SafeMapId:       cp.SafeMapID,
		EntrySpawnId:    cp.EntrySpawnID,

		MembershipState: cp.MembershipState,
		RecordedAtMs:    cp.RecordedAtMs,
		SourceTick:      cp.SourceTick,
	}
	if cp.TransferID != nil {
		j.TransferId = cp.TransferID[:]
	}
	if cp.SourceMapID != "" {
		src := cp.SourceMapID
		j.SourceMapId = &src
	}
	if cp.SourceChannelID != 0 {
		sch := cp.SourceChannelID
		j.SourceChannelId = &sch
	}
	return j
}
