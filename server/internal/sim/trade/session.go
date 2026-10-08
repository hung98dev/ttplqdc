// Package trade implements the direct-trade runtime: the per-session
// finite-state machine, session admission and lifecycle, offer/lock
// bookkeeping, and the seam that hands a committed JournalTrade to the
// durable queue (trading_auction.md § Direct Trade; ADR-0060/0062/0063).
//
// Sessions are runtime state of the owning map-instance simulation and
// are never persisted: a process restart cancels every open session and
// releases every trade lock without moving anything.
package trade

import (
	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
)

// State is the direct-trade FSM (docs/05_network/messages.md § 706
// TradeOfferState). COMPLETED is terminal and represented by removing
// the session — a session value never carries it.
type State int

const (
	// StateOpen is the admitted session before both sides confirm.
	StateOpen State = iota
	// StateLocked is reached once both sides confirmed; offers are frozen.
	StateLocked
	// StateCommitting covers the durable 708 finalization in flight.
	StateCommitting
)

// Offer is one offered stack slice (ADR-0062): q units of the instance.
type Offer struct {
	ItemInstanceID id.UUID
	ItemID         string
	Quantity       int
}

// Side is one participant of the session.
type Side struct {
	CharacterID  id.UUID
	AccountID    id.UUID
	Offers       []Offer
	CommonAmount int64
	Confirmed    bool
	Ledger       *items.TradeLockLedger
}

// Session is one runtime trade session between two characters.
type Session struct {
	TradeID      id.UUID
	Initiator    *Side
	Counterpart  *Side
	State        State
	Revision     uint64
	LastActivity int64 // unix ms of the last state-relevant request
	// CommitActor is the side that issued the admitted 708 — its
	// partner receives the 709 with an empty operation_id.
	CommitActor id.UUID
}

// other returns the opposite side of the session.
func (s *Session) other(who id.UUID) *Side {
	if s.Initiator.CharacterID == who {
		return s.Counterpart
	}
	return s.Initiator
}

// side returns the participant side of the given character.
func (s *Session) side(who id.UUID) *Side {
	if s.Initiator.CharacterID == who {
		return s.Initiator
	}
	return s.Counterpart
}

// has reports whether the character is a participant.
func (s *Session) has(who id.UUID) bool {
	return s.Initiator.CharacterID == who || s.Counterpart.CharacterID == who
}

// release drops every lock the session holds (cancel paths — nothing
// has moved, so the ledgers are simply cancelled).
func (s *Session) release() {
	s.Initiator.Ledger.Cancel()
	s.Counterpart.Ledger.Cancel()
}
