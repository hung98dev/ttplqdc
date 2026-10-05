// Package parity derives the canonical .geom.json bytes for every playable
// space from its Unity collision scene and checks them against the compiled
// content payload (physics_geometry_contract.md §6-§7, ADR-0071).
//
// Required anchor sets (F-2): the catalog-emitted `geometry` family only
// carries `anchors` for competitive spaces; every other space's required set
// is derived here as the union of all anchor-bearing record IDs keyed to that
// space (contract §7.3 anchors entry).
package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"thinhthan/internal/config"
)

// Payload is the compiler's canonical `-payload` output restricted to the
// fields parity consumes.
type Payload struct {
	Definitions map[string][]map[string]any
	Geometry    []SpaceRow
}

// SpaceRow is one row of the payload's `geometry` list.
type SpaceRow struct {
	SpaceID       string
	Kind          string
	LayoutProfile string
	SceneKey      string
	BoundsMaxMM   [2]int64 // x, y in integer millimeters
	Span          [2]config.Rat
	Anchors       []string
	RequiredTopo  string
	Modes         []string
}

// LoadPayload decodes a `compiler -payload` JSON document.
func LoadPayload(data []byte) (*Payload, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw struct {
		Definitions map[string][]map[string]any `json:"definitions"`
		Geometry    []map[string]any            `json:"geometry"`
	}
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parity: payload decode: %w", err)
	}
	p := &Payload{Definitions: raw.Definitions}
	for _, gr := range raw.Geometry {
		row, err := decodeSpaceRow(gr)
		if err != nil {
			return nil, err
		}
		p.Geometry = append(p.Geometry, *row)
	}
	return p, nil
}

// decodeSpaceRow maps one emitted geometry row onto the canonical fields,
// quantizing bounds_max_m (rational meters) to integer millimeters.
func decodeSpaceRow(m map[string]any) (*SpaceRow, error) {
	row := &SpaceRow{
		SpaceID:       strField(m, "space_id"),
		Kind:          strField(m, "kind"),
		LayoutProfile: strField(m, "layout_profile"),
		SceneKey:      strField(m, "scene_key"),
		RequiredTopo:  strField(m, "required_topology"),
	}
	if row.SpaceID == "" {
		return nil, fmt.Errorf("parity: geometry row missing space_id")
	}
	bm, ok := m["bounds_max_m"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parity: %s missing bounds_max_m", row.SpaceID)
	}
	for i, axis := range []string{"x", "y"} {
		mm, err := ratMM(bm[axis])
		if err != nil {
			return nil, fmt.Errorf("parity: %s bounds_max_m.%s: %w", row.SpaceID, axis, err)
		}
		row.BoundsMaxMM[i] = mm
	}
	if ss, ok := m["span_screens"].(map[string]any); ok {
		for i, axis := range []string{"x", "y"} {
			r, err := ratField(ss[axis])
			if err != nil {
				return nil, fmt.Errorf("parity: %s span_screens.%s: %w", row.SpaceID, axis, err)
			}
			row.Span[i] = r
		}
	}
	row.Anchors = strList(m["anchors"])
	row.Modes = strList(m["modes"])
	return row, nil
}

// ToSpaceRecord converts the payload row into the config.SpaceRecord that
// geometry.Parse validates against; the anchor set comes from RequiredAnchors
// for spaces the compiler does not emit anchors for (PvE).
func (r *SpaceRow) ToSpaceRecord(anchors []string) config.SpaceRecord {
	var rec config.SpaceRecord
	rec.SpaceID = r.SpaceID
	rec.SpaceKind = r.Kind
	rec.LayoutProfile = r.LayoutProfile
	rec.SceneKey = r.SceneKey
	rec.BoundsMM.MaxX = r.BoundsMaxMM[0]
	rec.BoundsMM.MaxY = r.BoundsMaxMM[1]
	rec.Span.Width = r.Span[0]
	rec.Span.Height = r.Span[1]
	rec.Anchors = anchors
	rec.RequiredTopology = r.RequiredTopo
	return rec
}

// anchorRule keys an anchor-bearing record to its space and extracts the
// anchor id field(s). Key fields may carry a space id directly
// (map_id/dungeon_id/space_id/source_map).
type anchorRule struct {
	family   string
	keyField string
	ids      func(rec map[string]any) []string
}

func singleID(field string) func(map[string]any) []string {
	return func(rec map[string]any) []string {
		if v, ok := rec[field].(string); ok && v != "" {
			return []string{v}
		}
		return nil
	}
}

// anchorRules enumerates every anchor-bearing emitted family and how its ids
// key to a space (contract §7.3 anchor list + §6 derivation).
var anchorRules = []anchorRule{
	{"spawn_anchor", "map_id", singleID("anchor_id")},
	{"spawn_group", "map_id", singleID("anchor_id")},
	{"world_map", "map_id", singleID("entry_spawn")},
	{"checkpoint", "map_id", singleID("checkpoint_id")},
	{"portal", "source_map", singleID("portal_id")},
	{"daily_anchor", "map_id", singleID("anchor_id")},
	{"quest_anchor", "map_id", singleID("anchor_id")},
	{"relic_anchor", "map_id", singleID("anchor_id")},
	{"chest", "map_id", singleID("chest_id")},
	{"fishing_spot", "map_id", singleID("spot_id")},
	{"village_object", "map_id", singleID("object_id")},
	{"npc", "map_id", singleID("npc_id")},
	{"dungeon_stage", "dungeon_id", func(rec map[string]any) []string {
		var out []string
		for _, w := range anyList(rec["waves"]) {
			if wm, ok := w.(map[string]any); ok {
				out = append(out, singleID("anchor_id")(wm)...)
			}
		}
		return out
	}},
	{"boss_anchor", "map_id", singleID("anchor_id")},
	// Every INSTANCED boss owns an arena anchor anchor.boss.<key> keyed to its
	// space (boss_catalog.md Roster rule); the compiler does not yet emit a
	// boss_anchor record for instanced rows, so derive it from the boss row.
	{"boss", "space_id", func(rec map[string]any) []string {
		if rec["mode"] != "INSTANCED" {
			return nil
		}
		id, _ := rec["boss_id"].(string)
		key := strings.TrimPrefix(id, "boss.")
		if key == id {
			return nil
		}
		return []string{"anchor.boss." + key}
	}},
}

// RequiredAnchors returns the sorted required-anchor id set per space: the
// union of every anchor-bearing record keyed to the space plus the emitted
// `anchors` on competitive geometry rows.
func (p *Payload) RequiredAnchors() map[string][]string {
	sets := make(map[string]map[string]struct{}, len(p.Geometry))
	for _, g := range p.Geometry {
		sets[g.SpaceID] = map[string]struct{}{}
		for _, a := range g.Anchors {
			sets[g.SpaceID][a] = struct{}{}
		}
	}
	for _, rule := range anchorRules {
		for _, rec := range p.Definitions[rule.family] {
			key, _ := rec[rule.keyField].(string)
			if key == "" {
				continue
			}
			set, ok := sets[key]
			if !ok {
				// Record keyed to a space that is not a playable space
				// (zone ids, un-playable targets): not an anchor obligation.
				continue
			}
			for _, id := range rule.ids(rec) {
				set[id] = struct{}{}
			}
		}
	}
	out := make(map[string][]string, len(sets))
	for space, set := range sets {
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out[space] = ids
	}
	return out
}

// Records builds the config.SpaceRecord per space with derived anchor sets.
func (p *Payload) Records() map[string]config.SpaceRecord {
	req := p.RequiredAnchors()
	recs := make(map[string]config.SpaceRecord, len(p.Geometry))
	for i := range p.Geometry {
		row := &p.Geometry[i]
		recs[row.SpaceID] = row.ToSpaceRecord(req[row.SpaceID])
	}
	return recs
}

// ---- decode helpers --------------------------------------------------------

func strField(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func strList(v any) []string {
	var out []string
	for _, e := range anyList(v) {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func anyList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

// ratField reads a JSON number or {numerator, denominator} object into a Rat.
func ratField(v any) (config.Rat, error) {
	switch t := v.(type) {
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return config.Rat{}, fmt.Errorf("non-integer %v", v)
		}
		return config.ReduceRat(n, 1)
	case map[string]any:
		num, ok1 := t["numerator"].(json.Number)
		den, ok2 := t["denominator"].(json.Number)
		if !ok1 || !ok2 {
			return config.Rat{}, fmt.Errorf("bad rational %v", v)
		}
		n, err := num.Int64()
		if err != nil {
			return config.Rat{}, err
		}
		d, err := den.Int64()
		if err != nil {
			return config.Rat{}, err
		}
		return config.ReduceRat(n, d)
	default:
		return config.Rat{}, fmt.Errorf("bad rational %v", v)
	}
}

// ratMM converts a meters value (integer or rational) into integer mm.
func ratMM(v any) (int64, error) {
	r, err := ratField(v)
	if err != nil {
		return 0, err
	}
	num := r.Num * 1000
	if num%r.Den != 0 {
		return 0, fmt.Errorf("meters %v not millimeter-exact", v)
	}
	return num / r.Den, nil
}
