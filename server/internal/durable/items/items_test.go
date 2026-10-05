package items

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/testing/pgtest"
)

var (
	sharedPool *pgxpool.Pool
	setupErr   error
	fixtureN   atomic.Int64
)

func testRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd + "/../../../.."
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	srv, err := pgtest.Ensure(ctx)
	switch {
	case errors.Is(err, pgtest.ErrUnavailable):
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
		os.Exit(m.Run())
	case err != nil:
		setupErr = err
		os.Exit(m.Run())
	}
	name := fmt.Sprintf("items_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(testRepoRoot()), "up"); err != nil {
		setupErr = err
	} else if p, err := pgxpool.New(ctx, dsn); err != nil {
		setupErr = err
	} else {
		sharedPool = p
	}
	code := m.Run()
	if sharedPool != nil {
		sharedPool.Close()
	}
	cleanup()
	srv.Close()
	os.Exit(code)
}

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedPool == nil {
		if setupErr != nil {
			t.Fatalf("postgres provisioning failed: %v", setupErr)
		}
		t.Fatal("postgres pool unavailable")
	}
	return sharedPool
}

// runTx runs fn in a transaction: commit on nil error, rollback otherwise.
// The operation error itself is returned for assertions.
func runTx(t *testing.T, fn func(ctx context.Context, tx pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}

func nextName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, fixtureN.Add(1))
}

func mkAccount(t *testing.T) id.UUID {
	t.Helper()
	acc := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO accounts (account_id) VALUES ($1)`, acc.String()); err != nil {
		t.Fatalf("account: %v", err)
	}
	return acc
}

func mkCharacter(t *testing.T, acct id.UUID) id.UUID {
	t.Helper()
	ch := id.NewV4()
	name := nextName("char")
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1,$2,$3,$3,'class.kim')`, ch.String(), acct.String(), name); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_inventories (character_id, capacity) VALUES ($1, 60)`,
		ch.String()); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	return ch
}

func mkGuild(t *testing.T, leader id.UUID) id.UUID {
	t.Helper()
	g := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO guilds (guild_id, name, name_key, state, recruitment_mode,
		  leader_character_id, guild_revision, guild_storage_revision, created_at)
		 VALUES ($1,$2,$2,'ACTIVE','CLOSED',$3,1,1,NOW())`,
		g.String(), nextName("guild"), leader.String()); err != nil {
		t.Fatalf("guild: %v", err)
	}
	return g
}

func mkBeast(t *testing.T, ch id.UUID, beastID string) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_beasts (character_id, beast_id, level, created_at, updated_at)
		 VALUES ($1,$2,1,NOW(),NOW())`, ch.String(), beastID); err != nil {
		t.Fatalf("beast: %v", err)
	}
}

func mkSoul(t *testing.T, ch, instanceID id.UUID) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_souls
		 (soul_instance_id, character_id, soul_id, level, current_soul_exp,
		  contracted_item_instance_id)
		 VALUES ($1,$2,'soul.test',1,0,$3)`,
		id.NewV4().String(), ch.String(), instanceID.String()); err != nil {
		t.Fatalf("soul: %v", err)
	}
}

func mkListing(t *testing.T, ch, acct, instanceID id.UUID, itemID string, qty int) id.UUID {
	t.Helper()
	l := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO auction_listings
		 (listing_id, seller_character_id, seller_account_id, item_instance_id,
		  item_id, quantity, price_common, listing_fee_common, state,
		  listed_at, expires_at, revision)
		 VALUES ($1,$2,$3,$4,$5,$6,100,10,'ACTIVE',NOW(),NOW()+interval '1 day',1)`,
		l.String(), ch.String(), acct.String(), instanceID.String(), itemID, qty); err != nil {
		t.Fatalf("listing: %v", err)
	}
	return l
}

func newStore(t *testing.T) *Store {
	t.Helper()
	return New(pool(t))
}

func invLoc(ch id.UUID, slot string) Location {
	return Location{Kind: LocCharacterInventory, CharacterID: ch, Slot: slot}
}

func mustCreate(t *testing.T, s *Store, def Def, f CreateFields, loc Location) id.UUID {
	t.Helper()
	var iid id.UUID
	err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		iid, err = s.Create(ctx, tx, def, f, loc)
		return err
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return iid
}

func locationCount(t *testing.T, iid id.UUID) int {
	t.Helper()
	var n int
	if err := pool(t).QueryRow(context.Background(),
		`SELECT COUNT(*) FROM item_locations WHERE item_instance_id=$1`,
		iid.String()).Scan(&n); err != nil {
		t.Fatalf("location count: %v", err)
	}
	return n
}

func locationOf(t *testing.T, s *Store, iid id.UUID) *Location {
	t.Helper()
	l, err := s.Location(context.Background(), iid)
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	return l
}

func unboundEquipDef() Def {
	return Def{ItemID: "item.equip.test", Kind: KindEquipment,
		DefaultBinding: BindingUnbound, BindingTrigger: TriggerNone}
}

func stackDef(itemID string, max int) Def {
	return Def{ItemID: itemID, Kind: KindConsumable, Stackable: true, MaxStack: max,
		DefaultBinding: BindingUnbound, BindingTrigger: TriggerNone}
}

// TestItemInstanceCreation — items.md § Item Instance: creation commits the
// instance with a resolved binding and its exactly-one location row
// atomically.
func TestItemInstanceCreation(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	ch := mkCharacter(t, acct)

	def := stackDef("item.mat.test", 9999)
	iid := mustCreate(t, s, def, CreateFields{Quantity: 5}, invLoc(ch, "inv.1"))

	inst, err := s.Get(context.Background(), iid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if inst.ItemID != def.ItemID || inst.Quantity != 5 ||
		inst.EffectiveBinding != BindingUnbound || inst.EnhancementLevel != 0 {
		t.Fatalf("instance row wrong: %+v", inst)
	}
	loc := locationOf(t, s, iid)
	if loc.Kind != LocCharacterInventory || loc.CharacterID != ch || loc.Slot != "inv.1" {
		t.Fatalf("location row wrong: %+v", loc)
	}
	if n := locationCount(t, iid); n != 1 {
		t.Fatalf("expected exactly 1 location row, got %d", n)
	}

	// Unknown id surfaces the sentinel.
	if _, err := s.Get(context.Background(), id.NewV4()); !errors.Is(err, ErrUnknownItem) {
		t.Fatalf("expected ErrUnknownItem, got %v", err)
	}
	// Quantity must respect the stack cap.
	err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.Create(ctx, tx, def, CreateFields{Quantity: 10000}, invLoc(ch, "inv.2"))
		return err
	})
	if !errors.Is(err, ErrQuantity) {
		t.Fatalf("expected ErrQuantity, got %v", err)
	}
	// Occupied slot rejects a second instance.
	err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.Create(ctx, tx, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		return err
	})
	if !errors.Is(err, ErrSlotOccupied) {
		t.Fatalf("expected ErrSlotOccupied, got %v", err)
	}
}

// TestSingleItemLocationConstraint — items.md § Ownership Context: every live
// instance keeps exactly one location row across custody transitions; direct
// inventory->inventory moves serialize on the row locks.
func TestSingleItemLocationConstraint(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	def := unboundEquipDef()

	t.Run("equip keeps single location", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Equip(ctx, tx, iid, def, ch, "slot.weapon", nil)
		}); err != nil {
			t.Fatalf("equip: %v", err)
		}
		loc := locationOf(t, s, iid)
		if n := locationCount(t, iid); loc.Kind != LocEquipped || loc.Slot != "slot.weapon" || n != 1 {
			t.Fatalf("equip location wrong: %+v rows=%d", loc, n)
		}
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Unequip(ctx, tx, iid, "inv.2", nil)
		}); err != nil {
			t.Fatalf("unequip: %v", err)
		}
		loc = locationOf(t, s, iid)
		if n := locationCount(t, iid); loc.Kind != LocCharacterInventory || loc.Slot != "inv.2" || n != 1 {
			t.Fatalf("unequip location wrong: %+v rows=%d", loc, n)
		}
	})

	t.Run("equip occupied slot is rejected", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		a := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		b := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.2"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Equip(ctx, tx, a, def, ch, "slot.weapon", nil)
		}); err != nil {
			t.Fatalf("first equip: %v", err)
		}
		err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Equip(ctx, tx, b, def, ch, "slot.weapon", nil)
		})
		if !errors.Is(err, ErrSlotOccupied) {
			t.Fatalf("expected ErrSlotOccupied, got %v", err)
		}
		if loc := locationOf(t, s, b); loc.Kind != LocCharacterInventory {
			t.Fatalf("loser moved: %+v", loc)
		}
	})

	t.Run("concurrent custody moves serialize", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		chB := mkCharacter(t, mkAccount(t))
		iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		errs := make(chan error, 2)
		for _, dst := range []Location{invLoc(chB, "inv.1"), invLoc(chB, "inv.2")} {
			go func(dst Location) {
				ctx := context.Background()
				tx, err := pool(t).Begin(ctx)
				if err != nil {
					errs <- err
					return
				}
				defer func() { _ = tx.Rollback(ctx) }()
				if err := s.Move(ctx, tx, iid, dst, nil); err != nil {
					errs <- err
					return
				}
				errs <- tx.Commit(ctx)
			}(dst)
		}
		e1, e2 := <-errs, <-errs
		// Whatever the serialization, custody must stay exactly-one-location
		// and land at one of the targets.
		if n := locationCount(t, iid); n != 1 {
			t.Fatalf("location rows=%d after race", n)
		}
		loc := locationOf(t, s, iid)
		if loc.Kind != LocCharacterInventory || loc.CharacterID != chB ||
			(loc.Slot != "inv.1" && loc.Slot != "inv.2") {
			t.Fatalf("post-race location wrong: %+v (errs %v %v)", loc, e1, e2)
		}
		if e1 != nil && e2 != nil {
			t.Fatalf("both moves failed: %v %v", e1, e2)
		}
	})

	t.Run("beast equipment dual-table atomicity", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		bdef := Def{ItemID: "item.beast.test", Kind: KindBeastEquipment,
			DefaultBinding: BindingUnbound, BindingTrigger: TriggerNone}
		mkBeast(t, ch, "beast.test.a")
		iid := mustCreate(t, s, bdef, CreateFields{Quantity: 1}, invLoc(ch, "inv.3"))

		// Wrong kind cannot enter a beast slot.
		reg := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.4"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.EquipBeast(ctx, tx, reg, def, ch, "beast.test.a", "vong_co", nil)
		}); !errors.Is(err, ErrInvalidLocation) {
			t.Fatalf("expected ErrInvalidLocation kind, got %v", err)
		}
		// Unknown beast.
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.EquipBeast(ctx, tx, iid, bdef, ch, "beast.missing", "vong_co", nil)
		}); !errors.Is(err, ErrInvalidLocation) {
			t.Fatalf("expected ErrInvalidLocation beast, got %v", err)
		}
		// Unknown slot.
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.EquipBeast(ctx, tx, iid, bdef, ch, "beast.test.a", "tail", nil)
		}); !errors.Is(err, ErrInvalidLocation) {
			t.Fatalf("expected ErrInvalidLocation slot, got %v", err)
		}

		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.EquipBeast(ctx, tx, iid, bdef, ch, "beast.test.a", "vong_co", nil)
		}); err != nil {
			t.Fatalf("equip-beast: %v", err)
		}
		var bel int
		if err := pool(t).QueryRow(context.Background(),
			`SELECT COUNT(*) FROM beast_equipment_locations WHERE item_instance_id=$1`,
			iid.String()).Scan(&bel); err != nil || bel != 1 {
			t.Fatalf("beast_equipment_locations rows=%d err=%v", bel, err)
		}
		loc := locationOf(t, s, iid)
		if n := locationCount(t, iid); loc.Kind != LocBeastEquipmentSlot || n != 1 {
			t.Fatalf("beast slot location wrong: %+v rows=%d", loc, n)
		}
		// Occupied beast slot rejects.
		iid2 := mustCreate(t, s, bdef, CreateFields{Quantity: 1}, invLoc(ch, "inv.5"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.EquipBeast(ctx, tx, iid2, bdef, ch, "beast.test.a", "vong_co", nil)
		}); !errors.Is(err, ErrSlotOccupied) {
			t.Fatalf("expected ErrSlotOccupied, got %v", err)
		}
		// Unequip removes the second-table row and returns to inventory.
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.UnequipBeast(ctx, tx, iid, "inv.6", nil)
		}); err != nil {
			t.Fatalf("unequip-beast: %v", err)
		}
		if err := pool(t).QueryRow(context.Background(),
			`SELECT COUNT(*) FROM beast_equipment_locations WHERE item_instance_id=$1`,
			iid.String()).Scan(&bel); err != nil || bel != 0 {
			t.Fatalf("beast_equipment_locations after unequip=%d err=%v", bel, err)
		}
		if loc := locationOf(t, s, iid); loc.Kind != LocCharacterInventory || loc.Slot != "inv.6" {
			t.Fatalf("unequip-beast location wrong: %+v", loc)
		}
	})
}

// TestCharacterBoundOwnership — items.md § Binding + § Account-Bound: the
// committed binding blocks custody transfer paths (character change, guild
// storage, auction escrow, direct trade) while preserving account-scoped
// use; laundering via source override or binding writes cannot loosen.
func TestCharacterBoundOwnership(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	ch := mkCharacter(t, acct)
	chSameAcct := mkCharacter(t, acct)
	acctB := mkAccount(t)
	chB := mkCharacter(t, acctB)
	g := mkGuild(t, ch)

	cdef := Def{ItemID: "item.bound.test", Kind: KindEquipment,
		DefaultBinding: BindingCharacterBound, BindingTrigger: TriggerNone}
	abdef := Def{ItemID: "item.acct.test", Kind: KindMaterial, Stackable: true, MaxStack: 99,
		DefaultBinding: BindingAccountBound, BindingTrigger: TriggerNone}

	blocked := func(name string, def Def) {
		t.Helper()
		ch := mkCharacter(t, acct)
		iid := mustCreate(t, s, def, CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		assertBlockedMoves := func() {
			t.Helper()
			cases := []struct {
				name string
				run  func(ctx context.Context, tx pgx.Tx) error
			}{
				{"same-account character move", func(ctx context.Context, tx pgx.Tx) error {
					return s.Move(ctx, tx, iid, invLoc(chSameAcct, "inv.1"), nil)
				}},
				{"cross-account character move", func(ctx context.Context, tx pgx.Tx) error {
					return s.Move(ctx, tx, iid, invLoc(chB, "inv.1"), nil)
				}},
				{"guild storage deposit", func(ctx context.Context, tx pgx.Tx) error {
					return s.GuildDeposit(ctx, tx, iid, g, "sec.1", nil)
				}},
				{"auction escrow", func(ctx context.Context, tx pgx.Tx) error {
					listing := mkListing(t, ch, acct, iid, def.ItemID, 1)
					return s.EscrowForListing(ctx, tx, iid, listing, nil)
				}},
			}
			for _, c := range cases {
				if err := runTx(t, c.run); !errors.Is(err, ErrBindingBlocksTransfer) {
					t.Fatalf("%s %s: expected ErrBindingBlocksTransfer, got %v", name, c.name, err)
				}
			}
			// Direct trade: COMMITTING re-validation rejects non-UNBOUND.
			led := NewTradeLockLedger(ch)
			if err := led.Offer(iid, 1, 1); err != nil {
				t.Fatalf("offer: %v", err)
			}
			if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
				return led.RevalidateOffers(ctx, tx)
			}); !errors.Is(err, ErrBindingBlocksTransfer) {
				t.Fatalf("%s trade commit: expected ErrBindingBlocksTransfer, got %v", name, err)
			}
		}
		assertBlockedMoves()
	}
	blocked("CHARACTER_BOUND", cdef)
	blocked("ACCOUNT_BOUND", abdef)

	t.Run("UNBOUND cross-character move allowed", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		iid := mustCreate(t, s, unboundEquipDef(), CreateFields{Quantity: 1}, invLoc(ch, "inv.1"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.Move(ctx, tx, iid, invLoc(chB, "inv.1"), nil)
		}); err != nil {
			t.Fatalf("unbound move: %v", err)
		}
		if loc := locationOf(t, s, iid); loc.CharacterID != chB {
			t.Fatalf("move landed wrong: %+v", loc)
		}
	})

	t.Run("binding laundering impossible", func(t *testing.T) {
		ch := mkCharacter(t, acct)
		// Source override may only tighten: UNBOUND def + CHARACTER_BOUND
		// source commits CHARACTER_BOUND.
		bind := BindingCharacterBound
		iid := mustCreate(t, s, unboundEquipDef(),
			CreateFields{Quantity: 1, SourceBinding: &bind}, invLoc(ch, "inv.1"))
		inst, err := s.Get(context.Background(), iid)
		if err != nil || inst.EffectiveBinding != BindingCharacterBound {
			t.Fatalf("source override not committed: %+v err=%v", inst, err)
		}
		// Loosening at creation is rejected.
		loose := BindingUnbound
		err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			_, err := s.Create(ctx, tx, cdef, CreateFields{Quantity: 1, SourceBinding: &loose},
				invLoc(ch, "inv.2"))
			return err
		})
		if !errors.Is(err, ErrBindingOverride) {
			t.Fatalf("expected ErrBindingOverride on loosening create, got %v", err)
		}
		// SetEffectiveBinding tightens only.
		err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.SetEffectiveBinding(ctx, tx, iid, BindingAccountBound)
		})
		if !errors.Is(err, ErrBindingOverride) {
			t.Fatalf("expected ErrBindingOverride on loosen, got %v", err)
		}
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.SetEffectiveBinding(ctx, tx, iid, BindingCharacterBound)
		}); err != nil {
			t.Fatalf("idempotent tighten: %v", err)
		}
		// And an UNBOUND instance can be tightened up.
		u := mustCreate(t, s, unboundEquipDef(), CreateFields{Quantity: 1}, invLoc(ch, "inv.2"))
		if err := runTx(t, func(ctx context.Context, tx pgx.Tx) error {
			return s.SetEffectiveBinding(ctx, tx, u, BindingAccountBound)
		}); err != nil {
			t.Fatalf("tighten: %v", err)
		}
		inst, _ = s.Get(context.Background(), u)
		if inst.EffectiveBinding != BindingAccountBound {
			t.Fatalf("tighten not committed: %+v", inst)
		}
	})
}

// TestAccountScopedAccess — items.md § Account-Bound: ACCOUNT_BOUND items may
// be viewed and used by any character of the owning account, never moved;
// CHARACTER_BOUND restricts use to the owner character. ListForAccount is
// the view grant.
func TestAccountScopedAccess(t *testing.T) {
	s := newStore(t)
	acct := mkAccount(t)
	ch := mkCharacter(t, acct)
	chSameAcct := mkCharacter(t, acct)
	chB := mkCharacter(t, mkAccount(t))
	ctx := context.Background()

	abdef := Def{ItemID: "item.acct.use", Kind: KindConsumable, Stackable: true, MaxStack: 99,
		DefaultBinding: BindingAccountBound, BindingTrigger: TriggerNone}
	ab := mustCreate(t, s, abdef, CreateFields{Quantity: 3}, invLoc(ch, "inv.1"))

	// Owner and same-account characters may use it.
	if err := s.AssertUsable(ctx, ab, acct, ch); err != nil {
		t.Fatalf("owner usable: %v", err)
	}
	if err := s.AssertUsable(ctx, ab, acct, chSameAcct); err != nil {
		t.Fatalf("same-account usable: %v", err)
	}
	// Another account is forbidden.
	if err := s.AssertUsable(ctx, ab, mkAccount(t), chB); !errors.Is(err, ErrForbiddenOwner) {
		t.Fatalf("expected ErrForbiddenOwner cross-account, got %v", err)
	}

	cdef := Def{ItemID: "item.char.use", Kind: KindConsumable,
		DefaultBinding: BindingCharacterBound, BindingTrigger: TriggerNone}
	cb := mustCreate(t, s, cdef, CreateFields{Quantity: 1}, invLoc(ch, "inv.2"))
	if err := s.AssertUsable(ctx, cb, acct, ch); err != nil {
		t.Fatalf("char-bound owner usable: %v", err)
	}
	// CHARACTER_BOUND excludes even same-account alts.
	if err := s.AssertUsable(ctx, cb, acct, chSameAcct); !errors.Is(err, ErrForbiddenOwner) {
		t.Fatalf("expected ErrForbiddenOwner same-account char-bound, got %v", err)
	}

	// The view grant lists both instances for the account.
	list, err := s.ListForAccount(ctx, acct)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[id.UUID]bool{}
	for _, inst := range list {
		got[inst.InstanceID] = true
	}
	if !got[ab] || !got[cb] {
		t.Fatalf("ListForAccount missing owned items: %v", got)
	}
	// And custody is unaffected by viewing.
	if loc := locationOf(t, s, ab); loc.Kind != LocCharacterInventory || loc.CharacterID != ch {
		t.Fatalf("view moved item: %+v", loc)
	}
}
