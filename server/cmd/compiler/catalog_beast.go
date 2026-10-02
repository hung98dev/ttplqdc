package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileBeast — spirit_beast_catalog.md driver (6 bindings): P2 rules,
// roster + level-curve contract, detailed profiles, upgrade costs,
// equipment roster, power-budget verification.
func compileBeast(c *Ctx, f *File, r *Registry) {
	st := &beastState{p2rules: map[string]beastP2Rule{}}
	c.Data["beast.state"] = st
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Passive Rules"):
			beastPassiveRules(c, f, b, st)
		case strings.HasPrefix(path, "Roster of 10"):
			beastRoster(c, f, b, st)
		case strings.HasPrefix(path, "Detailed Beast Profiles"):
			beastProfiles(c, f, b, st)
		case strings.HasPrefix(path, "Linh Đan"):
			beastUpgradeCost(c, f, b, st)
		case strings.HasPrefix(path, "Beast Equipment Roster"):
			beastEquipment(c, f, b, st)
		case strings.HasPrefix(path, "Power Budget Note"):
			beastBudget(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"beast binding %q has no driver", b.Raw)
		}
	}
	if len(st.p2rules) != 10 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"beast P2 rules = %d, want 10", len(st.p2rules))
	}
	if len(st.beasts) != 10 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"beast roster = %d, want 10", len(st.beasts))
	}
	if len(st.equipment) != 18 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"beast equipment = %d, want 18", len(st.equipment))
	}
}

type beastP2Rule struct {
	typ     string
	payload string
	icd     [3]int64
	line    int
}

type beastState struct {
	p2rules   map[string]beastP2Rule
	beasts    []string
	equipment []string
}

var icdRe = regexp.MustCompile(`([0-9]+)s`)

func beastPassiveRules(c *Ctx, f *File, b *SourceBinding, st *beastState) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "beast_id,P2 type,ICD Lv20 / Lv40 / Lv60"); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "Passive Rules table not found")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		id := cellAt(row, 0).Scalar()
		typ := cellAt(row, 1).Scalar()
		payload := ""
		if i := strings.Index(typ, "("); i >= 0 {
			payload = strings.Trim(typ[i:], "() ")
			typ = strings.TrimSpace(typ[:i])
		}
		var icd [3]int64
		ms := icdRe.FindAllStringSubmatch(cellAt(row, 2).Scalar(), -1)
		if len(ms) != 3 {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
				"%s ICD ladder %q not 3 values", id, cellAt(row, 2).Scalar())
			continue
		}
		for i, m := range ms {
			v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
			icd[i] = v.Int
			if icd[i] < 45 || icd[i] > 90 {
				c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
					"%s ICD %d outside 45..90s", id, icd[i])
			}
		}
		if icd[0] == icd[1] || icd[1] == icd[2] || icd[0] == icd[2] {
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
				"%s ICD tiers must be distinct", id)
		}
		st.p2rules[id] = beastP2Rule{typ: typ, payload: payload, icd: icd, line: row[0].Line}
		c.Emit(f.Name, b.Raw, "beast_passive_rule",
			[]config.Value{config.VStr(id)},
			map[string]config.Value{
				"beast_id":    config.VStr(id),
				"p2_type":     config.VStr(typ),
				"p2_payload":  config.VStr(payload),
				"icd_seconds": config.VList(config.VInt(icd[0]), config.VInt(icd[1]), config.VInt(icd[2])),
			}, row[0].Line)
	}
	c.consumed(f, b)
}

func beastRoster(c *Ctx, f *File, b *SourceBinding, st *beastState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, ch := range sec.Children {
			for _, fb := range allFences(ch, "text") {
				for _, l := range fb.FLines {
					l = strings.TrimSpace(l)
					if strings.Contains(l, "=") {
						if ex, err := ParseExpr(l[strings.IndexByte(l, '=')+1:]); err == nil {
							c.EmitParam(f.Name, b.Raw, "curve_rule",
								[]config.Value{config.VStr("linear")},
								map[string]config.Value{
									"name": config.VStr("linear"),
									"expr": config.VExpr(ex),
								}, fb.Line+1)
						}
					}
				}
			}
		}
	}
	emitTable(c, f, b, "beast_id,Name (vi-VN),English,Element,Primary Role",
		"beast", []string{"beast_id"}, nil)
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "beast_id,Name (vi-VN),English,Element,Primary Role"); t != nil {
			for ri := range t.Cells {
				st.beasts = append(st.beasts, cellAt(t.Cells[ri], 0).Scalar())
			}
		}
	}
}

var statCurveRe = regexp.MustCompile("^`?([A-Z_]+)`?:\\s*(\\+?[0-9.]+)\\s*(/[a-z]+)?\\s*->\\s*\\+([0-9.]+)\\s*(/[a-z]+)?")
var passiveHeadRe = regexp.MustCompile("\\*\\*Passive ([12]) — (.+?) \\(`?(beast\\.skill\\.[a-z0-9_.]+)`?(,\\s*Clutch)?\\)\\*\\*")
var levelPairRe = regexp.MustCompile("Level 1:\\s*`?(.+?)`?\\s*->\\s*Level 60:\\s*`?(.+?)`?\\.?\\s*$")
var icdLineRe = regexp.MustCompile("Internal cooldown:\\s*`?([0-9]+)s`?\\s*\\(Lv20\\)\\s*->\\s*`?([0-9]+)s`?\\s*\\(Lv40\\)\\s*->\\s*`?([0-9]+)s`?\\s*\\(Lv60\\)")
var elementLineRe = regexp.MustCompile("\\*\\*Element\\*\\*:\\s*([A-Z]+)")
var classRefRe = regexp.MustCompile("`?(class\\.[a-z_]+)`?")

// beastProfiles — `## N. beast.*` sections, registered bullets grammar.
func beastProfiles(c *Ctx, f *File, b *SourceBinding, st *beastState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, ch := range sec.Children {
			var id string
			if i := strings.Index(ch.Title, "`"); i >= 0 {
				if j := strings.Index(ch.Title[i+1:], "`"); j >= 0 {
					id = ch.Title[i+1 : i+1+j]
				}
			}
			if !strings.HasPrefix(id, "beast.") {
				continue
			}
			var element, classRef, desc string
			curves := map[string][2]config.Rat{}
			var p1ID, p1Desc, p1L1, p1L60 string
			var p2ID, p2Desc, p2TypeText string
			var icd [3]int64
			for _, bl := range ch.Content {
				for _, l := range bl.Prose {
					l = strings.TrimSpace(l)
					if m := elementLineRe.FindStringSubmatch(l); m != nil {
						element = m[1]
						if cm := classRefRe.FindStringSubmatch(l); cm != nil {
							classRef = cm[1]
						}
						continue
					}
					if m := statCurveRe.FindStringSubmatch(l); m != nil {
						s, _ := parseDecimal(strings.TrimPrefix(m[2], "+"))
						e, _ := parseDecimal(m[4])
						curves[m[1]] = [2]config.Rat{s, e}
						continue
					}
					if m := passiveHeadRe.FindStringSubmatch(l); m != nil {
						if m[1] == "1" {
							p1ID = m[3]
						} else {
							p2ID = m[3]
						}
						continue
					}
					if m := levelPairRe.FindStringSubmatch(l); m != nil {
						p1L1, p1L60 = m[1], m[2]
						continue
					}
					if m := icdLineRe.FindStringSubmatch(l); m != nil {
						for i, s := range m[1:] {
							v, _ := (TypeSpec{Name: "int"}).ParseValue(s)
							icd[i] = v.Int
						}
						continue
					}
					if strings.HasPrefix(l, "-") {
						rest := strings.TrimSpace(l[1:])
						if rest != "" && !strings.HasPrefix(rest, "**") {
							if p1ID != "" && p1Desc == "" && strings.Contains(l, ".") {
								p1Desc = rest
							} else if p2ID != "" && strings.Contains(l, ":") {
								if i := strings.Index(rest, ":"); i > 0 {
									p2TypeText = strings.TrimSpace(rest[:i])
									p2Desc = rest[i+1:]
								}
							} else if p2ID != "" && p2Desc == "" {
								p2Desc = rest
							}
						}
					}
					_ = desc
				}
			}
			fields := map[string]config.Value{
				"beast_id": config.VStr(id),
				"element":  config.VStr(element),
			}
			if classRef != "" {
				fields["class_id"] = config.VStr(classRef)
			}
			var cv []config.Value
			for name, se := range curves {
				cv = append(cv, config.VRec(map[string]config.Value{
					"stat":   config.VStr(name),
					"curve":  config.VStr("linear"),
					"lv1":    mustRat(se[0]),
					"lv60":   mustRat(se[1]),
					"levels": config.VList(curveExpand(se[0], se[1], curveKind(name))...),
				}))
			}
			fields["base_stats"] = config.VList(cv...)
			if p1ID != "" {
				fields["passive1"] = config.VRec(map[string]config.Value{
					"skill_id":     config.VStr(p1ID),
					"lv1_payload":  config.VStr(p1L1),
					"lv60_payload": config.VStr(p1L60),
					"description":  config.VStr(p1Desc),
				})
			}
			if p2ID != "" {
				rule := st.p2rules[id]
				if rule.typ != "" && p2TypeText != "" && !strings.HasPrefix(p2TypeText, rule.typ) {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, ch.Line,
						"%s P2 profile type %q != Passive Rules %q", id, p2TypeText, rule.typ)
				}
				fields["passive2"] = config.VRec(map[string]config.Value{
					"skill_id":       config.VStr(p2ID),
					"clutch":         config.VBool(true),
					"p2_type":        config.VStr(rule.typ),
					"profile_type":   config.VStr(p2TypeText),
					"description":    config.VStr(strings.TrimSpace(p2Desc)),
					"icd_seconds":    config.VList(config.VInt(icd[0]), config.VInt(icd[1]), config.VInt(icd[2])),
					"unlock_level":   config.VInt(20),
					"upgrade_levels": config.VList(config.VInt(40), config.VInt(60)),
				})
			}
			c.Emit(f.Name, b.Raw, "beast_detail",
				[]config.Value{config.VStr(id)}, fields, ch.Line)
		}
	}
	c.consumed(f, b)
}

// curveKind: integer-output stats floor, fraction stats are bp.
func curveKind(stat string) string {
	switch stat {
	case "CRIT_CHANCE", "ATTACK_SPEED", "DODGE_CHANCE", "DAMAGE_REDUCTION", "ACCURACY", "COOLDOWN_REDUCTION", "HP_REGEN":
		return "ratio"
	}
	return "int"
}

// curveExpand — `value(level) = start + (end-start)*(level-1)/59`: int
// floors, ratio output in bp rounded half up.
func curveExpand(start, end config.Rat, kind string) []config.Value {
	out := make([]config.Value, 60)
	for lv := 1; lv <= 60; lv++ {
		// v = start + (end-start)*(lv-1)/59
		d := config.Rat{Num: (end.Num*start.Den - start.Num*end.Den) * int64(lv-1), Den: end.Den * start.Den * 59}
		v := config.Rat{Num: start.Num*d.Den + d.Num*start.Den, Den: start.Den * d.Den}
		v, _ = config.ReduceRat(v.Num, v.Den)
		switch kind {
		case "int":
			fl := v.Num / v.Den
			if v.Num < 0 && v.Num%v.Den != 0 {
				fl--
			}
			out[lv-1] = config.VInt(fl)
		default:
			// bp = v*10000 rounded half up
			bp := v.Num * 10000
			q := bp / v.Den
			if bp%v.Den*2 >= v.Den {
				q++
			}
			out[lv-1] = config.VInt(q)
		}
	}
	return out
}

var levelRangeRe = regexp.MustCompile(`Lv\s*([0-9]+)\s*->\s*([0-9]+)`)

func beastUpgradeCost(c *Ctx, f *File, b *SourceBinding, st *beastState) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "Level Range,Material Required,Material Cost / Level,Common Currency / Level"); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "upgrade cost table not found")
		return
	}
	totMat := map[string]int64{}
	var totCur int64
	var covered int64
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		m := levelRangeRe.FindStringSubmatch(cellAt(row, 0).Scalar())
		if m == nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
				"level range %q", cellAt(row, 0).Scalar())
			continue
		}
		lo, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
		hi, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
		item := cellAt(row, 1).Scalar()
		mc, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 2).Scalar())
		cc, err := (TypeSpec{Name: "grouped_int"}).ParseValue(cellAt(row, 3).Scalar())
		if err != nil {
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line, "%v", err)
			continue
		}
		n := hi.Int - lo.Int + 1
		covered += n
		totMat[item] += n * mc.Int
		totCur += n * cc.Int
		c.Emit(f.Name, b.Raw, "beast_upgrade_cost",
			[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
			map[string]config.Value{
				"level_lo":           config.VInt(lo.Int),
				"level_hi":           config.VInt(hi.Int),
				"material_item_id":   config.VStr(item),
				"material_cost_each": config.VInt(mc.Int),
				"currency_each":      config.VInt(cc.Int),
			}, row[0].Line)
	}
	if covered != 59 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, tbl.Line,
			"upgrade ranges cover %d transitions, want 59", covered)
	}
	// authored totals (reference check): 58 so_cap / 90 trung_cap / 120 cao_cap / 96,400
	c.EmitParam(f.Name, b.Raw, "beast_upgrade_totals",
		[]config.Value{config.VStr("computed")},
		map[string]config.Value{
			"item.material.linh_dan.so_cap":    config.VInt(totMat["item.material.linh_dan.so_cap"]),
			"item.material.linh_dan.trung_cap": config.VInt(totMat["item.material.linh_dan.trung_cap"]),
			"item.material.linh_dan.cao_cap":   config.VInt(totMat["item.material.linh_dan.cao_cap"]),
			"currency.common":                  config.VInt(totCur),
		}, tbl.Line)
	c.consumed(f, b)
}

var tierLineRe = regexp.MustCompile(`Tier\s*(\d+):\s*Level\s*(\d+)`)
var statPairRe = regexp.MustCompile(`\+([0-9.]+)\s*([A-Z_]+)`)

func beastEquipment(c *Ctx, f *File, b *SourceBinding, st *beastState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				if m := tierLineRe.FindStringSubmatch(l); m != nil {
					t, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
					lv, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
					c.EmitParam(f.Name, b.Raw, "beast_eq_tier",
						[]config.Value{config.VInt(t.Int)},
						map[string]config.Value{
							"tier": config.VInt(t.Int), "level": config.VInt(lv.Int),
						}, fb.Line+1)
				}
			}
		}
		if tbl := findTable(sec, "item_id,Display Name (vi-VN),Slot,Req Lv,Fixed Primary Stat"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				id := cellAt(row, 0).Scalar()
				var stats []config.Value
				for _, sm := range statPairRe.FindAllStringSubmatch(cellAt(row, 4).Scalar(), -1) {
					d, _ := parseDecimal(sm[1])
					stats = append(stats, config.VRec(map[string]config.Value{
						"value": mustRat(d), "stat": config.VStr(sm[2]),
					}))
				}
				req, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 3).Scalar())
				st.equipment = append(st.equipment, id)
				c.Emit(f.Name, b.Raw, "beast_equipment",
					[]config.Value{config.VStr(id)},
					map[string]config.Value{
						"item_id":    config.VStr(id),
						"display":    config.VStr(cellAt(row, 1).Scalar()),
						"slot":       config.VStr(cellAt(row, 2).Scalar()),
						"req_level":  config.VInt(req.Int),
						"fixed_stat": config.VList(stats...),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func beastBudget(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			recs := map[string]config.Value{}
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if i := strings.IndexByte(l, '='); i > 0 {
					name := strings.TrimSpace(l[:i])
					raw := strings.TrimSpace(l[i+1:])
					if v, err := (TypeSpec{Name: "grouped_int"}).ParseValue(strings.Fields(raw)[0]); err == nil {
						recs[fieldName(name)] = v
					} else {
						recs[fieldName(name)] = config.VStr(raw)
					}
					continue
				}
				// worst-case computation lines `STAT : ... = value / ref = pct%`
				if strings.Contains(l, "→") || strings.Contains(l, ":") {
					recs["worst_case_"+strings.ToLower(strings.Fields(strings.ReplaceAll(l, ":", " "))[0])] = config.VStr(l)
				}
			}
			c.EmitParam(f.Name, b.Raw, "beast_budget_check",
				[]config.Value{config.VStr("fence"), config.VInt(int64(fb.Line))},
				recs, fb.Line+1)
		}
	}
	c.consumed(f, b)
}
