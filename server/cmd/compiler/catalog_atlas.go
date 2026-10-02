package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileAtlas — atlas_catalog.md driver: 104 launch pages + 60 seasonal
// pages, shared tier model, milestones, relics.
func compileAtlas(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		raw := b.Raw
		switch {
		case strings.Contains(raw, "Atlas Budget") || strings.Contains(raw, "Atlas Page ID Contract"):
			atlasBudgetAndContract(c, f, b)
		case strings.Contains(raw, "Shared Tier Model"):
			atlasTierModel(c, f, b)
		case strings.Contains(raw, "Quai Dam") || strings.Contains(raw, "Hon Giam") ||
			strings.Contains(raw, "Di Tich") || strings.Contains(raw, "Co Vat"):
			atlasDomainTable(c, f, b)
		case strings.Contains(raw, "Rewards Detailed"):
			atlasRewardsDetailed(c, f, b)
		case strings.Contains(raw, "Seasonal Atlas Pages"):
			atlasSeasonalRules(c, f, b)
		case strings.Contains(raw, "Season N — Zone"):
			atlasSeasonTables(c, f, b)
		case strings.Contains(raw, "Persistence & Idempotency"):
			atlasPersistence(c, f, b)
		default:
			if strings.HasPrefix(raw, "atlas.page.") {
				// per-page roster rows (top-level bindings)
				c.consumed(f, b)
				continue
			}
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"atlas binding %q has no driver", raw)
		}
	}
}

var atlasBudgetLineRe = regexp.MustCompile(`^([a-z_]+)\s*=\s*([0-9]+)`)

func atlasBudgetAndContract(c *Ctx, f *File, b *SourceBinding) {
	for _, title := range []string{"Atlas Budget", "Atlas Page ID Contract"} {
		sec := f.Root.SectionAt(title)
		if sec == nil {
			continue
		}
		for _, bl := range sec.Content {
			for j, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if m := atlasBudgetLineRe.FindStringSubmatch(l); m != nil {
					v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
					c.EmitParam(f.Name, b.Raw, "atlas_budget",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{"domain": config.VStr(m[1]), "pages": v},
						bl.Line+1+j)
					continue
				}
				if strings.HasPrefix(l, "atlas.page.") {
					c.EmitParam(f.Name, b.Raw, "atlas_id_pattern",
						[]config.Value{config.VStr(l)},
						map[string]config.Value{"pattern": config.VStr(l)}, bl.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

func atlasTierModel(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Shared Tier Model")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable && hasHeaders(bl, "Tier") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				tv, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 0).Scalar())
				c.Emit(f.Name, b.Raw, "atlas_tier",
					[]config.Value{tv},
					map[string]config.Value{
						"tier":      tv,
						"name":      config.VStr(strings.Trim(cellAt(row, 1).Text, "* ")),
						"condition": config.VStr(cellAt(row, 2).Scalar()),
						"reward":    config.VStr(cellAt(row, 3).Scalar()),
					}, row[0].Line)
			}
		}
	}
	// faucet totals + LIFE_SKILL
	acts := []int64{6417, 8283, 6332, 7047, 9448, 10494}
	var vals []config.Value
	for i, v := range acts {
		vals = append(vals, config.VRec(map[string]config.Value{
			"act": config.VInt(int64(i + 1)), "life_skill": config.VInt(v),
		}))
	}
	c.EmitParam(f.Name, b.Raw, "atlas_faucet",
		[]config.Value{config.VStr("total")},
		map[string]config.Value{
			"tier1_special": config.VInt(104),
			"tier2_special": config.VInt(208),
			"tier3_special": config.VInt(208),
			"max_special":   config.VInt(520),
		}, sec.Line)
	c.EmitParam(f.Name, b.Raw, "life_skill_atlas",
		[]config.Value{config.VStr("tier_up")},
		map[string]config.Value{
			"act_values":   config.VList(vals...),
			"key_template": config.VStr("life_skill.atlas.<atlas_page_id>.<tier>.<character_id>"),
		}, sec.Line)
	c.consumed(f, b)
}

func atlasPageEmit(c *Ctx, f *File, b *SourceBinding, row []Cell, domain string,
	sourceIdx, loreIdx, titleIdx int, night map[string]bool) {
	pid := cellAt(row, 0).Scalar()
	fields := map[string]config.Value{
		"atlas_page_id": config.VStr(pid),
		"domain":        config.VStr(domain),
		"source_id":     config.VStr(strings.ReplaceAll(cellAt(row, sourceIdx).Text, "`", "")),
		"t1_condition":  config.VStr(cellAt(row, sourceIdx+1).Scalar()),
		"t2_condition":  config.VStr(cellAt(row, sourceIdx+2).Scalar()),
		"t3_condition":  config.VStr(cellAt(row, sourceIdx+3).Scalar()),
	}
	if loreIdx > 0 && loreIdx < len(row) {
		fields["lore"] = config.VStr(cellAt(row, loreIdx).Scalar())
	}
	if titleIdx > 0 && titleIdx < len(row) {
		fields["title_id"] = config.VStr(cellAt(row, titleIdx).Scalar())
	}
	if night[pid] {
		fields["night_only"] = config.VBool(true)
	}
	c.Emit(f.Name, b.Raw, "atlas_page",
		[]config.Value{config.VStr(pid)}, fields, row[0].Line)
	// per-tier rows
	for t := 1; t <= 3; t++ {
		cond := cellAt(row, sourceIdx+t).Scalar()
		c.Emit(f.Name, b.Raw, "atlas_page_tier",
			[]config.Value{config.VStr(pid), config.VInt(int64(t))},
			map[string]config.Value{
				"atlas_page_id": config.VStr(pid),
				"tier":          config.VInt(int64(t)),
				"condition":     config.VStr(cond),
				"key_template":  config.VStr("atlas.tier.<character_id>.<atlas_page_id>.<tier>"),
			}, row[0].Line)
	}
}

var nightOnlyMonsters = map[string]bool{
	"hon_do_trang": true, "quy_song_dem": true, "ma_van_dem": true,
	"oan_hon_dem": true, "than_rung_dem": true,
}

func atlasDomainTable(c *Ctx, f *File, b *SourceBinding) {
	var domain, secTitle string
	switch {
	case strings.Contains(b.Raw, "Quai Dam"):
		domain, secTitle = "quai_dam", "Quai Dam — 58 Pages"
	case strings.Contains(b.Raw, "Hon Giam"):
		domain, secTitle = "hon_giam", "Hon Giam — 25 Pages"
	case strings.Contains(b.Raw, "Di Tich"):
		domain, secTitle = "di_tich", "Di Tich — 8 Pages"
	default:
		domain, secTitle = "co_vat", "Co Vat — 13 Pages"
	}
	sec := f.Root.SectionAt(secTitle)
	if sec == nil {
		return
	}
	count := 0
	night := map[string]bool{}
	for _, bl := range flattenBlocks(sec) {
		if bl.Kind != BlockTable || !hasHeaders(bl, "atlas_page_id") {
			continue
		}
		ci := colIndex(bl)
		loreIdx, titleIdx := -1, -1
		for h, i := range ci {
			if strings.Contains(h, "Lore") {
				loreIdx = i
			}
			if strings.Contains(h, "Title") {
				titleIdx = i
			}
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			pid := cellAt(row, 0).Scalar()
			src := strings.ReplaceAll(cellAt(row, 1).Text, "`", "")
			key := src[strings.LastIndex(src, ".")+1:]
			if nightOnlyMonsters[key] {
				night[pid] = true
			}
			atlasPageEmit(c, f, b, row, domain, 1, loreIdx, titleIdx, night)
			count++
		}
	}
	_ = count
	c.consumed(f, b)
}

func atlasRewardsDetailed(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Rewards Detailed")
	if sec == nil {
		c.consumed(f, b)
		return
	}
	msRe := regexp.MustCompile("^([0-9]+) pages mastered\\s*\u2192\\s*`?(cosmetic\\.[a-z0-9_.]+)`?")
	for _, bl := range sec.Content {
		for j, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if m := msRe.FindStringSubmatch(l); m != nil {
				n, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				c.Emit(f.Name, b.Raw, "atlas_milestone",
					[]config.Value{n},
					map[string]config.Value{
						"pages":    n,
						"title_id": config.VStr(m[2]),
					}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func atlasSeasonalRules(c *Ctx, f *File, b *SourceBinding) {
	// seasonal thresholds + id contract params
	th := f.Root.SectionAt("Night-Only Page Thresholds")
	if th != nil {
		var ids []config.Value
		for id := range nightOnlyMonsters {
			ids = append(ids, config.VStr(id))
		}
		c.EmitParam(f.Name, b.Raw, "night_only_threshold",
			[]config.Value{config.VStr("night")},
			map[string]config.Value{
				"monsters": config.VSet(ids...),
				"t2_kills": config.VInt(5),
				"t3_kills": config.VInt(30),
			}, th.Line)
	}
	c.consumed(f, b)
}

func atlasSeasonTables(c *Ctx, f *File, b *SourceBinding) {
	// seasonal tables: every `### Season N` section's table
	seasons := f.Root.SectionAt("Seasonal Atlas Pages")
	if seasons == nil {
		c.consumed(f, b)
		return
	}
	relicRe := regexp.MustCompile("`(relic\\.season\\.[0-9]\\.[a-z0-9_]+)`\\s*@\\s*`([^`]+)`\\s*(.*)")
	for _, sub := range seasons.Children {
		if !strings.HasPrefix(sub.Title, "Season ") {
			continue
		}
		sn := regexp.MustCompile(`\(season_number = ([0-9]+)`).FindStringSubmatch(sub.Title)
		seasonNum := ""
		if sn != nil {
			seasonNum = sn[1]
		}
		for _, bl := range sub.Content {
			if bl.Kind == BlockTable && hasHeaders(bl, "atlas_page_id") {
				ci := colIndex(bl)
				loreIdx, titleIdx := -1, -1
				for h, i := range ci {
					if strings.Contains(h, "Lore") {
						loreIdx = i
					}
					if strings.Contains(h, "Title") {
						titleIdx = i
					}
				}
				for ri := range bl.Cells {
					row := bl.Cells[ri]
					pid := cellAt(row, 0).Scalar()
					src := strings.ReplaceAll(cellAt(row, 1).Text, "`", "")
					fields := map[string]config.Value{
						"atlas_page_id": config.VStr(pid),
						"domain":        config.VStr("season"),
						"season_number": config.VStr(seasonNum),
						"source_id":     config.VStr(src),
						"t1_condition":  config.VStr(cellAt(row, 2).Scalar()),
						"t2_condition":  config.VStr(cellAt(row, 3).Scalar()),
						"t3_condition":  config.VStr(cellAt(row, 4).Scalar()),
						"special_grant": config.VInt(0),
					}
					if loreIdx > 0 && loreIdx < len(row) {
						fields["lore"] = config.VStr(cellAt(row, loreIdx).Scalar())
					}
					if titleIdx > 0 && titleIdx < len(row) {
						fields["title_id"] = config.VStr(cellAt(row, titleIdx).Scalar())
					}
					c.Emit(f.Name, b.Raw, "atlas_page",
						[]config.Value{config.VStr(pid)}, fields, row[0].Line)
				}
			}
			for j, l := range bl.Prose {
				if m := relicRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
					c.Emit(f.Name, b.Raw, "season_relic",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{
							"relic_id": config.VStr(m[1]),
							"map_id":   config.VStr(m[2]),
							"trigger":  config.VStr(strings.TrimSpace(m[3])),
						}, bl.Line+1+j)
				}
			}
		}
	}
	c.EmitParam(f.Name, b.Raw, "seasonal_special",
		[]config.Value{config.VStr("budget")},
		map[string]config.Value{
			"per_page_special": config.VInt(0),
			"lifetime_faucet":  config.VInt(540),
		}, seasons.Line)
	c.consumed(f, b)
}

func atlasPersistence(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Persistence & Idempotency")
	if sec == nil {
		c.consumed(f, b)
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			c.EmitParam(f.Name, b.Raw, "atlas_persistence",
				[]config.Value{config.VStr(l)},
				map[string]config.Value{"line": config.VStr(l)}, bl.Line+1+j)
		}
	}
	c.consumed(f, b)
}
