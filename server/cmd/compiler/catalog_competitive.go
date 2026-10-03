package main

// Competitive-space spec-section sources (CAT-006, ADR-0080): pvp.md and
// guild_war.md register `Compiler Source Schema` bindings that compile into
// geometry.spaces rows for the PVP and GUILD_WAR space kinds (physics
// contract §6.1 five-kind enum) plus their declared logical anchor sets
// (export parity, contract §6). They are additional registered compile
// inputs, not gameplay catalogs — the 24-file §3 manifest is unchanged.

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// markCompetitive records a declared competitive space so the playable-space
// validator expects its geometry row (data-driven, never a fixed count).
func markCompetitive(c *Ctx, id string) {
	set, _ := c.Data["competitive.spaces"].(map[string]bool)
	if set == nil {
		set = map[string]bool{}
		c.Data["competitive.spaces"] = set
	}
	set[id] = true
}

// bindingSpaceID extracts the registered space key from a binding's output
// cell (`space anchors / `map.pvp.five_element_arena“).
func bindingSpaceID(b *SourceBinding) string {
	parts := strings.Split(b.Output, "/")
	return strings.Trim(strings.TrimSpace(parts[len(parts)-1]), "` ")
}

// bareIDRe matches one ordered logical anchor id line (`altar.left`,
// `guild_war.seal.moc`) inside a source `text` fence.
var bareIDRe = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)+$`)

// spaceAnchorIDs collects the ordered anchor id set a `space anchors`
// binding declares: every bare dotted-id line in the resolved sections'
// text fences, in authored order.
func spaceAnchorIDs(c *Ctx, f *File, b *SourceBinding) []string {
	var ids []string
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if bareIDRe.MatchString(l) {
					ids = append(ids, l)
				}
			}
		}
	}
	return ids
}

// strVals maps strings to config values (anchors / modes lists).
func strVals(ss []string) []config.Value {
	out := make([]config.Value, 0, len(ss))
	for _, s := range ss {
		out = append(out, config.VStr(s))
	}
	return out
}

// emitCompetitiveSpace emits one geometry.spaces row for a PVP/GUILD_WAR
// space: the contract §6.1 field set plus `modes` and the declared
// `anchors` list (empty when the space declares no logical anchors).
func emitCompetitiveSpace(c *Ctx, f *File, b *SourceBinding, id, kind string,
	span, bounds, ext config.Value, profile, topology string,
	anchorIDs, modeIDs []string, line int) {
	fields := map[string]config.Value{
		"space_id":            config.VStr(id),
		"kind":                config.VStr(kind),
		"span_screens":        span,
		"bounds_max_m":        bounds,
		"reference_extent_px": ext,
		"layout_profile":      config.VStr(profile),
		"required_topology":   config.VStr(topology),
		"anchors":             config.VList(strVals(anchorIDs)...),
	}
	if modeIDs != nil {
		fields["modes"] = config.VList(strVals(modeIDs)...)
	}
	c.EmitGeom(f.Name, b.Raw, []config.Value{config.VStr(id)}, fields, line)
	markCompetitive(c, id)
}

// ---- pvp.md ---------------------------------------------------------------

var modeTokenRe = regexp.MustCompile(`[A-Z][A-Z0-9_]+`)

// compilePvp — `../03_systems/pvp.md` (CAT-006). The Competitive Space
// Geometry table emits one geometry.spaces row per registered mode row
// (kind=PVP constant); the Five Element Arena `altar.*` fence declares the
// ordered anchor set of `map.pvp.five_element_arena` (`map.pvp.duel_court`
// declares no logical anchors).
func compilePvp(c *Ctx, f *File, r *Registry) {
	anchors := map[string][]string{}
	for _, b := range r.Bindings {
		if strings.HasPrefix(b.Output, "space anchors") {
			anchors[bindingSpaceID(b)] = spaceAnchorIDs(c, f, b)
			c.consumed(f, b)
		}
	}
	emitted := map[string]bool{}
	for _, b := range r.Bindings {
		if !strings.HasPrefix(b.Output, "space geometry") {
			continue
		}
		for _, sec := range bindingSections(c, f, b) {
			tbl := findTable(sec, "modes,space_id,span (screens),bounds max (m),reference extent (px),layout_profile,required topology")
			if tbl == nil {
				c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
					"Competitive Space Geometry table missing")
				continue
			}
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				modes := modeTokenRe.FindAllString(cellAt(row, 0).Text, -1)
				id := cellAt(row, 1).Scalar()
				span, ok1 := pairRat(cellAt(row, 2).Scalar())
				bounds, ok2 := pairRat(cellAt(row, 3).Scalar())
				ext, ok3 := pairInt(cellAt(row, 4).Scalar())
				if id == "" || !ok1 || !ok2 || !ok3 {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"competitive space geometry row %q malformed", id)
					continue
				}
				emitCompetitiveSpace(c, f, b, id, "PVP", span, bounds, ext,
					cellAt(row, 5).Scalar(), strings.TrimSpace(cellAt(row, 6).Text),
					anchors[id], modes, row[0].Line)
				emitted[id] = true
			}
		}
		c.consumed(f, b)
	}
	orphanAnchors(c, f, anchors, emitted)
}

// orphanAnchors flags `space anchors` bindings whose registered space never
// got a geometry row — the declared anchor set would drop silently.
func orphanAnchors(c *Ctx, f *File, anchors map[string][]string, emitted map[string]bool) {
	for id := range anchors {
		if !emitted[id] {
			c.Diags.Addf(config.DiagIntegrationCheck, f.Path, 0,
				"space anchors declared for %q but no space geometry row emitted", id)
		}
	}
}

// ---- guild_war.md ----------------------------------------------------------

var (
	// `5.00 x 1.75 reference screens`
	gwSpanRe = regexp.MustCompile(`([0-9.]+)\s*x\s*([0-9.]+)`)
	// `(0,0)..(128.0m,25.2m)` — decimal corner pair, optional `m` suffix
	gwBoundsRe = regexp.MustCompile(`\(\s*([0-9.]+)\s*m?\s*,\s*([0-9.]+)\s*m?\s*\)\s*\.\.\s*\(\s*([0-9.]+)\s*m?\s*,\s*([0-9.]+)\s*m?\s*\)`)
	// `6400x1260px at 50 px/m`
	gwExtentRe = regexp.MustCompile(`([0-9]+)\s*x\s*([0-9]+)`)
)

// compileGuildWar — `../03_systems/guild_war.md` (CAT-006). The `Map`
// section's canonical-geometry fence emits the GUILD_WAR space row; the
// required-topology bullets are normative validation text emitted as the
// row's `required_topology`; the Stable objective IDs fence declares the
// ordered anchor set.
func compileGuildWar(c *Ctx, f *File, r *Registry) {
	anchors := map[string][]string{}
	for _, b := range r.Bindings {
		if strings.HasPrefix(b.Output, "space anchors") {
			anchors[bindingSpaceID(b)] = spaceAnchorIDs(c, f, b)
			c.consumed(f, b)
		}
	}
	emitted := map[string]bool{}
	for _, b := range r.Bindings {
		if !strings.HasPrefix(b.Output, "space geometry") {
			continue
		}
		for _, sec := range bindingSections(c, f, b) {
			fb := gwGeomFence(sec)
			if fb == nil {
				c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
					"Map section has no canonical geometry fence")
				continue
			}
			if id := emitGuildWarSpace(c, f, b, sec, fb, anchors); id != "" {
				emitted[id] = true
			}
		}
		c.consumed(f, b)
	}
	orphanAnchors(c, f, anchors, emitted)
}

// gwGeomFence finds the `Map` section's canonical geometry fence — the
// `text` fence carrying `space_id =`.
func gwGeomFence(sec *Section) *Block {
	for _, fb := range allFences(sec, "text") {
		for _, l := range fb.FLines {
			if chestFieldRe.MatchString(strings.TrimSpace(l)) &&
				strings.HasPrefix(strings.TrimSpace(l), "space_id") {
				return fb
			}
		}
	}
	return nil
}

// gwRequiredTopology joins the `Required topology:` bullet block that
// follows the geometry fence — normative validation text per the registry's
// defaults cell.
func gwRequiredTopology(sec *Section) string {
	var parts []string
	collecting := false
	for _, bl := range sec.Content {
		if bl.Kind != BlockProse {
			continue
		}
		for _, l := range bl.Prose {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, "Required topology") {
				collecting = true
				continue
			}
			if !collecting {
				continue
			}
			if strings.HasPrefix(t, "- ") {
				parts = append(parts, strings.TrimRight(strings.TrimSpace(t[2:]), ";"))
			} else if t != "" {
				collecting = false
			}
		}
	}
	return strings.Join(parts, "; ")
}

// emitGuildWarSpace parses the canonical-geometry fence's `key = value`
// lines and emits the single GUILD_WAR space row. Returns the emitted
// space id, or "" when the fence is malformed.
func emitGuildWarSpace(c *Ctx, f *File, b *SourceBinding, sec *Section, fb *Block, anchors map[string][]string) string {
	kv := map[string]string{}
	for j, l := range fb.FLines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		m := chestFieldRe.FindStringSubmatch(l)
		if m == nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, fb.Line+1+j,
				"guild war geometry line %q not key = value", l)
			continue
		}
		kv[m[1]] = strings.TrimSpace(m[2])
	}
	id := strings.Trim(kv["space_id"], "` ")
	var span, bounds, ext config.Value
	ok := true
	if m := gwSpanRe.FindStringSubmatch(kv["span"]); m != nil {
		if span, ok = pairRat(m[1] + "x" + m[2]); !ok {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, fb.Line,
				"guild war span %q malformed", kv["span"])
		}
	} else {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, fb.Line,
			"guild war geometry fence missing span")
		ok = false
	}
	if m := gwBoundsRe.FindStringSubmatch(kv["bounds"]); m != nil {
		var minX, minY, maxX, maxY config.Rat
		var e error
		if minX, e = parseDecimal(m[1]); e != nil {
			ok = false
		}
		if minY, e = parseDecimal(m[2]); e != nil {
			ok = false
		}
		if maxX, e = parseDecimal(m[3]); e != nil {
			ok = false
		}
		if maxY, e = parseDecimal(m[4]); e != nil {
			ok = false
		}
		if !ok {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, fb.Line,
				"guild war bounds %q malformed", kv["bounds"])
		} else if minX.Num != 0 || minY.Num != 0 {
			// bounds min is a registered constant `(0,0)`
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, fb.Line,
				"guild war bounds min (%s,%s) != (0,0)", minX, minY)
			ok = false
		} else {
			bounds = config.VRec(map[string]config.Value{
				"x": mustRat(maxX), "y": mustRat(maxY),
			})
		}
	} else {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, fb.Line,
			"guild war geometry fence missing bounds")
		ok = false
	}
	if m := gwExtentRe.FindStringSubmatch(kv["reference_extent"]); m != nil {
		if ext, ok = pairInt(m[1] + "x" + m[2]); !ok {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, fb.Line,
				"guild war reference_extent %q malformed", kv["reference_extent"])
		}
	} else {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, fb.Line,
			"guild war geometry fence missing reference_extent")
		ok = false
	}
	if id == "" {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, fb.Line,
			"guild war geometry fence missing space_id")
		ok = false
	}
	if !ok {
		return ""
	}
	emitCompetitiveSpace(c, f, b, id, "GUILD_WAR", span, bounds, ext,
		kv["layout_profile"], gwRequiredTopology(sec),
		anchors[id], nil, fb.Line)
	return id
}
