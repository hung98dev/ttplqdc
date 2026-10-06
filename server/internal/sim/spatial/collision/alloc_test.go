//go:build !race

package collision

import (
	"testing"

	"thinhthan/internal/sim/spatial/geometry"
)

// allocWorld is a minimal walkable space: flat floor plus one blocking wall.
func allocWorld() *World {
	g := &geometry.Geometry{
		SpaceID:         "test.alloc",
		Kind:            geometry.SpaceKindFieldOrTown,
		ContentRevision: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	g.BoundsMM.MaxX = 200000
	g.BoundsMM.MaxY = 50000
	g.Segments = []geometry.Segment{
		{ID: 1, Kind: geometry.SolidGround, X1: 0, Y1: 0, X2: 200000, Y2: 0},
		{ID: 2, Kind: geometry.Wall, X1: 60000, Y1: 0, X2: 60000, Y2: 400},
	}
	return NewWorld(g)
}

// TestResolveMoveZeroAllocs pins the movement-tick hot path to zero
// allocations (Q3.go.alloc): a converged runner pressed against a wall and
// a grounded runner on open floor both resolve without heap traffic.
func TestResolveMoveZeroAllocs(t *testing.T) {
	w := allocWorld()
	opts := MoveOpts{StepHeightMM: 300}
	resting := AABB{MinX: 57600, MinY: 0, MaxX: 60000, MaxY: 1800}
	open := AABB{MinX: 2000, MinY: 0, MaxX: 4400, MaxY: 1800}

	if got := testing.AllocsPerRun(200, func() {
		_ = w.ResolveMove(resting, 200, 0, opts)
	}); got != 0 {
		t.Fatalf("resting wall press: %v allocs/op, want 0", got)
	}
	if got := testing.AllocsPerRun(200, func() {
		_ = w.ResolveMove(open, 200, 0, opts)
	}); got != 0 {
		t.Fatalf("open-floor run: %v allocs/op, want 0", got)
	}
}
