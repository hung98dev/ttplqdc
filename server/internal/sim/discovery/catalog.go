package discovery

import (
	"encoding/json"
	"fmt"

	"thinhthan/internal/sim/spatial/parity"
)

// MapType distinguishes the 6 TOWN safe/social anchors from the 18
// FIELD maps of world_route_catalog.md § Map Metadata.
type MapType string

const (
	MapTown  MapType = "TOWN"
	MapField MapType = "FIELD"
)

// Map is one world_map row: type, recommended band, entry spawn and the
// authored first-discovery EXP slot (×100 scale, fixed content — never
// scaled to character level).
type Map struct {
	MapID        string
	Type         MapType
	EntrySpawn   string
	DiscoveryExp uint64
}

// Catalog is the compiled 24-map first-discovery binding.
type Catalog struct {
	recs map[string]Map
}

// CatalogFromPayload builds the binding from the decoded canonical
// payload's world_map rows. A normal-world map without exactly one
// first-discovery EXP value is a hard error, per the catalog
// invariants.
func CatalogFromPayload(p *parity.Payload) (Catalog, error) {
	if p == nil {
		return Catalog{}, fmt.Errorf("discovery: nil payload")
	}
	c := Catalog{recs: map[string]Map{}}
	for _, rec := range p.Definitions["world_map"] {
		mid := strField(rec, "map_id")
		if mid == "" {
			return Catalog{}, fmt.Errorf("discovery: world_map row without map_id")
		}
		exp, ok := u64Field(rec, "first_discovery_exp")
		if !ok {
			return Catalog{}, fmt.Errorf("discovery: %s has no first-discovery EXP slot", mid)
		}
		c.recs[mid] = Map{
			MapID:        mid,
			Type:         MapType(strField(rec, "type")),
			EntrySpawn:   strField(rec, "entry_spawn"),
			DiscoveryExp: exp,
		}
	}
	if len(c.recs) == 0 {
		return Catalog{}, fmt.Errorf("discovery: payload has no world_map rows")
	}
	return c, nil
}

// Lookup resolves one map's discovery slot.
func (c Catalog) Lookup(mapID string) (Map, bool) {
	m, ok := c.recs[mapID]
	return m, ok
}

// DiscoveryExp resolves the authored first-entry EXP for a map.
func (c Catalog) DiscoveryExp(mapID string) (uint64, bool) {
	m, ok := c.recs[mapID]
	if !ok || m.DiscoveryExp == 0 {
		return 0, false
	}
	return m.DiscoveryExp, true
}

// SafeAnchor resolves a travel-eligible safe/social anchor map (the six
// TOWN rows — world_route_catalog.md § Checkpoints) to its entry spawn.
// FIELD maps are never travel-eligible.
func (c Catalog) SafeAnchor(mapID string) (spawnAnchorID string, ok bool) {
	m, ok := c.recs[mapID]
	if !ok || m.Type != MapTown {
		return "", false
	}
	return m.EntrySpawn, true
}

// Maps returns every bound map id in sorted-emission order is not
// required — callers iterate for catalog assertions in tests.
func (c Catalog) Maps() []Map {
	out := make([]Map, 0, len(c.recs))
	for _, m := range c.recs {
		out = append(out, m)
	}
	return out
}

func strField(rec map[string]any, k string) string {
	s, _ := rec[k].(string)
	return s
}

func u64Field(rec map[string]any, k string) (uint64, bool) {
	switch v := rec[k].(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < 0 {
			return 0, false
		}
		return uint64(n), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case string:
		return 0, false
	default:
		return 0, false
	}
}
