// Package runtime is the in-process Global single-writer actor
// (service_boundaries.md § Ephemeral Global Runtime): one writer
// goroutine owns all ephemeral global state; every mutation passes a
// bounded mailbox; durable work crosses only through the typed
// DurableBridge over durable/queue — hosts never import pgx or SQL.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"thinhthan/internal/durable/queue"
)

// DefaultMailboxCap bounds the command mailbox (concurrency.md: every
// cross-goroutine queue declares capacity + overflow behavior).
const DefaultMailboxCap = 1024

// drainBudget bounds the in-flight flush on Shutdown.
const drainBudget = 5 * time.Second

// Config wires the Runtime. Queue is the durable command queue (IMP-082)
// the DurableBridge submits through; Reader backs DurableView reads
// during host Restore. Either may be nil in tests that avoid durable
// access.
type Config struct {
	MailboxCap int
	Queue      *queue.Queue
	Reader     Reader
}

const (
	stateNew int32 = iota
	stateRunning
	stateDraining
	stateStopped
)

// envelope is one mailbox slot.
type envelope struct {
	cmd    Command
	result chan Result
}

// Runtime is the Global actor: single writer goroutine + bounded mailbox
// + host registry + Durable bridge. Start restores every host's durable
// state before intake opens; Shutdown stops intake and flushes bounded.
type Runtime struct {
	cfg    Config
	mbox   chan *envelope
	mu     sync.Mutex
	state  int32
	hosts  []Host
	routes map[string]Host
	bridge *DurableBridge
	view   *DurableView
	wg     sync.WaitGroup

	execCtx    context.Context
	execCancel context.CancelFunc
}

// New builds the Runtime; intake stays closed until Start.
func New(cfg Config) *Runtime {
	mboxCap := cfg.MailboxCap
	if mboxCap <= 0 {
		mboxCap = DefaultMailboxCap
	}
	r := &Runtime{
		cfg:    cfg,
		mbox:   make(chan *envelope, mboxCap),
		state:  stateNew,
		routes: map[string]Host{},
	}
	if cfg.Queue != nil {
		r.bridge = newDurableBridge(cfg.Queue)
	}
	r.view = &DurableView{bridge: r.bridge, reader: cfg.Reader}
	return r
}

// Bridge returns the typed Durable bridge hosts/commands submit through.
// Nil when Config.Queue was nil.
func (r *Runtime) Bridge() *DurableBridge { return r.bridge }

// Start restores all registered hosts from Durable (registration order)
// then spawns the single writer goroutine. A restore failure fails the
// start closed — intake never opens on partial state.
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.state != stateNew {
		r.mu.Unlock()
		return errors.New("global: runtime already started")
	}
	r.execCtx, r.execCancel = context.WithCancel(context.Background())
	r.mu.Unlock()

	if err := r.restoreAll(ctx); err != nil {
		r.execCancel()
		r.mu.Lock()
		r.state = stateStopped
		r.mu.Unlock()
		return err
	}
	if r.bridge != nil {
		r.bridge.start(r.execCtx)
	}
	r.wg.Add(1)
	go r.writer(r.execCtx)

	r.mu.Lock()
	r.state = stateRunning
	r.mu.Unlock()
	return nil
}

// Submit enqueues cmd for the writer goroutine without blocking: a full
// mailbox returns ErrMailboxFull (typed backpressure — never blocks a
// simulation tick). Callers observe the outcome on the returned channel.
func (r *Runtime) Submit(_ context.Context, cmd Command) (<-chan Result, error) {
	if cmd == nil {
		return nil, errors.New("global: nil command")
	}
	env := &envelope{cmd: cmd, result: make(chan Result, 1)}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch r.state {
	case stateNew:
		return nil, ErrNotRunning
	case stateDraining, stateStopped:
		return nil, ErrShutdown
	}
	if _, ok := r.routes[cmd.Family()]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownCommandFamily, cmd.Family())
	}
	select {
	case r.mbox <- env:
		return env.result, nil
	default:
		return nil, ErrMailboxFull
	}
}

// Shutdown stops intake, waits (bounded by ctx) for the writer to drain
// in-flight mailbox entries, then flushes bridge commit waiters and
// closes host loops via the released exec context.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	switch r.state {
	case stateNew:
		r.state = stateStopped
		r.mu.Unlock()
		return nil
	case stateStopped:
		r.mu.Unlock()
		return nil
	}
	r.state = stateDraining
	r.mu.Unlock()

	r.execCancel()
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.bridge != nil {
		if err := r.bridge.close(ctx); err != nil {
			return err
		}
	}
	r.mu.Lock()
	r.state = stateStopped
	r.mu.Unlock()
	return nil
}

// writer is the single mutation goroutine.
func (r *Runtime) writer(ctx context.Context) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done():
			r.drain()
			return
		case env := <-r.mbox:
			r.process(ctx, env)
		}
	}
}

// drain flushes entries already in the mailbox under a bounded budget;
// entries that outlive the budget resolve with ErrShutdown.
func (r *Runtime) drain() {
	dctx, cancel := context.WithTimeout(context.Background(), drainBudget)
	defer cancel()
	for {
		select {
		case env := <-r.mbox:
			if dctx.Err() != nil {
				env.result <- Result{Err: ErrShutdown}
				continue
			}
			r.process(dctx, env)
		default:
			return
		}
	}
}

// process executes one command on the writer goroutine: Execute first,
// then the routed host's OnCommand callback. A panic recovers into an
// error Result — the writer goroutine never dies mid-command.
func (r *Runtime) process(ctx context.Context, env *envelope) {
	var err error
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("global: command panic: %v", p)
		}
		env.result <- Result{Err: err}
	}()
	err = env.cmd.Execute(ctx, r)
	if err != nil {
		return
	}
	if h := r.routes[env.cmd.Family()]; h != nil {
		err = h.OnCommand(ctx, env.cmd)
	}
}
