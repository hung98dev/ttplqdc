package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileDungeon — dungeon_catalog.md driver (10 bindings): shared defaults,
// instance geometry, first-clear EXP, 5 dungeon rosters with stage/wave
// detail, endgame tag + stat profile + remixes, settlement, repeat EXP,
// EXP channel rules.
func compileDungeon(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Shared Rules"):
			dgSharedRules(c, f, b)
		case strings.HasPrefix(path, "Canonical Instance Bounds"):
			dgBounds(c, f, b)
		case strings.HasPrefix(path, "Progression First-Clear EXP"):
			dgFirstClear(c, f, b)
		case dgDungeonRe.MatchString(path):
			dgRoster(c, f, b)
		case strings.HasPrefix(path, "Level-60 Endgame-Tagged Runs"):
			dgEndgameTag(c, f, b)
		case strings.HasPrefix(path, "Explicit Level-60 Stat Profile"):
			dgStatProfile(c, f, b)
		case strings.HasPrefix(path, "Concrete Mechanic Remixes"):
			dgRemixes(c, f, b)
		case strings.HasPrefix(path, "Endgame Reward Settlement"):
			dgSettlement(c, f, b)
		case strings.HasPrefix(path, "Dungeon Repeat EXP"):
			dgRepeatEXP(c, f, b)
		case strings.HasPrefix(path, "Dungeon EXP"):
			dgEXPRules(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"dungeon binding %q has no driver", b.Raw)
		}
	}
}

var dgDungeonRe = regexp.MustCompile(`^#?\s*N\s*—`)
var rewardSlotRe = regexp.MustCompile(`^([A-Z_]+).*->\s*(drop\.[a-z0-9_.<>]+)`)
var dgStageHeadRe = regexp.MustCompile("^([0-9]+)\\.\\s*`?(stage\\.[a-z0-9_.]+)`?\\s*$")
var dgWaveRe = regexp.MustCompile("`?(w[0-9]+)`?\\s*(?:\\(([^)]*)\\))?\\s*:")
var dgMonsterRefRe = regexp.MustCompile("([0-9]+)\\s+(monster\\.[a-z0-9_.]+)")
var dgSecretRe = regexp.MustCompile("`?(secret\\.[a-z0-9_.]+)`?")

func dgSharedRules(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if m := rewardSlotRe.FindStringSubmatch(l); m != nil {
					c.EmitParam(f.Name, b.Raw, "reward_slot",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{
							"slot":       config.VStr(m[1]),
							"drop_table": config.VStr(m[2]),
						}, fb.Line+1+j)
					continue
				}
				if m := chestFieldRe.FindStringSubmatch(l); m != nil {
					c.EmitParam(f.Name, b.Raw, "dungeon_default",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{"value": config.VStr(m[2])}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

func dgBounds(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "dungeon_id / space_id,span (screens),bounds max (m),reference extent (px),layout_profile,required traversable topology"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				did := cellAt(row, 0).Scalar()
				span, ok1 := pairRat(cellAt(row, 1).Scalar())
				bounds, ok2 := pairRat(cellAt(row, 2).Scalar())
				ext, ok3 := pairInt(cellAt(row, 3).Scalar())
				if !ok1 || !ok2 || !ok3 {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"dungeon bounds row %s malformed", did)
					continue
				}
				c.EmitGeom(f.Name, b.Raw,
					[]config.Value{config.VStr(did)},
					map[string]config.Value{
						"space_id":            config.VStr(did),
						"kind":                config.VStr("DUNGEON"),
						"span_screens":        span,
						"bounds_max_m":        bounds,
						"reference_extent_px": ext,
						"layout_profile":      config.VStr(cellAt(row, 4).Scalar()),
						"required_topology":   config.VStr(cellAt(row, 5).Scalar()),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func dgFirstClear(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "dungeon_id,Act,act_exp_total,first_progression_clear_exp"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				did := cellAt(row, 0).Scalar()
				total, _ := (TypeSpec{Name: "grouped_int"}).ParseValue(cellAt(row, 2).Scalar())
				exp, _ := (TypeSpec{Name: "grouped_int"}).ParseValue(cellAt(row, 3).Scalar())
				c.Emit(f.Name, b.Raw, "first_clear_exp",
					[]config.Value{config.VStr(did)},
					map[string]config.Value{
						"dungeon_id":    config.VStr(did),
						"act":           config.VStr(cellAt(row, 1).Scalar()),
						"act_exp_total": total,
						"exp":           exp,
						"reward_key":    config.VStr("reward.first_progression_clear." + did + ".<character_id>"),
						"share_of_act":  config.VStr("0.4%"),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

var dgHeadRe = regexp.MustCompile(`^[0-9]+\s*—`)

// dgRoster — `# N — <name>` dungeon sections: assignment fence + Stages list.
// The registry path is a template (`# N — <name>`); iterate real headings.
func dgRoster(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range f.Root.Children {
		if !dgHeadRe.MatchString(sec.Title) {
			continue
		}
		var dg map[string]config.Value
		var line int
		for _, bl := range sec.Content {
			if bl.Kind != BlockFence {
				continue
			}
			dg = map[string]config.Value{}
			line = bl.Line + 1
			for j, l := range bl.FLines {
				l = strings.TrimSpace(l)
				m := chestFieldRe.FindStringSubmatch(l)
				if m == nil {
					continue
				}
				if j == 0 {
					line = bl.Line + 1 + j
				}
				switch m[1] {
				case "dungeon_id":
					dg["dungeon_id"] = config.VStr(m[2])
				case "recommended_level", "minimum_level":
					v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
					dg[m[1]] = v
				case "final_boss":
					dg["final_boss"] = config.VStr(m[2])
				case "target_time":
					parts := strings.Split(strings.TrimSuffix(m[2], "m"), "..")
					if len(parts) == 2 {
						lo, _ := (TypeSpec{Name: "int"}).ParseValue(parts[0])
						hi, _ := (TypeSpec{Name: "int"}).ParseValue(parts[1])
						dg["target_minutes"] = config.VRec(map[string]config.Value{"lo": lo, "hi": hi})
					}
				}
			}
		}
		if dg["dungeon_id"].Kind == 0 {
			c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
				"dungeon section %q missing assignment fence", sec.Title)
			continue
		}
		did := dg["dungeon_id"].Str
		// defaults inherited from Shared Rules
		dg["type"] = config.VStr("PARTY")
		dg["party_size"] = config.VRec(map[string]config.Value{"lo": config.VInt(1), "hi": config.VInt(5)})
		dg["difficulty"] = config.VStr("NORMAL")
		dg["lockout"] = config.VStr("NONE")
		dg["encounter_scaling"] = config.VStr("PARTY_DEFAULT")
		c.Emit(f.Name, b.Raw, "dungeon", []config.Value{config.VStr(did)}, dg, line)
		// endgame variant (named finite rule)
		c.Emit(f.Name, b.Raw, "endgame_variant",
			[]config.Value{config.VStr(did), config.VStr("endgame")},
			map[string]config.Value{
				"dungeon_id":    config.VStr(did),
				"variant_id":    config.VStr("endgame"),
				"run_tag":       config.VStr("ENDGAME_L60"),
				"minimum_level": config.VInt(60),
				"reward_slot":   config.VStr("ENDGAME_REWARD"),
			}, line)
		dgStages(c, f, b, sec, did)
	}
	c.consumed(f, b)
}

func dgStages(c *Ctx, f *File, b *SourceBinding, sec *Section, did string) {
	var stage *struct {
		id         string
		order      int
		objective  []string
		waves      []config.Value
		boss       string
		checkpoint bool
		line       int
	}
	var stages []config.Value
	var secretID string
	var secretLines []string
	inStages := false
	inSecret := false
	flush := func() {
		if stage != nil {
			fields := map[string]config.Value{
				"dungeon_id": config.VStr(did),
				"stage_id":   config.VStr(stage.id),
				"order":      config.VInt(int64(stage.order)),
				"waves":      config.VList(stage.waves...),
			}
			if len(stage.objective) > 0 {
				fields["objective"] = config.VStr(strings.Join(stage.objective, " | "))
			}
			if stage.boss != "" {
				fields["boss_id"] = config.VStr(stage.boss)
			}
			if stage.checkpoint {
				fields["checkpoint"] = config.VBool(true)
			}
			c.Emit(f.Name, b.Raw, "dungeon_stage",
				[]config.Value{config.VStr(did), config.VStr(stage.id)}, fields, stage.line)
			stages = append(stages, config.VStr(stage.id))
			stage = nil
		}
	}
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if strings.HasPrefix(l, "Stages:") {
				inStages = true
				continue
			}
			if strings.HasPrefix(l, "Optional secret") {
				flush()
				inStages = false
				inSecret = true
				if m := dgSecretRe.FindStringSubmatch(l); m != nil {
					secretID = m[1]
				}
				continue
			}
			if m := dgStageHeadRe.FindStringSubmatch(l); m != nil {
				flush()
				inStages = true
				inSecret = false
				n, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				stage = &struct {
					id         string
					order      int
					objective  []string
					waves      []config.Value
					boss       string
					checkpoint bool
					line       int
				}{id: m[2], order: int(n.Int), line: bl.Line}
				continue
			}
			if inSecret && secretID != "" {
				secretLines = append(secretLines, strings.TrimPrefix(l, "- "))
				continue
			}
			if !inStages || stage == nil {
				continue
			}
			body := strings.TrimPrefix(l, "- ")
			if dgWaveRe.MatchString(body) {
				// wave spec — several `w<n>: ...` pieces may join on `;`
				for _, piece := range strings.Split(body, ";") {
					wm := dgWaveRe.FindStringSubmatch(piece)
					if wm == nil {
						continue
					}
					var mons []config.Value
					for _, mm := range dgMonsterRefRe.FindAllStringSubmatch(piece, -1) {
						cnt, _ := (TypeSpec{Name: "int"}).ParseValue(mm[1])
						mons = append(mons, config.VRec(map[string]config.Value{
							"count": cnt, "monster_id": config.VStr(mm[2]),
						}))
					}
					stageKey := strings.TrimPrefix(stage.id, "stage.")
					wfields := map[string]config.Value{
						"wave":      config.VStr(wm[1]),
						"monsters":  config.VList(mons...),
						"anchor_id": config.VStr("anchor.dungeon." + stageKey + "." + wm[1]),
					}
					if wm[2] != "" {
						wfields["area_trigger"] = config.VStr(wm[2])
					}
					stage.waves = append(stage.waves, config.VRec(wfields))
				}
				continue
			}
			if m := regexp.MustCompile(`boss\s+(boss\.[a-z0-9_.]+)`).FindStringSubmatch(body); m != nil {
				stage.boss = m[1]
				continue
			}
			if strings.Contains(body, "checkpoint") {
				stage.checkpoint = true
			}
			stage.objective = append(stage.objective, body)
		}
	}
	flush()
	if secretID != "" {
		c.Emit(f.Name, b.Raw, "dungeon_secret",
			[]config.Value{config.VStr(did), config.VStr(secretID)},
			map[string]config.Value{
				"dungeon_id": config.VStr(did),
				"secret_id":  config.VStr(secretID),
				"steps":      config.VStr(strings.Join(secretLines, " | ")),
			}, sec.Line)
	}
}

func dgEndgameTag(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := chestFieldRe.FindStringSubmatch(l); m != nil {
					c.EmitParam(f.Name, b.Raw, "endgame_tag",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{"value": config.VStr(m[2])}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

func dgStatProfile(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			rank := "rules"
			fields := map[string]config.Value{}
			var title string
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if title == "" && !strings.Contains(l, "=") {
					title = l
					if strings.Contains(l, "ELITE") {
						rank = "ELITE"
					} else if strings.Contains(l, "NORMAL") {
						rank = "NORMAL"
					}
					continue
				}
				if m := chestFieldRe.FindStringSubmatch(l); m != nil {
					if v, err := (TypeSpec{Name: "int"}).ParseValue(m[2]); err == nil {
						fields[m[1]] = v
					} else if rv, err := parseDecimal(m[2]); err == nil {
						fields[m[1]] = mustRat(rv)
					} else {
						fields[m[1]] = config.VStr(m[2])
					}
				} else if strings.HasSuffix(l, ":") && !strings.Contains(l, "=") {
					// label line e.g. "NORMAL L60 baseline:"
					if strings.Contains(l, "ELITE") {
						rank = "ELITE"
					} else if strings.Contains(l, "NORMAL") {
						rank = "NORMAL"
					}
				}
			}
			if len(fields) == 0 {
				continue
			}
			key := rank
			if title != "" {
				key = rank + "/" + strings.ReplaceAll(title, " ", "_")
			}
			fields["label"] = config.VStr(title)
			c.EmitParam(f.Name, b.Raw, "endgame_stat_profile",
				[]config.Value{config.VStr(key)}, fields, fb.Line+1)
		}
	}
	c.consumed(f, b)
}

var endgameHeadRe = regexp.MustCompile(`^endgame\.([a-z0-9_]+)\.([a-z0-9_]+)$`)

func dgRemixes(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, ch := range sec.Children {
			vid := ch.StableID()
			m := endgameHeadRe.FindStringSubmatch(vid)
			if m == nil {
				continue
			}
			var bullets []string
			for _, bl := range ch.Content {
				for _, l := range bl.Prose {
					l = strings.TrimSpace(strings.TrimPrefix(l, "- "))
					if l != "" {
						bullets = append(bullets, l)
					}
				}
			}
			bv := make([]config.Value, len(bullets))
			for i, s := range bullets {
				bv[i] = config.VStr(s)
			}
			c.Emit(f.Name, b.Raw, "endgame_remix",
				[]config.Value{config.VStr("dungeon." + m[1]), config.VStr(vid)},
				map[string]config.Value{
					"dungeon_id": config.VStr("dungeon." + m[1]),
					"variant_id": config.VStr(vid),
					"rules":      config.VList(bv...),
				}, ch.Line)
		}
	}
	c.consumed(f, b)
}

func dgSettlement(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				c.EmitParam(f.Name, b.Raw, "endgame_settlement_rule",
					[]config.Value{config.VInt(int64(fb.Line*1000 + j))},
					map[string]config.Value{"rule": config.VStr(l)}, fb.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func dgRepeatEXP(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if i := strings.IndexByte(l, '='); i > 0 && strings.Contains(l, "(") {
					name := strings.TrimSpace(l[:i])
					if ex, err := ParseExpr(strings.TrimSpace(l[i+1:])); err == nil {
						c.EmitParam(f.Name, b.Raw, "dungeon_repeat_formula",
							[]config.Value{config.VStr(name)},
							map[string]config.Value{"expr": config.VExpr(ex)}, fb.Line+1+j)
						continue
					}
				}
				c.EmitParam(f.Name, b.Raw, "dungeon_repeat_note",
					[]config.Value{config.VInt(int64(fb.Line*1000 + j))},
					map[string]config.Value{"note": config.VStr(l)}, fb.Line+1+j)
			}
		}
		if tbl := findTable(sec, "Act,act_exp_total,target_hours,denominator,dungeon_repeat_exp"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				gi := func(i int) config.Value {
					v, _ := (TypeSpec{Name: "grouped_int"}).ParseValue(cellAt(row, i).Scalar())
					return v
				}
				c.Emit(f.Name, b.Raw, "dungeon_repeat_exp",
					[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
					map[string]config.Value{
						"act":                config.VStr(cellAt(row, 0).Scalar()),
						"act_exp_total":      gi(1),
						"target_hours":       gi(2),
						"denominator":        gi(3),
						"dungeon_repeat_exp": gi(4),
						"act_dispatch":       config.VStr("min(character_act, dungeon_tier_act + 1)"),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func dgEXPRules(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		var idx int
		for _, bl := range sec.Content {
			for _, l := range bl.Prose {
				l = strings.TrimSpace(strings.TrimPrefix(l, "- "))
				if l == "" {
					continue
				}
				idx++
				c.EmitParam(f.Name, b.Raw, "dungeon_exp_rule",
					[]config.Value{config.VInt(int64(idx))},
					map[string]config.Value{"rule": config.VStr(l)}, bl.Line)
			}
		}
	}
	c.consumed(f, b)
}
