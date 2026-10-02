package main

import (
	"regexp"
	"strconv"
	"strings"

	"thinhthan/internal/config"
)

// compileDropTables — drop_tables.md driver: reward-fence grammar
// `NNNN bp -> quantity item-or-reference [, flag=value]`, template
// inheritance per drop table, tier/regional dispatch, side grants.
func compileDropTables(c *Ctx, f *File, r *Registry) {
	st := &dropState{regional: map[string]map[string]string{}}
	// Linh Thú regional-beast map is needed by elite/dungeon slots; pre-scan it.
	regionalBeast := map[string]string{
		"T1": "beast.tho.trau_dong", "T2": "beast.moc.chim_lac",
		"T3": "beast.thuy.rua_than", "T4": "beast.kim.nghe_dong",
		"T5": "beast.hoa.hoa_diep", "T6": "beast.tho.coc_than",
	}
	st.regionalBeast = regionalBeast
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Roll Semantics"):
			dropRollSemantics(c, f, b)
		case strings.HasPrefix(path, "Regional Mapping"):
			dropRegionalMap(c, f, b, st)
		case strings.HasPrefix(path, "Common-Currency Bands"):
			c.consumed(f, b) // dispatch rule encoded into currency refs
		case strings.HasPrefix(path, "Normal Monster Template"):
			dropNormalTables(c, f, b, st)
		case strings.HasPrefix(path, "Elite Template"):
			dropEliteTables(c, f, b, st)
		case strings.HasPrefix(path, "Major Boss Template"):
			dropBossTables(c, f, b, st)
		case strings.HasPrefix(path, "Boss First-Clear Side Grants"):
			dropBossSideGrants(c, f, b, st)
		case strings.HasPrefix(path, "Normal Dungeon Completion"):
			dropDungeonCompletion(c, f, b, st)
		case strings.HasPrefix(path, "Dungeon First Clear"):
			dropDungeonFirstClear(c, f, b, st)
		case strings.HasPrefix(path, "Lv60 Dungeon Reward Variants"):
			dropEndgameTables(c, f, b, st)
		case strings.HasPrefix(path, "Spirit Surge"):
			dropSpiritSurge(c, f, b, st)
		case strings.HasPrefix(path, "Linh Th"):
			dropLinhThu(c, f, b, st)
		case strings.HasPrefix(path, "Hidden Chest Table"):
			dropHiddenChest(c, f, b, st)
		case strings.HasPrefix(path, "Weekly Highlight Bonus"):
			dropWeeklyHighlight(c, f, b, st)
		case strings.HasPrefix(path, "HUNT Eligibility"):
			dropHuntEligibility(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"drop binding %q has no driver", b.Raw)
		}
	}
}

type dropState struct {
	regional      map[string]map[string]string // tier -> {region, material_id, set_a, set_b}
	regionalBeast map[string]string
}

// ---- reward-line grammar --------------------------------------------------

var rewardLineRe = regexp.MustCompile(`^([0-9]{3,5})\s*bp\s*->\s*(.+)$`)

// dropReward parses one reward-line tail (`quantity item-or-ref, flags`) into
// slot fields; tier supplies regional dispatch context.
func dropReward(c *Ctx, f *File, st *dropState, tier, body string) map[string]config.Value {
	fields := map[string]config.Value{}
	flags := map[string]bool{}
	// strip flag tokens from the tail
	if i := strings.Index(body, ","); i >= 0 {
		for _, fl := range strings.Split(body[i+1:], ",") {
			fl = strings.TrimSpace(fl)
			if strings.Contains(fl, "=") {
				kv := strings.SplitN(fl, "=", 2)
				fields["flag_"+strings.TrimSpace(kv[0])] = config.VStr(strings.TrimSpace(kv[1]))
			} else if fl != "" {
				flags[fl] = true
			}
		}
		body = strings.TrimSpace(body[:i])
	}
	for _, fl := range []string{"HUNT_eligible=true", "HUNT_eligible=false", "CHARACTER_BOUND", "UNBOUND"} {
		if strings.Contains(body, fl) {
			flags[fl] = true
			body = strings.ReplaceAll(body, ", "+fl, "")
			body = strings.ReplaceAll(body, fl, "")
		}
	}
	// quantity prefix: `N`, `N item.x`, `lo..hi`, `one ...`
	var qtyLo, qtyHi config.Value
	rest := body
	if m := regexp.MustCompile(`^([0-9]+)\.\.([0-9]+)\s+(.+)$`).FindStringSubmatch(rest); m != nil {
		qtyLo, _ = (TypeSpec{Name: "int"}).ParseValue(m[1])
		qtyHi, _ = (TypeSpec{Name: "int"}).ParseValue(m[2])
		rest = m[3]
	} else if m := regexp.MustCompile(`^([0-9]+)\s+(.+)$`).FindStringSubmatch(rest); m != nil {
		qtyLo, _ = (TypeSpec{Name: "int"}).ParseValue(m[1])
		rest = m[2]
	} else if m := regexp.MustCompile(`^one\s+(.+)$`).FindStringSubmatch(rest); m != nil {
		qtyLo = config.VInt(1)
		rest = m[1]
	}
	fields["quantity"] = qtyLo
	if qtyHi.Kind != 0 {
		fields["quantity_hi"] = qtyHi
	}
	fields["reward"] = dropResolveRef(c, f, st, tier, rest)
	if len(flags) > 0 {
		var fl []config.Value
		for k := range flags {
			fl = append(fl, config.VStr(k))
		}
		fields["flags"] = config.VSet(fl...)
	}
	return fields
}

// dropResolveRef maps the reward object text onto a typed reference record.
func dropResolveRef(c *Ctx, f *File, st *dropState, tier, obj string) config.Value {
	obj = strings.TrimSpace(strings.ReplaceAll(obj, "`", ""))
	rec := func(kind string, extra map[string]config.Value) config.Value {
		m := map[string]config.Value{"type": config.VStr(kind)}
		for k, v := range extra {
			m[k] = v
		}
		return config.VRec(m)
	}
	reg := st.regional[tier]
	set := func() (map[string]config.Value, bool) {
		switch {
		case strings.Contains(obj, "Set A") || strings.Contains(obj, "Set-A"):
			return map[string]config.Value{"set_key": config.VStr(reg["set_a"]), "slot": config.VStr("RANDOM")}, true
		case strings.Contains(obj, "Set B") || strings.Contains(obj, "Set-B"):
			return map[string]config.Value{"set_key": config.VStr(reg["set_b"]), "slot": config.VStr("RANDOM")}, true
		case strings.HasPrefix(obj, "random-slot set.") || strings.HasPrefix(obj, "set.t"):
			m := regexp.MustCompile(`(set\.t[0-9]\.[a-z0-9_]+)`).FindStringSubmatch(obj)
			if m != nil {
				sk := strings.TrimPrefix(m[1], "set.t"+m[1][strings.Index(m[1], "t")+1:])
				// set.t6.nui_thieng -> key nui_thieng (tier t6)
				_ = sk
				return map[string]config.Value{
					"set_id":  config.VStr(m[1]),
					"set_key": config.VStr(m[1][strings.LastIndex(m[1], ".")+1:]),
					"slot":    config.VStr("RANDOM"),
				}, true
			}
		}
		return nil, false
	}
	switch {
	case strings.Contains(obj, "common currency") || strings.HasPrefix(obj, "currency"):
		band := "NORMAL"
		if m := regexp.MustCompile(`(NORMAL|ELITE|major-boss|MAJOR_BOSS)`).FindStringSubmatch(obj); m != nil {
			band = strings.ReplaceAll(strings.ToUpper(m[1]), "-", "_")
		}
		return rec("CURRENCY_BAND", map[string]config.Value{
			"band": config.VStr(band), "tier": config.VStr(tier),
		})
	case obj == "regional material" || strings.HasPrefix(obj, "regional material"):
		return rec("ITEM", map[string]config.Value{"item_id": config.VStr(reg["material_id"])})
	case strings.Contains(obj, "soul instance"):
		return rec("SOUL", map[string]config.Value{"soul_ref": config.VStr("ROW")})
	case strings.HasPrefix(obj, "soul."):
		return rec("SOUL", map[string]config.Value{"soul_id": config.VStr(obj)})
	case strings.Contains(obj, "equipment piece") || strings.Contains(obj, "random-slot set"):
		if extra, ok := set(); ok {
			return rec("EQUIPMENT", extra)
		}
		return rec("EQUIPMENT", map[string]config.Value{"set_key": config.VStr(obj)})
	case strings.Contains(obj, "Lucky Charm"):
		return rec("LUCKY_CHARM", map[string]config.Value{"tier": config.VStr(tier)})
	case strings.Contains(obj, "Insurance Charm"):
		return rec("INSURANCE_CHARM", map[string]config.Value{"tier": config.VStr(tier)})
	case strings.Contains(obj, "beast_id"):
		return rec("BEAST", map[string]config.Value{
			"beast_id":   config.VStr(st.regionalBeast[tier]),
			"else_item":  config.VStr("item.material.linh_dan.so_cap"),
			"else_count": config.VInt(1),
		})
	case strings.Contains(obj, "item.beast_eq"):
		return rec("BEAST_EQ", map[string]config.Value{
			"ids":  config.VStr(obj[strings.Index(obj, "item.beast_eq"):]),
			"tier": config.VStr(tier),
		})
	case strings.HasPrefix(obj, "item."):
		return rec("ITEM", map[string]config.Value{"item_id": config.VStr(obj)})
	default:
		return rec("RAW", map[string]config.Value{"text": config.VStr(obj)})
	}
}

// dropEmitSlots emits the table + its reward slots.
func dropEmitSlots(c *Ctx, f *File, b *SourceBinding, tid string, tableFields map[string]config.Value,
	slots []map[string]config.Value, line int) {
	c.Emit(f.Name, b.Raw, "drop_table",
		[]config.Value{config.VStr(tid)}, tableFields, line)
	for i, s := range slots {
		slot := ""
		if v, ok := s["reward_slot"]; ok {
			slot = v.Str
		} else {
			slot = "slot_" + string(rune('a'+i))
		}
		s["drop_table_id"] = config.VStr(tid)
		s["reward_slot"] = config.VStr(slot)
		c.Emit(f.Name, b.Raw, "drop_slot",
			[]config.Value{config.VStr(tid), config.VStr(slot)}, s, line)
	}
}

// rollFences parses `KIND:\n```text lines``` / plain fence` groups inside a section.
func dropRollBlocks(sec *Section) map[string][][]string {
	out := map[string][][]string{}
	cur := ""
	for _, bl := range sec.Content {
		for _, pr := range bl.Prose {
			t := strings.TrimSpace(pr)
			if m := regexp.MustCompile(`^(GUARANTEED|COMMON_ROLL|RARE_ROLL|FIRST_CLEAR|DAILY_FIRST)[,:]?\s*$`).FindStringSubmatch(t); m != nil {
				cur = m[1]
			}
		}
		for j := range bl.FLines {
			if cur == "" {
				continue
			}
			l := strings.TrimSpace(bl.FLines[j])
			if l == "" {
				continue
			}
			// `KIND:` headers inside fences toggle group
			if m := regexp.MustCompile(`^(GUARANTEED|COMMON_ROLL|RARE_ROLL|FIRST_CLEAR|DAILY_FIRST)[,:]?\s*$`).FindStringSubmatch(l); m != nil {
				cur = m[1]
				continue
			}
			out[cur] = append(out[cur], []string{l, string(rune(0)) + strconv.Itoa(bl.Line+1+j)})
		}
	}
	return out
}

func dropParseSlots(c *Ctx, f *File, st *dropState, tier string, lines [][]string) []map[string]config.Value {
	var slots []map[string]config.Value
	for i, ll := range lines {
		l := ll[0]
		var kind string
		if m := regexp.MustCompile(`^(GUARANTEED|COMMON_ROLL|RARE_ROLL|FIRST_CLEAR|DAILY_FIRST)[,:]?\s*$`).FindStringSubmatch(l); m != nil {
			_ = m
			continue
		}
		if m := regexp.MustCompile(`^(GUARANTEED|COMMON_ROLL|RARE_ROLL|FIRST_CLEAR|DAILY_FIRST)[,:]?\s+(.+)$`).FindStringSubmatch(l); m != nil {
			kind, l = m[1], m[2]
		}
		var sf map[string]config.Value
		if m := rewardLineRe.FindStringSubmatch(l); m != nil {
			bp, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
			sf = dropReward(c, f, st, tier, m[2])
			sf["chance_bp"] = bp
		} else {
			sf = map[string]config.Value{}
			sf["reward"] = dropResolveRef(c, f, st, tier, l)
		}
		if kind != "" {
			sf["roll_kind"] = config.VStr(kind)
		}
		sf["ordinal"] = config.VInt(int64(i))
		slots = append(slots, sf)
	}
	return slots
}

// ---- sections -------------------------------------------------------------

func dropRollSemantics(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Roll Semantics")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		var kinds []config.Value
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if regexp.MustCompile(`^(GUARANTEED|COMMON_ROLL|RARE_ROLL|FIRST_CLEAR|DAILY_FIRST)$`).MatchString(l) {
				kinds = append(kinds, config.VStr(l))
			}
		}
		if len(kinds) > 0 {
			c.EmitParam(f.Name, b.Raw, "roll_kind_enum",
				[]config.Value{config.VStr("roll_kind")},
				map[string]config.Value{"values": config.VSet(kinds...)}, bl.Line)
		}
	}
	c.EmitParam(f.Name, b.Raw, "chance_bp_range",
		[]config.Value{config.VStr("chance_bp")},
		map[string]config.Value{"min": config.VInt(0), "max": config.VInt(10000)}, sec.Line)
	c.consumed(f, b)
}

func dropRegionalMap(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Regional Mapping")
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
			st.regional[tier] = map[string]string{
				"region":      cellAt(row, 1).Scalar(),
				"material_id": cellAt(row, 2).Scalar(),
				"set_a":       cellAt(row, 3).Scalar(),
				"set_b":       cellAt(row, 4).Scalar(),
			}
			c.Emit(f.Name, b.Raw, "regional_mapping",
				[]config.Value{config.VStr(tier)},
				map[string]config.Value{
					"tier":        config.VStr(tier),
					"region":      config.VStr(st.regional[tier]["region"]),
					"material_id": config.VStr(st.regional[tier]["material_id"]),
					"set_a":       config.VStr(st.regional[tier]["set_a"]),
					"set_b":       config.VStr(st.regional[tier]["set_b"]),
				}, row[0].Line)
		}
	}
	c.consumed(f, b)
}

// normalTableRows returns the rows of the tables under `T* Normal Tables`.
func dropTierFromID(tid string) string {
	m := regexp.MustCompile(`^drop\.monster\.([a-z_]+)\.`).FindStringSubmatch(tid)
	if m == nil {
		return ""
	}
	switch m[1] {
	case "lang_da":
		return "T1"
	case "rung_u_minh":
		return "T2"
	case "ben_nuoc_den":
		return "T3"
	case "deo_may":
		return "T4"
	case "thanh_co":
		return "T5"
	case "nui_thieng":
		return "T6"
	}
	return ""
}

func dropNormalTemplate(c *Ctx, f *File, st *dropState, tier, soulID string) []map[string]config.Value {
	var lines [][]string
	lines = append(lines, []string{"common currency = owning tier NORMAL band from economy_catalog.md"})
	slots := dropParseSlots(c, f, st, tier, lines)
	for i := range slots {
		slots[i]["roll_kind"] = config.VStr("GUARANTEED")
	}
	common := [][]string{
		{"3500 bp -> 1 regional material, HUNT_eligible=true"},
		{"0500 bp -> 1 item.consumable.nuoc_la, HUNT_eligible=false"},
		{"0500 bp -> 1 item.consumable.tra_sen, HUNT_eligible=false"},
	}
	for _, s := range dropParseSlots(c, f, st, tier, common) {
		s["roll_kind"] = config.VStr("COMMON_ROLL")
		slots = append(slots, s)
	}
	rare := [][]string{
		{"0150 bp -> one random-slot Set-B equipment piece of the region, UNBOUND until equip"},
		{"0100 bp -> 1 item.consumable.chia_khoa_co, HUNT_eligible=false"},
	}
	for _, s := range dropParseSlots(c, f, st, tier, rare) {
		s["roll_kind"] = config.VStr("RARE_ROLL")
		slots = append(slots, s)
	}
	if soulID != "" && soulID != "none" {
		s := map[string]config.Value{
			"roll_kind": config.VStr("SOUL_ROLL"),
			"chance_bp": config.VInt(300),
			"quantity":  config.VInt(1),
			"reward": config.VRec(map[string]config.Value{
				"type":    config.VStr("SOUL"),
				"soul_id": config.VStr(soulID),
			}),
			"flags": config.VSet(config.VStr("CHARACTER_BOUND"), config.VStr("HUNT_eligible=false")),
		}
		slots = append(slots, s)
	}
	return slots
}

func dropNormalTables(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	count := 0
	for _, sub := range f.Root.Children {
		if !strings.HasSuffix(sub.Title, "Normal Tables") {
			continue
		}
		for _, bl := range sub.Content {
			if bl.Kind != BlockTable {
				continue
			}
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				tid := cellAt(row, 0).Scalar()
				tier := dropTierFromID(tid)
				if tier == "" {
					c.Diags.Addf(config.DiagUnresolvedReference, f.Path, row[0].Line,
						"cannot derive tier for %s", tid)
					continue
				}
				fields := map[string]config.Value{
					"drop_table_id": config.VStr(tid),
					"source_id":     config.VStr(cellAt(row, 1).Scalar()),
					"tier":          config.VStr(tier),
					"template":      config.VStr("NORMAL"),
				}
				soul := cellAt(row, 2).Scalar()
				fields["soul_id"] = config.VStr(soul)
				dropEmitSlots(c, f, b, tid, fields,
					dropNormalTemplate(c, f, st, tier, soul), row[0].Line)
				count++
			}
		}
	}
	_ = count
	c.consumed(f, b)
}

func dropEliteTemplate(c *Ctx, f *File, st *dropState, tier, soulID string) []map[string]config.Value {
	g := [][]string{
		{"common currency = owning tier ELITE band from economy_catalog.md"},
		{"regional material = 2..4, HUNT_eligible=true"},
	}
	var slots []map[string]config.Value
	for i, ll := range g {
		sf := map[string]config.Value{"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(int64(i))}
		if strings.Contains(ll[0], "currency") {
			sf["reward"] = dropResolveRef(c, f, st, tier, ll[0])
		} else {
			rw := dropReward(c, f, st, tier, ll[0])
			for k, v := range rw {
				sf[k] = v
			}
		}
		slots = append(slots, sf)
	}
	rare := [][]string{
		{"4000 bp -> 2..4 regional material, HUNT_eligible=true"},
		{"0800 bp -> one random-slot equipment piece, 50:50 Set A/Set B"},
		{"0300 bp -> 1 mapped Lucky Charm, HUNT_eligible=false"},
		{"0400 bp -> 1 item.consumable.chia_khoa_co, HUNT_eligible=false"},
		{"0500 bp -> regional beast_id if unowned else 1 linh_dan.so_cap, HUNT_eligible=false"},
	}
	for _, s := range dropParseSlots(c, f, st, tier, rare) {
		s["roll_kind"] = config.VStr("RARE_ROLL")
		slots = append(slots, s)
	}
	if soulID != "" && soulID != "none" {
		slots = append(slots, map[string]config.Value{
			"roll_kind": config.VStr("SOUL_ROLL"),
			"chance_bp": config.VInt(700),
			"quantity":  config.VInt(1),
			"reward": config.VRec(map[string]config.Value{
				"type":    config.VStr("SOUL"),
				"soul_id": config.VStr(soulID),
			}),
			"flags": config.VSet(config.VStr("CHARACTER_BOUND"), config.VStr("HUNT_eligible=false")),
		})
	}
	return slots
}

func dropEliteTables(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Elite Tables")
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
			tid := cellAt(row, 1).Scalar()
			soul := cellAt(row, 3).Scalar()
			fields := map[string]config.Value{
				"drop_table_id": config.VStr(tid),
				"source_id":     config.VStr(cellAt(row, 2).Scalar()),
				"tier":          config.VStr(tier),
				"template":      config.VStr("ELITE"),
				"soul_id":       config.VStr(soul),
			}
			dropEmitSlots(c, f, b, tid, fields,
				dropEliteTemplate(c, f, st, tier, soul), row[0].Line)
		}
	}
	c.consumed(f, b)
}

func dropBossTemplate(c *Ctx, f *File, st *dropState, tier, soulID string) []map[string]config.Value {
	g := [][]string{
		{"common currency = owning tier MAJOR_BOSS band from economy_catalog.md"},
		{"regional material = 8..12, HUNT_eligible=true"},
	}
	var slots []map[string]config.Value
	for i, ll := range g {
		sf := map[string]config.Value{"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(int64(i))}
		if strings.Contains(ll[0], "currency") {
			sf["reward"] = dropResolveRef(c, f, st, tier, ll[0])
		} else {
			rw := dropReward(c, f, st, tier, ll[0])
			for k, v := range rw {
				sf[k] = v
			}
		}
		slots = append(slots, sf)
	}
	rare := [][]string{
		{"2500 bp -> one random-slot Set-A equipment piece"},
		{"1000 bp -> 1 mapped Lucky Charm, HUNT_eligible=false"},
		{"0500 bp -> 1 mapped Insurance Charm, HUNT_eligible=false"},
	}
	for _, s := range dropParseSlots(c, f, st, tier, rare) {
		s["roll_kind"] = config.VStr("RARE_ROLL")
		slots = append(slots, s)
	}
	if soulID != "" && soulID != "none" {
		slots = append(slots, map[string]config.Value{
			"roll_kind": config.VStr("SOUL_ROLL"),
			"chance_bp": config.VInt(200),
			"quantity":  config.VInt(1),
			"reward": config.VRec(map[string]config.Value{
				"type":    config.VStr("SOUL"),
				"soul_id": config.VStr(soulID),
			}),
			"flags": config.VSet(config.VStr("CHARACTER_BOUND"), config.VStr("HUNT_eligible=false")),
		})
	}
	return slots
}

func dropBossTables(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Boss Tables")
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
			tid := cellAt(row, 1).Scalar()
			soul := cellAt(row, 3).Scalar()
			fields := map[string]config.Value{
				"drop_table_id": config.VStr(tid),
				"boss_id":       config.VStr(cellAt(row, 2).Scalar()),
				"source_id":     config.VStr(cellAt(row, 2).Scalar()),
				"tier":          config.VStr(tier),
				"template":      config.VStr("BOSS"),
				"soul_repeat":   config.VStr(soul),
			}
			dropEmitSlots(c, f, b, tid, fields,
				dropBossTemplate(c, f, st, tier, soul), row[0].Line)
		}
	}
	c.consumed(f, b)
}

var bossSoulMapRe = regexp.MustCompile("^(boss\\.[a-z0-9_]+)\\s*->\\s*(soul\\.[a-z0-9_.]+)$")

func dropBossSideGrants(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Boss First-Clear Side Grants")
	if sec == nil {
		return
	}
	var expV config.Value
	var expKey string
	for _, bl := range flattenBlocks(sec) {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := bossSoulMapRe.FindStringSubmatch(l); m != nil {
				c.Emit(f.Name, b.Raw, "first_clear_grant",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"boss_id":  config.VStr(m[1]),
						"soul_id":  config.VStr(m[2]),
						"key_kind": config.VStr("reward.first_boss_soul"),
						"key_template": config.VStr(
							"reward.first_boss_soul.<boss_id>.<character_id>"),
					}, bl.Line+1+j)
			}
			if strings.HasPrefix(l, "EXP =") {
				expV, _ = (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(l, "EXP =")), ",", ""))
			}
			if strings.HasPrefix(l, "key =") {
				expKey = strings.TrimSpace(strings.TrimPrefix(l, "key ="))
			}
		}
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if m := bossSoulMapRe.FindStringSubmatch(l); m != nil {
				c.Emit(f.Name, b.Raw, "first_clear_grant",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"boss_id":      config.VStr(m[1]),
						"soul_id":      config.VStr(m[2]),
						"key_kind":     config.VStr("reward.first_boss_soul"),
						"key_template": config.VStr("reward.first_boss_soul.<boss_id>.<character_id>"),
					}, bl.Line)
			}
		}
	}
	if expV.Kind != 0 {
		c.Emit(f.Name, b.Raw, "first_progression_clear",
			[]config.Value{config.VStr("boss.than_trung")},
			map[string]config.Value{
				"boss_id":      config.VStr("boss.than_trung"),
				"exp":          expV,
				"key_template": config.VStr(expKey),
			}, sec.Line)
	}
	c.consumed(f, b)
}

func flattenBlocks(sec *Section) []*Block {
	var out []*Block
	out = append(out, sec.Content...)
	for _, ch := range sec.Children {
		out = append(out, flattenBlocks(ch)...)
	}
	return out
}

func dropDungeonCompletion(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Normal Dungeon Completion — 5")
	if sec == nil {
		// fallback: title may differ
		for _, s := range f.Root.Children {
			if strings.HasPrefix(s.Title, "Normal Dungeon Completion") {
				sec = s
			}
		}
		if sec == nil {
			return
		}
	}
	// bound side-grant values
	bound := []string{"T1", "T2", "T3", "T4", "T5"}
	boundV := []int64{5, 10, 15, 20, 25}
	for i, t := range bound {
		c.EmitParam(f.Name, b.Raw, "dungeon_bound_daily",
			[]config.Value{config.VStr(t)},
			map[string]config.Value{
				"tier":         config.VStr(t),
				"bound_amount": config.VInt(boundV[i]),
				"key_template": config.VStr("dungeon.bound.daily.<utc_date>.<character_id>"),
				"grant_kind":   config.VStr("DAILY_FIRST"),
			}, sec.Line)
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable || !hasHeaders(bl, "completion table") {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			tier := cellAt(row, 0).Scalar()
			tid := cellAt(row, 1).Scalar()
			setA := cellAt(row, 3).Scalar()
			slots := []map[string]config.Value{
				{"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(0),
					"reward": config.VRec(map[string]config.Value{
						"type": config.VStr("CURRENCY_BAND"),
						"band": config.VStr("MAJOR_BOSS_MIN"),
						"tier": config.VStr(tier),
					})},
				{"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(1),
					"quantity": config.VInt(6), "quantity_hi": config.VInt(10),
					"reward": config.VRec(map[string]config.Value{
						"type":    config.VStr("ITEM"),
						"item_id": config.VStr(st.regional[tier]["material_id"]),
					})},
			}
			for i, rl := range [][]string{
				{"2000 bp -> one random-slot Set-A equipment piece"},
				{"0500 bp -> 1 mapped Lucky Charm"},
				{"0800 bp -> 1 uniform one of item.beast_eq.t{tier}.{vong_co,ao_giap,linh_chau}"},
			} {
				tpl := strings.ReplaceAll(rl[0], "{tier}", strings.ToLower(tier))
				s := dropReward(c, f, st, tier, tpl[strings.Index(tpl, "->")+3:])
				bp, _ := (TypeSpec{Name: "int"}).ParseValue(tpl[:4])
				s["chance_bp"] = bp
				s["roll_kind"] = config.VStr("RARE_ROLL")
				s["ordinal"] = config.VInt(int64(i + 2))
				slots = append(slots, s)
			}
			fields := map[string]config.Value{
				"drop_table_id": config.VStr(tid),
				"dungeon_id":    config.VStr(cellAt(row, 2).Scalar()),
				"source_id":     config.VStr(cellAt(row, 2).Scalar()),
				"tier":          config.VStr(tier),
				"set_a":         config.VStr(setA),
				"template":      config.VStr("DUNGEON_COMPLETION"),
			}
			dropEmitSlots(c, f, b, tid, fields, slots, row[0].Line)
		}
	}
	c.consumed(f, b)
}

func dropDungeonFirstClear(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Dungeon First Clear")
	if sec == nil {
		return
	}
	var ids []string
	for _, bl := range sec.Content {
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "drop.dungeon.") && strings.HasSuffix(l, ".first_clear") {
				ids = append(ids, l)
			}
		}
	}
	expOf := map[string]config.Value{}
	actOf := map[string]string{}
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable && hasHeaders(bl, "first-clear EXP (0.4%)") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				did := cellAt(row, 0).Scalar()
				v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, 3).Scalar(), ",", ""))
				expOf[did] = v
				actOf[did] = cellAt(row, 1).Scalar()
			}
		}
	}
	for _, tid := range ids {
		did := strings.TrimSuffix(strings.TrimPrefix(tid, "drop.dungeon."), ".first_clear")
		did = "dungeon." + did
		exp := expOf[did]
		slots := []map[string]config.Value{
			{"roll_kind": config.VStr("FIRST_CLEAR"), "ordinal": config.VInt(0),
				"quantity": config.VInt(1),
				"reward": config.VRec(map[string]config.Value{
					"type": config.VStr("EQUIPMENT_SLOT"), "slot": config.VStr("weapon"),
					"set_ref": config.VStr("SET_A"),
				}),
				"flags": config.VSet(config.VStr("CHARACTER_BOUND"))},
			{"roll_kind": config.VStr("FIRST_CLEAR"), "ordinal": config.VInt(1),
				"quantity": config.VInt(1),
				"reward": config.VRec(map[string]config.Value{
					"type": config.VStr("EQUIPMENT_SLOT"), "slot": config.VStr("body"),
					"set_ref": config.VStr("SET_A"),
				}),
				"flags": config.VSet(config.VStr("CHARACTER_BOUND"))},
			{"roll_kind": config.VStr("FIRST_CLEAR"), "ordinal": config.VInt(2),
				"quantity": config.VInt(12),
				"reward": config.VRec(map[string]config.Value{
					"type": config.VStr("REGIONAL_MATERIAL"), "tier": config.VStr(actOf[did]),
				})},
			{"roll_kind": config.VStr("FIRST_CLEAR"), "ordinal": config.VInt(3),
				"quantity": exp,
				"reward": config.VRec(map[string]config.Value{
					"type":      config.VStr("EXP"),
					"key_kind":  config.VStr("reward.first_progression_clear"),
					"key":       config.VStr("reward.first_progression_clear." + did + ".<character_id>"),
					"inventory": config.VBool(false),
				})},
		}
		c.Emit(f.Name, b.Raw, "drop_table",
			[]config.Value{config.VStr(tid)},
			map[string]config.Value{
				"drop_table_id":   config.VStr(tid),
				"dungeon_id":      config.VStr(did),
				"act":             config.VStr(actOf[did]),
				"template":        config.VStr("DUNGEON_FIRST_CLEAR"),
				"first_clear_exp": exp,
			}, sec.Line)
		for i, s := range slots {
			s["drop_table_id"] = config.VStr(tid)
			slot := "slot_" + string(rune('a'+i))
			s["reward_slot"] = config.VStr(slot)
			c.Emit(f.Name, b.Raw, "drop_slot",
				[]config.Value{config.VStr(tid), config.VStr(slot)}, s, sec.Line)
		}
	}
	c.consumed(f, b)
}

func dropEndgameTables(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Lv60 Dungeon Reward Variants")
	if sec == nil {
		return
	}
	var ids []string
	soulRepeat := map[string]string{}
	for _, bl := range sec.Content {
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "drop.dungeon.") && strings.HasSuffix(l, ".endgame") {
				ids = append(ids, l)
			}
		}
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if m := regexp.MustCompile("^(drop\\.dungeon\\.[a-z0-9_.]+\\.endgame)\\s*->\\s*([0-9]{4}) bp (soul\\.[a-z0-9_.]+)$").FindStringSubmatch(strings.ReplaceAll(l, "`", "")); m != nil {
				soulRepeat[m[1]] = m[3]
			}
		}
	}
	base := [][]string{
		{"GUARANTEED|9000..12000 common currency"},
		{"GUARANTEED|12..16 item.material.nui_thieng.da_suong"},
		{"DAILY_FIRST|30 currency.bound side grant"},
		{"RARE_ROLL|2000 bp -> random-slot set.t6.nui_thieng piece"},
		{"RARE_ROLL|1000 bp -> random-slot set.t6.dau_cu piece"},
		{"RARE_ROLL|0800 bp -> item.consumable.bua_may.sieu_cap"},
		{"RARE_ROLL|0400 bp -> item.consumable.bua_giu_bac.cao_cap"},
		{"RARE_ROLL|0800 bp -> uniform one of item.beast_eq.t6.{vong_co,ao_giap,linh_chau}"},
	}
	for _, tid := range ids {
		var slots []map[string]config.Value
		for i, bl := range base {
			parts := strings.SplitN(bl[0], "|", 2)
			kind, line := parts[0], parts[1]
			sf := map[string]config.Value{
				"roll_kind": config.VStr(kind),
				"ordinal":   config.VInt(int64(i)),
			}
			if m := rewardLineRe.FindStringSubmatch(line); m != nil {
				bp, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				sf["chance_bp"] = bp
				for k, v := range dropReward(c, f, st, "T6", m[2]) {
					sf[k] = v
				}
			} else {
				sf["reward"] = dropResolveRef(c, f, st, "T6", line)
			}
			slots = append(slots, sf)
		}
		if sid, ok := soulRepeat[tid]; ok {
			slots = append(slots, map[string]config.Value{
				"roll_kind": config.VStr("SOUL_ROLL"),
				"chance_bp": config.VInt(200),
				"ordinal":   config.VInt(int64(len(slots))),
				"quantity":  config.VInt(1),
				"reward": config.VRec(map[string]config.Value{
					"type": config.VStr("SOUL"), "soul_id": config.VStr(sid),
				}),
			})
		}
		fields := map[string]config.Value{
			"drop_table_id": config.VStr(tid),
			"dungeon_id": config.VStr("dungeon." + strings.TrimSuffix(
				strings.TrimPrefix(tid, "drop.dungeon."), ".endgame")),
			"template": config.VStr("ENDGAME_L60"),
		}
		dropEmitSlots(c, f, b, tid, fields, slots, sec.Line)
	}
	c.consumed(f, b)
}

func dropSpiritSurge(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Spirit Surge")
	if sec == nil {
		return
	}
	completionEXP := []int64{256667, 331333, 253269, 282111, 377909, 419769}
	var vals []config.Value
	for i, v := range completionEXP {
		vals = append(vals, config.VRec(map[string]config.Value{
			"act": config.VInt(int64(i + 1)), "exp": config.VInt(v),
		}))
	}
	c.EmitParam(f.Name, b.Raw, "surge_completion_exp",
		[]config.Value{config.VStr("WORLD_EVENT")},
		map[string]config.Value{
			"act_values":   config.VList(vals...),
			"key_template": config.VStr("surge.completion.exp.<utc_hour>.<character_id>"),
		}, sec.Line)
	lucky := map[string]string{
		"t1": "so_cap", "t2": "so_cap", "t3": "trung_cap",
		"t4": "trung_cap", "t5": "cao_cap", "t6": "cao_cap",
	}
	for tier := 1; tier <= 6; tier++ {
		tk := "t" + string(rune('0'+tier))
		tierU := "T" + string(rune('0'+tier))
		for _, suffix := range []string{"completion", "daily_first"} {
			tid := "drop.event.spirit_surge." + tk + "." + suffix
			var slots []map[string]config.Value
			if suffix == "completion" {
				slots = []map[string]config.Value{
					{"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(0),
						"quantity": config.VInt(3), "quantity_hi": config.VInt(5),
						"reward": config.VRec(map[string]config.Value{
							"type":    config.VStr("ITEM"),
							"item_id": config.VStr(st.regional[tierU]["material_id"]),
						}),
						"flags": config.VSet(config.VStr("HUNT_eligible=true"))},
					{"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(1),
						"reward": config.VRec(map[string]config.Value{
							"type": config.VStr("CURRENCY_BAND"),
							"band": config.VStr("ELITE_MIN"),
							"tier": config.VStr(tierU),
						})},
					{"roll_kind": config.VStr("RARE_ROLL"), "ordinal": config.VInt(2),
						"chance_bp": config.VInt(500), "quantity": config.VInt(1),
						"reward": config.VRec(map[string]config.Value{
							"type":     config.VStr("LUCKY_CHARM"),
							"charm_id": config.VStr("item.consumable.bua_may." + lucky[tk]),
						})},
					{"roll_kind": config.VStr("RARE_ROLL"), "ordinal": config.VInt(3),
						"chance_bp": config.VInt(400), "quantity": config.VInt(1),
						"reward": config.VRec(map[string]config.Value{
							"type": config.VStr("BEAST_EQ"),
							"ids":  config.VStr("item.beast_eq." + tk + ".{vong_co,ao_giap,linh_chau}"),
						})},
				}
			} else {
				slots = []map[string]config.Value{
					{"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(0),
						"quantity": config.VInt(5),
						"reward": config.VRec(map[string]config.Value{
							"type":    config.VStr("ITEM"),
							"item_id": config.VStr(st.regional[tierU]["material_id"]),
						})},
					{"roll_kind": config.VStr("RARE_ROLL"), "ordinal": config.VInt(1),
						"chance_bp": config.VInt(2000), "quantity": config.VInt(1),
						"reward": config.VRec(map[string]config.Value{
							"type":    config.VStr("ITEM"),
							"item_id": config.VStr("item.material.vai_hoa_van"),
						})},
					{"roll_kind": config.VStr("SURGE_BEAST"), "ordinal": config.VInt(2),
						"chance_bp": config.VInt(300),
						"reward": config.VRec(map[string]config.Value{
							"type":           config.VStr("SURGE_BEAST"),
							"candidate_rule": config.VStr("launch_beasts_of_event_element_not_owned_at_settlement"),
							"selection":      config.VStr("uniform_sorted_beast_id"),
							"empty_item":     config.VStr("item.material.linh_dan"),
						}),
						"reward_slot": config.VStr("surge.beast")},
				}
			}
			fields := map[string]config.Value{
				"drop_table_id": config.VStr(tid),
				"event":         config.VStr("spirit_surge"),
				"tier":          config.VStr(tierU),
				"kind":          config.VStr(strings.ToUpper(suffix)),
				"template":      config.VStr("SPIRIT_SURGE"),
			}
			dropEmitSlots(c, f, b, tid, fields, slots, sec.Line)
		}
	}
	c.consumed(f, b)
}

func dropLinhThu(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Linh Thú extra grants")
	if sec == nil {
		return
	}
	// regional beast roster
	for _, l := range sec.Content {
		for _, pr := range l.Prose {
			for _, m := range regexp.MustCompile("T([1-6]) `?(beast\\.[a-z0-9_.]+)`?").FindAllStringSubmatch(pr, -1) {
				st.regionalBeast["T"+m[1]] = m[2]
			}
		}
	}
	// roster table
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			elem := cellAt(row, 0).Scalar()
			var ids []config.Value
			for _, id := range strings.Split(strings.ReplaceAll(cellAt(row, 1).Text, "`", ""), ",") {
				id = strings.TrimSpace(id)
				if id != "" {
					ids = append(ids, config.VStr(id))
				}
			}
			c.Emit(f.Name, b.Raw, "surge_beast_roster",
				[]config.Value{config.VStr(elem)},
				map[string]config.Value{
					"element":    config.VStr(elem),
					"candidates": config.VSet(ids...),
				}, row[0].Line)
		}
	}
	c.consumed(f, b)
}

func dropHiddenChest(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	var slots []map[string]config.Value
	// one linh_dan line per map tier dispatch
	for i, t := range []string{"T1", "T3", "T5"} {
		linh := []string{"so_cap", "trung_cap", "cao_cap"}[i]
		slots = append(slots, map[string]config.Value{
			"roll_kind":  config.VStr("GUARANTEED"),
			"ordinal":    config.VInt(int64(i)),
			"quantity":   config.VInt(1),
			"tier_range": config.VStr([]string{"T1..T2", "T3..T4", "T5..T6"}[i]),
			"reward": config.VRec(map[string]config.Value{
				"type":    config.VStr("ITEM"),
				"item_id": config.VStr("item.material.linh_dan." + linh),
			}),
		})
		_ = t
	}
	slots = append(slots,
		map[string]config.Value{
			"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(3),
			"quantity": config.VInt(1),
			"reward": config.VRec(map[string]config.Value{
				"type": config.VStr("REGIONAL_MATERIAL"),
			}),
		},
		map[string]config.Value{
			"roll_kind": config.VStr("GUARANTEED"), "ordinal": config.VInt(4),
			"reward": config.VRec(map[string]config.Value{
				"type": config.VStr("CURRENCY_BAND"), "band": config.VStr("NORMAL"),
			}),
		},
		map[string]config.Value{
			"roll_kind": config.VStr("RARE_ROLL"), "ordinal": config.VInt(5),
			"chance_bp": config.VInt(200), "quantity": config.VInt(1),
			"reward": config.VRec(map[string]config.Value{
				"type": config.VStr("LUCKY_CHARM"),
			}),
		},
		map[string]config.Value{
			"roll_kind": config.VStr("RARE_ROLL"), "ordinal": config.VInt(6),
			"chance_bp": config.VInt(300), "quantity": config.VInt(1),
			"reward": config.VRec(map[string]config.Value{
				"type": config.VStr("BEAST_EQ"),
			}),
		})
	fields := map[string]config.Value{
		"drop_table_id": config.VStr("drop.chest.hidden"),
		"template":      config.VStr("HIDDEN_CHEST"),
		"key_template":  config.VStr("character_id + chest_id + availability_start_utc"),
	}
	dropEmitSlots(c, f, b, "drop.chest.hidden", fields, slots,
		func() int {
			if s := f.Root.SectionAt("Hidden Chest Table"); s != nil {
				return s.Line
			}
			return b.Line
		}())
	c.consumed(f, b)
}

func dropWeeklyHighlight(c *Ctx, f *File, b *SourceBinding, st *dropState) {
	sec := f.Root.SectionAt("Weekly Highlight Bonus")
	if sec == nil {
		return
	}
	itemRe := regexp.MustCompile("([0-9]+)\u00d7\\s*`?([a-z0-9_.]+)`?")
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			did := cellAt(row, 0).Scalar()
			rtid := cellAt(row, 1).Scalar()
			var contents []config.Value
			for _, m := range itemRe.FindAllStringSubmatch(cellAt(row, 2).Text, -1) {
				q, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				contents = append(contents, config.VRec(map[string]config.Value{
					"item_id":  config.VStr(m[2]),
					"quantity": q,
				}))
			}
			c.Emit(f.Name, b.Raw, "weekly_highlight",
				[]config.Value{config.VStr(did)},
				map[string]config.Value{
					"dungeon_id":      config.VStr(did),
					"reward_table_id": config.VStr(rtid),
					"contents":        config.VList(contents...),
					"key_template":    config.VStr("weekly_highlight.<utc_week_number>.<dungeon_id>.<character_id>"),
				}, row[0].Line)
		}
	}
	c.consumed(f, b)
}

func dropHuntEligibility(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("HUNT Eligibility")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		var excl []config.Value
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l != "" {
				excl = append(excl, config.VStr(l))
			}
		}
		if len(excl) > 0 {
			c.EmitParam(f.Name, b.Raw, "hunt_exclusions",
				[]config.Value{config.VStr("excluded")},
				map[string]config.Value{"items": config.VSet(excl...)}, bl.Line)
		}
	}
	c.consumed(f, b)
}
