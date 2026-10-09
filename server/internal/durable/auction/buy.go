package auction

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/items"
	"thinhthan/internal/durable/lockorder"
)

// BuyIn is one C2S_AUCTION_BUY (732) intent.
type BuyIn struct {
	OperationID         id.UUID
	CharacterID         id.UUID
	AccountID           id.UUID
	ListingID           id.UUID
	ExpectedPriceCommon int64
	Lock                *items.TradeLockLedger
}

// BuyOut carries the committed settlement: the SOLD listing, the
// proceeds escrow id for S2C_AUCTION_SOLD (736), and the tax withheld.
type BuyOut struct {
	Listing    *Listing
	ProceedsID id.UUID
	Price      int64
	Tax        int64
	Amount     int64 // seller proceeds = price - tax
}

// Buy settles one FIXED_PRICE purchase atomically (trading_auction.md §
// Purchase + § Seller Proceeds): listing row lock, expected-price check,
// same-account rejection, buyer capacity validated before debit, tax
// sink, asset escrow -> buyer inventory, SOLD + PENDING proceeds in one
// transaction. A full buyer inventory leaves the listing ACTIVE and
// charges nothing; seller-cap overflow never blocks settlement (the
// proceeds stay PENDING until the seller claims).
func (s *Store) Buy(ctx context.Context, tx pgx.Tx, in BuyIn) (*BuyOut, error) {
	if err := s.gate(ctx, tx, in.CharacterID, false); err != nil {
		return nil, err
	}
	l, err := s.loadForUpdate(ctx, tx, in.ListingID)
	if err != nil {
		return nil, err
	}
	if l.State != StateActive || !s.now().Before(l.ExpiresAt) {
		return nil, ErrNotActive
	}
	if in.ExpectedPriceCommon != l.PriceCommon {
		return nil, ErrStateConflict
	}
	if l.SellerAccountID == in.AccountID {
		return nil, ErrSameAccount
	}
	slot, err := s.freeInventorySlot(ctx, tx, in.CharacterID)
	if err != nil {
		return nil, err
	}
	if _, err := currency.Debit(ctx, tx, currency.Mutation{
		CharacterID: in.CharacterID, CurrencyID: currency.Common,
		Delta:       l.PriceCommon,
		OperationID: in.OperationID,
		ReasonCode:  "AUCTION_PURCHASE",
		SourceRef:   l.ListingID.String(),
		Actor:       currency.ActorPlayer,
	}); err != nil {
		return nil, translateCurrency(err)
	}
	if err := s.releaseEscrowTo(ctx, tx, l.ItemInstanceID, l.ListingID,
		in.CharacterID, slot, in.Lock); err != nil {
		return nil, err
	}
	now := s.now()
	out := &BuyOut{
		ProceedsID: id.NewV4(),
		Price:      l.PriceCommon,
		Tax:        tax(l.PriceCommon, now),
	}
	out.Amount = out.Price - out.Tax
	var buyerAccount string
	if err := tx.QueryRow(ctx,
		`SELECT account_id FROM characters WHERE character_id=$1`,
		in.CharacterID.String()).Scan(&buyerAccount); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO auction_proceeds
	  (proceeds_id, settled_at, seller_character_id, seller_account_id,
	   buyer_character_id, buyer_account_id, proceeds_amount, item_id,
	   quantity, listing_id, state)
	  VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'PENDING')`,
		out.ProceedsID.String(), now, l.SellerCharacterID.String(),
		l.SellerAccountID.String(), in.CharacterID.String(), buyerAccount,
		out.Amount, l.ItemID, l.Quantity, l.ListingID.String()); err != nil {
		return nil, fmt.Errorf("auction: insert proceeds: %w", err)
	}
	buyerAcct, err := id.ParseUUID(buyerAccount)
	if err != nil {
		return nil, fmt.Errorf("auction: buyer account id: %w", err)
	}
	day := now.UTC().Truncate(24 * time.Hour)
	if err := s.rollups(ctx, tx, l, in.CharacterID, buyerAcct, out, day, now); err != nil {
		return nil, err
	}
	if err := s.transition(ctx, tx, l.ListingID, StateSold, &in.CharacterID, now); err != nil {
		return nil, err
	}
	l.State = StateSold
	l.EndedAt = &now
	out.Listing = l
	return out, nil
}

// freeInventorySlot returns the lowest free "inv.<N>" slot for the
// character, holding the character_inventories row lock so concurrent
// settlements serialize; ErrInventoryFull when capacity is exhausted
// (buyer-side capacity check runs before any debit).
func (s *Store) freeInventorySlot(ctx context.Context, tx pgx.Tx, characterID id.UUID) (string, error) {
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("character_inventories", characterID)); err != nil {
		return "", err
	}
	var capacity int
	if err := tx.QueryRow(ctx,
		`SELECT capacity FROM character_inventories WHERE character_id=$1`,
		characterID.String()).Scan(&capacity); err != nil {
		return "", fmt.Errorf("auction: load inventory capacity: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT slot FROM item_locations
	  WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY'`,
		characterID.String())
	if err != nil {
		return "", err
	}
	defer rows.Close()
	used := map[string]bool{}
	for rows.Next() {
		var slot string
		if err := rows.Scan(&slot); err != nil {
			return "", err
		}
		used[slot] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(used) >= capacity {
		return "", ErrInventoryFull
	}
	for i := 0; i < capacity; i++ {
		slot := fmt.Sprintf("inv.%d", i)
		if !used[slot] {
			return slot, nil
		}
	}
	return "", ErrInventoryFull
}

// releaseEscrowTo moves the escrowed asset into the buyer's inventory as
// part of the settlement transaction. Guarded on the exact escrow row
// (kind + listing) so a concurrent custody change aborts the move.
func (s *Store) releaseEscrowTo(ctx context.Context, tx pgx.Tx, instanceID, listingID,
	buyerCharacterID id.UUID, slot string, lock *items.TradeLockLedger) error {
	if lock != nil {
		var qty int
		if err := tx.QueryRow(ctx, `SELECT quantity FROM item_instances
		  WHERE item_instance_id=$1`, instanceID.String()).Scan(&qty); err != nil {
			return err
		}
		if err := lock.AssertOpAllowed(instanceID, qty, items.OpCustodyMove, 0); err != nil {
			return ErrItemLocked
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE item_locations
	   SET location_kind='CHARACTER_INVENTORY', character_id=$3,
	       listing_id=NULL, guild_id=NULL, slot=$4
	 WHERE item_instance_id=$1 AND location_kind='AUCTION_ESCROW'
	   AND listing_id=$2`,
		instanceID.String(), listingID.String(), buyerCharacterID.String(), slot)
	if err != nil {
		return fmt.Errorf("auction: release escrow: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotEscrowed
	}
	return nil
}
