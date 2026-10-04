package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// Queue-owned receipt SQL (save_rules.md: client queue admission is
// PostgreSQL-backed before enqueue). These mirror the idempotency
// receipt contract exactly; the state machine itself is not duplicated —
// execution/terminal transitions flow through Store.TrustedReplay.
const (
	ReceiptRejected = idempotency.ReceiptRejected
)

// admitReceipt inserts the ADMITTED durable_command_receipts row for a
// CLIENT command — reserve→admit→enqueue per concurrency.md. A duplicate
// primary key means admission already exists (crash retry); enqueue
// proceeds and TrustedReplay resolves the stored row.
func (q *Queue) admitReceipt(ctx context.Context, rec *journalv1.DurableCommandRecord,
	owner idempotency.Owner) error {
	opID := opIDOf(rec)
	// A crash retry may arrive after the receipt exists (possibly past its
	// replay horizon): an existing row means admission already succeeded —
	// enqueue proceeds and TrustedReplay resolves the stored row.
	var n int
	if err := q.pool.QueryRow(ctx,
		`SELECT count(*) FROM durable_command_receipts
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		rec.GetOperationFamily(), owner.ID.String(), opID.String()).Scan(&n); err != nil {
		return fmt.Errorf("%w: %v", ErrAdmissionFailed, err)
	}
	if n > 0 {
		return nil
	}
	issued := time.Time{}
	if ms, ok := id.OperationIssuedAt(opID); ok {
		issued = time.UnixMilli(ms).UTC()
	}
	fp := fingerprintOf(rec)
	_, err := q.pool.Exec(ctx,
		`INSERT INTO durable_command_receipts
		 (operation_family, owner_kind, owner_id, operation_id, request_fingerprint,
		  admitted_at, issued_at, replay_until, state)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		rec.GetOperationFamily(), string(owner.Kind), owner.ID.String(),
		opID.String(), fp[:],
		q.now(), issued, issued.AddDate(0, 0, 180), "ADMITTED")
	if err != nil {
		if isUniqueViolationErr(err) {
			// Admission already exists; the record still enqueues so its
			// stored receipt is resolved by the executor path.
			return nil
		}
		return fmt.Errorf("%w: %v", ErrAdmissionFailed, err)
	}
	return nil
}

// terminalizeReceipt flips a still-ADMITTED receipt to a terminal state —
// used for erasure-fence cancels and missing-executor drops, where the
// command must never execute.
func (q *Queue) terminalizeReceipt(ctx context.Context, st *recordState,
	state, outcome string) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE durable_command_receipts
		 SET state=$4, outcome_schema_version=$5, outcome=$6, completed_at=$7
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3
		   AND state='ADMITTED'`,
		st.key.family, st.owner.ID.String(), st.key.opID.String(),
		state, int32(1), []byte(outcome), q.now())
	if err != nil {
		return fmt.Errorf("queue: terminalize receipt: %w", err)
	}
	_ = tag // 0 rows: already terminal under another writer — idempotent.
	return nil
}

// Purge runs the outcome/receipt retention drivers under the queue
// lifecycle lock: the complete local inventory is enumerated first so
// retained outcomes referenced by queue/in-flight/journal work are never
// deleted (PRIV-008 — including rows past replay_until).
func (q *Queue) Purge(ctx context.Context, limit int64) (int64, error) {
	q.pmu.Lock()
	defer q.pmu.Unlock()
	q.mu.Lock()
	live := make([]string, 0, len(q.records))
	for k, st := range q.records {
		if st.refs > 0 {
			live = append(live, k.family+"|"+k.ownerID.String()+"|"+k.opID.String())
		}
	}
	q.mu.Unlock()

	var opsPurged int64
	if len(live) == 0 {
		n, err := q.store.PurgeOperations(ctx, limit)
		if err != nil {
			return 0, err
		}
		opsPurged = n
	} else {
		tag, err := q.pool.Exec(ctx,
			`DELETE FROM operations o
			 WHERE o.replay_until <= $1
			   AND NOT EXISTS (
			     SELECT 1 FROM durable_command_receipts r
			     WHERE r.operation_family=o.operation_family
			       AND r.owner_id=o.owner_id AND r.operation_id=o.operation_id
			       AND (r.disposition_ack_at IS NULL OR r.state='ADMITTED'))
			   AND NOT (o.operation_family || '|' || o.owner_id::text || '|' ||
			            o.operation_id::text = ANY($2))`,
			q.now(), live)
		if err != nil {
			return 0, fmt.Errorf("queue: purge operations: %w", err)
		}
		opsPurged = tag.RowsAffected()
	}
	if _, err := q.store.PurgeReceipts(ctx, limit); err != nil {
		return 0, err
	}
	return opsPurged, nil
}

func isUniqueViolationErr(err error) bool {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return pgerr.Code == "23505"
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if pg, ok := e.(*pgconn.PgError); ok && pg.Code == "23505" {
			return true
		}
	}
	return strings.Contains(err.Error(), "duplicate key value")
}
