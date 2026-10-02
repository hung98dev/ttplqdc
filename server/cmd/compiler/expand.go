package main

import (
	"thinhthan/internal/config"
)

// runExpansions performs the post-driver finite passes (contract §4):
// folds every declared space geometry into the geometry.spaces index so
// all 30 playable PvE spaces resolve an exact bounds/layout row.
func runExpansions(c *Ctx) {
	// geometry.spaces index: space_geometry defs (24 maps + 1 finale)
	// plus the 5 dungeon spaces already emitted by EmitGeom.
	fam := c.Defs.Families["space_geometry"]
	if fam != nil {
		for _, key := range fam.SortedKeys() {
			rec := fam.Records[config.KeyString(key)]
			c.EmitGeom("world_route_catalog.md", "Canonical Bounds and Layout Profiles",
				rec.Key, rec.Fields, 0)
		}
	}
}
