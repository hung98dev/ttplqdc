package social

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	sociald "thinhthan/internal/durable/social"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Wire ids the service handles.
const (
	msgIDChatSend          = 600
	msgIDChatMessage       = 601
	msgIDFriendRequestPush = 612
	msgIDFriendState       = 616
	msgIDBlockState        = 619
	msgIDReportResult      = 633
	msgIDSocialResult      = 654
	msgIDChatSendResult    = 655
)

// maxChatGraphemes mirrors social.md § Message Content (1..240).
const maxChatGraphemes = 240

// Service is the social wire surface: durable intents ride the
// ADR-0081 seam through the queue; C2S_CHAT_SEND is the non-durable
// fanout (ephemeral runtime per service_boundaries.md).
type Service struct {
	q       *queue.Queue
	store   *sociald.Store
	hub     Hub
	limiter *limiter
	now     func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the admission/rate-limit timestamp source (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New builds the service. hub may be nil — pushes then no-op and
// WHISPER/PARTY/GUILD/LOCAL fanouts fail closed.
func New(q *queue.Queue, store *sociald.Store, hub Hub, opts ...Option) *Service {
	s := &Service{q: q, store: store, hub: hub, limiter: newLimiter(),
		now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Register installs the social handlers: 611/613/614/615/617/618/632
// as durable intents and 600 as the non-durable chat send.
func (s *Service) Register(rt *router.Registry) error {
	for _, e := range []struct {
		id uint32
		h  router.Handler
	}{
		{611, s.friendRequest},
		{613, s.friendAccept},
		{614, s.friendDecline},
		{615, s.friendRemove},
		{617, s.blockAdd},
		{618, s.blockRemove},
		{632, s.report},
	} {
		if err := rt.Register(e.id, e.h); err != nil {
			return err
		}
	}
	return rt.RegisterNonDurable(msgIDChatSend, s.chatSend)
}

// send marshals msg and enqueues it on the bound conn (the listener's
// registry is authoritative for the delivery class per id).
func (s *Service) send(c *listener.Conn, msgID uint32, msg proto.Message) error {
	if c == nil {
		return nil
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Send(&protocolv1.Envelope{MessageId: msgID, Payload: payload},
		listener.DeliveryControl)
}

// deliver pushes one frame to every live session of characterID via the
// hub; a nil hub is a no-op.
func (s *Service) deliver(ctx context.Context, characterID id.UUID,
	msgID uint32, msg proto.Message) error {
	if s.hub == nil {
		return nil
	}
	return s.hub.Deliver(ctx, characterID, msgID, msg)
}

// presence reads the hub's presence; absent hub or unknown character is
// OFFLINE with zero zone/activity.
func (s *Service) presence(characterID id.UUID) Presence {
	if s.hub == nil {
		return Presence{}
	}
	return s.hub.Presence(characterID)
}

// submitError maps admission failures to wire codes (ADR-0081 clone).
func submitError(err error) error {
	switch {
	case errors.Is(err, queue.ErrQueueFull), errors.Is(err, queue.ErrBackpressure):
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED}
	case errors.Is(err, queue.ErrErasureFenced):
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_PENDING_DELETION}
	case errors.Is(err, queue.ErrShutdown):
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE}
	default:
		return err
	}
}

// awaitError maps a non-committed terminal receipt to a wire code.
func awaitError(err error) error {
	var te *idempotency.TerminalError
	if errors.As(err, &te) {
		switch te.State {
		case idempotency.ReceiptExpiredUncommitted:
			return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_OPERATION_EXPIRED}
		default:
			return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE}
		}
	}
	return err
}

// run is the shared durable-admission tail: Submit → AwaitClientOutcome
// → deliver the retained result → post-commit pushes → Ack
// (deliver-then-ack; a replayed delivery may re-ack).
func (s *Service) run(ctx context.Context, v router.View,
	rec *journalv1.DurableCommandRecord) (*journalv1.JournalOutcome, error) {
	fam := rec.GetOperationFamily()
	owner := idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	if err := s.q.Submit(ctx, rec); err != nil {
		return nil, submitError(err)
	}
	outcome, err := s.q.AwaitClientOutcome(ctx, fam, owner, opID)
	if err != nil {
		return nil, awaitError(err)
	}
	return outcome, nil
}
