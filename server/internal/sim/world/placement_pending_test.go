package world

import (
	"testing"
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// Saturate one map: every channel running at HardCap.
func saturate(e *testEnv, t *testing.T, mapID string) {
	t.Helper()
	for i := 1; i <= ChannelsPerMap; i++ {
		c, _ := e.w.director.Channel(mapID, uint32(i))
		c.State = ChannelRunning
		c.Occupancy = HardCap
	}
}

func TestPlacementPendingThenPlaced(t *testing.T) {
	// Saturated forced attach → 15 PLACEMENT_PENDING → occupancy frees →
	// the retry cadence resolves the wait and the resolve callback
	// re-admits the character.
	e := newTestEnv(t, testMaps())
	saturate(e, t, "map.test.alpha")

	cid := charID(1)
	resolved := make(chan PlacementResult, 1)
	_, pw, err := e.w.PlaceAttach(cid, "map.test.alpha",
		"spawn.entry.test.alpha", 0,
		func(w *Runtime, r PlacementResult) { resolved <- r })
	if err != nil {
		t.Fatalf("PlaceAttach: %v", err)
	}
	if pw == nil {
		t.Fatal("no PendingWait on saturated attach")
	}
	waitFor(t, "15 emitted", func() bool {
		return len(e.out.forChar(cid, MsgS2CPlacementPending)) > 0
	})

	// Free a slot below the hard cap; the next retry resolves.
	c, _ := e.w.director.Channel("map.test.alpha", 1)
	c.Occupancy = HardCap - 1
	e.clocks.Advance(6 * time.Second)
	e.w.TickOnce()

	select {
	case r := <-resolved:
		if r.ChannelIndex != 1 || r.MapID != "map.test.alpha" {
			t.Fatalf("resolved placement wrong: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pending never resolved")
	}
	// Pending removed from the ledger.
	if _, ok := e.w.director.Pending(cid); ok {
		t.Fatal("resolved pending still in ledger")
	}
}

func TestPendingRetryCadence(t *testing.T) {
	// Between emitted 15s the pending waits retry_after_ms before
	// re-emitting; TickOnce inside the window must not re-emit.
	e := newTestEnv(t, testMaps())
	saturate(e, t, "map.test.alpha")
	cid := charID(1)
	_, pw, err := e.w.PlaceAttach(cid, "map.test.alpha",
		"spawn.entry.test.alpha", 0, nil)
	if err != nil || pw == nil {
		t.Fatalf("pending attach: %v %v", err, pw)
	}
	waitFor(t, "15 emitted", func() bool {
		return len(e.out.forChar(cid, MsgS2CPlacementPending)) > 0
	})
	e.w.TickOnce()
	if n := len(e.out.forChar(cid, MsgS2CPlacementPending)); n != 1 {
		t.Fatalf("retry inside window re-emitted: %d pending notices", n)
	}
	e.clocks.Advance(6 * time.Second)
	e.w.TickOnce()
	if n := len(e.out.forChar(cid, MsgS2CPlacementPending)); n != 2 {
		t.Fatalf("retry past window: want 2 notices, got %d", n)
	}
}

func TestPendingRespawnStaysDead(t *testing.T) {
	// ADR-0082: a dead member respawned into a saturated map pends
	// instead of reviving — the member must stay dead and no 207 fires
	// until the wait resolves.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	e.killMember(t, cid)

	// Re-point the durable checkpoint at beta and saturate it: the
	// respawn placement must pend.
	e.loader.cpMap = "map.test.beta"
	e.loader.cpID = "checkpoint.test.beta"
	e.loader.cpAnchr = "checkpoint.test.beta"
	saturate(e, t, "map.test.beta")

	var op [16]byte
	op[0] = 0x88
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdRespawn, CharacterID: cid, OperationID: op,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "pending 15 for 208", func() bool {
		for _, m := range e.out.forChar(cid, MsgS2CPlacementPending) {
			pm := m.Msg.(*protocolv1.S2CPlacementPending)
			if pm.GetRequestMessageId() == 208 {
				return true
			}
		}
		return false
	})
	if n := len(e.out.forChar(cid, MsgS2CRespawn)); n > 0 {
		t.Fatal("207 emitted while the respawn placement pends")
	}
	for _, c := range e.emit.all() {
		if c.OperationID == [16]byte(op) {
			t.Fatal("checkpoint write committed while pending")
		}
	}
	// Still a member (dead) on the source channel.
	if _, _, ok := e.w.director.Member(cid); !ok {
		t.Fatal("member dropped while respawn placement pends")
	}

	// Resolve: free a channel and re-check — the member lands on beta
	// and the 207 goes out.
	c, _ := e.w.director.Channel("map.test.beta", 1)
	c.Occupancy = HardCap - 1
	e.clocks.Advance(6 * time.Second)
	e.w.TickOnce()
	waitFor(t, "207 after pending resolved", func() bool {
		return len(e.out.forChar(cid, MsgS2CRespawn)) > 0
	})
}

func TestPendingEmitCarriesRequestMessageID(t *testing.T) {
	// The 15 notice echoes the wire request id so the client can coalesce
	// the pending state under the initiating request (respawn → 208).
	e := newTestEnv(t, testMaps())
	saturate(e, t, "map.test.alpha")
	cid := charID(1)
	_, pw, err := e.w.PlaceAttach(cid, "map.test.alpha",
		"spawn.entry.test.alpha", 7, nil)
	if err != nil || pw == nil {
		t.Fatalf("pending: %v %v", err, pw)
	}
	waitFor(t, "15 emitted", func() bool {
		return len(e.out.forChar(cid, MsgS2CPlacementPending)) > 0
	})
	m := e.out.forChar(cid, MsgS2CPlacementPending)[0].
		Msg.(*protocolv1.S2CPlacementPending)
	if m.GetRequestMessageId() != 7 {
		t.Fatalf("request_message_id: want 7, got %d", m.GetRequestMessageId())
	}
}
