package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileWorldRoute — world_route_catalog.md driver (11 bindings): route
// shape, checkpoints, 24 map metadata rows, canonical bounds/layout, chests,
// 18 edge + 5 story-gate + 5 dungeon + 1 finale portals, village objects,
// fishing spots, portal-count validation.
func compileWorldRoute(c *Ctx, f *File, r *Registry) {
	st := &wrState{fieldMaps: map[string]bool{}, towns: map[string]bool{}}
	c.Data["worldroute.state"] = st
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Shared Rules"):
			wrSharedRules(c, f, b)
		case strings.HasPrefix(path, "Checkpoints"):
			wrCheckpoints(c, f, b)
		case strings.HasPrefix(path, "Map Metadata"):
			wrMapMetadata(c, f, b, st)
		case strings.HasPrefix(path, "Canonical Bounds"):
			wrBounds(c, f, b, st)
		case strings.HasPrefix(path, "Hidden Chests"):
			wrChests(c, f, b, st)
		case strings.HasPrefix(path, "Regional Route Edges"):
			wrEdges(c, f, b, st)
		case strings.HasPrefix(path, "Cross-Region Story Gates"):
			wrStoryGates(c, f, b, st)
		case strings.HasPrefix(path, "Dungeon Entrances"):
			wrDungeonEntrances(c, f, b, st)
		case strings.HasPrefix(path, "Act-VI Finale Entry"):
			wrFinaleEntry(c, f, b, st)
		case strings.HasPrefix(path, "Village Objects"):
			wrVillageObjects(c, f, b, st)
		case strings.HasPrefix(path, "Portal Count Validation"):
			wrPortalCount(c, f, b, st)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"world_route binding %q has no driver", b.Raw)
		}
	}
}

type wrState struct {
	fieldMaps map[string]bool
	towns     map[string]bool
	portals   int
}

// mapSuffix strips the leading `map.` for portal IDs.
func mapSuffix(id string) string { return strings.TrimPrefix(id, "map.") }

func pairRat(raw string) (config.Value, bool) {
	parts := strings.Split(raw, "x")
	if len(parts) != 2 {
		return config.Value{}, false
	}
	a, err1 := parseDecimal(parts[0])
	b, err2 := parseDecimal(parts[1])
	if err1 != nil || err2 != nil {
		return config.Value{}, false
	}
	return config.VRec(map[string]config.Value{"x": mustRat(a), "y": mustRat(b)}), true
}

func wrSharedRules(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				c.EmitParam(f.Name, b.Raw, "route_shape",
					[]config.Value{config.VInt(int64(j))},
					map[string]config.Value{"rule": config.VStr(l)}, fb.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

var checkpointLineRe = regexp.MustCompile(`^(checkpoint\.[a-z0-9_.]+)\s*->\s*(map\.[a-z0-9_.]+)\s*$`)

func wrCheckpoints(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := checkpointLineRe.FindStringSubmatch(l); m != nil {
					c.Emit(f.Name, b.Raw, "checkpoint",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{
							"checkpoint_id": config.VStr(m[1]),
							"map_id":        config.VStr(m[2]),
						}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

func wrMapMetadata(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "map_id,type,recommended,entry_spawn,first_discovery_exp"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				mid := cellAt(row, 0).Scalar()
				typ := cellAt(row, 1).Scalar()
				rec := cellAt(row, 2).Scalar()
				m := levelBandRe.FindStringSubmatch(rec)
				if m == nil {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
						"recommended %q want a-b", rec)
					continue
				}
				lo, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				hi, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
				expV, eerr := (TypeSpec{Name: "grouped_int"}).ParseValue(cellAt(row, 4).Scalar())
				if eerr != nil {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"first_discovery_exp %q", cellAt(row, 4).Scalar())
					continue
				}
				if typ == "FIELD" {
					st.fieldMaps[mid] = true
				} else if typ == "TOWN" {
					st.towns[mid] = true
				}
				c.Emit(f.Name, b.Raw, "world_map",
					[]config.Value{config.VStr(mid)},
					map[string]config.Value{
						"map_id":              config.VStr(mid),
						"type":                config.VStr(typ),
						"rec_level_lo":        config.VInt(lo.Int),
						"rec_level_hi":        config.VInt(hi.Int),
						"entry_spawn":         config.VStr(cellAt(row, 3).Scalar()),
						"first_discovery_exp": config.VInt(expV.Int),
					}, row[0].Line)
				c.Emit(f.Name, b.Raw, "discovery_reward",
					[]config.Value{config.VStr(mid)},
					map[string]config.Value{
						"map_id":       config.VStr(mid),
						"reward_key":   config.VStr("reward.discovery." + mid + ".<character_id>"),
						"exp":          config.VInt(expV.Int),
						"share_of_act": config.VStr("0.2%"),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func wrBounds(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "map_id,span (screens),bounds max (m),reference extent (px),layout_profile,required traversable topology"); tbl != nil {
			seenSpan, seenProfile := map[string]string{}, map[string]string{}
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				mid := cellAt(row, 0).Scalar()
				span, ok1 := pairRat(cellAt(row, 1).Scalar())
				bounds, ok2 := pairRat(cellAt(row, 2).Scalar())
				ext, ok3 := pairInt(cellAt(row, 3).Scalar())
				if !ok1 || !ok2 || !ok3 {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"bounds row %s has malformed pair cells", mid)
					continue
				}
				if prev, dup := seenSpan[cellAt(row, 1).Scalar()]; dup {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
						"span %s duplicates %s", cellAt(row, 1).Scalar(), prev)
				}
				seenSpan[cellAt(row, 1).Scalar()] = mid
				prof := cellAt(row, 4).Scalar()
				if prev, dup := seenProfile[prof]; dup {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
						"layout_profile %s duplicates %s", prof, prev)
				}
				seenProfile[prof] = mid
				c.Emit(f.Name, b.Raw, "space_geometry",
					[]config.Value{config.VStr(mid)},
					map[string]config.Value{
						"space_id":            config.VStr(mid),
						"kind":                config.VStr("FIELD_OR_TOWN"),
						"span_screens":        span,
						"bounds_max_m":        bounds,
						"reference_extent_px": ext,
						"layout_profile":      config.VStr(prof),
						"required_topology":   config.VStr(cellAt(row, 5).Scalar()),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func pairInt(raw string) (config.Value, bool) {
	parts := strings.Split(raw, "x")
	if len(parts) != 2 {
		return config.Value{}, false
	}
	a, e1 := (TypeSpec{Name: "int"}).ParseValue(parts[0])
	b2, e2 := (TypeSpec{Name: "int"}).ParseValue(parts[1])
	if e1 != nil || e2 != nil {
		return config.Value{}, false
	}
	return config.VRec(map[string]config.Value{"x": a, "y": b2}), true
}

var chestFieldRe = regexp.MustCompile(`^([a-z_]+)\s*=\s*(.+)$`)
var edgeRe = regexp.MustCompile(`^(map\.[a-z0-9_.]+)\s*<->\s*(map\.[a-z0-9_.]+)\s*$`)
var relicRe = regexp.MustCompile(`^(anchor\.relic\.[a-z0-9_.]+)\s+on\s+(map\.[a-z0-9_.]+)\s*$`)

func wrChests(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		// first-session fence declares enrichment for one chest of the
		// launch pair — collect before emitting so fields merge.
		firstFields := map[string]string{}
		firstLine := 0
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := chestFieldRe.FindStringSubmatch(l); m != nil {
					if firstLine == 0 {
						firstLine = fb.Line + 1 + j
					}
					firstFields[m[1]] = strings.TrimSpace(m[2])
				}
			}
		}
		firstID := firstFields["chest_id"]
		// launch pair: 2 chests per FIELD map (data-dependent expansion)
		for mid := range st.fieldMaps {
			for _, n := range []string{"01", "02"} {
				cid := "chest.hidden." + mid + "." + n
				out := map[string]config.Value{
					"chest_id":       config.VStr(cid),
					"map_id":         config.VStr(mid),
					"drop_table":     config.VStr("drop.chest.hidden"),
					"requires_perch": config.VBool(true),
				}
				line := sec.Line
				if cid == firstID {
					out["first_session_visible"] = config.VBool(true)
					out["spot_from"] = config.VStr(firstFields["spot_from"])
					if rg, eerr := (TypeSpec{Name: "int"}).ParseValue(firstFields["spot_range_m"]); eerr == nil {
						out["spot_range_m"] = rg
					}
					out["path_los"] = config.VStr(firstFields["path_los"])
					line = firstLine
				}
				c.Emit(f.Name, b.Raw, "chest",
					[]config.Value{config.VStr(cid)}, out, line)
			}
		}
		if tbl := findTable(sec, "chest_id,map_id,season_region_index"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				sidx, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 2).Scalar())
				c.Emit(f.Name, b.Raw, "chest",
					[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
					map[string]config.Value{
						"chest_id":            config.VStr(cellAt(row, 0).Scalar()),
						"map_id":              config.VStr(cellAt(row, 1).Scalar()),
						"season_region_index": sidx,
						"seasonal":            config.VBool(true),
						"drop_table":          config.VStr("drop.chest.hidden"),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

// emitPortal writes one directional portal record.
func (st *wrState) emitPortal(c *Ctx, f *File, b *SourceBinding, pid, src, dst, dstSpawn string, req map[string]config.Value, line int) {
	fields := map[string]config.Value{
		"portal_id":         config.VStr(pid),
		"source_map":        config.VStr(src),
		"destination":       config.VStr(dst),
		"destination_spawn": config.VStr(dstSpawn),
	}
	for k, v := range req {
		fields[k] = v
	}
	st.portals++
	c.Emit(f.Name, b.Raw, "portal", []config.Value{config.VStr(pid)}, fields, line)
}

func wrEdges(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				m := edgeRe.FindStringSubmatch(l)
				if m == nil {
					continue
				}
				a, bb := m[1], m[2]
				st.emitPortal(c, f, b,
					"portal."+mapSuffix(a)+".to."+mapSuffix(bb),
					a, bb, "spawn.entry."+mapSuffix(bb), nil, fb.Line+1+j)
				st.emitPortal(c, f, b,
					"portal."+mapSuffix(bb)+".to."+mapSuffix(a),
					bb, a, "spawn.entry."+mapSuffix(a), nil, fb.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

var storyReqRe = regexp.MustCompile("`(progression\\.story\\.a[a-z0-9_.]+)`\\s*and\\s*Level\\s*([0-9]+)")

func wrStoryGates(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "edge,forward requirement"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				m := edgeRe.FindStringSubmatch(cellAt(row, 0).Scalar())
				if m == nil {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"story-gate edge %q", cellAt(row, 0).Text)
					continue
				}
				reqText := cellAt(row, 1).Text
				rm := storyReqRe.FindStringSubmatch(reqText)
				if rm == nil {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"forward requirement %q", reqText)
					continue
				}
				lv, _ := (TypeSpec{Name: "int"}).ParseValue(rm[2])
				req := map[string]config.Value{
					"requirement_flag":  config.VStr(rm[1]),
					"requirement_level": lv,
				}
				a, bb := m[1], m[2]
				st.emitPortal(c, f, b,
					"portal."+mapSuffix(a)+".to."+mapSuffix(bb),
					a, bb, "spawn.entry."+mapSuffix(bb), req, row[0].Line)
				st.emitPortal(c, f, b,
					"portal."+mapSuffix(bb)+".to."+mapSuffix(a),
					bb, a, "spawn.entry."+mapSuffix(a), nil, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

// parsePortalBlock reads `portal.<id>` heading + indented `k = v` fields.
func parsePortalBlock(fb *Block) []map[string]string {
	var blocks []map[string]string
	var cur map[string]string
	for _, l := range fb.FLines {
		if strings.HasPrefix(l, "portal.") && !strings.Contains(l, " = ") {
			cur = map[string]string{"id": strings.TrimSpace(l)}
			blocks = append(blocks, cur)
			continue
		}
		if cur == nil {
			continue
		}
		if m := chestFieldRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			cur[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return blocks
}

func wrDungeonEntrances(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			var sawBlock bool
			for _, pb := range parsePortalBlock(fb) {
				sawBlock = true
				req := map[string]config.Value{}
				if pb["requirement"] != "" {
					req["requirement_flag"] = config.VStr(pb["requirement"])
				}
				dst := pb["destination"]
				dstSpawn := ""
				if strings.HasPrefix(dst, "map.") {
					dstSpawn = "spawn.entry." + mapSuffix(dst)
				}
				if pb["return_spawn"] != "" {
					req["return_spawn"] = config.VStr(pb["return_spawn"])
				}
				st.emitPortal(c, f, b, pb["id"], pb["source"], dst, dstSpawn, req, fb.Line+1)
			}
			if sawBlock {
				continue
			}
			// return-spawn pattern fence + relic anchors fence
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := relicRe.FindStringSubmatch(l); m != nil {
					c.Emit(f.Name, b.Raw, "relic_anchor",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{
							"anchor_id": config.VStr(m[1]),
							"map_id":    config.VStr(m[2]),
						}, fb.Line+1+j)
					continue
				}
				if m := regexp.MustCompile(`^(spawn\.return\.[a-z0-9_.<>]+)`).FindStringSubmatch(l); m != nil {
					c.EmitParam(f.Name, b.Raw, "return_spawn_pattern",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{"pattern": config.VStr(m[1])}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

func wrFinaleEntry(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, pb := range parsePortalBlock(fb) {
				req := map[string]config.Value{
					"requirement_flag": config.VStr(pb["requirement"]),
					"kind":             config.VStr("finale"),
				}
				dst := pb["destination space_id"]
				if dst == "" {
					dst = pb["destination encounter"]
				}
				if pb["return_spawn"] != "" {
					req["return_spawn"] = config.VStr(pb["return_spawn"])
				}
				if pb["destination encounter"] != "" {
					req["destination_encounter"] = config.VStr(pb["destination encounter"])
				}
				st.emitPortal(c, f, b, pb["id"], pb["source"], dst, "", req, fb.Line+1)
			}
		}
		for _, tbl := range allTables(sec) {
			if len(tbl.Headers) == 0 || tbl.Headers[0] != "space_id" {
				continue
			}
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				span, _ := pairRat(cellAt(row, 2).Scalar())
				bounds, _ := pairRat(cellAt(row, 3).Scalar())
				ext, _ := pairInt(cellAt(row, 4).Scalar())
				c.Emit(f.Name, b.Raw, "space_geometry",
					[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
					map[string]config.Value{
						"space_id":            config.VStr(cellAt(row, 0).Scalar()),
						"kind":                config.VStr(cellAt(row, 1).Scalar()),
						"span_screens":        span,
						"bounds_max_m":        bounds,
						"reference_extent_px": ext,
						"layout_profile":      config.VStr(cellAt(row, 5).Scalar()),
						"required_topology":   config.VStr(cellAt(row, 6).Scalar()),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func wrVillageObjects(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	for _, sec := range bindingSections(c, f, b) {
		// 3 object kinds x each TOWN (data-dependent expansion)
		for mid := range st.towns {
			for _, kind := range []string{"bonfire", "cooking_hearth", "sparring_ring"} {
				id := kind + "." + mid
				c.Emit(f.Name, b.Raw, "village_object",
					[]config.Value{config.VStr(id)},
					map[string]config.Value{
						"object_id": config.VStr(id),
						"kind":      config.VStr(kind),
						"map_id":    config.VStr(mid),
					}, sec.Line)
			}
		}
		// `## has_water and fishing spots` child: table
		for _, ch := range sec.Children {
			if tbl := findTable(ch, "map_id,has_water,spots"); tbl != nil {
				for ri := range tbl.Cells {
					row := tbl.Cells[ri]
					mid := cellAt(row, 0).Scalar()
					hasWater := cellAt(row, 1).Scalar()
					if mid == "all other launch maps" || hasWater != "true" {
						continue
					}
					spots := cellAt(row, 2).Text
					var idx int
					for _, tok := range strings.Split(spots, ",") {
						tok = strings.TrimSpace(strings.Trim(tok, "`"))
						if strings.HasPrefix(tok, ".") {
							tok = "fishing_spot." + mid + tok
						}
						if !strings.HasPrefix(tok, "fishing_spot.") {
							continue
						}
						idx++
						c.Emit(f.Name, b.Raw, "fishing_spot",
							[]config.Value{config.VStr(tok)},
							map[string]config.Value{
								"spot_id":  config.VStr(tok),
								"map_id":   config.VStr(mid),
								"ordering": config.VInt(int64(idx)),
							}, row[0].Line)
					}
					c.Emit(f.Name, b.Raw, "map_water",
						[]config.Value{config.VStr(mid)},
						map[string]config.Value{
							"map_id":     config.VStr(mid),
							"has_water":  config.VBool(true),
							"spot_count": config.VInt(int64(idx)),
						}, row[0].Line)
				}
			}
		}
	}
	c.consumed(f, b)
}

var portalCountRe = regexp.MustCompile(`^(\d+)\s+(.+)$`)

func wrPortalCount(c *Ctx, f *File, b *SourceBinding, st *wrState) {
	var declared int64
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "=") {
					m := regexp.MustCompile(`^=\s*(\d+)`).FindStringSubmatch(l)
					if m != nil {
						v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
						declared = v.Int
					}
					continue
				}
				if m := portalCountRe.FindStringSubmatch(l); m != nil {
					v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
					c.EmitParam(f.Name, b.Raw, "portal_count_component",
						[]config.Value{config.VStr(m[2])},
						map[string]config.Value{
							"component": config.VStr(m[2]),
							"count":     v,
						}, fb.Line+1)
				}
			}
		}
	}
	if declared > 0 && int64(st.portals) != declared {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, b.Line,
			"authored portals %d != declared %d", st.portals, declared)
	}
	c.consumed(f, b)
}
