package cosmetics

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
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/testing/pgtest"
)

var sharedPool *pgxpool.Pool
var setupErr error

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
	name := fmt.Sprintf("cosmetics_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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
	if cleanup != nil {
		cleanup()
	}
	srv.Close()
	os.Exit(code)
}

func requirePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if setupErr != nil {
		t.Skipf("pg setup: %v", setupErr)
	}
	if sharedPool == nil {
		t.Skip("pg pool unavailable")
	}
	return sharedPool
}

func wipe(t *testing.T) {
	t.Helper()
	pool := requirePool(t)
	for _, table := range []string{
		"character_feat_milestones", "character_feats",
		"character_cosmetic_equips", "character_cosmetic_entitlements",
		"account_cosmetic_entitlements", "account_iap_entitlements",
		"guild_cosmetic_selections", "guild_cosmetic_entitlements",
		"guild_memberships", "guilds",
		"item_locations", "item_instances",
		"character_currencies", "characters", "accounts",
	} {
		if _, err := pool.Exec(context.Background(),
			"DELETE FROM "+table); err != nil {
			t.Fatalf("wipe %s: %v", table, err)
		}
	}
}

// mkCharacter inserts one account + character row and returns both ids.
func mkCharacter(t *testing.T) (id.UUID, id.UUID) {
	t.Helper()
	acct, char := id.NewV4(), id.NewV4()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO accounts (account_id) VALUES ($1)`, acct[:]); err != nil {
		t.Fatalf("account: %v", err)
	}
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key,
		    class_id, created_at)
		 VALUES ($1,$2,$3,$4,'class.kim',now())`,
		char[:], acct[:], "c"+char.String()[:6], "c"+char.String()[:6]); err != nil {
		t.Fatalf("character: %v", err)
	}
	return acct, char
}

// mkIAPGrant inserts one GRANTED account_iap_entitlements row plus the
// matching account_cosmetic_entitlements link, returning the
// entitlement id (IAP grants are IMP-100's write path — tests seed
// the committed shape).
func mkIAPGrant(t *testing.T, acct id.UUID, cosmeticID string) id.UUID {
	t.Helper()
	ent := id.NewV4()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO account_iap_entitlements
		    (entitlement_id, account_id, product_id, entitlement_type,
		     grant_state, platform, platform_receipt, created_at,
		     granted_at)
		 VALUES ($1,$2,'product.cosmetic.test','DIRECT_ACCOUNT_COSMETIC',
		     'GRANTED','STEAM',$3,now(),now())`,
		ent[:], acct[:], ent.String()); err != nil {
		t.Fatalf("iap entitlement: %v", err)
	}
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO account_cosmetic_entitlements
		    (account_id, cosmetic_id, entitlement_id, granted_at)
		 VALUES ($1,$2,$3,now())`, acct[:], cosmeticID, ent[:]); err != nil {
		t.Fatalf("account cosmetic: %v", err)
	}
	return ent
}

// mkIAPRow inserts a bare GRANTED account_iap_entitlements row for FK
// targets (season-track source links).
func mkIAPRow(t *testing.T, acct, ent id.UUID) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO account_iap_entitlements
		    (entitlement_id, account_id, product_id, entitlement_type,
		     grant_state, platform, platform_receipt, created_at,
		     granted_at)
		 VALUES ($1,$2,'product.track.test','ONE_SHOT',
		     'GRANTED','STEAM',$3,now(),now())`,
		ent[:], acct[:], ent.String()); err != nil {
		t.Fatalf("iap row: %v", err)
	}
}

// mkGuild inserts an ACTIVE guild + membership row with the given role.
func mkGuild(t *testing.T, char id.UUID, role string) id.UUID {
	t.Helper()
	gid := id.NewV4()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO guilds (guild_id, name, name_key, state,
		    recruitment_mode, leader_character_id, guild_revision,
		    guild_storage_revision, created_at)
		 VALUES ($1,$2,$3,'ACTIVE','CLOSED',$4,1,1,now())`,
		gid[:], "g"+gid.String()[:6], "g"+gid.String()[:6], char[:]); err != nil {
		t.Fatalf("guild: %v", err)
	}
	mid := id.NewV4()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO guild_memberships
		    (character_id, guild_id, role, joined_at, membership_id)
		 VALUES ($1,$2,$3,now(),$4)`,
		char[:], gid[:], role, mid[:]); err != nil {
		t.Fatalf("membership: %v", err)
	}
	return gid
}

// mkItem grants qty of itemID into the character's inventory as one
// stack.
func mkItem(t *testing.T, char id.UUID, itemID string, qty int64) {
	t.Helper()
	inst := id.NewV4()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO item_instances
		    (item_instance_id, item_id, quantity, effective_binding,
		     created_at)
		 VALUES ($1,$2,$3,'UNBOUND',now())`,
		inst[:], itemID, qty); err != nil {
		t.Fatalf("item: %v", err)
	}
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO item_locations
		    (item_instance_id, location_kind, character_id, slot,
		     updated_at)
		 VALUES ($1,'CHARACTER_INVENTORY',$2,'inv.0',now())`,
		inst[:], char[:]); err != nil {
		t.Fatalf("item loc: %v", err)
	}
}

// mkCurrency seeds a currency balance row.
func mkCurrency(t *testing.T, char id.UUID, currencyID string,
	amount int64) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO character_currencies
		    (character_id, currency_id, balance, revision)
		 VALUES ($1,$2,$3,1)`, char[:], currencyID, amount); err != nil {
		t.Fatalf("currency: %v", err)
	}
}

// txOf runs fn inside a real committed transaction.
func txOf(t *testing.T, fn func(tx pgx.Tx) error) {
	t.Helper()
	tx, err := sharedPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err := fn(tx); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func txErr(fn func(tx pgx.Tx) error) error {
	tx, err := sharedPool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	return fn(tx)
}

// grant seeds one PLAY source row directly.
func grant(t *testing.T, char id.UUID, cosmeticID, sourceRef string) {
	t.Helper()
	txOf(t, func(tx pgx.Tx) error {
		return NewStore(sharedPool).Grant(context.Background(), tx, char,
			cosmeticID, SourcePlay, sourceRef, nil, id.NewV4(),
			time.Now().UTC())
	})
}
