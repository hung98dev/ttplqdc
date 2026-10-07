package inventory

import (
	"context"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/inventory"
	"thinhthan/internal/durable/items"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/router"
)

// ConsultPort is the read-only edge/world consult surface sibling
// admission handlers consume (ADR-0083). The consumer signature is
// (tick, ok, error) — the producer exposes (tick, error); the 1-line
// adapter at app/ composition maps producer nil-error to ok=true.
// A nil port fails closed at USE admission.
type ConsultPort interface {
	// PartitionTick returns the character's owning partition current
	// tick. ok=false means no live partition; err is the bounded-await
	// timeout/transport failure.
	PartitionTick(characterID id.UUID) (tick uint64, ok bool, err error)
}

// Service is the edge/inventory registration surface: it holds the
// durable seams the 400/428 handlers and the push surface need and
// registers its handlers on the router (consumed by the composition
// root).
type Service struct {
	q        *queue.Queue
	store    *inventory.Store
	consults ConsultPort
	now      func() time.Time

	// locks resolves the live trade-lock ledger for locked_quantity
	// merge (nil resolver = no locks); claimable resolves the catalog
	// tier ids an entitlement may still grant (nil = empty list).
	locks     func(id.UUID) *items.TradeLockLedger
	claimable func(entitlementID string) []string
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the admission timestamp source (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithLockResolver injects the trade-lock ledger lookup shared with
// the durable executors (locked_quantity reporting, ADR-0064).
func WithLockResolver(f func(id.UUID) *items.TradeLockLedger) Option {
	return func(s *Service) { s.locks = f }
}

// WithClaimableTiers injects the catalog binding that lists the reward
// tier ids claimable on one entitlement (composition's content
// snapshot; nil = empty claimable list).
func WithClaimableTiers(f func(entitlementID string) []string) Option {
	return func(s *Service) { s.claimable = f }
}

// New builds the service: the durable queue for Submit/Await/Ack, the
// pool + projection store for pushes, and the consult port for USE
// admission (nil consults are fail-closed).
func New(q *queue.Queue, store *inventory.Store,
	consults ConsultPort, opts ...Option) *Service {
	s := &Service{q: q, store: store, consults: consults,
		now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// EmitStatePushes is the exported composition surface the attach flow
// (post-ATTACH_OK) and the commit path call: it emits the three
// REPLACEABLE_STATE pushes 432 wallet, 433 inventory+loadouts, 435
// entitlement panel for the bound character. A call with no bound
// conn or no attached character is a no-op.
func (s *Service) EmitStatePushes(ctx context.Context, v router.View) error {
	if v.Conn == nil || v.CharacterID == nil {
		return nil
	}
	charID := *v.CharacterID

	wallet, err := s.store.WalletPush(ctx, charID)
	if err != nil {
		return err
	}
	if err := s.send(v.Conn, msgIDWalletState, wallet); err != nil {
		return err
	}

	var ledgerLocked func(id.UUID) int
	if s.locks != nil {
		if l := s.locks(charID); l != nil {
			ledgerLocked = l.LockedQty
		}
	}
	inv, err := s.store.InventoryPush(ctx, charID, ledgerLocked)
	if err != nil {
		return err
	}
	if err := s.send(v.Conn, msgIDInventoryState, inv); err != nil {
		return err
	}

	panel, err := s.store.EntitlementPush(ctx, v.AccountID, charID, s.claimable)
	if err != nil {
		return err
	}
	return s.send(v.Conn, msgIDEntitlementPanel, panel)
}
