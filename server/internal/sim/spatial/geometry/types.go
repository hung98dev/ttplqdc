// Package geometry loads and validates immutable 2-D side-scroller space
// geometry (physics_geometry_contract.md §1-7). All coordinates are integer
// millimeters in the space's own origin-at-bottom-left frame; slopes are
// tested with the contract's rational thresholds (§2.4), never trig.
package geometry

// SpaceKind identifies the space's play template. Values map 1:1 onto the
// schema's string enum and onto config.SpaceRecord.SpaceKind.
type SpaceKind int

const (
	SpaceKindFieldOrTown SpaceKind = iota // FIELD_OR_TOWN
	SpaceKindDungeon                      // DUNGEON
	SpaceKindFinale                       // FINALE
	SpaceKindPVP                          // PVP
	SpaceKindGuildWar                     // GUILD_WAR
)

var spaceKindNames = [...]string{
	"FIELD_OR_TOWN",
	"DUNGEON",
	"FINALE",
	"PVP",
	"GUILD_WAR",
}

// String returns the schema/config spelling of the kind.
func (k SpaceKind) String() string {
	if k < 0 || int(k) >= len(spaceKindNames) {
		return "UNKNOWN"
	}
	return spaceKindNames[k]
}

// ParseSpaceKind maps a schema/config kind string to its enum value.
func ParseSpaceKind(s string) (SpaceKind, bool) {
	for i, name := range spaceKindNames {
		if s == name {
			return SpaceKind(i), true
		}
	}
	return 0, false
}

// SegmentKind classifies a collider segment per contract §3/§7 slope rules.
type SegmentKind int

const (
	// SolidGround is a walkable floor with |slope| <= 5°.
	SolidGround SegmentKind = iota
	// Slope is a walkable incline with 5° < |slope| <= 45°.
	Slope
	// Wall blocks horizontal motion: vertical (x1==x2, y1<y2) or |slope| > 45°.
	Wall
	// Ceiling blocks upward motion with |slope| <= 45°.
	Ceiling
	// OneWayPlatform is a floor (|slope| <= 5°) that only stops falling bodies
	// whose feet were above the surface on the previous tick.
	OneWayPlatform
)

var segmentKindNames = [...]string{
	"SOLID_GROUND",
	"SLOPE",
	"WALL",
	"CEILING",
	"ONE_WAY_PLATFORM",
}

// String returns the schema spelling of the kind.
func (k SegmentKind) String() string {
	if k < 0 || int(k) >= len(segmentKindNames) {
		return "UNKNOWN"
	}
	return segmentKindNames[k]
}

// ParseSegmentKind maps a schema kind string to its enum value.
func ParseSegmentKind(s string) (SegmentKind, bool) {
	for i, name := range segmentKindNames {
		if s == name {
			return SegmentKind(i), true
		}
	}
	return 0, false
}

// IsWalkable reports whether the kind forms a surface a character can stand on.
func (k SegmentKind) IsWalkable() bool {
	return k == SolidGround || k == Slope || k == OneWayPlatform
}

// Segment is a single collider edge in millimeter coordinates. For every kind
// except a vertical Wall, x1 < x2. A vertical Wall uses x1 == x2 and y1 < y2.
type Segment struct {
	ID     int64
	Kind   SegmentKind
	X1, Y1 int64
	X2, Y2 int64
}

// CameraRegion is a camera-visible rectangle in millimeter coordinates.
type CameraRegion struct {
	ID         int64
	MinX, MinY int64
	MaxX, MaxY int64
}

// Anchor is a spawn/anchor point; y is the walkable floor height beneath it.
type Anchor struct {
	ID   string
	X, Y int64
}

// Geometry is an immutable, parsed-and-validated space geometry snapshot. It
// is keyed by (SpaceID, ContentRevision) and must not be mutated after Parse.
type Geometry struct {
	SpaceID         string
	Kind            SpaceKind
	LayoutProfile   string
	ContentRevision string
	BoundsMM        struct {
		MaxX int64
		MaxY int64
	}
	Segments      []Segment // sorted by ID
	CameraRegions []CameraRegion
	Anchors       []Anchor

	schemaVersion int64 // parsed document version; validated == 1
}
