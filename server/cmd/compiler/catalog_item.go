package main

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
	"thinhthan/internal/config"
)

// compileItem — item_catalog.md driver: registered item families emit item
// records (definition fences + Display lines), the Typed Item Use Payloads
// table emits item_use records, fishing catch tables emit catch entries,
// and the bonus-book schedule emits book grants.
func compileItem(c *Ctx, f *File, r *Registry) {
	st := &itemState{items: map[string]bool{}}
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Regional Craft"):
			itemRegionalMaterials(c, f, b, st)
		case strings.Contains(b.Raw, "level-2 heading"):
			itemDefinitions(c, f, b, st)
		case strings.HasPrefix(path, "Recovery Consumables") && strings.Contains(b.Raw, "second"):
			itemRecoveryUses(c, f, b, st)
		case strings.Contains(b.Raw, "Typed Item Use Payloads"):
			itemUsePayloads(c, f, b, st)
		case strings.Contains(b.Raw, "fishing.catch.default"):
			itemCatchDefault(c, f, b, st)
		case strings.Contains(b.Raw, "Seasonal catch"):
			itemCatchSeasonal(c, f, b, st)
		case strings.Contains(b.Raw, "Bonus Book Grant"):
			itemBookGrants(c, f, b, st)
		case strings.Contains(b.Raw, "Cosmetic Redemption"):
			// excluded illustration — consume without emitting
			c.consumed(f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"item binding %q has no driver", b.Raw)
		}
	}
	c.EmitParam(f.Name, "Definition Families and Fields", "rule_versions",
		[]config.Value{config.VStr("item_definition_strings")},
		map[string]config.Value{"version": config.VInt(1)}, 0)
}

type itemState struct {
	items      map[string]bool
	defaults   []config.Value // fishing.catch.default rows
	defaultIDs []string
}

var itemFieldTypes = map[string]string{
	"type":                      "enum",
	"rarity":                    "enum",
	"binding":                   "enum",
	"binding_trigger":           "enum",
	"stack_limit":               "int",
	"shared_cooldown_group":     "enum",
	"item_specific_cooldown_ms": "int",
	"HUNT_eligible":             "bool",
	"discard_allowed":           "bool",
	"tier":                      "enum",
}

func itemFieldValue(name, raw string) config.Value {
	switch itemFieldTypes[name] {
	case "int":
		v, _ := (TypeSpec{Name: "int"}).ParseValue(raw)
		return v
	case "bool":
		return config.VBool(raw == "true")
	default:
		return config.VStr(raw)
	}
}

// parseDefinitionFence reads `k = v` lines from a text fence.
func parseDefinitionFence(c *Ctx, f *File, flines []string, line int) map[string]config.Value {
	fields := map[string]config.Value{}
	for j, l := range flines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		m := itemFieldRe.FindStringSubmatch(l)
		if m == nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line+1+j,
				"item definition line %q", l)
			continue
		}
		name, val := m[1], strings.TrimSpace(m[2])
		if _, known := itemFieldTypes[name]; !known {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line+1+j,
				"unknown item field %q", name)
			continue
		}
		fields[name] = itemFieldValue(name, val)
	}
	return fields
}

var displayLineRe = regexp.MustCompile(`^Display:\s*\*\*(.+)\*\*\s*$`)

func displayOf(sec *Section) string {
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			if m := displayLineRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
				return norm.NFC.String(m[1])
			}
		}
	}
	return ""
}

var itemHeadRe = regexp.MustCompile("^`?(item\\.[a-z0-9_.]+)`?\\s*$")
var itemFieldRe = regexp.MustCompile(`^([A-Za-z_]+)\s*=\s*(.+)$`)
var anyHeadRe = regexp.MustCompile("^`?([a-z0-9_.]+)`?\\s*$")

// itemFamilies are the six registered level-1 families.
var itemFamilies = []string{
	"Enhancement Support Items",
	"Recovery Consumables",
	"Engagement & Exploration Items",
	"Cosmetic Redemption Material",
	"Bonus Progression Books",
	"Spirit Beast Materials (Linh Đan)",
}

func itemIsFamilyTitle(t string) bool {
	for _, fam := range itemFamilies {
		if t == fam {
			return true
		}
	}
	return false
}

func emitItem(c *Ctx, f *File, b *SourceBinding, id string, fields map[string]config.Value, display, identity string, line int) {
	has := func(k string) bool { _, ok := fields[k]; return ok }
	if !has("type") || !has("rarity") || !has("binding") ||
		!has("binding_trigger") || !has("stack_limit") {
		c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
			"item %s missing REQUIRED definition field", id)
		return
	}
	out := map[string]config.Value{"item_id": config.VStr(id)}
	for k, v := range fields {
		out[k] = v
	}
	if display == "" {
		c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
			"item %s missing Display line", id)
		return
	}
	out["display"] = config.VStr(display)
	out["identity_note"] = config.VStr(identity) // null -> KindNull handled below
	if identity == "" {
		out["identity_note"] = config.VNull()
	}
	ohas := func(k string) bool { _, ok := out[k]; return ok }
	if !ohas("shared_cooldown_group") {
		out["shared_cooldown_group"] = config.VStr("NONE")
	}
	if !ohas("item_specific_cooldown_ms") {
		out["item_specific_cooldown_ms"] = config.VInt(0)
	}
	if !ohas("HUNT_eligible") {
		out["HUNT_eligible"] = config.VBool(false)
	}
	if !ohas("discard_allowed") {
		out["discard_allowed"] = config.VBool(!strings.HasPrefix(id, "item.book."))
	}
	c.Emit(f.Name, b.Raw, "item",
		[]config.Value{config.VStr(id)}, out, line)
}

func itemRegionalMaterials(c *Ctx, f *File, b *SourceBinding, st *itemState) {
	sec := f.Root.SectionAt("Regional Craft / Enhancement Materials — 6")
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "regional materials missing")
		return
	}
	// first text fence = shared fields
	var shared map[string]config.Value
	var tbl *Block
	for _, bl := range sec.Content {
		if bl.Kind == BlockFence && shared == nil {
			shared = parseDefinitionFence(c, f, bl.FLines, bl.Line)
		}
		if bl.Kind == BlockTable {
			tbl = bl
		}
	}
	if shared == nil || tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"regional materials fence/table missing")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		id := cellAt(row, 0).Scalar()
		fields := map[string]config.Value{}
		for k, v := range shared {
			fields[k] = v
		}
		fields["tier"] = config.VStr(cellAt(row, 2).Scalar())
		emitItem(c, f, b, id, fields, norm.NFC.String(cellAt(row, 1).Scalar()),
			norm.NFC.String(cellAt(row, 3).Scalar()), row[0].Line)
		st.items[id] = true
	}
	c.consumed(f, b)
}

func itemDefinitions(c *Ctx, f *File, b *SourceBinding, st *itemState) {
	for _, fam := range f.Root.Children {
		if !itemIsFamilyTitle(fam.Title) {
			continue
		}
		for _, ch := range fam.Children {
			m := itemHeadRe.FindStringSubmatch(strings.TrimSpace(ch.Title))
			if m == nil {
				continue // fishing tables etc. are not items
			}
			id := m[1]
			var fields map[string]config.Value
			for _, bl := range ch.Content {
				if bl.Kind == BlockFence && fields == nil {
					fields = parseDefinitionFence(c, f, bl.FLines, bl.Line)
				}
			}
			if fields == nil {
				c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, ch.Line,
					"item %s has no definition fence", id)
				continue
			}
			emitItem(c, f, b, id, fields, displayOf(ch), "", ch.Line)
			st.items[id] = true
		}
	}
	c.consumed(f, b)
}

var restoreRe = regexp.MustCompile(`^restore\s*=\s*([0-9.]+)%\s*MAX_(HP|MP)`)

func itemRecoveryUses(c *Ctx, f *File, b *SourceBinding, st *itemState) {
	for _, id := range []string{"item.consumable.nuoc_la", "item.consumable.tra_sen"} {
		var sec *Section
		var findSec func(s *Section)
		findSec = func(s *Section) {
			for _, ch := range s.Children {
				if m := anyHeadRe.FindStringSubmatch(strings.TrimSpace(ch.Title)); m != nil && m[1] == id {
					sec = ch
				}
				findSec(ch)
			}
		}
		findSec(f.Root)
		if sec == nil {
			c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "%s missing", id)
			continue
		}
		// second fence (after definition) is the Effect payload
		var fences []*Block
		for _, bl := range sec.Content {
			if bl.Kind == BlockFence {
				fences = append(fences, bl)
			}
		}
		if len(fences) < 2 {
			c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
				"%s missing Effect fence", id)
			continue
		}
		fb := fences[1]
		found := false
		for j, l := range fb.FLines {
			if m := restoreRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
				d, _ := parseDecimal(m[1])
				ratio, _ := config.ReduceRat(d.Num, d.Den*100)
				c.Emit(f.Name, b.Raw, "item_use",
					[]config.Value{config.VStr(id)},
					map[string]config.Value{
						"item_id":        config.VStr(id),
						"use_kind":       config.VStr("RESTORE_RESOURCE"),
						"restore_ratio":  mustRat(ratio),
						"resource":       config.VStr(m[2]),
						"revive":         config.VBool(false),
						"respect_policy": config.VBool(true),
					}, fb.Line+1+j)
				found = true
			}
		}
		if !found {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, fb.Line,
				"%s Effect fence has no restore line", id)
		}
	}
	c.consumed(f, b)
}

// usePayloadFields — closed per-kind field sets (typed inputs, exhaustive).
var usePayloadFields = map[string][]string{
	"LUCKY_CHARM":          {"bonus_bp", "current_level_min", "current_level_max"},
	"INSURANCE_CHARM":      {"prevent_downgrade", "current_level_min", "current_level_max"},
	"HIDDEN_CHEST_KEY":     {"chest_family", "consume_quantity"},
	"BONFIRE_WINE":         {"required_state", "buff_id", "stat", "stage", "value_bp", "duration_ms", "start", "reapply", "persist_transfer"},
	"BONFIRE_KINDLE":       {"consume_quantity", "extension_ms", "max_remaining_ms", "required_out_of_combat", "reject_no_extension"},
	"FISHING_TOOL":         {"fishing_spot_family", "consume_quantity"},
	"FISHING_BAIT":         {"fishing_spot_family", "consume_quantity", "consume_timing"},
	"BEAST_FOOD":           {"bond_points", "consume_quantity"},
	"PROGRESSION_BOOK":     {"point_kind", "point_quantity", "consume_quantity"},
	"BEAST_LEVEL_MATERIAL": {"target_level_min", "target_level_max"},
}

var useIntFields = map[string]bool{
	"bonus_bp": true, "current_level_min": true, "current_level_max": true,
	"consume_quantity": true, "value_bp": true, "duration_ms": true,
	"extension_ms": true, "max_remaining_ms": true, "bond_points": true,
	"point_quantity": true, "target_level_min": true, "target_level_max": true,
}
var useBoolFields = map[string]bool{
	"prevent_downgrade": true, "persist_transfer": true,
	"required_out_of_combat": true, "reject_no_extension": true,
}

func itemUsePayloads(c *Ctx, f *File, b *SourceBinding, st *itemState) {
	sec := f.Root.SectionAt("Compiler Source Schema > Typed Item Use Payloads")
	if sec == nil {
		for _, s := range f.Root.FindSections("Typed Item Use Payloads") {
			sec = s
		}
	}
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "Typed Item Use Payloads missing")
		return
	}
	var tbl *Block
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable {
			tbl = bl
		}
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		id := cellAt(row, 0).Scalar()
		kind := cellAt(row, 1).Scalar()
		want, ok := usePayloadFields[kind]
		if !ok {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
				"unknown use_kind %q", kind)
			continue
		}
		fields := map[string]config.Value{
			"item_id":  config.VStr(id),
			"use_kind": config.VStr(kind),
		}
		seen := map[string]bool{}
		bad := false
		for _, piece := range strings.Split(strings.Trim(cellAt(row, 2).Text, "`"), ";") {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			m := chestFieldRe.FindStringSubmatch(piece)
			if m == nil {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
					"payload field %q", piece)
				bad = true
				continue
			}
			name, val := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
			allowed := false
			for _, w := range want {
				if w == name {
					allowed = true
				}
			}
			if !allowed || seen[name] {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
					"field %q not allowed/duplicate for %s", name, kind)
				bad = true
				continue
			}
			seen[name] = true
			switch {
			case useIntFields[name]:
				v, _ := (TypeSpec{Name: "int"}).ParseValue(val)
				fields[name] = v
			case useBoolFields[name]:
				fields[name] = config.VBool(val == "true")
			default:
				fields[name] = config.VStr(val)
			}
		}
		if bad {
			continue
		}
		for _, w := range want {
			if !seen[w] {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
					"%s missing required field %q", kind, w)
				bad = true
			}
		}
		if bad {
			continue
		}
		c.Emit(f.Name, b.Raw, "item_use",
			[]config.Value{config.VStr(id)}, fields, row[0].Line)
	}
	c.consumed(f, b)
}

func itemCatchDefault(c *Ctx, f *File, b *SourceBinding, st *itemState) {
	var sec *Section
	var findFish func(s *Section)
	findFish = func(s *Section) {
		for _, ch := range s.Children {
			if m := anyHeadRe.FindStringSubmatch(strings.TrimSpace(ch.Title)); m != nil && m[1] == "fishing.catch.default" {
				sec = ch
			}
			findFish(ch)
		}
	}
	findFish(f.Root)
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "fishing.catch.default missing")
		return
	}
	var tbl *Block
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable {
			tbl = bl
		}
	}
	if tbl == nil {
		return
	}
	var weights int64
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		iid := cellAt(row, 0).Scalar()
		set := cellAt(row, 1).Scalar()
		w, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 2).Scalar())
		weights += w.Int
		st.defaults = append(st.defaults, config.VRec(map[string]config.Value{
			"item_id":   config.VStr(iid),
			"set":       config.VStr(set),
			"weight_bp": w,
		}))
		st.defaultIDs = append(st.defaultIDs, iid)
		c.Emit(f.Name, b.Raw, "catch_entry",
			[]config.Value{config.VStr("fishing.catch.default"), config.VStr(iid)},
			map[string]config.Value{
				"table_id":  config.VStr("fishing.catch.default"),
				"item_id":   config.VStr(iid),
				"set":       config.VStr(set),
				"weight_bp": w,
			}, row[0].Line)
	}
	if weights != 10000 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, tbl.Line,
			"fishing.catch.default weights sum %d != 10000", weights)
	}
	c.Emit(f.Name, b.Raw, "catch_table",
		[]config.Value{config.VStr("fishing.catch.default")},
		map[string]config.Value{
			"table_id":       config.VStr("fishing.catch.default"),
			"season_index":   config.VNull(),
			"spot_selector":  config.VStr("fishing_spot.map.*.*"),
			"clones_default": config.VBool(false),
		}, tbl.Line)
	c.consumed(f, b)
}

// collectFishingSpots scans world_route has_water tables for spot ids in a
// region (`fishing_spot.map.<region>.NN`).
func collectFishingSpots(c *Ctx, region string) []string {
	wr := c.Catalogs["world_route_catalog.md"]
	if wr == nil {
		return nil
	}
	prefix := "fishing_spot.map." + region + "."
	var out []string
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, bl := range s.Content {
			if bl.Kind == BlockTable && hasHeaders(bl, "has_water") {
				for ri := range bl.Cells {
					row := bl.Cells[ri]
					if cellAt(row, 1).Scalar() != "true" {
						continue
					}
					for _, tok := range strings.Split(cellAt(row, 2).Text, ",") {
						tok = strings.TrimSpace(strings.Trim(tok, "`"))
						mid := cellAt(row, 0).Scalar()
						if strings.HasPrefix(tok, ".") {
							tok = "fishing_spot." + mid + tok
						}
						if strings.HasPrefix(tok, prefix) {
							out = append(out, tok)
						}
					}
				}
			}
		}
		for _, ch := range s.Children {
			walk(ch)
		}
	}
	walk(wr.Root)
	return out
}

var catchTableIDRe = regexp.MustCompile(`^fishing\.catch\.season\.([0-9]+)$`)

func itemCatchSeasonal(c *Ctx, f *File, b *SourceBinding, st *itemState) {
	sec := f.Root.SectionAt("Engagement & Exploration Items > Seasonal catch tables")
	if sec == nil {
		for _, s := range f.Root.FindSections("Seasonal catch tables") {
			sec = s
		}
	}
	if sec == nil {
		return
	}
	var tbl *Block
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable {
			tbl = bl
		}
	}
	if tbl == nil {
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		tid := cellAt(row, 0).Scalar()
		m := catchTableIDRe.FindStringSubmatch(tid)
		if m == nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
				"seasonal table id %q", tid)
			continue
		}
		selector := cellAt(row, 1).Scalar()
		// `fishing_spot.map.<region>.*` -> region
		region := strings.TrimSuffix(strings.TrimPrefix(selector, "fishing_spot.map."), ".*")
		spots := collectFishingSpots(c, region)
		if len(spots) == 0 {
			c.Diags.Addf(config.DiagUnresolvedReference, f.Path, row[0].Line,
				"seasonal table %s region %s has no has_water spots", tid, region)
			continue
		}
		extraID := cellAt(row, 2).Scalar()
		extraBP, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 3).Scalar())
		caBP, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 4).Scalar())
		// clone default, replace ca_bong, append extra
		var sum int64
		for _, d := range st.defaults {
			rec := d.Rec
			iid := rec["item_id"].Str
			w := rec["weight_bp"].Int
			if iid == "item.material.ca_bong" {
				w = caBP.Int
			}
			sum += w
			c.Emit(f.Name, b.Raw, "catch_entry",
				[]config.Value{config.VStr(tid), config.VStr(iid)},
				map[string]config.Value{
					"table_id":  config.VStr(tid),
					"item_id":   config.VStr(iid),
					"set":       rec["set"],
					"weight_bp": config.VInt(w),
				}, row[0].Line)
		}
		sum += extraBP.Int
		c.Emit(f.Name, b.Raw, "catch_entry",
			[]config.Value{config.VStr(tid), config.VStr(extraID)},
			map[string]config.Value{
				"table_id":  config.VStr(tid),
				"item_id":   config.VStr(extraID),
				"set":       config.VStr("SEASONAL_CATCH"),
				"weight_bp": extraBP,
			}, row[0].Line)
		if sum != 10000 {
			c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, row[0].Line,
				"seasonal table %s weights sum %d != 10000", tid, sum)
		}
		var spotVals []config.Value
		for _, sp := range spots {
			spotVals = append(spotVals, config.VStr(sp))
		}
		si, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
		c.Emit(f.Name, b.Raw, "catch_table",
			[]config.Value{config.VStr(tid)},
			map[string]config.Value{
				"table_id":       config.VStr(tid),
				"season_index":   si,
				"spot_selector":  config.VStr(selector),
				"spots":          config.VList(spotVals...),
				"clones_default": config.VBool(true),
			}, row[0].Line)
	}
	c.consumed(f, b)
}

func itemBookGrants(c *Ctx, f *File, b *SourceBinding, st *itemState) {
	var sec *Section
	for _, s := range f.Root.FindSections("Typed Bonus Book Grant Schedule") {
		sec = s
	}
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "book grant schedule missing")
		return
	}
	var tbl *Block
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable {
			tbl = bl
		}
	}
	if tbl == nil {
		return
	}
	var total int64
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		lv, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 0).Scalar())
		q, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 1).Scalar())
		total += q.Int
		for _, book := range []string{"item.book.potential", "item.book.skill"} {
			c.Emit(f.Name, b.Raw, "book_grant",
				[]config.Value{config.VStr(book), lv},
				map[string]config.Value{
					"item_id":  config.VStr(book),
					"level":    lv,
					"quantity": q,
				}, row[0].Line)
		}
	}
	if want, ok := bindingDecl(b, reDeclSum); ok && total != want {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, tbl.Line,
			"book grant total %d != declared %d", total, want)
	}
	c.consumed(f, b)
}
