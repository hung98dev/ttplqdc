package crafting

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

	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
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
	name := fmt.Sprintf("crafting_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

func mkCharacter(t *testing.T, accountID id.UUID, level int32) id.UUID {
	t.Helper()
	char := id.NewV4()
	name := fmt.Sprintf("cf_%d", time.Now().UnixNano()%1_000_000_000)
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level)
		 VALUES ($1,$2,$3,$4,'class.kim',$5)`,
		char.String(), accountID.String(), name, name, level); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_inventories (character_id, capacity)
		 VALUES ($1, 60)`, char.String()); err != nil {
		t.Fatalf("inventory row: %v", err)
	}
	return char
}

// credit grants common currency directly (bypasses audit — fixture).
func credit(t *testing.T, charID id.UUID, amount int64) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance, revision)
		 VALUES ($1,'currency.common',$2,0)
		 ON CONFLICT (character_id, currency_id)
		 DO UPDATE SET balance = character_currencies.balance + $2`,
		charID.String(), amount); err != nil {
		t.Fatalf("credit: %v", err)
	}
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

// mkEquip inserts one owned equipment instance at +enh.
func mkEquip(t *testing.T, charID id.UUID, itemID string, enh int16, slot uint32) id.UUID {
	t.Helper()
	inst := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_instances
		  (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		   item_state, content_revision, created_at)
		 VALUES ($1,$2,1,'UNBOUND',$3,'{}'::jsonb,NULL,NOW())`,
		inst.String(), itemID, enh); err != nil {
		t.Fatalf("equip: %v", err)
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

func invCount(t *testing.T, charID id.UUID) int {
	t.Helper()
	var n int
	if err := pool(t).QueryRow(context.Background(),
		`SELECT COUNT(*) FROM item_locations
		 WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY'`,
		charID.String()).Scan(&n); err != nil {
		t.Fatalf("inv count: %v", err)
	}
	return n
}

func balance(t *testing.T, charID id.UUID) int64 {
	t.Helper()
	var n int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT COALESCE(balance,0) FROM character_currencies
		 WHERE character_id=$1 AND currency_id='currency.common'`,
		charID.String()).Scan(&n); err != nil {
		return 0
	}
	return n
}

func enhOf(t *testing.T, instID id.UUID) int16 {
	t.Helper()
	var n int16
	if err := pool(t).QueryRow(context.Background(),
		`SELECT enhancement_level FROM item_instances WHERE item_instance_id=$1`,
		instID.String()).Scan(&n); err != nil {
		t.Fatalf("enh: %v", err)
	}
	return n
}

func pityOf(t *testing.T, instID id.UUID, target int64) int64 {
	t.Helper()
	var n int64
	err := pool(t).QueryRow(context.Background(),
		`SELECT pity_fail_count FROM enhancement_pity
		 WHERE item_instance_id=$1 AND target_level=$2`,
		instID.String(), target).Scan(&n)
	if err == pgx.ErrNoRows {
		return 0
	}
	if err != nil {
		t.Fatalf("pity: %v", err)
	}
	return n
}

// testCatalog builds the full authored expansion: 12 sets x 14 slots
// = 168 equipment items + a roll pool and per-tier bases.
func testCatalog() *equipment.Catalog {
	sets := map[string][]string{
		"t1": {"dinh_lang", "ben_da"},
		"t2": {"u_minh", "dom_lua_rung"},
		"t3": {"ben_nuoc", "xom_chim"},
		"t4": {"deo_may", "dau_ho"},
		"t5": {"thanh_co", "trong_tran"},
		"t6": {"nui_thieng", "dau_cu"},
	}
	slots := []string{"weapon", "head", "body", "hands", "legs", "feet",
		"necklace", "ring", "costume", "talisman", "jade", "seal", "relic", "charm"}
	enh := map[string][2]int64{
		"t1": {1, 10}, "t2": {2, 30}, "t3": {3, 75},
		"t4": {4, 150}, "t5": {5, 250}, "t6": {6, 250},
	}
	cat := &equipment.Catalog{
		Items:   map[string]*equipment.ItemDef{},
		Rolls:   map[string]*equipment.RollDef{},
		Budgets: map[string]*equipment.TierBudget{},
		EnhBase: map[string]*equipment.EnhancementBase{},
	}
	for tier, keys := range sets {
		for _, key := range keys {
			for i, slot := range slots {
				itemID := fmt.Sprintf("item.eq.%s.%s.%s", tier, key, slot)
				cat.Items[itemID] = &equipment.ItemDef{
					ID:         itemID,
					Kind:       "equipment",
					Tier:       tier,
					SetKey:     key,
					Slot:       slot,
					Binding:    "UNBOUND",
					StackLimit: 1,
					FixedStats: []equipment.StatTerm{
						{Stat: "ATTACK", Flat: 10 + int64(i)},
						{Stat: "CRIT_CHANCE", Fraction: config.Rat{Num: 1, Den: 20}},
					},
					Enhanceable:    []string{"ATTACK"},
					SecondaryRolls: 2,
				}
				cat.ItemIDs = append(cat.ItemIDs, itemID)
			}
		}
		e := enh[tier]
		cat.EnhBase[tier] = &equipment.EnhancementBase{
			Tier: tier, MaterialUnits: e[0], CommonCurrency: e[1],
		}
		cat.Budgets[tier] = &equipment.TierBudget{
			Tier: tier, UnitA: 100, UnitD: 50, UnitH: 500, UnitM: 200,
		}
	}
	for _, r := range []struct {
		id, stat, kind string
	}{
		{"roll.attack_flat", "ATTACK", "FLAT"},
		{"roll.max_hp_flat", "MAX_HP", "FLAT"},
		{"roll.crit_chance", "CRIT_CHANCE", "UTILITY"},
	} {
		rd := &equipment.RollDef{
			ID: r.id, Stat: r.stat, Kind: r.kind,
			TierRanges: map[string][2]config.Rat{},
		}
		for tier := range sets {
			rd.TierRanges[tier] = [2]config.Rat{
				{Num: 1, Den: 10}, {Num: 2, Den: 10},
			}
		}
		cat.Rolls[r.id] = rd
	}
	cat.FlatRanges = []*equipment.FlatRollRange{
		{Stat: "ATTACK", Unit: "A", Lo: config.Rat{Num: 1, Den: 10}, Hi: config.Rat{Num: 2, Den: 10}},
		{Stat: "MAX_HP", Unit: "H", Lo: config.Rat{Num: 1, Den: 10}, Hi: config.Rat{Num: 2, Den: 10}},
	}
	return cat
}

// testDefs resolves the test catalog: regional materials + consumable
// charms stackable; equipment outputs non-stackable.
func testDefs() Defs {
	catalog := map[string]ItemDef{}
	stack := func(itemID string) {
		catalog[itemID] = ItemDef{Def: items.Def{
			ItemID: itemID, Kind: items.KindMaterial, Stackable: true, MaxStack: 99}}
	}
	for _, t := range tierTable {
		stack(t.MaterialID)
	}
	for _, kind := range []string{charmKindLucky, charmKindInsure} {
		for _, g := range []string{luckyGradeLow, luckyGradeMid, luckyGradeHigh, luckyGradeSuper} {
			stack(kind + "." + g)
		}
	}
	for _, it := range testCatalog().Items {
		catalog[it.ID] = ItemDef{Def: items.Def{
			ItemID: it.ID, Kind: items.KindEquipment, Stackable: false, MaxStack: 1,
			DefaultBinding: items.BindingUnbound}}
	}
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
		Defs:    testDefs(),
		Recipes: RecipesFromCatalog(testCatalog()),
		Equip:   testCatalog(),
		ContentRevision: strPtr(
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
	}
}

// tx runs fn in one committed test transaction.
func tx(t *testing.T, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	txx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer txx.Rollback(ctx)
	fn(ctx, txx)
	if err := txx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func newOp() id.UUID { return id.NewV7(time.Now()) }

func strPtr(s string) *string { return &s }

// craftRecord builds a committed-shape 404 client record.
func craftRecord(t *testing.T, charID, opID id.UUID,
	recipeID string, batch uint32, snap *journalv1.JournalCraftSnapshot,
) *journalv1.DurableCommandRecord {
	t.Helper()
	req := &protocolv1.C2SCraft{
		OperationId:   opID[:],
		NpcId:         "npc.test.tho_nghe",
		RecipeId:      recipeID,
		BatchQuantity: batch,
	}
	rec, err := CraftRecord(id.NewV4(), charID, 1, 1, req, snap, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	return rec
}

// enhanceRecord builds a committed-shape 406 client record.
func enhanceRecord(t *testing.T, charID, opID, itemID id.UUID,
	target uint32, lucky, insurance id.UUID,
	res *journalv1.JournalEnhanceResult,
) *journalv1.DurableCommandRecord {
	t.Helper()
	req := &protocolv1.C2SEnhance{
		OperationId:    opID[:],
		NpcId:          "npc.test.tho_nghe",
		ItemInstanceId: itemID[:],
		TargetLevel:    target,
	}
	if lucky != (id.UUID{}) {
		req.LuckyCharmItemInstanceId = lucky[:]
	}
	if insurance != (id.UUID{}) {
		req.InsuranceItemInstanceId = insurance[:]
	}
	rec, err := EnhanceRecord(id.NewV4(), charID, 1, 1, req, res, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	return rec
}

// outcomeOf parses the committed outcome payload.
func outcomeOf(t *testing.T, oc idempotency.Outcome) *journalv1.JournalOutcome {
	t.Helper()
	out := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(oc.Payload, out); err != nil {
		t.Fatalf("outcome: %v", err)
	}
	return out
}
