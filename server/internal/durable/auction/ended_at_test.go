package auction

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestEndedAtSetOnEveryTerminalTransition asserts ended_at is stamped on
// every exit from ACTIVE and overwritten on RECLAIMED / MOVED_TO_CLAIM,
// so it always carries the latest state change (ADR-0070).
func TestEndedAtSetOnEveryTerminalTransition(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 10_000_000)

	readEnded := func(lid id.UUID) *time.Time {
		var et *time.Time
		if err := pool(t).QueryRow(context.Background(),
			`SELECT ended_at FROM auction_listings WHERE listing_id=$1`,
			lid.String()).Scan(&et); err != nil {
			t.Fatalf("ended_at: %v", err)
		}
		return et
	}
	readState := func(lid id.UUID) string {
		var st string
		if err := pool(t).QueryRow(context.Background(),
			`SELECT state FROM auction_listings WHERE listing_id=$1`,
			lid.String()).Scan(&st); err != nil {
			t.Fatalf("state: %v", err)
		}
		return st
	}

	// ACTIVE: ended_at NULL; CANCELLED: stamped.
	inst := mkItem(t, seller, "item.potion.hp", 1, 0, "UNBOUND")
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
	if et := readEnded(lid); et != nil {
		t.Fatalf("ACTIVE ended_at %v", et)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Cancel(ctx(t), tx, CancelIn{OperationID: id.NewV4(),
			CharacterID: seller, AccountID: acct, ListingID: lid})
		return err
	}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	cancelledAt := readEnded(lid)
	if cancelledAt == nil {
		t.Fatal("CANCELLED ended_at NULL")
	}

	// RECLAIMED: ended_at overwritten (must advance).
	time.Sleep(5 * time.Millisecond)
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Reclaim(ctx(t), tx, ReclaimIn{OperationID: id.NewV4(),
			CharacterID: seller, AccountID: acct, ListingID: lid})
		return err
	}); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if readState(lid) != string(StateReclaimed) {
		t.Fatalf("state %q", readState(lid))
	}
	reclaimedAt := readEnded(lid)
	if reclaimedAt == nil || !reclaimedAt.After(*cancelledAt) {
		t.Fatalf("RECLAIMED ended_at %v vs %v", reclaimedAt, cancelledAt)
	}

	// SOLD: ended_at stamped.
	inst2 := mkItem(t, seller, "item.potion.hp", 1, 1, "UNBOUND")
	buyerAcct, buyer := eligible(t, 10_000)
	var lid2 id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst2, Quantity: 1, PriceCommon: 100})
		if err != nil {
			return err
		}
		lid2 = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list2: %v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: buyer,
			AccountID: buyerAcct, ListingID: lid2, ExpectedPriceCommon: 100})
		return err
	}); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if et := readEnded(lid2); et == nil {
		t.Fatal("SOLD ended_at NULL")
	}

	// EXPIRED via sweep: ended_at stamped.
	inst3 := mkItem(t, seller, "item.potion.hp", 1, 2, "UNBOUND")
	var lid3 id.UUID
	if err := inTx(t, func(tx pgx.Tx) error {
		l, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst3, Quantity: 1, PriceCommon: 100})
		if err != nil {
			return err
		}
		lid3 = l.ListingID
		return nil
	}); err != nil {
		t.Fatalf("list3: %v", err)
	}
	// Force expiry: backdate expires_at then run the sweep.
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE auction_listings SET expires_at = now() - interval '1h' WHERE listing_id=$1`,
		lid3.String()); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		n, err := s.ExpireSweep(ctx(t), tx, time.Now(), id.UUID{})
		if err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("swept %d", n)
		}
		return nil
	}); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if readState(lid3) != string(StateExpired) {
		t.Fatalf("state %q", readState(lid3))
	}
	if et := readEnded(lid3); et == nil {
		t.Fatal("EXPIRED ended_at NULL")
	}
}
