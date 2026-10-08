package world

import (
	"errors"
	"testing"
	"time"
)

func TestChannelStartsOnFirstAdmission(t *testing.T) {
	// sharding.md: channel partition lifecycle is demand-driven — the
	// first admission boots the partition, runs the consequence-load
	// hook, then opens membership.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	if got := e.loader.loaded(); got != 1 {
		t.Fatalf("consequence load ran %d times, want 1", got)
	}
	c, _ := e.w.director.Channel("map.test.alpha", 1)
	if c.State != ChannelRunning {
		t.Fatalf("channel state: %v", c.State)
	}
	if _, _, ok := e.w.director.Member(cid); !ok {
		t.Fatal("no membership")
	}
}

func TestLoaderFailureQuarantinesChannel(t *testing.T) {
	// A consequence-load hook failure quarantines the channel: it stays
	// out of every order, the admission re-pends, and the failure is
	// recorded on the channel row.
	e := newTestEnv(t, testMaps())
	e.loader.loadErr = errors.New("durable read down")
	cid := charID(1)
	res, pw, err := e.w.PlaceAttach(cid, "map.test.alpha",
		"spawn.entry.test.alpha", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pw != nil {
		t.Fatal("pre-start select must not pend")
	}
	e.w.AdmitCharacter(cid, res, "spawn.entry.test.alpha",
		Vitals{MaxHP: 100, HP: 100})
	waitFor(t, "channel quarantined", func() bool {
		c, _ := e.w.director.Channel("map.test.alpha", 1)
		return c.Quarantined
	})
	waitFor(t, "admission re-pended", func() bool {
		_, ok := e.w.director.Pending(cid)
		return ok
	})
	waitFor(t, "15 emitted", func() bool {
		return len(e.out.forChar(cid, MsgS2CPlacementPending)) > 0
	})
	// A fresh forced attach skips the quarantined channel entirely.
	e.loader.loadErr = nil
	res2, _, err := e.w.PlaceAttach(charID(2), "map.test.alpha",
		"spawn.entry.test.alpha", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.ChannelIndex == 1 {
		t.Fatal("quarantined channel still selected")
	}
}

func TestIdleChannelStopsAfterIdleStop(t *testing.T) {
	// A channel whose last member leaves enters IdleSince; IdleStop
	// (600s) later the runtime drains durable writes then stops it
	// (sharding.md channel stop semantics).
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	// Leave: occupancy frees, IdleSince stamps.
	e.w.director.OnPlayerLeave(cid)
	e.clocks.Advance(IdleStop + time.Second)
	e.w.TickOnce()
	c, _ := e.w.director.Channel("map.test.alpha", 1)
	if c.State != ChannelStopped {
		t.Fatalf("idle channel not stopped: %v", c.State)
	}
	if _, ok := e.w.hostFor("map.test.alpha", 1); ok {
		t.Fatal("stopped channel keeps a host")
	}
	// Re-admission restarts the channel (Running again).
	cid2, _ := e.admit(t, 2, "map.test.alpha", "spawn.entry.test.alpha")
	if _, _, ok := e.w.director.Member(cid2); !ok {
		t.Fatal("restart admit failed")
	}
	if got := e.loader.loaded(); got != 2 {
		t.Fatalf("restart must re-run consequence load: %d loads", got)
	}
}

func TestQuarantinedChannelRestartedByOperator(t *testing.T) {
	// hostFailed quarantines; RestartAllStopped clears the quarantine so
	// the channel re-enters placement (operator restart path).
	e := newTestEnv(t, testMaps())
	e.loader.loadErr = errors.New("down")
	cid := charID(1)
	res, _, err := e.w.PlaceAttach(cid, "map.test.alpha",
		"spawn.entry.test.alpha", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	e.w.AdmitCharacter(cid, res, "spawn.entry.test.alpha", Vitals{MaxHP: 100})
	waitFor(t, "quarantined", func() bool {
		c, _ := e.w.director.Channel("map.test.alpha", 1)
		return c.Quarantined
	})
	e.loader.loadErr = nil
	e.w.director.RestartAllStopped()
	c, _ := e.w.director.Channel("map.test.alpha", 1)
	if c.Quarantined {
		t.Fatal("quarantine not cleared")
	}
	// The channel is selectable again for forced placement.
	res2, pw2, err := e.w.PlaceAttach(charID(9), "map.test.alpha",
		"spawn.entry.test.alpha", 0, nil)
	if err != nil || pw2 != nil {
		t.Fatalf("post-restart select: %v %v", err, pw2)
	}
	if res2.ChannelIndex != 1 {
		t.Fatalf("restarted channel not packed first: ch%d", res2.ChannelIndex)
	}
}

func TestForcedPlacementNeverFallsThroughToOrder(t *testing.T) {
	// The forced order's tail is pending — Select never returns
	// ErrCapacityFull for PlaceForced even when every channel is at
	// HardCap.
	e := newTestEnv(t, testMaps())
	saturate(e, t, "map.test.alpha")
	_, pw, err := e.w.PlaceAttach(charID(1), "map.test.alpha",
		"spawn.entry.test.alpha", 0, nil)
	if err != nil {
		t.Fatalf("forced attach must never capacity-fail: %v", err)
	}
	if pw == nil {
		t.Fatal("saturated forced attach must pend")
	}
}

func TestRespawnAwaitingPlacementTransfersPending(t *testing.T) {
	// When the respawn placement resolves on another channel, the
	// destination admits the member — membership moves.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	e.killMember(t, cid)
	// Point the durable checkpoint at beta so the respawn transfers.
	e.loader.cpMap = "map.test.beta"
	e.loader.cpID = "checkpoint.test.beta"
	e.loader.cpAnchr = "checkpoint.test.beta"
	var op [16]byte
	op[0] = 0x99
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdRespawn, CharacterID: cid, OperationID: op,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "member on beta", func() bool {
		m, _, ok := e.w.director.Member(cid)
		return ok && m == "map.test.beta"
	})
	waitFor(t, "207 on beta", func() bool {
		return len(e.out.forChar(cid, MsgS2CRespawn)) > 0
	})
}
