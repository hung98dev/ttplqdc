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

// ListIn is one C2S_AUCTION_LIST (730) intent resolved for the durable
// layer: the seller identity comes from the attached session view; the
// item instance, lot quantity and total lot price come off the wire.
type ListIn struct {
	OperationID      id.UUID
	CharacterID      id.UUID
	AccountID        id.UUID
	SessionEpoch     uint64
	OwnershipEpoch   uint64
	ItemInstanceID   id.UUID
	Quantity         int64
	PriceCommon      int64
	Lock             *items.TradeLockLedger
	AdmittedAtUnixMs int64
}

// List creates one FIXED_PRICE listing: eligibility gates, tradability,
// floor and cap checks, the non-refundable listing fee, the atomic
// inventory -> AUCTION_ESCROW move and the row insert, in the caller's
// transaction (trading_auction.md § Auction House).
func (s *Store) List(ctx context.Context, tx pgx.Tx, in ListIn) (*Listing, error) {
	if in.PriceCommon > MaxPriceCommon || in.PriceCommon < 0 || in.Quantity <= 0 {
		return nil, ErrOutOfRange
	}
	if err := s.gate(ctx, tx, in.CharacterID, true); err != nil {
		return nil, err
	}
	// Lock the instance row first so the quantity/binding read below is
	// stable for the whole settlement (EscrowForListing revalidates the
	// location under its own locks).
	var inst items.Instance
	var effBinding string
	if err := tx.QueryRow(ctx, `SELECT item_instance_id, item_id, quantity,
		effective_binding FROM item_instances WHERE item_instance_id=$1
		FOR UPDATE`, in.ItemInstanceID.String()).Scan(
		&inst.InstanceID, &inst.ItemID, &inst.Quantity, &effBinding); err != nil {
		return nil, fmt.Errorf("auction: load item instance: %w", err)
	}
	if inst.Quantity != int(in.Quantity) {
		// one listing = one indivisible lot: the wire quantity is the
		// whole lot, so it must equal the stack quantity being listed.
		return nil, fmt.Errorf("%w: lot quantity %d != stack %d", ErrOutOfRange, in.Quantity, inst.Quantity)
	}
	b, perr := items.ParseBinding(effBinding)
	if perr != nil {
		return nil, perr
	}
	inst.EffectiveBinding = b
	lk, err := s.defs(ctx, inst.ItemID)
	if err != nil {
		return nil, err
	}
	if !lk.Tradable {
		return nil, ErrUntradable
	}
	if err := tradabilityOf(&inst); err != nil {
		return nil, err
	}
	if in.PriceCommon < listingFloor(lk, in.Quantity) {
		return nil, ErrBelowFloor
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM auction_listings
	  WHERE seller_character_id=$1 AND state='ACTIVE'`,
		in.CharacterID.String()).Scan(&active); err != nil {
		return nil, err
	}
	if active >= MaxActiveListings {
		return nil, ErrCapacityFull
	}
	fee := listingFee(in.PriceCommon)
	if _, err := currency.Debit(ctx, tx, currency.Mutation{
		CharacterID: in.CharacterID, CurrencyID: currency.Common,
		Delta:       fee,
		OperationID: in.OperationID,
		ReasonCode:  "AUCTION_LISTING_FEE",
		SourceRef:   in.ItemInstanceID.String(),
		Actor:       currency.ActorPlayer,
	}); err != nil {
		return nil, translateCurrency(err)
	}
	listingID := id.NewV4()
	now := s.now()
	l := &Listing{
		ListingID:         listingID,
		SellerCharacterID: in.CharacterID,
		SellerAccountID:   in.AccountID,
		ItemInstanceID:    in.ItemInstanceID,
		ItemID:            inst.ItemID,
		Quantity:          in.Quantity,
		PriceCommon:       in.PriceCommon,
		ListingFeeCommon:  fee,
		State:             StateActive,
		ListedAt:          now,
		ExpiresAt:         now.Add(ListingDuration),
	}
	// The partial UNIQUE(item_instance_id) while escrowed guards the
	// asset; a collision is a retry/conflict, not a second listing.
	if _, err := tx.Exec(ctx, `INSERT INTO auction_listings
	  (listing_id, seller_character_id, seller_account_id, item_instance_id,
	   item_id, quantity, price_common, listing_fee_common, state,
	   listed_at, expires_at, ended_at, buyer_character_id, revision)
	  VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'ACTIVE',$9,$10,NULL,NULL,1)`,
		l.ListingID.String(), l.SellerCharacterID.String(), l.SellerAccountID.String(),
		l.ItemInstanceID.String(), l.ItemID, l.Quantity, l.PriceCommon,
		l.ListingFeeCommon, l.ListedAt, l.ExpiresAt); err != nil {
		return nil, fmt.Errorf("auction: insert listing: %w", err)
	}
	// Escrow after the insert: checkLocation requires the listing row.
	if err := s.items.EscrowForListing(ctx, tx, in.ItemInstanceID, listingID, in.Lock); err != nil {
		return nil, translateItems(err)
	}
	return l, nil
}

// tradabilityOf enforces the tradable-asset rules beyond the catalog
// flag: UNBOUND only, not beast equipment (items.md § Tradable Assets).
func tradabilityOf(inst *items.Instance) error {
	if inst.EffectiveBinding != items.BindingUnbound {
		return ErrUntradable
	}
	return nil
}

func translateItems(err error) error {
	switch {
	case errors.Is(err, items.ErrLocationConflict):
		return ErrNotEscrowed
	case errors.Is(err, items.ErrBindingBlocksTransfer):
		return ErrUntradable
	case errors.Is(err, items.ErrTradeLocked):
		return ErrItemLocked
	case errors.Is(err, items.ErrCapacity):
		return ErrInventoryFull
	case errors.Is(err, items.ErrSoulContracted):
		return ErrUntradable
	default:
		return err
	}
}

func translateCurrency(err error) error {
	switch {
	case errors.Is(err, currency.ErrInsufficientBalance):
		return ErrInsufficient
	case errors.Is(err, currency.ErrCapExceeded):
		return ErrCapExceeded
	default:
		return err
	}
}
