package trade

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestDirectTradeCurrencyFeeFivePercentCeiling covers the authored fee
// formula: fee = floor(offered_common * 0.05) charged to the offering
// side, receiver gets the rest (trading_auction.md § Direct Trade
// Fees; #215-canonical).
func TestDirectTradeCurrencyFeeFivePercentCeiling(t *testing.T) {
	pool := newPool(t)
	cases := []struct{ offered, fee int64 }{
		{0, 0}, {1, 0}, {19, 0}, {20, 1}, {100, 5}, {199, 9}, {1000, 50},
	}
	for _, tc := range cases {
		if got := feeOf(tc.offered); got != tc.fee {
			t.Fatalf("fee(%d)=%d want %d", tc.offered, got, tc.fee)
		}
	}
	init := seedParticipant(t, pool, 1000)
	cpart := seedParticipant(t, pool, 0)
	j := buildJournal(id.NewV4(), id.NewV4(), id.NewV4(), init, cpart, nil, nil, 199, 0)
	out, err := runSettle(t, pool, SettleIn{Trade: j, OperationID: id.NewV4(), Locks: lockedLedger(nil)})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if out.Fee != 9 {
		t.Fatalf("fee %d want 9", out.Fee)
	}
	if b := balanceOf(t, pool, init.char); b != 801 {
		t.Fatalf("offerer balance %d want 801 (gross debit)", b)
	}
	if b := balanceOf(t, pool, cpart.char); b != 190 {
		t.Fatalf("receiver balance %d want 190 (net of fee)", b)
	}
	var sent int64
	if err := pool.QueryRow(context.Background(),
		`SELECT common_sent_by_initiator FROM trade_settlement_records WHERE trade_id=$1`,
		tradeIDFromJournal(j).String()).Scan(&sent); err != nil {
		t.Fatalf("settlement row: %v", err)
	}
	if sent != 199 {
		t.Fatalf("common_sent %d want 199", sent)
	}
}

// TestCurrencyFeeChargedOnlyToOfferingSide: the fee is taken from the
// offer alone — the non-offering side is never debited.
func TestCurrencyFeeChargedOnlyToOfferingSide(t *testing.T) {
	pool := newPool(t)
	init := seedParticipant(t, pool, 400)
	cpart := seedParticipant(t, pool, 700)
	j := buildJournal(id.NewV4(), id.NewV4(), id.NewV4(), init, cpart, nil, nil, 200, 0)
	out, err := runSettle(t, pool, SettleIn{Trade: j, OperationID: id.NewV4(), Locks: lockedLedger(nil)})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if out.Fee != 10 {
		t.Fatalf("fee %d want 10", out.Fee)
	}
	if b := balanceOf(t, pool, init.char); b != 200 {
		t.Fatalf("offerer %d want 200", b)
	}
	if b := balanceOf(t, pool, cpart.char); b != 890 {
		t.Fatalf("receiver %d want 890 (700 + 190 net)", b)
	}
}

// TestFeeFailureRollsBackBothSides: a settlement-time failure after
// currency mutation aborts the whole trade — neither balance moves.
func TestFeeFailureRollsBackBothSides(t *testing.T) {
	pool := newPool(t)
	init := seedParticipant(t, pool, 500)
	cpart := seedParticipant(t, pool, 500)
	tradeID := id.NewV4()
	// Pre-seed a settlement row with the same trade_id so the insert
	// fails after currency moves — the whole tx must roll back.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO trade_settlement_records
		 (settlement_id, settled_at, initiator_character_id, initiator_account_id,
		  counterpart_character_id, counterpart_account_id,
		  common_sent_by_initiator, common_sent_by_counterpart, trade_id,
		  settlement_operation_id, item_transfers)
		 VALUES ($1, now(), $2,$3,$4,$5,0,0,$6,$7,'[]')`,
		id.NewV4().String(), init.char.String(), init.account.String(),
		cpart.char.String(), cpart.account.String(), tradeID.String(), id.NewV4().String()); err != nil {
		t.Fatalf("preseed settlement: %v", err)
	}
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	j := buildJournal(tradeID, id.NewV4(), id.NewV4(), init, cpart, nil, nil, 100, 0)
	_, serr := New(pool).Settle(ctx, tx, SettleIn{Trade: j, OperationID: id.NewV4(), Locks: lockedLedger(nil)})
	if serr == nil {
		t.Fatalf("expected settlement error")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if b := balanceOf(t, pool, init.char); b != 500 {
		t.Fatalf("init balance %d want 500 (rolled back)", b)
	}
	if b := balanceOf(t, pool, cpart.char); b != 500 {
		t.Fatalf("cpart balance %d want 500 (rolled back)", b)
	}
}

// TestCurrencyCapRejectsWholeTrade: a post-trade balance over the cap
// rejects the trade as a verdict — nothing moves.
func TestCurrencyCapRejectsWholeTrade(t *testing.T) {
	pool := newPool(t)
	cap := int64(2_000_000_000) // currency.Caps[common]
	init := seedParticipant(t, pool, cap)
	cpart := seedParticipant(t, pool, cap)
	// receiver already at cap: +950 net would exceed → verdict
	j := buildJournal(id.NewV4(), id.NewV4(), id.NewV4(), init, cpart,
		nil, nil, 1000, 0)
	_, err := runSettleExpectErr(t, pool, SettleIn{Trade: j, OperationID: id.NewV4(),
		Locks: lockedLedger(nil)})
	var v *Verdict
	if !errors.As(err, &v) || v.Code != protocolv1.ErrorCode_ERROR_CODE_CURRENCY_CAP_EXCEEDED {
		t.Fatalf("verdict %v want CURRENCY_CAP_EXCEEDED", err)
	}
	if b := balanceOf(t, pool, init.char); b != cap {
		t.Fatalf("init balance moved: %d", b)
	}
	if b := balanceOf(t, pool, cpart.char); b != cap {
		t.Fatalf("cpart balance moved: %d", b)
	}
}

// tradeIDFromJournal decodes the journal's trade id.
func tradeIDFromJournal(j *journalv1.JournalTrade) id.UUID {
	var tid id.UUID
	copy(tid[:], j.GetTradeId())
	return tid
}

// runSettleExpectErr runs the tx and returns the settle error (tx is
// always rolled back).
func runSettleExpectErr(t *testing.T, pool *pgxpool.Pool, in SettleIn) (SettleOut, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	out, serr := New(pool).Settle(ctx, tx, in)
	return out, serr
}
