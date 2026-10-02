package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileSoul — soul_catalog.md driver (2 bindings): Runtime Soul Roster
// (25 souls, closed trigger/payload dispatch) + Spatial Payloads.
// CAT-001: element enum required, rank-by-element counts validated.
func compileSoul(c *Ctx, f *File, r *Registry) {
	st := &soulState{byElement: map[string]map[string]int{}}
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Runtime Soul Roster"):
			soulRoster(c, f, b, st)
		case strings.HasPrefix(path, "Spatial Payloads"):
			soulSpatial(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"soul binding %q has no driver", b.Raw)
		}
	}
	// rule_versions.soul_element_bindings = 1 (explicit-source contract)
	c.EmitParam(f.Name, "Compiler Source Schema", "rule_versions",
		[]config.Value{config.VStr("soul_element_bindings")},
		map[string]config.Value{"version": config.VInt(1)}, 0)
	soulVerify(c, f, st)
}

type soulState struct {
	byElement map[string]map[string]int // element -> rank -> count
}

// soul trigger dispatch: name -> argument shape
var soulTriggerSig = map[string]string{
	"HP_DAMAGE": "", "CRIT": "", "REWARD_ELIGIBLE_KILL": "",
	"VOLUNTARY_LAND": "", "VOLUNTARY_TRAVEL_WIDTH": "",
	"HOSTILE_HP_DAMAGE_TAKEN": "", "HOSTILE_SHIELD_BREAK": "",
	"TARGET_CONTROL_OR_RECOVERY": "",
	"TARGET_PRE_HP_GE":           "r", "CROSS_HP_BELOW": "r", "HOSTILE_HP_DAMAGE_TAKEN_GE": "r",
	"ACCEPT_TAG": "s", "COMPLETE_TAG": "s", "TARGET_TAG": "s",
	"APPLY_STATUS": "s+", "APPLY_DISPLACEMENT": "s+", "APPLY_OWN_DOT": "s+", "ACTIVE_TAGS": "s+",
	"DISTINCT_ACTIONS_SAME_TARGET": "ii", "DISTINCT_DAMAGING_ACTIVE_IDS": "ii",
	"ACTION_DISTINCT_HOSTILES": "i", "STATIONARY": "i",
}

// soul payload dispatch
var soulPayloadSig = map[string]string{
	"DAMAGE_ADD": "v", "ACTION_CRIT_ADD": "v",
	"BUFF":           "esvi",
	"PREDICATE_BUFF": "esvs",
	"HEAL_SELF":      "v", "RESTORE_SELF_MP": "v", "RESTORE_SELF_MP_FLAT": "v",
	"ARM":                   "siP",
	"DAMAGE_TARGET":         "vs",
	"TARGET_DEBUFF":         "esvi",
	"TARGET_SLOW":           "vi",
	"SHIELD_SELF":           "vi",
	"HOT_SELF_TOTAL":        "viii",
	"DOT_TOTAL":             "svss",
	"RESIDUAL_EXTENSION_MS": "v",
	"SPATIAL_DAMAGE":        "svs",
}

var soulNameRe = regexp.MustCompile(`^([A-Z_]+)\s*(?:\((.*)\))?$`)

// parseSoulCall parses `NAME(args...)` against sig. Returns record or nil.
// sig letters: r=ratio, s=symbol/enum, s+=nonempty enum set, i=int, v=row value
// or allowed expr (v|level_step exprs|int literal), P=nested payload (rest).
func parseSoulCall(c *Ctx, f *File, sig map[string]string, raw string, line int) config.Value {
	m := soulNameRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "soul call %q", raw)
		return config.Value{}
	}
	name := m[1]
	want, ok := sig[name]
	if !ok {
		c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
			"unknown soul token %q", name)
		return config.Value{}
	}
	rec := map[string]config.Value{"kind": config.VStr(name)}
	args := splitTop(m[2], ',')
	var argv []string
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a != "" {
			argv = append(argv, a)
		}
	}
	if want == "" {
		if len(argv) != 0 {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
				"%s takes no args, got %q", name, m[2])
			return config.Value{}
		}
		return config.VRec(rec)
	}
	// ARM(selector, duration_ms, payload...) — payload tail re-joined
	var list []config.Value
	ai := 0
	for wi := 0; wi < len(want); wi++ {
		if ai >= len(argv) {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
				"%s missing arg %d", name, wi)
			return config.Value{}
		}
		arg := argv[ai]
		ai++
		switch want[wi] {
		case 'r':
			d, err := parseDecimal(arg)
			if err != nil {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
					"%s arg %d %q want ratio", name, wi, arg)
				return config.Value{}
			}
			list = append(list, mustRat(d))
		case 'i':
			// integer or allowed `level_step` expression
			if v, err := (TypeSpec{Name: "int"}).ParseValue(arg); err == nil {
				list = append(list, v)
			} else if ex, err := ParseExpr(arg); err == nil {
				list = append(list, config.VExpr(ex))
			} else {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
					"%s arg %d %q want int/expr", name, wi, arg)
				return config.Value{}
			}
		case 'v':
			if arg == "v" {
				list = append(list, config.VStr("v"))
			} else if d, err := parseDecimal(arg); err == nil {
				list = append(list, mustRat(d))
			} else if ex, err := ParseExpr(arg); err == nil {
				list = append(list, config.VExpr(ex))
			} else {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
					"%s arg %d %q want v/int/expr", name, wi, arg)
				return config.Value{}
			}
		case 's':
			if wi+1 < len(want) && want[wi+1] == '+' {
				// nonempty enum set: consume all remaining args
				var set []config.Value
				set = append(set, config.VStr(arg))
				for ai < len(argv) {
					set = append(set, config.VStr(argv[ai]))
					ai++
				}
				list = append(list, config.VSet(set...))
				wi++ // consume the '+'
			} else {
				list = append(list, config.VStr(arg))
			}
		case 'P': // nested payload: consume rest joined
			rest := strings.Join(argv[ai-1:], ",")
			var inner []config.Value
			for _, piece := range splitTop(rest, '+') {
				if pv := parseSoulCall(c, f, soulPayloadSig, piece, line); pv.Kind != 0 {
					inner = append(inner, pv)
				}
			}
			list = append(list, config.VList(inner...))
			ai = len(argv)
		}
	}
	if wi0 := want; strings.HasSuffix(wi0, "+") && len(list) == 0 {
		c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
			"%s wants nonempty set", name)
		return config.Value{}
	}
	rec["args"] = config.VList(list...)
	return config.VRec(rec)
}

// soulValue parses `0.04` -> rat or `250` -> int.
func soulValue(raw string) config.Value {
	if strings.Contains(raw, ".") || strings.HasPrefix(raw, "-") {
		if d, err := parseDecimal(raw); err == nil {
			return mustRat(d)
		}
	}
	if v, err := (TypeSpec{Name: "int"}).ParseValue(raw); err == nil {
		return v
	}
	if d, err := parseDecimal(raw); err == nil {
		return mustRat(d)
	}
	return config.VStr(raw)
}

var soulRankRe = regexp.MustCompile(`^soul\.(normal|elite|boss)\.`)

func soulRoster(c *Ctx, f *File, b *SourceBinding, st *soulState) {
	var tbl *Block
	for _, s := range f.Root.FindSections("Runtime Soul Roster") {
		for _, bl := range s.Content {
			if bl.Kind == BlockTable && hasHeaders(bl, "soul_id") {
				tbl = bl
			}
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "Runtime Soul Roster table missing")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		sid := cellAt(row, 0).Scalar()
		m := soulRankRe.FindStringSubmatch(sid)
		if m == nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
				"soul_id %q outside soul.normal/elite/boss", sid)
			continue
		}
		rank := strings.ToUpper(m[1])
		elem := cellAt(row, 2).Scalar()
		if elem != "KIM" && elem != "MOC" && elem != "THUY" && elem != "HOA" && elem != "THO" {
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
				"soul %s element %q not in five-element enum", sid, elem)
			continue
		}
		// values triple (Lv1/Lv3/Lv5; Lv2=Lv1, Lv4=Lv3)
		var vals []config.Value
		for _, t := range strings.Split(cellAt(row, 5).Scalar(), ",") {
			vals = append(vals, soulValue(strings.TrimSpace(t)))
		}
		if len(vals) != 3 {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
				"soul %s values want triple", sid)
			continue
		}
		var triggers []config.Value
		for _, t := range splitTop(strings.Trim(cellAt(row, 6).Text, "`"), ';') {
			t = strings.TrimSpace(strings.Trim(t, "`"))
			if t == "" {
				continue
			}
			if tv := parseSoulCall(c, f, soulTriggerSig, t, row[0].Line); tv.Kind != 0 {
				triggers = append(triggers, tv)
			}
		}
		var payloads []config.Value
		for _, t := range splitTop(strings.Trim(cellAt(row, 7).Text, "`"), ';') {
			t = strings.TrimSpace(strings.Trim(t, "`"))
			if t == "" {
				continue
			}
			if pv := parseSoulCall(c, f, soulPayloadSig, t, row[0].Line); pv.Kind != 0 {
				payloads = append(payloads, pv)
			}
		}
		icd, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 8).Scalar())
		if st.byElement[elem] == nil {
			st.byElement[elem] = map[string]int{}
		}
		st.byElement[elem][rank]++
		c.Emit(f.Name, b.Raw, "soul",
			[]config.Value{config.VStr(sid)},
			map[string]config.Value{
				"soul_id":   config.VStr(sid),
				"display":   config.VStr(cellAt(row, 1).Scalar()),
				"rank":      config.VStr(rank),
				"element":   config.VStr(elem),
				"source_id": config.VStr(cellAt(row, 3).Scalar()),
				"effect_id": config.VStr(cellAt(row, 4).Scalar()),
				"values":    config.VList(vals...),
				"level_map": config.VList(config.VInt(1), config.VInt(1), config.VInt(3), config.VInt(3), config.VInt(5)),
				"icd_ms":    icd,
				"icd_scope": config.VStr(cellAt(row, 9).Scalar()),
			}, row[0].Line)
		eid := cellAt(row, 4).Scalar()
		c.Emit(f.Name, b.Raw, "soul_effect",
			[]config.Value{config.VStr(sid), config.VStr(eid)},
			map[string]config.Value{
				"soul_id":   config.VStr(sid),
				"effect_id": config.VStr(eid),
				"triggers":  config.VList(triggers...),
				"payloads":  config.VList(payloads...),
			}, row[0].Line)
	}
	c.consumed(f, b)
}

func soulSpatial(c *Ctx, f *File, b *SourceBinding) {
	var tbl *Block
	for _, s := range f.Root.FindSections("Spatial Payloads") {
		for _, bl := range s.Content {
			if bl.Kind == BlockTable && hasHeaders(bl, "spatial_id") {
				tbl = bl
			}
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "Spatial Payloads table missing")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		sid := cellAt(row, 0).Scalar()
		geo := parseGeometry(c, f, cellAt(row, 1).Text, row[0].Line)
		c.Emit(f.Name, b.Raw, "spatial_soul",
			[]config.Value{config.VStr(sid)},
			map[string]config.Value{
				"spatial_id":  config.VStr(sid),
				"geometry":    config.VRec(geo),
				"origin":      config.VStr(cellAt(row, 2).Scalar()),
				"direction":   config.VStr(cellAt(row, 3).Scalar()),
				"timing":      config.VStr(cellAt(row, 4).Scalar()),
				"collision":   config.VStr(cellAt(row, 5).Scalar()),
				"monster_cap": func() config.Value { v, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 6).Scalar()); return v }(),
				"player_cap":  func() config.Value { v, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 7).Scalar()); return v }(),
				"cap_scope":   config.VStr(cellAt(row, 8).Scalar()),
			}, row[0].Line)
	}
	c.consumed(f, b)
}

func soulVerify(c *Ctx, f *File, st *soulState) {
	expect, wantTotal, declared := parseSoulExpectations(c)
	if !declared {
		return
	}
	total := 0
	for elem, e := range expect {
		ranks := map[string]int{"NORMAL": e.normal, "ELITE": e.elite, "BOSS": e.boss}
		for rank, n := range ranks {
			if n == 0 {
				continue
			}
			got := st.byElement[elem][rank]
			total += got
			if got != n {
				c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
					"CAT-001 %s/%s = %d, want %d", elem, rank, got, n)
			}
		}
	}
	if wantTotal > 0 && total != wantTotal {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"soul roster %d != declared %d", total, wantTotal)
	}
}
