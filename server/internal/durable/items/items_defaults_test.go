package items

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// TestTradeLockBlocksMoveUseDiscard — items.md § Trade Lock: while a stack is
// locked for a session offer, every custody op and any reduction below the
// locked quantity is INVALID_STATE; the free remainder stays
// usable/sellable/discardable/splittable; every end state releases only.
func TestTradeLockBlocksMoveUseDiscard(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	ch := mkCharacter(t, acct)
	chB := mkCharacter(t, acct)
	g := mkGuild(t, ch)

	def := stackDef("item.mat.lock", 9999)
	iid := mustCreate(t, s, def, CreateFields{Quantity: 5}, invLoc(ch, "inv.1"))
	led := NewTradeLockLedger(ch)
	if err := led.Offer(iid, 2, 5); err != nil {
		t.Fatalf("offer: %v", err)
	}
	if led.LockedQty(iid) != 2 {
		t.Fatalf("locked qty %d", led.LockedQty(iid))
	}
	// Offering the same stack twice is INVALID_STATE; q outside 1..n too.
	if err := led.Offer(iid, 1, 5); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("re-offer: expected ErrTradeLocked, got %v", err)
	}
	if err := led.Offer(iid, 0, 5); !errors.Is(err, ErrQuantity) {
		t.Fatalf("zero offer: expected ErrQuantity, got %v", err)
	}
	if err := led.Offer(iid, 6, 5); !errors.Is(err, ErrQuantity) {
		t.Fatalf("oversized offer: expected ErrQuantity, got %v", err)
	}

	// Every custody unit op on the locked stack is INVALID_STATE.
	listing := mkListing(t, ch, acct, iid, def.ItemID, 5)
	for name, run := range map[string]func(ctx context.Context, tx pgx.Tx) error{
		"move":          func(ctx context.Context, tx pgx.Tx) error { return s.Move(ctx, tx, iid, invLoc(chB, "inv.1"), led) },
		"unequip":       func(ctx context.Context, tx pgx.Tx) error { return s.Unequip(ctx, tx, iid, "inv.2", led) },
		"unequip beast": func(ctx context.Context, tx pgx.Tx) error { return s.UnequipBeast(ctx, tx, iid, "inv.2", led) },
		"guild deposit": func(ctx context.Context, tx pgx.Tx) error { return s.GuildDeposit(ctx, tx, iid, g, "sec.1", led) },
		"escrow":        func(ctx context.Context, tx pgx.Tx) error { return s.EscrowForListing(ctx, tx, iid, listing, led) },
		"discard":       func(ctx context.Context, tx pgx.Tx) error { return s.Discard(ctx, tx, iid, def, led) },
	} {
		if err := runTx(t, run); !errors.Is(err, ErrTradeLocked) {
			t.Fatalf("locked %s: expected ErrTradeLocked, got %v", name, err)
		}
	}
	// Equip on a locked equipment instance is INVALID_STATE too.
	eqDef := unboundEquipDef()
	eq := mustCreate(t, s, eqDef, CreateFields{Quantity: 1}, invLoc(ch, "inv.9"))
	if err := led.Offer(eq, 1, 1); err != nil {
		t.Fatalf("offer equip: %v", err)
	}
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.Equip(ctx, tx, eq, eqDef, ch, "slot.weapon", led)
	}); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("locked equip: expected ErrTradeLocked, got %v", err)
	}
	bdef := Def{ItemID: "item.beast.lock", Kind: KindBeastEquipment,
		DefaultBinding: BindingUnbound, BindingTrigger: TriggerNone}
	bi := mustCreate(t, s, bdef, CreateFields{Quantity: 1}, invLoc(ch, "inv.8"))
	mkBeast(t, ch, "beast.test.lock")
	if err := led.Offer(bi, 1, 1); err != nil {
		t.Fatalf("offer beast item: %v", err)
	}
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.EquipBeast(ctx, tx, bi, bdef, ch, "beast.test.lock", "vong_co", led)
	}); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("locked beast-equip: expected ErrTradeLocked, got %v", err)
	}

	// Reduction is limited to the free remainder (5 - 2 = 3).
	if err := led.AssertOpAllowed(iid, 5, OpReduce, 3); err != nil {
		t.Fatalf("free-remainder reduce: %v", err)
	}
	if err := led.AssertOpAllowed(iid, 5, OpReduce, 4); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("below-locked reduce: expected ErrTradeLocked, got %v", err)
	}
	if err := led.AssertOpAllowed(iid, 5, OpMergeInto, 1); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("merge-into: expected ErrTradeLocked, got %v", err)
	}

	// The free remainder splits; the locked part stays.
	var newID id.UUID
	err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		newID, err = s.SplitStack(ctx, tx, iid, 3, "inv.2", led)
		return err
	})
	if err != nil {
		t.Fatalf("split free remainder: %v", err)
	}
	if loc := locationOf(t, s, newID); loc.Kind != LocCharacterInventory || loc.Slot != "inv.2" {
		t.Fatalf("split location wrong: %+v", loc)
	}
	inst, _ := s.Get(context.Background(), iid)
	if inst.Quantity != 2 {
		t.Fatalf("locked stack qty=%d after split", inst.Quantity)
	}
	// Splitting below the locked quantity is rejected.
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.SplitStack(ctx, tx, iid, 1, "inv.3", led)
		return err
	}); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("split below locked: expected ErrTradeLocked, got %v", err)
	}
	// Merge into the locked stack is rejected outright.
	other := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.4"))
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.MergeStacks(ctx, tx, iid, other, def, led)
	}); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("merge-into-locked: expected ErrTradeLocked, got %v", err)
	}
	// Merge out of the locked stack may only take the free remainder...
	// (locked stack is at exactly locked qty now, so any merge-out fails).
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.MergeStacks(ctx, tx, other, iid, def, led)
	}); !errors.Is(err, ErrTradeLocked) {
		t.Fatalf("merge-out-of-locked: expected ErrTradeLocked, got %v", err)
	}

	// COMMITTING revalidation passes on the intact offer.
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return led.RevalidateOffers(ctx, tx)
	}); err != nil {
		t.Fatalf("revalidate intact offer: %v", err)
	}
	// Any non-committing end releases only; nothing moves.
	led.Cancel()
	if led.LockedQty(iid) != 0 {
		t.Fatal("cancel did not release")
	}
	if loc := locationOf(t, s, iid); loc.Kind != LocCharacterInventory || loc.CharacterID != ch {
		t.Fatalf("cancel moved item: %+v", loc)
	}
	// After release the stack moves normally.
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.Move(ctx, tx, iid, invLoc(chB, "inv.1"), led)
	}); err != nil {
		t.Fatalf("post-release move: %v", err)
	}
}

// TestDefinitionDefaultsDiscardAllowed — items.md § Definition Defaults:
// QUEST and CHARACTER_BOUND books are protected; every other item defaults
// discardable; an explicit field overrides the derived default.
func TestDefinitionDefaultsDiscardAllowed(t *testing.T) {
	tr, fa := true, false
	for _, c := range []struct {
		name string
		def  Def
		want bool
	}{
		{"quest protected", Def{ItemID: "item.quest.x", Kind: KindQuest}, false},
		{"char-bound book protected",
			Def{ItemID: "item.book.potential_strength", Kind: KindConsumable,
				DefaultBinding: BindingCharacterBound}, false},
		{"ordinary item", Def{ItemID: "item.mat.wood", Kind: KindMaterial}, true},
		{"explicit quest override",
			Def{ItemID: "item.quest.y", Kind: KindQuest, DiscardAllowed: &tr}, true},
		{"explicit book override",
			Def{ItemID: "item.book.x", Kind: KindConsumable,
				DefaultBinding: BindingCharacterBound, DiscardAllowed: &tr}, true},
		{"explicit false override",
			Def{ItemID: "item.mat.x", Kind: KindMaterial, DiscardAllowed: &fa}, false},
		{"bound non-book discardable",
			Def{ItemID: "item.equip.sword", Kind: KindEquipment,
				DefaultBinding: BindingCharacterBound}, true},
		{"unbound book discardable",
			Def{ItemID: "item.book.x", Kind: KindConsumable,
				DefaultBinding: BindingUnbound}, true},
	} {
		got := *ApplyDefinitionDefaults(c.def).DiscardAllowed
		if got != c.want {
			t.Fatalf("%s: discard_allowed=%v want %v", c.name, got, c.want)
		}
	}
	// The discardable flag actually gates Discard.
	s := newStore(t)
	acct := mkAccount(t)
	ch := mkCharacter(t, acct)
	qdef := Def{ItemID: "item.quest.x", Kind: KindQuest, DefaultBinding: BindingUnbound}
	iid := mustCreate(t, s, qdef, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.Discard(ctx, tx, iid, qdef, nil)
	}); !errors.Is(err, ErrNotDiscardable) {
		t.Fatalf("quest discard: expected ErrNotDiscardable, got %v", err)
	}
}

// TestSharedCooldownGroups — items.md § Definition Defaults + Consumables:
// default group is NONE; the four shared groups carry their seconds.
func TestSharedCooldownGroups(t *testing.T) {
	for _, c := range []struct {
		g    CooldownGroup
		want int
	}{
		{CooldownNone, 0}, {CooldownHP, 8}, {CooldownMP, 8},
		{CooldownBuff, 5}, {CooldownFood, 1},
	} {
		if got := CooldownSeconds(c.g); got != c.want {
			t.Fatalf("cooldown %s = %d want %d", c.g, got, c.want)
		}
	}
	d := ApplyDefinitionDefaults(Def{ItemID: "x"})
	if d.SharedCooldownGroup != CooldownNone {
		t.Fatalf("default cooldown group = %s", d.SharedCooldownGroup)
	}
	d = ApplyDefinitionDefaults(Def{ItemID: "x", SharedCooldownGroup: CooldownFood})
	if d.SharedCooldownGroup != CooldownFood {
		t.Fatalf("explicit group lost: %s", d.SharedCooldownGroup)
	}
}

// TestDiscardFullGuard exercises the remaining § Transfer/Use/Discard guard:
// discard from EQUIPPED / AUCTION_ESCROW, soul-contracted protection, and the
// atomic instance+location delete.
func TestDiscardFullGuard(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	def := unboundEquipDef()

	t.Run("equipped item not discardable", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Equip(ctx, tx, iid, def, ch, "slot.weapon", nil)
		}); err != nil {
			t.Fatalf("equip: %v", err)
		}
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Discard(ctx, tx, iid, def, nil)
		}); !errors.Is(err, ErrNotDiscardable) {
			t.Fatalf("expected ErrNotDiscardable, got %v", err)
		}
	})

	t.Run("escrowed item not discardable", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		listing := mkListing(t, ch, acct, iid, def.ItemID, 1)
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.EscrowForListing(ctx, tx, iid, listing, nil)
		}); err != nil {
			t.Fatalf("escrow: %v", err)
		}
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Discard(ctx, tx, iid, def, nil)
		}); !errors.Is(err, ErrNotDiscardable) {
			t.Fatalf("expected ErrNotDiscardable, got %v", err)
		}
		// Reclaim returns it to the seller inventory.
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.ReclaimEscrow(ctx, tx, iid, "inv.9", nil)
		}); err != nil {
			t.Fatalf("reclaim: %v", err)
		}
		if loc := locationOf(t, s, iid); loc.Kind != LocCharacterInventory || loc.CharacterID != ch {
			t.Fatalf("reclaim landed wrong: %+v", loc)
		}
	})

	t.Run("soul contracted protected in same tx", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		mkSoul(t, ch, iid)
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Discard(ctx, tx, iid, def, nil)
		}); !errors.Is(err, ErrSoulContracted) {
			t.Fatalf("expected ErrSoulContracted, got %v", err)
		}
		// Escrow is also barred while contracted.
		listing := mkListing(t, ch, acct, iid, def.ItemID, 1)
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.EscrowForListing(ctx, tx, iid, listing, nil)
		}); !errors.Is(err, ErrSoulContracted) {
			t.Fatalf("contracted escrow: expected ErrSoulContracted, got %v", err)
		}
	})

	t.Run("destroy commits both rows", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Discard(ctx, tx, iid, def, nil)
		}); err != nil {
			t.Fatalf("discard: %v", err)
		}
		if _, err := s.Get(context.Background(), iid); !errors.Is(err, ErrUnknownItem) {
			t.Fatalf("instance still present: %v", err)
		}
		if n := locationCount(t, iid); n != 0 {
			t.Fatalf("location rows left: %d", n)
		}
	})
}

// TestStackMergeSplit — items.md § Stack Rules: compatible stacks merge up to
// the cap with overflow residual; incompatible pairs never merge; split
// carves exact quantities and binding is preserved.
func TestStackMergeSplit(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	def := stackDef("item.mat.merge", 10)

	t.Run("merge to cap leaves residual", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		a := mustCreate(t, s, def, CreateFields{Quantity: 7}, invLoc(ch, "inv.1"))
		b := mustCreate(t, s, def, CreateFields{Quantity: 6}, invLoc(ch, "inv.2"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.MergeStacks(ctx, tx, a, b, def, nil)
		}); err != nil {
			t.Fatalf("merge: %v", err)
		}
		ia, _ := s.Get(context.Background(), a)
		ib, _ := s.Get(context.Background(), b)
		if ia.Quantity != 10 || ib.Quantity != 3 {
			t.Fatalf("merge result %d+%d", ia.Quantity, ib.Quantity)
		}
	})

	t.Run("full merge deletes source row", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		a := mustCreate(t, s, def, CreateFields{Quantity: 4}, invLoc(ch, "inv.1"))
		b := mustCreate(t, s, def, CreateFields{Quantity: 4}, invLoc(ch, "inv.2"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.MergeStacks(ctx, tx, a, b, def, nil)
		}); err != nil {
			t.Fatalf("merge: %v", err)
		}
		if ia, _ := s.Get(context.Background(), a); ia.Quantity != 8 {
			t.Fatalf("dst qty %d", ia.Quantity)
		}
		if _, err := s.Get(context.Background(), b); !errors.Is(err, ErrUnknownItem) {
			t.Fatalf("src still present: %v", err)
		}
		if n := locationCount(t, b); n != 0 {
			t.Fatalf("src location rows left: %d", n)
		}
	})

	t.Run("incompatible stacks never merge", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		a := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		// Different item_id.
		otherDef := stackDef("item.mat.other", 10)
		b := mustCreate(t, s, otherDef, CreateFields{Quantity: 1}, invLoc(ch, "inv.2"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.MergeStacks(ctx, tx, a, b, def, nil)
		}); !errors.Is(err, ErrIncompatibleStack) {
			t.Fatalf("item_id mismatch: expected ErrIncompatibleStack, got %v", err)
		}
		// Different committed binding.
		bind := BindingAccountBound
		c := mustCreate(t, s, def, CreateFields{Quantity: 1, SourceBinding: &bind}, invLoc(ch, "inv.3"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.MergeStacks(ctx, tx, a, c, def, nil)
		}); !errors.Is(err, ErrIncompatibleStack) {
			t.Fatalf("binding mismatch: expected ErrIncompatibleStack, got %v", err)
		}
		// Different item_state.
		d := mustCreate(t, s, def, CreateFields{Quantity: 1, ItemState: []byte(`{"x":1}`)}, invLoc(ch, "inv.4"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.MergeStacks(ctx, tx, a, d, def, nil)
		}); !errors.Is(err, ErrIncompatibleStack) {
			t.Fatalf("state mismatch: expected ErrIncompatibleStack, got %v", err)
		}
		// Self-merge.
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.MergeStacks(ctx, tx, a, a, def, nil)
		}); !errors.Is(err, ErrIncompatibleStack) {
			t.Fatalf("self merge: expected ErrIncompatibleStack, got %v", err)
		}
	})

	t.Run("split preserves binding and slot rules", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		bind := BindingAccountBound
		src := mustCreate(t, s, def, CreateFields{Quantity: 9, SourceBinding: &bind}, invLoc(ch, "inv.1"))
		var newID id.UUID
		err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			var err error
			newID, err = s.SplitStack(ctx, tx, src, 4, "inv.2", nil)
			return err
		})
		if err != nil {
			t.Fatalf("split: %v", err)
		}
		ni, _ := s.Get(context.Background(), newID)
		si, _ := s.Get(context.Background(), src)
		if ni.Quantity != 4 || si.Quantity != 5 || ni.EffectiveBinding != BindingAccountBound {
			t.Fatalf("split result src=%+v new=%+v", si, ni)
		}
		if loc := locationOf(t, s, newID); loc.Slot != "inv.2" || loc.CharacterID != ch {
			t.Fatalf("split location: %+v", loc)
		}
		// Occupied slot rejects.
		err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			_, err := s.SplitStack(ctx, tx, src, 1, "inv.2", nil)
			return err
		})
		if !errors.Is(err, ErrSlotOccupied) {
			t.Fatalf("occupied split slot: expected ErrSlotOccupied, got %v", err)
		}
		// qty >= stack rejects (a whole-stack move is Move, not Split).
		err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			_, err := s.SplitStack(ctx, tx, src, 5, "inv.3", nil)
			return err
		})
		if !errors.Is(err, ErrQuantity) {
			t.Fatalf("full split: expected ErrQuantity, got %v", err)
		}
	})
}

// TestGuildStorageCycle — deposit/withdraw roundtrip keeps custody clean and
// depositor identity on the row (ADR-0049 signal).
func TestGuildStorageCycle(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	ch := mkCharacter(t, acct)
	g := mkGuild(t, ch)
	def := unboundEquipDef()

	iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.GuildDeposit(ctx, tx, iid, g, "sec.1", nil)
	}); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	loc := locationOf(t, s, iid)
	if loc.Kind != LocGuildStorage || loc.GuildID != g ||
		loc.DepositorCharacterID != ch || loc.DepositorAccountID != acct {
		t.Fatalf("guild location wrong: %+v", loc)
	}
	// Occupied guild slot rejects.
	iid2 := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.2"))
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.GuildDeposit(ctx, tx, iid2, g, "sec.1", nil)
	}); !errors.Is(err, ErrSlotOccupied) {
		t.Fatalf("occupied guild slot: expected ErrSlotOccupied, got %v", err)
	}
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.GuildWithdraw(ctx, tx, iid, ch, "inv.3", nil)
	}); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if loc := locationOf(t, s, iid); loc.Kind != LocCharacterInventory || loc.CharacterID != ch || loc.Slot != "inv.3" {
		t.Fatalf("withdraw landed wrong: %+v", loc)
	}
	// Deposit to a nonexistent guild fails.
	if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return s.GuildDeposit(ctx, tx, iid2, id.NewV4(), "sec.9", nil)
	}); !errors.Is(err, ErrInvalidLocation) {
		t.Fatalf("unknown guild: expected ErrInvalidLocation, got %v", err)
	}
}
