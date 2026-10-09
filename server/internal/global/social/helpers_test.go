package social

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
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	sociald "thinhthan/internal/durable/social"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	"thinhthan/internal/edge/session"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

// fakeHub records every Deliver and answers Presence/Fanout from the
// captured server conns plus test-configured zones and rosters.
type fakeHub struct {
	mu        sync.Mutex
	conns     map[id.UUID][]*listener.Conn
	zones     map[id.UUID]string
	roster    map[string][]id.UUID // "PARTY"|"GUILD" → members
	delivered []delivered
}

type delivered struct {
	to    id.UUID
	msgID uint32
	msg   proto.Message
}

func newFakeHub() *fakeHub {
	return &fakeHub{conns: map[id.UUID][]*listener.Conn{},
		zones: map[id.UUID]string{}, roster: map[string][]id.UUID{}}
}

func (h *fakeHub) bind(charID id.UUID, c *listener.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[charID] = append(h.conns[charID], c)
}

func (h *fakeHub) Deliver(ctx context.Context, charID id.UUID, msgID uint32,
	m proto.Message) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.delivered = append(h.delivered, delivered{charID, msgID, m})
	for _, c := range h.conns[charID] {
		payload, err := proto.Marshal(m)
		if err != nil {
			return err
		}
		if err := c.Send(&protocolv1.Envelope{MessageId: msgID, Payload: payload},
			listener.DeliveryControl); err != nil {
			return err
		}
	}
	return nil
}

func (h *fakeHub) Presence(charID id.UUID) Presence {
	h.mu.Lock()
	defer h.mu.Unlock()
	on := len(h.conns[charID]) > 0
	return Presence{Online: on, ZoneID: h.zones[charID]}
}

func (h *fakeHub) Fanout(ctx context.Context, channel protocolv1.ChatChannel,
	senderID, targetID id.UUID) ([]id.UUID, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch channel {
	case protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER:
		return []id.UUID{senderID, targetID}, nil
	case protocolv1.ChatChannel_CHAT_CHANNEL_WORLD:
		var out []id.UUID
		for c := range h.conns {
			out = append(out, c)
		}
		return out, nil
	case protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL:
		zone := h.zones[senderID]
		var out []id.UUID
		for c, z := range h.zones {
			if z == zone && len(h.conns[c]) > 0 {
				out = append(out, c)
			}
		}
		return out, nil
	case protocolv1.ChatChannel_CHAT_CHANNEL_PARTY,
		protocolv1.ChatChannel_CHAT_CHANNEL_GUILD:
		key := "PARTY"
		if channel == protocolv1.ChatChannel_CHAT_CHANNEL_GUILD {
			key = "GUILD"
		}
		for _, m := range h.roster[key] {
			if m == senderID {
				return h.roster[key], nil
			}
		}
		return nil, ErrNotMember
	}
	return nil, ErrNotMember
}

func (h *fakeHub) seen(charID id.UUID, msgID uint32) []proto.Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []proto.Message
	for _, d := range h.delivered {
		if d.to == charID && d.msgID == msgID {
			out = append(out, d.msg)
		}
	}
	return out
}

// waitSeen polls the recorded deliveries until at least n frames of
// msgID reached charID — the S2C result lands on the wire before the
// post-commit pushes finish.
func (h *fakeHub) waitSeen(t *testing.T, charID id.UUID, msgID uint32,
	n int) []proto.Message {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := h.seen(charID, msgID)
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("waitSeen %d to %s: got %d", msgID, charID, len(got))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// connTap wraps the session intent sink: on C2S_CHARACTER_ATTACH (6) it
// records the server-side conn under the attached character (for hub
// Deliver) and flips the conn phase to IN_WORLD before the session
// dispatches — matching production's offer-time ordering.
type connTap struct {
	inner listener.IntentSink
	hub   *fakeHub
}

func (t *connTap) Enqueue(ctx context.Context, c *listener.Conn, f listener.Inbound) error {
	if f.MessageID == 6 {
		if m, ok := f.Payload.(*protocolv1.C2SCharacterAttach); ok && len(m.GetCharacterId()) == 16 {
			var charID id.UUID
			copy(charID[:], m.GetCharacterId())
			t.hub.bind(charID, c)
		}
		c.SetPhase(listener.PhaseInWorld)
	}
	return t.inner.Enqueue(ctx, c, f)
}

type testTimer struct{ *time.Timer }

func (t testTimer) Stop() bool { return t.Timer.Stop() }

// env wires the full stack for one test: migrated pool, durable queue,
// ProducerClient mux over sociald.Executors, session registry, router
// with the social handlers, and the fake hub.
type env struct {
	sessions *session.Registry
	accounts *account.Store
	router   *router.Registry
	q        *queue.Queue
	store    *sociald.Store
	svc      *Service
	hub      *fakeHub
	pool     *pgxpool.Pool
}

func newEnv(t *testing.T) *env {
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
	sstore := sociald.New(p)
	hub := newFakeHub()
	q := queue.New(64, idempotency.NewStore(p),
		queue.Deps{Pool: p, Now: func() time.Time { return time.Now().UTC() }})
	q.RegisterExecutor(queue.ProducerActivity, accounts.ActivityExecutor)
	execs := sociald.Executors(sstore)
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
	svc := New(q, sstore, hub)
	if err := svc.Register(rt); err != nil {
		t.Fatalf("register: %v", err)
	}
	sessions := session.New(session.Config{
		Capacity:        64,
		ContentRevision: strings.Repeat("a", 64),
		Now:             func() time.Time { return time.Now().UTC() },
		After: func(d time.Duration, f func()) session.Timer {
			return testTimer{time.AfterFunc(d, f)}
		},
	}, accounts, q)
	sessions.SetRouter(rt)
	return &env{sessions: sessions, accounts: accounts, router: rt,
		q: q, store: sstore, svc: svc, hub: hub, pool: p}
}

func (e *env) seedAccount(t *testing.T) id.UUID {
	t.Helper()
	acct := id.NewV7(time.Now().UTC())
	if err := e.accounts.CreateAccount(context.Background(), nil, acct, time.Now().UTC()); err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acct
}

func (e *env) seedCharacter(t *testing.T, acct id.UUID, level int32) id.UUID {
	t.Helper()
	charID := id.NewV7(time.Now().UTC())
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO characters
		 (character_id, account_id, name, name_key, class_id, level)
		 VALUES ($1,$2,$3,$4,'class.kim',$5)`,
		charID.String(), acct.String(), "Soc "+charID.String()[:8], charID.String(),
		level); err != nil {
		t.Fatalf("seed character: %v", err)
	}
	return charID
}

// setActive marks the character online via its character_activity row.
func (e *env) setActive(t *testing.T, charID id.UUID, on bool) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO character_activity (character_id, session_active)
		 VALUES ($1,$2)
		 ON CONFLICT (character_id) DO UPDATE SET session_active = $2`,
		charID.String(), on); err != nil {
		t.Fatalf("setActive: %v", err)
	}
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
		Intents:    &connTap{inner: e.sessions, hub: e.hub},
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
	for i := 0; i < 32; i++ {
		env := wsRead(t, c)
		if env.MessageId == msgID {
			return env
		}
	}
	t.Fatalf("never saw message id %d", msgID)
	return nil
}

// decode decodes an envelope payload into m.
func decode(t *testing.T, env *protocolv1.Envelope, msgID uint32, m proto.Message) {
	t.Helper()
	if env.GetMessageId() != msgID {
		t.Fatalf("message id = %d, want %d", env.GetMessageId(), msgID)
	}
	if err := proto.Unmarshal(env.Payload, m); err != nil {
		t.Fatalf("unmarshal %d: %v", msgID, err)
	}
}
