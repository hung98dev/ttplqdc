package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/testing/pgtest"
)

// sharedDSN is one baseline-applied database per package run (schema setup
// dominates; tests isolate by fresh random operation IDs).
var sharedDSN string

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd + "/../../../.."
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	srv, err := pgtest.Ensure(ctx)
	if err == nil {
		defer srv.Close()
		name := "idempotency_tests_" + time.Now().Format("20060102150405")
		dsn, cleanup, e2 := srv.NewDB(ctx, name)
		if e2 != nil {
			fmt.Fprintf(os.Stderr, "pgtest newdb: %v\n", e2)
		} else {
			defer cleanup()
			if e3 := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot()), "up"); e3 != nil {
				fmt.Fprintf(os.Stderr, "pgtest migrate: %v\n", e3)
			} else {
				sharedDSN = dsn
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
	}
	os.Exit(m.Run())
}

func newStore(t *testing.T, clock func() time.Time) (*Store, *pgxpool.Pool) {
	t.Helper()
	if sharedDSN == "" {
		t.Skip("DEFERRED(local-missing): no postgres")
	}
	pool, err := pgxpool.New(context.Background(), sharedDSN)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	opts := []Option{}
	if clock != nil {
		opts = append(opts, WithClock(clock))
	}
	return NewStore(pool, opts...), pool
}

func freshOwner() Owner {
	return Owner{Kind: OwnerCharacter, ID: id.NewV4()}
}

func fp(payload string) [32]byte {
	return sha256.Sum256([]byte(payload))
}

// TestOperationDeduplication: the same operation ID under one owner commits
// once; the second Execute replays the committed outcome without re-running
// the callback.
func TestOperationDeduplication(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "inventory.mutate"

	calls := 0
	cb := func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
		calls++
		return Outcome{SchemaVersion: 1, Payload: []byte(`{"balance":7}`)}, nil
	}
	out, err := s.Execute(ctx, family, owner, opID, fp("req-a"), cb)
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	out2, err := s.Execute(ctx, family, owner, opID, fp("req-a"), cb)
	if err != nil {
		t.Fatalf("replay execute: %v", err)
	}
	if calls != 1 {
		t.Fatalf("callback ran %d times, want 1", calls)
	}
	if !jsonEqual(out.Payload, out2.Payload) {
		t.Fatalf("outcome mismatch %q vs %q", out.Payload, out2.Payload)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM operations WHERE operation_family=$1 AND operation_id=$2`,
		family, opID.String()).Scan(&n); err != nil || n != 1 {
		t.Fatalf("ops rows=%d err=%v", n, err)
	}
}

// TestCommitBeforeResponseRetry: a retry after commit (response lost)
// returns the identical committed outcome.
func TestCommitBeforeResponseRetry(t *testing.T) {
	s, _ := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "auction.buy"
	payload := fp("bid:listing:42")

	out1, err := s.Execute(ctx, family, owner, opID, payload,
		func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
			return Outcome{SchemaVersion: 1, Payload: []byte(`{"item":"x","amount":100}`)}, nil
		})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	// Simulate lost response: identical retry must reconstruct the outcome.
	out2, err := s.Execute(ctx, family, owner, opID, payload,
		func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
			t.Fatal("callback re-executed on retry")
			return Outcome{}, nil
		})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if !jsonEqual(out1.Payload, out2.Payload) {
		t.Fatalf("retry outcome %q != committed %q", out2.Payload, out1.Payload)
	}
}

// jsonEqual compares JSONB outcome payloads semantically (key order and
// spacing are not preserved by JSONB storage).
func jsonEqual(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}

// TestConflictingPayloadRejection: same operation ID with a different
// request fingerprint is OPERATION_CONFLICT, never re-executed.
func TestConflictingPayloadRejection(t *testing.T) {
	s, _ := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "inventory.mutate"

	if _, err := s.Execute(ctx, family, owner, opID, fp("req-a"),
		func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
			return Outcome{SchemaVersion: 1, Payload: []byte(`{"ok":1}`)}, nil
		}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := s.Execute(ctx, family, owner, opID, fp("req-b"),
		func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
			t.Fatal("conflicting payload executed")
			return Outcome{}, nil
		})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

// TestPostgresUniqueConstraint: the PK (operation_family, owner_id,
// operation_id) is enforced by real Postgres — a second insert of the same
// key is a unique violation, and Execute's race path resolves to the stored
// outcome.
func TestPostgresUniqueConstraint(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	owner := freshOwner()
	opID := id.NewV7(time.Now())
	family := "sim.kill_settlement"
	payload := fp("kill:mob:1")

	if _, err := s.Execute(ctx, family, owner, opID, payload,
		func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
			return Outcome{SchemaVersion: 1, Payload: []byte(`{"exp":10}`)}, nil
		}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO operations
		 (operation_family, owner_kind, owner_id, operation_id, request_fingerprint,
		  outcome, created_at, completed_at, replay_until)
		 VALUES ($1,'CHARACTER',$2,$3,$4,'{}'::jsonb,now(),now(),now()+interval '180 days')`,
		family, owner.ID.String(), opID.String(), payload[:])
	if err == nil {
		t.Fatal("duplicate operations insert succeeded")
	}
}

// TestOperationKeyScopedByOwner: the same operation_id under two owners
// commits twice (owner-scoped dedupe key per ADR-0065).
func TestOperationKeyScopedByOwner(t *testing.T) {
	s, pool := newStore(t, nil)
	ctx := context.Background()
	opID := id.NewV7(time.Now())
	family := "sim.kill_settlement"
	payload := fp("kill:mob:2")
	ownerA := freshOwner()
	ownerB := freshOwner()

	cb := func(ctx context.Context, tx pgx.Tx) (Outcome, error) {
		return Outcome{SchemaVersion: 1, Payload: []byte(`{"exp":5}`)}, nil
	}
	if _, err := s.Execute(ctx, family, ownerA, opID, payload, cb); err != nil {
		t.Fatalf("owner A: %v", err)
	}
	if _, err := s.Execute(ctx, family, ownerB, opID, payload, cb); err != nil {
		t.Fatalf("owner B: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM operations WHERE operation_family=$1 AND operation_id=$2`,
		family, opID.String()).Scan(&n); err != nil || n != 2 {
		t.Fatalf("ops rows=%d, want 2 (err=%v)", n, err)
	}
}
