package world

import (
	"testing"
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// forced placement must never answer MAP_CAPACITY_FULL — ADR-0061/0070's
// order list + ADR-0062 pending admission replace rejection.

func TestForcedPlacementBypassesSoftCap(t *testing.T) {
	// A forced attach must never fail while capacity remains: every
	// running channel at SoftCap (18) — the player-initiated cap — still
	// admits up to the forced hard cap (22).
	e := newTestEnv(t, testMaps())
	for i := 1; i <= ChannelsPerMap; i++ {
		c, _ := e.w.director.Channel("map.test.alpha", uint32(i))
		c.State = ChannelRunning
		c.Occupancy = SoftCap
	}
	cid, res := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	if res.ChannelIndex != 1 {
		t.Fatalf("forced hard-cap tail should resolve ch1, got ch%d", res.ChannelIndex)
	}
	if _, _, ok := e.w.director.Member(cid); !ok {
		t.Fatal("member not admitted")
	}
}

func TestForcedPlacementPrefersStoppedOverSoftCapOverflow(t *testing.T) {
	// A stopped channel always outranks overflowing a running channel's
	// soft cap — forced placement isolates load rather than packing past
	// the normal cap.
	e := newTestEnv(t, testMaps())
	ch, _ := e.w.director.Channel("map.test.alpha", 1)
	ch.State = ChannelRunning
	ch.Occupancy = SoftCap

	cid, res := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	if res.ChannelIndex != 2 {
		t.Fatalf("stopped ch2 must beat the soft-capped ch1, got ch%d",
			res.ChannelIndex)
	}
	if _, _, ok := e.w.director.Member(cid); !ok {
		t.Fatal("member not admitted")
	}
}

func TestForcedPlacementSaturatedGoesPending(t *testing.T) {
	// Every channel at HardCap → forced attach pends (never
	// MAP_CAPACITY_FULL) with a 15 notice.
	e := newTestEnv(t, testMaps())
	for i := 1; i <= ChannelsPerMap; i++ {
		c, _ := e.w.director.Channel("map.test.alpha", uint32(i))
		c.State = ChannelRunning
		c.Occupancy = HardCap
	}
	cid := charID(1)
	resolved := make(chan PlacementResult, 1)
	_, pw, err := e.w.PlaceAttach(cid, "map.test.alpha", "spawn.entry.test.alpha",
		0, func(w *Runtime, r PlacementResult) { resolved <- r })
	if err != nil {
		t.Fatalf("forced attach must not fail closed: %v", err)
	}
	if pw == nil {
		t.Fatal("expected PendingWait on saturated forced attach")
	}
	if pw.Reason != PendingFirstLogin || pw.RetryAfterMs != PendingRetryMS {
		t.Fatalf("pending row wrong: %+v", pw)
	}
	waitFor(t, "15 emitted", func() bool {
		return len(e.out.forChar(cid, MsgS2CPlacementPending)) > 0
	})
	m := e.out.forChar(cid, MsgS2CPlacementPending)[0].
		Msg.(*protocolv1.S2CPlacementPending)
	if m.GetRetryAfterMs() != 5000 ||
		m.GetReason() != protocolv1.PlacementReason_PLACEMENT_REASON_FIRST_LOGIN {
		t.Fatalf("15 fields: %+v", m)
	}
	select {
	case <-resolved:
		t.Fatal("pending resolved while saturated")
	default:
	}
}

func TestForcedPlacementCooldownBypassed(t *testing.T) {
	// Switch cooldown gates 109 consult admission (edge side); the
	// director-side forced placement itself never consults the cooldown —
	// a reconnect may land inside the cooldown window.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	e.w.director.MarkSwitch(cid)

	// Forced re-placement (reattach) must not be gated by the switch
	// cooldown: Select has no cooldown check; consult answers it.
	res, pw, err := e.w.PlaceAttach(cid, "map.test.alpha",
		"spawn.entry.test.alpha", 0, nil)
	_ = res
	_ = pw
	if err != nil {
		t.Fatalf("forced attach during cooldown: %v", err)
	}
}

func TestForcedPlacementChannelSwitchTarget(t *testing.T) {
	// A channel switch prefers the exact target channel while it is
	// below HardCap — the player chose it.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")

	// Target ch3 running below soft cap.
	ch3, _ := e.w.director.Channel("map.test.alpha", 3)
	ch3.State = ChannelRunning
	ch3.Occupancy = 2

	var op [16]byte
	op[0] = 0x31
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdChannelSwitch, CharacterID: cid,
		OperationID: op, TargetChannel: 3,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "105 toward ch3", func() bool {
		for _, m := range e.out.forChar(cid, MsgS2CTransferPrepare) {
			p := m.Msg.(*protocolv1.S2CTransferPrepare)
			if p.GetChannelIndex() == 3 {
				return true
			}
		}
		return false
	})
	// Complete the switch: member lands on ch3 and MarkSwitch records it.
	for _, m := range e.out.forChar(cid, MsgS2CTransferPrepare) {
		p := m.Msg.(*protocolv1.S2CTransferPrepare)
		if p.GetChannelIndex() == 3 {
			_ = e.w.PostCommand(cid, &Command{
				Kind: CmdPresentationReady, CharacterID: cid,
				TransferID: [16]byte(p.GetTransferId()),
			})
		}
	}
	waitFor(t, "member on ch3", func() bool {
		_, ch, ok := e.w.director.Member(cid)
		return ok && ch == 3
	})
}

func TestSwitchCooldownBlocksSecondConsult(t *testing.T) {
	// The admission consult fails closed while the cooldown is live —
	// a member who just switched cannot re-consult within 10s.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	e.w.director.MarkSwitch(cid)
	rep := e.consultSync(t, Consult{
		Kind: ConsultPlacementChannel, CharacterID: cid, TargetChannel: 2,
	})
	if rep.OK {
		t.Fatal("channel consult must reject inside the switch cooldown")
	}
	if rep.Code == protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Fatal("cooldown consult needs a concrete error code")
	}
	// After the cooldown the consult re-evaluates capacity: channel 2 is
	// stopped, so the answer is the capacity reject (a stopped channel
	// is never a consult-resolvable switch target).
	e.clocks.Advance(ChannelSwitchCooldown + time.Second)
	rep = e.consultSync(t, Consult{
		Kind: ConsultPlacementChannel, CharacterID: cid, TargetChannel: 2,
	})
	if rep.OK || rep.Code != protocolv1.ErrorCode_ERROR_CODE_MAP_CAPACITY_FULL {
		t.Fatalf("post-cooldown consult on stopped ch2: want MAP_CAPACITY_FULL, got %+v", rep)
	}
}
