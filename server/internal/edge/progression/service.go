// Package progression is the edge surface for the 511–515 progression
// wire set: C2S_SKILL_UPGRADE (511), C2S_POTENTIAL_ALLOCATE (512) and
// C2S_RESPEC (513) admit as durable commands; S2C_PROGRESSION_MUTATE_
// RESULT (514) delivers the committed client_result and
// S2C_PROGRESSION_STATE (515) is the replaceable full snapshot pushed
// after attach and every committed change.
//
// ADR-0083: C2S_RESPEC carries a required NPC-service consult —
// NpcServiceValid — declared here as ConsultPort. The producer lives in
// edge/world (IMP-018); when no consult is bound the consumer fails
// closed with INVALID_STATE and never fabricates a verdict.
package progression

import (
	"time"

	progressiond "thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Wire ids this service owns (messages.md § Progression).
const (
	msgIDSkillUpgrade = 511
	msgIDAllocate     = 512
	msgIDRespec       = 513
	msgIDMutateResult = 514
	msgIDState        = 515
)

// ServiceRespec is the canonical npc service id C2S_RESPEC consults on
// (npcs.md § services; npc_shop_catalog.md `service.respec`).
const ServiceRespec = "service.respec"

// ConsultPort is the ADR-0083 consult seam: one typed request and one
// single-shot reply. The producer (edge/world, IMP-018) answers whether
// the named NPC currently offers serviceID to this character —
// existence, capability, range and open-session validity folded in. The
// signature matches the producer exactly; consumers are fail-closed:
//
//	(true, nil)  → the service is valid at admission; proceed.
//	(false, nil) → invalid service/NPC or out of range → OUT_OF_RANGE.
//	(*ConsultError) → a typed wire-code rejection (e.g. IN_COMBAT).
//	(other error) → consult unavailable → INVALID_STATE.
type ConsultPort interface {
	NpcServiceValid(characterID, npcID, serviceID string) (bool, error)
}

// ConsultError lets a consult producer carry a wire-code rejection (e.g.
// ERROR_CODE_IN_COMBAT); handlers map it onto S2C_ERROR verbatim.
type ConsultError struct {
	Code protocolv1.ErrorCode
}

func (e *ConsultError) Error() string { return "progression: consult reject " + e.Code.String() }

// Service is the edge/progression registration surface: the durable
// queue for Submit/Await/Ack, the progression store for spatial evidence
// and post-commit 515 reads, and the optional NPC-service consult.
type Service struct {
	q        *queue.Queue
	store    *progressiond.Store
	consults ConsultPort
	now      func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the admission timestamp source (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New builds the service. consults may be nil — the respec handler then
// fails closed until IMP-018 binds the producer.
func New(q *queue.Queue, store *progressiond.Store, consults ConsultPort, opts ...Option) *Service {
	s := &Service{q: q, store: store, consults: consults,
		now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Register installs the 511/512/513 handlers on the router registry.
func (s *Service) Register(rt *router.Registry) error {
	for _, e := range []struct {
		id uint32
		h  router.Handler
	}{
		{msgIDSkillUpgrade, s.upgrade},
		{msgIDAllocate, s.allocate},
		{msgIDRespec, s.respec},
	} {
		if err := rt.Register(e.id, e.h); err != nil {
			return err
		}
	}
	return nil
}
