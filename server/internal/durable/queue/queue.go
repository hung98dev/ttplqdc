// Package queue is the bounded durable command queue between
// Sim/Global/Edge producers and PostgreSQL (save_rules.md closed producer
// registry, concurrency.md queue rules, deployment.md journal bound). It
// owns admission validation, per-aggregate ordering, CLIENT receipt
// admission, typed backpressure, reference tracking for ack/journal
// disposition, the frozen inventory handed to the outbox journal writer,
// and erasure admission fences. File-level journal I/O belongs to the
// composition root (IMP-069).
package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	obscore "thinhthan/internal/observability/core"
)

// Deps wires the queue's external dependencies.
type Deps struct {
	// Now is the clock for admission timestamps and latency metrics.
	Now func() time.Time
	// Pool is the PostgreSQL pool for queue-owned receipt admission SQL.
	Pool *pgxpool.Pool
	// Metrics is the bounded-cardinality registry; nil disables emission.
	Metrics *obscore.Registry
	// Gate is the durable admission-capacity gate wired into the same
	// holding bound the store replays under (concurrency.md). Nil uses
	// idempotency.NewBoundedGate(64) — the number is not re-specified here.
	Gate idempotency.QueueGate
	// Workers is the fixed executor pool size; <=0 uses 4.
	Workers int
	// RetryDelay is the transient-failure redispatch delay; <=0 uses 25ms.
	RetryDelay time.Duration
}

// Queue is the bounded durable command queue.
type Queue struct {
	capacity   int
	store      *idempotency.Store
	pool       *pgxpool.Pool
	now        func() time.Time
	gate       idempotency.QueueGate
	workers    int
	retryDelay time.Duration
	metrics    *queueMetrics

	executors map[ProducerKind]Executor

	slots chan struct{}     // capacity semaphore: queued + in-flight bound
	work  chan *recordState // dispatch channel (capacity = workers)
	stop  chan struct{}     // closes exec ctx
	mu    sync.Mutex
	pmu   sync.Mutex // purge lifecycle lock

	closed    bool
	records   map[recordKey]*recordState
	lanes     map[AggKey]*lane
	deferred  []*recordState // destructive jobs parked on subject refs
	fenced    map[id.UUID]struct{}
	seq       uint64
	notify    chan struct{} // broadcast on ref/state transitions (cap 1)
	execCtx   context.Context
	cancelCtx context.CancelFunc
	wg        sync.WaitGroup
}

// New creates the queue bounded by capacity (composition derives the total
// from deployment.md's count/disk bound: journal record cap <= 1048576).
func New(capacity int, store *idempotency.Store, deps Deps) *Queue {
	if capacity <= 0 {
		capacity = 1
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	if deps.Gate == nil {
		deps.Gate = idempotency.NewBoundedGate(64)
	}
	if deps.Workers <= 0 {
		deps.Workers = 4
	}
	if deps.RetryDelay <= 0 {
		deps.RetryDelay = 25 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	q := &Queue{
		capacity:   capacity,
		store:      store,
		pool:       deps.Pool,
		now:        deps.Now,
		gate:       deps.Gate,
		workers:    deps.Workers,
		retryDelay: deps.RetryDelay,
		executors:  make(map[ProducerKind]Executor),
		slots:      make(chan struct{}, capacity),
		work:       make(chan *recordState, deps.Workers),
		stop:       make(chan struct{}),
		records:    make(map[recordKey]*recordState),
		lanes:      make(map[AggKey]*lane),
		fenced:     make(map[id.UUID]struct{}),
		notify:     make(chan struct{}, 1),
		execCtx:    ctx,
		cancelCtx:  cancel,
	}
	q.metrics = newQueueMetrics(deps.Metrics)
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	return q
}

// Submit validates a record against the closed producer registry, reserves
// capacity, admits a durable receipt for CLIENT commands, and enqueues
// without ever blocking the caller's tick — a full queue or saturated
// admission gate returns a typed error instead. DB outage during CLIENT
// admission enqueues nothing.
func (q *Queue) Submit(ctx context.Context, rec *journalv1.DurableCommandRecord) error {
	agg, err := ValidateRecord(rec)
	if err != nil {
		q.metrics.submit(ctx, "unknown", "reject")
		return err
	}
	kind := ProducerOf(rec)

	select {
	case q.slots <- struct{}{}:
	default:
		q.metrics.submit(ctx, kind.String(), "queue_full")
		return fmt.Errorf("%w: %d", ErrQueueFull, q.capacity)
	}

	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		<-q.slots
		return ErrShutdown
	}
	if _, fenced := q.fenced[agg.OwnerID]; fenced {
		q.mu.Unlock()
		<-q.slots
		q.metrics.submit(ctx, kind.String(), "erasure_fenced")
		return ErrErasureFenced
	}
	q.mu.Unlock()

	var gateRelease func()
	if kind == ProducerClient {
		// Reserve admission capacity before the receipt insert. A
		// saturated gate holds only until the caller's context ends,
		// returning ErrBackpressure — the queue-depth bound above is the
		// non-blocking one.
		rel, err := q.gate.Reserve(ctx)
		if err != nil {
			<-q.slots
			q.metrics.submit(ctx, kind.String(), "backpressure")
			return ErrBackpressure
		}
		gateRelease = rel
		if err := q.admitReceipt(ctx, rec, idempotency.Owner{Kind: agg.OwnerKind, ID: agg.OwnerID}); err != nil {
			rel()
			<-q.slots
			q.metrics.submit(ctx, kind.String(), "admission_failed")
			return err
		}
	}

	st := &recordState{
		key: recordKey{
			family:  rec.GetOperationFamily(),
			ownerID: agg.OwnerID,
			opID:    opIDOf(rec),
		},
		agg:      agg,
		kind:     kind,
		owner:    idempotency.Owner{Kind: agg.OwnerKind, ID: agg.OwnerID},
		frozen:   proto.Clone(rec).(*journalv1.DurableCommandRecord),
		refs:     refQueued,
		release:  gateRelease,
		slotHeld: true,
	}
	st.frozen.AdmissionSequence = q.nextSeq()

	q.mu.Lock()
	if _, dup := q.records[st.key]; dup {
		// Crash retry while the original is still live: dedupe on the
		// (family, owner, operation) identity — the in-flight record
		// settles once.
		q.mu.Unlock()
		if gateRelease != nil {
			gateRelease()
		}
		<-q.slots
		return nil
	}
	q.records[st.key] = st
	if subject, gated := q.destructiveSubjectLocked(st); gated && q.subjectRefsLocked(subject) > 0 {
		q.deferred = append(q.deferred, st)
	} else {
		q.enqueueLaneLocked(st)
	}
	q.pumpLanesLocked()
	q.mu.Unlock()
	q.metrics.submit(ctx, kind.String(), "ok")
	q.emitDepth(ctx)
	return nil
}

// Ack waits until the command committed and every queue/in-flight/journal
// reference is disposed, then marks the receipt's disposition ack
// (ids.md). Journal references held by a FrozenInventory must first be
// released through ReleaseInventory.
func (q *Queue) Ack(ctx context.Context, family string, owner idempotency.Owner, opID id.UUID) error {
	key := recordKey{family: family, ownerID: owner.ID, opID: opID}
	started := q.now()
	defer func() { q.metrics.wait(ctx, q.now().Sub(started).Milliseconds()) }()
	for {
		q.mu.Lock()
		st, ok := q.records[key]
		if !ok || st.refs == 0 {
			q.mu.Unlock()
			break
		}
		q.mu.Unlock()
		select {
		case <-q.notify:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return q.store.Acknowledge(ctx, family, owner, opID)
}

// ErasureFence establishes the erasure admission fence on one subject
// owner: cancels its uncommitted queued commands (ADMITTED receipts to
// terminal REJECTED, non-executing), rejects subsequent submits for that
// owner, and defers destructive JOB work keyed to the subject until every
// remaining subject reference is disposed (PRIV-008/JRN-008).
func (q *Queue) ErasureFence(ownerID id.UUID) error {
	q.mu.Lock()
	q.fenced[ownerID] = struct{}{}
	var cancel []*recordState
	for _, st := range q.records {
		if st.agg.OwnerID != ownerID || st.refs&refInflight != 0 {
			continue
		}
		if q.cancelQueuedLocked(st) {
			cancel = append(cancel, st)
		}
	}
	q.mu.Unlock()
	for _, st := range cancel {
		if st.kind == ProducerClient {
			_ = q.terminalizeReceipt(context.Background(), st,
				idempotency.ReceiptRejected, "ERASURE_FENCED")
		}
	}
	q.signal()
	return nil
}

// Freeze returns the immutable snapshot of every queued + in-flight +
// journal-referenced record for the composition-root journal writer. Each
// returned record gains a journal reference; ReleaseInventory disposes it
// after the writer durably publishes.
func (q *Queue) Freeze() *FrozenInventory {
	inv := &FrozenInventory{}
	q.mu.Lock()
	for _, st := range q.records {
		if st.refs == 0 {
			continue
		}
		st.refs |= refJournal
		inv.keys = append(inv.keys, st.key)
		inv.Records = append(inv.Records, proto.Clone(st.frozen).(*journalv1.DurableCommandRecord))
	}
	q.mu.Unlock()
	return inv
}

// ReleaseInventory disposes the journal references one Freeze snapshot
// pinned, after the writer durably published the outbox journal.
func (q *Queue) ReleaseInventory(inv *FrozenInventory) {
	q.mu.Lock()
	for _, key := range inv.keys {
		if st, ok := q.records[key]; ok {
			st.refs &^= refJournal
			q.disposeCheckLocked(st)
		}
	}
	q.mu.Unlock()
	q.signal()
}

// Live reports whether (family, owner, opID) still has a
// queue/in-flight/journal reference — the ReconcileOrphans predicate.
func (q *Queue) Live(family string, owner idempotency.Owner, opID id.UUID) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	st, ok := q.records[recordKey{family: family, ownerID: owner.ID, opID: opID}]
	return ok && st.refs > 0
}

// Shutdown stops new submissions, cancels in-flight execution contexts,
// and returns when workers drain (ctx bounds the drain). Unresolved
// records stay in the inventory for Freeze().
func (q *Queue) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	q.closed = true
	close(q.work)
	q.cancelCtx()
	q.mu.Unlock()
	done := make(chan struct{})
	go func() { q.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---------------------------------------------------------------------------
// internal

func (q *Queue) nextSeq() uint64 {
	q.mu.Lock()
	q.seq++
	s := q.seq
	q.mu.Unlock()
	return s
}

func (q *Queue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// enqueueLaneLocked appends st to its aggregate lane.
func (q *Queue) enqueueLaneLocked(st *recordState) {
	l, ok := q.lanes[st.agg]
	if !ok {
		l = &lane{}
		q.lanes[st.agg] = l
	}
	l.queue = append(l.queue, st)
}

// pumpLanesLocked dispatches each lane's head while a worker channel slot
// is free. Only lane heads are dispatched: per-aggregate FIFO is preserved.
// Heads honoring a retry delay wait for their timer's pump instead.
func (q *Queue) pumpLanesLocked() {
	if q.closed {
		return
	}
	now := time.Now()
	for _, l := range q.lanes {
		for !l.running && len(l.queue) > 0 {
			head := l.queue[0]
			if head.retryNotBefore.After(now) {
				break
			}
			select {
			case q.work <- head:
				l.running = true
			default:
				return
			}
		}
	}
}

// finishHeadLocked pops the executed lane head and redispatches.
func (q *Queue) finishHeadLocked(st *recordState) {
	l := q.lanes[st.agg]
	if l != nil && len(l.queue) > 0 && l.queue[0] == st {
		l.queue = l.queue[1:]
		l.running = false
	}
}

// cancelQueuedLocked removes an uncommitted record from its lane or the
// deferred list and releases its references. A record dispatched to the
// worker channel but not yet executing is still refQueued and cancels
// here; the worker discards it on pickup. Returns true if cancelled.
func (q *Queue) cancelQueuedLocked(st *recordState) bool {
	if st.refs&refQueued == 0 {
		return false
	}
	if l := q.lanes[st.agg]; l != nil {
		for i, r := range l.queue {
			if r == st {
				l.queue = append(l.queue[:i], l.queue[i+1:]...)
				if i == 0 && l.running {
					l.running = false
				}
				break
			}
		}
	}
	for i, r := range q.deferred {
		if r == st {
			q.deferred = append(q.deferred[:i], q.deferred[i+1:]...)
			break
		}
	}
	st.refs &^= refQueued
	st.resolved = true
	q.disposeCheckLocked(st)
	return true
}

// disposeCheckLocked releases the capacity slot and drops the record once
// every reference is disposed.
func (q *Queue) disposeCheckLocked(st *recordState) {
	if st.refs != 0 {
		return
	}
	if st.release != nil {
		st.release()
		st.release = nil
	}
	if st.slotHeld {
		st.slotHeld = false
		select {
		case <-q.slots:
		default:
		}
	}
	if cur, ok := q.records[st.key]; ok && cur == st {
		delete(q.records, st.key)
	}
}

// subjectRefsLocked counts live references to commands owned by one
// subject — the destructive-worker gate input.
func (q *Queue) subjectRefsLocked(subject id.UUID) int {
	n := 0
	for _, st := range q.records {
		if st.agg.OwnerID == subject && st.refs > 0 {
			n++
		}
	}
	return n
}

// destructiveSubjectLocked identifies a JOB record as the destructive
// erasure worker for a fenced subject: its job_key names the subject UUID
// (ids.md job_key components carry typed identity). ERASURE_RESUME
// continuation records are ProducerErasureResume and never gated here.
func (q *Queue) destructiveSubjectLocked(st *recordState) (id.UUID, bool) {
	if st.kind != ProducerJob || len(q.fenced) == 0 {
		return id.UUID{}, false
	}
	jobKey := st.frozen.GetJob().GetJobKey()
	if jobKey == "" {
		return id.UUID{}, false
	}
	for subject := range q.fenced {
		if strings.Contains(jobKey, subject.String()) {
			return subject, true
		}
	}
	return id.UUID{}, false
}

// retryDeferredLocked moves deferred jobs whose subject refs cleared back
// into their lanes.
func (q *Queue) retryDeferredLocked() {
	if len(q.deferred) == 0 {
		return
	}
	rest := q.deferred[:0]
	for _, st := range q.deferred {
		subject, _ := q.destructiveSubjectLocked(st)
		if q.subjectRefsLocked(subject) > 0 {
			rest = append(rest, st)
			continue
		}
		q.enqueueLaneLocked(st)
	}
	q.deferred = rest
}

// worker runs one executor pool member. The queued → in-flight
// transition happens on pickup so a dispatched-but-unstarted record is
// still cancellable; a cancelled/duplicate delivery frees the lane.
func (q *Queue) worker() {
	defer q.wg.Done()
	for st := range q.work {
		q.mu.Lock()
		cur, ok := q.records[st.key]
		if !ok || cur != st || st.resolved || st.refs&refQueued == 0 {
			q.finishHeadLocked(st)
			q.pumpLanesLocked()
			q.mu.Unlock()
			continue
		}
		st.refs &^= refQueued
		st.refs |= refInflight
		if st.release != nil {
			st.release()
			st.release = nil
		}
		q.mu.Unlock()
		q.execute(st)
	}
}

// resolveLocked finishes a record after execution resolved: pops the lane
// head, drops the in-flight reference, and re-checks deferred gates.
func (q *Queue) resolveLocked(st *recordState) {
	q.finishHeadLocked(st)
	st.refs &^= refInflight
	st.resolved = true
	q.disposeCheckLocked(st)
	q.retryDeferredLocked()
	q.pumpLanesLocked()
}

// requeueLocked keeps a transiently failed record at its lane head and
// schedules redispatch after the retry delay (same operation ID — the
// store dedupes a mid-flight commit).
func (q *Queue) requeueLocked(st *recordState) {
	if q.closed {
		// Draining: leave the record unresolved for the frozen inventory.
		st.refs &^= refInflight
		st.refs |= refQueued
		if l := q.lanes[st.agg]; l != nil {
			l.running = false
		}
		q.retryDeferredLocked()
		return
	}
	st.refs &^= refInflight
	st.refs |= refQueued
	st.retryNotBefore = time.Now().Add(q.retryDelay)
	if l := q.lanes[st.agg]; l != nil {
		l.running = false
	}
	delay := q.retryDelay
	time.AfterFunc(delay, func() {
		q.mu.Lock()
		if _, ok := q.records[st.key]; ok && st.refs&refQueued != 0 {
			q.pumpLanesLocked()
		}
		q.mu.Unlock()
	})
}

var errNoExecutor = errors.New("queue: no executor registered for producer kind")

func opIDOf(rec *journalv1.DurableCommandRecord) id.UUID {
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	return opID
}

func fingerprintOf(rec *journalv1.DurableCommandRecord) [32]byte {
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	return fp
}
