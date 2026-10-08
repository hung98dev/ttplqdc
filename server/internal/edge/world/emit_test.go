package world

import (
	"context"
	"testing"

	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/runtime"
	simworld "thinhthan/internal/sim/world"
)

// TestEmitterDropsNonCheckpointKinds: world partitions emit only
// sim.checkpoint — any other flat durable command is a no-op, never an
// error (the partition must not wedge on an unknown kind).
func TestEmitterDropsNonCheckpointKinds(t *testing.T) {
	e := newEnv(t, nil)
	em := NewEmitter(e.w, e.q)
	cmd := runtime.DurableCommand{
		Kind:   runtime.CmdWorldConsequence,
		Family: "sim.world_consequence",
	}
	if err := em.Emit(context.Background(), cmd); err != nil {
		t.Fatalf("non-checkpoint emit: %v", err)
	}
}

// TestEmitterDropsConsumedPayload: a checkpoint op whose ledger entry was
// already consumed (replay after restart) drops silently — no Submit.
func TestEmitterDropsConsumedPayload(t *testing.T) {
	e := newEnv(t, nil)
	em := NewEmitter(e.w, e.q)
	cmd := runtime.DurableCommand{
		Kind:        runtime.CmdCheckpoint,
		Family:      simworld.FamilySimCheckpoint,
		OperationID: id.NewV4(),
	}
	if err := em.Emit(context.Background(), cmd); err != nil {
		t.Fatalf("consumed-payload emit: %v", err)
	}
	if e.receiptCount(t, id.UUID(cmd.OperationID)) != 0 {
		t.Fatal("consumed payload produced a durable receipt")
	}
}

// TestEmitCheckpointChain: the full server-driven write path — a
// committed transfer's checkpoint write rides the partition EMIT phase
// into the emitter, lands as a durable_command_receipts row under the
// ProducerCheckpoint executor, and applies the checkpoint/map columns.
func TestEmitCheckpointChain(t *testing.T) {
	e := newEnv(t, nil)
	e.w.SetEmitPort(NewEmitter(e.w, e.q))
	_, charID := e.seedCharacter(t)
	e.admit(t, charID)

	// Post the portal effect command (as the committed 104 would), let
	// the transfer freeze, then expire the budget so recovery writes the
	// member's checkpoint/map under ProducerCheckpoint.
	op := id.NewV4()
	if err := e.w.PostCommand(charID, &simworld.Command{
		Kind:        simworld.CmdPortal,
		CharacterID: charID,
		OperationID: op,
		PortalID:    "portal.alpha.beta",
	}); err != nil {
		t.Fatalf("post portal: %v", err)
	}
	waitFor(t, "transfer prepare emitted", func() bool {
		return len(e.out.forChar(charID, 105)) == 1
	})

	e.clocks.Advance(simworld.TransferBudgetWorld + 2e9)
	e.w.TickOnce()

	// ProducerCheckpoint commits have no admission receipt — the applied
	// row change is the evidence: the seeded default map flips to the
	// checkpoint's safe map.
	waitFor(t, "checkpoint write committed", func() bool {
		return e.mapOf(t, charID) == "map.alpha"
	})
}
