package progression

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	progressiond "thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	"thinhthan/internal/edge/session"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

// fakeConsult is the test-side ConsultPort stub: verdict/error are
// settable and every call is recorded for assertion.
type fakeConsult struct {
	mu    sync.Mutex
	ok    bool
	err   error
	calls int
	last  [3]string
}

func (f *fakeConsult) NpcServiceValid(characterID, npcID, serviceID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.last = [3]string{characterID, npcID, serviceID}
	return f.ok, f.err
}

func (f *fakeConsult) set(ok bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ok, f.err = ok, err
}

func (f *fakeConsult) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// env wires the full ADR-0081 stack for one test: migrated pool, durable
// queue (activity + the ProducerClient family-mux over Executors),
// router with the 511–513 handlers, session registry.
type env struct {
	sessions *session.Registry
	accounts *account.Store
	router   *router.Registry
	q        *queue.Queue
	store    *progressiond.Store
	svc      *Service
	consults *fakeConsult
}

func newEnv(t *testing.T, capacity int, consults *fakeConsult) *env {
	t.Helper()
	ctx := context.Background()
	dsn := pgtest.FreshDB(t)
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir("../../../.."), "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	p, err := db.Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(p.Close)

	accounts := account.NewStore(p)
	pstore := progressiond.NewStore(p)
	var cp ConsultPort // nil fakeConsult must stay an unbound port
	if consults != nil {
		cp = consults
	}
	q := queue.New(capacity, idempotency.NewStore(p),
		queue.Deps{Pool: p, Now: func() time.Time { return time.Now().UTC() }})
	q.RegisterExecutor(queue.ProducerActivity, accounts.ActivityExecutor)
	// The composition ProducerClient family-mux over the exported
	// Executors map — the production binding lives in app/ wiring.
	execs := progressiond.Executors(pstore)
	q.RegisterExecutor(queue.ProducerClient,
		func(ctx context.Context, tx pgx.Tx, rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
			ex := execs[rec.GetOperationFamily()]
			if ex == nil {
				return idempotency.Outcome{}, fmt.Errorf("no executor for %q", rec.GetOperationFamily())
			}
			return ex(ctx, tx, rec)
		})
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = q.Shutdown(c)
	})

	rt := router.New()
	svc := New(q, pstore, cp)
	if err := svc.Register(rt); err != nil {
		t.Fatalf("register: %v", err)
	}
	sessions := session.New(session.Config{
		Capacity:        capacity,
		ContentRevision: strings.Repeat("a", 64),
		Now:             func() time.Time { return time.Now().UTC() },
		After: func(d time.Duration, f func()) session.Timer {
			return testTimer{time.AfterFunc(d, f)}
		},
	}, accounts, q)
	sessions.SetRouter(rt)
	return &env{sessions: sessions, accounts: accounts, router: rt,
		q: q, store: pstore, consults: consults, svc: svc}
}

// intentTap wraps the session intent sink so tests can drive the
// phase transition production gets from the world baseline (300 —
// IMP-018 has not landed): the conn enters IN_WORLD on attach exactly
// as it would once the baseline is delivered.
//
// The phase must flip BEFORE the attach is dispatched, not after it
// returns: r.attach only offers S2C_CHARACTER_ATTACH_OK to the outbound
// queue, so the write loop can flush the 7 and the client can fire its
// next intent while this goroutine is still between the Enqueue return
// and SetPhase. The read loop validates that intent concurrently — a
// post-Enqueue flip leaves a window where the conn still reads
// CHARACTER_SELECT and a legal in-world op (511/512/513) is rejected
// MESSAGE_NOT_ALLOWED_IN_STATE. Flipping first matches production's
// ordering (real transitions latch at Send offer-time, before the wire).
type intentTap struct {
	inner listener.IntentSink
}

func (it *intentTap) Enqueue(ctx context.Context, c *listener.Conn, f listener.Inbound) error {
	if f.MessageID == 6 {
		c.SetPhase(listener.PhaseInWorld)
	}
	return it.inner.Enqueue(ctx, c, f)
}

type testTimer struct{ *time.Timer }

func (t testTimer) Stop() bool { return t.Timer.Stop() }

func (e *env) seedAccount(t *testing.T) id.UUID {
	t.Helper()
	acct := id.NewV7(time.Now().UTC())
	if err := e.accounts.CreateAccount(context.Background(), nil, acct, time.Now().UTC()); err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acct
}

// seedCharacter inserts a live character with explicit progression
// columns.
func (e *env) seedCharacter(t *testing.T, acct id.UUID, level int32,
	skillPts, potPts int32) id.UUID {
	t.Helper()
	charID := id.NewV7(time.Now().UTC())
	if _, err := e.accounts.Pool().Exec(context.Background(),
		`INSERT INTO characters
		 (character_id, account_id, name, name_key, class_id, level,
		  unspent_skill_points, unspent_potential_points)
		 VALUES ($1,$2,$3,$4,'class.kim',$5,$6,$7)`,
		charID.String(), acct.String(), "Prog "+charID.String()[:8], charID.String(),
		level, skillPts, potPts); err != nil {
		t.Fatalf("seed character: %v", err)
	}
	return charID
}

// dial opens a WS connection and completes the gameplay HELLO; returns
// the conn + negotiated session epoch.
func (e *env) dial(t *testing.T, acct id.UUID) (*websocket.Conn, uint64) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	l, err := listener.New(listener.Config{
		Addr:              ln.Addr().String(),
		TLS:               listener.TerminationProxy,
		ProtocolMajor:     1,
		HelloWindow:       0,
		ConnTimeout:       5 * time.Minute,
		HeartbeatInterval: 2 * time.Minute,
	}, listener.Deps{
		Session:    e.sessions,
		Intents:    &intentTap{inner: e.sessions},
		Disconnect: e.sessions,
		RTT:        e.sessions,
	})
	if err != nil {
		t.Fatalf("listener: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = l.ServeListener(ctx, ln) }()
	t.Cleanup(func() {
		c, c2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer c2()
		_ = l.Drain(c)
	})

	c, _, err := websocket.Dial(context.Background(), "ws://"+ln.Addr().String()+"/ws", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { c.Close(websocket.StatusNormalClosure, "done") })

	tk, err := e.sessions.IssueTicket(context.Background(), acct, id.UUID{}, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}
	wsEnv(t, c, 1, 0, 1, &protocolv1.C2SHello{
		ClientBuild: 1,
		Credential:  &protocolv1.C2SHello_GameplayTicket{GameplayTicket: tk.Credential},
	})
	var ok *protocolv1.S2CHelloOk
	for i := 0; i < 16 && ok == nil; i++ {
		env := wsRead(t, c)
		if env.MessageId == 3 {
			var e2 protocolv1.S2CError
			if err := proto.Unmarshal(env.Payload, &e2); err == nil {
				t.Fatalf("hello rejected: %v", e2.GetErrorCode())
			}
			t.Fatal("hello rejected")
		}
		if env.MessageId == 2 {
			ok = &protocolv1.S2CHelloOk{}
			if err := proto.Unmarshal(env.Payload, ok); err != nil {
				t.Fatalf("unmarshal hello_ok: %v", err)
			}
		}
	}
	if ok == nil {
		t.Fatal("no hello response")
	}
	return c, ok.GetSessionEpoch()
}

// attach binds the character on the open session (ids 6→7).
func attach(t *testing.T, c *websocket.Conn, epoch uint64, charID id.UUID) {
	t.Helper()
	wsEnv(t, c, 6, epoch, 2, &protocolv1.C2SCharacterAttach{CharacterId: charID[:]})
	if env := wsReadUntil(t, c, 7); env.GetMessageId() != 7 {
		t.Fatalf("attach response id %d", env.GetMessageId())
	}
}

func opBytes() []byte {
	o := id.NewV7(time.Now().UTC())
	return o[:]
}

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

// wsReadTimeout bounds a single envelope read. The durable write path
// (queue enqueue → executor tx → commit → mailbox push of 514/515) can
// exceed 10 s under `go test -race` on a loaded CI runner; 30 s keeps a
// true hang detectable while tolerating worst-case durable latency.
//
// It also deflakes the downstream ack family: a read deadline expiring
// mid-commit used to run t.Cleanup while the handler's run() was still
// inside Submit → AwaitClientOutcome → Ack — the cancelled listener ctx
// surfaced as `idempotency: ack: timeout: context already done`. Every
// wire wait in this package funnels through wsRead, so this const is the
// single bound for the whole read+ack timeout family.
const wsReadTimeout = 30 * time.Second

func wsRead(t *testing.T, c *websocket.Conn) *protocolv1.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wsReadTimeout)
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

func decodeMutateResult(t *testing.T, env *protocolv1.Envelope) *protocolv1.S2CProgressionMutateResult {
	t.Helper()
	if env.GetMessageId() != 514 {
		t.Fatalf("message id = %d, want 514", env.GetMessageId())
	}
	var r protocolv1.S2CProgressionMutateResult
	if err := proto.Unmarshal(env.Payload, &r); err != nil {
		t.Fatalf("unmarshal 514: %v", err)
	}
	return &r
}

func decodeState(t *testing.T, env *protocolv1.Envelope) *protocolv1.S2CProgressionState {
	t.Helper()
	if env.GetMessageId() != 515 {
		t.Fatalf("message id = %d, want 515", env.GetMessageId())
	}
	var s protocolv1.S2CProgressionState
	if err := proto.Unmarshal(env.Payload, &s); err != nil {
		t.Fatalf("unmarshal 515: %v", err)
	}
	return &s
}

func decodeError(t *testing.T, env *protocolv1.Envelope) *protocolv1.S2CError {
	t.Helper()
	if env.GetMessageId() != 3 {
		t.Fatalf("message id = %d, want 3", env.GetMessageId())
	}
	var e protocolv1.S2CError
	if err := proto.Unmarshal(env.Payload, &e); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	return &e
}
