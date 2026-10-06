package movement

import (
	"context"
	"sync"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/collision"
	"thinhthan/internal/sim/spatial/geometry"
)

// manualClock is the partition's injectable time source: tests advance it
// explicitly so every tick is deterministic.
type manualClock struct {
	mu  sync.Mutex
	now time.Duration
}

func (c *manualClock) Now() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now += d
	c.mu.Unlock()
}

const testRevision = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// flatWorld is the synthetic test space: a wide flat floor (no contact
// events while running — the alloc fixture needs zero Contacts), a
// mountable 200 mm ledge, a 400 mm block, a wall, a low ceiling and a
// one-way platform for drop tests.
func flatWorld() *collision.World {
	g := &geometry.Geometry{
		SpaceID:         "test.flat",
		Kind:            geometry.SpaceKindFieldOrTown,
		ContentRevision: testRevision,
	}
	g.BoundsMM.MaxX = 200000
	g.BoundsMM.MaxY = 50000
	g.Segments = []geometry.Segment{
		{ID: 1, Kind: geometry.SolidGround, X1: 0, Y1: 0, X2: 200000, Y2: 0},
		// Mountable ledge (<= 300 mm step) walking right.
		{ID: 2, Kind: geometry.SolidGround, X1: 20000, Y1: 200, X2: 40000, Y2: 200},
		// Blocking wall (vertical face) at x=60000.
		{ID: 3, Kind: geometry.Wall, X1: 60000, Y1: 0, X2: 60000, Y2: 400},
		// Ceiling over x 80000..90000 at y 2400 — above the standing box
		// top (1800) but reachable inside the first jump's apex.
		{ID: 4, Kind: geometry.Ceiling, X1: 80000, Y1: 2400, X2: 90000, Y2: 2400},
		// One-way platform at y 1500 spanning x 100000..110000. No solid
		// geometry may sit anywhere inside the 1800 mm box that stands or
		// jumps through this column — a surface strictly above the feet is
		// a penetration, not a ceiling.
		{ID: 5, Kind: geometry.OneWayPlatform, X1: 100000, Y1: 1500, X2: 110000, Y2: 1500},
		// Slab with a 400 mm top over x 140000..150000 — an illegal-move
		// trigger when a position starts below it (penetration > step).
		{ID: 7, Kind: geometry.SolidGround, X1: 140000, Y1: 400, X2: 150000, Y2: 400},
	}
	g.Anchors = []geometry.Anchor{
		{ID: "spawn.entry.test", X: 5000, Y: 0},
		{ID: "checkpoint.test", X: 8000, Y: 0},
	}
	return collision.NewWorld(g)
}

// testRig drives a real partition Run loop through a parked gate: the
// loop's Sleep blocks on a channel the test pokes once per wanted tick,
// so between releases the tick goroutine is parked inside cfg.Sleep and
// test-side entity reads/writes never race it.
type testRig struct {
	p   *runtime.Partition
	s   *System
	clk *manualClock
	ctx context.Context
	cxl context.CancelFunc

	gate   chan struct{}
	parked chan struct{}

	// seqs records the greatest client_seq each entity submitted — the
	// SeqSource seam for 107.last_processed_client_seq (F-02: the
	// runtime-side field is unexported).
	seqsMu  sync.Mutex
	seqs    map[uint64]uint64
	runDone chan error
}

// fakeOutbound records Outbound messages for assertions.
type fakeOutbound struct {
	mu   sync.Mutex
	msgs []runtime.Outbound
}

func (f *fakeOutbound) Enqueue(o runtime.Outbound) error {
	f.mu.Lock()
	f.msgs = append(f.msgs, o)
	f.mu.Unlock()
	return nil
}

func (f *fakeOutbound) byID(id uint32) []*protocolv1.S2CMovementCorrection {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*protocolv1.S2CMovementCorrection
	for i := range f.msgs {
		if f.msgs[i].MessageID == id {
			if m, ok := f.msgs[i].Msg.(*protocolv1.S2CMovementCorrection); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func (f *fakeOutbound) errors() []*protocolv1.S2CError {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*protocolv1.S2CError
	for i := range f.msgs {
		if f.msgs[i].MessageID == 3 {
			if m, ok := f.msgs[i].Msg.(*protocolv1.S2CError); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

// newRig builds a partition + movement system on the flat world and runs
// the real tick loop on a goroutine under a manual clock.
func newRig(t *testing.T, out runtime.OutboundPort) *testRig {
	t.Helper()
	clk := &manualClock{}
	gate := make(chan struct{})
	parked := make(chan struct{}, 1)
	ctx, cxl := context.WithCancel(context.Background())
	p, err := runtime.NewPartition(runtime.PartitionConfig{
		MapID:           "map_test_flat",
		ChannelID:       7,
		InstanceID:      id.UUID{0x01},
		ContentRevision: testRevision,
		Seed:            0x5eed,
		Now:             clk.Now,
		Sleep: func(time.Duration) {
			// Announce parked (once) then block until the test releases a
			// tick — no owed-tick backlog exists because runTicks advances
			// the clock exactly one step per release.
			select {
			case parked <- struct{}{}:
			default:
			}
			select {
			case <-gate:
			case <-ctx.Done():
			}
		},
	}, runtime.Ports{})
	if err != nil {
		t.Fatalf("NewPartition: %v", err)
	}
	r := &testRig{
		p:      p,
		clk:    clk,
		seqs:   make(map[uint64]uint64),
		gate:   gate,
		parked: parked,
	}
	cfg := Config{World: flatWorld()}
	if out != nil {
		cfg.Outbound = out
		cfg.SeqSource = func(id uint64) uint64 {
			r.seqsMu.Lock()
			defer r.seqsMu.Unlock()
			return r.seqs[id]
		}
	}
	sys := New(cfg)
	r.s = sys
	p.RegisterSystem(runtime.PhaseMovement, sys.Step)
	r.ctx, r.cxl = ctx, cxl
	r.runDone = make(chan error, 1)
	go func() { r.runDone <- p.Run(ctx) }()
	t.Cleanup(cxl)
	// The goroutine must reach its first parked Sleep before the test
	// mutates partition state — otherwise setup races the start gate.
	select {
	case <-r.parked:
	case err := <-r.runDone:
		t.Fatalf("partition Run exited before first tick: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("partition Run never parked")
	}
	return r
}

// submit records the client seq for SeqSource then enqueues the intent
// through the real mailbox.
func (r *testRig) submit(entityID uint64, seq uint64, i runtime.Intent) {
	r.seqsMu.Lock()
	r.seqs[entityID] = seq
	r.seqsMu.Unlock()
	i.EntityID = entityID
	i.ClientSeq = seq
	_ = r.p.SubmitIntent(i)
}

// runTicks releases the parked tick goroutine n times — driving the real
// ingest path (mailbox -> routeIntent -> applyInboxes -> movement Step).
// Each release advances the clock one step; the loop parks again before
// runTicks returns, so callers always observe a fully quiesced partition.
func (r *testRig) runTicks(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for i := 0; i < n; i++ {
		// Drain any stale parked token, advance exactly one step, wake.
		select {
		case <-r.parked:
		default:
		}
		r.clk.advance(runtime.Step)
		select {
		case r.gate <- struct{}{}:
		case err := <-r.runDone:
			t.Fatalf("partition Run exited: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatalf("tick goroutine not parked: TickN=%d", r.p.TickN())
		}
		// Wait until the goroutine parks again — the tick completed and all
		// its writes happened-before the parked send.
		for {
			select {
			case <-r.parked:
				goto parked
			case err := <-r.runDone:
				t.Fatalf("partition Run exited: %v", err)
			default:
				if time.Now().After(deadline) {
					t.Fatalf("tick stall at i=%d: TickN=%d", i, r.p.TickN())
				}
				time.Sleep(time.Millisecond)
			}
		}
	parked:
	}
}
