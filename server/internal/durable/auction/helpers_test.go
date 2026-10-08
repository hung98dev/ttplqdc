package auction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
	"thinhthan/internal/durable/schema"
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
	name := fmt.Sprintf("auction_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

// mkCharacter inserts a character at the given level created `age` ago;
// pass level>=15 and age>=24h for an auction-eligible character.
func mkCharacter(t *testing.T, accountID id.UUID, level int, age time.Duration) id.UUID {
	t.Helper()
	char := id.NewV4()
	name := fmt.Sprintf("ah_%d", time.Now().UnixNano()%1_000_000_000)
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, created_at)
		 VALUES ($1,$2,$3,$4,'class.kim',$5,$6)`,
		char.String(), accountID.String(), name, name, level,
		time.Now().Add(-age)); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_inventories (character_id, capacity) VALUES ($1,60)`,
		char.String()); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	return char
}

// mkItem inserts one instance into the character's inventory slot.
func mkItem(t *testing.T, charID id.UUID, itemID string, qty int32, slot int,
	binding string) id.UUID {
	t.Helper()
	inst := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_instances
		  (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		   item_state, content_revision, created_at)
		 VALUES ($1,$2,$3,$4,0,'{}'::jsonb,NULL,NOW())`,
		inst.String(), itemID, qty, binding); err != nil {
		t.Fatalf("instance: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_locations (item_instance_id, location_kind, character_id, slot, updated_at)
		 VALUES ($1,'CHARACTER_INVENTORY',$2,$3,NOW())`,
		inst.String(), charID.String(), fmt.Sprintf("inv.%d", slot)); err != nil {
		t.Fatalf("location: %v", err)
	}
	return inst
}

func mkWallet(t *testing.T, charID id.UUID, amount int64) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance, revision)
		 VALUES ($1,'currency.common',$2,0)`, charID.String(), amount); err != nil {
		t.Fatalf("wallet: %v", err)
	}
}

func wallet(t *testing.T, charID id.UUID) int64 {
	t.Helper()
	var b int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT balance FROM character_currencies WHERE character_id=$1 AND currency_id='currency.common'`,
		charID.String()).Scan(&b); err != nil {
		t.Fatalf("wallet read: %v", err)
	}
	return b
}

// testLookup resolves the fixed auction test catalog: one generic
// consumable (no NPC value), one NPC-valued material, and a T3 weapon.
func testLookup() Lookup {
	catalog := map[string]*LookupItem{
		"item.potion.hp":   {ItemID: "item.potion.hp", Category: "CONSUMABLE", Tradable: true},
		"item.mat.ore":     {ItemID: "item.mat.ore", Category: "MATERIAL", NPCBaseBuyPrice: 250, Tradable: true},
		"item.weapon.t3":   {ItemID: "item.weapon.t3", Category: "EQUIPMENT", Tier: 3, Tradable: true},
		"item.locked":      {ItemID: "item.locked", Category: "CONSUMABLE", Tradable: false},
		"item.search.only": {ItemID: "item.search.only", Category: "SEARCH", Tradable: true},
	}
	return func(_ context.Context, itemID string) (*LookupItem, error) {
		if l, ok := catalog[itemID]; ok {
			return l, nil
		}
		return nil, fmt.Errorf("no lookup %q", itemID)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	return New(pool(t), items.New(pool(t)), testLookup())
}

// eligible is a level-20 character older than 24 h with a funded wallet.
func eligible(t *testing.T, balance int64) (id.UUID, id.UUID) {
	t.Helper()
	acct := mkAccount(t)
	char := mkCharacter(t, acct, 20, 48*time.Hour)
	mkWallet(t, char, balance)
	return acct, char
}

// inTx runs fn inside a transaction, commits on nil error and returns
// fn's error for assertion (a rejection rolls the whole tx back).
func inTx(t *testing.T, fn func(tx pgx.Tx) error) error {
	t.Helper()
	tx, err := pool(t).Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	err = fn(tx)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return err
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}
