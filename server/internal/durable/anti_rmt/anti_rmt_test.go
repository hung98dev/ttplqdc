package anti_rmt

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/auction"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/trade"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// predicates returns evaluation results keyed by predicate name.
func evalAll(t *testing.T, s *Store, acct id.UUID, at time.Time) map[string]Predicate {
	t.Helper()
	var preds []Predicate
	inTx(t, func(tx pgx.Tx) error {
		var err error
		preds, err = s.EvaluateAccount(ctx0(), tx, acct, at)
		return err
	})
	out := map[string]Predicate{}
	for _, p := range preds {
		out[p.Name] = p
	}
	return out
}

// TestNetOutflow7d asserts predicate (a) arithmetic: whole-day rollup
// rows plus raw partial-day settlements over [T-7d,T); the top-1%
// kth-cutoff (k = ceil(N/100)) admits every account at or above the
// cutoff including ties.
func TestNetOutflow7d(t *testing.T) {
	s := New(pool(t))
	at := evalT()
	acct := mkAccount(t)

	// Whole-day rollups: 03-15..03-19 inside [03-13T15, 03-20T15).
	for d := 15; d <= 19; d++ {
		seedAcctRollup(t, acct, time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC),
			4_000_000, 0)
	}
	// Raw partials: oldest day (03-13T18) + current day (03-20T10) —
	// together 6M outflow => net 26M > 20M.
	partner := mkAccount(t)
	pc := mkCharacter(t, partner, 60, 72*time.Hour)
	mc := mkCharacter(t, acct, 60, 72*time.Hour)
	seedTrade(t, mc, acct, pc, partner,
		time.Date(2026, 3, 13, 18, 0, 0, 0, time.UTC), 3_000_000, 0, nil)
	seedTrade(t, mc, acct, pc, partner,
		time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC), 3_000_000, 0, nil)

	// Population: 99 more accounts with one tiny event each in-window.
	for i := 0; i < 99; i++ {
		a := mkAccount(t)
		c := mkCharacter(t, a, 60, 72*time.Hour)
		seedTrade(t, c, a, pc, partner,
			time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC), 100, 0, nil)
	}
	// N = 100 + partner + acct itself = 101 -> k = ceil(101/100) = 2.
	// acct (26M) and partner (26M inflow-negative... its net is
	// -(received-fee) — recompute: partner received 6M-fee = net -5.7M).
	preds := evalAll(t, s, acct, at)
	if !preds["net_outflow_7d_top_1pct"].Qualified {
		t.Fatalf("net_outflow predicate not qualified: %+v",
			preds["net_outflow_7d_top_1pct"])
	}

	// Negative control: an active account below the 20M floor and below
	// the cutoff does not qualify.
	small := mkAccount(t)
	sc := mkCharacter(t, small, 60, 72*time.Hour)
	seedTrade(t, sc, small, pc, partner,
		time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC), 100, 0, nil)
	if evalAll(t, s, small, at)["net_outflow_7d_top_1pct"].Qualified {
		t.Fatal("small account qualified")
	}

	// An account with no events in the window is never flagged even
	// though its rollup history would rank first.
	idle := mkAccount(t)
	seedAcctRollup(t, idle, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		999_000_000, 0)
	if evalAll(t, s, idle, at)["net_outflow_7d_top_1pct"].Qualified {
		t.Fatal("inactive account qualified")
	}
}

// TestPartnerConcentration30d asserts predicate (b): per-partner
// aggregation across whole-day rollups and partial-day raw rows, the
// 5M floor, and integer 5*top3 >= 4*total — OR'd across characters.
func TestPartnerConcentration30d(t *testing.T) {
	s := New(pool(t))
	at := evalT()
	acct := mkAccount(t)
	c1 := mkCharacter(t, acct, 60, 72*time.Hour)
	c2 := mkCharacter(t, acct, 60, 72*time.Hour)
	p1, p2, p3 := id.NewV4(), id.NewV4(), id.NewV4()

	// c1: vols spread across four partners -> no concentration.
	seedCharRollup(t, c1, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		6_000_000, 0, map[string]int64{
			p1.String(): 2_000_000, p2.String(): 2_000_000,
			p3.String(): 2_000_000}, nil)
	// c2: 5.4M concentrated on one partner over two days + a raw
	// partial-day send — aggregated per partner before ranking.
	seedCharRollup(t, c2, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		3_000_000, 0, map[string]int64{p1.String(): 3_000_000}, nil)
	seedCharRollup(t, c2, time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC),
		2_000_000, 0, map[string]int64{p1.String(): 1_000_000,
			p2.String(): 1_000_000}, nil)
	src := mkAccount(t)
	sc := mkCharacter(t, src, 60, 72*time.Hour)
	seedTrade(t, c2, acct, sc, src,
		time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC), 400_000, 0, nil)
	// c2 totals: p1 4.4M? — trade send went to sc (a fourth partner).
	// total = 3M+2M+0.4M = 5.4M >= 5M; top3 = 4M+1M+0.4M = 5.4M ->
	// 5*5.4 >= 4*5.4 -> qualified.
	preds := evalAll(t, s, acct, at)
	if !preds["partner_concentration_30d"].Qualified {
		t.Fatalf("partner concentration not qualified: %+v",
			preds["partner_concentration_30d"])
	}

	// Negative: below the 5M floor the same shape does not qualify.
	b := mkAccount(t)
	bc := mkCharacter(t, b, 60, 72*time.Hour)
	seedCharRollup(t, bc, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		4_999_999, 0, map[string]int64{p1.String(): 4_999_999}, nil)
	if evalAll(t, s, b, at)["partner_concentration_30d"].Qualified {
		t.Fatal("sub-floor concentration qualified")
	}
}

// TestItemConcentration30d asserts predicate (c): received_items >= 50
// and top-3 source share >= 0.80 across rollup counts plus raw
// item_transfers, auction proceeds, and guild audits.
func TestItemConcentration30d(t *testing.T) {
	s := New(pool(t))
	at := evalT()
	acct := mkAccount(t)
	c := mkCharacter(t, acct, 60, 72*time.Hour)
	s1 := id.NewV4()

	// 40 units on whole days + 15 raw units on the current partial day.
	seedCharRollup(t, c, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC),
		0, 0, nil, map[string]int64{s1.String(): 40})
	src := mkAccount(t)
	sc := mkCharacter(t, src, 60, 72*time.Hour)
	seedTrade(t, sc, src, c, acct,
		time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC), 0, 0,
		[]trade.ItemTransfer{{
			FromCharacterID: sc, ToCharacterID: c,
			ItemInstanceID: id.NewV4(), DeliveredInstanceID: id.NewV4(),
			ItemID: "item.potion.hp", Quantity: 10,
		}})
	seedAuction(t, sc, src, c, acct,
		time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC), 500, 480, 5)
	// totals: s1 40 + sc 15 = 55 >= 50; top3 covers all -> qualified.
	preds := evalAll(t, s, acct, at)
	if !preds["item_concentration_30d"].Qualified {
		t.Fatalf("item concentration not qualified: %+v",
			preds["item_concentration_30d"])
	}

	// Negative: >= 50 units but the top-3 share is under 0.80 (4
	// equal sources -> 45/60 = 0.75).
	b := mkAccount(t)
	bc := mkCharacter(t, b, 60, 72*time.Hour)
	q1, q2, q3, q4 := id.NewV4(), id.NewV4(), id.NewV4(), id.NewV4()
	seedCharRollup(t, bc, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
		0, 0, nil, map[string]int64{
			q1.String(): 15, q2.String(): 15,
			q3.String(): 15, q4.String(): 15})
	if evalAll(t, s, b, at)["item_concentration_30d"].Qualified {
		t.Fatal("diffuse item sources qualified")
	}
}

// TestRestartPersistence asserts the read path is purely the
// persistent aggregation tables + raw records: a fresh Store over the
// same database sees identical predicate outcomes.
func TestRestartPersistence(t *testing.T) {
	acct := mkAccount(t)
	flagTrigger(t, acct)
	at := evalT()
	s1 := New(pool(t))
	first := evalAll(t, s1, acct, at)
	// "Restart": new Store instance, new transaction — totals must be
	// unchanged because nothing lives in memory.
	s2 := New(pool(t))
	second := evalAll(t, s2, acct, at)
	for name, p := range first {
		if second[name].Qualified != p.Qualified {
			t.Fatalf("%s outcome changed across restart: %v -> %v",
				name, p.Qualified, second[name].Qualified)
		}
	}
}

// TestTradeGateLevelAge asserts the COMMITTING re-validation of the
// direct-trade gates (ADR-0041): level >= 10 and character age >= 24h
// read from the characters row — the wire never carries either.
func TestTradeGateLevelAge(t *testing.T) {
	st := trade.New(pool(t))
	cpart := mkAccount(t)
	cpartChar := mkCharacter(t, cpart, 60, 72*time.Hour)
	mkWallet(t, cpartChar, 0)

	run := func(init participant) *journalv1.JournalOutcome {
		t.Helper()
		tradeID, settleID, opID := id.NewV4(), id.NewV4(), id.NewV7(time.Now())
		j := tradeJournal(tradeID, settleID, opID, init,
			participant{cpart, cpartChar}, 1000, 0)
		mkWallet(t, init.char, 10_000)
		return submitFinalise(t, st, init, j)
	}

	// Level 9 rejects TRADE_ELIGIBILITY_LEVEL_REQUIRED.
	acct9 := mkAccount(t)
	l9 := mkCharacter(t, acct9, 9, 72*time.Hour)
	if r := run(participant{acct9, l9}); r.GetErrorCode() !=
		protocolv1.ErrorCode_ERROR_CODE_TRADE_ELIGIBILITY_LEVEL_REQUIRED {
		t.Fatalf("L9 code %v", r.GetErrorCode())
	}
	// Level 10 permits (age satisfied).
	acct10 := mkAccount(t)
	l10 := mkCharacter(t, acct10, 10, 25*time.Hour)
	if r := run(participant{acct10, l10}); r.GetStatus() !=
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("L10 trade status %v code %v", r.GetStatus(), r.GetErrorCode())
	}
	// 23-hour character age rejects TRADE_ELIGIBILITY_AGE_REQUIRED even
	// at level 10 — the gate reads character age, never account age.
	acctY := mkAccount(t)
	young := mkCharacter(t, acctY, 10, 23*time.Hour)
	if r := run(participant{acctY, young}); r.GetErrorCode() !=
		protocolv1.ErrorCode_ERROR_CODE_TRADE_ELIGIBILITY_AGE_REQUIRED {
		t.Fatalf("23h code %v", r.GetErrorCode())
	}
	// Forged client-declared level: nothing in the wire carries level —
	// the DB row (level 9) is authoritative and still rejects. Same
	// character, same request shape => identical rejection.
	if r := run(participant{acct9, l9}); r.GetErrorCode() !=
		protocolv1.ErrorCode_ERROR_CODE_TRADE_ELIGIBILITY_LEVEL_REQUIRED {
		t.Fatalf("forged L9 code %v", r.GetErrorCode())
	}
}

// TestAuctionGateLevel asserts ADR-0063 all-ops level >= 15: a level-10
// character — eligible for direct trade — still rejects AH listing
// with the canonical auction level gate.
func TestAuctionGateLevel(t *testing.T) {
	lookup := func(_ context.Context, itemID string) (*auction.LookupItem, error) {
		if itemID == "item.potion.hp" {
			return &auction.LookupItem{ItemID: itemID,
				Category: "CONSUMABLE", Tradable: true}, nil
		}
		return nil, errors.New("unknown item")
	}
	s := auction.New(pool(t), items.New(pool(t)), lookup)
	acct := mkAccount(t)
	l10 := mkCharacter(t, acct, 10, 72*time.Hour)
	mkWallet(t, l10, 10_000)
	inst := id.NewV4()
	if _, err := pool(t).Exec(ctx0(),
		`INSERT INTO item_instances
		 (item_instance_id, item_id, quantity, effective_binding,
		  enhancement_level, item_state, created_at)
		 VALUES ($1,'item.potion.hp',1,'UNBOUND',0,'{}'::jsonb,NOW())`,
		inst.String()); err != nil {
		t.Fatalf("instance: %v", err)
	}
	err := func() error {
		tx, e := pool(t).Begin(ctx0())
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback(ctx0()) }()
		_, err := s.List(ctx0(), tx, auction.ListIn{
			OperationID: id.NewV4(), CharacterID: l10, AccountID: acct,
			ItemInstanceID: inst, Quantity: 1, PriceCommon: 100})
		return err
	}()
	if !errors.Is(err, auction.ErrLevelGate) {
		t.Fatalf("L10 listing err=%v, want ErrLevelGate", err)
	}
}
