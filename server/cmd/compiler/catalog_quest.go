package main

import (
	"regexp"
	"strconv"
	"strings"

	"thinhthan/internal/config"
)

// compileQuest — quest_catalog.md driver: 24 MAIN + 12 SIDE quests,
// quest-local object expansion, daily bounty pool + board generation,
// event quest.
func compileQuest(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		raw := b.Raw
		switch {
		case strings.Contains(raw, "Quest-Local Objects"):
			questLocalObjects(c, f, b)
		case strings.Contains(raw, "EXP Budget"):
			questExpBudget(c, f, b)
		case strings.Contains(raw, "Bounty Set EXP"):
			questBountyExp(c, f, b)
		case strings.Contains(raw, "Shared Main-Quest Rules"):
			questSharedRules(c, f, b)
		case strings.Contains(raw, "quest.main.*") || strings.HasPrefix(raw, "# ACT"):
			questMainActs(c, f, b)
		case strings.Contains(raw, "reward table"):
			questSideRewards(c, f, b)
		case strings.Contains(raw, "local-object table"):
			questSideObjects(c, f, b)
		case strings.Contains(raw, "DAILY Bounty Template Pool"):
			questDailyPool(c, f, b)
		case strings.Contains(raw, "Board Generation"):
			questBoardGen(c, f, b)
		case strings.Contains(raw, "Spirit Surge Event Quest"):
			questSurgeEvent(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"quest binding %q has no driver", raw)
		}
	}
}

var questHeadRe = regexp.MustCompile("^`?(quest\\.[a-z0-9_.]+)`?\\s*—\\s*(.+)$")
var questKVRe = regexp.MustCompile("^`?([a-z_]+)`?\\s*=\\s*`?([^`]+?)`?\\s*[,.]?$")

func questSection(c *Ctx, f *File, sec *Section, b *SourceBinding) {
	m := questHeadRe.FindStringSubmatch(strings.TrimSpace(sec.Title))
	if m == nil {
		return
	}
	qid := m[1]
	fields := map[string]config.Value{
		"quest_id": config.VStr(qid),
		"display":  config.VStr(strings.TrimSpace(m[2])),
		"type":     config.VStr("MAIN"),
	}
	ordinal := 0
	rewardsSeen := false
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "Objective:") || strings.HasPrefix(l, "Objectives:") {
				rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "Objectives:"), "Objective:"))
				if rest != "" {
					for _, tok := range strings.Split(rest, ";") {
						tok = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(tok), "."))
						if tok == "" {
							continue
						}
						ordinal++
						emitInlineObjective(c, f, b, qid, ordinal, tok, bl.Line)
					}
				}
				continue
			}
			if strings.HasPrefix(l, "Rewards:") {
				rewardsSeen = true
				rest := strings.TrimSpace(strings.TrimPrefix(l, "Rewards:"))
				if rest != "" {
					emitInlineRewards(c, f, b, qid, rest, bl.Line)
				}
				continue
			}
			if strings.HasPrefix(l, "Prerequisite:") {
				fields["prerequisite"] = config.VStr(strings.TrimSpace(strings.TrimPrefix(l, "Prerequisite:")))
				continue
			}
			if kv := questKVRe.FindStringSubmatch(l); kv != nil {
				fields[kv[1]] = config.VStr(kv[2])
				continue
			}
			if strings.HasPrefix(l, "Objectives") || strings.HasPrefix(l, "Rewards") {
				continue
			}
		}
		// objectives numbered list: prose lines "1. `VERB` ..."
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			om := regexp.MustCompile(`^([0-9]+)\.\s+` + "`?" + `([A-Z_]+)` + "`?" + `\s*(.*)$`).FindStringSubmatch(l)
			if om != nil {
				ordinal++
				fields := map[string]config.Value{
					"quest_id": config.VStr(qid),
					"ordinal":  config.VInt(int64(ordinal)),
					"verb":     config.VStr(om[2]),
					"detail":   config.VStr(strings.ReplaceAll(om[3], "`", "")),
				}
				// parse `any N of a|b|c` / `N target` / plain target
				det := strings.ReplaceAll(om[3], "`", "")
				if km := regexp.MustCompile(`^any\s+([0-9]+)\s+of\s+(.+)$`).FindStringSubmatch(det); km != nil {
					fields["count"] = config.VStr(km[1])
					var tg []config.Value
					for _, t := range strings.Split(km[2], "|") {
						tg = append(tg, config.VStr(strings.TrimSpace(t)))
					}
					fields["targets"] = config.VSet(tg...)
				} else if km := regexp.MustCompile(`^([0-9]+)\s+(.+)$`).FindStringSubmatch(det); km != nil {
					fields["count"] = config.VStr(km[1])
					fields["target"] = config.VStr(strings.TrimSpace(km[2]))
				} else if det != "" {
					fields["target"] = config.VStr(det)
				}
				c.Emit(f.Name, b.Raw, "quest_objective",
					[]config.Value{config.VStr(qid), config.VInt(int64(ordinal))},
					fields, bl.Line)
			}
		}
		if !rewardsSeen {
			continue
		}
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			emitQuestReward(c, f, b, qid, l, bl.Line+1+j)
		}
	}
	c.Emit(f.Name, b.Raw, "quest",
		[]config.Value{config.VStr(qid)}, fields, sec.Line)
}

var questRewardRe = regexp.MustCompile(`^([0-9]+)\s+([a-z0-9_.]+)$`)

func emitQuestReward(c *Ctx, f *File, b *SourceBinding, qid, l string, line int) {
	var slot string
	fields := map[string]config.Value{"quest_id": config.VStr(qid)}
	switch {
	case strings.HasPrefix(l, "EXP "):
		v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(strings.TrimPrefix(l, "EXP "), ",", ""))
		slot, fields["exp"] = "exp", v
	case strings.HasPrefix(l, "common "):
		v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(strings.TrimPrefix(l, "common "), ",", ""))
		slot, fields["common"] = "common", v
	case strings.HasPrefix(l, "bound "):
		v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.TrimPrefix(l, "bound "))
		slot, fields["bound"] = "bound", v
	default:
		if strings.HasPrefix(l, "beast_grant") {
			slot = "beast_grant"
			fields["grant"] = config.VStr(l)
		} else if m := questRewardRe.FindStringSubmatch(l); m != nil {
			q, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
			slot = "item." + m[2]
			fields["item_id"] = config.VStr(m[2])
			fields["quantity"] = q
		} else if strings.Contains(l, "=") {
			kv := strings.SplitN(l, "=", 2)
			slot = "flag." + strings.TrimSpace(kv[0])
			fields["flag"] = config.VStr(strings.TrimSpace(kv[0]))
			fields["value"] = config.VStr(strings.TrimSpace(kv[1]))
		} else {
			slot = "line." + l
			fields["raw"] = config.VStr(l)
		}
	}
	fields["reward_slot"] = config.VStr(slot)
	c.Emit(f.Name, b.Raw, "quest_reward",
		[]config.Value{config.VStr(qid), config.VStr(slot)}, fields, line)
}

func questLocalObjects(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Quest-Local Objects")
	if sec == nil {
		c.consumed(f, b)
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l != "" {
				c.EmitParam(f.Name, b.Raw, "quest_object_namespace",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"namespace": config.VStr(l)}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func questExpBudget(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("EXP Budget")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable && hasHeaders(bl, "Act") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, 2).Scalar(), ",", ""))
				c.Emit(f.Name, b.Raw, "exp_budget",
					[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
					map[string]config.Value{
						"act":           config.VStr(cellAt(row, 0).Scalar()),
						"level_band":    config.VStr(cellAt(row, 1).Scalar()),
						"act_exp_total": v,
					}, row[0].Line)
			}
		}
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			c.EmitParam(f.Name, b.Raw, "exp_allocation",
				[]config.Value{config.VStr(l)},
				map[string]config.Value{"rule": config.VStr(l)}, bl.Line+1+j)
		}
	}
	c.consumed(f, b)
}

func questBountyExp(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Bounty Set EXP — BOUNTY_REPEAT Channel")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable && hasHeaders(bl, "bounty_set_exp") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				num := func(i int) config.Value {
					v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, i).Scalar(), ",", ""))
					return v
				}
				c.Emit(f.Name, b.Raw, "bounty_exp",
					[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
					map[string]config.Value{
						"act":            config.VStr(cellAt(row, 0).Scalar()),
						"act_exp_total":  num(1),
						"target_hours":   num(2),
						"bounty_set_exp": num(3),
					}, row[0].Line)
			}
		}
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "Formula:") || strings.Contains(l, "bounty_set_exp(act) =") {
				c.EmitParam(f.Name, b.Raw, "bounty_formula",
					[]config.Value{config.VStr("bounty_set_exp")},
					map[string]config.Value{"formula": config.VStr(l)}, bl.Line)
			}
		}
	}
	c.consumed(f, b)
}

func questSharedRules(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Shared Main-Quest Rules")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if kv := chestFieldRe.FindStringSubmatch(l); kv != nil {
				c.EmitParam(f.Name, b.Raw, "main_default",
					[]config.Value{config.VStr(kv[1])},
					map[string]config.Value{
						"field": config.VStr(kv[1]),
						"value": config.VStr(strings.TrimSpace(kv[2])),
					}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func questMainActs(c *Ctx, f *File, b *SourceBinding) {
	for _, act := range f.Root.Children {
		if !strings.HasPrefix(act.Title, "ACT ") {
			continue
		}
		for _, q := range act.Children {
			questSection(c, f, q, b)
		}
	}
	c.consumed(f, b)
}

func questSideRewards(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Optional SIDE Quests — 12")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		switch {
		case hasHeaders(bl, "Act/Tier"):
			// deterministic per-act reward bundle
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				num := func(i int) config.Value {
					v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, i).Scalar(), ",", ""))
					return v
				}
				act := strings.Fields(cellAt(row, 0).Scalar())[0]
				c.Emit(f.Name, b.Raw, "side_reward",
					[]config.Value{config.VStr(act)},
					map[string]config.Value{
						"act":               config.VStr(act),
						"exp":               num(1),
						"common":            num(2),
						"regional_material": num(3),
						"bound":             num(4),
					}, row[0].Line)
			}
		case hasHeaders(bl, "mystery_type"):
			// roster table
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				qid := cellAt(row, 0).Scalar()
				c.Emit(f.Name, b.Raw, "quest",
					[]config.Value{config.VStr(qid)},
					map[string]config.Value{
						"quest_id":        config.VStr(qid),
						"type":            config.VStr("SIDE"),
						"display":         config.VStr(cellAt(row, 1).Scalar()),
						"mystery_type":    config.VStr(cellAt(row, 2).Scalar()),
						"mystery_owner":   config.VStr("quest"),
						"quest_giver":     config.VStr(cellAt(row, 3).Scalar()),
						"objectives_text": config.VStr(cellAt(row, 4).Scalar()),
					}, row[0].Line)
			}
		}
	}
	// starter grant line for chiec_non_ben_da
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			if strings.Contains(l, "item.tool.can_cau_tre") {
				qid := "quest.side.a1.chiec_non_ben_da"
				c.Emit(f.Name, b.Raw, "quest_reward",
					[]config.Value{config.VStr(qid), config.VStr("item.item.tool.can_cau_tre")},
					map[string]config.Value{
						"quest_id":    config.VStr(qid),
						"reward_slot": config.VStr("item.item.tool.can_cau_tre"),
						"item_id":     config.VStr("item.tool.can_cau_tre"),
						"quantity":    config.VInt(1),
					}, bl.Line)
				c.Emit(f.Name, b.Raw, "quest_reward",
					[]config.Value{config.VStr(qid), config.VStr("item.item.consumable.moi_cau")},
					map[string]config.Value{
						"quest_id":    config.VStr(qid),
						"reward_slot": config.VStr("item.item.consumable.moi_cau"),
						"item_id":     config.VStr("item.consumable.moi_cau"),
						"quantity":    config.VInt(5),
					}, bl.Line)
			}
		}
	}
	c.consumed(f, b)
}

func questSideObjects(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Optional SIDE Quests — 12")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable || !hasHeaders(bl, "map_id") {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			qid := cellAt(row, 0).Scalar()
			mapID := cellAt(row, 1).Scalar()
			prefix := cellAt(row, 2).Scalar()
			authentic := strings.Split(cellAt(row, 3).Scalar(), ",")
			wrong := strings.Split(cellAt(row, 4).Scalar(), ",")
			oneway := cellAt(row, 5).Scalar()
			ord := 0
			for _, k := range authentic {
				k = strings.TrimSpace(k)
				if k == "" || k == "NONE" {
					continue
				}
				ord++
				oid := "quest_object." + prefix + "." + k
				c.Emit(f.Name, b.Raw, "quest_object",
					[]config.Value{config.VStr(oid)},
					map[string]config.Value{
						"object_id":  config.VStr(oid),
						"quest_id":   config.VStr(qid),
						"prefix":     config.VStr(prefix),
						"key":        config.VStr(k),
						"ordinal":    config.VInt(int64(ord)),
						"authentic":  config.VBool(true),
						"map_id":     config.VStr(mapID),
						"interact_m": config.VInt(2),
						"reach_m":    config.VInt(1),
					}, row[0].Line)
				c.Emit(f.Name, b.Raw, "quest_anchor",
					[]config.Value{config.VStr("anchor.quest." + prefix + "." + k)},
					map[string]config.Value{
						"anchor_id": config.VStr("anchor.quest." + prefix + "." + k),
						"map_id":    config.VStr(mapID),
						"kind":      config.VStr("FIXED_POINT"),
						"object_id": config.VStr(oid),
					}, row[0].Line)
			}
			for _, k := range wrong {
				k = strings.TrimSpace(k)
				if k == "" || k == "NONE" {
					continue
				}
				oid := "quest_object." + prefix + "." + k
				c.Emit(f.Name, b.Raw, "quest_object",
					[]config.Value{config.VStr(oid)},
					map[string]config.Value{
						"object_id":  config.VStr(oid),
						"quest_id":   config.VStr(qid),
						"prefix":     config.VStr(prefix),
						"key":        config.VStr(k),
						"authentic":  config.VBool(false),
						"map_id":     config.VStr(mapID),
						"interact_m": config.VInt(2),
						"reach_m":    config.VInt(1),
					}, row[0].Line)
				c.Emit(f.Name, b.Raw, "quest_anchor",
					[]config.Value{config.VStr("anchor.quest." + prefix + "." + k)},
					map[string]config.Value{
						"anchor_id": config.VStr("anchor.quest." + prefix + "." + k),
						"map_id":    config.VStr(mapID),
						"kind":      config.VStr("FIXED_POINT"),
						"object_id": config.VStr(oid),
					}, row[0].Line)
			}
			if oneway != "" && oneway != "NONE" {
				c.Emit(f.Name, b.Raw, "quest_platform",
					[]config.Value{config.VStr(oneway)},
					map[string]config.Value{
						"platform_id": config.VStr(oneway),
						"quest_id":    config.VStr(qid),
						"map_id":      config.VStr(mapID),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func questDailyPool(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("DAILY Bounty Template Pool — 12 standard + 1 mystery meta")
	if sec == nil {
		return
	}
	tierFence := -1
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable && hasHeaders(bl, "template_id") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				tid := cellAt(row, 0).Scalar()
				c.Emit(f.Name, b.Raw, "daily_template",
					[]config.Value{config.VStr(tid)},
					map[string]config.Value{
						"template_id": config.VStr(tid),
						"family":      config.VStr(cellAt(row, 1).Scalar()),
						"requirement": config.VStr(cellAt(row, 2).Scalar()),
						"reward":      config.VStr(cellAt(row, 3).Scalar()),
					}, row[0].Line)
			}
		}
		if bl.Kind == BlockFence && len(bl.FLines) > 0 && strings.HasPrefix(strings.TrimSpace(bl.FLines[0]), "T1") {
			tierFence++
		}
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if strings.HasPrefix(l, "quest.daily.") {
				c.EmitParam(f.Name, b.Raw, "daily_quest_id",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"pattern": config.VStr(l)}, bl.Line+1+j)
				continue
			}
			if m := regexp.MustCompile(`^T([0-9])\s+([0-9]+)`).FindStringSubmatch(l); m != nil {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
				fam := "daily_tier_bound"
				if tierFence > 0 {
					fam = "daily_tier_common"
				}
				c.EmitParam(f.Name, b.Raw, fam,
					[]config.Value{config.VStr("T" + m[1])},
					map[string]config.Value{"tier": config.VStr("T" + m[1]), "amount": v}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func questBoardGen(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Board Generation (ADR-0061)")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable && hasHeaders(bl, "standard weight") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				sw, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 1).Scalar())
				mw, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 2).Scalar())
				c.Emit(f.Name, b.Raw, "board_weight",
					[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
					map[string]config.Value{
						"template_id":     config.VStr(cellAt(row, 0).Scalar()),
						"standard_weight": sw,
						"mystery_weight":  mw,
					}, row[0].Line)
			}
		}
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if strings.Contains(l, "=") && !strings.HasPrefix(l, "anchor.") {
				kv := strings.SplitN(l, "=", 2)
				c.EmitParam(f.Name, b.Raw, "board_algorithm",
					[]config.Value{config.VStr(strings.TrimSpace(kv[0]))},
					map[string]config.Value{
						"step": config.VStr(strings.TrimSpace(kv[0])),
						"expr": config.VStr(strings.TrimSpace(kv[1])),
					}, bl.Line+1+j)
				continue
			}
			if strings.HasPrefix(l, "anchor.") || strings.HasPrefix(l, "marker.") || strings.HasPrefix(l, "area.") {
				c.EmitParam(f.Name, b.Raw, "daily_anchor_pattern",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"pattern": config.VStr(l)}, bl.Line+1+j)
			}
		}
	}
	// finite anchor expansion: 18 FIELD maps from world_route
	wr := c.Catalogs["world_route_catalog.md"]
	if wr != nil {
		var keys []string
		for _, sec := range wr.Root.Children {
			for _, bl := range sec.Content {
				if bl.Kind != BlockTable || !hasHeaders(bl, "type") {
					continue
				}
				for ri := range bl.Cells {
					row := bl.Cells[ri]
					if strings.EqualFold(cellAt(row, 1).Scalar(), "FIELD") {
						keys = append(keys, cellAt(row, 0).Scalar())
					}
				}
			}
		}
		count := 0
		for _, mid := range keys {
			mk := strings.TrimPrefix(mid, "map.")
			for _, a := range []struct{ suffix, kind string }{
				{"p1", "ROUTE_POINT"}, {"p2", "ROUTE_POINT"},
				{"1", "MARKER"}, {"2", "MARKER"}, {"side", "SIDE_AREA"},
			} {
				var id string
				switch a.kind {
				case "ROUTE_POINT":
					id = "anchor.daily." + mk + "." + a.suffix
				case "MARKER":
					id = "marker.daily." + mk + "." + a.suffix
				default:
					id = "area.daily." + mk + "." + a.suffix
				}
				c.Emit(f.Name, b.Raw, "daily_anchor",
					[]config.Value{config.VStr(id)},
					map[string]config.Value{
						"anchor_id": config.VStr(id),
						"map_id":    config.VStr(mid),
						"kind":      config.VStr(a.kind),
					}, sec.Line)
				count++
			}
		}
	}
	c.consumed(f, b)
}

func flattenText(f *File) string {
	var sb strings.Builder
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, bl := range s.Content {
			for _, l := range bl.FLines {
				sb.WriteString(l + "\n")
			}
			for _, l := range bl.Prose {
				sb.WriteString(l + "\n")
			}
			for _, row := range bl.Cells {
				for _, cell := range row {
					sb.WriteString(cell.Text + "\n")
				}
			}
		}
		for _, ch := range s.Children {
			walk(ch)
		}
	}
	walk(f.Root)
	return sb.String()
}

func questSurgeEvent(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Spirit Surge Event Quest")
	if sec == nil {
		return
	}
	qid := "quest.event.spirit_surge.contribute"
	ord := 0
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if om := regexp.MustCompile(`^([0-9]+)\.\s+(.+)$`).FindStringSubmatch(l); om != nil {
				ord++
				c.Emit(f.Name, b.Raw, "quest_objective",
					[]config.Value{config.VStr(qid), config.VInt(int64(ord))},
					map[string]config.Value{
						"quest_id": config.VStr(qid),
						"ordinal":  config.VInt(int64(ord)),
						"verb":     config.VStr("EVENT"),
						"detail":   config.VStr(om[2]),
					}, bl.Line)
			}
		}
	}
	c.Emit(f.Name, b.Raw, "quest",
		[]config.Value{config.VStr(qid)},
		map[string]config.Value{
			"quest_id":        config.VStr(qid),
			"type":            config.VStr("EVENT"),
			"reward_delegate": config.VStr("drop.event.spirit_surge.<tier>.completion"),
		}, sec.Line)
	c.consumed(f, b)
}

// strconv alias guard
var _ = strconv.Itoa

var questVerbRe = regexp.MustCompile(`(TURN_IN|INTERACT|REACH|KILL|TALK|DUNGEON|BOSS|CHOOSE|EXPLORE)`)
var questIDRe = regexp.MustCompile(`([a-z_]+\.[a-z0-9_.]+)`)

func emitInlineObjective(c *Ctx, f *File, b *SourceBinding, qid string, ordinal int, tok string, line int) {
	det := strings.ReplaceAll(tok, "`", "")
	verb := ""
	if m := questVerbRe.FindStringSubmatch(det); m != nil {
		verb = m[1]
	}
	if verb == "" {
		verb = "SPECIAL"
	}
	fields := map[string]config.Value{
		"quest_id": config.VStr(qid),
		"ordinal":  config.VInt(int64(ordinal)),
		"verb":     config.VStr(verb),
		"detail":   config.VStr(det),
	}
	if km := regexp.MustCompile(`any\s+([0-9]+)\s+of\s+(.+)$`).FindStringSubmatch(det); km != nil {
		fields["count"] = config.VStr(km[1])
		var tg []config.Value
		for _, t := range strings.Split(km[2], "|") {
			tg = append(tg, config.VStr(strings.TrimSpace(t)))
		}
		fields["targets"] = config.VSet(tg...)
	} else if km := regexp.MustCompile(`KILL\s+([0-9]+)\s+(.+)`).FindStringSubmatch(det); km != nil {
		fields["count"] = config.VStr(km[1])
		fields["target"] = config.VStr(km[2])
	} else if km := regexp.MustCompile(`KILL\s+(.+)$`).FindStringSubmatch(det); km != nil {
		fields["target"] = config.VStr(km[1])
	} else if m := questIDRe.FindStringSubmatch(det); m != nil {
		fields["target"] = config.VStr(m[1])
	}
	c.Emit(f.Name, b.Raw, "quest_objective",
		[]config.Value{config.VStr(qid), config.VInt(int64(ordinal))}, fields, line)
}

var questZoneByAct = map[string]string{
	"a1": "lang_da", "a2": "rung_u_minh", "a3": "ben_nuoc_den",
	"a4": "deo_may", "a5": "thanh_co", "a6": "nui_thieng",
}

var inlineRewardRe = regexp.MustCompile("(EXP|common|material|bound|flag|item)\\s*`?([^`,]+)`?")

func emitInlineRewards(c *Ctx, f *File, b *SourceBinding, qid, rest string, line int) {
	for _, m := range inlineRewardRe.FindAllStringSubmatch(rest, -1) {
		kind := m[1]
		val := strings.TrimSpace(m[2])
		switch kind {
		case "EXP":
			v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(val, ",", ""))
			emitQuestRewardRec(c, f, b, qid, "exp", map[string]config.Value{"exp": v}, line)
		case "common":
			v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(val, ",", ""))
			emitQuestRewardRec(c, f, b, qid, "common", map[string]config.Value{"common": v}, line)
		case "bound":
			v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(val, ",", ""))
			emitQuestRewardRec(c, f, b, qid, "bound", map[string]config.Value{"bound": v}, line)
		case "material":
			v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(val, ",", ""))
			zone := questZoneByAct[questAct(qid)]
			emitQuestRewardRec(c, f, b, qid, "material", map[string]config.Value{
				"kind":     config.VStr("regional_material"),
				"region":   config.VStr(zone),
				"quantity": v,
			}, line)
		case "flag":
			emitQuestRewardRec(c, f, b, qid, "flag."+val, map[string]config.Value{
				"flag":  config.VStr(val),
				"value": config.VStr("true"),
			}, line)
		}
	}
}

func emitQuestRewardRec(c *Ctx, f *File, b *SourceBinding, qid, slot string, fields map[string]config.Value, line int) {
	fields["quest_id"] = config.VStr(qid)
	fields["reward_slot"] = config.VStr(slot)
	c.Emit(f.Name, b.Raw, "quest_reward",
		[]config.Value{config.VStr(qid), config.VStr(slot)}, fields, line)
}

func questAct(qid string) string {
	if i := strings.Index(qid, ".a"); i >= 0 && i+2 < len(qid) {
		return qid[i+1 : i+3]
	}
	return ""
}
