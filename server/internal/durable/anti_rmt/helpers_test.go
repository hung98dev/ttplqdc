package anti_rmt

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
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/durable/trade"
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
	// server/internal/durable/anti_rmt -> repo root
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
	name := fmt.Sprintf("anti_rmt_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

func ctx0() context.Context { return context.Background() }

func inTx(t *testing.T, fn func(tx pgx.Tx) error) {
	t.Helper()
	tx, err := pool(t).Begin(ctx0())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx0()) }()
	if err := fn(tx); err != nil {
		t.Fatalf("tx: %v", err)
	}
	if err := tx.Commit(ctx0()); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func mkAccount(t *testing.T) id.UUID {
	t.Helper()
	acct := id.NewV4()
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("account: %v", err)
	}
	return acct
}

func mkCharacter(t *testing.T, accountID id.UUID, level int, age time.Duration) id.UUID {
	t.Helper()
	char := id.NewV4()
	name := fmt.Sprintf("ar_%d", time.Now().UnixNano()%1_000_000_000)
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, created_at)
		 VALUES ($1,$2,$3,$4,'class.kim',$5,$6)`,
		char.String(), accountID.String(), name, name, level,
		time.Now().Add(-age)); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO character_inventories (character_id, capacity) VALUES ($1,60)`,
		char.String()); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	return char
}

func mkWallet(t *testing.T, charID id.UUID, amount int64) {
	t.Helper()
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO character_currencies (character_id, currency_id, balance)
		 VALUES ($1,'currency.common',$2) ON CONFLICT DO NOTHING`, charID.String(), amount); err != nil {
		t.Fatalf("wallet: %v", err)
	}
}

// seedAcctRollup inserts one account-level rollup row at utc_day.
func seedAcctRollup(t *testing.T, acct id.UUID, day time.Time, outflow, inflow int64) {
	t.Helper()
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO economy_account_daily_rollups
		 (account_id, utc_day, common_outflow, common_inflow, updated_at)
		 VALUES ($1,$2,$3,$4,NOW())`,
		acct.String(), day.UTC().Truncate(24*time.Hour), outflow, inflow); err != nil {
		t.Fatalf("acct rollup: %v", err)
	}
}

// seedCharRollup inserts one character rollup row with partner maps.
func seedCharRollup(t *testing.T, char id.UUID, day time.Time,
	outflow, inflow int64, vols, counts map[string]int64) {
	t.Helper()
	volsJSON := mustJSON(vols)
	countsJSON := mustJSON(counts)
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO economy_character_daily_rollups
		 (character_id, utc_day, common_outflow, common_inflow,
		  trade_partner_volumes, item_partner_counts, updated_at)
		 VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,NOW())`,
		char.String(), day.UTC().Truncate(24*time.Hour), outflow, inflow,
		volsJSON, countsJSON); err != nil {
		t.Fatalf("char rollup: %v", err)
	}
}

func mustJSON(m map[string]int64) string {
	if len(m) == 0 {
		return "{}"
	}
	out := "{"
	first := true
	for k, v := range m {
		if !first {
			out += ","
		}
		first = false
		out += fmt.Sprintf("%q:%d", k, v)
	}
	return out + "}"
}

// seedTrade inserts one settled direct-trade record with optional
// item transfers (the raw provenance partial-day reads consume).
func seedTrade(t *testing.T, initChar, initAcct, cpartChar, cpartAcct id.UUID,
	settledAt time.Time, initSent, cpartSent int64,
	transfers []trade.ItemTransfer) {
	t.Helper()
	settleID := id.NewV4()
	tJSON := "[]"
	if len(transfers) > 0 {
		tJSON = "["
		for i, tr := range transfers {
			if i > 0 {
				tJSON += ","
			}
			tJSON += fmt.Sprintf(
				`{"from_character_id":%q,"to_character_id":%q,`+
					`"item_instance_id":%q,"delivered_instance_id":%q,`+
					`"item_id":%q,"quantity":%d}`,
				tr.FromCharacterID, tr.ToCharacterID,
				tr.ItemInstanceID, tr.DeliveredInstanceID,
				tr.ItemID, tr.Quantity)
		}
		tJSON += "]"
	}
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO trade_settlement_records
		 (settlement_id, trade_id, settlement_operation_id, settled_at,
		  initiator_character_id, initiator_account_id,
		  counterpart_character_id, counterpart_account_id,
		  common_sent_by_initiator, common_sent_by_counterpart,
		  item_transfers)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)`,
		settleID.String(), id.NewV4().String(), id.NewV4().String(), settledAt,
		initChar.String(), initAcct.String(),
		cpartChar.String(), cpartAcct.String(),
		initSent, cpartSent, tJSON); err != nil {
		t.Fatalf("trade record: %v", err)
	}
}

// seedAuction inserts a SOLD listing + its proceeds row (the raw
// auction provenance for partial days).
func seedAuction(t *testing.T, sellerChar, sellerAcct, buyerChar, buyerAcct id.UUID,
	settledAt time.Time, price, proceeds int64, qty int) {
	t.Helper()
	if price < 100 {
		t.Fatalf("price_common floor 100")
	}
	listingID := id.NewV4()
	instID := id.NewV4()
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO item_instances
		 (item_instance_id, item_id, quantity, effective_binding,
		  enhancement_level, item_state, created_at)
		 VALUES ($1,'item.potion.hp',$2,'UNBOUND',0,'{}'::jsonb,NOW())`,
		instID.String(), qty); err != nil {
		t.Fatalf("instance: %v", err)
	}
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO auction_listings
		 (listing_id, seller_character_id, seller_account_id,
		  item_instance_id, item_id, quantity, price_common,
		  listing_fee_common, state, listed_at, expires_at, ended_at,
		  buyer_character_id, revision)
		 VALUES ($1,$2,$3,$4,'item.potion.hp',$5,$6,10,'SOLD',$7,$7,$7,$8,0)`,
		listingID.String(), sellerChar.String(), sellerAcct.String(),
		instID.String(), qty, price, settledAt, buyerChar.String()); err != nil {
		t.Fatalf("listing: %v", err)
	}
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO auction_proceeds
		 (proceeds_id, listing_id, item_id, quantity,
		  seller_character_id, seller_account_id,
		  buyer_character_id, buyer_account_id,
		  proceeds_amount, settled_at, state)
		 VALUES ($1,$2,'item.potion.hp',$3,$4,$5,$6,$7,$8,$9,'PENDING')`,
		id.NewV4().String(), listingID.String(), qty,
		sellerChar.String(), sellerAcct.String(),
		buyerChar.String(), buyerAcct.String(),
		proceeds, settledAt); err != nil {
		t.Fatalf("proceeds: %v", err)
	}
}

// flagOf reads accounts.economy_review_flagged_at.
func flagOf(t *testing.T, acct id.UUID) bool {
	t.Helper()
	var at *time.Time
	if err := pool(t).QueryRow(ctx0(),
		`SELECT economy_review_flagged_at FROM accounts WHERE account_id=$1`,
		acct.String()).Scan(&at); err != nil {
		t.Fatalf("flag read: %v", err)
	}
	return at != nil
}

// auditCount counts audit_events rows for the account/action.
func auditCount(t *testing.T, acct id.UUID, action string) int {
	t.Helper()
	var n int
	if err := pool(t).QueryRow(ctx0(),
		`SELECT COUNT(*) FROM audit_events
		 WHERE subject_account_id=$1 AND action=$2`,
		acct.String(), action).Scan(&n); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return n
}

// record runs RecordSettlement inside one committed tx.
func record(t *testing.T, s *Store, acct id.UUID, at time.Time) {
	t.Helper()
	inTx(t, func(tx pgx.Tx) error {
		return s.RecordSettlement(ctx0(), tx, SettlementEvent{AccountID: acct, At: at})
	})
}

// --- trade gate harness (mirrors durable/trade test wiring) ---

type participant struct {
	account, char id.UUID
}

func tradeJournal(tradeID, settlementID, opID id.UUID,
	init, cpart participant, initCommon, cpartCommon int64) *journalv1.JournalTrade {
	return &journalv1.JournalTrade{
		TradeId:          tradeID[:],
		ExpectedRevision: 3,
		Initiator: &journalv1.JournalTradeSide{
			CharacterId: init.char[:], AccountId: init.account[:],
			CommonAmount: initCommon,
		},
		Counterpart: &journalv1.JournalTradeSide{
			CharacterId: cpart.char[:], AccountId: cpart.account[:],
			CommonAmount: cpartCommon,
		},
		SettlementId:                settlementID[:],
		FeeCommon:                   initCommon/20 + cpartCommon/20,
		FinalizedAtMs:               time.Now().UnixMilli(),
		InitiatingClientOperationId: opID[:],
	}
}

func submitFinalise(t *testing.T, store *trade.Store,
	p participant, j *journalv1.JournalTrade) *journalv1.JournalOutcome {
	t.Helper()
	idem := idempotency.NewStore(pool(t))
	var opID id.UUID
	copy(opID[:], j.GetInitiatingClientOperationId())
	req := &protocolv1.C2STradeFinalise{
		OperationId:      opID[:],
		TradeId:          j.GetTradeId(),
		ExpectedRevision: j.GetExpectedRevision(),
	}
	rec, err := trade.FinaliseRecord(p.account, 7, 11, p.char, req, j, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	locks := func(id.UUID) *items.TradeLockLedger {
		return items.NewTradeLockLedger(id.NewV4())
	}
	ex := trade.Executors(trade.Deps{Store: store, Locks: locks})[trade.FinaliseFamily]
	out, err := idem.TrustedReplay(ctx0(), trade.FinaliseFamily,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: p.char}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return ex(ctx, tx, rec)
		})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	var outcome journalv1.JournalOutcome
	if err := protojson.Unmarshal(out.Payload, &outcome); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	return &outcome
}

var _ = proto.Marshal // keep proto import used
