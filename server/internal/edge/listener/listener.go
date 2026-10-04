// Package listener is the edge listener of IMP-081: WebSocket upgrade,
// envelope framing + the ordered validation table, the C2S_HELLO
// handshake, the bounded outbound queue with REPLACEABLE_STATE supersede
// and S2C_STATE_DELTA merge, slow-consumer close (WS 4008), TLS termination
// modes and the heartbeat plumbing — per docs/05_network/protocol.md,
// docs/05_network/messages.md, docs/07_security/rate_limits.md and
// docs/07_security/external_integrations.md §4.
//
// The edge routes intent only — it never mutates gameplay state
// (service_boundaries.md § Edge).
package listener

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// Config carries the protocol constants of the packet (defaults follow
// protocol.md § Connection Backpressure / § Handshake / § Heartbeat and
// rate_limits.md § Protocol Reject Budget).
type Config struct {
	// Addr is the listen address ("host:port"); PROXY mode requires a
	// loopback bind.
	Addr string
	// TLS selects SERVER (terminate here, CertFile/KeyFile) or PROXY
	// (loopback-only, XFF trusted from loopback peers).
	TLS TerminationMode
	// CertFile/KeyFile are TLS_CERT_FILE/TLS_KEY_FILE, reloaded on SIGHUP
	// in SERVER mode.
	CertFile string
	KeyFile  string
	// TrustXFFLoopback honors X-Forwarded-For from loopback peers
	// (PROXY mode).
	TrustXFFLoopback bool

	// MinProtocolMajor / ProtocolMajor / ProtocolMinor pin the wire
	// version (versioning.md). ProtocolMinor advertises our minor in
	// HELLO_OK when the session leaves it unset.
	MinProtocolMajor uint32
	ProtocolMajor    uint32
	ProtocolMinor    uint32
	// MinBuild / RecommendedBuild / CurrentBuild implement the build gate
	// (versioning.md § Version Gate): client_build < MinBuild is refused
	// CLIENT_UPDATE_REQUIRED.
	MinBuild         uint32
	RecommendedBuild uint32
	CurrentBuild     uint32

	// HelloWindow bounds the PRE_HELLO handshake (10 s).
	HelloWindow time.Duration
	// HeartbeatInterval paces S2C_HEARTBEAT (5 s).
	HeartbeatInterval time.Duration
	// ConnTimeout is the no-valid-traffic limit (15 s).
	ConnTimeout time.Duration
	// ReadLimit is the inbound frame bound (64 KiB).
	ReadLimit int64
	// OutboundFrames / OutboundBytes bound the per-connection send queue
	// (256 frames / 1 MiB).
	OutboundFrames int
	OutboundBytes  int
	// PreAttachInbound bounds the inbound intent queue before attach (8).
	PreAttachInbound int
	// RejectBudget / RejectWindow implement rate_limits.md § Protocol
	// Reject Budget: >20 non-closing rejects in 10 s close the connection.
	RejectBudget int
	RejectWindow time.Duration

	// DrainNotice is the grace period between S2C_SERVER_DRAINING and the
	// connection close during Drain.
	DrainNotice time.Duration
}

// Deps wires the ports the listener drives (service_boundaries.md § Edge).
type Deps struct {
	Metrics    Metrics
	Session    SessionPort
	Intents    IntentSink
	RTT        RTTSink
	Disconnect DisconnectSink
	// Now is the injectable clock (defaults to time.Now).
	Now func() time.Time
	// Logf receives internal diagnostics (defaults to log.Printf).
	Logf func(string, ...any)
}

// Listener is one WSS endpoint.
type Listener struct {
	cfg  Config
	deps Deps

	mux     *http.ServeMux
	srv     *http.Server
	reload  *certReloader
	stopHUP func()

	mu    sync.Mutex
	conns map[*Conn]struct{}
	// draining latched by Drain: new upgrades are refused after it.
	draining bool
}

// New builds the listener. Serve must still be called to bind.
func New(cfg Config, deps Deps) (*Listener, error) {
	if cfg.HelloWindow <= 0 {
		cfg.HelloWindow = 10 * time.Second
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 5 * time.Second
	}
	if cfg.ConnTimeout <= 0 {
		cfg.ConnTimeout = 15 * time.Second
	}
	if cfg.ReadLimit <= 0 {
		cfg.ReadLimit = 64 << 10
	}
	if cfg.OutboundFrames <= 0 {
		cfg.OutboundFrames = 256
	}
	if cfg.OutboundBytes <= 0 {
		cfg.OutboundBytes = 1 << 20
	}
	if cfg.PreAttachInbound <= 0 {
		cfg.PreAttachInbound = 8
	}
	if cfg.RejectBudget <= 0 {
		cfg.RejectBudget = 20
	}
	if cfg.RejectWindow <= 0 {
		cfg.RejectWindow = 10 * time.Second
	}
	if cfg.DrainNotice <= 0 {
		cfg.DrainNotice = 2 * time.Second
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Metrics == nil {
		deps.Metrics = noopMetrics{}
	}
	if deps.Logf == nil {
		deps.Logf = log.Printf
	}
	l := &Listener{
		cfg:   cfg,
		deps:  deps,
		mux:   http.NewServeMux(),
		conns: make(map[*Conn]struct{}),
	}
	l.mux.HandleFunc("/ws", l.handleWS)
	return l, nil
}

func (l *Listener) logf(format string, args ...any) {
	if l.deps.Logf != nil {
		l.deps.Logf(format, args...)
	}
}

// Serve binds the address and serves until ctx is done or Drain is called.
// In SERVER mode it terminates TLS (CertFile/KeyFile, SIGHUP reload); in
// PROXY mode Addr must be loopback.
func (l *Listener) Serve(ctx context.Context) error {
	l.srv = &http.Server{
		Addr:    l.cfg.Addr,
		Handler: l.mux,
	}
	if l.cfg.TLS == TerminationProxy {
		host, _, err := net.SplitHostPort(l.cfg.Addr)
		if err != nil {
			return fmt.Errorf("edge/listener: proxy addr %q: %w", l.cfg.Addr, err)
		}
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("edge/listener: TLS_TERMINATION=PROXY requires a loopback bind, got %q", l.cfg.Addr)
		}
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = l.srv.Shutdown(shutdownCtx)
	}()
	var err error
	if l.cfg.TLS == TerminationServer {
		r, rerr := newCertReloader(l.cfg.CertFile, l.cfg.KeyFile)
		if rerr != nil {
			return rerr
		}
		l.reload = r
		l.stopHUP = r.watchSIGHUP(l.deps.Logf)
		defer l.stopHUP()
		l.srv.TLSConfig = tlsConfig(r)
		err = l.srv.ListenAndServeTLS("", "")
	} else {
		err = l.srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// ServeListener serves the WS handler on an already-bound net.Listener —
// used by tests and embedders that manage their own accept loop.
func (l *Listener) ServeListener(ctx context.Context, ln net.Listener) error {
	l.srv = &http.Server{Handler: l.mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = l.srv.Shutdown(shutdownCtx)
	}()
	err := l.srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// AddrOf returns the bound address once Serve is running — helper for
// tests using ":0".
func (l *Listener) AddrOf() string {
	if l.srv == nil {
		return ""
	}
	return l.srv.Addr
}

// handleWS upgrades one /ws request. When draining, new upgrades are
// refused so the LB can drain the node (protocol.md § Connection
// Lifecycle).
func (l *Listener) handleWS(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	if l.draining {
		l.mu.Unlock()
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	l.mu.Unlock()

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled, // spec-pinned
	})
	if err != nil {
		l.deps.Logf("listener: accept failed: %v", err)
		return
	}
	meta := HelloMeta{
		RemoteAddr: addr(r.RemoteAddr),
		ClientIP:   clientIP(r, l.cfg.TrustXFFLoopback || l.cfg.TLS == TerminationProxy),
	}
	c := newConn(l, ws, meta)
	// Library-level read bound as a hard backstop (frames well past the
	// 64 KiB protocol limit): the protocol MESSAGE_TOO_LARGE close comes
	// from our own bounded read in readLoop.
	ws.SetReadLimit(4 * l.cfg.ReadLimit)
	l.mu.Lock()
	l.conns[c] = struct{}{}
	l.mu.Unlock()
	l.deps.Metrics.ConnOpened()
	// The connection's lifecycle is owned by run; the handler must not
	// return until teardown (the WS is hijacked anyway).
	c.run(r.Context())
}

func addr(s string) net.Addr {
	a, err := net.ResolveTCPAddr("tcp", s)
	if err != nil {
		return nil
	}
	return a
}

func (l *Listener) forget(c *Conn) {
	l.mu.Lock()
	delete(l.conns, c)
	l.mu.Unlock()
}

// Drain stops accepting upgrades, announces S2C_SERVER_DRAINING (id 9) on
// every live connection, waits DrainNotice, then closes them (protocol.md
// § Connection Lifecycle). The HTTP server is shut down after conns close.
func (l *Listener) Drain(ctx context.Context) error {
	l.mu.Lock()
	l.draining = true
	l.mu.Unlock()

	if l.srv != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = l.srv.Shutdown(shutdownCtx) // stops listeners; hijacked conns unaffected
		cancel()
	}

	l.mu.Lock()
	conns := make([]*Conn, 0, len(l.conns))
	for c := range l.conns {
		conns = append(conns, c)
	}
	l.mu.Unlock()

	deadline := l.deps.Now().Add(l.cfg.DrainNotice)
	for _, c := range conns {
		_ = c.sendS2C(drainingMsg(deadline), 9, DeliveryControl)
	}
	if d := time.Until(deadline); d > 0 {
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
		case <-t.C:
		}
		t.Stop()
	}
	for _, c := range conns {
		c.closeWith(int(websocket.StatusGoingAway), "SERVER_DRAINING")
	}
	return nil
}

// drainingMsg builds S2C_SERVER_DRAINING{reason=MAINTENANCE,
// drain_deadline_ms=abs, reconnect_after_ms} (protocol.md § Lifecycle).
func drainingMsg(deadline time.Time) *protocolv1.S2CServerDraining {
	return &protocolv1.S2CServerDraining{
		Reason:           protocolv1.ServerDrainingReason_SERVER_DRAINING_REASON_MAINTENANCE,
		DrainDeadlineMs:  deadline.UnixMilli(),
		ReconnectAfterMs: 3000,
	}
}
