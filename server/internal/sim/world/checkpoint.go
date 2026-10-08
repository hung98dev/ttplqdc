package world

import (
	"context"
	"sync"

	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/runtime"
)

// CheckpointWrite is the sim-side value form of the durable
// sim.checkpoint payload (JournalCheckpoint). The emit adapter converts
// it into the record payload — sim/ never imports the journal protos.
type CheckpointWrite struct {
	CharacterID     id.UUID
	OwnershipEpoch  uint64
	CheckpointID    string
	SafeMapID       string
	EntrySpawnID    string
	TransferID      *id.UUID // nil when the write is not transfer-bound
	SourceMapID     string
	SourceChannelID uint32
	MembershipState string
	RecordedAtMs    int64
	SourceTick      uint64
}

// checkpointWrites holds the typed payload of each emitted
// sim.checkpoint durable command until the flat DurableCommand is
// translated into a DurableCommandRecord by the emit adapter. The world
// is the only producer of this family; entries are keyed by operation id
// and are single-shot.
type checkpointWrites struct {
	mu sync.Mutex
	m  map[[16]byte]*CheckpointWrite
}

func newCheckpointWrites() *checkpointWrites {
	return &checkpointWrites{m: make(map[[16]byte]*CheckpointWrite)}
}

func (c *checkpointWrites) put(op [16]byte, cp *CheckpointWrite) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[op] = cp
}

// Peek returns the payload for op without dropping it — the emit adapter
// drops only after the durable record is accepted, so a failed Submit
// retries with the payload still registered.
func (c *checkpointWrites) Peek(op [16]byte) (*CheckpointWrite, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp, ok := c.m[op]
	return cp, ok
}

// Drop removes the payload after the durable record was accepted.
func (c *checkpointWrites) Drop(op [16]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, op)
}

// CommitCheckpoint registers the payload and queues the flat
// sim.checkpoint durable command on the partition; emission happens in
// EMIT_DURABLE_COMMANDS order at the partition's next tick.
func (w *Runtime) CommitCheckpoint(p *runtime.Partition, op id.UUID,
	cp *CheckpointWrite) error {
	w.checkpointLedger.put([16]byte(op), cp)
	return p.QueueCommand(runtime.DurableCommand{
		Kind:        runtime.CmdCheckpoint,
		Family:      FamilySimCheckpoint,
		OwnerKind:   runtime.OwnerCharacter,
		Owner:       cp.CharacterID,
		OperationID: op,
	})
}

// CheckpointPayload exposes a registered write for the emit adapter.
func (w *Runtime) CheckpointPayload(op [16]byte) (*CheckpointWrite, bool) {
	return w.checkpointLedger.Peek(op)
}

// DropCheckpointPayload retires a registered write after the emit
// adapter's Submit was accepted.
func (w *Runtime) DropCheckpointPayload(op [16]byte) {
	w.checkpointLedger.Drop(op)
}

// LoadCheckpoint resolves the character's durable checkpoint and its safe
// map/anchor through the injected loader port.
func (w *Runtime) LoadCheckpoint(ctx context.Context, characterID id.UUID) (CheckpointDef, error) {
	cpID, _, _, err := w.cfg.Loader.LoadCheckpoint(ctx, characterID)
	if err != nil {
		return CheckpointDef{}, err
	}
	def, ok := w.cfg.Maps.Checkpoint(cpID)
	if !ok {
		return CheckpointDef{}, ErrNoCheckpoint
	}
	return def, nil
}
