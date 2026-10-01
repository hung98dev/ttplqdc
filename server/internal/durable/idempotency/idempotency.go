// Package idempotency is the PostgreSQL-backed operation dedupe/result
// replay primitive (data_model.md § operations + § durable_command_receipts,
// database.md §5, ids.md § Trusted Queued-Client Replay). It owns the
// at-most-once settlement guarantee: identical in-horizon retries
// reconstruct the committed outcome, conflicting payloads under the same
// operation ID are rejected, and expired IDs never execute again — even
// after the generic outcome row was purged.
package idempotency

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// OwnerKind is the operations.owner_kind CHECK enum.
type OwnerKind string

const (
	OwnerAccount   OwnerKind = "ACCOUNT"
	OwnerCharacter OwnerKind = "CHARACTER"
	OwnerGuild     OwnerKind = "GUILD"
	OwnerWorld     OwnerKind = "WORLD"
)

// Owner scopes an operation key; owner_id has no FK and no ownership is
// inferred from an untrusted operation ID.
type Owner struct {
	Kind OwnerKind
	ID   id.UUID
}

// Outcome is the typed committed result. For queued client commands the
// payload is schema-v1 JournalOutcome bytes; the generic operations row
// carries the bounded schema-versioned JSONB reference.
type Outcome struct {
	SchemaVersion int32
	Payload       []byte
}

// Sentinel errors; wire codes map at the edge only.
var (
	ErrMalformed          = errors.New("idempotency: malformed operation id")       // PROTOCOL_MALFORMED
	ErrExpired            = errors.New("idempotency: operation expired")            // OPERATION_EXPIRED
	ErrConflict           = errors.New("idempotency: conflicting request payload")  // OPERATION_CONFLICT
	ErrDependency         = errors.New("idempotency: temporary dependency failure") // TEMPORARY_DEPENDENCY_FAILURE
	ErrRejected           = errors.New("idempotency: request terminally rejected")  // typed REJECTED
	ErrExpiredUncommitted = errors.New("idempotency: admitted request expired uncommitted")
	ErrReplayHold         = errors.New("idempotency: missing or conflicting receipt proof")
	ErrBackpressure       = errors.New("idempotency: durable queue capacity exhausted")
)

// Command is the frozen unit of work committed under the receipt lock. It
// performs the domain value mutation on tx and returns the typed outcome.
type Command func(ctx context.Context, tx pgx.Tx) (Outcome, error)

// QueueGate reserves durable-queue capacity before admission. The
// per-partition durable-result queue is bounded at 64 (save_rules.md).
type QueueGate interface {
	// Reserve takes a queue slot; release frees it. An error means the
	// request never reached the database — enqueues nothing.
	Reserve(ctx context.Context) (release func(), err error)
}

// BoundedGate is an in-process capacity-64 admission gate.
type BoundedGate struct {
	slots chan struct{}
}

func NewBoundedGate(capacity int) *BoundedGate {
	return &BoundedGate{slots: make(chan struct{}, capacity)}
}

func (g *BoundedGate) Reserve(ctx context.Context) (func(), error) {
	select {
	case g.slots <- struct{}{}:
		return func() { <-g.slots }, nil
	case <-ctx.Done():
		return nil, ErrBackpressure
	}
}

// Store executes and replays durable operations.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
	gate QueueGate
}

type Option func(*Store)

// WithClock overrides wall-clock reads (tests move time past horizons).
func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

// WithQueueGate injects admission-capacity behavior (tests force outage).
func WithQueueGate(g QueueGate) Option {
	return func(s *Store) { s.gate = g }
}

func NewStore(pool *pgxpool.Pool, opts ...Option) *Store {
	s := &Store{pool: pool, now: func() time.Time { return time.Now().UTC() }, gate: NewBoundedGate(64)}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Execute is the generic operations-row path: dedupe on
// (operation_family, owner_id, operation_id), fingerprint-auth before
// mutable preconditions, cb mutation + outcome + row insert atomically.
func (s *Store) Execute(ctx context.Context, family string, owner Owner, opID id.UUID,
	fingerprint [32]byte, cb Command) (Outcome, error) {
	if err := s.validateID(opID); err != nil {
		return Outcome{}, err
	}
	issued, _ := id.OperationIssuedAt(opID) // zero for non-v7 (server v5)
	if out, done, err := s.lookupOperation(ctx, family, owner, opID, fingerprint); err != nil {
		return Outcome{}, err
	} else if done {
		return out, nil
	}
	var res Outcome
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		res, err = cb(ctx, tx)
		if err != nil {
			return err
		}
		return insertOperation(ctx, tx, family, owner, opID, fingerprint, res, issued, s.now())
	})
	if err != nil {
		if isUniqueViolation(err) {
			// Concurrent winner: its row defines the outcome.
			out, done, lerr := s.lookupOperation(ctx, family, owner, opID, fingerprint)
			if lerr == nil && done {
				return out, nil
			}
			if lerr != nil {
				return Outcome{}, lerr
			}
			return Outcome{}, ErrReplayHold
		}
		return Outcome{}, err
	}
	return res, nil
}

// validateID enforces the operation-ID contract before any mutable
// precondition: malformed/future-skew → PROTOCOL_MALFORMED, expired →
// OPERATION_EXPIRED even after purge. Server UUIDv5 ops carry no issued_at —
// their replay window starts at completed_at and dedup is natural-key based.
func (s *Store) validateID(opID id.UUID) error {
	if opID.Version() == 5 {
		if opID.IsNil() {
			return ErrMalformed
		}
		return nil
	}
	var verr *id.OperationIDError
	if err := id.ValidateOperationID(opID, s.now()); errors.As(err, &verr) {
		if verr.Kind == id.OperationIDExpired {
			return ErrExpired
		}
		return ErrMalformed
	} else if err != nil {
		return fmt.Errorf("idempotency: validate id: %w", err)
	}
	return nil
}

// lookupOperation is the owner+fingerprint auth step. done=false means no
// row exists (fresh execute).
func (s *Store) lookupOperation(ctx context.Context, family string, owner Owner,
	opID id.UUID, fingerprint [32]byte) (out Outcome, done bool, err error) {
	var stored []byte
	err = s.pool.QueryRow(ctx,
		`SELECT request_fingerprint, outcome FROM operations
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		family, owner.ID.String(), opID.String()).Scan(&stored, &out.Payload)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Outcome{}, false, nil
	case err != nil:
		return Outcome{}, false, fmt.Errorf("idempotency: lookup: %w", err)
	}
	if string(stored) != string(fingerprint[:]) {
		return Outcome{}, false, ErrConflict
	}
	out.SchemaVersion = 1
	return out, true, nil
}

func insertOperation(ctx context.Context, tx pgx.Tx, family string, owner Owner,
	opID id.UUID, fingerprint [32]byte, out Outcome, issued int64, now time.Time) error {
	completed := now
	replayUntil := completed.AddDate(0, 0, 180)
	if opID.Version() == 7 && issued > 0 {
		replayUntil = time.UnixMilli(issued).UTC().AddDate(0, 0, 180)
	}
	payload := out.Payload
	if payload == nil {
		payload = []byte("{}")
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO operations
		 (operation_family, owner_kind, owner_id, operation_id, request_fingerprint,
		  outcome, created_at, completed_at, replay_until)
		 VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9)`,
		family, string(owner.Kind), owner.ID.String(), opID.String(),
		fingerprint[:], string(payload), now, completed, replayUntil)
	return err
}

func isUniqueViolation(err error) bool {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return pgerr.Code == "23505"
	}
	// pgx wraps commit-time violations in the returned error chain.
	for e := err; e != nil; e = errors.Unwrap(e) {
		if pg, ok := e.(*pgconn.PgError); ok && pg.Code == "23505" {
			return true
		}
	}
	return strings.Contains(err.Error(), "duplicate key value")
}
