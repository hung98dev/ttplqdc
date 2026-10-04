package runtime

import (
	"context"
)

// PhaseID enumerates the twelve ordered tick phases of
// docs/04_architecture/realtime_loop.md in execution order.
type PhaseID int

const (
	// PhaseExternalResults applies committed durable results.
	PhaseExternalResults PhaseID = iota
	// PhaseIngest applies queued player commands in receive order.
	PhaseIngest
	// PhaseTimersStatus advances timers and status-effect ticks.
	PhaseTimersStatus
	// PhaseMovement resolves movement and collision.
	PhaseMovement
	// PhaseActions starts or advances actions.
	PhaseActions
	// PhaseProjectiles advances projectiles and spatial effects.
	PhaseProjectiles
	// PhaseAI runs AI decisions.
	PhaseAI
	// PhaseHits resolves the hit/effect pipeline and displacement.
	PhaseHits
	// PhaseDeath resolves deaths, encounter objectives and spawn transitions.
	PhaseDeath
	// PhaseDurable emits pending durable commands.
	PhaseDurable
	// PhaseReplication builds replication state and events (AOI + deltas).
	PhaseReplication
	// PhaseCleanup removes expired transient entities.
	PhaseCleanup

	phaseCount = 12
)

// phaseOrder is the mandated phase sequence for tests and diagnostics.
var phaseOrder = [phaseCount]PhaseID{
	PhaseExternalResults, PhaseIngest, PhaseTimersStatus, PhaseMovement,
	PhaseActions, PhaseProjectiles, PhaseAI, PhaseHits, PhaseDeath,
	PhaseDurable, PhaseReplication, PhaseCleanup,
}

// PhaseOrder returns the twelve phase ids in execution order.
func PhaseOrder() [phaseCount]PhaseID { return phaseOrder }

// TickMetrics is the per-tick budget record (runtime and queue wait).
type TickMetrics struct {
	RuntimeNanos   int64
	QueueWaitNanos int64
	Rejected       int
}

func (p *Partition) runBuiltin(ph PhaseID, tc *TickContext) {
	switch ph {
	case PhaseExternalResults:
		p.applyResults()
	case PhaseIngest:
		p.applyInboxes()
	case PhaseDurable:
		p.emitDurable(tc)
	case PhaseReplication:
		p.buildReplication(tc)
	case PhaseCleanup:
		p.cleanupExpired(tc)
	}
}

// applyResults drains the durable-result port within the per-tick budget
// and applies each committed result — APPLY_COMMITTED_EXTERNAL_RESULTS.
// Runtime state is RUNTIME_ONLY: applied results mark only bookkeeping the
// sim keeps for itself (operation acks); there is no persisted result to
// rehydrate.
func (p *Partition) applyResults() {
	if p.ports.Results == nil {
		return
	}
	n := p.ports.Results.DrainInto(p.resBuf[:])
	for i := 0; i < n; i++ {
		p.onResult(&p.resBuf[i])
	}
}

// onResult applies one durable result inside the partition. Result kinds
// that carry world state arrive as typed commands back through intents or
// ports in their owning tasks; the runtime's own bookkeeping is the
// committed-operation ledger.
func (p *Partition) onResult(r *Result) {
	if p.resultHandler != nil {
		p.resultHandler(p, r)
	}
}

// emitDurable drains the pending durable-command buffer into the emit port
// in emission order — EMIT_DURABLE_COMMANDS. Commands the port refuses stay
// pending for the next tick; the buffer never reorders.
func (p *Partition) emitDurable(tc *TickContext) {
	if p.ports.Durable == nil || p.pendingN == 0 {
		return
	}
	ctx := context.Background()
	k := 0
	for i := 0; i < p.pendingN; i++ {
		cmd := &p.pending[i]
		cmd.Tick = tc.Tick
		cmd.ContentRevision = p.cfg.ContentRevision
		if err := p.ports.Durable.Emit(ctx, *cmd); err != nil {
			break
		}
		k++
	}
	copy(p.pending[:], p.pending[k:p.pendingN])
	p.pendingN -= k
}

// cleanupExpired removes transient entities past ExpiresAtTick —
// CLEANUP_EXPIRED_TRANSIENT_ENTITIES.
func (p *Partition) cleanupExpired(tc *TickContext) {
	for slot := classBase[ClassTransient]; slot < EntityCap; slot++ {
		if !p.used[slot] {
			continue
		}
		e := &p.ent[slot]
		if e.ExpiresAtTick != 0 && e.ExpiresAtTick <= tc.Tick {
			_ = p.Remove(e.ID)
		}
	}
}
