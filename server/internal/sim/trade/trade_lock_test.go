package trade

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestSameAccountTradeForbidden (lock-matrix coverage): same-account
// invites are rejected before any ledger is touched.
func TestSameAccountTradeForbidden(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	acct := id.NewV4()
	m := mgr(fakePresence{
		a: view(acct, "map.a", 0, 0), b: view(acct, "map.a", 1, 0),
	}, fakeStacks{}.lookup)
	res, _ := m.Invite(id.NewV4(), a, b)
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_SAME_ACCOUNT_FORBIDDEN {
		t.Fatalf("code %v", res.GetResult().GetErrorCode())
	}
	if m.Len() != 0 {
		t.Fatalf("session created")
	}
}

// TestTwelveEntriesPerSide: at most 12 item entries per side.
func TestTwelveEntriesPerSide(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	stacks := fakeStacks{}
	var offers []*protocolv1.TradeOfferItem
	for i := 0; i < MaxOffersPerSide; i++ {
		iid := id.NewV4()
		stacks[iid] = struct {
			owner  id.UUID
			itemID string
			qty    int
		}{owner: a, itemID: "item.x", qty: 1}
		offers = append(offers, offerItem(iid, 1))
	}
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, stacks.lookup)
	tradeID := pair(t, m, a, b)
	// 12 entries → admitted
	res, _ := m.OfferUpdate(id.NewV4(), a, tradeID, 0, offers, 0)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("12 entries rejected: %v", res.GetResult().GetErrorCode())
	}
	// 13th entry → rejected
	iid13 := id.NewV4()
	stacks[iid13] = struct {
		owner  id.UUID
		itemID string
		qty    int
	}{owner: a, itemID: "item.x", qty: 1}
	res, _ = m.OfferUpdate(id.NewV4(), a, tradeID, 1,
		append(offers, offerItem(iid13, 1)), 0)
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT {
		t.Fatalf("13th entry code %v", res.GetResult().GetErrorCode())
	}
}

// TestRestartReleasesTradeLocks: restart drops the session and frees
// the locked stack for other operations.
func TestRestartReleasesTradeLocks(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	iid := id.NewV4()
	m := mgr(fakePresence{
		a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
	}, fakeStacks{iid: {owner: a, itemID: "item.x", qty: 8}}.lookup)
	tradeID := pair(t, m, a, b)
	m.OfferUpdate(id.NewV4(), a, tradeID, 0, []*protocolv1.TradeOfferItem{offerItem(iid, 8)}, 0)
	if q := m.LockedQuantity(a, iid); q != 8 {
		t.Fatalf("locked %d want 8", q)
	}
	m.Restart()
	if l := m.Ledger(a); l != nil {
		t.Fatalf("ledger leaked after restart")
	}
}

// TestInviteExpiryTimesOut: a pending invite expires through Tick.
func TestInviteExpiryTimesOut(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	base := time.Now()
	clock := &nowStub{t: base}
	m := New(Deps{
		Presence: fakePresence{
			a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
		},
		Stacks:    fakeStacks{}.lookup,
		Snapshots: stubSnapshot,
	}).WithClock(clock.now)
	res, _ := m.Invite(id.NewV4(), a, b)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("invite: %v", res.GetResult().GetErrorCode())
	}
	var tradeID id.UUID
	copy(tradeID[:], res.GetTradeId())
	clock.t = base.Add(61 * time.Second)
	emits := m.Tick()
	if len(emits) != 2 {
		t.Fatalf("expiry emits %d want 2", len(emits))
	}
	if r := emits[0].Msg.(*protocolv1.S2CTradeCancelled).GetReason(); r != protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_EXPIRED {
		t.Fatalf("reason %v want EXPIRED", r)
	}
	if m.Len() != 0 {
		t.Fatalf("pending left")
	}
}

// TestIdleTimeoutCancels: an OPEN session idle 120s is cancelled with
// INACTIVE_TIMEOUT to both sides.
func TestIdleTimeoutCancels(t *testing.T) {
	a, b := id.NewV4(), id.NewV4()
	base := time.Now()
	clock := &nowStub{t: base}
	m := New(Deps{
		Presence: fakePresence{
			a: view(id.NewV4(), "map.a", 0, 0), b: view(id.NewV4(), "map.a", 0, 1),
		},
		Stacks:    fakeStacks{}.lookup,
		Snapshots: stubSnapshot,
	}).WithClock(clock.now)
	pair(t, m, a, b)
	clock.t = base.Add(121 * time.Second)
	emits := m.Tick()
	if len(emits) != 2 {
		t.Fatalf("idle emits %d want 2", len(emits))
	}
	if r := emits[0].Msg.(*protocolv1.S2CTradeCancelled).GetReason(); r != protocolv1.TradeCancelReason_TRADE_CANCEL_REASON_INACTIVE_TIMEOUT {
		t.Fatalf("reason %v", r)
	}
}

type nowStub struct{ t time.Time }

func (n *nowStub) now() time.Time { return n.t }
