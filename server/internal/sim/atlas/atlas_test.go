package atlas_test

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
	durableatlas "thinhthan/internal/durable/atlas"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/schema"
	protocolv1 "thinhthan/internal/protocol/v1"
	simatlas "thinhthan/internal/sim/atlas"
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
	name := fmt.Sprintf("atlas_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

func seed(t *testing.T, pool *pgxpool.Pool) id.UUID {
	t.Helper()
	acct, char := id.NewV4(), id.NewV4()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	key := "atlchar" + char.String()[:10]
	if _, err := pool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, current_exp)
		 VALUES ($1,$2,$3,$4,'class.kim',1,0)`,
		char.String(), acct.String(), key, key); err != nil {
		t.Fatalf("seed character: %v", err)
	}
	return char
}

func deps(pool *pgxpool.Pool) durableatlas.Deps {
	return durableatlas.Deps{
		Store:    durableatlas.NewStore(pool),
		Progress: progression.NewStore(pool, progression.WithClock(time.Now)),
		Now:      time.Now,
	}
}

// settleRec builds a sim.atlas_settlement record with one atlas_progress
// entry — what sim/atlas's emission queues for one source event.
func settleRec(char, op id.UUID, pageID string, delta uint64) *journalv1.DurableCommandRecord {
	return &journalv1.DurableCommandRecord{
		OperationFamily: durableatlas.SettlementFamily,
		OwnerId:         char[:],
		OperationId:     op[:],
		Command: &journalv1.DurableCommandRecord_Reward{
			Reward: &journalv1.JournalRewardCommand{
				Kind: durableatlas.RewardKind,
				Slots: []*journalv1.JournalRewardSlot{
					{AtlasProgress: []*journalv1.JournalAtlas{
						{PageId: pageID, Delta: delta},
					}},
				},
			},
		},
	}
}

func ackRec(char, op id.UUID, pageID string, tier uint32) *journalv1.DurableCommandRecord {
	return &journalv1.DurableCommandRecord{
		OperationFamily: durableatlas.AcknowledgeFamily,
		OwnerId:         char[:],
		OperationId:     op[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				Request: &journalv1.JournalClientCommand_C2SAtlasClaim{
					C2SAtlasClaim: &protocolv1.C2SAtlasClaim{
						OperationId: op[:],
						AtlasPageId: pageID,
						Tier:        tier,
					},
				},
			},
		},
	}
}

func runInTx(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error)) *journalv1.JournalOutcome {
	t.Helper()
	var out idempotency.Outcome
	err := pgx.BeginTxFunc(context.Background(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		out, err = fn(context.Background(), tx)
		return err
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	o := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, o); err != nil {
		t.Fatalf("decode outcome: %v", err)
	}
	return o
}

func atlasRow(t *testing.T, pool *pgxpool.Pool, char id.UUID, pageID string) durableatlas.PageRow {
	t.Helper()
	var row durableatlas.PageRow
	err := pgx.BeginTxFunc(context.Background(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		row, err = durableatlas.NewStore(pool).Page(context.Background(), tx, char, pageID)
		return err
	})
	if err != nil {
		t.Fatalf("page row: %v", err)
	}
	return row
}

func specialBalance(t *testing.T, pool *pgxpool.Pool, char id.UUID) int64 {
	t.Helper()
	var bal int64
	err := pool.QueryRow(context.Background(),
		`SELECT balance FROM character_currencies
		 WHERE character_id=$1 AND currency_id='currency.special'`,
		char.String()).Scan(&bal)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0
	}
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return bal
}

func expOf(t *testing.T, pool *pgxpool.Pool, char id.UUID) int64 {
	t.Helper()
	var exp int64
	if err := pool.QueryRow(context.Background(),
		`SELECT current_exp FROM characters WHERE character_id=$1`,
		char.String()).Scan(&exp); err != nil {
		t.Fatalf("exp: %v", err)
	}
	return exp
}

func revisionOf(t *testing.T, pool *pgxpool.Pool, char id.UUID) int64 {
	t.Helper()
	var rev int64
	err := pool.QueryRow(context.Background(),
		`SELECT revision FROM character_atlas_state WHERE character_id=$1`, char.String()).Scan(&rev)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0
	}
	if err != nil {
		t.Fatalf("revision: %v", err)
	}
	return rev
}

func cosmeticCount(t *testing.T, pool *pgxpool.Pool, char id.UUID, sourceRef string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM character_cosmetic_entitlements
		 WHERE character_id=$1 AND source_ref=$2`, char.String(), sourceRef).Scan(&n); err != nil {
		t.Fatalf("cosmetics: %v", err)
	}
	return n
}

// TestTierRewardAutoSettlesAtPromotion — a promotion settles its full
// bundle in the same transaction: currency.special credit, presentation
// entitlement, LIFE_SKILL EXP for the current act, tier row and the
// deterministic reward_operation_id; no unclaimed state exists.
func TestTierRewardAutoSettlesAtPromotion(t *testing.T) {
	pool := newPool(t)
	char := seed(t, pool)
	exec := durableatlas.SettlementExecutor(deps(pool))
	pageID := "atlas.page.quai_dam.lang_da.dom_dom_ma" // T1 = 1 kill

	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return exec(ctx, tx, settleRec(char, id.NewV4(), pageID, 1))
	})
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("settle: %v", out.GetErrorCode())
	}

	row := atlasRow(t, pool, char, pageID)
	if row.ReachedTier != 1 || row.Counter != 1 {
		t.Fatalf("tier=%d counter=%d, want 1/1", row.ReachedTier, row.Counter)
	}
	if row.RewardOpID == nil || *row.RewardOpID != durableatlas.TierOpID(char, pageID, 1) {
		t.Fatalf("reward_operation_id = %v, want deterministic tier triple", row.RewardOpID)
	}
	if got := specialBalance(t, pool, char); got != 1 {
		t.Fatalf("special = %d, want 1 (T1 bundle auto-settled)", got)
	}
	if got := cosmeticCount(t, pool, char, "atlas.tier."+pageID+".1"); got != 1 {
		t.Fatalf("entitlement rows = %d, want 1", got)
	}
	if got := expOf(t, pool, char); got != 6417 {
		t.Fatalf("exp = %d, want 6417 (act I LIFE_SKILL)", got)
	}
	if got := revisionOf(t, pool, char); got != 1 {
		t.Fatalf("revision = %d, want 1", got)
	}
	if len(out.GetRewardSlots()) != 1 ||
		out.GetRewardSlots()[0].GetRewardSlot() != "atlas.tier."+pageID+".1" {
		t.Fatalf("reward_slots = %v", out.GetRewardSlots())
	}
}

// TestClaimAcknowledgeOnly — 504 writes acknowledged_at once, grants
// nothing (no currency, no counter change), and returns the settled
// tier view.
func TestClaimAcknowledgeOnly(t *testing.T) {
	pool := newPool(t)
	char := seed(t, pool)
	settle := durableatlas.SettlementExecutor(deps(pool))
	ack := durableatlas.AcknowledgeExecutor(deps(pool))
	pageID := "atlas.page.quai_dam.lang_da.dom_dom_ma"

	runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return settle(ctx, tx, settleRec(char, id.NewV4(), pageID, 1))
	})
	expBefore, specialBefore := expOf(t, pool, char), specialBalance(t, pool, char)

	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return ack(ctx, tx, ackRec(char, id.NewV4(), pageID, 1))
	})
	res := out.GetS2CAtlasClaimResult()
	if res == nil || res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("ack result = %v", res)
	}
	if res.GetGranted() == nil || len(res.GetGranted().GetCurrencyDelta()) != 1 {
		t.Fatalf("granted view = %v, want tier-1 bundle echo", res.GetGranted())
	}
	row := atlasRow(t, pool, char, pageID)
	if row.AcknowledgedAt == nil {
		t.Fatal("acknowledged_at not set after 504")
	}
	if expOf(t, pool, char) != expBefore || specialBalance(t, pool, char) != specialBefore {
		t.Fatal("ack granted currency/exp — 504 must acknowledge only")
	}
	if got := revisionOf(t, pool, char); got != 2 {
		t.Fatalf("revision = %d, want 2 (settle + ack)", got)
	}
}

// TestClaimUnreachedTierRejected — 504 on a tier beyond the reached tier
// resolves ATLAS_TIER_NOT_REACHED and changes no state.
func TestClaimUnreachedTierRejected(t *testing.T) {
	pool := newPool(t)
	char := seed(t, pool)
	settle := durableatlas.SettlementExecutor(deps(pool))
	ack := durableatlas.AcknowledgeExecutor(deps(pool))
	pageID := "atlas.page.quai_dam.lang_da.dom_dom_ma"

	runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return settle(ctx, tx, settleRec(char, id.NewV4(), pageID, 1))
	})
	revBefore := revisionOf(t, pool, char)

	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return ack(ctx, tx, ackRec(char, id.NewV4(), pageID, 2))
	})
	res := out.GetS2CAtlasClaimResult()
	if res == nil || res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_ERROR {
		t.Fatalf("ack result = %v, want ERROR", res)
	}
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_ATLAS_TIER_NOT_REACHED {
		t.Fatalf("error = %v, want ATLAS_TIER_NOT_REACHED", res.GetResult().GetErrorCode())
	}
	row := atlasRow(t, pool, char, pageID)
	if row.AcknowledgedAt != nil {
		t.Fatal("rejected ack wrote acknowledged_at")
	}
	if revisionOf(t, pool, char) != revBefore {
		t.Fatal("rejected ack bumped revision")
	}
}

// TestDuplicateClaimAcknowledgesOnce — a duplicate 504 for the same
// (page, tier) returns the existing acknowledgement: acknowledged_at
// stays the first timestamp and no revision bump repeats.
func TestDuplicateClaimAcknowledgesOnce(t *testing.T) {
	pool := newPool(t)
	char := seed(t, pool)
	settle := durableatlas.SettlementExecutor(deps(pool))
	ack := durableatlas.AcknowledgeExecutor(deps(pool))
	pageID := "atlas.page.quai_dam.lang_da.dom_dom_ma"

	runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return settle(ctx, tx, settleRec(char, id.NewV4(), pageID, 1))
	})
	runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return ack(ctx, tx, ackRec(char, id.NewV4(), pageID, 1))
	})
	first := atlasRow(t, pool, char, pageID).AcknowledgedAt
	revAfterFirst := revisionOf(t, pool, char)
	time.Sleep(2 * time.Millisecond)

	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return ack(ctx, tx, ackRec(char, id.NewV4(), pageID, 1))
	})
	res := out.GetS2CAtlasClaimResult()
	if res == nil || res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("dup ack = %v, want SUCCESS (existing row returned)", res)
	}
	second := atlasRow(t, pool, char, pageID).AcknowledgedAt
	if second == nil || !second.Equal(*first) {
		t.Fatalf("acknowledged_at moved %v -> %v; dup-504 must not rewrite", first, second)
	}
	if revisionOf(t, pool, char) != revAfterFirst {
		t.Fatal("dup-504 bumped revision")
	}
}

// TestEventKindsPromoteTiersOnce — each unlock kind (MONSTER_KILLED,
// FISH_CAUGHT, CHEST_OPENED, DISH_COOKED, BOSS_DEFEATED, SOUL_ACQUIRED)
// promotes its page's tier exactly once; a repeated event for the same
// page never re-grants the settled tier bundle.
func TestEventKindsPromoteTiersOnce(t *testing.T) {
	pool := newPool(t)
	char := seed(t, pool)
	exec := durableatlas.SettlementExecutor(deps(pool))

	cases := []struct {
		kind     simatlas.EventKind
		sourceID string
		pageID   string
		delta    uint64
	}{
		{simatlas.EventMonsterKilled, "monster.lang_da.dom_dom_ma", "atlas.page.quai_dam.lang_da.dom_dom_ma", 1},
		{simatlas.EventFishCaught, "item.material.ca_bong", "atlas.page.co_vat.ca_bong", 1},
		{simatlas.EventChestOpened, "chest.hidden.lang_da.01", "atlas.page.co_vat.ruong_co_01", 1},
		{simatlas.EventDishCooked, "item.consumable.food.ca_bong_kho", "atlas.page.co_vat.ca_bong_kho", 1},
		{simatlas.EventBossWitness, "boss.quy_nhap_trang", "atlas.page.di_tich.quy_nhap_trang", 1},
		{simatlas.EventSoulAcquired, "soul.normal.dom_dom_ma", "atlas.page.hon_giam.dom_dom_ma", 3},
	}
	for _, c := range cases {
		pages := simatlas.Resolve(simatlas.SourceEvent{Kind: c.kind, SourceID: c.sourceID, Value: c.delta})
		found := false
		for _, p := range pages {
			if p.ID == c.pageID {
				found = true
			}
		}
		if !found {
			t.Fatalf("kind %v source %s did not resolve page %s", c.kind, c.sourceID, c.pageID)
		}
	}

	specialBefore := specialBalance(t, pool, char)
	for _, c := range cases {
		runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return exec(ctx, tx, settleRec(char, id.NewV4(), c.pageID, c.delta))
		})
	}
	for _, c := range cases {
		row := atlasRow(t, pool, char, c.pageID)
		wantTier := uint32(1)
		if c.pageID == "atlas.page.hon_giam.dom_dom_ma" {
			wantTier = 2 // soul level 3 clears T1(1) + T2(Lv3)
		}
		if row.ReachedTier != wantTier {
			t.Fatalf("%s tier = %d, want %d", c.pageID, row.ReachedTier, wantTier)
		}
	}
	// Repeat events on the same pages promote nothing again — each tier
	// settles once.
	for _, c := range cases {
		runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return exec(ctx, tx, settleRec(char, id.NewV4(), c.pageID, c.delta))
		})
	}
	for _, c := range cases {
		key := "atlas.tier." + c.pageID + ".1"
		if got := cosmeticCount(t, pool, char, key); got != 1 {
			t.Fatalf("%s: tier-1 entitlement rows = %d, want 1 (settle once)", c.pageID, got)
		}
	}
	if got := specialBalance(t, pool, char) - specialBefore; got != 8 {
		t.Fatalf("special delta = %d, want 8 (5×T1=1 + soul T1+T2=1+2)", got)
	}
}

// TestSeasonalPagesReuseAtlasStore — IMP-052 regression: seasonal
// chapter pages settle through this same runtime into character_atlas;
// no second store exists.
func TestSeasonalPagesReuseAtlasStore(t *testing.T) {
	pool := newPool(t)
	char := seed(t, pool)
	exec := durableatlas.SettlementExecutor(deps(pool))
	pageID := "atlas.page.season.0.lang_da.dom_dom_nguyen"

	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return exec(ctx, tx, settleRec(char, id.NewV4(), pageID, 1))
	})
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("seasonal settle: %v", out.GetErrorCode())
	}
	row := atlasRow(t, pool, char, pageID)
	if row.ReachedTier != 1 || row.Counter != 1 {
		t.Fatalf("seasonal page tier=%d counter=%d, want 1/1 on character_atlas", row.ReachedTier, row.Counter)
	}
	// Seasonal pages grant 0 special — no currency movement.
	if got := specialBalance(t, pool, char); got != 0 {
		t.Fatalf("seasonal granted %d special, want 0", got)
	}
}
