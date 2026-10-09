package skills

import (
	"errors"

	"thinhthan/internal/sim/spatial/collision"
	"thinhthan/internal/sim/spatial/geometry"
)

// CAT-002 barrier: exactly one grounded stationary AABB created at the
// action's ACTIVE that blocks enemy movement and projectiles until the
// authored expiry tick. No damage, shield, zone, extra target query or
// displacement is ever created by the barrier.

// Barrier is one placed barrier instance.
type Barrier struct {
	Box         collision.AABB
	ExpiresTick uint64 // authoritative removal tick (exclusive)
}

// BarrierSet tracks the live barriers of one world instance.
type BarrierSet struct {
	items []Barrier
}

// BlockedBy reports whether a mover's box collides with a live barrier
// (enemy movement and projectile blocking) at a tick.
func (s *BarrierSet) BlockedBy(b collision.AABB, tick uint64) bool {
	for _, it := range s.items {
		if tick < it.ExpiresTick && it.Box.Intersects(b) {
			return true
		}
	}
	return false
}

// BlocksPoint reports whether a projectile center (expanded by radius)
// collides with a live barrier at a tick.
func (s *BarrierSet) BlocksPoint(x, y, radiusMM int64, tick uint64) bool {
	b := collision.AABB{MinX: x - radiusMM, MaxX: x + radiusMM, MinY: y - radiusMM, MaxY: y + radiusMM}
	return s.BlockedBy(b, tick)
}

// Expire drops barriers at or before the authoritative tick.
func (s *BarrierSet) Expire(tick uint64) {
	rest := s.items[:0]
	for _, it := range s.items {
		if tick < it.ExpiresTick {
			rest = append(rest, it)
		}
	}
	s.items = rest
}

// Live returns the live barriers.
func (s *BarrierSet) Live() []Barrier { return s.items }

// Placement errors.
var (
	ErrBarrierRange     = errors.New("skills: barrier cast point outside authored cast range")
	ErrBarrierGround    = errors.New("skills: barrier anchor unresolved or ungrounded")
	ErrBarrierOverlap   = errors.New("skills: barrier placement overlaps solid geometry")
	ErrBarrierBounds    = errors.New("skills: barrier AABB outside playable bounds")
	ErrBarrierTargeting = errors.New("skills: barrier geometry requires AREA_POSITION targeting")
)

// PlaceBarrier resolves a BARRIER_POSITION cast (CAT-002):
// ground-snap the requested point to a bottom-center anchor, validate
// Euclidean cast range from the skill origin, reject solid overlap or
// an AABB outside playable bounds, then register exactly one blocking
// AABB [anchor.x-thickness/2, anchor.x+thickness/2] × [anchor.y,
// anchor.y+height] living until the authored expiry tick.
func PlaceBarrier(w *collision.World, s *BarrierSet, d *Def, o Origin, castX int64, tick uint64) (Barrier, error) {
	b := d.Geom.Barrier
	if b == nil || d.Targeting != TargetAreaPosition {
		return Barrier{}, ErrBarrierTargeting
	}
	// Euclidean cast range from the skill origin, measured before the
	// ground snap uses the requested point's depth at anchor y = 0 —
	// the authoritative rule resolves the snapped anchor, so validate
	// against the snapped position below.
	anchorX := castX
	// Ground-snap: the anchor's bottom edge rests on the highest
	// walkable surface under the barrier footprint.
	anchorY, ok := floorUnder(w, anchorX-b.ThicknessMM/2, anchorX+b.ThicknessMM/2)
	if !ok {
		return Barrier{}, ErrBarrierGround
	}
	box := collision.AABB{
		MinX: anchorX - b.ThicknessMM/2, MaxX: anchorX + b.ThicknessMM/2,
		MinY: anchorY, MaxY: anchorY + b.HeightMM,
	}
	// Euclidean cast range against the resolved anchor.
	dx := anchorX - o.X
	dy := anchorY - o.Y
	if dx*dx+dy*dy > b.CastMM*b.CastMM {
		return Barrier{}, ErrBarrierRange
	}
	// Playable bounds.
	g := w.Geometry()
	if box.MinX < 0 || box.MinY < 0 || box.MaxX > g.BoundsMM.MaxX || box.MaxY > g.BoundsMM.MaxY {
		return Barrier{}, ErrBarrierBounds
	}
	// Solid overlap: any geometry segment whose interior crosses the
	// barrier box rejects the placement (the floor under the MinY edge
	// shares a boundary only and never counts).
	if overlapsSolid(w, box) {
		return Barrier{}, ErrBarrierOverlap
	}
	it := Barrier{Box: box, ExpiresTick: tick + ceilTick(b.DurationMs)}
	s.items = append(s.items, it)
	return it, nil
}

// overlapsSolid reports whether any authored segment crosses the box's
// interior (strict overlap on both axes — resting on the bottom edge
// is not overlap).
func overlapsSolid(w *collision.World, box collision.AABB) bool {
	for _, s := range w.Geometry().Segments {
		lo := s.X1
		hi := s.X2
		if s.X2 < s.X1 {
			lo, hi = s.X2, s.X1
		}
		sy1 := s.Y1
		sy2 := s.Y2
		if sy2 < sy1 {
			sy1, sy2 = sy2, sy1
		}
		if lo < box.MaxX && hi > box.MinX && sy1 < box.MaxY && sy2 > box.MinY {
			return true
		}
	}
	return false
}

// floorUnder returns the highest walkable surface height over the
// footprint interval [xa, xb], or false when no walkable segment covers
// the footprint.
func floorUnder(w *collision.World, xa, xb int64) (int64, bool) {
	best := int64(-1)
	for _, s := range w.Geometry().Segments {
		if !s.Kind.IsWalkable() || s.X1 == s.X2 {
			continue
		}
		if s.X1 > xa || s.X2 < xb {
			continue // must cover the whole footprint
		}
		ya := geometry.SurfaceHeight(s, xa)
		yb := geometry.SurfaceHeight(s, xb)
		y := ya
		if yb > y {
			y = yb
		}
		if y > best {
			best = y
		}
	}
	return best, best >= 0
}
