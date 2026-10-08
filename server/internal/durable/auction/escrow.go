package auction

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/items"
)

// CancelIn is one C2S_AUCTION_CANCEL_LISTING (734) intent: the caller's
// own ACTIVE listing is cancelled; the asset stays in escrow until an
// explicit reclaim (735).
type CancelIn struct {
	OperationID id.UUID
	CharacterID id.UUID
	AccountID   id.UUID
	ListingID   id.UUID
}

// Cancel transitions ACTIVE -> CANCELLED and stamps ended_at (ADR-0070).
func (s *Store) Cancel(ctx context.Context, tx pgx.Tx, in CancelIn) (*Listing, error) {
	if err := s.gate(ctx, tx, in.CharacterID, false); err != nil {
		return nil, err
	}
	l, err := s.loadForUpdate(ctx, tx, in.ListingID)
	if err != nil {
		return nil, err
	}
	if l.SellerCharacterID != in.CharacterID {
		return nil, ErrNotSeller
	}
	if l.State != StateActive {
		return nil, ErrNotActive
	}
	now := s.now()
	if err := s.transition(ctx, tx, l.ListingID, StateCancelled, nil, now); err != nil {
		return nil, err
	}
	l.State = StateCancelled
	l.EndedAt = &now
	return l, nil
}

// ReclaimIn is one C2S_AUCTION_RECLAIM (740) intent: returns a
// cancelled/expired escrow asset to the seller's inventory; a full
// inventory leaves the asset in escrow (741 INVENTORY_FULL).
type ReclaimIn struct {
	OperationID id.UUID
	CharacterID id.UUID
	AccountID   id.UUID
	ListingID   id.UUID // escrow_asset_id on the wire
	Lock        *items.TradeLockLedger
}

// Reclaim transitions CANCELLED|EXPIRED -> RECLAIMED with the escrow
// move in the same transaction; ended_at is overwritten (ADR-0070).
func (s *Store) Reclaim(ctx context.Context, tx pgx.Tx, in ReclaimIn) (*Listing, error) {
	if err := s.gate(ctx, tx, in.CharacterID, false); err != nil {
		return nil, err
	}
	l, err := s.loadForUpdate(ctx, tx, in.ListingID)
	if err != nil {
		return nil, err
	}
	if l.SellerCharacterID != in.CharacterID {
		return nil, ErrNotSeller
	}
	if l.State != StateCancelled && l.State != StateExpired {
		return nil, ErrNotReclaimable
	}
	slot, err := s.freeInventorySlot(ctx, tx, in.CharacterID)
	if err != nil {
		return nil, err
	}
	if err := s.items.ReclaimEscrow(ctx, tx, l.ItemInstanceID, slot, in.Lock); err != nil {
		return nil, translateItems(err)
	}
	now := s.now()
	if err := s.transition(ctx, tx, l.ListingID, StateReclaimed, nil, now); err != nil {
		return nil, err
	}
	l.State = StateReclaimed
	l.EndedAt = &now
	return l, nil
}

// ProceedsClaimIn is one C2S_AUCTION_PROCEEDS_CLAIM (742) intent.
type ProceedsClaimIn struct {
	OperationID id.UUID
	CharacterID id.UUID
	ProceedsID  id.UUID
}

// ClaimProceeds settles a PENDING proceeds credit into the seller's
// common balance when `current + amount <= cap`; otherwise the proceeds
// stay PENDING and the claim reports CURRENCY_CAP_EXCEEDED
// (trading_auction.md § Seller Proceeds and Currency Cap). Idempotent:
// a CLAIMED row replays as already-claimed without crediting twice.
func (s *Store) ClaimProceeds(ctx context.Context, tx pgx.Tx, in ProceedsClaimIn) (*Proceeds, error) {
	if err := s.gate(ctx, tx, in.CharacterID, false); err != nil {
		return nil, err
	}
	var p Proceeds
	err := tx.QueryRow(ctx, `SELECT proceeds_id, settled_at, seller_character_id,
		seller_account_id, buyer_character_id, buyer_account_id, proceeds_amount,
		item_id, quantity, listing_id, state, claimed_at, claim_operation_id
	  FROM auction_proceeds WHERE proceeds_id=$1 FOR UPDATE`,
		in.ProceedsID.String()).Scan(&p.ProceedsID, &p.SettledAt,
		&p.SellerCharacterID, &p.SellerAccountID, &p.BuyerCharacterID, &p.BuyerAccountID,
		&p.ProceedsAmount, &p.ItemID, &p.Quantity, &p.ListingID, &p.State,
		&p.ClaimedAt, &p.ClaimOperationID)
	if err != nil {
		return nil, fmt.Errorf("auction: load proceeds: %w", err)
	}
	if p.SellerCharacterID != in.CharacterID {
		return nil, ErrNotSeller
	}
	if p.State == ProceedsClaimed {
		// Idempotent replay: already claimed — report the settled credit,
		// never credit a second time.
		return &p, nil
	}
	var balance int64
	if err := tx.QueryRow(ctx,
		`SELECT balance FROM character_currencies
		  WHERE character_id=$1 AND currency_id=$2`,
		in.CharacterID.String(), string(currency.Common)).Scan(&balance); err != nil {
		return nil, fmt.Errorf("auction: load balance: %w", err)
	}
	if balance+p.ProceedsAmount > currency.Caps[currency.Common] {
		// Stays PENDING; the wire reports CURRENCY_CAP_EXCEEDED (743).
		return nil, ErrCapExceeded
	}
	if _, err := currency.Credit(ctx, tx, currency.Mutation{
		CharacterID: in.CharacterID, CurrencyID: currency.Common,
		Delta:       p.ProceedsAmount,
		OperationID: in.OperationID,
		ReasonCode:  "AUCTION_PROCEEDS_CLAIM",
		SourceRef:   p.ListingID.String(),
		Actor:       currency.ActorPlayer,
	}); err != nil {
		return nil, translateCurrency(err)
	}
	now := s.now()
	if _, err := tx.Exec(ctx, `UPDATE auction_proceeds
	   SET state='CLAIMED', claimed_at=$2, claim_operation_id=$3
	 WHERE proceeds_id=$1`, in.ProceedsID.String(), now, in.OperationID.String()); err != nil {
		return nil, err
	}
	p.State = ProceedsClaimed
	p.ClaimedAt = &now
	p.ClaimOperationID = &in.OperationID
	return &p, nil
}

// IsRejectCode reports whether err is a deterministic domain rejection
// the executor commits as a verdict (vs infrastructure failure).
func IsRejectCode(err error) bool {
	switch {
	case errors.Is(err, ErrNotActive), errors.Is(err, ErrNotReclaimable),
		errors.Is(err, ErrNotSeller), errors.Is(err, ErrSameAccount),
		errors.Is(err, ErrBelowFloor), errors.Is(err, ErrOutOfRange),
		errors.Is(err, ErrCapacityFull), errors.Is(err, ErrInventoryFull),
		errors.Is(err, ErrInsufficient), errors.Is(err, ErrCapExceeded),
		errors.Is(err, ErrLevelGate), errors.Is(err, ErrAgeGate),
		errors.Is(err, ErrProceedsNotPending), errors.Is(err, ErrStateConflict),
		errors.Is(err, ErrItemLocked), errors.Is(err, ErrNotEscrowed),
		errors.Is(err, ErrUntradable):
		return true
	}
	return false
}
