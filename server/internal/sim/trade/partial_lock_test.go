package trade

import (
	"testing"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestPartialStackLocksOfferedQuantity: offering q < n locks exactly
// q — the remainder stays free (ADR-0062).
func TestPartialStackLocksOfferedQuantity(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	iid := id.NewV4()
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, fakeStacks{iid: {owner: a, itemID: "item.potion", qty: 10}}.lookup)
	tradeID := pair(t, m, a, b)
	res, _ := m.OfferUpdate(id.NewV4(), a, tradeID, 0,
		[]*protocolv1.TradeOfferItem{offerItem(iid, 4)}, 0)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("offer: %v", res.GetResult().GetErrorCode())
	}
	if q := m.LockedQuantity(a, iid); q != 4 {
		t.Fatalf("locked %d want 4", q)
	}
}

// TestRemainderUsableLockedQuantityProtected: the free remainder
// supports OpReduce down to the locked quantity; reducing below the
// locked amount rejects.
func TestRemainderUsableLockedQuantityProtected(t *testing.T) {
	l := items.NewTradeLockLedger(id.NewV4())
	iid := id.NewV4()
	if err := l.Offer(iid, 3, 10); err != nil {
		t.Fatalf("offer: %v", err)
	}
	// reduce 7 → leaves 3 (= locked) allowed
	if err := l.AssertOpAllowed(iid, 10, items.OpReduce, 7); err != nil {
		t.Fatalf("reduce to locked rejected: %v", err)
	}
	// reduce 8 → leaves 2 < 3 locked → rejected
	if err := l.AssertOpAllowed(iid, 10, items.OpReduce, 8); err == nil {
		t.Fatalf("reduce below locked allowed")
	}
}

// TestMergeIntoLockedStackRejected: merging into a locked stack is
// always banned.
func TestMergeIntoLockedStackRejected(t *testing.T) {
	l := items.NewTradeLockLedger(id.NewV4())
	iid := id.NewV4()
	if err := l.Offer(iid, 2, 5); err != nil {
		t.Fatalf("offer: %v", err)
	}
	if err := l.AssertOpAllowed(iid, 5, items.OpMergeInto, 1); err == nil {
		t.Fatalf("merge into locked stack allowed")
	}
	if err := l.AssertOpAllowed(iid, 5, items.OpCustodyMove, 5); err == nil {
		t.Fatalf("custody move on locked stack allowed")
	}
}

// TestSessionCancelledOnParticipantTransferOrDeath: participant map
// transfer / respawn / instance change / death cancels the session —
// release only, both sides see CANCELLED.
func TestSessionCancelledOnParticipantTransferOrDeath(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	iid := id.NewV4()
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, fakeStacks{iid: {owner: a, itemID: "item.x", qty: 2}}.lookup)
	tradeID := pair(t, m, a, b)
	m.OfferUpdate(id.NewV4(), a, tradeID, 0, []*protocolv1.TradeOfferItem{offerItem(iid, 2)}, 0)
	emits := m.CancelOnParticipantEvent(a)
	if len(emits) != 2 {
		t.Fatalf("cancel emits %d want 2", len(emits))
	}
	if r := emits[0].Msg.(*protocolv1.S2CTradeCancelled).GetReason(); r != protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_CANCELLED {
		t.Fatalf("reason %v want CANCELLED", r)
	}
	if m.Len() != 0 {
		t.Fatalf("session left")
	}
	if q := m.LockedQuantity(a, iid); q != 0 {
		t.Fatalf("lock not released")
	}
}

// TestDisconnectCancelsForPartner: a disconnect cancels with
// PARTNER_DISCONNECTED on the remaining side only.
func TestDisconnectCancelsForPartner(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, fakeStacks{}.lookup)
	pair(t, m, a, b)
	emits := m.Disconnected(a)
	if len(emits) != 1 {
		t.Fatalf("disconnect emits %d want 1", len(emits))
	}
	if emits[0].To != b {
		t.Fatalf("partner emit to %v", emits[0].To)
	}
	if r := emits[0].Msg.(*protocolv1.S2CTradeCancelled).GetReason(); r != protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_PARTNER_DISCONNECTED {
		t.Fatalf("reason %v want PARTNER_DISCONNECTED", r)
	}
}
