package runtime_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/global/runtime"
	"thinhthan/internal/testing/pgtest"
)

var (
	sharedDSN string
	setupErr  error
)

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
	switch {
	case errors.Is(err, pgtest.ErrUnavailable):
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
		os.Exit(m.Run())
	case err != nil:
		setupErr = err
		os.Exit(m.Run())
	}
	name := fmt.Sprintf("global_runtime_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot()), "up"); err != nil {
		setupErr = err
	} else {
		sharedDSN = dsn
	}
	code := m.Run()
	cleanup()
	srv.Close()
	os.Exit(code)
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedDSN == "" {
		if setupErr != nil {
			t.Fatalf("postgres provisioning failed: %v", setupErr)
		}
		t.Skip("DEFERRED(local-missing): no postgres")
	}
	pool, err := pgxpool.New(context.Background(), sharedDSN)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// countHost mutates only on the writer goroutine (Restore at Start,
// OnCommand + command Execute during intake); no locks — -race fails the
// test if a second writer ever touches it.
type countHost struct {
	total    int
	executed int
	observed int
	restored int
}

func (h *countHost) Name() string { return "count" }

func (h *countHost) Restore(_ context.Context, _ *runtime.DurableView) error {
	h.restored++
	return nil
}

func (h *countHost) OnCommand(_ context.Context, _ runtime.Command) error {
	h.observed++
	return nil
}

// addCmd applies a read-modify-write; any second writer loses updates or
// trips -race.
type addCmd struct {
	host *countHost
	v    int
}

func (c *addCmd) Family() string { return "test.count" }

func (c *addCmd) Execute(_ context.Context, _ *runtime.Runtime) error {
	cur := c.host.total
	time.Sleep(0) // widen the race window
	c.host.total = cur + c.v
	c.host.executed++
	return nil
}

func TestSingleWriterSerialization(t *testing.T) {
	rt := runtime.New(runtime.Config{MailboxCap: 1024})
	h := &countHost{}
	if err := rt.RegisterHost(h, "test.count"); err != nil {
		t.Fatalf("RegisterHost: %v", err)
	}
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	const submitters, per = 8, 50
	var wg sync.WaitGroup
	results := make(chan (<-chan runtime.Result), submitters*per)
	for range submitters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range per {
				ch, err := rt.Submit(context.Background(), &addCmd{host: h, v: 1})
				if err != nil {
					t.Errorf("Submit: %v", err)
					return
				}
				results <- ch
			}
		}()
	}
	wg.Wait()
	close(results)
	for ch := range results {
		if res := <-ch; res.Err != nil {
			t.Fatalf("command result: %v", res.Err)
		}
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if h.executed != submitters*per || h.total != submitters*per {
		t.Fatalf("executed=%d total=%d, want %d", h.executed, h.total, submitters*per)
	}
	if h.observed != submitters*per {
		t.Fatalf("host observed %d commands, want %d", h.observed, submitters*per)
	}
}

// blockCmd parks inside Execute until released.
type blockCmd struct {
	entered  chan struct{}
	release  chan struct{}
	released bool
}

func (c *blockCmd) Family() string { return "test.block" }

func (c *blockCmd) Execute(ctx context.Context, _ *runtime.Runtime) error {
	select {
	case c.entered <- struct{}{}:
	default:
	}
	select {
	case <-c.release:
		c.released = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type noopHost struct{ name string }

func (h *noopHost) Name() string { return h.name }
func (h *noopHost) Restore(_ context.Context, _ *runtime.DurableView) error {
	return nil
}
func (h *noopHost) OnCommand(_ context.Context, _ runtime.Command) error { return nil }

func TestMailboxBackpressure(t *testing.T) {
	rt := runtime.New(runtime.Config{MailboxCap: 2})
	if err := rt.RegisterHost(&noopHost{name: "block"}, "test.block"); err != nil {
		t.Fatalf("RegisterHost: %v", err)
	}
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	blocked := &blockCmd{entered: make(chan struct{}, 1), release: make(chan struct{})}
	first, err := rt.Submit(context.Background(), blocked)
	if err != nil {
		t.Fatalf("Submit blocker: %v", err)
	}
	select {
	case <-blocked.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("writer never entered blocking command")
	}

	chans := make([]<-chan runtime.Result, 0, 2)
	fillers := make([]*blockCmd, 0, 2)
	for i := range 2 {
		f := &blockCmd{entered: make(chan struct{}, 1), release: make(chan struct{})}
		ch, err := rt.Submit(context.Background(), f)
		if err != nil {
			t.Fatalf("Submit filler %d: %v", i, err)
		}
		chans = append(chans, ch)
		fillers = append(fillers, f)
	}

	start := time.Now()
	if _, err := rt.Submit(context.Background(), &blockCmd{
		entered: make(chan struct{}, 1), release: make(chan struct{})}); !errors.Is(err, runtime.ErrMailboxFull) {
		t.Fatalf("full mailbox Submit = %v, want ErrMailboxFull", err)
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("full-mailbox Submit blocked %v — must return backpressure", d)
	}

	close(blocked.release)
	if res := <-first; res.Err != nil {
		t.Fatalf("blocker result: %v", res.Err)
	}
	for _, f := range fillers {
		close(f.release)
	}
	for i, ch := range chans {
		select {
		case res := <-ch:
			if res.Err != nil {
				t.Fatalf("filler %d result: %v", i, res.Err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("filler %d never resolved", i)
		}
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestSubmitLifecycleErrors(t *testing.T) {
	rt := runtime.New(runtime.Config{MailboxCap: 4})
	if err := rt.RegisterHost(&noopHost{name: "h"}, "fam"); err != nil {
		t.Fatalf("RegisterHost: %v", err)
	}
	cmd := runtime.Func("fam", func(_ context.Context, _ *runtime.Runtime) error { return nil })

	if _, err := rt.Submit(context.Background(), cmd); !errors.Is(err, runtime.ErrNotRunning) {
		t.Fatalf("pre-Start Submit = %v, want ErrNotRunning", err)
	}
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := rt.Submit(context.Background(),
		runtime.Func("unknown", func(_ context.Context, _ *runtime.Runtime) error { return nil })); !errors.Is(err, runtime.ErrUnknownCommandFamily) {
		t.Fatalf("unknown family Submit = %v, want ErrUnknownCommandFamily", err)
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if _, err := rt.Submit(context.Background(), cmd); !errors.Is(err, runtime.ErrShutdown) {
		t.Fatalf("post-Shutdown Submit = %v, want ErrShutdown", err)
	}
}

// orderHost appends to restored/seen from writer-owned contexts only.
type orderHost struct {
	name string
	log  *[]string
}

func (h *orderHost) Name() string { return h.name }
func (h *orderHost) Restore(_ context.Context, _ *runtime.DurableView) error {
	*h.log = append(*h.log, "restore:"+h.name)
	return nil
}
func (h *orderHost) OnCommand(_ context.Context, _ runtime.Command) error {
	*h.log = append(*h.log, "cmd:"+h.name)
	return nil
}

func TestRestoreBeforeIntake(t *testing.T) {
	rt := runtime.New(runtime.Config{MailboxCap: 8})
	log := []string{}
	for _, name := range []string{"h1", "h2"} {
		if err := rt.RegisterHost(&orderHost{name: name, log: &log}, "fam."+name); err != nil {
			t.Fatalf("RegisterHost %s: %v", name, err)
		}
	}
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(log) != 2 || log[0] != "restore:h1" || log[1] != "restore:h2" {
		t.Fatalf("restore order = %v, want [restore:h1 restore:h2] before intake", log)
	}
	if _, err := rt.Submit(context.Background(),
		runtime.Func("fam.h1", func(_ context.Context, _ *runtime.Runtime) error { return nil })); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if log[2] != "cmd:h1" {
		t.Fatalf("host never observed routed command: %v", log)
	}
}

func TestHostRestoreError(t *testing.T) {
	rt := runtime.New(runtime.Config{MailboxCap: 4})
	if err := rt.RegisterHost(&failHost{}, "fam"); err != nil {
		t.Fatalf("RegisterHost: %v", err)
	}
	if err := rt.Start(context.Background()); !errors.Is(err, runtime.ErrHostRestore) {
		t.Fatalf("Start = %v, want ErrHostRestore", err)
	}
	if _, err := rt.Submit(context.Background(),
		runtime.Func("fam", func(_ context.Context, _ *runtime.Runtime) error { return nil })); !errors.Is(err, runtime.ErrShutdown) {
		t.Fatalf("Submit after failed Start = %v, want ErrShutdown", err)
	}
}

type failHost struct{}

func (h *failHost) Name() string { return "fail" }
func (h *failHost) Restore(_ context.Context, _ *runtime.DurableView) error {
	return errors.New("durable read failed")
}
func (h *failHost) OnCommand(_ context.Context, _ runtime.Command) error { return nil }

func TestCommandPanicRecovered(t *testing.T) {
	rt := runtime.New(runtime.Config{MailboxCap: 4})
	if err := rt.RegisterHost(&noopHost{name: "h"}, "fam"); err != nil {
		t.Fatalf("RegisterHost: %v", err)
	}
	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	ch, err := rt.Submit(context.Background(),
		runtime.Func("fam", func(_ context.Context, _ *runtime.Runtime) error { panic("boom") }))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res := <-ch; res.Err == nil {
		t.Fatal("panic command resolved nil error")
	}
	// Writer survives the panic: a follow-up command still resolves.
	ch2, err := rt.Submit(context.Background(),
		runtime.Func("fam", func(_ context.Context, _ *runtime.Runtime) error { return nil }))
	if err != nil {
		t.Fatalf("Submit after panic: %v", err)
	}
	select {
	case res := <-ch2:
		if res.Err != nil {
			t.Fatalf("post-panic command: %v", res.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writer died on panic")
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// --- Durable bridge rebuild test (real queue.Queue + pgx) -------------

type bossRec struct {
	family string
	rec    *journalv1.DurableCommandRecord
}

func (c *bossRec) Family() string                          { return c.family }
func (c *bossRec) Record() *journalv1.DurableCommandRecord { return c.rec }

func newBossScheduleRec() *journalv1.DurableCommandRecord {
	opID := id.NewV7(time.Now())
	fpr := sha256.Sum256([]byte("boss-schedule" + opID.String()))
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    "boss.schedule",
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
		OwnerId:            id.WorldOwnerID[:],
		OperationId:        opID[:],
		EnqueuedAtMs:       time.Now().UnixMilli(),
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE,
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_PublicSchedule{
			PublicSchedule: &journalv1.JournalPublicSchedule{}},
	}
}

// pgReader adapts a real pgxpool to the runtime.Reader interface —
// allowed here because _test.go is exempt from the SQL-owner import gate
// (production wiring injects a durable-side implementation).
type pgReader struct{ pool *pgxpool.Pool }

func (r *pgReader) ReadTx(ctx context.Context, fn func(tx runtime.Tx) error) error {
	return pgx.BeginTxFunc(ctx, r.pool,
		pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error { return fn(pgxTx{tx}) })
}

type pgxTx struct{ tx pgx.Tx }

func (t pgxTx) QueryRow(ctx context.Context, query string, args ...any) runtime.Row {
	return t.tx.QueryRow(ctx, query, args...)
}

func (t pgxTx) Query(ctx context.Context, query string, args ...any) (runtime.Rows, error) {
	return t.tx.Query(ctx, query, args...)
}

// scheduleHost is the Durable-backed host: Restore counts committed
// boss.schedule operations; scheduled holds what this boot submitted.
type scheduleHost struct {
	restored    int
	committed   int
	restoreRows int
}

func (h *scheduleHost) Name() string { return "boss-schedule" }

func (h *scheduleHost) Restore(ctx context.Context, view *runtime.DurableView) error {
	h.restored++
	return view.ReadTx(ctx, func(tx runtime.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT count(*) FROM operations WHERE operation_family = 'boss.schedule'`).
			Scan(&h.restoreRows)
	})
}

func (h *scheduleHost) OnCommand(_ context.Context, _ runtime.Command) error { return nil }

func newDurableRuntime(t *testing.T) (*runtime.Runtime, *queue.Queue, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	store := idempotency.NewStore(pool)
	q := queue.New(64, store, queue.Deps{
		Pool:       pool,
		Workers:    4,
		BackoffMin: time.Millisecond,
		BackoffMax: 8 * time.Millisecond,
		Gate:       idempotency.NewBoundedGate(64),
	})
	rt := runtime.New(runtime.Config{Queue: q, Reader: &pgReader{pool}})
	return rt, q, pool
}

func TestRestartRebuildFromDurable(t *testing.T) {
	ctx := context.Background()
	rt, q, _ := newDurableRuntime(t)
	h1 := &scheduleHost{}
	if err := rt.RegisterHost(h1, "boss.schedule"); err != nil {
		t.Fatalf("RegisterHost: %v", err)
	}
	if err := rt.Bridge().RegisterExecutor(queue.ProducerPublicSchedule,
		func(_ context.Context, _ pgx.Tx, _ *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
			return idempotency.Outcome{SchemaVersion: 1, Payload: []byte(`{"ok":true}`)}, nil
		}); err != nil {
		t.Fatalf("RegisterExecutor: %v", err)
	}
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Shared test DB accumulates rows across runs; assert on the delta.
	base := h1.restoreRows

	commit, err := rt.Bridge().SubmitDurable(ctx,
		&bossRec{family: "boss.schedule", rec: newBossScheduleRec()})
	if err != nil {
		t.Fatalf("SubmitDurable: %v", err)
	}
	select {
	case c := <-commit:
		if c.Err != nil {
			t.Fatalf("commit: %v", c.Err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("commit never resolved")
	}
	h1.committed++
	if err := rt.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := q.Shutdown(ctx); err != nil {
		t.Fatalf("queue shutdown: %v", err)
	}

	// Restart: new runtime + new queue over the same durable store; state
	// must come back from Durable only (no carry-over memory).
	rt2, q2, _ := newDurableRuntime(t)
	h2 := &scheduleHost{}
	if err := rt2.RegisterHost(h2, "boss.schedule"); err != nil {
		t.Fatalf("RegisterHost rt2: %v", err)
	}
	if err := rt2.Bridge().RegisterExecutor(queue.ProducerPublicSchedule,
		func(_ context.Context, _ pgx.Tx, _ *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
			return idempotency.Outcome{SchemaVersion: 1, Payload: []byte(`{"ok":true}`)}, nil
		}); err != nil {
		t.Fatalf("RegisterExecutor rt2: %v", err)
	}
	if err := rt2.Start(ctx); err != nil {
		t.Fatalf("Start rt2: %v", err)
	}
	if h2.restoreRows != base+1 {
		t.Fatalf("restart restored %d boss.schedule rows, want %d", h2.restoreRows, base+1)
	}
	if h2.committed != 0 {
		t.Fatalf("ephemeral state leaked across restart: committed=%d", h2.committed)
	}
	if err := rt2.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown rt2: %v", err)
	}
	if err := q2.Shutdown(ctx); err != nil {
		t.Fatalf("queue2 shutdown: %v", err)
	}
}
