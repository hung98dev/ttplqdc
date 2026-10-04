package collision

import (
	"testing"

	"thinhthan/internal/sim/spatial/geometry"
)

// TestCorrectionOnlyForIllegalOrForcedMoves pins the ADR-0069 contract: a
// verdict is produced only for geometry-illegal moves; ordinary resolutions
// (wall stops, landings, drops, step-ups) never yield a correction.
func TestCorrectionOnlyForIllegalOrForcedMoves(t *testing.T) {
	segs := []geometry.Segment{
		{ID: 1, Kind: geometry.SolidGround, X1: 0, Y1: 4000, X2: 51200, Y2: 4000},
		{ID: 2, Kind: geometry.Wall, X1: 30000, Y1: 4000, X2: 30000, Y2: 8000},
		{ID: 3, Kind: geometry.OneWayPlatform, X1: 10000, Y1: 9000, X2: 16000, Y2: 9000},
	}
	w := worldOf([2]int64{51200, 28800}, segs...)

	assertNone := func(name string, r Result) {
		t.Helper()
		if reason, need := r.Correction(); need || reason != CorrectionNone {
			t.Fatalf("%s: ordinary move flagged %v", name, reason)
		}
	}

	// Ordinary resolutions are not corrections.
	assertNone("free walk", w.ResolveMove(charBox(5000, 4000), 1500, 0, moveOpts))
	assertNone("wall stop", w.ResolveMove(charBox(28500, 4000), 3000, 0, moveOpts))
	assertNone("floor landing", w.ResolveMove(charBox(5000, 9000), 0, -4000, moveOpts))
	assertNone("one-way drop", w.ResolveMove(charBox(12000, 11000), 0, -5000,
		MoveOpts{StepHeightMM: 300, DropIgnorePlatformID: 3, DropIgnoreUntilTick: 999, Tick: 1}))
	assertNone("rise", w.ResolveMove(charBox(5000, 5000), 0, 2000, moveOpts))

	// A box already inside blocking geometry is the illegal-move case: only
	// this produces a correction verdict.
	bad := charBox(29900, 4000) // MinX edge past the wall face
	bad.MaxX = 30100
	r := w.ResolveMove(bad, 500, 0, moveOpts)
	reason, need := r.Correction()
	if !need || reason != CorrectionIllegalMove {
		t.Fatalf("penetrating move: got (%v,%v)", reason, need)
	}
}

// TestCorrectionReasonMapping pins the enum values to the wire
// MovementCorrectionReason numbers so the edge can cast directly.
func TestCorrectionReasonMapping(t *testing.T) {
	wire := map[CorrectionReason]int32{
		CorrectionNone:        0, // MOVEMENT_CORRECTION_REASON_UNSPECIFIED
		CorrectionIllegalMove: 1, // MOVEMENT_CORRECTION_REASON_ILLEGAL_MOVE
		CorrectionKnockback:   2, // MOVEMENT_CORRECTION_REASON_KNOCKBACK
		CorrectionPortal:      3, // MOVEMENT_CORRECTION_REASON_PORTAL
		CorrectionRespawn:     4, // MOVEMENT_CORRECTION_REASON_RESPAWN
		CorrectionForced:      5, // MOVEMENT_CORRECTION_REASON_FORCED
	}
	for r, want := range wire {
		if int32(r) != want {
			t.Fatalf("reason %d want wire %d", int32(r), want)
		}
	}
}
