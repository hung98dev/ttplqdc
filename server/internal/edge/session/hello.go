package session

import (
	"context"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// IssueTicket implements the gameplay.ticket admission of session.md
// § Login Queue: grants a slot + 60s single-use ticket, or answers the
// FIFO queue position. Reconnect paths (live character / grace) bypass.
// The registry mutex is the single Edge admission owner.
func (r *Registry) IssueTicket(ctx context.Context, accountID id.UUID,
	clientBuild uint32, platform protocolv1.ClientPlatform, protocolMinor uint32,
	contentRevision string) (Ticket, error) {
	now := r.cfg.Now()
	var t Ticket

	r.mu.Lock()
	defer r.mu.Unlock()

	// Reconnect bypass: character live or in grace.
	if s, ok := r.byAccount[accountID]; ok {
		if s.charID != nil || s.graceTimer != nil {
			r.queue.bypass(accountID)
		}
	}
	granted, pos := r.queue.admit(accountID, now)
	if !granted {
		return Ticket{QueuePosition: pos, RetryAfterMs: RetryAfterMs(pos)}, nil
	}
	var raw [32]byte
	if err := fillRandom(raw[:]); err != nil {
		return t, err
	}
	cred := credString(raw[:])
	expires := now.Add(r.cfg.TicketTTL)
	r.tickets[cred] = &ticket{
		accountID:       accountID,
		clientBuild:     clientBuild,
		platform:        platform,
		protocolMinor:   protocolMinor,
		contentRevision: contentRevision,
		expiresAt:       expires,
	}
	return Ticket{Credential: cred, ExpiresAt: expires}, nil
}

// Hello implements listener.SessionPort: consumes the HELLO credential
// (gameplay ticket or resume credential), mints/re-attaches the session,
// supersedes any older live connection for the same account.
func (r *Registry) Hello(ctx context.Context, meta listener.HelloMeta,
	msg *protocolv1.C2SHello) listener.HelloDecision {
	now := r.cfg.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	var s *sess
	var acctID id.UUID
	switch cred := msg.GetCredential().(type) {
	case *protocolv1.C2SHello_GameplayTicket:
		var ok bool
		s, ok = r.consumeTicketLocked(cred.GameplayTicket, msg, now)
		if !ok {
			return reject(protocolv1.ErrorCode_ERROR_CODE_AUTH_INVALID)
		}
		if s == nil {
			return reject(protocolv1.ErrorCode_ERROR_CODE_AUTH_EXPIRED)
		}
		acctID = s.accountID
	case *protocolv1.C2SHello_ResumeCredential:
		resumed, err := r.consumeResumeLocked(cred.ResumeCredential, now)
		if err != nil {
			if err == ErrResumeExpired {
				return reject(protocolv1.ErrorCode_ERROR_CODE_RESUME_EXPIRED)
			}
			return reject(protocolv1.ErrorCode_ERROR_CODE_AUTH_INVALID)
		}
		s = resumed
		acctID = s.accountID
		// A resume HELLO takes over the binding: if the previous conn is
		// still open (its close raced the resume), it is the stale side.
		if s.conn != nil {
			_ = r.send(s.conn, 8, &protocolv1.S2CSessionReplaced{
				Reason: protocolv1.SessionReplacedReason_SESSION_REPLACED_REASON_NEWER_SESSION,
			})
			r.closeAfterSend(s.conn)
			delete(r.conns, s.conn)
			s.conn = nil
		}
	default:
		return reject(protocolv1.ErrorCode_ERROR_CODE_AUTH_INVALID)
	}

	// Re-check account status at HELLO: a ban between ticket and connect
	// still denies entry (auth.md § Status Enforcement).
	arow, err := r.store.GetAccount(ctx, nil, acctID)
	if err != nil {
		return reject(protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE)
	}
	switch arow.Status {
	case account.StatusBanned:
		r.releaseLocked(s)
		return reject(protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_BANNED)
	}
	s.pendingDeletion = arow.Status == account.StatusPendingDeletion

	// Supersede: a second HELLO for the same account replaces the older
	// connection — old conn gets SESSION_REPLACED{NEWER_SESSION} + close
	// (ADR-0030); the live character re-attaches exactly like resume.
	var resumedChar *id.UUID
	if old, ok := r.byAccount[acctID]; ok && old != s {
		r.supersedeLocked(old, s)
		resumedChar = s.charID
	} else if s.charID != nil {
		resumedChar = s.charID
	}
	if s.graceTimer != nil {
		s.graceTimer.Stop()
		s.graceTimer = nil
	}

	r.byAccount[acctID] = s
	r.sessions[s.epoch] = s
	r.queue.setState(acctID, pickSlotState(s))
	r.scheduleRotateLocked(s)

	rc, cred, err := r.issueResumeLocked(s)
	if err != nil {
		return reject(protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE)
	}
	_ = rc

	ok := &protocolv1.S2CHelloOk{
		SessionId:              s.id[:],
		SessionEpoch:           s.epoch,
		AccountId:              s.accountID[:],
		ServerTimeMs:           now.UnixMilli(),
		HeartbeatIntervalMs:    5000,
		ConnectionTimeoutMs:    15000,
		ResumeCredential:       cred,
		ResumeExpiresAtMs:      rc.expiresAt.UnixMilli(),
		ProtocolMinor:          r.cfg.ProtocolMinor,
		ContentRevision:        s.contentRevision,
		PendingDeletion:        s.pendingDeletion,
		ResumeRotateIntervalMs: uint32(r.cfg.ResumeRotateEvery / time.Millisecond),
	}
	if resumedChar != nil {
		ok.ResumedCharacterId = resumedChar[:]
	}
	return listener.HelloDecision{OK: ok}
}

func reject(code protocolv1.ErrorCode) listener.HelloDecision {
	var retry protocolv1.Retryability
	switch code {
	case protocolv1.ErrorCode_ERROR_CODE_AUTH_INVALID,
		protocolv1.ErrorCode_ERROR_CODE_AUTH_EXPIRED,
		protocolv1.ErrorCode_ERROR_CODE_RESUME_EXPIRED:
		retry = protocolv1.Retryability_RETRYABILITY_REAUTHENTICATE
	case protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE:
		retry = protocolv1.Retryability_RETRYABILITY_RETRY_SAME_OPERATION
	default:
		retry = protocolv1.Retryability_RETRYABILITY_NEVER
	}
	return listener.HelloDecision{Reject: &listener.ErrorReject{Code: code, Retryability: retry}}
}

// consumeTicketLocked redeems a single-use gameplay ticket into a new
// session (nil,false → AUTH_INVALID; nil,true → AUTH_EXPIRED).
func (r *Registry) consumeTicketLocked(cred string, msg *protocolv1.C2SHello, now time.Time) (*sess, bool) {
	t, ok := r.tickets[cred]
	if !ok {
		return nil, false
	}
	delete(r.tickets, cred) // single-use regardless of outcome
	if now.After(t.expiresAt) {
		return nil, true
	}
	s := &sess{
		id:              id.NewV7(now),
		accountID:       t.accountID,
		epoch:           r.mintEpochLocked(t.accountID),
		attaching:       true,
		contentRevision: pickRevision(t.contentRevision, r.cfg.ContentRevision),
		platform:        t.platform,
	}
	return s, true
}

func pickRevision(clientRev, serverRev string) string {
	if clientRev != "" {
		return clientRev
	}
	return serverRev
}

func pickSlotState(s *sess) slotState {
	if s.charID != nil {
		return slotAttached
	}
	return slotCharacterSelect
}

// supersedeLocked hands the live character (if any) to the new session
// and pushes SESSION_REPLACED to the old bound conn.
func (r *Registry) supersedeLocked(old, newS *sess) {
	newS.charID = old.charID
	newS.attaching = false
	if newS.charID != nil {
		r.liveChar[*newS.charID] = newS
		newS.ownershipEpoch = r.mintOwnershipEpochLocked(*newS.charID)
	}
	old.charID = nil
	delete(r.sessions, old.epoch)
	if old.conn != nil {
		_ = r.send(old.conn, 8, &protocolv1.S2CSessionReplaced{
			Reason: protocolv1.SessionReplacedReason_SESSION_REPLACED_REASON_NEWER_SESSION,
		})
		r.closeAfterSend(old.conn)
		old.conn = nil
	}
}

// closeAfterSend delays a policy close by a short interval so a queued
// outbound frame (SESSION_REPLACED) flushes to the wire before the close
// frame: Conn.Send only enqueues to the write loop while Conn.Close
// writes the close handshake immediately.
func (r *Registry) closeAfterSend(c *listener.Conn) {
	r.cfg.After(50*time.Millisecond, func() { c.Close(1008, "superseded") })
}

// releaseLocked frees the account's slot + session bookkeeping.
func (r *Registry) releaseLocked(s *sess) {
	if s.rotateTimer != nil {
		s.rotateTimer.Stop()
		s.rotateTimer = nil
	}
	delete(r.sessions, s.epoch)
	delete(r.byAccount, s.accountID)
	r.queue.release(s.accountID)
	if s.charID != nil {
		delete(r.liveChar, *s.charID)
	}
}
