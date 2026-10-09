package fishing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/reward"
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
	name := fmt.Sprintf("fishing_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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
	name := fmt.Sprintf("fk_%d", time.Now().UnixNano()%1_000_000_000)
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1,$2,$3,$4,'class.kim')`, char.String(), accountID.String(), name, name); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_inventories (character_id, capacity)
		 VALUES ($1, 60)`, char.String()); err != nil {
		t.Fatalf("inventory row: %v", err)
	}
	return char
}

// mkItem inserts one owned stack at inv.<slot>.
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
		inst.String(), charID.String(), fmt.Sprintf("inv.%d", slot)); err != nil {
		t.Fatalf("location: %v", err)
	}
	return inst
}

func qty(t *testing.T, charID id.UUID, itemID string) int64 {
	t.Helper()
	var n int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT COALESCE(SUM(ii.quantity),0) FROM item_instances ii
		 JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		 WHERE il.character_id=$1 AND il.location_kind='CHARACTER_INVENTORY' AND ii.item_id=$2`,
		charID.String(), itemID).Scan(&n); err != nil {
		t.Fatalf("qty: %v", err)
	}
	return n
}

func expOf(t *testing.T, charID id.UUID) int64 {
	t.Helper()
	var n int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT current_exp FROM characters WHERE character_id=$1`, charID.String()).Scan(&n); err != nil {
		t.Fatalf("exp: %v", err)
	}
	return n
}

func catchState(t *testing.T, charID id.UUID) (string, int) {
	t.Helper()
	var day *string
	var n int
	if err := pool(t).QueryRow(context.Background(),
		`SELECT fishing_utc_date::text, fishing_catch_count FROM characters
		 WHERE character_id=$1`, charID.String()).Scan(&day, &n); err != nil {
		t.Fatalf("catch state: %v", err)
	}
	if day == nil {
		return "", n
	}
	return *day, n
}

// testDefs resolves every authored catch/tool id to a stackable def.
func testDefs() Defs {
	catalog := map[string]ItemDef{}
	stack := func(itemID string) {
		catalog[itemID] = ItemDef{Def: items.Def{
			ItemID: itemID, Kind: items.KindMaterial, Stackable: true, MaxStack: 99}}
	}
	for _, r := range defaultTable {
		stack(r.ItemID)
	}
	for _, s := range seasonalExtras {
		stack(s.extraID)
	}
	stack(RodItemID)
	stack(BaitItemID)
	return func(_ context.Context, itemID string) (ItemDef, error) {
		d, ok := catalog[itemID]
		if !ok {
			return ItemDef{}, fmt.Errorf("defs: unknown %s", itemID)
		}
		return d, nil
	}
}

func newDeps(t *testing.T) Deps {
	t.Helper()
	return Deps{
		Items:   items.New(pool(t)),
		Rewards: reward.NewStore(pool(t)),
		Prog:    progression.NewStore(pool(t)),
		Defs:    testDefs(),
		ContentRevision: strPtr(
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
		Now: func() time.Time { return time.Now().UTC() },
	}
}

func newOp() id.UUID { return id.NewV7(time.Now()) }

// interactRecord builds a committed-shape fishing client record.
func interactRecord(t *testing.T, family string, charID, opID id.UUID,
	kind protocolv1.InteractKind, targetID string,
) *journalv1.DurableCommandRecord {
	t.Helper()
	req := &protocolv1.C2SInteract{
		InteractKind: kind,
		TargetId:     targetID,
		OperationId:  opID[:],
	}
	rec, err := InteractRecord(id.NewV4(), charID, 1, 2, req, time.Now().UTC())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.GetOperationFamily() != family {
		t.Fatalf("family %q want %q", rec.GetOperationFamily(), family)
	}
	return rec
}

// insertReceipt commits one ADMITTED receipt row — the durable cast
// sequence is the per-day ordinal of these rows.
func insertReceipt(t *testing.T, family string, charID, opID id.UUID,
	admittedAt time.Time) {
	t.Helper()
	fpr := make([]byte, 32)
	copy(fpr, opID[:])
	if _, err := pool(t).Exec(context.Background(), `
		INSERT INTO durable_command_receipts
		  (operation_family, owner_kind, owner_id, operation_id,
		   request_fingerprint, admitted_at, issued_at, replay_until, state)
		VALUES ($1,'CHARACTER',$2,$3,$4,$5::timestamptz,$5::timestamptz,$5::timestamptz + INTERVAL '180 days','ADMITTED')`,
		family, charID.String(), opID.String(), fpr, admittedAt); err != nil {
		t.Fatalf("receipt: %v", err)
	}
}

// outcomeOf decodes the executor outcome payload.
func outcomeOf(t *testing.T, o idempotency.Outcome) *journalv1.JournalOutcome {
	t.Helper()
	out := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(o.Payload, out); err != nil {
		t.Fatalf("outcome: %v", err)
	}
	return out
}

func strPtr(s string) *string { return &s }
