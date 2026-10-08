package cooking

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

func (c *captureDurable) emitted() []runtime.DurableCommand {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]runtime.DurableCommand(nil), c.cmd...)
}

var testAnchors = []geometry.Anchor{
	{ID: "bonfire.map.lang_da.dinh_lang", X: 18880, Y: 8000},
	{ID: "cooking_hearth.map.lang_da.dinh_lang", X: 42496, Y: 4000},
}

// rig wires a partition running in the background, one player entity
// and the cooking channel registered on PhaseTimersStatus (settlements
// queue before PhaseDurable flushes them the same tick).
type rig struct {
	p   *runtime.Partition
	clk *testClock
	cap *captureDurable
	c   *Channel
	em  *Emission

	charID id.UUID
	eid    uint64
	cancel context.CancelFunc
}

func newRig(t *testing.T) *rig {
	t.Helper()
	clk := &testClock{}
	cap := &captureDurable{}
	p, err := runtime.NewPartition(runtime.PartitionConfig{
		MapID:           "map.lang_da.dinh_lang",
		ChannelID:       1,
		ContentRevision: testRevision,
		Seed:            1,
		Now:             clk.Now,
		Sleep:           func(time.Duration) {},
		Metrics:         core.NewRegistry(nil),
	}, runtime.Ports{Durable: cap})
	if err != nil {
		t.Fatalf("NewPartition: %v", err)
	}
	em := NewEmission()
	c := New("map.lang_da.dinh_lang", testAnchors, em)
	p.RegisterSystem(runtime.PhaseTimersStatus, c.Tick)
	eid, err := p.Admit(runtime.ClassPlayer)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if err := p.SetPosition(eid, 18880, 8000); err != nil {
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
	// Run seeds simTime from the clock at start: wait for it before any
	// advance, or the gap the first run() opens is lost.
	deadline := time.Now().Add(5 * time.Second)
	for !p.Started() {
		if time.Now().After(deadline) {
			t.Fatal("partition start deadline")
		}
		time.Sleep(time.Millisecond)
	}
	return &rig{p: p, clk: clk, cap: cap, c: c, em: em,
		charID: charID, eid: eid, cancel: cancel}
}

// run advances the sim clock n ticks and waits for the partition to
// execute them. Catch-up is capped at MaxCatchUpTicks per slice — the
// partition resyncs past a bigger gap without ticking — so the clock
// advances in slices of three.
func (r *rig) run(n uint64) {
	target := r.p.TickN() + n
	deadline := time.Now().Add(60 * time.Second)
	for r.p.TickN() < target {
		if time.Now().After(deadline) {
			panic("rig: tick deadline")
		}
		r.clk.advance(runtime.MaxCatchUpTicks)
		time.Sleep(time.Millisecond)
	}
}

func entity(t *testing.T, p *runtime.Partition, eid uint64) *runtime.Entity {
	t.Helper()
	e, err := p.Entity(eid)
	if err != nil {
		t.Fatalf("entity: %v", err)
	}
	return e
}

// TestKindleActivatesExtendsCaps: inactive → Activated (+30 min),
// in-window → Extended, beyond the 120-min server-time cap → Capped
// (consume-nothing verdict per world_rules.md § Kindling).
func TestKindleActivatesExtendsCaps(t *testing.T) {
	r := newRig(t)
	defer r.cancel()
	e := entity(t, r.p, r.eid)

	if v := r.c.AdmitKindle(e, 0); !v.OK {
		t.Fatalf("kindle admit rejected: %s", v.Code)
	}
	if fx := r.c.ApplyKindle(0); fx != EffectActivated {
		t.Fatalf("kindle effect %v want Activated", fx)
	}
	if !r.c.BonfireActive(0) || r.c.ExpiresAtTick() != KindleExtendTicks {
		t.Fatalf("bonfire state after activate")
	}
	if fx := r.c.ApplyKindle(10); fx != EffectExtended {
		t.Fatalf("kindle effect %v want Extended", fx)
	}
	if r.c.ExpiresAtTick() != 2*KindleExtendTicks {
		t.Fatalf("expiry %d", r.c.ExpiresAtTick())
	}
	// At now=0 the cap is tick 144000; expiry can reach exactly that —
	// the next extension is capped.
	for i := 0; i < 4; i++ {
		r.c.ApplyKindle(0)
	}
	if fx := r.c.ApplyKindle(0); fx != EffectCapped {
		t.Fatalf("kindle effect %v want Capped", fx)
	}
	// Admission mirrors the cap: OK but no durable consume.
	if v := r.c.AdmitKindle(e, 0); !v.OK || v.Consume {
		t.Fatalf("capped kindle verdict %+v want OK/Consume=false", v)
	}
}

// TestAdmissionsGateRangeAndCombat: interact reach (2.5 m) and the
// no-combat gate guard all three kinds; rest additionally requires an
// active bonfire, stationarity and the 6 m radius.
func TestAdmissionsGateRangeAndCombat(t *testing.T) {
	r := newRig(t)
	defer r.cancel()

	// Out of interact range → OUT_OF_RANGE.
	if err := r.p.SetPosition(r.eid, 18880+InteractRangeMM+1, 8000); err != nil {
		t.Fatal(err)
	}
	r.run(2) // movement lands in the snap
	e := entity(t, r.p, r.eid)
	if v := r.c.AdmitKindle(e, 0); v.OK || v.Code != "OUT_OF_RANGE" {
		t.Fatalf("kindle out-of-range verdict %+v", v)
	}
	// In range, in combat → STATE_CONFLICT.
	if err := r.p.SetPosition(r.eid, 18880, 8000); err != nil {
		t.Fatal(err)
	}
	r.run(2)
	e = entity(t, r.p, r.eid)
	e.InCombatWith = 42
	if v := r.c.AdmitCook(e); v.OK || v.Code != "STATE_CONFLICT" {
		t.Fatalf("cook in-combat verdict %+v", v)
	}
	e.InCombatWith = 0
	// Cook admission requires the HEARTH, not the bonfire.
	if v := r.c.AdmitCook(e); v.OK || v.Code != "OUT_OF_RANGE" {
		t.Fatalf("cook at bonfire verdict %+v", v)
	}
	// Rest before kindle → inactive bonfire → TARGET_INVALID.
	if v := r.c.AdmitRest(r.charID, e, 0); v.OK || v.Code != "TARGET_INVALID" {
		t.Fatalf("rest inactive verdict %+v", v)
	}
	r.c.ApplyKindle(0)
	if v := r.c.AdmitRest(r.charID, e, 0); !v.OK {
		t.Fatalf("rest admit: %s", v.Code)
	}
}

// TestRestSessionSettlesExpAndBond: a continuous rest emits one
// sim.rest_settlement per completed 10 s tick and one
// sim.beast_settlement per completed 300 s interval — payloads staged
// on the emission ledger until the emit adapter commits them.
func TestRestSessionSettlesExpAndBond(t *testing.T) {
	r := newRig(t)
	defer r.cancel()
	e := entity(t, r.p, r.eid)
	r.c.ApplyKindle(0)
	if v := r.c.AdmitRest(r.charID, e, 0); !v.OK {
		t.Fatalf("rest admit: %s", v.Code)
	}
	if !r.c.ApplyRest(r.charID, e) {
		t.Fatalf("rest did not start")
	}
	r.run(BondIntervalTicks)
	got := r.cap.emitted()
	var rest, bond int
	for _, cmd := range got {
		switch cmd.Family {
		case FamilyRestSettlement:
			rest++
		case FamilyBeastSettlement:
			bond++
		}
	}
	if rest != 30 || bond != 1 {
		t.Fatalf("settlements rest=%d bond=%d want 30/1", rest, bond)
	}
	// The staged payload carries the rest slot (1,000 EXP, bonfire ref).
	op := got[0].OperationID
	payload, ok := r.em.Payload(op)
	if !ok {
		t.Fatalf("payload missing for %s", op)
	}
	if payload.GetKind() != KindRest ||
		payload.GetSlots()[0].GetCharacterExp() != RestExpPerTick {
		t.Fatalf("rest payload %v exp %d",
			payload.GetKind(), payload.GetSlots()[0].GetCharacterExp())
	}
}

// TestRestEndsOnMovement: the first moved tick ends the session and no
// further settlement emits.
func TestRestEndsOnMovement(t *testing.T) {
	r := newRig(t)
	defer r.cancel()
	e := entity(t, r.p, r.eid)
	r.c.ApplyKindle(0)
	if !r.c.ApplyRest(r.charID, e) {
		t.Fatalf("rest did not start")
	}
	r.run(10)
	if err := r.p.SetPosition(r.eid, 18900, 8000); err != nil {
		t.Fatal(err)
	}
	r.run(2)
	if r.c.IsResting(r.charID) {
		t.Fatalf("rest survived movement")
	}
	r.run(RestExpTickInterval)
	for _, cmd := range r.cap.emitted() {
		if cmd.Family == FamilyRestSettlement {
			t.Fatalf("settlement emitted after movement end")
		}
	}
}

// TestRestEndsOnRadiusExit: stepping outside the 6 m radius ends the
// session (same rule via position, not velocity).
func TestRestEndsOnRadiusExit(t *testing.T) {
	r := newRig(t)
	defer r.cancel()
	e := entity(t, r.p, r.eid)
	r.c.ApplyKindle(0)
	if !r.c.ApplyRest(r.charID, e) {
		t.Fatalf("rest did not start")
	}
	if err := r.p.SetPosition(r.eid, 18880+RestRadiusMM+50, 8000); err != nil {
		t.Fatal(err)
	}
	r.run(2)
	if r.c.IsResting(r.charID) {
		t.Fatalf("rest survived radius exit")
	}
}

// TestRestStandUpToggle: a second admitted BONFIRE_REST ends the
// session (explicit stand-up).
func TestRestStandUpToggle(t *testing.T) {
	r := newRig(t)
	defer r.cancel()
	e := entity(t, r.p, r.eid)
	r.c.ApplyKindle(0)
	if !r.c.ApplyRest(r.charID, e) {
		t.Fatalf("rest did not start")
	}
	// Re-admission while resting is the stand-up toggle.
	if v := r.c.AdmitRest(r.charID, e, 0); !v.OK {
		t.Fatalf("stand-up admit rejected")
	}
	if r.c.ApplyRest(r.charID, e) {
		t.Fatalf("stand-up reported started")
	}
	if r.c.IsResting(r.charID) {
		t.Fatalf("still resting after stand-up")
	}
}
