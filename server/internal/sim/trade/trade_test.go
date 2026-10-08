package trade

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// fakePresence is the test Presence: a fixed map of participants.
type fakePresence map[id.UUID]ParticipantView

func (f fakePresence) Participant(c id.UUID) (ParticipantView, bool) {
	v, ok := f[c]
	return v, ok
}
func (f fakePresence) CanDirectInteract(a, b id.UUID) error { return nil }

// fakeStacks is the test StackReader: instance → owner/item/qty.
type fakeStacks map[id.UUID]struct {
	owner  id.UUID
	itemID string
	qty    int
}

func (f fakeStacks) lookup(iid id.UUID) (id.UUID, string, int, bool) {
	if s, ok := f[iid]; ok {
		return s.owner, s.itemID, s.qty, true
	}
	return id.UUID{}, "", 0, false
}

func stubSnapshot(_ id.UUID, o Offer) *journalv1.JournalItem {
	return &journalv1.JournalItem{
		ItemInstanceId: o.ItemInstanceID[:], ItemId: o.ItemID, Quantity: uint64(o.Quantity),
	}
}

func view(acct id.UUID, mapID string, x, y float64) ParticipantView {
	return ParticipantView{AccountID: acct, Level: 30, Age: 72 * time.Hour, MapID: mapID, X: x, Y: y, Online: true}
}

func mgr(pres Presence, stacks StackReader) *Manager {
	return New(Deps{Presence: pres, Stacks: stacks, Snapshots: stubSnapshot})
}

func offerItem(iid id.UUID, q int) *protocolv1.TradeOfferItem {
	return &protocolv1.TradeOfferItem{ItemInstanceId: iid[:], Quantity: uint32(q)}
}

// pair runs invite+accept to an OPEN session and returns the trade id.
func pair(t *testing.T, m *Manager, a, b id.UUID) id.UUID {
	t.Helper()
	res, emits := m.Invite(id.NewV4(), a, b)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("invite: %v", res.GetResult().GetErrorCode())
	}
	if len(emits) != 1 {
		t.Fatalf("invite emits %d want 1 (S2C invite)", len(emits))
	}
	var tradeID id.UUID
	copy(tradeID[:], res.GetTradeId())
	res2, emits2 := m.Accept(id.NewV4(), b, tradeID)
	if res2.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("accept: %v", res2.GetResult().GetErrorCode())
	}
	if len(emits2) != 2 {
		t.Fatalf("accept emits %d want 2 (706 both)", len(emits2))
	}
	return tradeID
}

// TestSameMapDistanceFourMetersCheck: invite rejects beyond 4 m or on
// a different map; at ≤4 m it admits.
func TestSameMapDistanceFourMetersCheck(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	acctA, acctB := id.NewV4(), id.NewV4()
	// 4.1 m apart → OUT_OF_RANGE
	m := mgr(fakePresence{
		a: view(acctA, "map.a", 0, 0), b: view(acctB, "map.a", 4.1, 0),
	}, fakeStacks{}.lookup)
	res, _ := m.Invite(id.NewV4(), a, b)
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE {
		t.Fatalf("code %v want OUT_OF_RANGE", res.GetResult().GetErrorCode())
	}
	// different map → OUT_OF_RANGE
	m = mgr(fakePresence{
		a: view(acctA, "map.a", 0, 0), b: view(acctB, "map.b", 0, 0),
	}, fakeStacks{}.lookup)
	res, _ = m.Invite(id.NewV4(), a, b)
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE {
		t.Fatalf("code %v want OUT_OF_RANGE", res.GetResult().GetErrorCode())
	}
	// exactly 4 m → admitted
	m = mgr(fakePresence{
		a: view(acctA, "map.a", 0, 0), b: view(acctB, "map.a", 3, 1.999),
	}, fakeStacks{}.lookup)
	res, emits := m.Invite(id.NewV4(), a, b)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("invite: %v", res.GetResult().GetErrorCode())
	}
	if len(emits) != 1 {
		t.Fatalf("emits %d", len(emits))
	}
}

// TestTwoPlayerAtomicExchange: the golden path — open, offers, lock,
// finalise journal, partner 709 on settle.
func TestTwoPlayerAtomicExchange(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	acctA, acctB := id.NewV4(), id.NewV4()
	iidA, iidB := id.NewV4(), id.NewV4()
	stacks := fakeStacks{
		iidA: {owner: a, itemID: "item.sword", qty: 1},
		iidB: {owner: b, itemID: "item.shield", qty: 3},
	}
	m := mgr(fakePresence{
		a: view(acctA, "map.a", 0, 0), b: view(acctB, "map.a", 1, 1),
	}, stacks.lookup)
	tradeID := pair(t, m, a, b)

	res, emits := m.OfferUpdate(id.NewV4(), a, tradeID, 0,
		[]*protocolv1.TradeOfferItem{offerItem(iidA, 1)}, 0)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("offer a: %v", res.GetResult().GetErrorCode())
	}
	if len(emits) != 2 {
		t.Fatalf("offer emits %d", len(emits))
	}
	res, _ = m.OfferUpdate(id.NewV4(), b, tradeID, 1,
		[]*protocolv1.TradeOfferItem{offerItem(iidB, 2)}, 100)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("offer b: %v", res.GetResult().GetErrorCode())
	}
	// both confirm → LOCKED
	if res, emits = m.Confirm(id.NewV4(), a, tradeID, 2); res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("confirm a: %v", res.GetResult().GetErrorCode())
	} else if st := emits[0].Msg.(*protocolv1.S2CTradeOfferState).GetState(); st != protocolv1.TradeOfferState_TRADE_OFFER_STATE_OPEN {
		t.Fatalf("state after one confirm %v", st)
	}
	res, emits = m.Confirm(id.NewV4(), b, tradeID, 2)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("confirm b: %v", res.GetResult().GetErrorCode())
	}
	if st := emits[0].Msg.(*protocolv1.S2CTradeOfferState).GetState(); st != protocolv1.TradeOfferState_TRADE_OFFER_STATE_LOCKED {
		t.Fatalf("state %v want LOCKED", st)
	}
	// finalise → COMMITTING journal
	j, res, emits := m.Finalise(id.NewV4(), a, tradeID, 2)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("finalise: %v", res.GetResult().GetErrorCode())
	}
	if j == nil || len(j.GetInitiator().GetItems()) != 1 || len(j.GetCounterpart().GetItems()) != 1 {
		t.Fatalf("journal sides wrong: %+v", j)
	}
	if j.GetCounterpart().GetCommonAmount() != 100 {
		t.Fatalf("counterpart common %d", j.GetCounterpart().GetCommonAmount())
	}
	if j.GetFeeCommon() != 5 {
		t.Fatalf("fee %d want 5", j.GetFeeCommon())
	}
	if len(j.GetSettlementId()) != 16 || len(j.GetInitiatingClientOperationId()) != 16 {
		t.Fatalf("journal ids missing")
	}
	if st := emits[0].Msg.(*protocolv1.S2CTradeOfferState).GetState(); st != protocolv1.TradeOfferState_TRADE_OFFER_STATE_COMMITTING {
		t.Fatalf("state %v want COMMITTING", st)
	}
	// settle → partner 709 (b offered; a committed)
	var sid id.UUID
	copy(sid[:], j.GetSettlementId())
	emits = m.Settled(tradeID, sid, protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED)
	if len(emits) != 1 {
		t.Fatalf("settled emits %d want 1", len(emits))
	}
	res709 := emits[0].Msg.(*protocolv1.S2CTradeResult)
	if emits[0].To != b {
		t.Fatalf("partner 709 to %v want %v", emits[0].To, b)
	}
	if len(res709.GetResult().GetOperationId()) != 0 {
		t.Fatalf("partner operation_id must be empty")
	}
	// partner b receives a's offer: the sword, no common, no fee.
	if res709.GetCommonReceived() != 0 || res709.GetFee() != 0 {
		t.Fatalf("partner common %d fee %d", res709.GetCommonReceived(), res709.GetFee())
	}
	if len(res709.GetReceived()) != 1 || res709.GetReceived()[0].GetItemId() != "item.sword" {
		t.Fatalf("partner received %+v", res709.GetReceived())
	}
	if m.Len() != 0 {
		t.Fatalf("session still tracked")
	}
}

// TestTradeLockInPlace: offered stacks stay in CHARACTER_INVENTORY
// under the runtime lock — a custody move on a locked stack rejects.
func TestTradeLockInPlace(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	iid := id.NewV4()
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, fakeStacks{iid: {owner: a, itemID: "item.x", qty: 5}}.lookup)
	tradeID := pair(t, m, a, b)
	if res, _ := m.OfferUpdate(id.NewV4(), a, tradeID, 0,
		[]*protocolv1.TradeOfferItem{offerItem(iid, 5)}, 0); res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("offer: %v", res.GetResult().GetErrorCode())
	}
	l := m.Ledger(a)
	if l == nil || l.LockedQty(iid) != 5 {
		t.Fatalf("ledger locked %v", l)
	}
}

// TestTradeLockInPlaceSettlement: with an offered full stack the
// durable layer asserts the item still sits in the inventory — the
// lock is custody-preserving until settlement.
func TestTradeLockInPlaceSettlement(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	iid := id.NewV4()
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, fakeStacks{iid: {owner: a, itemID: "item.x", qty: 3}}.lookup)
	tradeID := pair(t, m, a, b)
	m.OfferUpdate(id.NewV4(), a, tradeID, 0, []*protocolv1.TradeOfferItem{offerItem(iid, 3)}, 0)
	m.OfferUpdate(id.NewV4(), b, tradeID, 1, nil, 50)
	m.Confirm(id.NewV4(), a, tradeID, 2)
	m.Confirm(id.NewV4(), b, tradeID, 2)
	j, res, _ := m.Finalise(id.NewV4(), a, tradeID, 2)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS || j == nil {
		t.Fatalf("finalise: %v", res.GetResult().GetErrorCode())
	}
	if got := j.GetInitiator().GetItems()[0].GetQuantity(); got != 3 {
		t.Fatalf("journal qty %d", got)
	}
}

// TestTradeRestartCancelsSessions: process restart drops every open
// session and releases every lock — value-free.
func TestTradeRestartCancelsSessions(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	iid := id.NewV4()
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, fakeStacks{iid: {owner: a, itemID: "item.x", qty: 5}}.lookup)
	tradeID := pair(t, m, a, b)
	m.OfferUpdate(id.NewV4(), a, tradeID, 0, []*protocolv1.TradeOfferItem{offerItem(iid, 2)}, 0)
	if m.LockedQuantity(a, iid) != 2 {
		t.Fatalf("locked %d", m.LockedQuantity(a, iid))
	}
	m.Restart()
	if m.Len() != 0 {
		t.Fatalf("sessions left after restart")
	}
	if m.LockedQuantity(a, iid) != 0 {
		t.Fatalf("lock not released")
	}
}

// TestTradeSameAccountForbidden: two characters on one account never
// enter a trade session.
func TestTradeSameAccountForbidden(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	acct := id.NewV4()
	m := mgr(fakePresence{
		a: view(acct, "map.a", 0, 0), b: view(acct, "map.a", 0, 1),
	}, fakeStacks{}.lookup)
	res, _ := m.Invite(id.NewV4(), a, b)
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_SAME_ACCOUNT_FORBIDDEN {
		t.Fatalf("code %v want SAME_ACCOUNT_FORBIDDEN", res.GetResult().GetErrorCode())
	}
}
