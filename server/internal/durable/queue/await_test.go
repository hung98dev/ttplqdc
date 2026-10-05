package queue_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	v1 "thinhthan/internal/protocol/v1"
)

func journalOutcome(t *testing.T, opID id.UUID) []byte {
	t.Helper()
	out, err := protojson.Marshal(&journalv1.JournalOutcome{
		Status:      v1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CCharacterCreateResult{
			S2CCharacterCreateResult: &v1.S2CCharacterCreateResult{
				Result: &v1.OperationResult{
					OperationId: opID[:],
					Status:      v1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				Character: &v1.CharacterSummary{
					CharacterName: "hero",
					ClassId:       "warrior",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal journal outcome: %v", err)
	}
	return out
}

// TestAwaitClientOutcome_ReturnsCommittedJournalOutcome — the seam waits
// for resolution and returns the retained schema-v1 JournalOutcome with
// its typed client_result member decodable.
func TestAwaitClientOutcome_ReturnsCommittedJournalOutcome(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "grant")
	op := opOf(t, rec)
	f.rec.out = journalOutcome(t, op)
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}

	out, err := f.q.AwaitClientOutcome(context.Background(),
		"inventory.mutate", owner, op)
	if err != nil {
		t.Fatalf("await: %v", err)
	}
	if out.GetStatus() != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status = %v", out.GetStatus())
	}
	if got := out.GetOperationId(); !bytes.Equal(got, op[:]) {
		t.Fatalf("operation_id mismatch")
	}
	got, ok := out.GetClientResult().(*journalv1.JournalOutcome_S2CCharacterCreateResult)
	if !ok {
		t.Fatalf("client_result type = %T", out.GetClientResult())
	}
	if got.S2CCharacterCreateResult.GetCharacter().GetCharacterName() != "hero" {
		t.Fatalf("character_name = %q", got.S2CCharacterCreateResult.GetCharacter().GetCharacterName())
	}
}

// TestAwaitClientOutcome_WaitsForResolution — while the executor is
// still running, the seam blocks; once resolved the retained outcome is
// returned.
func TestAwaitClientOutcome_WaitsForResolution(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "slow")
	op := opOf(t, rec)
	f.rec.out = journalOutcome(t, op)
	f.rec.block = make(chan struct{})
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-f.rec.entered

	done := make(chan error, 1)
	go func() {
		_, err := f.q.AwaitClientOutcome(context.Background(), "inventory.mutate", owner, op)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("await returned before resolution: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	f.rec.unblock()
	if err := <-done; err != nil {
		t.Fatalf("await: %v", err)
	}
}

// TestAwaitClientOutcome_RetriedOperationReturnsRetainedOutcome — a
// retried operation_id re-reads the committed row and never re-executes.
func TestAwaitClientOutcome_RetriedOperationReturnsRetainedOutcome(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "retry")
	op := opOf(t, rec)
	f.rec.out = journalOutcome(t, op)
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := f.q.AwaitClientOutcome(context.Background(), "inventory.mutate", owner, op); err != nil {
		t.Fatalf("await 1: %v", err)
	}

	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	out, err := f.q.AwaitClientOutcome(context.Background(), "inventory.mutate", owner, op)
	if err != nil {
		t.Fatalf("await 2: %v", err)
	}
	if out.GetStatus() != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status = %v", out.GetStatus())
	}
	if f.rec.callCount() != 1 {
		t.Fatalf("executor calls = %d, want 1 (no re-execution)", f.rec.callCount())
	}
}

// TestAwaitClientOutcome_TerminalNonexecution — a domain-rejected
// command surfaces the recorded terminal state as
// *idempotency.TerminalError, never a fabricated outcome.
func TestAwaitClientOutcome_TerminalNonexecution(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "reject")
	op := opOf(t, rec)
	f.rec.err = errors.New("domain rejected")
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}

	_, err := f.q.AwaitClientOutcome(context.Background(), "inventory.mutate", owner, op)
	var terr *idempotency.TerminalError
	if !errors.As(err, &terr) {
		t.Fatalf("await err = %v, want *idempotency.TerminalError", err)
	}
	if terr.State != idempotency.ReceiptRejected {
		t.Fatalf("terminal state = %q", terr.State)
	}
	if !errors.Is(err, idempotency.ErrRejected) {
		t.Fatalf("await err = %v, want Is(ErrRejected)", err)
	}
}

// TestAwaitClientOutcome_OutcomeRetainedAcrossDispositionAck — Ack only
// marks the row purge-eligible; the retained outcome stays readable for
// the full replay horizon (deliver-then-ack is the caller's contract).
func TestAwaitClientOutcome_OutcomeRetainedAcrossDispositionAck(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "ack")
	op := opOf(t, rec)
	f.rec.out = journalOutcome(t, op)
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := f.q.AwaitClientOutcome(context.Background(), "inventory.mutate", owner, op); err != nil {
		t.Fatalf("await before ack: %v", err)
	}
	if err := f.q.Ack(context.Background(), "inventory.mutate", owner, op); err != nil {
		t.Fatalf("ack: %v", err)
	}
	out, err := f.q.AwaitClientOutcome(context.Background(), "inventory.mutate", owner, op)
	if err != nil {
		t.Fatalf("await after ack: %v", err)
	}
	if out.GetClientResult() == nil {
		t.Fatal("client_result lost after disposition ack")
	}
}

// TestAwaitClientOutcome_FenceTerminalization — a queued command
// cancelled by erasure-fence terminalizes the receipt outside the
// executor; the seam returns the recorded REJECTED terminal state.
func TestAwaitClientOutcome_FenceTerminalization(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	// Occupy the single worker so the client record stays queued.
	blocker := clientRec(charOwner(), "inventory.mutate", "blocker")
	f.rec.block = make(chan struct{})
	if err := f.q.Submit(context.Background(), blocker); err != nil {
		t.Fatalf("submit blocker: %v", err)
	}
	<-f.rec.entered

	rec := clientRec(owner, "inventory.mutate", "fenced")
	op := opOf(t, rec)
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := f.q.ErasureFence(owner.ID); err != nil {
		t.Fatalf("fence: %v", err)
	}

	_, err := f.q.AwaitClientOutcome(context.Background(), "inventory.mutate", owner, op)
	var terr *idempotency.TerminalError
	if !errors.As(err, &terr) {
		t.Fatalf("await err = %v, want *idempotency.TerminalError", err)
	}
	if terr.State != idempotency.ReceiptRejected {
		t.Fatalf("terminal state = %q", terr.State)
	}
}

// TestAwaitClientOutcome_NoReceipt — an operation that was never
// admitted reports the declared no-receipt error.
func TestAwaitClientOutcome_NoReceipt(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	_, err := f.q.AwaitClientOutcome(context.Background(),
		"inventory.mutate", charOwner(), id.NewV7(time.Now()))
	if !errors.Is(err, queue.ErrNoClientReceipt) {
		t.Fatalf("await err = %v, want ErrNoClientReceipt", err)
	}
}

// TestAwaitClientOutcome_ContextDeadline — an unresolved operation
// bounds the wait on the caller's context.
func TestAwaitClientOutcome_ContextDeadline(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "hang")
	op := opOf(t, rec)
	f.rec.block = make(chan struct{})
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-f.rec.entered

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err := f.q.AwaitClientOutcome(ctx, "inventory.mutate", owner, op)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("await err = %v, want DeadlineExceeded", err)
	}
}
