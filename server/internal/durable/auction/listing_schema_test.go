package auction

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestSettlingNeverCommitted asserts SETTLING exists only inside the
// purchase transaction: it is never a committed state (data_model.md §
// auction_listings). After settlement the row is SOLD; a concurrent
// reader can never observe SETTLING committed.
func TestSettlingNeverCommitted(t *testing.T) {
	s := testStore(t)
	sellerAcct, seller := eligible(t, 1_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	buyerAcct, buyer := eligible(t, 1_000_000)

	var lid id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: sellerAcct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 1000})
		if err != nil {
			return err
		}
		lid = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		out, err := s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: buyer,
			AccountID: buyerAcct, ListingID: lid, ExpectedPriceCommon: 1000})
		if err != nil {
			return err
		}
		if out.Listing.State != StateSold {
			t.Fatalf("state %q", out.Listing.State)
		}
		return nil
	}); err != nil {
		t.Fatalf("buy: %v", err)
	}
	var st string
	if err := pool(t).QueryRow(context.Background(),
		`SELECT state FROM auction_listings WHERE listing_id=$1`, lid.String()).Scan(&st); err != nil {
		t.Fatalf("read: %v", err)
	}
	if st == "SETTLING" {
		t.Fatal("SETTLING committed")
	}
	if st != string(StateSold) {
		t.Fatalf("state %q", st)
	}
}

// TestKeysetSearchOrder asserts the keyset search returns listings in
// canonical (item_id, price_common, listing_id) order with stable
// pagination (ADR-0065).
func TestKeysetSearchOrder(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 10_000_000)
	var prices []int64
	want := []int64{300, 100, 200, 500}
	for i, p := range want {
		inst := mkItem(t, seller, "item.search.only", 1, i, "UNBOUND")
		if err := inTx(t, func(tx pgx.Tx) error {
			_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
				AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: p})
			return err
		}); err != nil {
			t.Fatalf("list %d: %v", i, err)
		}
	}
	var page *SearchPage
	if err := inTx(t, func(tx pgx.Tx) (err error) {
		page, err = s.Search(ctx(t), tx, SearchQuery{ItemID: "item.search.only", Sort: SortPriceAsc, PageSize: 2})
		return err
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Listings) != 2 || page.NextCursor == "" {
		t.Fatalf("page %v cursor %q", len(page.Listings), page.NextCursor)
	}
	prices = append(prices, page.Listings[0].PriceCommon, page.Listings[1].PriceCommon)
	var page2 *SearchPage
	if err := inTx(t, func(tx pgx.Tx) (err error) {
		page2, err = s.Search(ctx(t), tx, SearchQuery{ItemID: "item.search.only", Sort: SortPriceAsc,
			PageSize: 2, Cursor: page.NextCursor})
		return err
	}); err != nil {
		t.Fatalf("search2: %v", err)
	}
	prices = append(prices, page2.Listings[0].PriceCommon, page2.Listings[1].PriceCommon)
	exp := []int64{100, 200, 300, 500}
	for i := range exp {
		if prices[i] != exp[i] {
			t.Fatalf("order %v want %v", prices, exp)
		}
	}
}

// TestAssetUniqueWhileEscrowed asserts an item instance can back at most
// one escrowed listing: the partial UNIQUE(item_instance_id) blocks a
// second listing on the same asset (ADR-0065).
func TestAssetUniqueWhileEscrowed(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 10_000_000)
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
	list := func() error {
		return inTx(t, func(tx pgx.Tx) error {
			_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
				AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 100})
			return err
		})
	}
	if err := list(); err != nil {
		t.Fatalf("first list: %v", err)
	}
	if err := list(); !errors.Is(err, ErrNotEscrowed) && err == nil {
		t.Fatalf("second list err=%v", err)
	}
}

func ctx(t *testing.T) context.Context { return context.Background() }
