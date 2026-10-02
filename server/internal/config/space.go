package config

// SpaceRecord is one compiled playable-space geometry record
// (physics_geometry_contract.md §6.1, config.md § Map Geometry Compile).
// Bounds are quantized integer millimeters; spans are exact screen rationals.
type SpaceRecord struct {
	SpaceID       string
	SpaceKind     string // WORLD | DUNGEON | FINALE
	LayoutProfile string
	SceneKey      string
	BoundsMM      struct {
		MaxX int64
		MaxY int64
	}
	Span struct {
		Width  Rat
		Height Rat
	}
	Anchors          []string
	RequiredTopology string
}
