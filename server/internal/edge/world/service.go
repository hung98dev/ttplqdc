package world

import (
	"time"

	"thinhthan/internal/durable/queue"
	simworld "thinhthan/internal/sim/world"
)

// Wire ids this service owns.
const (
	msgIDInteract  = 103 // C2S_INTERACT
	msgIDPortal    = 104 // C2S_PORTAL_USE
	msgIDChannel   = 109 // C2S_CHANNEL_SWITCH
	msgIDRespawn   = 208 // C2S_RESPAWN_REQUEST (non-durable, ADR-0082)
	msgIDInteractR = 116 // S2C_INTERACT_RESULT
	msgIDChannelR  = 110 // S2C_CHANNEL_SWITCH_RESULT
)

// Service is the edge/world registration surface: the durable queue for
// Submit/AwaitClientOutcome/Ack, the consult port for ADR-0083 admission,
// and the world runtime for post-commit world commands.
type Service struct {
	w        *simworld.Runtime
	q        *queue.Queue
	consults Consults
	now      func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the admission timestamp source (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New builds the service; a nil consults is built over w (fail-closed
// when w is nil too).
func New(w *simworld.Runtime, q *queue.Queue, consults Consults, opts ...Option) *Service {
	s := &Service{w: w, q: q, consults: consults,
		now: func() time.Time { return time.Now().UTC() }}
	if s.consults == nil {
		s.consults = NewConsults(w)
	}
	for _, o := range opts {
		o(s)
	}
	return s
}
