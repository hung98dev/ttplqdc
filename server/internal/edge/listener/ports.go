package listener

import (
	"context"
	"net"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/edge/heartbeat"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Inbound is one dispatched C2S frame handed to the intent sink: decoded
// payload plus the envelope fields the consumer needs for ordering and
// correlation (docs/05_network/protocol.md § Envelope).
type Inbound struct {
	MessageID     uint32
	SessionEpoch  uint64
	ClientSeq     uint64
	CorrelationID uint64
	Payload       proto.Message
}

// HelloMeta carries the connection metadata SessionPort.Hello needs: the
// peer's addresses, with ClientIP resolved through X-Forwarded-For only
// when the peer is a loopback proxy (protocol.md § TLS termination).
type HelloMeta struct {
	RemoteAddr net.Addr
	ClientIP   net.IP
}

// HelloDecision is the SessionPort's verdict for one C2S_HELLO: exactly one
// of OK / Reject.
type HelloDecision struct {
	// OK carries the fully populated S2C_HELLO_OK payload (session_id,
	// epoch, credentials, cadence) — the listener assigns envelope
	// session_epoch from OK.SessionEpoch and learns resume state from
	// OK.ResumedCharacterId.
	OK *protocolv1.S2CHelloOk
	// Reject emits S2C_ERROR{error_code, retryability, close_after=true}
	// then closes the connection.
	Reject *ErrorReject
}

// ErrorReject is a protocol-level rejection carried on S2C_ERROR.
type ErrorReject struct {
	Code           protocolv1.ErrorCode
	Retryability   protocolv1.Retryability
	RetryAfterMs   uint32
	SafeMessageKey string
}

// SessionPort is the edge→session boundary
// (docs/04_architecture/service_boundaries.md § Edge): the listener owns the
// connection; the session owner authenticates HELLOs.
type SessionPort interface {
	Hello(ctx context.Context, meta HelloMeta, hello *protocolv1.C2SHello) HelloDecision
}

// IntentSink receives every dispatched C2S frame after validation. The edge
// never interprets payload semantics — it enqueues typed intents only
// (service_boundaries.md § Edge).
type IntentSink interface {
	Enqueue(ctx context.Context, c *Conn, f Inbound) error
}

// RTTSink receives per-session RTT samples (protocol.md § Heartbeat: server
// receive time minus echo_server_ms, only when echo != 0).
type RTTSink interface {
	Publish(c *Conn, s heartbeat.Sample)
}

// DisconnectSink learns of connection closes (normal or error) so the
// session owner can transition the session lifecycle.
type DisconnectSink interface {
	Disconnected(c *Conn, code int, reason string)
}

// Metrics is the narrow observability surface the listener emits into
// observability/core at the composition root (service_boundaries.md § Edge —
// connection lifecycle, reject-budget, queue-depth signals). A nil Metrics
// is valid; all hooks are no-op-safe.
type Metrics interface {
	ConnOpened()
	ConnClosed(code int, reason string)
	Reject(code string)
	QueueDepth(frames int, bytes int)
}

// noopMetrics backs a nil Deps.Metrics.
type noopMetrics struct{}

func (noopMetrics) ConnOpened()            {}
func (noopMetrics) ConnClosed(int, string) {}
func (noopMetrics) Reject(string)          {}
func (noopMetrics) QueueDepth(int, int)    {}
