package chat

import (
	"context"
	"sync"
	"time"

	journalv1 "thinhthan/internal/durable/journal/v1"
)

// Submitter is the queue's non-blocking accept path — Submit returns
// once the record is durably enqueued, not when the row is applied.
// The composition root injects *queue.Queue.
type Submitter interface {
	Submit(ctx context.Context, rec *journalv1.DurableCommandRecord) error
}

// Writer is the async bounded-batch producer the chat delivery path
// feeds (social.md § Chat History: moderation logs persist
// asynchronously; delivery never waits on them). Enqueue appends to an
// in-memory buffer; a background flusher submits records to the queue
// in batches of at most BatchBound or when FlushInterval elapses.
// Enqueue never blocks the caller beyond an append: once the buffer
// reaches bufferCap the entry is dropped and counted (delivery always
// wins over logging completeness under overload).
type Writer struct {
	sub         Submitter
	mu          sync.Mutex
	buf         []Entry
	dropped     uint64
	submitted   uint64
	flushEvery  time.Duration
	stop        chan struct{}
	stopped     chan struct{}
	enqueueFail func(error)
}

// BatchBound caps one flush — the "bounded" of the bounded-batch rule.
const BatchBound = 64

// bufferCap bounds memory the writer may hold between flushes.
const bufferCap = 4 * BatchBound

// DefaultFlushEvery is the flush cadence between full batches.
const DefaultFlushEvery = 250 * time.Millisecond

// NewWriter starts the bounded-batch flusher. flushEvery <= 0 uses
// DefaultFlushEvery. The optional fail hook observes submit errors
// (metrics/logging); nil is fine.
func NewWriter(sub Submitter, flushEvery time.Duration,
	fail func(error)) *Writer {
	if flushEvery <= 0 {
		flushEvery = DefaultFlushEvery
	}
	w := &Writer{
		sub:         sub,
		flushEvery:  flushEvery,
		stop:        make(chan struct{}),
		stopped:     make(chan struct{}),
		enqueueFail: fail,
	}
	go w.run()
	return w
}

// Enqueue buffers one accepted message. It never waits on the durable
// bound; on buffer saturation the entry is dropped and counted.
func (w *Writer) Enqueue(e Entry) {
	w.mu.Lock()
	if len(w.buf) >= bufferCap {
		w.dropped++
		w.mu.Unlock()
		return
	}
	w.buf = append(w.buf, e)
	w.mu.Unlock()
}

// Dropped/Submitted are the load counters an ops metric can read.
func (w *Writer) Dropped() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dropped
}

func (w *Writer) Submitted() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.submitted
}

// Flush pushes every buffered entry now (returns after submits are
// accepted, not applied).
func (w *Writer) Flush(ctx context.Context) {
	for {
		batch := w.take(BatchBound)
		if len(batch) == 0 {
			return
		}
		w.submitBatch(ctx, batch)
	}
}

// Close drains the buffer and stops the flusher.
func (w *Writer) Close(ctx context.Context) {
	close(w.stop)
	<-w.stopped
	w.Flush(ctx)
}

func (w *Writer) run() {
	defer close(w.stopped)
	t := time.NewTicker(w.flushEvery)
	defer t.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			w.Flush(context.Background())
		}
	}
}

func (w *Writer) take(n int) []Entry {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return nil
	}
	if n > len(w.buf) {
		n = len(w.buf)
	}
	batch := make([]Entry, n)
	copy(batch, w.buf[:n])
	w.buf = w.buf[n:]
	return batch
}

func (w *Writer) submitBatch(ctx context.Context, batch []Entry) {
	for _, e := range batch {
		rec, err := NewRecord(e)
		if err != nil {
			w.fail(err)
			continue
		}
		if err := w.sub.Submit(ctx, rec); err != nil {
			w.fail(err)
			continue
		}
		w.mu.Lock()
		w.submitted++
		w.mu.Unlock()
	}
}

func (w *Writer) fail(err error) {
	if w.enqueueFail != nil {
		w.enqueueFail(err)
	}
}
