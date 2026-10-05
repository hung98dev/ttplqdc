package character

import (
	"time"

	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/router"
)

// msgIDCreate is the wire id this service owns.
const msgIDCreate = 12

// Service is the edge/character registration surface: it holds the
// durable seam dependencies the id-12 handler needs and registers its
// handler on the router (consumed by the composition root).
type Service struct {
	q        *queue.Queue
	accounts *account.Store
	now      func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the admission timestamp source (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New builds the service: the durable queue for Submit/Await/Ack and
// the account store for the post-create S2C_CHARACTER_LIST read.
func New(q *queue.Queue, accounts *account.Store, opts ...Option) *Service {
	s := &Service{q: q, accounts: accounts,
		now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Register installs the id-12 handler on the router registry.
func (s *Service) Register(rt *router.Registry) error {
	return rt.Register(msgIDCreate, s.create)
}
