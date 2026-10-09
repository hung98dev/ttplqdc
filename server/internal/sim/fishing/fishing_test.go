package fishing

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/observability/core"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
)

const testRevision = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// testClock is the manually advanced partition clock.
type testClock struct{ now atomic.Int64 }

func (c *testClock) Now() time.Duration { return time.Duration(c.now.Load()) }
func (c *testClock) advance(steps uint64) {
	c.now.Add(int64(steps) * int64(runtime.Step))
}

// captureDurable collects emitted flat durable commands.
type captureDurable struct {
	mu  sync.Mutex
	cmd []runtime.DurableCommand
}

func (c *captureDurable) Emit(_ context.Context, cmd runtime.DurableCommand) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cmd = append(c.cmd, cmd)
	return nil
}

var testAnchors = []geometry.Anchor{
	{ID: "fishing_spot.map.lang_da.ben_da.01", X: 10000, Y: 2000},
	{ID: "bonfire.map.lang_da.dinh_lang", X: 18880, Y: 8000},
}

// rig wires a partition running in the background, one player entity
// at the spot and the fishing channel registered on PhaseTimersStatus.
type rig struct {
	p    *runtime.Partition
	clk  *testClock
	c    *Channel
	done atomic.Uint64

	charID id.UUID
	eid    uint64
	cancel context.CancelFunc
}

func newRig(t *testing.T) *rig {
	t.Helper()
	clk := &testClock{}
	p, err := runtime.NewPartition(runtime.PartitionConfig{
		MapID:           "map.lang_da.ben_da",
		ChannelID:       1,
		ContentRevision: testRevision,
		Seed:            1,
		Now:             clk.Now,
		Sleep:           func(time.Duration) {},
		Metrics:         core.NewRegistry(nil),
	}, runtime.Ports{Durable: &captureDurable{}})
	if err != nil {
		t.Fatalf("NewPartition: %v", err)
	}
	c := New("map.lang_da.ben_da", testAnchors)
	p.RegisterSystem(runtime.PhaseTimersStatus, c.Tick)
	r := &rig{p: p, clk: clk, c: c}
	// PhaseCleanup runs last; when it records a tick, that tick is
	// fully done.
	p.RegisterSystem(runtime.PhaseCleanup, func(_ *runtime.Partition, tc *runtime.TickContext) {
		r.done.Store(tc.Tick)
	})
	eid, err := p.Admit(runtime.ClassPlayer)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if err := p.SetPosition(eid, 10000, 2000); err != nil {
		t.Fatalf("pos: %v", err)
	}
	charID := id.NewV4()
	c.SetMemberEntity(func(cid id.UUID) (uint64, bool) {
		if cid == charID {
			return eid, true
		}
		return 0, false
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = p.Run(ctx) }()
	t.Cleanup(cancel)
	deadline := time.Now().Add(5 * time.Second)
	for !p.Started() {
		if time.Now().After(deadline) {
			t.Fatal("partition start deadline")
		}
		time.Sleep(time.Millisecond)
	}
	r.charID, r.eid, r.cancel = charID, eid, cancel
	return r
}

// run advances the sim clock n ticks and waits for the partition to
// execute them (mirrors sim/cooking's rig).
func (r *rig) run(n uint64) {
	target := r.p.TickN() + n
	deadline := time.Now().Add(60 * time.Second)
	for r.done.Load() < target {
		if time.Now().After(deadline) {
			panic("rig: tick deadline")
		}
		r.clk.advance(runtime.MaxCatchUpTicks)
		time.Sleep(time.Millisecond)
	}
}

func entityAt(r *rig) *runtime.Entity {
	e, _ := r.p.Entity(r.eid)
	return e
}

// TestCastHookWindowSuccess — CAST admitted at the spot, window opens
// 2 s later, the single HOOK inside [0.40 s, 1.20 s] is accepted.
func TestCastHookWindowSuccess(t *testing.T) {
	r := newRig(t)
	e := entityAt(r)
	if v := r.c.AdmitCast(r.charID, e); !v.OK {
		t.Fatalf("cast rejected: %s", v.Code)
	}
	r.c.ApplyCast(r.charID, "fishing_spot.map.lang_da.ben_da.01", r.p.TickN())
	if r.c.StateOf(r.charID) != StateCasting {
		t.Fatal("not CASTING after apply")
	}
	// HOOK before the window opens is rejected.
	if v := r.c.AdmitHook(r.charID, e, r.p.TickN()); v.OK {
		t.Fatal("hook admitted while CASTING")
	}
	// Advance past the 2 s open.
	r.run(CastToWindowTicks + 1)
	if !r.c.WindowOpen(r.charID) {
		t.Fatal("window did not open at 2 s")
	}
	// The accepted hook lands inside [8, 24] ticks of window start.
	r.run(HookAcceptMinTicks)
	if v := r.c.AdmitHook(r.charID, e, r.p.TickN()); !v.OK {
		t.Fatalf("in-window hook rejected: %s", v.Code)
	}
	r.c.ApplyHook(r.charID)
	if r.c.StateOf(r.charID) != StateIdle {
		t.Fatal("session not closed after hook apply")
	}
}

// TestHookWithoutCastFails — a HOOK with no live session is rejected.
func TestHookWithoutCastFails(t *testing.T) {
	r := newRig(t)
	if v := r.c.AdmitHook(r.charID, entityAt(r), r.p.TickN()); v.OK {
		t.Fatal("hook admitted without a cast")
	}
}

// TestCastDuringWindowRejected — a second CAST (or any non-HOOK kind
// reaching the fishing consult) inside HOOK_WINDOW is rejected; only
// the single HOOK is accepted.
func TestCastDuringWindowRejected(t *testing.T) {
	r := newRig(t)
	e := entityAt(r)
	r.c.ApplyCast(r.charID, "fishing_spot.map.lang_da.ben_da.01", r.p.TickN())
	r.run(CastToWindowTicks + HookAcceptMinTicks)
	if v := r.c.AdmitCast(r.charID, e); v.OK {
		t.Fatal("cast admitted during hook window")
	}
}

// TestMistimedHookMisses — a HOOK inside the window but outside the
// accept interval resolves the cast as failure (miss, session closed,
// bait stays consumed on the durable side).
func TestMistimedHookMisses(t *testing.T) {
	r := newRig(t)
	e := entityAt(r)
	r.c.ApplyCast(r.charID, "fishing_spot.map.lang_da.ben_da.01", r.p.TickN())
	r.run(CastToWindowTicks) // window just opened, tick = start
	// Hook at window start (before +8 ticks) -> miss.
	if v := r.c.AdmitHook(r.charID, e, r.p.TickN()); v.OK {
		t.Fatal("early hook accepted")
	}
	if r.c.StateOf(r.charID) != StateIdle {
		t.Fatal("missed hook did not close the session")
	}
}

// TestWindowExpiryMiss — a window with no hook expires to failure.
func TestWindowExpiryMiss(t *testing.T) {
	r := newRig(t)
	r.c.ApplyCast(r.charID, "fishing_spot.map.lang_da.ben_da.01", r.p.TickN())
	r.run(CastToWindowTicks + HookAcceptMaxTicks + 2)
	if r.c.StateOf(r.charID) != StateIdle {
		t.Fatal("expired window left session open")
	}
}

// TestDisconnectResolvesFailure — disconnect during CASTING or the
// window force-closes the session (bait stays consumed, no catch).
func TestDisconnectResolvesFailure(t *testing.T) {
	r := newRig(t)
	e := entityAt(r)
	r.c.ApplyCast(r.charID, "fishing_spot.map.lang_da.ben_da.01", r.p.TickN())
	r.run(CastToWindowTicks + 1)
	if !r.c.WindowOpen(r.charID) {
		t.Fatal("window did not open")
	}
	r.c.EndFishing(r.charID)
	if r.c.StateOf(r.charID) != StateIdle {
		t.Fatal("disconnect left session open")
	}
	if v := r.c.AdmitHook(r.charID, e, r.p.TickN()); v.OK {
		t.Fatal("hook accepted after disconnect")
	}
}

// TestAdmissionGates — no spot anchor on the map, combat or dead
// entity, and out-of-range casts all fail closed.
func TestAdmissionGates(t *testing.T) {
	r := newRig(t)
	e := entityAt(r)
	// No spots.
	empty := New("map.nowhere", nil)
	if v := empty.AdmitCast(r.charID, e); v.OK || v.Code != "TARGET_INVALID" {
		t.Fatalf("spotless map admitted: %+v", v)
	}
	// Combat.
	e.InCombatWith = 9
	if v := r.c.AdmitCast(r.charID, e); v.OK || v.Code != "STATE_CONFLICT" {
		t.Fatalf("combat admitted: %+v", v)
	}
	e.InCombatWith = 0
	// Dead.
	e.Dead = true
	if v := r.c.AdmitCast(r.charID, e); v.OK {
		t.Fatal("dead admitted")
	}
	e.Dead = false
	// Out of range (no spot within 2500 mm).
	if err := r.p.SetPosition(r.eid, 60000, 60000); err != nil {
		t.Fatalf("pos: %v", err)
	}
	if v := r.c.AdmitCast(r.charID, entityAt(r)); v.OK || v.Code != "OUT_OF_RANGE" {
		t.Fatalf("far cast admitted: %+v", v)
	}
}
