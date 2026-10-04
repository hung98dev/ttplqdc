package listener

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// fakeSession accepts every valid HELLO and assigns a fixed epoch.
type fakeSession struct {
	ok   *protocolv1.S2CHelloOk
	rej  *ErrorReject
	seen *protocolv1.C2SHello
}

func (f *fakeSession) Hello(_ context.Context, _ HelloMeta, h *protocolv1.C2SHello) HelloDecision {
	f.seen = h
	if f.rej != nil {
		return HelloDecision{Reject: f.rej}
	}
	return HelloDecision{OK: f.ok}
}

type fakeSink struct {
	mu     sync.Mutex
	frames []Inbound
}

func (f *fakeSink) Enqueue(_ context.Context, _ *Conn, in Inbound) error {
	f.mu.Lock()
	f.frames = append(f.frames, in)
	f.mu.Unlock()
	return nil
}

// startListener boots a listener on a loopback :0 socket and returns the
// ws URL, the listener, and a stop function.
func startListener(t *testing.T, cfg Config, deps Deps) (string, *Listener, func()) {
	t.Helper()
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:0"
	}
	l, err := New(cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = l.ServeListener(ctx, ln) }()
	return fmt.Sprintf("ws://%s/ws", ln.Addr().String()), l, func() {
		cancel()
		_ = ln.Close()
	}
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func sendEnv(t *testing.T, c *websocket.Conn, env *protocolv1.Envelope) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dst []byte
	dst = EncodeEnvelope(dst, env, env.Payload)
	if err := c.Write(ctx, websocket.MessageBinary, dst); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readEnv(t *testing.T, c *websocket.Conn) *protocolv1.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	typ, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("want binary frame, got %v", typ)
	}
	var env protocolv1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	return &env
}

func helloEnv(build uint32) *protocolv1.Envelope {
	h := &protocolv1.C2SHello{
		Credential:  &protocolv1.C2SHello_GameplayTicket{GameplayTicket: "ticket"},
		ClientBuild: build,
	}
	p, _ := proto.Marshal(h)
	return &protocolv1.Envelope{
		ProtocolMajor: 1,
		MessageId:     1,
		ClientSeq:     1,
		Payload:       p,
	}
}

func defaultDeps() (Deps, *fakeSession, *fakeSink) {
	fs := &fakeSession{ok: &protocolv1.S2CHelloOk{
		SessionId:    []byte("0123456789abcdef"),
		SessionEpoch: 77,
		AccountId:    []byte("fedcba9876543210"),
	}}
	sink := &fakeSink{}
	return Deps{Session: fs, Intents: sink}, fs, sink
}

func TestVersionHandshake(t *testing.T) {
	deps, fs, _ := defaultDeps()
	url, _, stop := startListener(t, Config{
		ProtocolMajor: 1, ProtocolMinor: 3,
		MinBuild: 100, CurrentBuild: 120,
	}, deps)
	defer stop()

	// Good handshake: HELLO_OK comes back with the session's epoch.
	c := dial(t, url)
	sendEnv(t, c, helloEnv(110))
	got := readEnv(t, c)
	if got.MessageId != 2 {
		t.Fatalf("want HELLO_OK (2), got %d", got.MessageId)
	}
	var ok protocolv1.S2CHelloOk
	if err := proto.Unmarshal(got.Payload, &ok); err != nil {
		t.Fatal(err)
	}
	if ok.SessionEpoch != 77 || ok.ProtocolMinor != 3 {
		t.Fatalf("hello_ok: %s", ok.String())
	}
	if fs.seen == nil || fs.seen.ClientBuild != 110 {
		t.Fatalf("session port did not see the hello: %+v", fs.seen)
	}
	_ = c.Close(websocket.StatusNormalClosure, "bye")

	// Build below the minimum: CLIENT_UPDATE_REQUIRED.
	c2 := dial(t, url)
	sendEnv(t, c2, helloEnv(50))
	e2 := readEnv(t, c2)
	if e2.MessageId != 3 {
		t.Fatalf("want S2C_ERROR (3), got %d", e2.MessageId)
	}
	var serr protocolv1.S2CError
	if err := proto.Unmarshal(e2.Payload, &serr); err != nil {
		t.Fatal(err)
	}
	if serr.ErrorCode != protocolv1.ErrorCode_ERROR_CODE_CLIENT_UPDATE_REQUIRED || !serr.CloseAfter {
		t.Fatalf("want CLIENT_UPDATE_REQUIRED close_after, got %s", serr.String())
	}
	// And the connection is closed by the server.
	if _, _, err := c2.Read(context.Background()); err == nil {
		t.Fatalf("expected server close")
	}
}

func TestFrameSizeLimit(t *testing.T) {
	deps, _, _ := defaultDeps()
	url, _, stop := startListener(t, Config{ProtocolMajor: 1}, deps)
	defer stop()

	c := dial(t, url)
	big := make([]byte, 70<<10)
	// Stuff the size into an envelope-looking frame that exceeds 64 KiB.
	env := &protocolv1.Envelope{ProtocolMajor: 1, MessageId: 9999, ClientSeq: 1, Payload: big}
	sendEnv(t, c, env)
	got := readEnv(t, c)
	if got.MessageId != 3 {
		t.Fatalf("want S2C_ERROR (3), got %d", got.MessageId)
	}
	var serr protocolv1.S2CError
	if err := proto.Unmarshal(got.Payload, &serr); err != nil {
		t.Fatal(err)
	}
	if serr.ErrorCode != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_TOO_LARGE {
		t.Fatalf("want MESSAGE_TOO_LARGE, got %v", serr.ErrorCode)
	}
	if _, _, err := c.Read(context.Background()); err == nil {
		t.Fatalf("expected server close after too-large frame")
	}
}

func TestUnknownMessageRejected(t *testing.T) {
	deps, _, _ := defaultDeps()
	url, _, stop := startListener(t, Config{ProtocolMajor: 1}, deps)
	defer stop()

	c := dial(t, url)
	sendEnv(t, c, helloEnv(110))
	if got := readEnv(t, c); got.MessageId != 2 {
		t.Fatalf("handshake failed: %d", got.MessageId)
	}
	// Unregistered id → MESSAGE_UNKNOWN without close.
	sendEnv(t, c, &protocolv1.Envelope{ProtocolMajor: 1, MessageId: 9999, SessionEpoch: 77, ClientSeq: 2})
	got := readEnv(t, c)
	if got.MessageId != 3 {
		t.Fatalf("want S2C_ERROR (3), got %d", got.MessageId)
	}
	var serr protocolv1.S2CError
	if err := proto.Unmarshal(got.Payload, &serr); err != nil {
		t.Fatal(err)
	}
	if serr.ErrorCode != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_UNKNOWN || serr.CloseAfter {
		t.Fatalf("want MESSAGE_UNKNOWN no-close, got %s", serr.String())
	}
	// Connection stays usable.
	sendEnv(t, c, &protocolv1.Envelope{ProtocolMajor: 1, MessageId: 9999, SessionEpoch: 77, ClientSeq: 3})
	if got2 := readEnv(t, c); got2.MessageId != 3 {
		t.Fatalf("second reject: got %d", got2.MessageId)
	}
}

func TestDrainClosesAccept(t *testing.T) {
	deps, _, _ := defaultDeps()
	url, l, stop := startListener(t, Config{ProtocolMajor: 1, DrainNotice: 50 * time.Millisecond}, deps)
	defer stop()

	c := dial(t, url)
	sendEnv(t, c, helloEnv(110))
	if got := readEnv(t, c); got.MessageId != 2 {
		t.Fatalf("handshake failed: %d", got.MessageId)
	}
	if err := l.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Draining sends S2C_SERVER_DRAINING then closes live conns — read
	// until the close frame (the drain notice arrives first).
	deadline := time.Now().Add(3 * time.Second)
	var closed bool
	for time.Now().Before(deadline) && !closed {
		rctx, rcancel := context.WithDeadline(context.Background(), deadline)
		_, _, err := c.Read(rctx)
		rcancel()
		if err != nil {
			closed = true // close frame observed
		}
	}
	if !closed {
		t.Fatalf("live conn must be closed by drain")
	}
	// New upgrades are refused.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := websocket.Dial(ctx, url, nil); err == nil {
		t.Fatalf("draining listener must refuse new upgrades")
	}
}
