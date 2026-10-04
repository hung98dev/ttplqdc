// Package collision implements the deterministic axis-swept AABB collider
// over geometry.Segment space data (physics_geometry_contract.md §3-§5).
// All arithmetic is integer millimeter math; ordering decisions use exact
// rational comparisons so a given (World, input) pair resolves identically
// on every platform.
package collision

// AABB is an axis-aligned bounding box in millimeter coordinates.
type AABB struct {
	MinX, MinY int64
	MaxX, MaxY int64
}

// Translate returns the box moved by (dx, dy).
func (b AABB) Translate(dx, dy int64) AABB {
	return AABB{b.MinX + dx, b.MinY + dy, b.MaxX + dx, b.MaxY + dy}
}

// Intersects reports whether two boxes overlap with positive area. Edge
// contact alone does not count as intersection.
func (b AABB) Intersects(o AABB) bool {
	return b.MinX < o.MaxX && b.MaxX > o.MinX && b.MinY < o.MaxY && b.MaxY > o.MinY
}

// ContainsPoint reports whether (x, y) is inside the box, edges inclusive.
func (b AABB) ContainsPoint(x, y int64) bool {
	return x >= b.MinX && x <= b.MaxX && y >= b.MinY && y <= b.MaxY
}

// Width / Height are the box extents in millimeters.
func (b AABB) Width() int64  { return b.MaxX - b.MinX }
func (b AABB) Height() int64 { return b.MaxY - b.MinY }
