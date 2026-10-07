package inventory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/schema"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

var (
	sharedPool *pgxpool.Pool
	setupErr   error
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
	name := fmt.Sprintf("inventory_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

func mkAccount(t *testing.T) id.UUID {
	t.Helper()
	acct := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("account: %v", err)
	}
	return acct
}

func mkCharacter(t *testing.T, accountID id.UUID) id.UUID {
	t.Helper()
	char := id.NewV4()
	name := fmt.Sprintf("inv_%d", time.Now().UnixNano()%1_000_000_000)
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1,$2,$3,$4,'class.kim')`, char.String(), accountID.String(), name, name); err != nil {
		t.Fatalf("character: %v", err)
	}
	return char
}

// mkItem inserts one instance into the character's inventory on wire
// slot n (item_locations.slot = 'inv.<n>').
func mkItem(t *testing.T, charID id.UUID, itemID string, qty int32, slot uint32) id.UUID {
	t.Helper()
	inst := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_instances
		  (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		   item_state, content_revision, created_at)
		 VALUES ($1,$2,$3,'UNBOUND',0,'{}'::jsonb,NULL,NOW())`,
		inst.String(), itemID, qty); err != nil {
		t.Fatalf("instance: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_locations (item_instance_id, location_kind, character_id, slot, updated_at)
		 VALUES ($1,'CHARACTER_INVENTORY',$2,$3,NOW())`,
		inst.String(), charID.String(), InvSlot(slot)); err != nil {
		t.Fatalf("location: %v", err)
	}
	return inst
}

func mkWallet(t *testing.T, charID id.UUID, currencyID string, amount int64) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance, revision)
		 VALUES ($1,$2,$3,0)`, charID.String(), currencyID, amount); err != nil {
		t.Fatalf("wallet: %v", err)
	}
}

// testDefs resolves the fixed test catalog.
func testDefs() Defs {
	catalog := map[string]ItemDef{
		"item.potion.hp": {Def: items.Def{
			ItemID: "item.potion.hp", Kind: items.KindConsumable, Stackable: true,
			MaxStack: 99, SharedCooldownGroup: items.CooldownHP}, Rarity: 1, Use: UseConsumable},
		"item.potion.mp": {Def: items.Def{
			ItemID: "item.potion.mp", Kind: items.KindConsumable, Stackable: true,
			MaxStack: 99, SharedCooldownGroup: items.CooldownMP}, Rarity: 1, Use: UseConsumable},
		"item.food.basic": {Def: items.Def{
			ItemID: "item.food.basic", Kind: items.KindConsumable, Stackable: true,
			MaxStack: 50, SharedCooldownGroup: items.CooldownFood}, Rarity: 1, Use: UseConsumable},
		"item.book.potential": {Def: items.Def{
			ItemID: "item.book.potential", Kind: items.KindConsumable, Stackable: true,
			MaxStack: 99, DefaultBinding: items.BindingCharacterBound}, Rarity: 3,
			Use: UseBookPotential},
		"item.book.skill": {Def: items.Def{
			ItemID: "item.book.skill", Kind: items.KindConsumable, Stackable: true,
			MaxStack: 99, DefaultBinding: items.BindingCharacterBound}, Rarity: 3,
			Use: UseBookSkill},
		"item.mat.ore": {Def: items.Def{
			ItemID: "item.mat.ore", Kind: items.KindMaterial, Stackable: true,
			MaxStack: 9999}, Rarity: 0},
		"item.weapon.sword": {Def: items.Def{
			ItemID: "item.weapon.sword", Kind: items.KindEquipment, Stackable: false},
			Rarity: 5},
	}
	return func(_ context.Context, itemID string) (ItemDef, error) {
		if d, ok := catalog[itemID]; ok {
			return d, nil
		}
		return ItemDef{}, fmt.Errorf("no def %q", itemID)
	}
}

func testDeps(t *testing.T) Deps {
	t.Helper()
	return Deps{
		Items:     items.New(pool(t)),
		Cooldowns: NewCooldownTracker(nil),
		Defs:      testDefs(),
	}
}

// runMutate drives the real durable path: build the edge record, then
// TrustedReplay through the mutate executor (receipt + commit +
// retained outcome).
func runMutate(t *testing.T, d Deps, idem *idempotency.Store,
	accountID, charID id.UUID, opID id.UUID, op protocolv1.InventoryOp,
	inst id.UUID, toSlot, qty uint32, spatialTick *uint64) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	req := &protocolv1.C2SInventoryMutate{
		OperationId: opID[:],
		Op:          op,
		ToSlot:      toSlot,
		Quantity:    qty,
	}
	if inst != (id.UUID{}) {
		req.ItemInstanceId = inst[:]
	}
	var spatial *journalv1.JournalSource
	if spatialTick != nil {
		spatial = &journalv1.JournalSource{Tick: *spatialTick}
	}
	rec, err := MutateRecord(accountID, 1, 1, charID, req, spatial, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	out, err := idem.TrustedReplay(ctx, MutateFamily,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: charID}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return d.mutateExecutor(ctx, tx, rec)
		})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return decodeOutcome(t, out)
}

func runExpand(t *testing.T, d Deps, idem *idempotency.Store,
	accountID, charID id.UUID, opID id.UUID, expected uint32) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	req := &protocolv1.C2SInventoryExpand{OperationId: opID[:], ExpectedCapacity: expected}
	rec, err := ExpandRecord(accountID, 1, 1, charID, req, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	out, err := idem.TrustedReplay(ctx, ExpandFamily,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: charID}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return d.expandExecutor(ctx, tx, rec)
		})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return decodeOutcome(t, out)
}

func decodeOutcome(t *testing.T, out idempotency.Outcome) *journalv1.JournalOutcome {
	t.Helper()
	o := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, o); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	return o
}
