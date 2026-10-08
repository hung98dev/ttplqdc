package equipment

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
	name := fmt.Sprintf("equipment_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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
	name := fmt.Sprintf("eqp_%d", time.Now().UnixNano()%1_000_000_000)
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level)
		 VALUES ($1,$2,$3,$4,'class.kim',10)`, char.String(), accountID.String(), name, name); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_inventories (character_id, capacity, revision)
		 VALUES ($1, 60, 0)`, char.String()); err != nil {
		t.Fatalf("inventory row: %v", err)
	}
	return char
}

// mkItem inserts one instance at inventory slot inv.<n>.
func mkItem(t *testing.T, charID id.UUID, itemID string, slot int) id.UUID {
	t.Helper()
	inst := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_instances
		  (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		   item_state, content_revision, created_at)
		 VALUES ($1,$2,1,'UNBOUND',0,'{}'::jsonb,NULL,NOW())`,
		inst.String(), itemID); err != nil {
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

// mkSoul contracts a soul row onto an equipped item instance.
func mkSoul(t *testing.T, charID, itemInstID id.UUID) id.UUID {
	t.Helper()
	soul := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_souls
		  (soul_instance_id, character_id, soul_id, level, current_soul_exp,
		   contracted_item_instance_id)
		 VALUES ($1,$2,'soul.test',1,0,$3)`,
		soul.String(), charID.String(), itemInstID.String()); err != nil {
		t.Fatalf("soul: %v", err)
	}
	return soul
}

// testDefs resolves the fixed test equipment catalog: one item per
// canonical slot plus wrong-kind fixtures.
func testDefs() Defs {
	mk := func(itemID, slotID string, lvlMin int) EquipDef {
		return EquipDef{
			Def:    items.Def{ItemID: itemID, Kind: items.KindEquipment},
			SlotID: slotID, LevelMin: lvlMin, EquipmentItem: true,
		}
	}
	return func(_ context.Context, itemID string) (EquipDef, error) {
		switch itemID {
		case "item.eq.weapon":
			return mk(itemID, "weapon", 1), nil
		case "item.eq.head":
			return mk(itemID, "head", 1), nil
		case "item.eq.head.l20":
			return mk(itemID, "head", 20), nil
		case "item.consumable.potion":
			return EquipDef{Def: items.Def{ItemID: itemID, Kind: items.KindConsumable}}, nil
		default:
			return EquipDef{}, fmt.Errorf("unknown item %s", itemID)
		}
	}
}

func newOp() id.UUID { return id.NewV7(time.Now()) }

// runChange submits one C2SLoadoutChange through TrustedReplay.
func runChange(t *testing.T, d Deps, idem *idempotency.Store,
	accountID, charID id.UUID, req *protocolv1.C2SLoadoutChange) *protocolv1.S2CLoadoutResult {
	t.Helper()
	ctx := context.Background()
	rec, err := Record(accountID, 1, 1, charID, req, nil, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	out, err := idem.TrustedReplay(ctx, Family,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: charID}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return d.executor(ctx, tx, rec)
		})
	if err != nil {
		t.Fatalf("trusted replay: %v", err)
	}
	o := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, o); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	res := o.GetS2CLoadoutResult()
	if res == nil {
		t.Fatalf("outcome missing S2CLoadoutResult")
	}
	return res
}

func equipReq(kind protocolv1.LoadoutChangeKind, oneof func(*protocolv1.C2SLoadoutChange)) *protocolv1.C2SLoadoutChange {
	op := newOp()
	r := &protocolv1.C2SLoadoutChange{OperationId: op[:], Kind: kind}
	oneof(r)
	return r
}

// locationOf reads the item's location row.
func locationOf(t *testing.T, inst id.UUID) (kind, charID, slot string) {
	t.Helper()
	err := pool(t).QueryRow(context.Background(),
		`SELECT location_kind, COALESCE(character_id::text,''), COALESCE(slot,'')
		 FROM item_locations WHERE item_instance_id = $1`, inst.String()).Scan(&kind, &charID, &slot)
	if err != nil {
		t.Fatalf("locationOf: %v", err)
	}
	return kind, charID, slot
}
