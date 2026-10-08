package trade

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// buildJournal assembles the two-sided journal a LOCKED session embeds
// — initiator offers `initItems`/`initCommon`, counterpart mirrors.
func buildJournal(tradeID, settlementID, opID id.UUID,
	init, cpart participant, initItems, cpartItems []*journalv1.JournalItem,
	initCommon, cpartCommon int64) *journalv1.JournalTrade {
	return &journalv1.JournalTrade{
		TradeId:          tradeID[:],
		ExpectedRevision: 3,
		Initiator: &journalv1.JournalTradeSide{
			CharacterId: init.char[:], AccountId: init.account[:],
			Items: initItems, CommonAmount: initCommon,
		},
		Counterpart: &journalv1.JournalTradeSide{
			CharacterId: cpart.char[:], AccountId: cpart.account[:],
			Items: cpartItems, CommonAmount: cpartCommon,
		},
		SettlementId:                settlementID[:],
		FeeCommon:                   feeOf(initCommon) + feeOf(cpartCommon),
		FinalizedAtMs:               time.Now().UnixMilli(),
		InitiatingClientOperationId: opID[:],
	}
}

// submitFinalise drives the durable path the edge exposes: the
// CLIENT708 record → TrustedReplay through the family executor.
func submitFinalise(t *testing.T, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, store *Store, idem *idempotency.Store,
	locks func(id.UUID) *items.TradeLockLedger,
	p participant, tradeID, settlementID id.UUID,
	j *journalv1.JournalTrade) *journalv1.JournalOutcome {
	t.Helper()
	// the committing operation id is the journal's embedded client op.
	var opID id.UUID
	copy(opID[:], j.GetInitiatingClientOperationId())
	req := &protocolv1.C2STradeFinalise{
		OperationId:      opID[:],
		TradeId:          tradeID[:],
		ExpectedRevision: j.GetExpectedRevision(),
	}
	rec, err := FinaliseRecord(p.account, 7, 11, p.char, req, j, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	ex := Executors(Deps{Store: store, Locks: locks})[FinaliseFamily]
	out, err := idem.TrustedReplay(ctx0(), FinaliseFamily,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: p.char}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return ex(ctx, tx, rec)
		})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return decodeOutcome(t, out)
}

func ctx0() context.Context { return context.Background() }

// TestAdmittedFinalizationKeepsOriginalClientEvidenceAcrossRestart
// exercises the replay path: the same committed record replays the
// retained outcome — original operation_id/request/epochs — after a
// "restart" (a second TrustedReplay of the same record).
func TestAdmittedFinalizationKeepsOriginalClientEvidenceAcrossRestart(t *testing.T) {
	pool := newPool(t)
	store, idem := New(pool), idempotency.NewStore(pool)
	init := seedParticipant(t, pool, 1000)
	cpart := seedParticipant(t, pool, 0)
	iid := seedStack(t, pool, init.char, "item.sword.t1", 2, "inv.0")
	tradeID, settlementID := id.NewV4(), id.NewV4()
	opID := id.NewV7(time.Now())
	j := buildJournal(tradeID, settlementID, opID, init, cpart,
		[]*journalv1.JournalItem{journalItem(iid, "item.sword.t1", 2)}, nil, 0, 0)
	req := &protocolv1.C2STradeFinalise{OperationId: opID[:], TradeId: tradeID[:], ExpectedRevision: 3}
	rec, err := FinaliseRecord(init.account, 7, 11, init.char, req, j, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	locks := lockedLedger(map[id.UUID]map[id.UUID][2]int{init.char: {iid: {2, 2}}})
	ex := Executors(Deps{Store: store, Locks: locks})[FinaliseFamily]
	exec := func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
		return ex(ctx, tx, rec)
	}
	owner := idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: init.char}
	out1, err := idem.TrustedReplay(ctx0(), FinaliseFamily, owner, opID, fp, exec)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	o1 := decodeOutcome(t, out1)
	if o1.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("first status %v code %v", o1.GetStatus(), o1.GetErrorCode())
	}
	// "Restart": replay the identical record — the retained outcome
	// must carry the original client evidence (op id, family owner).
	out2, err := idem.TrustedReplay(ctx0(), FinaliseFamily, owner, opID, fp, exec)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	o2 := decodeOutcome(t, out2)
	if o2.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("replay status %v", o2.GetStatus())
	}
	got := o2.GetS2CTradeResult().GetResult().GetOperationId()
	if string(got) != string(opID[:]) {
		t.Fatalf("replayed operation_id changed: %x want %x", got, opID)
	}
	if o2.GetS2CTradeResult().GetTradeId() == nil ||
		string(o2.GetS2CTradeResult().GetTradeId()) != string(tradeID[:]) {
		t.Fatalf("replayed trade_id mismatch")
	}
}

// TestTradeRequestResultPerRequest: every submitted finalise resolves
// to exactly one outcome carrying that request's operation id — no
// request is answered by another's result.
func TestTradeRequestResultPerRequest(t *testing.T) {
	pool := newPool(t)
	store, idem := New(pool), idempotency.NewStore(pool)
	init := seedParticipant(t, pool, 0)
	cpart := seedParticipant(t, pool, 0)
	opA, opB := id.NewV7(time.Now()), id.NewV7(time.Now().Add(time.Millisecond))
	tradeA, tradeB := id.NewV4(), id.NewV4()
	ja := buildJournal(tradeA, id.NewV4(), opA, init, cpart, nil, nil, 0, 0)
	jb := buildJournal(tradeB, id.NewV4(), opB, init, cpart, nil, nil, 0, 0)
	ra, err := FinaliseRecord(init.account, 7, 11, init.char,
		&protocolv1.C2STradeFinalise{OperationId: opA[:], TradeId: tradeA[:], ExpectedRevision: 3}, ja, time.Now())
	if err != nil {
		t.Fatalf("rec a: %v", err)
	}
	rb, err := FinaliseRecord(init.account, 7, 11, init.char,
		&protocolv1.C2STradeFinalise{OperationId: opB[:], TradeId: tradeB[:], ExpectedRevision: 3}, jb, time.Now())
	if err != nil {
		t.Fatalf("rec b: %v", err)
	}
	owner := idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: init.char}
	ex := Executors(Deps{Store: store, Locks: lockedLedger(nil)})[FinaliseFamily]
	run := func(rec *journalv1.DurableCommandRecord, op id.UUID) *journalv1.JournalOutcome {
		var fp [32]byte
		copy(fp[:], rec.GetRequestFingerprint())
		out, err := idem.TrustedReplay(ctx0(), FinaliseFamily, owner, op, fp,
			func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) { return ex(ctx, tx, rec) })
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		return decodeOutcome(t, out)
	}
	oa, ob := run(ra, opA), run(rb, opB)
	if string(oa.GetOperationId()) != string(opA[:]) || string(ob.GetOperationId()) != string(opB[:]) {
		t.Fatalf("results not one-per-request: %x / %x", oa.GetOperationId(), ob.GetOperationId())
	}
	if string(oa.GetS2CTradeResult().GetTradeId()) != string(tradeA[:]) ||
		string(ob.GetS2CTradeResult().GetTradeId()) != string(tradeB[:]) {
		t.Fatalf("trade ids crossed between requests")
	}
}

// TestTradeResultOperationId: the committing side's 709 carries the
// 708 operation_id; the partner's carries an empty one (delivered by
// the session's Settled path — verified in sim tests).
func TestTradeResultOperationId(t *testing.T) {
	pool := newPool(t)
	init := seedParticipant(t, pool, 500)
	cpart := seedParticipant(t, pool, 500)
	tradeID, settlementID := id.NewV4(), id.NewV4()
	opID := id.NewV7(time.Now())
	j := buildJournal(tradeID, settlementID, opID, init, cpart, nil, nil, 100, 0)
	out := submitFinalise(t, pool, New(pool), idempotency.NewStore(pool),
		lockedLedger(nil), init, tradeID, settlementID, j)
	res := out.GetS2CTradeResult()
	if res == nil {
		t.Fatalf("missing S2CTradeResult")
	}
	if string(res.GetResult().GetOperationId()) != string(opID[:]) {
		t.Fatalf("operation_id %x want %x", res.GetResult().GetOperationId(), opID)
	}
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status %v code %v", res.GetResult().GetStatus(), res.GetResult().GetErrorCode())
	}
}

// TestLockedQuantityPartialStack: a partial-stack offer commits a
// split — the source stack keeps the remainder and the receiver gets
// a fresh instance holding exactly the offered quantity.
func TestLockedQuantityPartialStack(t *testing.T) {
	pool := newPool(t)
	init := seedParticipant(t, pool, 0)
	cpart := seedParticipant(t, pool, 0)
	iid := seedStack(t, pool, init.char, "item.potion.hp", 10, "inv.0")
	tradeID := id.NewV4()
	j := buildJournal(tradeID, id.NewV4(), id.NewV4(), init, cpart,
		[]*journalv1.JournalItem{journalItem(iid, "item.potion.hp", 4)}, nil, 0, 0)
	locks := lockedLedger(map[id.UUID]map[id.UUID][2]int{init.char: {iid: {4, 10}}})
	out, err := runSettle(t, pool, SettleIn{Trade: j, OperationID: id.NewV4(), Locks: locks})
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if q := stackQty(t, pool, iid); q != 6 {
		t.Fatalf("source stack %d want 6", q)
	}
	var delivered int
	for _, tr := range out.ItemTransfers {
		if tr.ItemInstanceID == iid {
			delivered = tr.Quantity
			if tr.DeliveredInstanceID == tr.ItemInstanceID {
				t.Fatalf("partial split must mint a delivered instance")
			}
			owner, ok := ownerOf(t, pool, tr.DeliveredInstanceID)
			if !ok || owner != cpart.char {
				t.Fatalf("delivered instance owner %v ok=%v", owner, ok)
			}
			if q := stackQty(t, pool, tr.DeliveredInstanceID); q != 4 {
				t.Fatalf("delivered qty %d want 4", q)
			}
		}
	}
	if delivered != 4 {
		t.Fatalf("delivered %d want 4", delivered)
	}
}
