package inventory

import (
	"context"
	"testing"

	"thinhthan/internal/durable/items"
)

// TestLockedQuantityReported covers ADR-0064: the 433 InventoryPush
// merges the live trade-lock ledger into locked_quantity per slot,
// while LoadState (ledger-free) reports 0.
func TestLockedQuantityReported(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	instA := mkItem(t, char, "item.mat.ore", 10, 0)
	mkItem(t, char, "item.potion.hp", 3, 1)

	s := NewStore(pool(t))
	// No ledger: locked_quantity 0.
	push, err := s.InventoryPush(ctx, char, nil)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(push.Slots) != 2 {
		t.Fatalf("slots %d", len(push.Slots))
	}
	for _, sv := range push.Slots {
		if sv.GetLockedQuantity() != 0 {
			t.Fatalf("slot %d locked %d, want 0", sv.GetSlot(), sv.GetLockedQuantity())
		}
	}

	// Ledger locking 4 of instA's 10: locked_quantity 4 on that slot.
	ledger := items.NewTradeLockLedger(char)
	if err := ledger.Offer(instA, 4, 10); err != nil {
		t.Fatalf("offer: %v", err)
	}
	push, err = s.InventoryPush(ctx, char, ledger.LockedQty)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	var saw bool
	for _, sv := range push.Slots {
		if sv.GetItem() != nil && string(sv.GetItem().GetItemInstanceId()) == string(instA[:]) {
			saw = true
			if sv.GetLockedQuantity() != 4 {
				t.Fatalf("locked %d, want 4", sv.GetLockedQuantity())
			}
		} else if sv.GetLockedQuantity() != 0 {
			t.Fatalf("unlocked slot %d reports %d", sv.GetSlot(), sv.GetLockedQuantity())
		}
	}
	if !saw {
		t.Fatal("locked instance not in push")
	}
}

// TestWalletPushRevision covers the ratified wallet_revision =
// SUM(character_currencies.revision) derivation.
func TestWalletPushRevision(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	mkWallet(t, char, "currency.common", 25000)
	if _, err := pool(t).Exec(ctx,
		`UPDATE character_currencies SET revision=revision+3 WHERE character_id=$1`,
		char.String()); err != nil {
		t.Fatalf("rev: %v", err)
	}
	s := NewStore(pool(t))
	push, err := s.WalletPush(ctx, char)
	if err != nil {
		t.Fatalf("wallet: %v", err)
	}
	if push.GetWalletRevision() != 3 {
		t.Fatalf("wallet_revision %d, want 3", push.GetWalletRevision())
	}
	if len(push.Balances) != 1 || push.Balances[0].GetAmount() != 25000 ||
		push.Balances[0].GetCap() != 2_000_000_000 {
		t.Fatalf("balances %+v", push.Balances)
	}
}

// TestInventoryPushLoadoutRevision covers loadout_revision =
// SUM(character_loadouts.revision) and the fixed 3-loadout projection.
func TestInventoryPushLoadoutRevision(t *testing.T) {
	ctx := context.Background()
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	if _, err := pool(t).Exec(ctx,
		`INSERT INTO character_loadouts (character_id, loadout_index, role, revision)
		 VALUES ($1,1,'ACTIVE',2),($1,2,'SUPPORT',5),($1,3,'SUPPORT',7)`,
		char.String()); err != nil {
		t.Fatalf("loadouts: %v", err)
	}
	s := NewStore(pool(t))
	push, err := s.InventoryPush(ctx, char, nil)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if push.GetLoadoutRevision() != 14 {
		t.Fatalf("loadout_revision %d, want 14", push.GetLoadoutRevision())
	}
	if len(push.Loadouts) != 3 {
		t.Fatalf("loadouts %d, want 3", len(push.Loadouts))
	}
	if push.Loadouts[0].GetLoadoutId() != "loadout.primary" || !push.Loadouts[0].GetIsActive() {
		t.Fatalf("loadout 0 %+v", push.Loadouts[0])
	}
}
