package listener

import (
	"context"

	"google.golang.org/protobuf/proto"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// handleHello runs the handshake of protocol.md § Handshake (ADR-0064):
// exactly one C2S_HELLO carrying client_seq=1, session_epoch=0,
// server_seq=0 within the 10 s window; a build below minimum_supported_
// build is refused CLIENT_UPDATE_REQUIRED; SessionPort.Hello decides the
// rest (bad credential, replace-login, resume).
func (c *Conn) handleHello(ctx context.Context, env *protocolv1.Envelope) bool {
	// Handshake-scoped field checks (stricter than the generic table):
	// HELLO is always seq 1, epoch 0, server_seq 0.
	if env.ClientSeq != 1 || env.SessionEpoch != 0 {
		c.closeWith(wsCloseProtocol, "PROTOCOL_VIOLATION")
		return false
	}
	var hello protocolv1.C2SHello
	if err := proto.Unmarshal(env.Payload, &hello); err != nil {
		c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED, protocolv1.Retryability_RETRYABILITY_NEVER, wsCloseProtocol, "PROTOCOL_MALFORMED")
		return false
	}
	// Build gate (versioning.md): client_build below the server minimum
	// gets CLIENT_UPDATE_REQUIRED, never a session.
	if c.l.cfg.MinBuild != 0 && hello.ClientBuild < c.l.cfg.MinBuild {
		c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_CLIENT_UPDATE_REQUIRED, protocolv1.Retryability_RETRYABILITY_NEVER, wsCloseNormal, "CLIENT_UPDATE_REQUIRED")
		return false
	}
	if c.l.deps.Session == nil {
		// No session owner wired — fail closed, do not leak the socket.
		c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_AUTH_REQUIRED, protocolv1.Retryability_RETRYABILITY_NEVER, wsClosePolicy, "AUTH_REQUIRED")
		return false
	}
	decision := c.l.deps.Session.Hello(ctx, c.meta, &hello)
	if decision.Reject != nil {
		r := decision.Reject
		c.sendErrorNow(r.Code, r.Retryability, env.ClientSeq)
		c.closeWith(wsClosePolicy, r.Code.String())
		return false
	}
	if decision.OK == nil {
		// Fail closed: an empty decision is an AUTH_REQUIRED rejection.
		c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_AUTH_REQUIRED, protocolv1.Retryability_RETRYABILITY_NEVER, wsClosePolicy, "AUTH_REQUIRED")
		return false
	}
	ok := decision.OK
	if ok.SessionEpoch == 0 {
		// A zero epoch would leave row (e) disarmed — fail closed.
		c.sendAndClose(protocolv1.ErrorCode_ERROR_CODE_AUTH_REQUIRED, protocolv1.Retryability_RETRYABILITY_NEVER, wsClosePolicy, "AUTH_REQUIRED")
		return false
	}
	c.mu.Lock()
	c.st.SessionEpoch = ok.SessionEpoch
	c.st.LastClientSeq = 1
	c.st.Phase = PhaseCharacterSelect
	c.st.Attached = len(ok.ResumedCharacterId) > 0 // resumed attach
	c.mu.Unlock()
	if len(ok.ResumedCharacterId) > 0 {
		c.attachOK.Store(true)
	}
	if ok.HeartbeatIntervalMs == 0 {
		ok.HeartbeatIntervalMs = uint32(c.l.cfg.HeartbeatInterval.Milliseconds())
	}
	if ok.ConnectionTimeoutMs == 0 {
		ok.ConnectionTimeoutMs = uint32(c.l.cfg.ConnTimeout.Milliseconds())
	}
	if ok.ProtocolMinor == 0 {
		ok.ProtocolMinor = c.l.cfg.ProtocolMinor
	}
	if err := c.sendS2C(ok, 2, DeliveryControl); err != nil {
		c.l.logf("listener: HELLO_OK send failed: %v", err)
		return false
	}
	return true
}
