package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileEquipment — equipment_catalog.md driver: 12 sets x 14 slots = 168
// expanded `item.eq.*` item IDs, tier budgets, fixed stats, roll pools,
// element layouts, set effects and support signatures.
func compileEquipment(c *Ctx, f *File, r *Registry) {
	st := &eqState{
		sets:   map[string]map[string]config.Value{},
		layout: map[string]map[string]string{},
		budget: map[string]map[string]config.Value{},
	}
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Launch Shape") || strings.Contains(b.Raw, "Concrete Item-ID Expansion"):
			st.expandB = b
			eqShape(c, f, b, st)
		case strings.HasPrefix(path, "Tier Budget"):
			eqTierBudget(c, f, b, st)
		case strings.HasPrefix(path, "Fixed Base Stats"):
			eqFixedStats(c, f, b, st)
		case strings.HasPrefix(path, "Secondary Roll Pool"):
			eqRollPool(c, f, b, st)
		case strings.HasPrefix(path, "Element Layouts"):
			eqLayouts(c, f, b, st)
		case strings.HasPrefix(path, "Binding"):
			eqBinding(c, f, b, st)
		case strings.HasPrefix(path, "Typed Set-Effect Convention"):
			eqConvention(c, f, b, st)
		case strings.HasPrefix(path, "Tier / Set Roster"):
			eqSetRoster(c, f, b, st)
		case strings.HasPrefix(path, "Support Signature Guardrail"):
			c.consumed(f, b) // rule prose, enforced by emitted fields
		case strings.HasPrefix(path, "Acquisition Contract"):
			if sec := f.Root.SectionAt("Acquisition Contract"); sec != nil {
				for _, bl := range sec.Content {
					for j, l := range bl.Prose {
						l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "-"))
						if l == "" {
							continue
						}
						c.EmitParam(f.Name, b.Raw, "acquisition_rule",
							[]config.Value{config.VStr(l)},
							map[string]config.Value{"rule": config.VStr(l)}, bl.Line+1+j)
					}
				}
			}
			c.consumed(f, b)
		case strings.HasPrefix(path, "Enhancement Base Costs"):
			eqEnhancementBase(c, f, b, st)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"equipment binding %q has no driver", b.Raw)
		}
	}
	eqExpand(c, f, st)
}

type eqState struct {
	sets       map[string]map[string]config.Value // set_key -> set fields
	setKeyList []string                           // roster order
	layout     map[string]map[string]string       // layout -> slot -> element
	budget     map[string]map[string]config.Value // tier -> units
	fixed      map[string][]config.Value          // slot -> fixed stat terms
	enhance    map[string][]string                // slot -> enhanceable stats
	ringCharm  map[string]map[string]config.Value // tier -> {CRIT_CHANCE, COOLDOWN_REDUCTION}
	slots      []string
	expandB    *SourceBinding
}

// ---- tables -------------------------------------------------------------

func eqShape(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Canonical Slot Order")
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "Canonical Slot Order missing")
		return
	}
	var slots []config.Value
	for _, bl := range sec.Content {
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			m := regexp.MustCompile(`^([0-9]+)\s+([a-z_]+)`).FindStringSubmatch(l)
			if m == nil {
				continue
			}
			slots = append(slots, config.VStr(m[2]))
			st.slots = append(st.slots, m[2])
		}
	}
	c.EmitParam(f.Name, b.Raw, "slot_order",
		[]config.Value{config.VStr("equipment")},
		map[string]config.Value{"slots": config.VList(slots...)}, sec.Line)
	c.consumed(f, b)
}

func eqTierBudget(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Tier Budget")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			tier := cellAt(row, 0).Scalar()
			num := func(i int) config.Value {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, i).Scalar())
				return v
			}
			fields := map[string]config.Value{
				"tier":   config.VStr(tier),
				"levels": config.VStr(cellAt(row, 1).Scalar()),
				"rarity": config.VStr(cellAt(row, 2).Scalar()),
				"unit_A": num(3), "unit_D": num(4),
				"unit_H": num(5), "unit_M": num(6),
				"secondary_rolls": num(7),
			}
			st.budget[tier] = fields
			c.Emit(f.Name, b.Raw, "tier_budget",
				[]config.Value{config.VStr(tier)}, fields, row[0].Line)
		}
	}
	c.consumed(f, b)
}

// fixedTermRe parses `1.20D DEFENSE` / `0.70A ATTACK` terms; the special
// `tier CRIT_CHANCE`/`tier COOLDOWN_REDUCTION` tokens resolve per tier.
var fixedTermRe = regexp.MustCompile(`([0-9.]+)([ADHM])\s+([A-Z_]+)`)
var tierStatRe = regexp.MustCompile(`tier\s+([A-Z_]+)`)

func eqFixedStats(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Fixed Base Stats by Slot")
	if sec == nil {
		return
	}
	st.fixed = map[string][]config.Value{}
	st.enhance = map[string][]string{}
	st.ringCharm = map[string]map[string]config.Value{}
	tierSeen := false
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		// table 1: slot/fixed stats; table 2: tier utility values
		if hasHeaders(bl, "slot") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				slot := cellAt(row, 0).Scalar()
				text := strings.ReplaceAll(cellAt(row, 1).Text, "`", "")
				var terms []config.Value
				consumed := ""
				for _, m := range fixedTermRe.FindAllStringSubmatch(text, -1) {
					coef, _ := parseDecimal(m[1])
					terms = append(terms, config.VRec(map[string]config.Value{
						"coefficient": mustRat(coef),
						"unit":        config.VStr(m[2]),
						"stat":        config.VStr(m[3]),
					}))
					consumed += m[0]
				}
				// `tier X` utility stat reference
				if tm := tierStatRe.FindStringSubmatch(text); tm != nil {
					terms = append(terms, config.VRec(map[string]config.Value{
						"tier_utility": config.VStr(tm[1]),
						"stat":         config.VStr(tm[1]),
					}))
				}
				var enh []string
				for _, t := range strings.Split(cellAt(row, 2).Scalar(), ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						enh = append(enh, t)
					}
				}
				st.fixed[slot] = terms
				st.enhance[slot] = enh
				for _, t := range enh {
					c.Emit(f.Name, b.Raw, "fixed_stat",
						[]config.Value{config.VStr(slot), config.VStr(t)},
						map[string]config.Value{
							"slot": config.VStr(slot), "stat": config.VStr(t),
							"enhanceable": config.VBool(true),
						}, row[0].Line)
				}
			}
		} else if hasHeaders(bl, "Tier") && hasHeaders(bl, "CRIT_CHANCE") {
			tierSeen = true
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				tier := cellAt(row, 0).Scalar()
				rv, _ := parseDecimal(cellAt(row, 1).Scalar())
				cv, _ := parseDecimal(cellAt(row, 2).Scalar())
				st.ringCharm[tier] = map[string]config.Value{
					"CRIT_CHANCE":        mustRat(rv),
					"COOLDOWN_REDUCTION": mustRat(cv),
				}
			}
		}
	}
	_ = tierSeen
	c.consumed(f, b)
}

func eqRollPool(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Secondary Roll Pool")
	if sec == nil {
		return
	}
	var rollIDs []string
	var flatRanges []config.Value
	for _, bl := range sec.Content {
		if bl.Kind == BlockFence {
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "roll.") {
					rollIDs = append(rollIDs, l)
					continue
				}
				// `STAT = lo UNIT .. hi UNIT`
				m := regexp.MustCompile(`^([A-Z_]+)\s*=\s*([0-9.]+)([ADHM])\s*\.\.\s*([0-9.]+)([ADHM])$`).
					FindStringSubmatch(l)
				if m != nil {
					lo, _ := parseDecimal(m[2])
					hi, _ := parseDecimal(m[4])
					flatRanges = append(flatRanges, config.VRec(map[string]config.Value{
						"stat":       config.VStr(m[1]),
						"lo_coef":    mustRat(lo),
						"unit":       config.VStr(m[3]),
						"hi_coef":    mustRat(hi),
						"range_kind": config.VStr("FLAT_UNIT"),
					}))
				}
			}
		}
	}
	// utility tables: per-tier ranges; map column headers to roll ids
	utilCols := map[string]string{
		"CRIT_CHANCE": "roll.crit_chance", "ATTACK_SPEED": "roll.attack_speed",
		"CAST_SPEED": "roll.cast_speed", "COOLDOWN_REDUCTION": "roll.cooldown_reduction",
		"LIFESTEAL": "roll.lifesteal", "REFLECT": "roll.reflect",
		"ABSORB": "roll.absorb", "HEAL_REDUCTION": "roll.heal_reduction",
	}
	rangeRe := regexp.MustCompile(`^([0-9.]+)\.\.([0-9.]+)`)
	utilRanges := map[string]map[string]config.Value{} // roll_id -> tier -> [lo,hi]
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		ci := colIndex(bl)
		for _, h := range bl.Headers {
			stat := strings.TrimSpace(h)
			rid, ok := utilCols[stat]
			if !ok {
				continue
			}
			idx, ok := ci[stat]
			if !ok {
				continue
			}
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				if idx >= len(row) {
					continue
				}
				tier := cellAt(row, 0).Scalar()
				if !strings.HasPrefix(tier, "T") {
					continue
				}
				m := rangeRe.FindStringSubmatch(cellAt(row, idx).Scalar())
				if m == nil {
					continue
				}
				lo, _ := parseDecimal(m[1])
				hi, _ := parseDecimal(m[2])
				if utilRanges[rid] == nil {
					utilRanges[rid] = map[string]config.Value{}
				}
				utilRanges[rid][tier] = config.VList(mustRat(lo), mustRat(hi))
			}
		}
	}
	// emit roll_def per roll id
	flatStatOf := map[string]string{
		"roll.attack_flat": "ATTACK", "roll.defense_flat": "DEFENSE",
		"roll.max_hp_flat": "MAX_HP", "roll.max_mp_flat": "MAX_MP",
	}
	for _, rid := range rollIDs {
		fields := map[string]config.Value{"stat_id": config.VStr(rid)}
		if stat, ok := flatStatOf[rid]; ok {
			fields["stat"] = config.VStr(stat)
			fields["kind"] = config.VStr("FLAT")
			fields["range_kind"] = config.VStr("FLAT_UNIT")
		} else {
			fields["kind"] = config.VStr("UTILITY")
			fields["stage"] = config.VStr("FLAT_ADD")
			stat := strings.TrimPrefix(rid, "roll.")
			fields["stat"] = config.VStr(strings.ToUpper(stat))
			if tr, ok := utilRanges[rid]; ok {
				var tiers []config.Value
				for _, t := range []string{"T1", "T2", "T3", "T4", "T5", "T6"} {
					if rng, ok := tr[t]; ok {
						tiers = append(tiers, config.VRec(map[string]config.Value{
							"tier":  config.VStr(t),
							"range": rng,
						}))
					}
				}
				fields["tier_ranges"] = config.VList(tiers...)
			}
		}
		c.Emit(f.Name, b.Raw, "roll_def",
			[]config.Value{config.VStr(rid)}, fields, 0)
	}
	c.EmitParam(f.Name, b.Raw, "flat_roll_ranges",
		[]config.Value{config.VStr("secondary_pool")},
		map[string]config.Value{"ranges": config.VList(flatRanges...)}, 0)
	c.consumed(f, b)
}

func eqLayouts(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Element Layouts")
	if sec == nil {
		return
	}
	for _, ch := range sec.Children {
		layout := strings.TrimPrefix(ch.Title, "Layout ")
		m := map[string]string{}
		for _, bl := range ch.Content {
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				parts := strings.Fields(l)
				if len(parts) == 2 {
					m[parts[0]] = parts[1]
				}
			}
		}
		st.layout[layout] = m
		for slot, elem := range m {
			c.Emit(f.Name, b.Raw, "element_layout",
				[]config.Value{config.VStr(layout), config.VStr(slot)},
				map[string]config.Value{
					"layout": config.VStr(layout), "slot": config.VStr(slot),
					"element": config.VStr(elem),
				}, ch.Line)
		}
	}
	c.consumed(f, b)
}

func eqBinding(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Binding")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			m := chestFieldRe.FindStringSubmatch(strings.TrimSpace(l))
			if m == nil {
				continue
			}
			c.EmitParam(f.Name, b.Raw, "binding_default",
				[]config.Value{config.VStr(m[1])},
				map[string]config.Value{
					"field": config.VStr(m[1]),
					"value": config.VStr(strings.TrimSpace(m[2])),
				}, bl.Line+1+j)
		}
	}
	c.consumed(f, b)
}

func eqConvention(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Typed Set-Effect Convention")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if i := strings.Index(l, "->"); i > 0 {
				lhs := strings.TrimSpace(l[:i])
				rhs := strings.TrimSpace(l[i+2:])
				c.EmitParam(f.Name, b.Raw, "effect_typing",
					[]config.Value{config.VStr(lhs)},
					map[string]config.Value{
						"pattern": config.VStr(lhs),
						"stage":   config.VStr(rhs),
					}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

// ---- set roster -----------------------------------------------------------

var setHeadRe = regexp.MustCompile("^`?(set\\.t([0-9])\\.([a-z0-9_]+))`?\\s*—\\s*(.+)$")
var kvLineRe = regexp.MustCompile("^([a-z ]+):\\s*`?([^`]+?)`?\\s*$")
var setEffRe = regexp.MustCompile("^([0-9]+)pc\\s+(effect\\.[^\\s`]+)\\s*->\\s*(.+)$")
var setEffNoIDRe = regexp.MustCompile("^([0-9]+)pc\\s*->\\s*(.+)$")
var supportRe = regexp.MustCompile("^`?(support\\.set\\.[a-z0-9_]+)`?\\s*->\\s*(.+)$")

func eqSetRoster(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Tier / Set Roster")
	if sec == nil {
		return
	}
	for _, tierSec := range sec.Children {
		for _, setSec := range tierSec.Children {
			m := setHeadRe.FindStringSubmatch(strings.TrimSpace(setSec.Title))
			if m == nil {
				continue
			}
			setID := m[1]
			tier := "T" + m[2]
			key := m[3]
			display := strings.TrimSpace(m[4])
			var layout, source string
			for _, bl := range setSec.Content {
				for _, l := range bl.Prose {
					l = strings.TrimSpace(l)
					if strings.HasPrefix(l, "key:") {
						layoutKV := kvLineRe.FindStringSubmatch(l)
						if layoutKV != nil {
							key = strings.TrimSpace(layoutKV[2])
						}
					} else if strings.HasPrefix(l, "layout:") {
						layoutKV := kvLineRe.FindStringSubmatch(l)
						if layoutKV != nil {
							layout = strings.TrimSpace(layoutKV[2])
						}
					} else if strings.HasPrefix(l, "source identity:") {
						source = strings.TrimSpace(strings.TrimPrefix(l, "source identity:"))
					}
				}
				if bl.Kind == BlockFence {
					for j, l := range bl.FLines {
						l = strings.TrimSpace(l)
						var thr, eid, payload string
						if em := setEffRe.FindStringSubmatch(l); em != nil {
							thr, eid, payload = em[1], em[2], em[3]
						} else if em := setEffNoIDRe.FindStringSubmatch(l); em != nil {
							thr = em[1]
							eid = "effect." + setID + "." + thr
							payload = em[2]
						} else if sm := supportRe.FindStringSubmatch(l); sm != nil {
							// handled in support fence below
							continue
						} else {
							continue
						}
						tv, _ := (TypeSpec{Name: "int"}).ParseValue(thr)
						c.Emit(f.Name, b.Raw, "set_effect",
							[]config.Value{config.VStr(key), tv, config.VStr(eid)},
							map[string]config.Value{
								"set_key":   config.VStr(key),
								"threshold": tv,
								"effect_id": config.VStr(eid),
								"payload":   config.VStr(payload),
								"stat_ops":  config.VList(parseEffectOps(payload)...),
							}, bl.Line+1+j)
					}
				}
			}
			// support signature fence: `support.set.<key> -> payload` + priority
			for _, bl := range setSec.Content {
				for j, l := range bl.FLines {
					l = strings.TrimSpace(l)
					if sm := supportRe.FindStringSubmatch(l); sm != nil {
						c.Emit(f.Name, b.Raw, "set_support",
							[]config.Value{config.VStr(key)},
							map[string]config.Value{
								"set_key":    config.VStr(key),
								"support_id": config.VStr(sm[1]),
								"payload":    config.VStr(sm[2]),
								"stat_ops":   config.VList(parseEffectOps(sm[2])...),
							}, bl.Line+1+j)
					}
					if strings.HasPrefix(l, "support_priority") {
						v := strings.TrimSpace(strings.SplitN(l, "=", 2)[1])
						pv, _ := (TypeSpec{Name: "int"}).ParseValue(v)
						c.EmitParam(f.Name, b.Raw, "support_priority",
							[]config.Value{config.VStr(key)},
							map[string]config.Value{"priority": pv}, bl.Line+1+j)
					}
				}
			}
			fields := map[string]config.Value{
				"set_id":          config.VStr(setID),
				"set_key":         config.VStr(key),
				"tier":            config.VStr(tier),
				"layout":          config.VStr(layout),
				"display":         config.VStr(display),
				"source_identity": config.VStr(source),
			}
			st.sets[key] = fields
			st.setKeyList = append(st.setKeyList, key)
			c.Emit(f.Name, b.Raw, "equipment_set",
				[]config.Value{config.VStr(key)}, fields, setSec.Line)
		}
	}
	c.consumed(f, b)
}

func eqEnhancementBase(c *Ctx, f *File, b *SourceBinding, st *eqState) {
	sec := f.Root.SectionAt("Enhancement Base Costs")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable || !hasHeaders(bl, "Tier") {
			continue
		}
		if len(bl.Headers) != 3 {
			// expected-cost reference table -> validation params
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				tier := cellAt(row, 0).Scalar()
				if !regexp.MustCompile(`^T[1-6]$`).MatchString(tier) {
					continue
				}
				var costs []config.Value
				for ci := 1; ci < len(row) && ci < len(bl.Headers); ci++ {
					h := strings.Trim(strings.TrimSpace(bl.Headers[ci]), "*")
					v := strings.ReplaceAll(cellAt(row, ci).Scalar(), ",", "")
					iv, err := (TypeSpec{Name: "int"}).ParseValue(v)
					if err != nil {
						continue
					}
					costs = append(costs, config.VRec(map[string]config.Value{
						"level": config.VStr(h), "cost": iv,
					}))
				}
				c.EmitParam(f.Name, b.Raw, "expected_cost",
					[]config.Value{config.VStr(tier)},
					map[string]config.Value{"tier": config.VStr(tier), "costs": config.VList(costs...)},
					row[0].Line)
			}
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			tier := cellAt(row, 0).Scalar()
			if !regexp.MustCompile(`^T[1-6]$`).MatchString(tier) {
				continue
			}
			mu, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 1).Scalar())
			cc, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, 2).Scalar(), ",", ""))
			c.Emit(f.Name, b.Raw, "enhancement_base",
				[]config.Value{config.VStr(tier)},
				map[string]config.Value{
					"tier":            config.VStr(tier),
					"material_units":  mu,
					"common_currency": cc,
				}, row[0].Line)
		}
	}
	c.consumed(f, b)
}

// ---- expansion ------------------------------------------------------------

var tierLevelRe = regexp.MustCompile(`^([0-9]+)-([0-9]+)$`)

func eqExpand(c *Ctx, f *File, st *eqState) {
	if len(st.slots) == 0 || len(st.setKeyList) == 0 || len(st.budget) == 0 {
		return
	}
	count := 0
	for _, key := range st.setKeyList {
		set := st.sets[key]
		tier := set["tier"].Str
		layout := set["layout"].Str
		budget := st.budget[tier]
		lvRange := budget["levels"].Str
		lvMin, lvMax := "", ""
		if m := tierLevelRe.FindStringSubmatch(lvRange); m != nil {
			lvMin, lvMax = m[1], m[2]
		}
		for si, slot := range st.slots {
			id := "item.eq." + strings.ToLower(tier) + "." + key + "." + slot
			elem := ""
			if l, ok := st.layout[layout]; ok {
				elem = l[slot]
			}
			// resolve fixed stats against tier units
			var resolved []config.Value
			for _, term := range st.fixed[slot] {
				tr := term.Rec
				if tr["tier_utility"].Kind == config.KindString {
					stat := tr["tier_utility"].Str
					if rc, ok := st.ringCharm[tier]; ok {
						resolved = append(resolved, config.VRec(map[string]config.Value{
							"stat":  config.VStr(stat),
							"value": rc[stat],
						}))
					}
					continue
				}
				coef := tr["coefficient"].Rat
				unit := tr["unit"].Str
				unitV := budget["unit_"+unit].Int
				val := coef.Num * unitV / coef.Den
				resolved = append(resolved, config.VRec(map[string]config.Value{
					"stat":  tr["stat"],
					"value": config.VInt(val),
				}))
			}
			var enh []config.Value
			for _, e := range st.enhance[slot] {
				enh = append(enh, config.VStr(e))
			}
			fields := map[string]config.Value{
				"item_id":           config.VStr(id),
				"kind":              config.VStr("EQUIPMENT"),
				"tier":              config.VStr(tier),
				"set_key":           config.VStr(key),
				"slot":              config.VStr(slot),
				"slot_ordinal":      config.VInt(int64(si)),
				"element":           config.VStr(elem),
				"level_min":         config.VStr(lvMin),
				"level_max":         config.VStr(lvMax),
				"rarity":            budget["rarity"],
				"binding":           config.VStr("UNBOUND"),
				"binding_trigger":   config.VStr("ON_EQUIP"),
				"stack_limit":       config.VInt(1),
				"fixed_stats":       config.VList(resolved...),
				"enhanceable_stats": config.VSet(enh...),
				"secondary_rolls":   budget["secondary_rolls"],
			}
			if lvMin != "" {
				if v, err := (TypeSpec{Name: "int"}).ParseValue(lvMin); err == nil {
					fields["level_min"] = v
				}
				if v, err := (TypeSpec{Name: "int"}).ParseValue(lvMax); err == nil {
					fields["level_max"] = v
				}
			}
			c.Emit(f.Name, "Concrete Item-ID Expansion", "equipment",
				[]config.Value{config.VStr(id)}, fields, 0)
			count++
		}
	}
	if want, ok := bindingDecl(st.expandB, reDeclEmission); ok && int64(count) != want {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"equipment expansion %d != declared %d", count, want)
	}
}
