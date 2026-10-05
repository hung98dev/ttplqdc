package character

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
	"thinhthan/internal/durable/character"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	"thinhthan/internal/edge/session"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

type testTimer struct{ *time.Timer }

func (t testTimer) Stop() bool { return t.Timer.Stop() }

// env wires the full ADR-0081 stack for one test: migrated pool,
// durable queue (activity + client executors), router with the id-12
// handler, session registry. Returns the pieces tests act on.
type env struct {
	sessions *session.Registry
	accounts *account.Store
	router   *router.Registry
	q        *queue.Queue
}

func newEnv(t *testing.T, capacity int) *env {
	t.Helper()
	ctx := context.Background()
	dsn := pgtest.FreshDB(t)
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir("../../../.."), "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	accounts := account.NewStore(pool)
	chars := character.NewStore(pool)
	q := queue.New(capacity, idempotency.NewStore(pool),
		queue.Deps{Pool: pool, Now: func() time.Time { return time.Now().UTC() }})
	q.RegisterExecutor(queue.ProducerActivity, accounts.ActivityExecutor)
	q.RegisterExecutor(queue.ProducerClient, chars.CreateExecutor)
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = q.Shutdown(c)
	})

	rt := router.New()
	if err := New(q, accounts).Register(rt); err != nil {
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
	return &env{sessions: sessions, accounts: accounts, router: rt, q: q}
}

// seedAccount inserts one ACTIVE account row.
func (e *env) seedAccount(t *testing.T) id.UUID {
	t.Helper()
	ctx := context.Background()
	acct := id.NewV7(time.Now().UTC())
	if err := e.accounts.CreateAccount(ctx, nil, acct, time.Now().UTC()); err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acct
}

// suspendAccount moves an account into SUSPENDED_PAYMENT_RECONCILIATION.
func (e *env) suspendAccount(t *testing.T, acct id.UUID) {
	t.Helper()
	if _, err := e.accounts.Pool().Exec(context.Background(),
		`UPDATE accounts SET status='SUSPENDED_PAYMENT_RECONCILIATION' WHERE account_id=$1`,
		acct); err != nil {
		t.Fatalf("suspend: %v", err)
	}
}

// seedCharacter inserts one live character owned by the account.
func (e *env) seedCharacter(t *testing.T, acct id.UUID, name string) id.UUID {
	t.Helper()
	ctx := context.Background()
	charID := id.NewV7(time.Now().UTC())
	if _, err := e.accounts.Pool().Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, map_id)
		 VALUES ($1,$2,$3,$4,'class.kim',1,'map.lang_da.dinh_lang')`,
		charID, acct, name, name); err != nil {
		t.Fatalf("create character: %v", err)
	}
	return charID
}

// dial opens a WS connection and completes the gameplay HELLO for the
// account; returns the conn + negotiated session epoch.
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
		Intents:    e.sessions,
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
			t.Fatalf("hello rejected")
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

// opIDBytes is a fresh UUIDv7 operation id as bytes.
func opIDBytes() []byte {
	o := id.NewV7(time.Now().UTC())
	return o[:]
}

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

// wsRead reads one envelope (binary frames only).
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

// wsReadUntil reads envelopes until one carries msgID.
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

// decodeCreateResult unmarshals an id-13 envelope payload.
func decodeCreateResult(t *testing.T, env *protocolv1.Envelope) *protocolv1.S2CCharacterCreateResult {
	t.Helper()
	if env.GetMessageId() != 13 {
		t.Fatalf("message id = %d, want 13", env.GetMessageId())
	}
	var r protocolv1.S2CCharacterCreateResult
	if err := proto.Unmarshal(env.Payload, &r); err != nil {
		t.Fatalf("unmarshal create result: %v", err)
	}
	return &r
}

// decodeError unmarshals an id-3 envelope payload.
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

// decodeList unmarshals an id-14 envelope payload.
func decodeList(t *testing.T, env *protocolv1.Envelope) *protocolv1.S2CCharacterList {
	t.Helper()
	if env.GetMessageId() != 14 {
		t.Fatalf("message id = %d, want 14", env.GetMessageId())
	}
	var l protocolv1.S2CCharacterList
	if err := proto.Unmarshal(env.Payload, &l); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	return &l
}
