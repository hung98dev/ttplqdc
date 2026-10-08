package discovery

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
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/schema"
	protocolv1 "thinhthan/internal/protocol/v1"
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
	name := fmt.Sprintf("discovery_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

func seed(t *testing.T, pool *pgxpool.Pool) (id.UUID, id.UUID) {
	t.Helper()
	acct, char := id.NewV4(), id.NewV4()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	key := "dscchar" + char.String()[:10]
	if _, err := pool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, current_exp)
		 VALUES ($1,$2,$3,$4,'class.kim',1,0)`,
		char.String(), acct.String(), key, key); err != nil {
		t.Fatalf("seed character: %v", err)
	}
	return acct, char
}

func seedBalance(t *testing.T, pool *pgxpool.Pool, char id.UUID, balance int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance)
		 VALUES ($1,'currency.common',$2)`, char.String(), balance); err != nil {
		t.Fatalf("seed balance: %v", err)
	}
}

func deps(pool *pgxpool.Pool) Deps {
	exp := map[string]uint64{
		"map.lang_da.dinh_lang":    7700,
		"map.ben_nuoc_den.cho_ben": 131700,
	}
	anchors := map[string]string{
		"map.lang_da.dinh_lang":    "checkpoint.lang_da.dinh_lang",
		"map.ben_nuoc_den.cho_ben": "checkpoint.ben_nuoc_den.cho_ben",
	}
	fees := map[string]int64{
		"map.lang_da.dinh_lang":    0,
		"map.ben_nuoc_den.cho_ben": 200,
	}
	return Deps{
		Store:    New(pool, time.Now),
		Progress: progression.NewStore(pool, progression.WithClock(time.Now)),
		ExpFor: func(m string) (uint64, bool) {
			v, ok := exp[m]
			return v, ok
		},
		AnchorFor: func(m string) (string, bool) {
			v, ok := anchors[m]
			return v, ok
		},
		FeeFor: func(m string) (int64, bool) {
			v, ok := fees[m]
			return v, ok
		},
		Now: time.Now,
	}
}

func rewardRec(char, op id.UUID, mapID string) *journalv1.DurableCommandRecord {
	return &journalv1.DurableCommandRecord{
		OperationFamily: Family,
		OwnerId:         char[:],
		OperationId:     op[:],
		Command: &journalv1.DurableCommandRecord_Reward{
			Reward: &journalv1.JournalRewardCommand{
				Kind:        RewardKind,
				DiscoveryId: &mapID,
			},
		},
	}
}

func travelRec(char, op id.UUID, dest string) *journalv1.DurableCommandRecord {
	svc := TravelServiceID
	return &journalv1.DurableCommandRecord{
		OperationFamily: familyNpcService,
		OwnerId:         char[:],
		OperationId:     op[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				Request: &journalv1.JournalClientCommand_C2SInteract{
					C2SInteract: &protocolv1.C2SInteract{
						InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
						ServiceId:    &svc,
						ServiceParam: &dest,
					},
				},
			},
		},
	}
}

func decodeOutcome(t *testing.T, out idempotency.Outcome) *journalv1.JournalOutcome {
	t.Helper()
	o := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, o); err != nil {
		t.Fatalf("decode outcome: %v", err)
	}
	return o
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

func discoveredRow(t *testing.T, pool *pgxpool.Pool, char id.UUID, mapID string) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM character_discoveries
		 WHERE character_id=$1 AND kind='MAP' AND content_key=$2)`,
		char.String(), mapID).Scan(&ok); err != nil {
		t.Fatalf("discovered: %v", err)
	}
	return ok
}

func runInTx(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error)) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out, err := fn(ctx, tx)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return decodeOutcome(t, out)
}

func TestDiscoverySettlementCommitsOnceOnly(t *testing.T) {
	pool := newPool(t)
	_, char := seed(t, pool)
	d := deps(pool)
	exec := RewardExecutor(d)
	op1, op2 := id.NewV4(), id.NewV4()

	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return exec(ctx, tx, rewardRec(char, op1, "map.ben_nuoc_den.cho_ben"))
	})
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("first settle: %v", out.GetErrorCode())
	}
	if len(out.GetRewardSlots()) != 1 || out.GetRewardSlots()[0].GetCharacterExp() != 131700 {
		t.Fatalf("slots = %v", out.GetRewardSlots())
	}
	if !discoveredRow(t, pool, char, "map.ben_nuoc_den.cho_ben") {
		t.Fatal("discovery row missing after first settle")
	}
	if got := expOf(t, pool, char); got != 131700 {
		t.Fatalf("exp = %d, want 131700", got)
	}

	// A second record for the same key commits SUCCESS but never
	// re-grants — reconnect/replay safety is durable, not best-effort.
	out = runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return exec(ctx, tx, rewardRec(char, op2, "map.ben_nuoc_den.cho_ben"))
	})
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("replay settle: %v", out.GetErrorCode())
	}
	if len(out.GetRewardSlots()) != 0 {
		t.Fatalf("replay granted %d slots", len(out.GetRewardSlots()))
	}
	if got := expOf(t, pool, char); got != 131700 {
		t.Fatalf("exp after replay = %d, want 131700", got)
	}
}

func TestDiscoverySettlementLevelsUp(t *testing.T) {
	pool := newPool(t)
	_, char := seed(t, pool)
	d := deps(pool)
	exec := RewardExecutor(d)
	op := id.NewV4()

	runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return exec(ctx, tx, rewardRec(char, op, "map.ben_nuoc_den.cho_ben"))
	})
	// 131700 exp → cumulative to reach level 5 is 10000+40000+90000+160000=300000,
	// level 4 needs 140000 — so 131700 lands at level 3 (cum 50000 → reach 4).
	var lvl int32
	if err := pool.QueryRow(context.Background(),
		`SELECT level FROM characters WHERE character_id=$1`, char.String()).Scan(&lvl); err != nil {
		t.Fatalf("level: %v", err)
	}
	if lvl != 3 {
		t.Fatalf("level = %d, want 3", lvl)
	}
	var sk, pot int32
	if err := pool.QueryRow(context.Background(),
		`SELECT unspent_skill_points, unspent_potential_points FROM characters WHERE character_id=$1`,
		char.String()).Scan(&sk, &pot); err != nil {
		t.Fatalf("points: %v", err)
	}
	if sk != 2 || pot != 8 {
		t.Fatalf("points = %d/%d, want 2/8 (two level-ups)", sk, pot)
	}
}

func TestTravelCommitsFeeAndDiscoveryGates(t *testing.T) {
	pool := newPool(t)
	_, char := seed(t, pool)
	seedBalance(t, pool, char, 500)
	d := deps(pool)
	exec := TravelExecutor(d)

	// Undiscovered destination → NOT_DISCOVERED verdict, no debit.
	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return exec(ctx, tx, travelRec(char, id.NewV4(), "map.ben_nuoc_den.cho_ben"))
	})
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_NOT_DISCOVERED {
		t.Fatalf("undiscovered travel: %v", out.GetErrorCode())
	}
	if bal := balanceOf(t, pool, char); bal != 500 {
		t.Fatalf("debit on rejected travel: bal = %d", bal)
	}

	// Commit the discovery, then travel → SUCCESS + 200 fee debit.
	runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return RewardExecutor(d)(ctx, tx, rewardRec(char, id.NewV4(), "map.ben_nuoc_den.cho_ben"))
	})
	out = runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return exec(ctx, tx, travelRec(char, id.NewV4(), "map.ben_nuoc_den.cho_ben"))
	})
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("discovered travel: %v", out.GetErrorCode())
	}
	if bal := balanceOf(t, pool, char); bal != 300 {
		t.Fatalf("post-travel balance = %d, want 300", bal)
	}
	s2c := out.GetS2CInteractResult()
	if len(s2c.GetCurrencyDelta()) != 1 || s2c.GetCurrencyDelta()[0].GetAmount() != -200 {
		t.Fatalf("currency_delta = %v", s2c.GetCurrencyDelta())
	}
}

func TestTravelInsufficientBalanceVerdict(t *testing.T) {
	pool := newPool(t)
	_, char := seed(t, pool)
	seedBalance(t, pool, char, 50)
	d := deps(pool)

	runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return RewardExecutor(d)(ctx, tx, rewardRec(char, id.NewV4(), "map.ben_nuoc_den.cho_ben"))
	})
	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return TravelExecutor(d)(ctx, tx, travelRec(char, id.NewV4(), "map.ben_nuoc_den.cho_ben"))
	})
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY {
		t.Fatalf("poor travel: %v", out.GetErrorCode())
	}
	if bal := balanceOf(t, pool, char); bal != 50 {
		t.Fatalf("debit on insufficient travel: bal = %d", bal)
	}
}

func TestNpcServiceMuxDispatch(t *testing.T) {
	pool := newPool(t)
	_, char := seed(t, pool)
	seedBalance(t, pool, char, 100)
	d := deps(pool)
	mux := NpcServiceMux{TravelServiceID: TravelExecutor(d)}

	// Registered service routes to the travel executor.
	out := runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return mux.Exec(ctx, tx, travelRec(char, id.NewV4(), "map.lang_da.dinh_lang"))
	})
	// lang_da is undiscovered → NOT_DISCOVERED proves travel ran (fee 0).
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_NOT_DISCOVERED {
		t.Fatalf("routed travel: %v", out.GetErrorCode())
	}

	// Unregistered service_id → exactly one TARGET_INVALID 116.
	svc := "respec"
	badOp := id.NewV4()
	bad := &journalv1.DurableCommandRecord{
		OperationFamily: familyNpcService,
		OwnerId:         char[:],
		OperationId:     badOp[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				Request: &journalv1.JournalClientCommand_C2SInteract{
					C2SInteract: &protocolv1.C2SInteract{
						InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
						ServiceId:    &svc,
					},
				},
			},
		},
	}
	out = runInTx(t, pool, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return mux.Exec(ctx, tx, bad)
	})
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("unregistered service: %v, want TARGET_INVALID", out.GetErrorCode())
	}
}
