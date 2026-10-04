package geometry

import (
	"errors"
	"fmt"
	"sort"

	"thinhthan/internal/config"
)

// Sentinel categories for parse/validation failures.
var (
	// ErrMalformed marks syntactic failures: bad JSON, wrong types,
	// non-integer numbers, unknown keys, missing required keys.
	ErrMalformed = errors.New("geometry: malformed document")
	// ErrSchema marks contract violations against schema v1 or the
	// registered SpaceRecord (bounds, slope rules, coverage, anchors).
	ErrSchema = errors.New("geometry: schema violation")
)

// Error is a path-tagged geometry failure naming the JSON field path.
type Error struct {
	Path     string
	Msg      string
	sentinel error
}

// Error returns "<path>: <msg>".
func (e *Error) Error() string { return e.Path + ": " + e.Msg }

// Unwrap returns the sentinel category (ErrMalformed or ErrSchema).
func (e *Error) Unwrap() error { return e.sentinel }

// Contract constants (physics_geometry_contract.md §4.1, §6.2, §7).
const (
	// MinCameraRegionXMM / MinCameraRegionYMM: a camera region must hold at
	// least one reference screen (25.6 m x 14.4 m at 1 mm/unit).
	MinCameraRegionXMM = 25600
	MinCameraRegionYMM = 14400
)

// Validate checks a parsed Geometry against schema v1 rules and the
// registered SpaceRecord. It returns the first *Error found.
func Validate(g *Geometry, rec *config.SpaceRecord) error {
	if g.schemaVersion != 1 {
		return schemaErr("schema_version", fmt.Sprintf("expected 1, got %d", g.schemaVersion))
	}
	if g.SpaceID != rec.SpaceID {
		return schemaErr("space_id", fmt.Sprintf("space %q does not match registered space %q", g.SpaceID, rec.SpaceID))
	}
	if g.Kind.String() != rec.SpaceKind {
		return schemaErr("space_kind", fmt.Sprintf("kind %q does not match registered kind %q", g.Kind, rec.SpaceKind))
	}
	if g.LayoutProfile != rec.LayoutProfile {
		return schemaErr("layout_profile", fmt.Sprintf("profile %q does not match registered profile %q", g.LayoutProfile, rec.LayoutProfile))
	}
	if !isLowerHex64(g.ContentRevision) {
		return schemaErr("content_revision", "expected 64 lowercase hex chars")
	}
	if g.BoundsMM.MaxX <= 0 || g.BoundsMM.MaxY <= 0 {
		return schemaErr("bounds_mm", "bounds must be positive")
	}
	if g.BoundsMM.MaxX != rec.BoundsMM.MaxX || g.BoundsMM.MaxY != rec.BoundsMM.MaxY {
		return schemaErr("bounds_mm", fmt.Sprintf("bounds %dx%d do not match registered %dx%d",
			g.BoundsMM.MaxX, g.BoundsMM.MaxY, rec.BoundsMM.MaxX, rec.BoundsMM.MaxY))
	}
	if err := validateSegments(g); err != nil {
		return err
	}
	if err := validateRegions(g); err != nil {
		return err
	}
	return validateAnchors(g, rec)
}

// validateSegments enforces sort order, per-kind slope rules, endpoint bounds
// and duplicate rejection (contract §7 invariants).
func validateSegments(g *Geometry) error {
	type segKey struct {
		kind           SegmentKind
		x1, y1, x2, y2 int64
	}
	seen := make(map[segKey]struct{}, len(g.Segments))
	for i, s := range g.Segments {
		path := fmt.Sprintf("segments[%d]", i)
		if i > 0 && g.Segments[i-1].ID >= s.ID {
			return schemaErr(path+".id", "segments not sorted by ascending id")
		}
		for _, p := range [][2]int64{{s.X1, s.Y1}, {s.X2, s.Y2}} {
			if p[0] < 0 || p[0] > g.BoundsMM.MaxX || p[1] < 0 || p[1] > g.BoundsMM.MaxY {
				return schemaErr(path, "segment endpoint outside bounds")
			}
		}
		if err := checkSlopeRule(path, s); err != nil {
			return err
		}
		k := segKey{s.Kind, s.X1, s.Y1, s.X2, s.Y2}
		if _, dup := seen[k]; dup {
			return schemaErr(path, "duplicate segment")
		}
		seen[k] = struct{}{}
	}
	return nil
}

// checkSlopeRule applies the contract's rational slope classification:
// |slope| <= 5° iff |dy|*1_000_000 <= |dx|*87_489; <= 45° iff |dy| <= |dx|.
// Vertical is allowed only for WALL (x1 == x2, y1 < y2).
func checkSlopeRule(path string, s Segment) error {
	dx, dy := s.X2-s.X1, s.Y2-s.Y1
	adx, ady := abs64(dx), abs64(dy)
	le5 := adx > 0 && ady*1_000_000 <= adx*87_489
	le45 := ady <= adx // vertical dx==0 only satisfies when ady==0
	if s.X1 == s.X2 {
		if s.Kind != Wall || s.Y1 >= s.Y2 {
			return schemaErr(path, "vertical segment must be WALL with y1 < y2")
		}
		return nil
	}
	if s.X1 >= s.X2 {
		return schemaErr(path, "non-vertical segment requires x1 < x2")
	}
	switch s.Kind {
	case SolidGround, OneWayPlatform:
		if !le5 {
			return schemaErr(path, fmt.Sprintf("kind %s requires |slope| <= 5 degrees", s.Kind))
		}
	case Slope:
		if le5 || !le45 {
			return schemaErr(path, "kind SLOPE requires 5 < |slope| <= 45 degrees")
		}
	case Wall:
		if le45 {
			return schemaErr(path, "kind WALL requires |slope| > 45 degrees")
		}
	case Ceiling:
		if !le45 {
			return schemaErr(path, "kind CEILING requires |slope| <= 45 degrees")
		}
	}
	return nil
}

// validateRegions enforces the camera-region contract: at least one region,
// each within bounds and at least one reference screen in size, and their
// union covering every walkable segment (contract §6.2, §7).
func validateRegions(g *Geometry) error {
	if len(g.CameraRegions) == 0 {
		return schemaErr("camera_regions", "at least one region required")
	}
	seen := make(map[int64]struct{}, len(g.CameraRegions))
	for i, r := range g.CameraRegions {
		path := fmt.Sprintf("camera_regions[%d]", i)
		if _, dup := seen[r.ID]; dup {
			return schemaErr(path+".id", "duplicate region id")
		}
		seen[r.ID] = struct{}{}
		if r.MinX >= r.MaxX || r.MinY >= r.MaxY {
			return schemaErr(path, "degenerate region")
		}
		if r.MinX < 0 || r.MaxX > g.BoundsMM.MaxX || r.MinY < 0 || r.MaxY > g.BoundsMM.MaxY {
			return schemaErr(path, "region outside bounds")
		}
		if r.MaxX-r.MinX < MinCameraRegionXMM || r.MaxY-r.MinY < MinCameraRegionYMM {
			return schemaErr(path, fmt.Sprintf("region smaller than %dx%dmm", MinCameraRegionXMM, MinCameraRegionYMM))
		}
	}
	for i, s := range g.Segments {
		if !s.Kind.IsWalkable() {
			continue
		}
		if !coveredByRegions(s, g.CameraRegions) {
			return schemaErr(fmt.Sprintf("segments[%d]", i), "walkable segment not covered by camera regions")
		}
	}
	return nil
}

// coveredByRegions reports whether every point of segment s lies inside the
// union of regions. Each region covers a contiguous x-interval of s (s is
// linear in x); the union must cover [x1, x2].
func coveredByRegions(s Segment, regions []CameraRegion) bool {
	type iv struct{ lo, hi int64 }
	var ivs []iv
	dx := s.X2 - s.X1
	for _, r := range regions {
		lo := max64(s.X1, r.MinX)
		hi := min64(s.X2, r.MaxX)
		if lo > hi {
			continue
		}
		// Clip to where the surface line stays inside [r.MinY, r.MaxY].
		// y(x) = y1 + (x-x1)(y2-y1)/(x2-x1) is linear in x, so each
		// constraint contributes a contiguous x interval.
		xLo, xHi := lo, hi
		for _, bound := range []struct {
			yb   int64
			need bool // true: y(x) >= yb; false: y(x) <= yb
		}{{r.MinY, true}, {r.MaxY, false}} {
			lo2, hi2 := clipLinear(s.X1, s.Y1, dx, s.Y2-s.Y1, xLo, xHi, bound.yb, bound.need)
			xLo, xHi = lo2, hi2
		}
		if xLo <= xHi {
			ivs = append(ivs, iv{xLo, xHi})
		}
	}
	if len(ivs) == 0 {
		return false
	}
	sort.Slice(ivs, func(i, j int) bool { return ivs[i].lo < ivs[j].lo })
	cur := ivs[0].lo
	if cur > s.X1 {
		return false
	}
	maxHi := ivs[0].hi
	for _, v := range ivs[1:] {
		if v.lo > maxHi+1 {
			return false
		}
		if v.hi > maxHi {
			maxHi = v.hi
		}
	}
	return maxHi >= s.X2
}

// clipLinear narrows [xLo,xHi] to the x where y1 + (x-x1)*dy/dx satisfies the
// bound (need: y(x) >= yb, else y(x) <= yb). Exact rational comparison only.
func clipLinear(x1, y1, dx, dy, xLo, xHi, yb int64, needLower bool) (int64, int64) {
	// Solve linear crossing in integer domain by scanning the boundary of the
	// feasible region: evaluate endpoints exactly; if both fail, empty; if the
	// binding end fails, walk it inward to the greatest feasible x.
	eval := func(x int64) int64 { return y1*dx + (x-x1)*dy } // = y(x)*dx (dx > 0)
	ok := func(x int64) bool {
		v := eval(x)
		if needLower {
			return v >= yb*dx
		}
		return v <= yb*dx
	}
	loOK, hiOK := ok(xLo), ok(xHi)
	if !loOK && !hiOK {
		// Linear: if both endpoints violate the same side there is still a
		// feasible middle only when the line crosses the bound inside, which
		// a monotone function cannot do twice.
		return 1, 0
	}
	if loOK && hiOK {
		return xLo, xHi
	}
	// Exactly one side feasible; find the crossing x by binary search on the
	// monotone predicate (monotone because y(x) is monotone in x).
	a, b := xLo, xHi // ok(a) != ok(b); preserve ok(a)==true
	if !loOK {
		a, b = xHi, xLo
	}
	// a satisfies, b does not; linear monotonicity means the feasible set on
	// [xLo,xHi] is contiguous toward a. Binary search the boundary between
	// a and b regardless of direction.
	lo, hi := min64(a, b), max64(a, b)
	if a == lo { // feasible prefix [lo, p]
		p := lo
		q := hi
		for p < q {
			m := p + (q-p+1)/2
			if ok(m) {
				p = m
			} else {
				q = m - 1
			}
		}
		return lo, p
	}
	// feasible suffix [p, hi]
	p := lo
	q := hi
	for p < q {
		m := p + (q-p)/2
		if ok(m) {
			q = m
		} else {
			p = m + 1
		}
	}
	return p, hi
}

// validateAnchors enforces: unique ids, inside bounds, id set equal to
// SpaceRecord.Anchors, and each anchor standing on a walkable surface
// (contract §7.5: anchor y is the quantized floor height at anchor x).
func validateAnchors(g *Geometry, rec *config.SpaceRecord) error {
	if len(g.Anchors) == 0 {
		return schemaErr("anchors", "at least one anchor required")
	}
	want := make(map[string]bool, len(rec.Anchors))
	for _, id := range rec.Anchors {
		want[id] = true
	}
	seen := make(map[string]bool, len(g.Anchors))
	for i, a := range g.Anchors {
		path := fmt.Sprintf("anchors[%d]", i)
		if seen[a.ID] {
			return schemaErr(path+".id", "duplicate anchor id")
		}
		seen[a.ID] = true
		if !want[a.ID] {
			return schemaErr(path+".id", fmt.Sprintf("anchor %q not in registered space anchors", a.ID))
		}
		if a.X < 0 || a.X > g.BoundsMM.MaxX || a.Y < 0 || a.Y > g.BoundsMM.MaxY {
			return schemaErr(path, "anchor outside bounds")
		}
		if !anchorOnFloor(g, a) {
			return schemaErr(path, "anchor does not stand on a walkable surface")
		}
	}
	if len(seen) != len(want) {
		return schemaErr("anchors", "anchor set does not equal registered space anchors")
	}
	return nil
}

// anchorOnFloor reports whether some walkable segment's quantized surface
// height at a.X equals a.Y (contract §7.5 anchor-under-CHARACTER rule).
func anchorOnFloor(g *Geometry, a Anchor) bool {
	for _, s := range g.Segments {
		if !s.Kind.IsWalkable() || s.X1 == s.X2 {
			continue
		}
		if a.X < s.X1 || a.X > s.X2 {
			continue
		}
		if SurfaceHeight(s, a.X) == a.Y {
			return true
		}
	}
	return false
}

// SurfaceHeight returns the quantized floor height of a non-vertical segment
// at x (contract §2.3): y1 + RoundDiv((x-x1)*(y2-y1), x2-x1).
func SurfaceHeight(s Segment, x int64) int64 {
	return s.Y1 + RoundDiv((x-s.X1)*(s.Y2-s.Y1), s.X2-s.X1)
}

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func schemaErr(path, msg string) error {
	return &Error{Path: path, Msg: msg, sentinel: ErrSchema}
}
