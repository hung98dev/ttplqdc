package skills

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/spatial/collision"
)

// SweepResult is the collision-resolved outcome of a DASH_LINE /
// MOVE_LINE / MOVE_CONTACT_LINE motion: the caster's forced authored
// displacement truncated by blocking geometry.
type SweepResult struct {
	// EndX is the resolved end anchor x (feet position).
	EndX int64
	// EndY is the resolved end anchor y (walkable floor under the box).
	EndY int64
	// Truncated reports motion stopped before the authored distance
	// (wall/obstruction) — the authored motion deadline is still
	// reserved either way.
	Truncated bool
	// PathMinX / PathMaxX bound the resolved horizontal path.
	PathMinX, PathMaxX int64
}

// SweepLine resolves an authored horizontal line motion through the
// authoritative collision world: the caster box sweeps in the facing
// direction up to the authored distance, stopping at blocked geometry
// (and mounting ledges up to step height). Returns the resolved end
// anchor and truncation flag.
func SweepLine(w *collision.World, caster collision.AABB, distanceMM int64, facing protocolv1.Facing, tick uint64) SweepResult {
	dx := distanceMM
	if facing == protocolv1.Facing_FACING_LEFT {
		dx = -dx
	}
	res := w.ResolveMove(caster, dx, 0, collision.MoveOpts{StepHeightMM: 300, Tick: tick})
	endX := res.Final.MinX
	out := SweepResult{
		EndX:      endX,
		EndY:      res.Final.MinY,
		Truncated: res.VxZeroed,
	}
	if dx > 0 {
		out.PathMinX, out.PathMaxX = caster.MinX, endX
	} else {
		out.PathMinX, out.PathMaxX = endX, caster.MinX
	}
	return out
}

// ContactBox returns the swept contact box of a MOVE_CONTACT_LINE or
// DASH_LINE hit volume: the resolved path span (both end anchors)
// expanded by the authored hit half-height.
func ContactBox(startX, endX, startY, endY, hitHalfHeightMM, casterWidthMM int64) collision.AABB {
	minX, maxX := startX, endX
	if endX < startX {
		minX, maxX = endX, startX+casterWidthMM
	} else {
		maxX = endX + casterWidthMM
	}
	minY, maxY := startY, endY
	if endY < startY {
		minY, maxY = endY, startY
	}
	return collision.AABB{
		MinX: minX, MaxX: maxX,
		MinY: minY - hitHalfHeightMM, MaxY: maxY + hitHalfHeightMM,
	}
}
