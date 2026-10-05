package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
)

// DurableCommand is a typed host request for durable work: the record is
// the IMP-082 queue admission unit (family/owner/op identity validated by
// queue.Submit); Family selects the registered executor.
type DurableCommand interface {
	Family() string
	Record() *journalv1.DurableCommandRecord
}

// Commit resolves one durable submission: Err nil = committed + all refs
// settled; non-nil = typed queue failure or expiry of the bounded wait.
// Executor-side rejection is a domain result the family's executor
// surfaces itself — Commit reports queue settlement.
type Commit struct {
	Err error
}

// DurableBridge is the runtime-owned adapter over queue.Queue: typed
// submission plus commit fanback. Hosts use this typed surface only —
// they never import queue or pgx themselves. The pgx-naming executor
// callbacks are authored where SQL is legal (durable-side host wiring or
// the test harness) and installed per producer kind here.
type DurableBridge struct {
	q       *queue.Queue
	wg      sync.WaitGroup
	mu      sync.Mutex
	life    context.Context
	closeFn context.CancelFunc
	started bool
	closed  bool
}

func newDurableBridge(q *queue.Queue) *DurableBridge {
	return &DurableBridge{q: q}
}

// RegisterExecutor installs the executor for one producer kind; records
// of a kind with no executor resolve terminally in the queue (IMP-082).
// Composition calls this during host setup — global/ itself can never
// write an executor body because the signature names pgx.Tx.
func (b *DurableBridge) RegisterExecutor(kind queue.ProducerKind, ex queue.Executor) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrShutdown
	}
	b.q.RegisterExecutor(kind, ex)
	return nil
}

// start binds the bridge lifecycle to the runtime's exec context.
func (b *DurableBridge) start(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.life, b.closeFn = context.WithCancel(ctx)
	b.started = true
}

// SubmitDurable admits one durable command and returns a channel that
// resolves once the queue reports the record fully settled (every
// queued/in-flight/journal ref disposed; receipt disposition-acked for
// CLIENT producers). The fanback goroutine is bounded by the bridge
// lifecycle, not the caller's ctx.
func (b *DurableBridge) SubmitDurable(ctx context.Context, dc DurableCommand) (<-chan Commit, error) {
	if dc == nil || dc.Record() == nil {
		return nil, errors.New("global: nil durable command")
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, ErrShutdown
	}
	if !b.started {
		b.mu.Unlock()
		return nil, ErrNotRunning
	}
	life := b.life
	b.mu.Unlock()

	rec := dc.Record()
	if err := b.q.Submit(ctx, rec); err != nil {
		return nil, err
	}
	agg, err := queue.ValidateRecord(rec)
	if err != nil {
		return nil, err
	}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	owner := idempotency.Owner{Kind: agg.OwnerKind, ID: agg.OwnerID}

	out := make(chan Commit, 1)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		var err error
		if queue.ProducerOf(rec) == queue.ProducerClient {
			// CLIENT producers carry a durable receipt: Ack waits for all
			// refs to dispose then marks the receipt disposition-acked
			// (ids.md). Receipt-less producers have nothing to ack —
			// settlement is observed via Live instead.
			err = b.q.Ack(life, agg.Family, owner, opID)
		} else {
			err = b.waitSettled(life, agg.Family, owner, opID)
		}
		out <- Commit{Err: err}
		close(out)
	}()
	return out, nil
}

// waitSettled blocks until the queue reports the record fully resolved
// (every queued/in-flight/journal ref disposed) or life ends.
func (b *DurableBridge) waitSettled(life context.Context, family string,
	owner idempotency.Owner, opID id.UUID) error {
	for b.q.Live(family, owner, opID) {
		select {
		case <-life.Done():
			return life.Err()
		case <-time.After(time.Millisecond):
		}
	}
	return nil
}

// close stops new submissions and flushes pending commit waiters bounded
// by ctx (Shutdown's bounded in-flight flush).
func (b *DurableBridge) close(ctx context.Context) error {
	b.mu.Lock()
	b.closed = true
	if b.closeFn != nil {
		b.closeFn()
	}
	b.mu.Unlock()
	done := make(chan struct{})
	go func() { b.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
