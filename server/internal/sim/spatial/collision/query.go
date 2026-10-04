package collision

import (
	"thinhthan/internal/sim/spatial/geometry"
)

// MoveOpts carries the per-move call-site parameters.
type MoveOpts struct {
	// StepHeightMM bounds the ledge/slope rise a move may mount in one tick
	// (contract §4.1 MAX_STEP_HEIGHT = 300mm).
	StepHeightMM int64
	// DropIgnorePlatformID is a ONE_WAY_PLATFORM id currently being
	// dropped through; it stops blocking while Tick < DropIgnoreUntilTick.
	DropIgnorePlatformID int64
	// DropIgnoreUntilTick is the tick (exclusive) at which the ignored
	// platform resumes blocking.
	DropIgnoreUntilTick uint64
	// Tick is the authoritative server tick of the move.
	Tick uint64
}

// Contact records one resolved surface contact.
type Contact struct {
	SegmentID int64
	Kind      geometry.SegmentKind
	X, Y      int64 // quantized contact point
}

// Result is the deterministic outcome of ResolveMove.
type Result struct {
	Final         AABB
	VxZeroed      bool // horizontal motion stopped by a wall
	VyZeroed      bool // vertical motion stopped by floor/ceiling
	Grounded      bool // feet resting on a walkable surface (gap <= 1mm)
	GroundSegment int64
	OnOneWay      bool
	Contacts      []Contact

	illegal bool // input state already penetrated blocking geometry
}

// ResolveMove sweeps box by (dx, dy) against the world: the horizontal sweep
// runs first (walls stop it; ledges <= StepHeightMM are mounted), then the
// vertical sweep resolves floor/ceiling contact. Integration follows
// contract §4.2; results depend only on (world, box, dx, dy, opts).
func (w *World) ResolveMove(box AABB, dx, dy int64, opts MoveOpts) Result {
	res := Result{Final: box, GroundSegment: -1}
	cur := box

	// --- Phase 1: X sweep -------------------------------------------------
	if dx != 0 {
		var stop bool
		cur, stop = w.sweepX(cur, dx, opts, &res)
		if stop {
			res.VxZeroed = true
		}
	}

	// --- Phase 2: Y sweep -------------------------------------------------
	cur = w.sweepY(cur, dy, opts, &res)
	res.Final = cur

	// --- Ground re-evaluation ----------------------------------------------
	_, seg, ok := w.groundAt(cur, opts.Tick, -1)
	if ok {
		res.Grounded = true
		res.GroundSegment = seg.ID
		res.OnOneWay = seg.Kind == geometry.OneWayPlatform
	}
	return res
}

// GroundAt reports the id of the walkable segment the box is standing on and
// its quantized surface height — the highest surface under the foot span
// within 1mm of the feet (gap <= 1mm counts as touching). Returns ok=false
// when airborne. ignoreID >= 0 skips that one-way platform at this tick.
func (w *World) GroundAt(box AABB, tick uint64, ignoreID int64) (y int64, seg int64, ok bool) {
	sy, s, ok := w.groundAt(box, tick, ignoreID)
	return sy, s.ID, ok
}

// groundAt is GroundAt returning the full segment record for internal use.
func (w *World) groundAt(box AABB, tick uint64, ignoreID int64) (int64, geometry.Segment, bool) {
	_ = tick
	best := int64(-1)
	var hit geometry.Segment
	w.forEach(footSpan(box), func(s geometry.Segment) {
		if !s.Kind.IsWalkable() || s.X1 == s.X2 {
			return
		}
		if ignoreID >= 0 && s.Kind == geometry.OneWayPlatform && s.ID == ignoreID {
			return
		}
		top, ok := surfaceMax(s, box.MinX, box.MaxX)
		if !ok {
			return
		}
		// Standing: feet at or within 1mm above the surface (gap <= 1 = touch).
		if box.MinY < top-1 || box.MinY > top+1 {
			return
		}
		if top > best || (top == best && s.ID < hit.ID) {
			best, hit = top, s
		}
	})
	if best < 0 {
		return 0, geometry.Segment{}, false
	}
	return best, hit, true
}

// footSpan widens the box by the 1mm contact epsilon so grazing surfaces are
// still indexed.
func footSpan(b AABB) AABB {
	return AABB{b.MinX - 1, b.MinY - 1, b.MaxX + 1, b.MaxY + 1}
}

// surfaceMax returns the highest quantized surface height of non-vertical
// segment s over the x-interval [xa, xb]. ok=false when there is no overlap.
func surfaceMax(s geometry.Segment, xa, xb int64) (int64, bool) {
	lo := max64(s.X1, xa)
	hi := min64(s.X2, xb)
	if lo > hi {
		return 0, false
	}
	ya := geometry.SurfaceHeight(s, lo)
	yb := geometry.SurfaceHeight(s, hi)
	if yb > ya {
		ya = yb
	}
	return ya, true
}

// surfaceMin is the mirror for ceilings (lowest surface over the interval).
func surfaceMin(s geometry.Segment, xa, xb int64) (int64, bool) {
	lo := max64(s.X1, xa)
	hi := min64(s.X2, xb)
	if lo > hi {
		return 0, false
	}
	ya := geometry.SurfaceHeight(s, lo)
	yb := geometry.SurfaceHeight(s, hi)
	if yb < ya {
		ya = yb
	}
	return ya, true
}

// SweepX performs the horizontal phase alone: returns the post-sweep box and
// whether motion was stopped by a wall. Contacts are not recorded — use
// ResolveMove for the integrated move.
func (w *World) SweepX(box AABB, dx int64, opts MoveOpts) (AABB, bool) {
	res := Result{}
	return w.sweepX(box, dx, opts, &res)
}

// SweepY performs the vertical phase alone.
func (w *World) SweepY(box AABB, dy int64, opts MoveOpts) AABB {
	res := Result{}
	return w.sweepY(box, dy, opts, &res)
}

// sweepX moves the box horizontally by dx, stopping at the earliest wall
// contact (exact rational ordering, ties to the lowest segment id) and
// mounting ledges whose top is within opts.StepHeightMM.
func (w *World) sweepX(box AABB, dx int64, opts MoveOpts, res *Result) (AABB, bool) {
	cur := box
	remaining := dx // signed travel left
	for i := 0; i < len(w.segs)+1 && remaining != 0; i++ {
		dist, wall, hit, pen := w.xContact(cur, remaining)
		if !hit {
			return cur.Translate(remaining, 0), false
		}
		move := dist * sign64(remaining)
		cur = cur.Translate(move, 0)
		remaining -= move
		if pen {
			res.illegal = true
		}
		wallTop := max64(wall.Y1, wall.Y2)
		step := wallTop - cur.MinY
		// A mountable ledge needs walkable support at the wall's top within
		// step height — a bare wall top is not a stair.
		if step > 0 && step <= opts.StepHeightMM && w.hasSupport(cur, wallTop) {
			cur.MinY += step
			cur.MaxY += step
			res.Contacts = append(res.Contacts, Contact{
				SegmentID: wall.ID, Kind: wall.Kind,
				X: edgeX(cur, remaining), Y: wallTop,
			})
			continue
		}
		// Stop at the wall (non-penetrating side; quantized once).
		res.Contacts = append(res.Contacts, Contact{
			SegmentID: wall.ID, Kind: wall.Kind,
			X: edgeX(cur, remaining), Y: cur.MinY,
		})
		return cur, true
	}
	if remaining != 0 {
		cur = cur.Translate(remaining, 0)
	}
	return cur, false
}

// xContact finds the earliest WALL the box's leading edge reaches within
// |dx| millimeters. Returns the penetration distance cx (absolute mm the
// edge travels to reach the wall), the segment, whether any contact exists,
// and whether the start position already penetrated the wall.
func (w *World) xContact(box AABB, dx int64) (int64, geometry.Segment, bool, bool) {
	adir := abs64(dx)
	best := int64(-1)
	var wall geometry.Segment
	var pen bool
	probe := box.Translate(dx, 0)
	probeBox := AABB{
		min64(box.MinX, probe.MinX), box.MinY - 1,
		max64(box.MaxX, probe.MaxX), box.MaxY + 1,
	}
	w.forEach(probeBox, func(s geometry.Segment) {
		if s.Kind != geometry.Wall {
			return
		}
		// Vertical overlap is strict: sharing an edge does not block.
		if box.MinY >= max64(s.Y1, s.Y2) || box.MaxY <= min64(s.Y1, s.Y2) {
			return
		}
		var xw int64
		if s.X1 == s.X2 {
			xw = s.X1
		} else {
			xw = wallXAtBoxSpan(s, box, dx)
		}
		var dist int64
		if dx > 0 {
			dist = xw - box.MaxX
		} else {
			dist = box.MinX - xw
		}
		if dist < 0 {
			// Edge past the wall line: only a penetration when the wall still
			// lies inside the box span. A wall fully behind the moving box
			// does not block it.
			inside := xw > box.MinX && xw < box.MaxX
			if inside && (best < 0 || s.ID < wall.ID) {
				best, wall, pen = 0, s, true
			}
			return
		}
		if dist > adir {
			return
		}
		if best < 0 || dist < best || (dist == best && s.ID < wall.ID) {
			best, wall, pen = dist, s, false
		}
	})
	if best < 0 {
		return 0, geometry.Segment{}, false, false
	}
	return best, wall, true, pen
}

// wallXAtBoxSpan returns the wall's x coordinate at the y where a moving
// box first meets it: for dx>0 the minimum wall x over the y-overlap, for
// dx<0 the maximum. x(y) is linear in y, so the extremum is at an interval
// endpoint; each endpoint is quantized via RoundDiv.
func wallXAtBoxSpan(s geometry.Segment, box AABB, dx int64) int64 {
	yLo := max64(min64(s.Y1, s.Y2), box.MinY)
	yHi := min64(max64(s.Y1, s.Y2), box.MaxY)
	dy := s.Y2 - s.Y1
	xAt := func(y int64) int64 {
		return s.X1 + geometry.RoundDiv((y-s.Y1)*(s.X2-s.X1), dy)
	}
	if dx > 0 {
		return min64(xAt(yLo), xAt(yHi))
	}
	return max64(xAt(yLo), xAt(yHi))
}

// hasSupport reports whether a walkable surface exists at ~y (±1mm) under
// the box's foot span — the ledge condition for auto step-up.
func (w *World) hasSupport(box AABB, y int64) bool {
	found := false
	w.forEach(box, func(s geometry.Segment) {
		if found || !s.Kind.IsWalkable() || s.X1 == s.X2 {
			return
		}
		if top, ok := surfaceMax(s, box.MinX, box.MaxX); ok && top >= y-1 && top <= y+1 {
			found = true
		}
	})
	return found
}

// edgeX is the leading-edge x for a box after travel `remaining` is consumed:
// the face that would next meet a wall in the given direction.
func edgeX(b AABB, remaining int64) int64 {
	if remaining > 0 {
		return b.MaxX
	}
	return b.MinX
}

// sweepY resolves vertical motion after the X phase. Order per contract
// §4.2: first, any feet-below-surface penetration up to StepHeightMM is
// lifted (auto-step/slope follow); then the dy sweep lands on floors
// (falling) or clamps to ceilings (rising).
func (w *World) sweepY(box AABB, dy int64, opts MoveOpts, res *Result) AABB {
	cur := box

	// Penetration resolve / step-up: highest walkable surface above the feet.
	if lift, seg, ok := w.penetratingSurface(cur, opts); ok {
		// Resolve to the non-penetrating side. On a SLOPE the box entered
		// laterally, so the lift is ordinary ground-following at any size;
		// on flat surfaces a lift beyond the step height means the input
		// was teleported into the floor — an illegal-move verdict.
		if lift-cur.MinY > opts.StepHeightMM && seg.Kind != geometry.Slope {
			res.illegal = true
		}
		d := lift - cur.MinY
		cur.MinY += d
		cur.MaxY += d
		res.Contacts = append(res.Contacts, Contact{
			SegmentID: seg.ID, Kind: seg.Kind, X: cur.MinX, Y: lift,
		})
	}

	switch {
	case dy < 0:
		fall := -dy
		best := int64(-1)
		var seg geometry.Segment
		w.forEach(AABB{cur.MinX, cur.MinY - fall, cur.MaxX, cur.MaxY}, func(s geometry.Segment) {
			if !s.Kind.IsWalkable() || s.X1 == s.X2 {
				return
			}
			if w.oneWayIgnored(s, opts) {
				return
			}
			top, ok := surfaceMax(s, cur.MinX, cur.MaxX)
			if !ok || top > cur.MinY+1 || top < cur.MinY-fall {
				return
			}
			// One-way platforms block only when the previous-tick feet were
			// at or above the surface (contract §4.2 one-way rule).
			if s.Kind == geometry.OneWayPlatform && cur.MinY < top {
				return
			}
			if top > best || (top == best && s.ID < seg.ID) {
				best, seg = top, s
			}
		})
		if best >= 0 {
			d := cur.MinY - best
			cur.MinY -= d
			cur.MaxY -= d
			res.VyZeroed = true
			res.Contacts = append(res.Contacts, Contact{
				SegmentID: seg.ID, Kind: seg.Kind, X: cur.MinX, Y: best,
			})
		} else {
			cur = cur.Translate(0, -fall)
		}
	case dy > 0:
		bestTop := int64(-1)
		var seg geometry.Segment
		w.forEach(AABB{cur.MinX, cur.MinY, cur.MaxX, cur.MaxY + dy}, func(s geometry.Segment) {
			if s.Kind != geometry.Ceiling || s.X1 == s.X2 {
				return
			}
			c, ok := surfaceMin(s, cur.MinX, cur.MaxX)
			if !ok || c < cur.MaxY || c > cur.MaxY+dy {
				return
			}
			if bestTop < 0 || c < bestTop || (c == bestTop && s.ID < seg.ID) {
				bestTop, seg = c, s
			}
		})
		if bestTop >= 0 {
			d := bestTop - cur.MaxY
			cur.MaxY += d
			cur.MinY += d
			res.VyZeroed = true
			res.Contacts = append(res.Contacts, Contact{
				SegmentID: seg.ID, Kind: seg.Kind, X: cur.MinX, Y: bestTop,
			})
		} else {
			cur = cur.Translate(0, dy)
		}
	}
	return cur
}

// penetratingSurface returns the highest walkable surface whose top is
// strictly above the feet (a state only reachable by walking into a rise
// during the X phase or by a bad spawn).
func (w *World) penetratingSurface(box AABB, opts MoveOpts) (int64, geometry.Segment, bool) {
	best := int64(-1)
	var seg geometry.Segment
	w.forEach(box, func(s geometry.Segment) {
		if !s.Kind.IsWalkable() || s.X1 == s.X2 {
			return
		}
		if w.oneWayIgnored(s, opts) {
			return
		}
		top, ok := surfaceMax(s, box.MinX, box.MaxX)
		if !ok || top <= box.MinY {
			return
		}
		// A surface strictly above the feet only penetrates when the box's
		// foot span actually overlaps its x-extent — guaranteed by surfaceMax.
		if s.Kind == geometry.OneWayPlatform {
			// One-way surfaces never penetrate upward.
			return
		}
		if best < 0 || top > best || (top == best && s.ID < seg.ID) {
			best, seg = top, s
		}
	})
	if best < 0 {
		return 0, geometry.Segment{}, false
	}
	return best, seg, true
}

// oneWayIgnored reports whether the platform is inside the caller's
// drop-ignore window for this tick.
func (w *World) oneWayIgnored(s geometry.Segment, opts MoveOpts) bool {
	return s.Kind == geometry.OneWayPlatform &&
		s.ID == opts.DropIgnorePlatformID &&
		opts.Tick < opts.DropIgnoreUntilTick
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func sign64(v int64) int64 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
