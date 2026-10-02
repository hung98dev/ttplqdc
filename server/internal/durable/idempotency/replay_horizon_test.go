package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

func committedOutcome() Command {
	return func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
		return Outcome{SchemaVersion: 1, Payload: []byte(`{"ok":true}`)}, nil
	}
}

func failNoExec(t *testing.T) Command {
	return func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
		t.Fatal("frozen command executed where replay/terminal was required")
		return Outcome{}, nil
	}
}

func receiptState(t *testing.T, pool *pgxpool.Pool, family string, owner Owner, opID id.UUID) string {
	t.Helper()
	var state string
	err := pool.QueryRow(context.Background(),
		`SELECT state FROM durable_command_receipts
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		family, owner.ID.String(), opID.String()).Scan(&state)
	if err != nil {
		t.Fatalf("receipt state: %v", err)
	}
	return state
}

func opRowCount(t *testing.T, pool *pgxpool.Pool, family string, owner Owner, opID id.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM operations
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		family, owner.ID.String(), opID.String()).Scan(&n)
	if err != nil {
		t.Fatalf("ops count: %v", err)
	}
	return n
}

// TestTrustedCommittedReceiptAfterHorizonAndOperationPurge: a retained
// COMMITTED receipt reconstructs the exact outcome even past the 180-day
// horizon and after the generic outcome was purged.
func TestTrustedCommittedReceiptAfterHorizonAndOperationPurge(t *testing.T) {
	base := time.Now()
	now := base
	s, pool := newStore(t, func() time.Time { return now })
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(base)
	family := "inventory.mutate"
	payload := fp("req")

	out, err := s.TrustedReplay(ctx, family, owner, opID, payload, committedOutcome())
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	// Acknowledge the receipt so the referenced generic outcome may purge;
	// the COMMITTED receipt row itself is retained (receipt purge is a
	// separate daily sweep and is not invoked here).
	if err := s.Acknowledge(ctx, family, owner, opID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	now = base.Add(181 * 24 * time.Hour)
	if _, err := s.PurgeOperations(ctx, 100); err != nil {
		t.Fatalf("purge ops: %v", err)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 0 {
		t.Fatalf("ops row not purged")
	}
	out2, err := s.TrustedReplay(ctx, family, owner, opID, payload, failNoExec(t))
	if err != nil {
		t.Fatalf("trusted replay past horizon: %v", err)
	}
	if string(out.Payload) != string(out2.Payload) {
		t.Fatalf("replayed %q != committed %q", out2.Payload, out.Payload)
	}
}

// TestTrustedExpiredAdmittedTerminalZeroWrites: an ADMITTED receipt whose
// horizon passed terminalizes EXPIRED_UNCOMMITTED with zero value writes.
func TestTrustedExpiredAdmittedTerminalZeroWrites(t *testing.T) {
	base := time.Now()
	now := base
	s, pool := newStore(t, func() time.Time { return now })
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(base)
	family := "inventory.mutate"

	if err := s.admit(ctx, family, owner, opID, fp("req")); err != nil {
		t.Fatalf("admit: %v", err)
	}
	now = base.Add(181 * 24 * time.Hour)
	_, err := s.TrustedReplay(ctx, family, owner, opID, fp("req"), failNoExec(t))
	var te *TerminalError
	if !errors.As(err, &te) || te.State != ReceiptExpiredUncommitted {
		t.Fatalf("want EXPIRED_UNCOMMITTED terminal, got %v", err)
	}
	if st := receiptState(t, pool, family, owner, opID); st != ReceiptExpiredUncommitted {
		t.Fatalf("state=%s", st)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 0 {
		t.Fatalf("zero writes violated: %d ops rows", n)
	}
}

// TestMissingReceiptBlocksTrustedReplay: a generic commit with no receipt
// is ambiguous proof — replay holds, never rerolls.
func TestMissingReceiptBlocksTrustedReplay(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "inventory.mutate"
	payload := fp("req")

	if _, err := s.Execute(ctx, family, owner, opID, payload, committedOutcome()); err != nil {
		t.Fatalf("generic commit: %v", err)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 1 {
		t.Fatalf("setup: ops row missing")
	}
	_, err := s.TrustedReplay(ctx, family, owner, opID, payload, failNoExec(t))
	if !errors.Is(err, ErrReplayHold) {
		t.Fatalf("want ErrReplayHold, got %v", err)
	}
}

// TestReceiptCommitAtomicWithOperation: value mutation + COMMITTED receipt
// + operations insert commit atomically; a failing command leaves no ops
// row and a retained typed REJECTED.
func TestReceiptCommitAtomicWithOperation(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()

	// success path: both rows under one commit
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	if _, err := s.TrustedReplay(ctx, "inventory.mutate", owner, opID, fp("a"), committedOutcome()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if st := receiptState(t, pool, "inventory.mutate", owner, opID); st != ReceiptCommitted {
		t.Fatalf("state=%s", st)
	}
	if n := opRowCount(t, pool, "inventory.mutate", owner, opID); n != 1 {
		t.Fatalf("ops row missing")
	}

	// failure path: no ops row, retained REJECTED
	owner2 := freshOwner()
	opID2 := id.NewV7(time.Now())
	_, err := s.TrustedReplay(ctx, "inventory.mutate", owner2, opID2, fp("b"),
		func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
			return Outcome{}, errors.New("domain failure")
		})
	var te *TerminalError
	if !errors.As(err, &te) || te.State != ReceiptRejected {
		t.Fatalf("want REJECTED terminal, got %v", err)
	}
	if st := receiptState(t, pool, "inventory.mutate", owner2, opID2); st != ReceiptRejected {
		t.Fatalf("state=%s", st)
	}
	if n := opRowCount(t, pool, "inventory.mutate", owner2, opID2); n != 0 {
		t.Fatalf("failed command left ops row")
	}
}

// TestOrphanAdmittedReconcilesWithoutExecution: quiescent-startup orphans
// reconcile generic commits (recovering COMMITTED) or terminalize without
// executing.
func TestOrphanAdmittedReconcilesWithoutExecution(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	noLive := func(string, Owner, id.UUID) bool { return false }

	// Orphan with a generic commit → recovered COMMITTED, no execution.
	ownerA := freshOwner()
	opA := id.NewV7(time.Now())
	payload := fp("req")
	if err := s.admit(ctx, "inventory.mutate", ownerA, opA, payload); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if _, err := s.Execute(ctx, "inventory.mutate", ownerA, opA, payload, committedOutcome()); err != nil {
		t.Fatalf("generic commit: %v", err)
	}
	holds, err := s.ReconcileOrphans(ctx, noLive)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(holds) != 0 {
		t.Fatalf("unexpected holds: %v", holds)
	}
	if st := receiptState(t, pool, "inventory.mutate", ownerA, opA); st != ReceiptCommitted {
		t.Fatalf("state=%s, want COMMITTED", st)
	}

	// Orphan with no commit proof → terminalized without executing.
	ownerB := freshOwner()
	opB := id.NewV7(time.Now())
	if err := s.admit(ctx, "inventory.mutate", ownerB, opB, fp("req")); err != nil {
		t.Fatalf("admit: %v", err)
	}
	holds, err = s.ReconcileOrphans(ctx, noLive)
	if err != nil {
		t.Fatalf("reconcile2: %v", err)
	}
	if len(holds) != 0 {
		t.Fatalf("unexpected holds: %v", holds)
	}
	if st := receiptState(t, pool, "inventory.mutate", ownerB, opB); st != ReceiptRejected {
		t.Fatalf("state=%s, want REJECTED", st)
	}
	if n := opRowCount(t, pool, "inventory.mutate", ownerB, opB); n != 0 {
		t.Fatalf("orphan executed writes")
	}
}

// TestClientAdmissionDatabaseOutageEnqueuesNothing: with PostgreSQL down,
// admission fails closed (TEMPORARY_DEPENDENCY_FAILURE) and enqueues
// nothing.
func TestClientAdmissionDatabaseOutageEnqueuesNothing(t *testing.T) {
	s, pool := newStore(t, nil)
	pool.Close() // simulate outage
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())

	_, err := s.TrustedReplay(ctx, "inventory.mutate", owner, opID, fp("req"), failNoExec(t))
	if !errors.Is(err, ErrDependency) {
		t.Fatalf("want ErrDependency, got %v", err)
	}
	// Nothing was enqueued — verify on a fresh pool.
	pool2, err := pgxpool.New(ctx, sharedDSN)
	if err != nil {
		t.Fatalf("pool2: %v", err)
	}
	defer pool2.Close()
	var n int
	pool2.QueryRow(ctx,
		`SELECT count(*) FROM durable_command_receipts
		 WHERE operation_family='inventory.mutate' AND operation_id=$1`,
		opID.String()).Scan(&n)
	if n != 0 {
		t.Fatalf("outage enqueued %d receipts", n)
	}
}

// TestReceiptErasureRetainsRestrictedOwnerKey: erasure never repoints
// receipts to the tombstone account; the restricted pseudonymous key is
// preserved (owner_id has no FK and is never rewritten).
func TestReceiptErasureRetainsRestrictedOwnerKey(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	// Tombstone owner is a valid receipt owner (ACCOUNT kind, no FK).
	tombstone, err := id.ParseUUID("00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("tombstone: %v", err)
	}
	owner := Owner{Kind: OwnerAccount, ID: tombstone}
	opID := id.NewV7(time.Now())
	family := "account.link"

	if _, err := s.TrustedReplay(ctx, family, owner, opID, fp("req"), committedOutcome()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var gotOwner string
	if err := pool.QueryRow(ctx,
		`SELECT owner_id::text FROM durable_command_receipts
		 WHERE operation_family=$1 AND operation_id=$2`,
		family, opID.String()).Scan(&gotOwner); err != nil {
		t.Fatalf("read owner: %v", err)
	}
	if gotOwner != tombstone.String() {
		t.Fatalf("owner key repointed: %s", gotOwner)
	}
}

// TestUnackedReceiptPreventsPurge: unacknowledged receipts (and the generic
// outcomes they reference) are never purged; acked terminal rows at the
// horizon purge.
func TestUnackedReceiptPreventsPurge(t *testing.T) {
	base := time.Now()
	now := base
	s, pool := newStore(t, func() time.Time { return now })
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(base)
	family := "inventory.mutate"

	if _, err := s.TrustedReplay(ctx, family, owner, opID, fp("req"), committedOutcome()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	now = base.Add(181 * 24 * time.Hour)
	// unacked: receipt AND referenced generic outcome survive purge
	if _, err := s.PurgeReceipts(ctx, 100); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if st := receiptState(t, pool, family, owner, opID); st != ReceiptCommitted {
		t.Fatalf("unacked receipt purged")
	}
	if _, err := s.PurgeOperations(ctx, 100); err != nil {
		t.Fatalf("purge ops: %v", err)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 1 {
		t.Fatalf("unacked reference outcome purged")
	}
	// acknowledge → now purgeable
	if err := s.Acknowledge(ctx, family, owner, opID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if _, err := s.PurgeReceipts(ctx, 100); err != nil {
		t.Fatalf("purge2: %v", err)
	}
	var n int
	pool.QueryRow(ctx,
		`SELECT count(*) FROM durable_command_receipts
		 WHERE operation_family=$1 AND operation_id=$2`,
		family, opID.String()).Scan(&n)
	if n != 0 {
		t.Fatalf("acked terminal receipt not purged")
	}
}

// TestReplayBeforeMutablePreconditions: owner+fingerprint authentication
// precedes mutable preconditions — a matching replay returns the committed
// outcome and a conflicting one errors, both without executing.
func TestReplayBeforeMutablePreconditions(t *testing.T) {
	s, _ := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "inventory.mutate"

	if _, err := s.TrustedReplay(ctx, family, owner, opID, fp("req"), committedOutcome()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	// Replay reconstructs without touching the mutation path.
	out, err := s.TrustedReplay(ctx, family, owner, opID, fp("req"), failNoExec(t))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if string(out.Payload) != `{"ok":true}` {
		t.Fatalf("outcome %q", out.Payload)
	}
}

// TestOwnerFingerprintMismatchCannotReplay: a retry under the same
// operation ID with a different request fingerprint is OPERATION_CONFLICT.
func TestOwnerFingerprintMismatchCannotReplay(t *testing.T) {
	s, _ := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "inventory.mutate"

	if _, err := s.TrustedReplay(ctx, family, owner, opID, fp("req-a"), committedOutcome()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	_, err := s.TrustedReplay(ctx, family, owner, opID, fp("req-b"), failNoExec(t))
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

// TestRetryAt180DayBoundaryExpired: an op issued exactly at the horizon
// boundary is expired — zero writes, never replayed as new.
func TestRetryAt180DayBoundaryExpired(t *testing.T) {
	base := time.Now()
	s, pool := newStore(t, func() time.Time { return base })
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(base.Add(-180 * 24 * time.Hour))
	family := "inventory.mutate"

	_, err := s.Execute(ctx, family, owner, opID, fp("req"), failNoExec(t))
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired, got %v", err)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 0 {
		t.Fatalf("expired op wrote %d rows", n)
	}
}

// TestPurgedUuidCannotExecuteAgain: an expired client UUIDv7 returns
// OPERATION_EXPIRED before new execution even after its row was purged.
func TestPurgedUuidCannotExecuteAgain(t *testing.T) {
	base := time.Now()
	now := base
	s, pool := newStore(t, func() time.Time { return now })
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(base)
	family := "inventory.mutate"

	if _, err := s.Execute(ctx, family, owner, opID, fp("req"), committedOutcome()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	now = base.Add(181 * 24 * time.Hour)
	if _, err := s.PurgeOperations(ctx, 100); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 0 {
		t.Fatalf("setup: row not purged")
	}
	_, err := s.Execute(ctx, family, owner, opID, fp("req"), failNoExec(t))
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

// TestFutureTimestampOver60SecondsMalformed: issued_at > now+60s is
// PROTOCOL_MALFORMED — rejected before any write.
func TestFutureTimestampOver60SecondsMalformed(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now().Add(61 * time.Second))
	family := "inventory.mutate"

	_, err := s.Execute(ctx, family, owner, opID, fp("req"), failNoExec(t))
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("want ErrMalformed, got %v", err)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 0 {
		t.Fatalf("malformed op wrote rows")
	}
}

// TestPurgeNeverBeforeReplayUntil: neither receipts nor operations purge
// before replay_until.
func TestPurgeNeverBeforeReplayUntil(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "inventory.mutate"

	if _, err := s.TrustedReplay(ctx, family, owner, opID, fp("req"), committedOutcome()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := s.Acknowledge(ctx, family, owner, opID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if _, err := s.PurgeReceipts(ctx, 100); err != nil {
		t.Fatalf("purge receipts: %v", err)
	}
	if st := receiptState(t, pool, family, owner, opID); st != ReceiptCommitted {
		t.Fatalf("receipt purged before horizon")
	}
	if _, err := s.PurgeOperations(ctx, 100); err != nil {
		t.Fatalf("purge ops: %v", err)
	}
	if n := opRowCount(t, pool, family, owner, opID); n != 1 {
		t.Fatalf("operation purged before horizon")
	}
}

// TestServerNaturalKeyDedupAfterOperationPurge: a server UUIDv5 op whose
// generic outcome expired is still deduped by the owning table's natural
// key — re-execution cannot grant twice.
func TestServerNaturalKeyDedupAfterOperationPurge(t *testing.T) {
	base := time.Now()
	now := base
	s, pool := newStore(t, func() time.Time { return now })
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS idempotency_test_ledger (
		  natural_key TEXT PRIMARY KEY,
		  amount BIGINT NOT NULL)`); err != nil {
		t.Fatalf("ledger table: %v", err)
	}
	owner := Owner{Kind: OwnerWorld, ID: id.WorldOwnerID}
	opID := id.ServerJobOperationID("sim.kill_settlement",
		"m1", "0", "0", "11111111-1111-4111-8111-111111111111", "1", "100")
	payload := fp("kill")

	cb := func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
		tag, err := tx.Exec(ctx,
			`INSERT INTO idempotency_test_ledger (natural_key, amount)
			 VALUES ($1,10) ON CONFLICT (natural_key) DO NOTHING`,
			"sim:m1:0:0:11111111-1111-4111-8111-111111111111:1:100")
		if err != nil {
			return Outcome{}, err
		}
		if tag.RowsAffected() == 0 {
			// Natural-key dedup: reconstruct prior outcome.
			return Outcome{SchemaVersion: 1, Payload: []byte(`{"exp":10,"dedup":true}`)}, nil
		}
		return Outcome{SchemaVersion: 1, Payload: []byte(`{"exp":10}`)}, nil
	}

	out1, err := s.Execute(ctx, "sim.kill_settlement", owner, opID, payload, cb)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	// Expire and purge the generic outcome.
	now = base.Add(181 * 24 * time.Hour)
	if _, err := s.PurgeOperations(ctx, 100); err != nil {
		t.Fatalf("purge: %v", err)
	}
	out2, err := s.Execute(ctx, "sim.kill_settlement", owner, opID, payload, cb)
	if err != nil {
		t.Fatalf("re-execute after purge: %v", err)
	}
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_test_ledger
		WHERE natural_key='sim:m1:0:0:11111111-1111-4111-8111-111111111111:1:100'`).Scan(&n)
	if n != 1 {
		t.Fatalf("natural dedup violated: %d ledger rows", n)
	}
	_ = out1
	_ = out2
}
