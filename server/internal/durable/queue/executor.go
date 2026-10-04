package queue

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// Executor applies one frozen durable command inside the committing
// transaction. CLIENT commands run inside Store.TrustedReplay (receipt
// lock + generic-outcome reconcile); every other producer runs inside
// Store.Execute. The domain callback is injected by composition.
type Executor func(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error)

// RegisterExecutor installs the domain callback for one producer kind.
// Composition registers every kind it admits; a record without an
// executor resolves terminally (no infinite retry).
func (q *Queue) RegisterExecutor(kind ProducerKind, ex Executor) {
	q.mu.Lock()
	q.executors[kind] = ex
	q.mu.Unlock()
}

// execute runs one record through the store path for its producer kind
// and resolves or requeues it.
func (q *Queue) execute(st *recordState) {
	q.mu.Lock()
	ex := q.executors[st.kind]
	q.mu.Unlock()

	if ex == nil {
		q.resolveTerminal(st, errNoExecutor)
		return
	}

	cb := func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return ex(ctx, tx, st.frozen)
	}

	ctx := q.execCtx
	started := q.now()
	var err error
	if st.kind == ProducerClient {
		_, err = q.store.TrustedReplay(ctx, st.key.family, st.owner,
			st.key.opID, fingerprintOf(st.frozen), cb)
	} else {
		_, err = q.store.Execute(ctx, st.key.family, st.owner,
			st.key.opID, fingerprintOf(st.frozen), cb)
	}
	if err == nil {
		q.metrics.commit(ctx, q.now().Sub(started).Milliseconds())
	}

	switch {
	case err == nil:
		q.mu.Lock()
		q.resolveLocked(st)
		q.mu.Unlock()
		q.signal()
	case transientErr(err):
		q.mu.Lock()
		q.requeueLocked(st)
		q.mu.Unlock()
	default:
		q.resolveTerminal(st, err)
	}
}

// transientErr classifies retryable failures: typed dependency errors,
// replay holds (missing/conflicting proof stays unresolved, never
// dropped), and context deadline/cancel when the queue is not draining.
func transientErr(err error) bool {
	switch {
	case errors.Is(err, idempotency.ErrDependency),
		errors.Is(err, idempotency.ErrReplayHold):
		return true
	case errors.Is(err, context.DeadlineExceeded):
		return true
	case errors.Is(err, context.Canceled):
		return true
	default:
		return false
	}
}

// resolveTerminal settles a domain rejection or unresolvable record once:
// CLIENT receipts are already terminally written by TrustedReplay or are
// terminalized here; non-client commands retain a typed rejected outcome
// through a settle execute so a retry of the same operation ID replays it
// instead of re-executing.
func (q *Queue) resolveTerminal(st *recordState, cause error) {
	if st.kind == ProducerClient {
		var terr *idempotency.TerminalError
		if !errors.As(cause, &terr) {
			// TrustedReplay already wrote the terminal receipt on a cb
			// rejection; any other non-transient failure leaves ADMITTED,
			// which we terminalize here (idempotent under the state guard).
			_ = q.terminalizeReceipt(context.Background(), st,
				idempotency.ReceiptRejected, "QUEUE_TERMINAL")
		}
	} else {
		msg := "REJECTED:" + cause.Error()
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
		outcome, _ := json.Marshal(map[string]string{"rejected": msg})
		_, err := q.store.Execute(q.execCtx, st.key.family, st.owner, st.key.opID,
			fingerprintOf(st.frozen), func(context.Context, pgx.Tx) (idempotency.Outcome, error) {
				return idempotency.Outcome{SchemaVersion: 1, Payload: outcome}, nil
			})
		if err != nil && transientErr(err) {
			q.mu.Lock()
			q.requeueLocked(st)
			q.mu.Unlock()
			return
		}
	}
	q.mu.Lock()
	q.resolveLocked(st)
	q.mu.Unlock()
	q.signal()
}
