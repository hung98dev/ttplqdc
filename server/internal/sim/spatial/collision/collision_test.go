package collision

import (
	"math/rand/v2"
	"testing"

	"thinhthan/internal/sim/spatial/geometry"
)

// charBox is the CHARACTER collider profile (800mm x 1600mm) at foot position.
func charBox(x, feet int64) AABB {
	return AABB{MinX: x, MinY: feet, MaxX: x + 800, MaxY: feet + 1600}
}

func worldOf(bounds [2]int64, segs ...geometry.Segment) *World {
	g := &geometry.Geometry{}
	g.BoundsMM.MaxX = bounds[0]
	g.BoundsMM.MaxY = bounds[1]
	g.Segments = segs
	return NewWorld(g)
}

var moveOpts = MoveOpts{StepHeightMM: 300}

// TestSlopeStepTransitions covers slope walking, auto step-up within
// MAX_STEP_HEIGHT, and hard wall stops (contract §4.2).
func TestSlopeStepTransitions(t *testing.T) {
	segs := []geometry.Segment{
		{ID: 1, Kind: geometry.SolidGround, X1: 0, Y1: 4000, X2: 10000, Y2: 4000},
		{ID: 2, Kind: geometry.Slope, X1: 10000, Y1: 4000, X2: 16000, Y2: 7000}, // up, 26.6°
		{ID: 3, Kind: geometry.SolidGround, X1: 16000, Y1: 7000, X2: 26000, Y2: 7000},
		{ID: 4, Kind: geometry.Wall, X1: 30000, Y1: 4000, X2: 30000, Y2: 4250}, // 250mm step
		{ID: 5, Kind: geometry.SolidGround, X1: 26000, Y1: 4000, X2: 30000, Y2: 4000},
		{ID: 6, Kind: geometry.SolidGround, X1: 30000, Y1: 4250, X2: 40000, Y2: 4250},
		{ID: 7, Kind: geometry.Wall, X1: 40000, Y1: 4250, X2: 40000, Y2: 5000}, // 750mm wall
	}
	w := worldOf([2]int64{51200, 28800}, segs...)

	// 1) Walk right onto and up the slope: X sweep is unobstructed, then the
	// feet-below-surface penetration is resolved upward (slope follow).
	r := w.ResolveMove(charBox(9000, 4000), 1500, 0, moveOpts)
	// Rigid box: feet rest on the highest surface point under the foot span,
	// which on an up-slope is the leading edge.
	wantFeet := geometry.SurfaceHeight(segs[1], 9000+1500+800)
	if r.Final.MinY != wantFeet {
		t.Fatalf("slope follow: feet=%d want surface=%d", r.Final.MinY, wantFeet)
	}
	if !r.Grounded || r.GroundSegment != 2 {
		t.Fatalf("slope grounded: %+v", r)
	}
	if r.VxZeroed || r.VyZeroed {
		t.Fatalf("slope move must not zero velocity: %+v", r)
	}

	// 2) Descending the same slope backwards: a fall lands on the slope at
	// the new position's highest surface point under the foot span.
	r = w.ResolveMove(charBox(15000, 6900), -4000, -4000, moveOpts)
	if !r.Grounded || r.GroundSegment != 2 {
		t.Fatalf("descend not grounded: %+v", r)
	}
	wantY := geometry.SurfaceHeight(segs[1], 15000-4000+800)
	if r.Final.MinY != wantY {
		t.Fatalf("descend feet=%d want %d", r.Final.MinY, wantY)
	}

	// 3) 250mm ledge: auto step-up, no velocity lost.
	r = w.ResolveMove(charBox(29000, 4000), 1500, 0, moveOpts)
	if r.Final.MinY != 4250 {
		t.Fatalf("step-up feet=%d want 4250", r.Final.MinY)
	}
	if r.Final.MinX != 30500 { // box advanced the full 1500mm past the ledge
		t.Fatalf("step-up x=%d", r.Final.MinX)
	}
	if r.VxZeroed {
		t.Fatalf("step-up must not zero vx")
	}
	if !r.Grounded || r.GroundSegment != 6 {
		t.Fatalf("step-up ground: %+v", r)
	}

	// 4) 750mm wall: horizontal stop at the wall face, vx zeroed.
	r = w.ResolveMove(charBox(38500, 4250), 2000, 0, moveOpts)
	if r.Final.MaxX != 40000 {
		t.Fatalf("wall stop MaxX=%d want 40000", r.Final.MaxX)
	}
	if !r.VxZeroed {
		t.Fatalf("wall must zero vx")
	}
	if len(r.Contacts) == 0 || r.Contacts[len(r.Contacts)-1].SegmentID != 7 {
		t.Fatalf("wall contact: %+v", r.Contacts)
	}

	// 5) Falling lands on the first surface hit: drop from above the plateau.
	r = w.ResolveMove(charBox(20000, 9000), 0, -3000, moveOpts)
	if r.Final.MinY != 7000 || !r.VyZeroed || r.GroundSegment != 3 {
		t.Fatalf("landing: %+v", r)
	}
}

// TestOneWayLedgeDrop covers one-way platform semantics: they stop falls only
// when the previous-tick feet were above the surface, honor the drop-ignore
// window, and never block upward motion (contract §4.2).
func TestOneWayLedgeDrop(t *testing.T) {
	segs := []geometry.Segment{
		{ID: 1, Kind: geometry.SolidGround, X1: 0, Y1: 4000, X2: 51200, Y2: 4000},
		{ID: 2, Kind: geometry.OneWayPlatform, X1: 16000, Y1: 11000, X2: 22000, Y2: 11000},
	}
	w := worldOf([2]int64{51200, 28800}, segs...)
	opts := moveOpts
	opts.Tick = 100

	// Falling from above lands on the platform.
	r := w.ResolveMove(charBox(18000, 13000), 0, -5000, opts)
	if !r.VyZeroed || r.Final.MinY != 11000 || r.GroundSegment != 2 || !r.OnOneWay {
		t.Fatalf("one-way landing: %+v", r)
	}

	// Within the drop-ignore window the same fall passes through to ground.
	ignore := opts
	ignore.DropIgnorePlatformID = 2
	ignore.DropIgnoreUntilTick = 200
	r = w.ResolveMove(charBox(18000, 13000), 0, -5000, ignore)
	if r.Final.MinY != 8000 || r.GroundSegment == 2 {
		t.Fatalf("drop-ignore: %+v", r)
	}

	// After the window expires the platform blocks again.
	expired := ignore
	expired.Tick = 200
	r = w.ResolveMove(charBox(18000, 13000), 0, -5000, expired)
	if r.Final.MinY != 11000 {
		t.Fatalf("expired window: %+v", r)
	}

	// Feet below the platform fall through without blocking.
	r = w.ResolveMove(charBox(18000, 10000), 0, -3000, opts)
	if r.Final.MinY != 7000 || r.GroundSegment == 2 {
		t.Fatalf("below platform: %+v", r)
	}

	// Rising through the platform is unobstructed.
	r = w.ResolveMove(charBox(18000, 9000), 0, 5000, opts)
	if r.Final.MinY != 14000 || r.VyZeroed {
		t.Fatalf("rise through: %+v", r)
	}
}

// TestDeterministicSweepVectors runs >=10,000 seeded vectors and requires the
// integrated resolution to be bitwise-identical across two runs (contract
// §2 determinism).
func TestDeterministicSweepVectors(t *testing.T) {
	segs := []geometry.Segment{
		{ID: 1, Kind: geometry.SolidGround, X1: 0, Y1: 4000, X2: 10000, Y2: 4000},
		{ID: 2, Kind: geometry.Slope, X1: 10000, Y1: 4000, X2: 16000, Y2: 7000},
		{ID: 3, Kind: geometry.SolidGround, X1: 16000, Y1: 7000, X2: 26000, Y2: 7000},
		{ID: 4, Kind: geometry.Wall, X1: 30000, Y1: 4000, X2: 30000, Y2: 4250},
		{ID: 5, Kind: geometry.SolidGround, X1: 26000, Y1: 4000, X2: 30000, Y2: 4000},
		{ID: 6, Kind: geometry.SolidGround, X1: 30000, Y1: 4250, X2: 40000, Y2: 4250},
		{ID: 7, Kind: geometry.Wall, X1: 40000, Y1: 4250, X2: 40000, Y2: 5000},
		{ID: 8, Kind: geometry.Slope, X1: 40000, Y1: 5000, X2: 46000, Y2: 2000},
		{ID: 9, Kind: geometry.SolidGround, X1: 46000, Y1: 2000, X2: 51200, Y2: 2000},
		{ID: 10, Kind: geometry.OneWayPlatform, X1: 16000, Y1: 11000, X2: 22000, Y2: 11000},
		{ID: 11, Kind: geometry.Ceiling, X1: 0, Y1: 14000, X2: 51200, Y2: 14000},
		{ID: 12, Kind: geometry.Wall, X1: 0, Y1: 0, X2: 0, Y2: 28800},
		{ID: 13, Kind: geometry.Wall, X1: 51200, Y1: 0, X2: 51200, Y2: 28800},
	}
	w := worldOf([2]int64{51200, 28800}, segs...)

	run := func(seed uint64, n int) []Result {
		rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
		out := make([]Result, n)
		for i := 0; i < n; i++ {
			x := rng.Int64N(49000)
			feet := rng.Int64N(20000)
			dx := rng.Int64N(6001) - 3000
			dy := rng.Int64N(6001) - 3000
			opts := MoveOpts{StepHeightMM: 300, Tick: uint64(rng.Int64N(1000))}
			if rng.Int64N(4) == 0 {
				opts.DropIgnorePlatformID = 10
				opts.DropIgnoreUntilTick = uint64(rng.Int64N(2000))
			}
			out[i] = w.ResolveMove(charBox(x, feet), dx, dy, opts)
		}
		return out
	}

	const n = 10000
	a, b := run(0xC0FFEE, n), run(0xC0FFEE, n)
	for i := 0; i < n; i++ {
		ra, rb := a[i], b[i]
		if ra.Final != rb.Final || ra.VxZeroed != rb.VxZeroed || ra.VyZeroed != rb.VyZeroed ||
			ra.Grounded != rb.Grounded || ra.GroundSegment != rb.GroundSegment ||
			ra.OnOneWay != rb.OnOneWay || len(ra.Contacts) != len(rb.Contacts) {
			t.Fatalf("vector %d diverged: %+v vs %+v", i, ra, rb)
		}
		for j := range ra.Contacts {
			if ra.Contacts[j] != rb.Contacts[j] {
				t.Fatalf("vector %d contact %d diverged", i, j)
			}
		}
		// Invariants hold for every legal input: the final box stays inside
		// the world's enclosing walls. (Illegal starting states are reported
		// by Correction() and are not repositioned by physics alone.)
		if _, need := ra.Correction(); !need {
			if ra.Final.MaxX > 51200 || ra.Final.MinX < 0 {
				t.Fatalf("vector %d escaped walls: %+v", i, ra.Final)
			}
		}
	}
}
