package session

import (
	"context"
	"errors"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// DurableIntent is the closed set of C2S ids that admit durable-intent
// records through the router (protobuf_conventions.md §7 client
// expansion); router.go holds the same table — kept in one place there.
type DurableRouter interface {
	// Handles reports whether message id is in the durable-intent set.
	Handles(id uint32) bool
	// Dispatch routes the inbound intent to its registered handler.
	Dispatch(ctx context.Context, c *listener.Conn, in listener.Inbound) error
}

// SetRouter wires the durable-intent router (late binding: cmd/server
// builds registry → router → adapter chain).
func (r *Registry) SetRouter(d DurableRouter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.router = d
}

// Enqueue implements listener.IntentSink: binds conn↔session on the first
// inbound (session_epoch identifies it), emits S2C_CHARACTER_LIST when no
// character was resumed, then routes attach/detach/durable intents.
func (r *Registry) Enqueue(ctx context.Context, c *listener.Conn, f listener.Inbound) error {
	r.mu.Lock()
	s, ok := r.sessions[f.SessionEpoch]
	if !ok {
		r.mu.Unlock()
		return nil // superseded/expired session: drop
	}
	bind := false
	if s.conn == nil {
		s.conn = c
		r.conns[c] = s
		bind = true
	}
	r.mu.Unlock()

	if bind && s.charID == nil {
		// CHARACTER_LIST follows HELLO_OK for sessions without a resumed
		// character (messages.md id 14).
		if err := r.sendCharacterList(ctx, s); err != nil {
			return err
		}
	}

	var err error
	switch f.MessageID {
	case 6: // C2S_CHARACTER_ATTACH
		m, ok := f.Payload.(*protocolv1.C2SCharacterAttach)
		if !ok {
			return nil // listener already parsed the typed payload
		}
		err = r.attach(ctx, s, m)
	case 10: // C2S_CHARACTER_DETACH
		err = r.detach(ctx, s, &protocolv1.C2SCharacterDetach{})
	default:
		if r.router != nil && r.router.Handles(f.MessageID) {
			err = r.router.Dispatch(ctx, c, f)
		}
		// Non-durable ids without a registered handler are consumed
		// silently — no owning feature has landed yet.
	}
	if err != nil {
		var pe *protoError
		var re *router.RejectError
		switch {
		case errors.As(err, &pe):
			if sendErr := r.send(c, 3, &protocolv1.S2CError{
				ErrorCode:    pe.code,
				Retryability: retryOf(pe.code),
			}); sendErr != nil {
				return sendErr
			}
		case errors.As(err, &re):
			if sendErr := r.send(c, 3, &protocolv1.S2CError{
				ErrorCode:    re.Code,
				Retryability: retryOf(re.Code),
			}); sendErr != nil {
				return sendErr
			}
		}
	}
	return err
}

// Disconnected implements listener.DisconnectSink: a session holding a
// live character enters the 30 s reconnect grace (slot state
// RECONNECT_GRACE); a session without one releases its slot at once.
func (r *Registry) Disconnected(c *listener.Conn, code int, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.conns[c]
	if !ok {
		return
	}
	delete(r.conns, c)
	// Only the currently bound conn matters: a superseded conn's close must
	// not grace/detach the live session (sess.conn was already nilled).
	if s.conn != c {
		return
	}
	s.conn = nil
	if s.charID == nil {
		r.releaseLocked(s)
		return
	}
	r.queue.setState(s.accountID, slotReconnectGrace)
	grace := r.cfg.Grace
	s.graceTimer = r.cfg.After(grace, func() { r.graceExpired(s) })
}

// graceExpired detaches the character and frees the slot after the
// reconnect window lapses.
func (r *Registry) graceExpired(s *sess) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r.mu.Lock()
	if s.graceTimer == nil || s.conn != nil {
		r.mu.Unlock()
		return
	}
	s.graceTimer = nil
	var charID *id.UUID
	if s.charID != nil {
		c := *s.charID
		charID = &c
		delete(r.liveChar, *s.charID)
		s.charID = nil
	}
	r.releaseLocked(s)
	r.mu.Unlock()
	if charID != nil {
		_ = account.SubmitActivity(ctx, r.q, *charID, s.epoch,
			account.ActivityDetached, r.cfg.Now())
	}
}

// retryOf maps wire error codes to a retryability class (errors.md).
func retryOf(code protocolv1.ErrorCode) protocolv1.Retryability {
	switch code {
	case protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_PENDING_DELETION,
		protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_BANNED,
		protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_SUSPENDED,
		protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER,
		protocolv1.ErrorCode_ERROR_CODE_CHARACTER_ALREADY_ACTIVE:
		return protocolv1.Retryability_RETRYABILITY_NEVER
	case protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED,
		protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED:
		return protocolv1.Retryability_RETRYABILITY_BACKOFF
	default:
		return protocolv1.Retryability_RETRYABILITY_NEVER
	}
}

// RevokeAccountSessions implements auth.SessionRevoker: every live
// session of the account is notified with SESSION_REPLACED{REVOKED},
// closed and released; an attached character is detached.
func (r *Registry) RevokeAccountSessions(ctx context.Context, accountID id.UUID) {
	var closeConns []*listener.Conn
	var charID *id.UUID
	var epoch uint64
	r.mu.Lock()
	if s, ok := r.byAccount[accountID]; ok {
		epoch = s.epoch
		if s.graceTimer != nil {
			s.graceTimer.Stop()
			s.graceTimer = nil
		}
		if s.resumeCurrent != nil {
			delete(r.resumes, s.resumeCurrent.cred)
			delete(r.resumeSess, s.resumeCurrent.cred)
		}
		if s.resumePrev != nil {
			delete(r.resumes, s.resumePrev.cred)
			delete(r.resumeSess, s.resumePrev.cred)
		}
		if s.conn != nil {
			closeConns = append(closeConns, s.conn)
			delete(r.conns, s.conn)
			s.conn = nil
		}
		if s.charID != nil {
			c := *s.charID
			charID = &c
			delete(r.liveChar, *s.charID)
			s.charID = nil
		}
		r.releaseLocked(s)
	}
	r.mu.Unlock()

	for _, c := range closeConns {
		_ = r.send(c, 8, &protocolv1.S2CSessionReplaced{
			Reason: protocolv1.SessionReplacedReason_SESSION_REPLACED_REASON_REVOKED,
		})
		c.Close(1008, "revoked")
	}
	if charID != nil {
		_ = account.SubmitActivity(ctx, r.q, *charID, epoch,
			account.ActivityDetached, r.cfg.Now())
	}
}
