package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// awaitPoll bounds the receipt re-read delay for terminal transitions the
// queue never signals internally — cancel/fence terminalization lands
// after the record's resolved flag, and ReconcileOrphans terminalizes
// outside any record at all.
const awaitPoll = 50 * time.Millisecond

// AwaitClientOutcome is the declared client-outcome await seam
// (ADR-0081, service_boundaries.md § Edge / Session): it waits for the
// terminal resolution of a submitted CLIENT command and returns the
// retained schema-v1 JournalOutcome — carrying the typed client_result
// member the handler delivers on the session connection. A conclusively
// recorded terminal nonexecution returns *idempotency.TerminalError.
//
// A retried operation_id returns the retained committed outcome without
// re-execution — reconnect/replay safe. The retained row is read before
// any purge: deliver-then-Ack is the caller's contract; disposition ack
// only makes the row purge-eligible at/after its replay horizon.
// Handlers never poll durable_command_receipts themselves.
func (q *Queue) AwaitClientOutcome(ctx context.Context, family string,
	owner idempotency.Owner, opID id.UUID) (*journalv1.JournalOutcome, error) {
	key := recordKey{family: family, ownerID: owner.ID, opID: opID}
	started := q.now()
	defer func() { q.metrics.wait(ctx, q.now().Sub(started).Milliseconds()) }()

	// The executor terminalizes the receipt before resolving the record,
	// so a resolved (or absent) record implies the terminal read below.
	for {
		q.mu.Lock()
		st, ok := q.records[key]
		resolved := !ok || st.resolved
		q.mu.Unlock()
		if resolved {
			break
		}
		select {
		case <-q.notify:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	for {
		state, ver, outcome, err := q.outcomeReceipt(ctx, family, owner, opID)
		if err != nil {
			return nil, err
		}
		switch state {
		case "":
			return nil, ErrNoClientReceipt
		case idempotency.ReceiptCommitted:
			if ver != nil && *ver != 1 {
				return nil, fmt.Errorf("queue: outcome schema version %d unsupported", *ver)
			}
			out := &journalv1.JournalOutcome{}
			if err := protojson.Unmarshal(outcome, out); err != nil {
				return nil, fmt.Errorf("queue: retained outcome undecodable: %w", err)
			}
			return out, nil
		case idempotency.ReceiptAdmitted:
			// Resolving elsewhere; re-read on the poll tick or the next
			// transition signal.
		default:
			return nil, &idempotency.TerminalError{State: state, Outcome: outcome}
		}
		select {
		case <-q.notify:
		case <-time.After(awaitPoll):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// outcomeReceipt reads the retained receipt state/version/outcome by
// primary key — the queue-owned declared read of durable_command_receipts.
// A missing row reports empty state.
func (q *Queue) outcomeReceipt(ctx context.Context, family string,
	owner idempotency.Owner, opID id.UUID) (string, *int32, []byte, error) {
	var state string
	var ver *int32
	var outcome []byte
	err := q.pool.QueryRow(ctx,
		`SELECT state, outcome_schema_version, outcome
		 FROM durable_command_receipts
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		family, owner.ID.String(), opID.String()).Scan(&state, &ver, &outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, nil, nil
	}
	if err != nil {
		return "", nil, nil, fmt.Errorf("queue: read outcome receipt: %w", err)
	}
	return state, ver, outcome, nil
}
