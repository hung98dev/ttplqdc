package world

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"thinhthan/internal/sim/spatial/geometry"
	"thinhthan/internal/sim/spatial/maps"
	"thinhthan/internal/sim/spatial/parity"
)

// catalog.go — the content adapter. The active compiled payload's canonical
// rows (world_map / space_geometry / portal / checkpoint / npc /
// npc_service_route emits) plus each map's committed geometry file produce
// the MapCatalog read surface. Composition decodes the payload with
// parity.LoadPayload; the adapter itself is pure — no I/O of its own.

// snapshotCatalog is the compiled MapCatalog.
type snapshotCatalog struct {
	recs        map[string]MapRecord
	order       []string
	portals     map[string]PortalDef
	checkpoints map[string]CheckpointDef
}

// CatalogFromPayload builds the catalog from the decoded canonical payload.
// World maps are the `world_map` rows; every map's committed geometry
// (maps.LoadOne) supplies bounds, layout profile, required topology and the
// anchor set — a map with no committed geometry is a hard error, never a
// silently incomplete catalog.
func CatalogFromPayload(p *parity.Payload) (MapCatalog, error) {
	if p == nil {
		return nil, fmt.Errorf("world: nil payload")
	}
	geomRecs := p.Records()
	geoms := map[string]*geometry.Geometry{}
	rows := map[string]parity.SpaceRow{}
	for i := range p.Geometry {
		r := &p.Geometry[i]
		rows[r.SpaceID] = *r
	}

	c := &snapshotCatalog{
		recs:        map[string]MapRecord{},
		portals:     map[string]PortalDef{},
		checkpoints: map[string]CheckpointDef{},
	}

	for _, rec := range p.Definitions["world_map"] {
		mid := sField(rec, "map_id")
		row, ok := rows[mid]
		if !ok {
			return nil, fmt.Errorf("world: %s has no space_geometry row", mid)
		}
		grec, ok := geomRecs[mid]
		if !ok {
			return nil, fmt.Errorf("world: %s has no SpaceRecord", mid)
		}
		g, err := maps.LoadOne(mid, grec)
		if err != nil {
			return nil, err
		}
		geoms[mid] = g
		anchors := make([]geometry.Anchor, len(g.Anchors))
		copy(anchors, g.Anchors)
		mr := MapRecord{
			MapID:            mid,
			Kind:             MapKind(sField(rec, "type")),
			Region:           regionKey(mid),
			BoundsMaxX:       row.BoundsMaxMM[0],
			BoundsMaxY:       row.BoundsMaxMM[1],
			LayoutProfile:    row.LayoutProfile,
			RequiredTopology: row.RequiredTopo,
			Anchors:          anchors,
			EntrySpawn:       sField(rec, "entry_spawn"),
			RecLevelLo:       iField(rec, "rec_level_lo"),
			RecLevelHi:       iField(rec, "rec_level_hi"),
		}
		if _, ok := mr.Anchor(mr.EntrySpawn); !ok {
			return nil, fmt.Errorf("world: %s entry_spawn %q not in committed geometry", mid, mr.EntrySpawn)
		}
		c.recs[mid] = mr
		c.order = append(c.order, mid)
	}
	sort.Strings(c.order)

	for _, rec := range p.Definitions["portal"] {
		pd := PortalDef{
			PortalID:     sField(rec, "portal_id"),
			SourceMap:    sField(rec, "source_map"),
			DestMap:      sField(rec, "destination"),
			DestSpawn:    sField(rec, "destination_spawn"),
			AnchorID:     sField(rec, "portal_id"),
			RequireFlag:  sField(rec, "requirement_flag"),
			RequireLevel: iField(rec, "requirement_level"),
		}
		src, ok := c.recs[pd.SourceMap]
		if !ok {
			// Portals anchored on non-world spaces (dungeon exits etc.)
			// are not this runtime's concern.
			continue
		}
		// World→world portals pin the destination spawn anchor; portals
		// into non-world spaces (dungeon entries) are kept as authored
		// but are unusable until their destination runtime exists —
		// consult/apply paths reject them at use time.
		if _, ok := c.recs[pd.DestMap]; ok {
			dst := c.recs[pd.DestMap]
			if _, ok := dst.Anchor(pd.DestSpawn); !ok {
				return nil, fmt.Errorf("world: portal %s destination spawn %q missing on %s",
					pd.PortalID, pd.DestSpawn, pd.DestMap)
			}
		}
		if _, ok := src.Anchor(pd.AnchorID); !ok {
			return nil, fmt.Errorf("world: portal %s anchor missing on %s", pd.PortalID, pd.SourceMap)
		}
		c.portals[pd.PortalID] = pd
		src.Portals = append(src.Portals, pd)
		c.recs[pd.SourceMap] = src
	}
	for mid := range c.recs {
		mr := c.recs[mid]
		sort.Slice(mr.Portals, func(i, j int) bool { return mr.Portals[i].PortalID < mr.Portals[j].PortalID })
		c.recs[mid] = mr
	}

	for _, rec := range p.Definitions["checkpoint"] {
		cd := CheckpointDef{
			CheckpointID: sField(rec, "checkpoint_id"),
			MapID:        sField(rec, "map_id"),
			AnchorID:     sField(rec, "checkpoint_id"),
		}
		mr, ok := c.recs[cd.MapID]
		if !ok {
			return nil, fmt.Errorf("world: checkpoint %s map %q is not a world map", cd.CheckpointID, cd.MapID)
		}
		if _, ok := mr.Anchor(cd.AnchorID); !ok {
			return nil, fmt.Errorf("world: checkpoint %s anchor missing on %s", cd.CheckpointID, cd.MapID)
		}
		c.checkpoints[cd.CheckpointID] = cd
		mr.Checkpoints = append(mr.Checkpoints, cd)
		c.recs[cd.MapID] = mr
	}

	services := map[string][]string{}
	for _, rec := range p.Definitions["npc_service_route"] {
		npc := sField(rec, "npc_id")
		services[npc] = append(services[npc], sField(rec, "service_id"))
	}
	for _, rec := range p.Definitions["npc"] {
		nd := NpcDef{
			NpcID:    sField(rec, "npc_id"),
			MapID:    sField(rec, "map_id"),
			AnchorID: sField(rec, "npc_id"),
			Services: services[sField(rec, "npc_id")],
		}
		mr, ok := c.recs[nd.MapID]
		if !ok {
			continue // NPCs on non-world maps are out of scope
		}
		if _, ok := mr.Anchor(nd.AnchorID); !ok {
			return nil, fmt.Errorf("world: npc %s anchor missing on %s", nd.NpcID, nd.MapID)
		}
		mr.Npcs = append(mr.Npcs, nd)
		c.recs[nd.MapID] = mr
	}
	for mid := range c.recs {
		mr := c.recs[mid]
		sort.Slice(mr.Npcs, func(i, j int) bool { return mr.Npcs[i].NpcID < mr.Npcs[j].NpcID })
		c.recs[mid] = mr
	}
	return c, nil
}

// regionKey extracts the progression-region segment of `map.<region>.<name>` —
// the Addressables `region.<zone_key>` group key (client_assets.md).
func regionKey(mapID string) string {
	parts := strings.Split(mapID, ".")
	if len(parts) == 3 {
		return parts[1]
	}
	return ""
}

// --- payload row helpers ----------------------------------------------------

func sField(rec map[string]any, k string) string {
	s, _ := rec[k].(string)
	return s
}

func iField(rec map[string]any, k string) int32 {
	n, _ := rec[k].(json.Number)
	i, _ := n.Int64()
	return int32(i)
}

func (c *snapshotCatalog) Lookup(mapID string) (MapRecord, bool) {
	mr, ok := c.recs[mapID]
	return mr, ok
}

func (c *snapshotCatalog) Maps() []MapRecord {
	out := make([]MapRecord, len(c.order))
	for i, mid := range c.order {
		out[i] = c.recs[mid]
	}
	return out
}

func (c *snapshotCatalog) Checkpoint(checkpointID string) (CheckpointDef, bool) {
	cd, ok := c.checkpoints[checkpointID]
	return cd, ok
}

func (c *snapshotCatalog) Portal(portalID string) (PortalDef, bool) {
	pd, ok := c.portals[portalID]
	return pd, ok
}
