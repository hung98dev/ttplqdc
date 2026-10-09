package moderation

import (
	"context"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Service is the production social.Moderation consult bound at the
// composition root (fix(IMP-034) port): on each send admission it
// checks the operator-issued restriction store, then the automated
// filter. Check order is restriction → filter; a muted sender is denied
// before content inspection.
type Service struct {
	restrictions *Restrictions
	filter       *Filter
	now          func() time.Time
}

// New constructs the consult. `now` is injectable for tests; nil uses
// time.Now.
func New(restrictions *Restrictions, filter *Filter,
	now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{restrictions: restrictions, filter: filter, now: now}
}

// CheckSend implements social.Moderation: returns ok=false plus the
// wire error code when the send is restricted (PERMISSION_DENIED on a
// muted channel subset) or rejected by the automated filter
// (CHAT_TEXT_INVALID).
func (s *Service) CheckSend(_ context.Context, senderID id.UUID,
	channel protocolv1.ChatChannel, _ id.UUID,
	text string) (protocolv1.ErrorCode, bool) {
	if s.restrictions != nil &&
		s.restrictions.Restricted(senderID, channel, s.now()) {
		return protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED, false
	}
	if s.filter != nil && s.filter.Reject(senderID, text, s.now()) {
		return protocolv1.ErrorCode_ERROR_CODE_CHAT_TEXT_INVALID, false
	}
	return 0, true
}

// Mute is the operator-action entry point — the only sanction path. A
// mute may target all player-authored channels or an explicit subset
// with server-authoritative expiry (social.md § Moderation).
func (s *Service) Mute(characterID id.UUID,
	channels []protocolv1.ChatChannel, until time.Time) {
	s.restrictions.Mute(characterID, channels, until)
}

// Lift clears a restriction (operator unmute / verdict reversal).
func (s *Service) Lift(characterID id.UUID) {
	s.restrictions.Lift(characterID)
}
