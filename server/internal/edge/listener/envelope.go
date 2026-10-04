package listener

import (
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// Phase is the connection-level legality state of protocol.md § Phase
// Legality. Phase is NOT gameplay state: it is derived from the outbound
// S2C messages that carry the session through the lifecycle.
type Phase int

// Phases are exactly the rows of protocol.md § Phase Legality (ADR-0069):
// the gap between ATTACH_OK and the world baseline is still
// CHARACTER_SELECT — there is no attaching phase.
const (
	PhasePreHello Phase = iota
	PhaseCharacterSelect
	PhaseInWorld
	PhaseDead
	PhaseTransfer
	PhasePlacementPending
)

func (p Phase) String() string {
	switch p {
	case PhasePreHello:
		return "PRE_HELLO"
	case PhaseCharacterSelect:
		return "CHARACTER_SELECT"
	case PhaseInWorld:
		return "IN_WORLD"
	case PhaseDead:
		return "DEAD"
	case PhaseTransfer:
		return "TRANSFER"
	case PhasePlacementPending:
		return "PLACEMENT_PENDING"
	}
	return "UNKNOWN"
}

// ConnState is the per-connection state the ordered validation table
// consults. Exported for the packet's validation tests; mutated only by the
// connection loop (single writer) and SetPhase.
type ConnState struct {
	Phase         Phase
	SessionEpoch  uint64
	LastClientSeq uint64
	Attached      bool // character attached (7 delivered, or resumed attach)

	protocolMajor uint32
	now           func() time.Time
	rejects       rejectBudget
	buckets       map[string]*bucket
}

// Reject is the outcome of a frame that fails validation. Exactly the
// fields of protocol.md § Envelope Validation: a reject either closes the
// connection (Close=true, CloseStatus carries the WS status code) or emits
// S2C_ERROR without closing (ErrCode set, counted toward the reject
// budget), or is silently dropped (SilentDrop — no frame, no budget count).
type Reject struct {
	// SilentDrop: frame dropped with no reply and no budget count.
	SilentDrop bool
	// ErrCode + Retryability populate S2C_ERROR; empty when no error frame
	// is sent (protocol-violation closes).
	ErrCode      protocolv1.ErrorCode
	Retryability protocolv1.Retryability
	// Close: terminate the connection after any error frame is sent.
	Close bool
	// CloseStatus is the WS status code used when Close=true.
	CloseStatus int
	// CloseReason is the wire close reason (also the metric label).
	CloseReason string
}

const (
	// wsCloseNormal/Protocol/Policy/TooBig map ErrorCode rejections to WS
	// status codes (implementer mapping; only 4008 SLOW_CONSUMER is
	// spec-pinned in protocol.md § Connection Backpressure).
	wsCloseNormal   = 1000
	wsCloseProtocol = 1002
	wsClosePolicy   = 1008
	wsCloseTooBig   = 1009
	// WSCloseSlowConsumer is the spec-pinned close for § Connection
	// Backpressure: unsent non-replaceable frame or 5 s above 75 %.
	WSCloseSlowConsumer = 4008
)

func (s *ConnState) bucketFor(spec bucketSpec) *bucket {
	if s.buckets == nil {
		s.buckets = make(map[string]*bucket)
	}
	b := s.buckets[spec.key]
	if b == nil {
		b = spec.new()
		s.buckets[spec.key] = b
	}
	return b
}

func (s *ConnState) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Validate applies the ordered validation table of protocol.md § Envelope
// Validation to one parsed envelope. A nil return means: dispatch the frame
// (or handle it internally for HELLO/HEARTBEAT). The rows are evaluated in
// the canonical order (a)-(k); each case documents its row.
func Validate(env *protocolv1.Envelope, st *ConnState) *Reject {
	// Row (a) is covered by the caller: oversize/unparseable frames close
	// before Validate runs (MESSAGE_TOO_LARGE / PROTOCOL_MALFORMED).

	// Row (b): unsupported protocol_major — the only closing version check.
	if env.ProtocolMajor != st.protocolMajor {
		return &Reject{
			ErrCode: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_UNSUPPORTED, Retryability: protocolv1.Retryability_RETRYABILITY_NEVER,
			Close: true, CloseStatus: wsCloseProtocol, CloseReason: "PROTOCOL_UNSUPPORTED",
		}
	}

	ent := lookup(env.MessageId)

	// Row (c): any non-HELLO frame before HELLO_OK closes PROTOCOL_VIOLATION.
	// A HELLO-shaped envelope falls through to the normal path where the
	// handshake validates its seq/epoch fields.
	if st.Phase == PhasePreHello && env.MessageId != 1 {
		return &Reject{Close: true, CloseStatus: wsCloseProtocol, CloseReason: "PROTOCOL_VIOLATION"}
	}

	// Row (d): server_seq != 0 or an S2C-only message_id from the client
	// closes PROTOCOL_VIOLATION.
	if env.ServerSeq != 0 || (ent != nil && !ent.row.c2s) {
		return &Reject{Close: true, CloseStatus: wsCloseProtocol, CloseReason: "PROTOCOL_VIOLATION"}
	}

	// Row (e): session_epoch mismatch once an epoch is assigned — emit
	// S2C_ERROR SESSION_EPOCH_STALE{RECONNECT, close_after} then close.
	if st.SessionEpoch != 0 && env.SessionEpoch != st.SessionEpoch {
		return &Reject{
			ErrCode: protocolv1.ErrorCode_ERROR_CODE_SESSION_EPOCH_STALE, Retryability: protocolv1.Retryability_RETRYABILITY_RECONNECT,
			Close: true, CloseStatus: wsCloseNormal, CloseReason: "SESSION_EPOCH_STALE",
		}
	}

	// Row (f): unregistered message_id — MESSAGE_UNKNOWN, no close,
	// still counts toward the reject budget.
	if ent == nil {
		return &Reject{
			ErrCode: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_UNKNOWN, Retryability: protocolv1.Retryability_RETRYABILITY_NEVER,
			CloseReason: "MESSAGE_UNKNOWN",
		}
	}

	// Row (g): client_seq regression — STALE_INPUT, no close, and the
	// stale frame does not advance LastClientSeq.
	if env.MessageId != 1 && st.LastClientSeq != 0 && env.ClientSeq <= st.LastClientSeq {
		return &Reject{
			ErrCode: protocolv1.ErrorCode_ERROR_CODE_STALE_INPUT, Retryability: protocolv1.Retryability_RETRYABILITY_NEVER,
			CloseReason: "STALE_INPUT",
		}
	}

	// Rows (h)+(i): phase legality. Realtime input in DEAD/TRANSFER/
	// PLACEMENT_PENDING drops silently (still advancing client_seq and
	// counting toward rate buckets); everything else phase-illegal gets
	// MESSAGE_NOT_ALLOWED_IN_STATE without closing.
	// Rows (g) onward consumed this envelope's client_seq position —
	// advance regardless of outcome (silent drop, not-allowed reject,
	// payload failure, rate limit, or dispatch).
	st.LastClientSeq = env.ClientSeq

	verdict := phaseVerdictFor(ent, st)
	if verdict == dropSilently {
		// Still counts toward the frame's rate buckets (protocol.md
		// § Envelope Validation row h): consume a token, drop regardless.
		st.checkRateLimit(ent, st.clock())
		return &Reject{SilentDrop: true, CloseReason: "SILENT_DROP"}
	}
	if verdict == notAllowed {
		return &Reject{
			ErrCode: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE, Retryability: protocolv1.Retryability_RETRYABILITY_NEVER,
			CloseReason: "MESSAGE_NOT_ALLOWED_IN_STATE",
		}
	}

	// Rows (j)/(k) — payload parse and rate limit — are decided by the
	// caller after decode (RejectPayloadParse / RejectRateLimited).
	return nil
}

// RejectPayloadParse is row (j): a registered message_id whose payload does
// not parse — PROTOCOL_MALFORMED, no close.
func RejectPayloadParse() *Reject {
	return &Reject{
		ErrCode: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED, Retryability: protocolv1.Retryability_RETRYABILITY_NEVER,
		CloseReason: "PROTOCOL_MALFORMED",
	}
}

// RejectRateLimited is row (k): a bucket denied the frame — RATE_LIMITED,
// no close, and never counted toward the reject budget.
func RejectRateLimited() *Reject {
	return &Reject{
		ErrCode: protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED, Retryability: protocolv1.Retryability_RETRYABILITY_BACKOFF,
		CloseReason: "RATE_LIMITED",
	}
}

// checkRateLimit evaluates every bucket of the dispatched message at `now`.
func (s *ConnState) checkRateLimit(e *registryEntry, now time.Time) bool {
	for _, spec := range e.buckets {
		if !s.bucketFor(spec).allow(now) {
			return false
		}
	}
	return true
}

// recordReject applies the reject budget (>20 non-closing rejections in
// RejectWindow → PROTOCOL_VIOLATION close).
func (s *ConnState) recordReject(now time.Time, window time.Duration) *Reject {
	if s.rejects.record(now, now.Add(-window)) {
		return &Reject{Close: true, CloseStatus: wsCloseProtocol, CloseReason: "PROTOCOL_VIOLATION"}
	}
	return nil
}
