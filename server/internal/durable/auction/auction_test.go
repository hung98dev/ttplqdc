package auction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestFixedPriceListingEscrow asserts listing atomically moves the asset
// inventory -> AUCTION_ESCROW keyed to the new listing (trading_auction.md
// § Escrow).
func TestFixedPriceListingEscrow(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 1_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 5, 0, "UNBOUND")
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 5, PriceCommon: 1000})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	var kind, listingRef string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT location_kind, listing_id FROM item_locations WHERE item_instance_id=$1`,
		inst.String()).Scan(&kind, &listingRef); err != nil {
		t.Fatalf("loc: %v", err)
	}
	if kind != "AUCTION_ESCROW" || listingRef != lid.String() {
		t.Fatalf("escrow %s %s", kind, listingRef)
	}
	if w := wallet(t, seller); w != 1_000_000-1000/100 { // fee max(10, 1%)=10... 1000*1%=10
		t.Fatalf("wallet %d", w)
	}
}

// TestFixedPricePurchaseSettlement asserts the purchase commits
// atomically: debit, item to buyer, SOLD + ended_at, proceeds PENDING.
func TestFixedPricePurchaseSettlement(t *testing.T) {
	s := testStore(t)
	sellerAcct, seller := eligible(t, 1_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 3, 0, "UNBOUND")
	buyerAcct, buyer := eligible(t, 10_000)
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: sellerAcct, ItemInstanceID: inst, Quantity: 3, PriceCommon: 600})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	var out *BuyOut
	if err := inTx(t, func(tx pgx.Tx) (err error) {
		out, err = s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: buyer,
			AccountID: buyerAcct, ListingID: lid, ExpectedPriceCommon: 600})
		return err
	}); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if out.Amount != 570 || out.Tax != 30 {
		t.Fatalf("settle %+v", out)
	}
	if w := wallet(t, buyer); w != 10_000-600 {
		t.Fatalf("buyer wallet %d", w)
	}
	var kind, charRef string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT location_kind, character_id FROM item_locations WHERE item_instance_id=$1`,
		inst.String()).Scan(&kind, &charRef); err != nil {
		t.Fatalf("loc: %v", err)
	}
	if kind != "CHARACTER_INVENTORY" || charRef != buyer.String() {
		t.Fatalf("item %s %s", kind, charRef)
	}
	var pst string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT state FROM auction_proceeds WHERE proceeds_id=$1`, out.ProceedsID.String()).Scan(&pst); err != nil {
		t.Fatalf("proceeds: %v", err)
	}
	if pst != "PENDING" {
		t.Fatalf("proceeds %q", pst)
	}
}

// TestTaxDeductionAndProceedsEscrow asserts proceeds = price - tax
// persist as seller escrow (never fails on seller cap).
func TestTaxDeductionAndProceedsEscrow(t *testing.T) {
	s := testStore(t)
	sellerAcct, seller := eligible(t, 2_000_000_000-1) // near cap
	inst := mkItem(t, seller, "item.weapon.t3", 1, 0, "UNBOUND")
	buyerAcct, buyer := eligible(t, 100_000)
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: sellerAcct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 50_000})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	var out *BuyOut
	if err := inTx(t, func(tx pgx.Tx) (err error) {
		out, err = s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: buyer,
			AccountID: buyerAcct, ListingID: lid, ExpectedPriceCommon: 50_000})
		return err
	}); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if out.Amount != 47_500 || out.Tax != 2_500 {
		t.Fatalf("tax %+v", out)
	}
	var amount int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT proceeds_amount FROM auction_proceeds WHERE listing_id=$1`, lid.String()).Scan(&amount); err != nil {
		t.Fatalf("proceeds: %v", err)
	}
	if amount != 47_500 {
		t.Fatalf("amount %d", amount)
	}
	// Seller near cap: claim rejects CURRENCY_CAP_EXCEEDED, stays PENDING.
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.ClaimProceeds(ctx(t), tx, ProceedsClaimIn{OperationID: id.NewV4(),
			CharacterID: seller, ProceedsID: out.ProceedsID})
		return err
	}); !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("claim err=%v", err)
	}
	var st string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT state FROM auction_proceeds WHERE proceeds_id=$1`, out.ProceedsID.String()).Scan(&st); err != nil {
		t.Fatalf("state: %v", err)
	}
	if st != "PENDING" {
		t.Fatalf("still %q", st)
	}
}

// TestAuctionPriceFloorCode asserts a listing below the floor rejects
// AH_PRICE_FLOOR_NOT_MET (ErrBelowFloor) before escrow or debit.
func TestAuctionPriceFloorCode(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 1_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 2, 0, "UNBOUND")
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 2, PriceCommon: 150})
		return err
	}); !errors.Is(err, ErrBelowFloor) {
		t.Fatalf("floor err=%v", err)
	}
	if w := wallet(t, seller); w != 1_000_000 {
		t.Fatalf("fee debited %d", w)
	}
}

// TestAuctionSameAccountBuy asserts seller and same-account characters
// cannot buy their own listing (SAME_ACCOUNT_FORBIDDEN).
func TestAuctionSameAccountBuy(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 1_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	alt := mkCharacter(t, acct, 20, 48*time.Hour) // same account
	mkWallet(t, alt, 10_000)
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 100})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range []id.UUID{seller, alt} {
		if err := inTx(t, func(tx pgx.Tx) error {
			_, err := s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: c,
				AccountID: acct, ListingID: lid, ExpectedPriceCommon: 100})
			return err
		}); !errors.Is(err, ErrSameAccount) {
			t.Fatalf("buy err=%v", err)
		}
	}
}

// TestAuctionExpectedPrice asserts an expected_price mismatch rejects
// STATE_CONFLICT (ADR-0060).
func TestAuctionExpectedPrice(t *testing.T) {
	s := testStore(t)
	sellerAcct, seller := eligible(t, 1_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	buyerAcct, buyer := eligible(t, 10_000)
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: sellerAcct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 500})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: buyer,
			AccountID: buyerAcct, ListingID: lid, ExpectedPriceCommon: 400})
		return err
	}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	if w := wallet(t, buyer); w != 10_000 {
		t.Fatalf("debited %d", w)
	}
}
