// Package maps loads the committed canonical geometry files
// (<space_id>.geom.json, physics_geometry_contract.md §7.3) into validated
// *geometry.Geometry values keyed by space id for collision.ResolveMove.
//
// Files are embedded at compile time: a .geom.json that fails
// geometry.Parse against its config.SpaceRecord is a broken build input, not
// a runtime condition — callers treat a load error as fatal.
package maps

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"thinhthan/internal/config"
	"thinhthan/internal/sim/spatial/geometry"
)

//go:embed *.geom.json
var embedded embed.FS

// SpaceIDs returns the sorted ids of every committed space geometry.
func SpaceIDs() []string {
	entries, err := embedded.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("maps: embedded dir unreadable: %v", err))
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".geom.json") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".geom.json"))
		}
	}
	sort.Strings(ids)
	return ids
}

// Raw returns the committed canonical bytes for spaceID.
func Raw(spaceID string) ([]byte, error) {
	data, err := embedded.ReadFile(spaceID + ".geom.json")
	if err != nil {
		return nil, fmt.Errorf("maps: %s: %w", spaceID, err)
	}
	return data, nil
}

// LoadOne parses and validates the committed geometry of one space against
// its compiled SpaceRecord (rec.Anchors must be the §6 required set).
func LoadOne(spaceID string, rec config.SpaceRecord) (*geometry.Geometry, error) {
	data, err := Raw(spaceID)
	if err != nil {
		return nil, err
	}
	g, err := geometry.Parse(data, rec)
	if err != nil {
		return nil, fmt.Errorf("maps: %s: %w", spaceID, err)
	}
	return g, nil
}

// Load parses every committed space against its SpaceRecord; recs must carry
// an entry per committed file.
func Load(recs map[string]config.SpaceRecord) (map[string]*geometry.Geometry, error) {
	out := make(map[string]*geometry.Geometry, len(recs))
	for _, id := range SpaceIDs() {
		rec, ok := recs[id]
		if !ok {
			return nil, fmt.Errorf("maps: %s has committed geometry but no SpaceRecord", id)
		}
		g, err := LoadOne(id, rec)
		if err != nil {
			return nil, err
		}
		out[id] = g
	}
	return out, nil
}
