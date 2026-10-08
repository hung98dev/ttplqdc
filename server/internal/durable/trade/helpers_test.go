package trade

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
	"thinhthan/internal/testing/pgtest"
)

var (
	sharedDSN string
	setupErr  error
)

func repoRoot() string {
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
	name := fmt.Sprintf("trade_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot()), "up"); err != nil {
		setupErr = err
	} else {
		sharedDSN = dsn
	}
	code := m.Run()
	cleanup()
	srv.Close()
	os.Exit(code)
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedDSN == "" {
		if setupErr != nil {
			t.Fatalf("postgres provisioning failed: %v", setupErr)
		}
		t.Skip("DEFERRED(local-missing): no postgres")
	}
	pool, err := pgxpool.New(context.Background(), sharedDSN)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// participant is one seeded trade actor.
type participant struct {
	account, char id.UUID
}

// seedParticipant seeds account+character (trade-eligible: level 60,
// created well past 24h) + inventory row + common balance.
func seedParticipant(t *testing.T, pool *pgxpool.Pool, balance int64) participant {
	t.Helper()
	acct, char := id.NewV4(), id.NewV4()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	key := "trdchar" + char.String()[:10]
	if _, err := pool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, created_at)
		 VALUES ($1,$2,$3,$4,'class.kim',60, now() - interval '7 days')`,
		char.String(), acct.String(), key, key); err != nil {
		t.Fatalf("seed character: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO character_inventories (character_id, capacity) VALUES ($1, 60)`,
		char.String()); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO character_currencies (character_id, currency_id, balance)
		 VALUES ($1,'currency.common',$2)`, char.String(), balance); err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	return participant{account: acct, char: char}
}

// seedStack seeds one UNBOUND stack in a character's inventory.
func seedStack(t *testing.T, pool *pgxpool.Pool, char id.UUID, itemID string, qty int, slot string) id.UUID {
	t.Helper()
	iid := id.NewV4()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO item_instances (item_instance_id, item_id, quantity, effective_binding, created_at)
		 VALUES ($1,$2,$3,'UNBOUND', now())`, iid.String(), itemID, qty); err != nil {
		t.Fatalf("seed instance: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO item_locations (item_instance_id, location_kind, character_id, slot, updated_at)
		 VALUES ($1,'CHARACTER_INVENTORY',$2,$3, now())`, iid.String(), char.String(), slot); err != nil {
		t.Fatalf("seed location: %v", err)
	}
	return iid
}

func balanceOf(t *testing.T, pool *pgxpool.Pool, char id.UUID) int64 {
	t.Helper()
	var bal int64
	if err := pool.QueryRow(context.Background(),
		`SELECT balance FROM character_currencies
		 WHERE character_id=$1 AND currency_id='currency.common'`,
		char.String()).Scan(&bal); err != nil {
		t.Fatalf("balance: %v", err)
	}
	return bal
}

func stackQty(t *testing.T, pool *pgxpool.Pool, iid id.UUID) int {
	t.Helper()
	var q int
	if err := pool.QueryRow(context.Background(),
		`SELECT quantity FROM item_instances WHERE item_instance_id=$1`,
		iid.String()).Scan(&q); err != nil {
		t.Fatalf("stack qty: %v", err)
	}
	return q
}

// ownerOf resolves the inventory owner of an instance.
func ownerOf(t *testing.T, pool *pgxpool.Pool, iid id.UUID) (id.UUID, bool) {
	t.Helper()
	var owner string
	err := pool.QueryRow(context.Background(),
		`SELECT character_id::text FROM item_locations
		  WHERE item_instance_id=$1 AND location_kind='CHARACTER_INVENTORY'`,
		iid.String()).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return id.UUID{}, false
	}
	if err != nil {
		t.Fatalf("owner: %v", err)
	}
	c, err := id.ParseUUID(owner)
	if err != nil {
		t.Fatalf("owner parse: %v", err)
	}
	return c, true
}

// journalItem builds the minimal committed snapshot for one offer.
func journalItem(iid id.UUID, itemID string, qty int) *journalv1.JournalItem {
	return &journalv1.JournalItem{
		ItemInstanceId:   iid[:],
		ItemId:           itemID,
		Quantity:         uint64(qty),
		EffectiveBinding: "UNBOUND",
	}
}

// lockedLedger returns a Locks seam where each participant's ledger
// locks exactly the given offers: char → instance → {locked q, stack n}.
func lockedLedger(m map[id.UUID]map[id.UUID][2]int) func(id.UUID) *items.TradeLockLedger {
	return func(charID id.UUID) *items.TradeLockLedger {
		qs, ok := m[charID]
		if !ok {
			return items.NewTradeLockLedger(charID)
		}
		l := items.NewTradeLockLedger(charID)
		for iid, qn := range qs {
			if err := l.Offer(iid, qn[0], qn[1]); err != nil {
				panic(err)
			}
		}
		return l
	}
}

// runSettle drives the settlement inside one committed tx.
func runSettle(t *testing.T, pool *pgxpool.Pool, in SettleIn) (SettleOut, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	out, err := New(pool).Settle(ctx, tx, in)
	if err != nil {
		return SettleOut{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return out, nil
}

func decodeOutcome(t *testing.T, out idempotency.Outcome) *journalv1.JournalOutcome {
	t.Helper()
	o := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, o); err != nil {
		t.Fatalf("decode outcome: %v", err)
	}
	return o
}
