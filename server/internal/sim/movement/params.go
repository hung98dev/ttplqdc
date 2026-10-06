package movement

import "thinhthan/internal/sim/spatial/collision"

// Canonical locomotion constants in integer millimeter units
// (movement.md § Baseline Physics, physics_geometry_contract.md §4.1).
// Velocities are mm/s, acceleration mm/s^2, distances mm; one tick is
// 50 ms (20 Hz), so per-tick displacement divides speed by 20.
const (
	// TickMillis is the fixed simulation step.
	TickMillis = 50
	// RunSpeedMmS is BASE_RUN_SPEED 6.0 m/s.
	RunSpeedMmS = 6000
	// FirstJumpMmS is FIRST_JUMP_IMPULSE +11.0 m/s.
	FirstJumpMmS = 11000
	// SecondJumpMmS is SECOND_JUMP_IMPULSE +10.0 m/s.
	SecondJumpMmS = 10000
	// GravityMmS2 is GRAVITY magnitude 28.0 m/s^2 (downward).
	GravityMmS2 = 28000
	// MaxFallMmS is MAX_FALL_SPEED 20.0 m/s (downward clamp).
	MaxFallMmS = 20000
	// AirControlBp is AIR_CONTROL 0.85 in basis points.
	AirControlBp = 8500
	// MaxStepHeightMm is MAX_STEP_HEIGHT 0.30 m.
	MaxStepHeightMm = 300
	// DropIgnoreTicks is the one-way drop-through ignore window
	// (ONE_WAY_DROP_IGNORE_MS 300 ms = 6 ticks).
	DropIgnoreTicks = 6
	// MaxJumpCount is the canonical double-jump budget.
	MaxJumpCount = 2

	// CharHalfWidthMm is half the CHARACTER collider width (800 mm).
	CharHalfWidthMm = 400
	// CharHeightMm is the CHARACTER collider height (1800 mm).
	CharHeightMm = 1800
)

// charBox is the CHARACTER reference collider: 800x1800 mm anchored at
// mid-feet (x, y) at scale (1,1,1) — physics_geometry_contract.md §3,
// ADR-0046. It is sprite-independent: the 64x96 px silhouette never
// changes this box.
func charBox(x, y int64) collision.AABB {
	return collision.AABB{
		MinX: x - CharHalfWidthMm,
		MinY: y,
		MaxX: x + CharHalfWidthMm,
		MaxY: y + CharHeightMm,
	}
}
