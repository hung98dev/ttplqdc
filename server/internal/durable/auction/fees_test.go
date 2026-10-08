package auction

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestAuctionTaxFivePercentCeiling asserts sale tax = floor(price*5%)
// outside the Morning Market window (trading_auction.md § Fees).
func TestAuctionTaxFivePercentCeiling(t *testing.T) {
	cases := []struct{ price, want int64 }{
		{100, 5}, {1_000, 50}, {19_999, 999}, {2_000_000_000, 100_000_000},
	}
	for _, c := range cases {
		// 2026-10-01 12:00 HCM = outside 06:00-08:00 window.
		at := time.Date(2026, 10, 1, 12, 0, 0, 0, hcm)
		if got := tax(c.price, at); got != c.want {
			t.Fatalf("tax(%d)=%d want %d", c.price, got, c.want)
		}
	}
}

// TestMorningMarketThreePercentCeiling asserts purchases committed inside
// 06:00-08:00 Asia/Ho_Chi_Minh pay floor(price*3%) — including listings
// created before the window (rate by purchase_commit_timestamp).
func TestMorningMarketThreePercentCeiling(t *testing.T) {
	inWindow := func() time.Time { // today's 07:00 HCM, inside the window
		n := time.Now().In(hcm)
		return time.Date(n.Year(), n.Month(), n.Day(), 7, 0, 0, 0, hcm)
	}()
	if got := tax(10_000, inWindow); got != 300 {
		t.Fatalf("window tax %d", got)
	}
	// UTC boundary: 23:30 UTC = 06:30 HCM next day — in window.
	at := time.Date(2026, 9, 30, 23, 30, 0, 0, time.UTC)
	if got := tax(10_000, at); got != 300 {
		t.Fatalf("utc-cross tax %d", got)
	}
	// Edges: 06:00 inclusive, 08:00 exclusive.
	if tax(10_000, time.Date(2026, 10, 1, 6, 0, 0, 0, hcm)) != 300 {
		t.Fatal("06:00 not in window")
	}
	if tax(10_000, time.Date(2026, 10, 1, 8, 0, 0, 0, hcm)) != 500 {
		t.Fatal("08:00 still in window")
	}
	// Live buy inside the window: tax is 3% even though listed earlier.
	s := testStore(t).WithNow(func() time.Time { return inWindow })
	sellerAcct, seller := eligible(t, 10_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	buyerAcct, buyer := eligible(t, 100_000)
	lid := listOK(t, s, sellerAcct, seller, inst, 10_000)
	var out *BuyOut
	if err := inTx(t, func(tx pgx.Tx) (err error) {
		out, err = s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: buyer,
			AccountID: buyerAcct, ListingID: lid, ExpectedPriceCommon: 10_000})
		return err
	}); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if out.Tax != 300 || out.Amount != 9_700 {
		t.Fatalf("window settle %+v", out)
	}
}

// TestSaleFeeAndProceedsAtomic asserts the listing fee and proceeds
// escrow commit in the same transaction as the sale: fee is a
// non-refundable sink, proceeds = price - tax.
func TestSaleFeeAndProceedsAtomic(t *testing.T) {
	s := testStore(t)
	sellerAcct, seller := eligible(t, 1_000)
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: sellerAcct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 5_000})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	// fee = max(10, floor(5000*1%)) = 50.
	if w := wallet(t, seller); w != 1_000-50 {
		t.Fatalf("fee wallet %d", w)
	}
	buyerAcct, buyer := eligible(t, 10_000)
	var out *BuyOut
	if err := inTx(t, func(tx pgx.Tx) (err error) {
		out, err = s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: buyer,
			AccountID: buyerAcct, ListingID: lid, ExpectedPriceCommon: 5_000})
		return err
	}); err != nil {
		t.Fatalf("buy: %v", err)
	}
	// Seller proceeds PENDING exist post-commit; fee never refunded on
	// later cancel paths either (the listing is already SOLD).
	var amount int64
	if err := pool(t).QueryRow(ctx(t),
		`SELECT proceeds_amount FROM auction_proceeds WHERE proceeds_id=$1`,
		out.ProceedsID.String()).Scan(&amount); err != nil {
		t.Fatalf("proceeds: %v", err)
	}
	if amount != 4_750 {
		t.Fatalf("amount %d", amount)
	}
	if w := wallet(t, seller); w != 950 {
		t.Fatalf("seller credited early: %d", w)
	}
}

// listOK lists one unit of inst at price, returning the listing id.
func listOK(t *testing.T, s *Store, acct, char, inst id.UUID, price int64) id.UUID {
	t.Helper()
	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: char,
			AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: price})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	return lid
}
