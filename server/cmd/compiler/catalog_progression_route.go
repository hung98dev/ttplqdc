package main

import (
	"regexp"
	"strconv"
	"strings"

	"thinhthan/internal/config"
)

// compileProgressionRoute — progression_route.md driver (13 bindings).
// §3 families: `level` thresholds (finite 10000*L*L, L1..59),
// `portfolio` (act, channel), `exp_unit` (act, source_kind), plus the named
// act/channel/bypass/guardrail tables the registry declares.
func compileProgressionRoute(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		path := ""
		if len(b.SectionPaths) > 0 {
			path = b.SectionPaths[0]
		}
		switch {
		case strings.HasPrefix(path, "Target Pace"):
			prPace(c, f, b)
		case strings.HasPrefix(path, "Acts"):
			prActs(c, f, b)
		case strings.HasPrefix(path, "First Session"):
			emitTable(c, f, b, "minute,Beat,map / object,peak,source",
				"first_session_beat", []string{"minute"}, nil)
		case strings.HasPrefix(path, "Zone Contract"):
			emitRule(c, f, b, "route_rule")
		case strings.HasPrefix(path, "EXP Budget"):
			prExpBudget(c, f, b)
		case strings.HasPrefix(path, "Seven-Channel EXP Source Portfolio"):
			prChannelPortfolio(c, f, b)
		case strings.HasPrefix(path, "Seven-Channel"):
			prChannelActs(c, f, b)
		case strings.HasPrefix(path, "Baseline Leveling Pace"), strings.HasPrefix(path, "Act III Efficiency Note"):
			prPaceRules(c, f, b)
		case strings.HasPrefix(path, "Concrete First-Discovery EXP"):
			prDiscovery(c, f, b)
		case strings.HasPrefix(path, "Concrete Major First-Completion EXP"):
			prFirstClear(c, f, b)
		case strings.HasPrefix(path, "Optional SIDE Contribution"):
			emitRule(c, f, b, "route_rule")
		case strings.HasPrefix(path, "Channel EXP Rate References"):
			prUnitExp(c, f, b)
		case strings.HasPrefix(path, "Endgame"):
			prValidationRules(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"progression_route binding %q has no driver", b.Raw)
		}
	}
}

var actLineRe = regexp.MustCompile(`^Act\s+([IVX]+)\s+\(Lv\s*([0-9]+)-([0-9]+)\):\s*~?([0-9,]+)\s*hours`)
var totalRe = regexp.MustCompile(`Total:\s*~?([0-9,]+)\s*hours`)

// prPace — `Target Pace` fences: per-act hour lines + total.
func prPace(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := actLineRe.FindStringSubmatch(l); m != nil {
					hours, err := groupedInt(m[4])
					if err != nil {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, fb.Line+1+j, "%v", err)
						continue
					}
					c.Emit(f.Name, b.Raw, "pace_act",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{
							"act":          config.VStr(m[1]),
							"level_min":    config.VInt(atoi(m[2])),
							"level_max":    config.VInt(atoi(m[3])),
							"target_hours": config.VInt(hours),
						}, fb.Line+1+j)
					continue
				}
				if m := totalRe.FindStringSubmatch(l); m != nil {
					hours, err := groupedInt(m[1])
					if err != nil {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, fb.Line+1+j, "%v", err)
						continue
					}
					c.EmitParam(f.Name, b.Raw, "pace_total",
						[]config.Value{config.VStr("lv1_60")},
						map[string]config.Value{
							"target_hours": config.VInt(hours),
						}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

// prActs — `Acts` table → `act` family; `New system` `name <lv>` pairs
// become the unlock list.
func prActs(c *Ctx, f *File, b *SourceBinding) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "Levels,Purpose,New system"); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"Acts table not found")
		return
	}
	types := inputTypes(b)
	for ri := range tbl.Cells {
		vals := rowValues(c, f, tbl, ri, types)
		// `New system` cell: `name <lv>; name <lv>` or none
		unlocks := []config.Value{}
		ns := vals["new_system"].Str
		for _, part := range strings.Split(ns, ";") {
			part = strings.TrimSpace(part)
			if part == "" || part == "none" {
				continue
			}
			fields := strings.Fields(part)
			if len(fields) < 2 {
				c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, tbl.Cells[ri][0].Line,
					"unparseable unlock %q", part)
				continue
			}
			lv, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
			if err != nil {
				c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, tbl.Cells[ri][0].Line,
					"unlock level %q", part)
				continue
			}
			unlocks = append(unlocks, config.VRec(map[string]config.Value{
				"system": config.VStr(strings.Join(fields[:len(fields)-1], " ")),
				"level":  config.VInt(lv),
			}))
		}
		vals["unlocks"] = config.VList(unlocks...)
		band := vals["levels"].Str
		c.Emit(f.Name, b.Raw, "act",
			[]config.Value{config.VStr(band)}, vals, tbl.Cells[ri][0].Line)
	}
	c.consumed(f, b)
}

// prExpBudget — `EXP Budget` act totals + verification + the finite
// 10000*L*L level curve (59 threshold records, total 702,100,000).
func prExpBudget(c *Ctx, f *File, b *SourceBinding) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "Act,Level band,act_exp_total"); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"EXP Budget act totals table not found")
		return
	}
	types := inputTypes(b)
	var sum int64
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		vals := rowValues(c, f, tbl, ri, types)
		actRaw := strings.Trim(cellAt(row, 0).Text, "* ")
		if actRaw == "TOTAL" {
			tot, err := groupedInt(strings.Trim(cellAt(row, 2).Text, "* "))
			if err != nil {
				c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line, "%v", err)
			} else {
				c.EmitParam(f.Name, b.Raw, "act_exp_total",
					[]config.Value{config.VStr("TOTAL")},
					map[string]config.Value{"exp": config.VInt(tot)}, row[0].Line)
				if tot != 702100000 {
					c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, row[0].Line,
						"EXP budget TOTAL %d, want 702100000", tot)
				}
			}
			continue
		}
		vals["act"] = config.VStr(actRaw)
		sum += vals["act_exp_total"].Int
		c.Emit(f.Name, b.Raw, "act_exp_budget",
			[]config.Value{config.VStr(actRaw)}, vals, row[0].Line)
	}
	if sum != 0 && sum != 702100000 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, tbl.Line,
			"act EXP budgets sum %d, want 702100000", sum)
	}

	// finite curve: exp_required(L) = 10000*L*L for L1..59 (×100 scale)
	var cum int64
	for l := int64(1); l <= 59; l++ {
		req := int64(10000) * l * l
		cum += req
		c.Emit(f.Name, b.Raw, "level",
			[]config.Value{config.VInt(l)},
			map[string]config.Value{
				"level":          config.VInt(l),
				"exp_required":   config.VInt(req),
				"exp_cumulative": config.VInt(cum),
			}, tbl.Line)
	}
	c.consumed(f, b)
}

var channelNameRe = regexp.MustCompile("`([A-Z_]+)`")

// prChannelPortfolio — `Channel, % of act budget, Target hours` →
// `exp_channel` params keyed by channel.
func prChannelPortfolio(c *Ctx, f *File, b *SourceBinding) {
	types := inputTypes(b)
	for _, sec := range bindingSections(c, f, b) {
		tbl := findTable(sec, "Channel,% of act budget,Target hours")
		if tbl == nil {
			continue
		}
		for ri := range tbl.Cells {
			vals := rowValues(c, f, tbl, ri, types)
			name := channelName(vals["channel"].Str)
			if name == "" || name == "TOTAL" {
				continue
			}
			vals["channel"] = config.VStr(name)
			c.Emit(f.Name, b.Raw, "exp_channel",
				[]config.Value{config.VStr(name)}, vals,
				tbl.Cells[ri][0].Line)
		}
	}
	c.consumed(f, b)
}

// prChannelActs — per-act allocation `Channel, Act I..VI` + STORY_ONCE
// sub-split `Sub-channel, Act I..VI` → `portfolio` (act, channel) and
// `story_subchannel` (act, sub_channel).
func prChannelActs(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, tbl := range allTables(sec) {
			sig := headerSig(tbl)
			if !strings.HasPrefix(sig, "Channel,Act I") && !strings.HasPrefix(sig, "Sub-channel,Act I") {
				continue
			}
			isSub := strings.HasPrefix(sig, "Sub-channel")
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				first := strings.Trim(cellAt(row, 0).Text, "* ")
				if first == "" || first == "act_exp_total" {
					continue
				}
				name := channelName(first)
				if name == "" {
					name = subChannelName(first)
				}
				if name == "" {
					continue
				}
				for ci := 1; ci < len(tbl.Headers); ci++ {
					act := strings.TrimSpace(tbl.Headers[ci])
					exp, err := groupedInt(strings.Trim(cellAt(row, ci).Text, "* "))
					if err != nil {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
							"channel %s act %s: %v", first, act, err)
						continue
					}
					fam := "portfolio"
					if isSub {
						fam = "story_subchannel"
					}
					c.Emit(f.Name, b.Raw, fam,
						[]config.Value{config.VStr(strings.TrimPrefix(act, "Act ")), config.VStr(name)},
						map[string]config.Value{
							"act":     config.VStr(strings.TrimPrefix(act, "Act ")),
							"channel": config.VStr(name),
							"exp":     config.VInt(exp),
						}, row[0].Line)
				}
			}
		}
	}
	c.consumed(f, b)
}

func channelName(s string) string {
	if m := channelNameRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// subChannelName maps `MAIN quests 3.0%`-style labels to stable sub-channel
// tokens.
func subChannelName(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "MAIN quests"):
		return "MAIN_QUEST"
	case strings.HasPrefix(s, "SIDE quests"):
		return "SIDE_QUEST"
	case strings.HasPrefix(s, "safe anchor"):
		return "SAFE_ANCHOR"
	case strings.HasPrefix(s, "3 fields"):
		return "FIELDS_3"
	case strings.HasPrefix(s, "first clear"):
		return "FIRST_CLEAR"
	case strings.HasPrefix(s, "STORY_ONCE"):
		return "STORY_ONCE"
	}
	return ""
}

var expHourRe = regexp.MustCompile(`\b([IVX]+)\s+([0-9][0-9,]*)`)

// prPaceRules — Baseline pace + Act III efficiency: zero-bounty reallocation
// params + implied act EXP/hour + rule records.
func prPaceRules(c *Ctx, f *File, b *SourceBinding) {
	emitRule(c, f, b, "route_rule")
	for _, sec := range bindingSections(c, f, b) {
		for _, bl := range allBlocks(sec) {
			if bl.Kind != BlockProse {
				continue
			}
			for j, l := range bl.Prose {
				if strings.Contains(l, "EXP/hour by act") {
					for _, m := range expHourRe.FindAllStringSubmatch(l, -1) {
						h, err := groupedInt(m[2])
						if err != nil {
							continue
						}
						c.EmitParam(f.Name, b.Raw, "act_exp_per_hour",
							[]config.Value{config.VStr(m[1])},
							map[string]config.Value{"exp_per_hour": config.VInt(h)},
							bl.Line+j)
					}
				}
				if strings.Contains(l, "48.275862%") {
					fs, _ := parseDecimal("48.275862")
					ds, _ := parseDecimal("21.724138")
					fv, _ := config.VRat(fs.Num, fs.Den)
					dv, _ := config.VRat(ds.Num, ds.Den)
					c.EmitParam(f.Name, b.Raw, "zero_bounty_realloc",
						[]config.Value{config.VStr("default")},
						map[string]config.Value{
							"field_combat_share":   fv,
							"dungeon_repeat_share": dv,
						}, bl.Line+j)
				}
			}
		}
	}
}

// prDiscovery — first-discovery EXP table + character key pattern + rules.
func prDiscovery(c *Ctx, f *File, b *SourceBinding) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "Act,safe anchor 0.2%,each field 0.2%,three fields 0.6%,discovery total 0.8%"); t != nil {
			tbl = t
		}
		// key fence: reward.discovery.<map_id>.<character_id>
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "reward.") {
					c.EmitParam(f.Name, b.Raw, "discovery_key",
						[]config.Value{config.VStr("pattern")},
						map[string]config.Value{"pattern": config.VStr(l)},
						fb.Line+1+j)
				}
			}
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"first-discovery EXP table not found")
		return
	}
	types := inputTypes(b)
	types["act"] = enumSpec("I", "II", "III", "IV", "V", "VI")
	for ri := range tbl.Cells {
		vals := rowValues(c, f, tbl, ri, types)
		c.Emit(f.Name, b.Raw, "discovery_exp",
			[]config.Value{vals["act"]}, vals, tbl.Cells[ri][0].Line)
	}
	c.consumed(f, b)
}

var idTokenRe = regexp.MustCompile("`([a-z0-9_.]+)`")

// prFirstClear — major first-completion EXP table + key pattern.
func prFirstClear(c *Ctx, f *File, b *SourceBinding) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "Act,source,first-completion EXP"); t != nil {
			tbl = t
		}
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "reward.") {
					c.EmitParam(f.Name, b.Raw, "first_clear_key",
						[]config.Value{config.VStr("pattern")},
						map[string]config.Value{"pattern": config.VStr(l)},
						fb.Line+1+j)
				}
			}
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"first-completion EXP table not found")
		return
	}
	types := inputTypes(b)
	types["act"] = enumSpec("I", "II", "III", "IV", "V", "VI")
	types["source"] = TypeSpec{Name: "string"}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		vals := rowValues(c, f, tbl, ri, types)
		src := vals["source"].Str
		if m := idTokenRe.FindStringSubmatch(src); m != nil {
			src = m[1]
		} else {
			src = strings.Fields(src)[0]
		}
		vals["source"] = config.VStr(src)
		c.Emit(f.Name, b.Raw, "first_clear_exp",
			[]config.Value{vals["act"]}, vals, row[0].Line)
	}
	c.consumed(f, b)
}

// prUnitExp — assumed unit frequencies + per-unit EXP table →
// `exp_unit` (act, source_kind) plus `exp_unit_frequency` params.
func prUnitExp(c *Ctx, f *File, b *SourceBinding) {
	nameRe := regexp.MustCompile(`^([A-Za-z_ ]+):`)
	rateRe := regexp.MustCompile(`([0-9][0-9.,]*)\s*([a-z]+/hour|[a-z]+/h)\b`)
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			cur := ""
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := nameRe.FindStringSubmatch(l); m != nil {
					cur = strings.TrimSpace(m[1])
				}
				if cur == "" || strings.HasPrefix(cur, "STORY_ONCE") {
					continue
				}
				for _, m := range rateRe.FindAllStringSubmatch(l, -1) {
					rate, err := parseDecimal(m[1])
					if err != nil {
						continue
					}
					rv, err := config.VRat(rate.Num, rate.Den)
					if err != nil {
						continue
					}
					c.EmitParam(f.Name, b.Raw, "exp_unit_frequency",
						[]config.Value{config.VStr(cur), rv},
						map[string]config.Value{
							"rate": rv,
							"unit": config.VStr(m[2]),
						}, fb.Line+1+j)
				}
			}
		}
	}
	// Per-Unit EXP table: Channel, Formula basis, Act I..VI
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		for _, t := range allTables(sec) {
			if strings.HasPrefix(headerSig(t), "Channel,Formula basis") {
				tbl = t
			}
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"per-unit EXP table not found")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		kind := strings.Trim(cellAt(row, 0).Scalar(), "` ")
		basis := cellAt(row, 1).Scalar()
		for ci := 2; ci < len(tbl.Headers); ci++ {
			act := strings.TrimPrefix(strings.TrimSpace(tbl.Headers[ci]), "Act ")
			raw := cellAt(row, ci).Scalar()
			v, err := parseCell(TypeSpec{Name: "int"}, raw)
			if err != nil {
				v, err = parseCell(TypeSpec{Name: "grouped_int"}, raw)
			}
			if err != nil {
				c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
					"%s %s: %v", kind, act, err)
				continue
			}
			c.Emit(f.Name, b.Raw, "exp_unit",
				[]config.Value{config.VStr(act), config.VStr(kind)},
				map[string]config.Value{
					"act":           config.VStr(act),
					"source_kind":   config.VStr(kind),
					"formula_basis": config.VStr(basis),
					"exp_per_unit":  v,
				}, row[0].Line)
		}
	}
	c.consumed(f, b)
}

// prValidationRules — Endgame/Anti-FOMO/Validation/Invariants: rule records
// + invariant fence assignments into params.
func prValidationRules(c *Ctx, f *File, b *SourceBinding) {
	emitRule(c, f, b, "route_rule")
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" || strings.Contains(l, "=") && !strings.HasPrefix(l, "reject") {
					parts := strings.SplitN(l, "=", 2)
					if len(parts) != 2 {
						continue
					}
					name := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					if name == "" || strings.Contains(name, " ") && len(strings.Fields(name)) > 8 {
						continue
					}
					c.EmitParam(f.Name, b.Raw, "route_invariant",
						[]config.Value{config.VStr(name)},
						map[string]config.Value{"value": config.VStr(val)},
						fb.Line+1+j)
				}
			}
		}
	}
}

func groupedInt(s string) (int64, error) {
	s = strings.ReplaceAll(s, ",", "")
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

func atoi(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
