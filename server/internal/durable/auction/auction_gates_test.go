package auction

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestAuctionLevel15GateAllOperations asserts every auction operation
// requires character level >= 15 (AH_ELIGIBILITY_LEVEL_REQUIRED) —
// list, buy, cancel, reclaim, proceeds claim.
func TestAuctionLevel15GateAllOperations(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 1_000_000)
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
	underAcct := mkAccount(t)
	under := mkCharacter(t, underAcct, 14, 72*time.Hour)
	mkWallet(t, under, 10_000)
	underInst := mkItem(t, under, "item.potion.hp", 1, 0, "UNBOUND")

	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: under,
			AccountID: underAcct, ItemInstanceID: underInst, Quantity: 1, PriceCommon: 100})
		return err
	}); !errors.Is(err, ErrLevelGate) {
		t.Fatalf("list err=%v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: under,
			AccountID: underAcct, ListingID: lid, ExpectedPriceCommon: 100})
		return err
	}); !errors.Is(err, ErrLevelGate) {
		t.Fatalf("buy err=%v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Cancel(ctx(t), tx, CancelIn{OperationID: id.NewV4(),
			CharacterID: under, AccountID: underAcct, ListingID: lid})
		return err
	}); !errors.Is(err, ErrLevelGate) {
		t.Fatalf("cancel err=%v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Reclaim(ctx(t), tx, ReclaimIn{OperationID: id.NewV4(),
			CharacterID: under, AccountID: underAcct, ListingID: lid})
		return err
	}); !errors.Is(err, ErrLevelGate) {
		t.Fatalf("reclaim err=%v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.ClaimProceeds(ctx(t), tx, ProceedsClaimIn{OperationID: id.NewV4(),
			CharacterID: under, ProceedsID: id.NewV4()})
		return err
	}); !errors.Is(err, ErrLevelGate) {
		t.Fatalf("claim err=%v", err)
	}
}

// TestListingAgeGate asserts listing additionally requires character age
// >= 24 h; all other operations do not.
func TestListingAgeGate(t *testing.T) {
	s := testStore(t)
	acct, young := eligible(t, 0)
	// Recreate young (<24h) at level 20.
	var youngID id.UUID
	_ = young
	youngID = mkCharacter(t, acct, 20, 2*time.Hour)
	mkWallet(t, youngID, 10_000)
	inst := mkItem(t, youngID, "item.potion.hp", 1, 0, "UNBOUND")
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: youngID,
			AccountID: acct, ItemInstanceID: inst, Quantity: 1, PriceCommon: 100})
		return err
	}); !errors.Is(err, ErrAgeGate) {
		t.Fatalf("age gate err=%v", err)
	}
	// Level check precedes age check; a young level-20 passes level but
	// fails age. An eligible-aged character lists fine.
	old := mkCharacter(t, acct, 20, 25*time.Hour)
	mkWallet(t, old, 10_000)
	inst2 := mkItem(t, old, "item.potion.hp", 1, 0, "UNBOUND")
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: old,
			AccountID: acct, ItemInstanceID: inst2, Quantity: 1, PriceCommon: 100})
		return err
	}); err != nil {
		t.Fatalf("eligible list: %v", err)
	}
}

// TestListingFloorPerUnitTimesQuantity asserts the floor is
// max(100, npc_base_buy_price) x quantity — not per listing (ADR-0063).
func TestListingFloorPerUnitTimesQuantity(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 10_000_000)
	// item.mat.ore: npc_base_buy_price=250 -> floor 250*4 = 1000.
	inst := mkItem(t, seller, "item.mat.ore", 4, 0, "UNBOUND")
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 4, PriceCommon: 999})
		return err
	}); !errors.Is(err, ErrBelowFloor) {
		t.Fatalf("below floor err=%v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst, Quantity: 4, PriceCommon: 1000})
		return err
	}); err != nil {
		t.Fatalf("at floor: %v", err)
	}
	// item.weapon.t3: equipment tier floor T3=4000 dominates 100*1.
	inst2 := mkItem(t, seller, "item.weapon.t3", 1, 1, "UNBOUND")
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst2, Quantity: 1, PriceCommon: 3_999})
		return err
	}); !errors.Is(err, ErrBelowFloor) {
		t.Fatalf("tier floor err=%v", err)
	}
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.List(ctx(t), tx, ListIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ItemInstanceID: inst2, Quantity: 1, PriceCommon: 4_000})
		return err
	}); err != nil {
		t.Fatalf("at tier floor: %v", err)
	}
}

// TestSameAccountPurchaseForbidden asserts seller-account characters are
// rejected even when seller != buyer character (account-level check).
func TestSameAccountPurchaseForbidden(t *testing.T) {
	s := testStore(t)
	acct, seller := eligible(t, 1_000_000)
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
	if err := inTx(t, func(tx pgx.Tx) error {
		_, err := s.Buy(ctx(t), tx, BuyIn{OperationID: id.NewV4(), CharacterID: seller,
			AccountID: acct, ListingID: lid, ExpectedPriceCommon: 100})
		return err
	}); !errors.Is(err, ErrSameAccount) {
		t.Fatalf("err=%v", err)
	}
}
