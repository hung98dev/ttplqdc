package session

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

// testClock is a manually advanced clock for the injectable Now/After.
type testClock struct {
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at  time.Time
	f   func()
	off bool
}

func (c *testClock) Now() time.Time { return c.now }

// After implements Config.After — timers fire on Advance.
func (c *testClock) After(d time.Duration, f func()) Timer {
	t := &fakeTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock and fires due timers.
func (c *testClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
	var run []*fakeTimer
	for _, t := range c.timers {
		if !t.off && !c.now.Before(t.at) {
			t.off = true
			run = append(run, t)
		}
	}
	for _, t := range run {
		t.f()
	}
}

func (t *fakeTimer) Stop() bool {
	if t.off {
		return false
	}
	t.off = true
	return true
}

// newTestStore runs migrations on a fresh database and returns the
// account store + pool.
func newTestStore(t *testing.T) (*account.Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	dsn := pgtest.FreshDB(t)
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot(t)), "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return account.NewStore(pool), ctx
}

// newTestRegistry builds a Registry with a fake clock + real durable
// queue (executors run against the migrated pool).
func newTestRegistry(t *testing.T, capacity int) (*Registry, *account.Store, *testClock) {
	t.Helper()
	store, ctx := newTestStore(t)
	clk := &testClock{now: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}
	q := queue.New(capacity, idempotency.NewStore(store.Pool()),
		queue.Deps{Pool: store.Pool(), Now: clk.Now})
	q.RegisterExecutor(queue.ProducerActivity, store.ActivityExecutor)
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = q.Shutdown(c)
	})
	reg := New(Config{
		Capacity:        capacity,
		ContentRevision: strings.Repeat("a", 64),
		Now:             clk.Now,
		After:           clk.After,
	}, store, q)
	_ = ctx
	return reg, store, clk
}

// seedAccount inserts one ACTIVE account row.
func seedAccount(t *testing.T, store *account.Store) id.UUID {
	t.Helper()
	ctx := context.Background()
	accID := id.NewV7(time.Now().UTC())
	if err := store.CreateAccount(ctx, nil, accID, time.Now().UTC()); err != nil {
		t.Fatalf("create account: %v", err)
	}
	return accID
}

// seedCharacter inserts one character owned by the account.
func seedCharacter(t *testing.T, store *account.Store, accountID id.UUID, name string) id.UUID {
	t.Helper()
	ctx := context.Background()
	charID := id.NewV7(time.Now().UTC())
	_, err := store.Pool().Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, map_id)
		 VALUES ($1,$2,$3,$4,'class.kim',1,'map.lang_da.dinh_lang')`,
		charID, accountID, name, name)
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	return charID
}

// ---------------------------------------------------------------------------
// WS client harness

// wsEnv writes one envelope.
func wsEnv(t *testing.T, c *websocket.Conn, msgID uint32, epoch, seq uint64, m proto.Message) {
	t.Helper()
	payload, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	env, err := proto.Marshal(&protocolv1.Envelope{
		ProtocolMajor: 1, MessageId: msgID, SessionEpoch: epoch,
		ClientSeq: seq, Payload: payload,
	})
	if err != nil {
		t.Fatalf("marshal env: %v", err)
	}
	if err := c.Write(context.Background(), websocket.MessageBinary, env); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

// wsRead reads one envelope.
func wsRead(t *testing.T, c *websocket.Conn) *protocolv1.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	typ, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("ws frame type %v", typ)
	}
	var env protocolv1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal env: %v", err)
	}
	return &env
}

// wsDial opens a WS connection to the listener's /ws.
func wsDial(t *testing.T, addr string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws://"+addr+"/ws", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { c.Close(websocket.StatusNormalClosure, "done") })
	return c
}

// wsHello exchanges C2S_HELLO for the HELLO_OK envelope (or the reject).
func wsHello(t *testing.T, c *websocket.Conn, h *protocolv1.C2SHello) *protocolv1.Envelope {
	t.Helper()
	wsEnv(t, c, 1, 0, 1, h)
	// HELLO_OK (2) or S2C_ERROR (3); skip interleaved heartbeat frames.
	for i := 0; i < 16; i++ {
		env := wsRead(t, c)
		if env.MessageId == 2 || env.MessageId == 3 {
			return env
		}
	}
	t.Fatal("no hello response")
	return nil
}

// startListener runs a real listener on a loopback port wired to the
// registry; returns the dial address.
func startListener(t *testing.T, reg *Registry) string {
	t.Helper()
	return startListenerWindow(t, reg, 0)
}

// startListenerWindow overrides the HELLO window (0 = spec default).
func startListenerWindow(t *testing.T, reg *Registry, helloWindow time.Duration) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	l, err := listener.New(listener.Config{
		Addr:          ln.Addr().String(),
		TLS:           listener.TerminationProxy,
		ProtocolMajor: 1,
		HelloWindow:   helloWindow,
		// Keep the heartbeat timeout far above slow postgres ops so the
		// listener does not reap test conns mid-assertion; heartbeat
		// cadence is long so frames do not interleave assertions.
		ConnTimeout:       5 * time.Minute,
		HeartbeatInterval: 2 * time.Minute,
	}, listener.Deps{
		Session:    reg,
		Intents:    reg,
		Disconnect: reg,
		RTT:        reg,
	})
	if err != nil {
		t.Fatalf("listener: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = l.ServeListener(ctx, ln) }()
	t.Cleanup(func() {
		c2, c2cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer c2cancel()
		_ = l.Drain(c2)
	})
	return ln.Addr().String()
}

// attach issues C2S_CHARACTER_ATTACH and reads the ATTACH_OK envelope.
func wsAttach(t *testing.T, c *websocket.Conn, epoch, seq uint64, charID id.UUID) *protocolv1.Envelope {
	t.Helper()
	wsEnv(t, c, 6, epoch, seq, &protocolv1.C2SCharacterAttach{CharacterId: charID[:]})
	return wsRead(t, c)
}

// repoRoot resolves the repository root from the test's cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	return "../../../.."
}

// wsReadOrErr is wsRead that tolerates a closed connection.
func wsReadOrErr(t *testing.T, c *websocket.Conn) *protocolv1.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	typ, data, err := c.Read(ctx)
	if err != nil {
		return nil
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("ws frame type %v", typ)
	}
	var env protocolv1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal env: %v", err)
	}
	return &env
}

// testCtx is the background context for session tests.
func testCtx() context.Context { return context.Background() }

// waitActive polls character_activity.session_active until it matches.
func waitActive(t *testing.T, store *account.Store, charID id.UUID, want bool) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var active bool
		err := store.Pool().QueryRow(ctx,
			`SELECT session_active FROM character_activity WHERE character_id = $1`,
			charID).Scan(&active)
		if err == nil && active == want {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("session_active never became %v", want)
}

// readUntil reads envelopes until one carries msgID (10 s deadline each
// read — helper for sequences like 14→7).
func wsReadUntil(t *testing.T, c *websocket.Conn, msgID uint32) *protocolv1.Envelope {
	t.Helper()
	for i := 0; i < 16; i++ {
		env := wsRead(t, c)
		if env.MessageId == msgID {
			return env
		}
	}
	t.Fatalf("never saw message id %d", msgID)
	return nil
}

// helloTicket builds the C2S_HELLO for a gameplay ticket.
func helloTicket(cred string, build uint32) *protocolv1.C2SHello {
	return &protocolv1.C2SHello{
		ClientBuild: build,
		Credential:  &protocolv1.C2SHello_GameplayTicket{GameplayTicket: cred},
	}
}

// helloResume builds the C2S_HELLO for a resume credential.
func helloResume(cred string, build uint32) *protocolv1.C2SHello {
	return &protocolv1.C2SHello{
		ClientBuild: build,
		Credential:  &protocolv1.C2SHello_ResumeCredential{ResumeCredential: cred},
	}
}
