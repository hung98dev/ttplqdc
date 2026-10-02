package main

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
	"thinhthan/internal/config"
)

// compileCosmetic — cosmetic_catalog.md driver: titles, frames, appearances,
// feats, sinks, seasonal matrix, IAP products, guild cosmetics.
func compileCosmetic(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		raw := b.Raw
		switch {
		case strings.Contains(raw, "TITLE"):
			cosmoTitles(c, f, b)
		case strings.Contains(raw, "PROFILE_FRAME"):
			cosmoTable(c, f, b, "PROFILE_FRAME — 9", "PROFILE_FRAME")
			cosmoFrameRedemption(c, f, b)
		case strings.Contains(raw, "CHARACTER_APPEARANCE"):
			cosmoAppearances(c, f, b)
		case strings.Contains(raw, "Special-Currency Choice"):
			cosmoFencesToParams(c, f, b, "Special-Currency Choice", "special_choice")
		case strings.Contains(raw, "FOLKLORE FEATS"):
			cosmoFeats(c, f, b)
		case strings.Contains(raw, "CURRENCY.COMMON COSMETIC SINKS"):
			cosmoCommonSinks(c, f, b)
		case strings.Contains(raw, "SEASONAL COSMETICS"):
			cosmoSeasonal(c, f, b)
		case strings.Contains(raw, "IAP STORE"):
			cosmoIAP(c, f, b)
		case strings.Contains(raw, "SPECIAL-CURRENCY SINKS"):
			cosmoTable(c, f, b, "SPECIAL-CURRENCY SINKS — 20", "SPECIAL_SINK")
		case strings.Contains(raw, "GUILD-SCOPED"):
			cosmoGuild(c, f, b)
		case strings.Contains(raw, "Source / Duplicate Semantics"):
			cosmoFencesToParams(c, f, b, "Source / Duplicate Semantics", "operation_key")
		case strings.Contains(raw, "Spirit Surge Cosmetic Material"):
			cosmoFencesToParams(c, f, b, "Spirit Surge Cosmetic Material", "material_sink")
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"cosmetic binding %q has no driver", raw)
		}
	}
}

func cosmoTitles(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("TITLE — 20 Core + 107 Atlas = 127")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		if hasHeaders(bl, "cosmetic_id") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				cid := cellAt(row, 0).Scalar()
				c.Emit(f.Name, b.Raw, "cosmetic",
					[]config.Value{config.VStr(cid)},
					map[string]config.Value{
						"cosmetic_id": config.VStr(cid),
						"kind":        config.VStr("TITLE"),
						"display":     config.VStr(norm.NFC.String(cellAt(row, 1).Scalar())),
						"unlock":      config.VStr(cellAt(row, 2).Scalar()),
						"scope":       config.VStr("CHARACTER"),
					}, row[0].Line)
			}
		} else if hasHeaders(bl, "Count") {
			// atlas counting table: expansion happens below via atlas_page defs
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				n, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 0).Scalar())
				c.EmitParam(f.Name, b.Raw, "atlas_title_count",
					[]config.Value{config.VStr(cellAt(row, 1).Scalar())},
					map[string]config.Value{
						"pattern": config.VStr(cellAt(row, 1).Scalar()),
						"count":   n,
						"example": config.VStr(cellAt(row, 2).Scalar()),
					}, row[0].Line)
			}
		}
	}
	// 107 atlas titles expand from atlas_page title_id fields + milestones
	ap := c.Defs.Get("atlas_page")
	if ap != nil {
		for _, ks := range ap.SortedKeys() {
			rec := ap.Records[config.KeyString(ks)]
			if rec.Fields["domain"].Str == "season" {
				continue
			}
			tid, ok := rec.Fields["title_id"]
			if !ok || tid.Str == "" || tid.Str == "—" {
				continue
			}
			c.Emit(f.Name, b.Raw, "cosmetic",
				[]config.Value{config.VStr(tid.Str)},
				map[string]config.Value{
					"cosmetic_id":   tid,
					"kind":          config.VStr("TITLE_ATLAS"),
					"atlas_page_id": config.VStr(rec.Fields["atlas_page_id"].Str),
					"scope":         config.VStr("CHARACTER"),
				}, 0)
		}
	}
	ms := c.Defs.Get("atlas_milestone")
	if ms != nil {
		for _, ks := range ms.SortedKeys() {
			rec := ms.Records[config.KeyString(ks)]
			tid := rec.Fields["title_id"]
			c.Emit(f.Name, b.Raw, "cosmetic",
				[]config.Value{tid},
				map[string]config.Value{
					"cosmetic_id": tid,
					"kind":        config.VStr("TITLE_ATLAS_MILESTONE"),
					"scope":       config.VStr("CHARACTER"),
				}, 0)
		}
	}
	c.consumed(f, b)
}

func cosmoTable(c *Ctx, f *File, b *SourceBinding, secTitle, kind string) {
	sec := f.Root.SectionAt(secTitle)
	if sec == nil {
		c.consumed(f, b)
		return
	}
	for _, bl := range flattenBlocks(sec) {
		if bl.Kind != BlockTable || !hasHeaders(bl, "cosmetic_id") {
			continue
		}
		ci := colIndex(bl)
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			cid := cellAt(row, 0).Scalar()
			fields := map[string]config.Value{
				"cosmetic_id": config.VStr(cid),
				"kind":        config.VStr(kind),
				"scope":       config.VStr("CHARACTER"),
			}
			if i, ok := ci["Display"]; ok {
				fields["display"] = config.VStr(norm.NFC.String(cellAt(row, i).Scalar()))
			}
			if i, ok := ci["Display (vi-VN)"]; ok {
				fields["display"] = config.VStr(norm.NFC.String(cellAt(row, i).Scalar()))
			}
			if i, ok := ci["Unlock source"]; ok {
				fields["unlock"] = config.VStr(cellAt(row, i).Scalar())
			}
			if i, ok := ci["Unlock condition"]; ok {
				fields["unlock"] = config.VStr(cellAt(row, i).Scalar())
			}
			if i, ok := ci["`currency.common` cost"]; ok {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, i).Scalar(), ",", ""))
				fields["cost_common"] = v
			}
			if i, ok := ci["kind"]; ok {
				fields["sink_kind"] = config.VStr(cellAt(row, i).Scalar())
			}
			c.Emit(f.Name, b.Raw, "cosmetic",
				[]config.Value{config.VStr(cid)}, fields, row[0].Line)
		}
	}
	c.consumed(f, b)
}

var cosmoHeadRe = regexp.MustCompile("^`?(cosmetic\\.[a-z0-9_.]+)`?\\s*$")

func cosmoFrameRedemption(c *Ctx, f *File, b *SourceBinding) {
	// `cosmetic.frame.nui_thieng` redemption fences inside PROFILE_FRAME section
	sec := f.Root.SectionAt("PROFILE_FRAME — 9")
	if sec == nil {
		return
	}
	for _, ch := range sec.Children {
		m := cosmoHeadRe.FindStringSubmatch(strings.TrimSpace(ch.Title))
		if m == nil {
			continue
		}
		cid := m[1]
		routeIdx := 0
		for _, bl := range ch.Content {
			var kv []config.Value
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if f2 := chestFieldRe.FindStringSubmatch(l); f2 != nil {
					kv = append(kv, config.VStr(f2[1]+"="+strings.TrimSpace(f2[2])))
				}
			}
			if len(kv) > 0 {
				routeIdx++
				c.Emit(f.Name, b.Raw, "redemption_route",
					[]config.Value{config.VStr(cid), config.VInt(int64(routeIdx))},
					map[string]config.Value{
						"cosmetic_id": config.VStr(cid),
						"fields":      config.VSet(kv...),
					}, bl.Line)
			}
		}
	}
}

func cosmoAppearances(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("CHARACTER_APPEARANCE — 4")
	if sec == nil {
		return
	}
	for _, ch := range sec.Children {
		m := cosmoHeadRe.FindStringSubmatch(strings.TrimSpace(ch.Title))
		if m == nil {
			continue
		}
		cid := m[1]
		fields := map[string]config.Value{
			"cosmetic_id": config.VStr(cid),
			"kind":        config.VStr("CHARACTER_APPEARANCE"),
			"scope":       config.VStr("CHARACTER"),
		}
		var routes []config.Value
		for _, bl := range ch.Content {
			for _, l := range bl.Prose {
				l = strings.TrimSpace(l)
				if m2 := displayLineRe.FindStringSubmatch(l); m2 != nil {
					fields["display"] = config.VStr(norm.NFC.String(m2[1]))
				}
				if strings.HasPrefix(l, "Unlock:") {
					fields["unlock"] = config.VStr(strings.TrimSpace(strings.TrimPrefix(l, "Unlock:")))
				}
			}
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if f2 := chestFieldRe.FindStringSubmatch(l); f2 != nil {
					routes = append(routes, config.VStr(f2[1]+"="+strings.TrimSpace(f2[2])))
				}
			}
		}
		if len(routes) > 0 {
			fields["redemption"] = config.VSet(routes...)
		}
		c.Emit(f.Name, b.Raw, "cosmetic",
			[]config.Value{config.VStr(cid)}, fields, ch.Line)
	}
	c.consumed(f, b)
}

func cosmoFeats(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("FOLKLORE FEATS CATALOG")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind == BlockTable && hasHeaders(bl, "feat_id") {
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				fid := cellAt(row, 0).Scalar()
				c.Emit(f.Name, b.Raw, "feat",
					[]config.Value{config.VStr(fid)},
					map[string]config.Value{
						"feat_id":       config.VStr(fid),
						"feat_type":     config.VStr(strings.Trim(cellAt(row, 1).Text, "`")),
						"threshold":     config.VStr(cellAt(row, 2).Scalar()),
						"tracked_event": config.VStr(cellAt(row, 3).Scalar()),
						"cosmetic_id":   config.VStr(cellAt(row, 4).Scalar()),
						"key_template":  config.VStr("character_id + feat_id + milestone_threshold"),
					}, row[0].Line)
			}
		}
	}
	// counter rules + idempotency key fence
	for _, ch := range sec.Children {
		for _, bl := range ch.Content {
			var lines []config.Value
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if l != "" {
					lines = append(lines, config.VStr(l))
				}
			}
			if len(lines) > 0 {
				c.EmitParam(f.Name, b.Raw, "feat_rule",
					[]config.Value{config.VStr(ch.Title)},
					map[string]config.Value{"rules": config.VSet(lines...)}, bl.Line)
			}
		}
	}
	c.consumed(f, b)
}

func cosmoCommonSinks(c *Ctx, f *File, b *SourceBinding) {
	cosmoTable(c, f, b, "CURRENCY.COMMON COSMETIC SINKS", "COMMON_SINK")
	// redemption rules bullets
	sec := f.Root.SectionAt("Common Sink Redemption Rules")
	if sec != nil {
		for _, bl := range sec.Content {
			for j, l := range bl.Prose {
				l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "-"))
				if l == "" {
					continue
				}
				c.EmitParam(f.Name, b.Raw, "redemption_rule",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"rule": config.VStr(l)}, bl.Line+1+j)
			}
		}
	}
}

func cosmoSeasonal(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("SEASONAL COSMETICS")
	if sec == nil {
		return
	}
	for _, bl := range flattenBlocks(sec) {
		if bl.Kind != BlockTable || !hasHeaders(bl, "cosmetic_id") {
			// seasons 1-5 matrix table uses n/free title/... headers
			if bl.Kind == BlockTable && hasHeaders(bl, "n") {
				for ri := range bl.Cells {
					row := bl.Cells[ri]
					for ci := 1; ci < len(row); ci++ {
						cid := cellAt(row, ci).Scalar()
						if cid == "" {
							continue
						}
						track := "FREE"
						if strings.Contains(cid, ".paid") {
							track = "PAID"
						}
						c.Emit(f.Name, b.Raw, "cosmetic",
							[]config.Value{config.VStr(cid)},
							map[string]config.Value{
								"cosmetic_id": config.VStr(cid),
								"kind":        config.VStr("SEASONAL"),
								"season":      config.VStr(cellAt(row, 0).Scalar()),
								"track":       config.VStr(track),
								"scope":       config.VStr("CHARACTER"),
							}, row[0].Line)
					}
				}
			}
			continue
		}
		ci := colIndex(bl)
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			cid := cellAt(row, 0).Scalar()
			track := "FREE"
			if strings.Contains(cid, ".paid") {
				track = "PAID"
			}
			fields := map[string]config.Value{
				"cosmetic_id": config.VStr(cid),
				"kind":        config.VStr("SEASONAL"),
				"track":       config.VStr(track),
				"scope":       config.VStr("CHARACTER"),
			}
			if i, ok := ci["Display (vi-VN)"]; ok {
				fields["display"] = config.VStr(norm.NFC.String(cellAt(row, i).Scalar()))
			}
			if i, ok := ci["Unlock"]; ok {
				fields["unlock"] = config.VStr(cellAt(row, i).Scalar())
			}
			c.Emit(f.Name, b.Raw, "cosmetic",
				[]config.Value{config.VStr(cid)}, fields, row[0].Line)
		}
	}
	// Seasonal Atlas T3 titles — 60: expand from seasonal atlas_page title_id
	ap := c.Defs.Get("atlas_page")
	count := 0
	if ap != nil {
		for _, ks := range ap.SortedKeys() {
			rec := ap.Records[config.KeyString(ks)]
			if rec.Fields["domain"].Str != "season" {
				continue
			}
			tid, ok := rec.Fields["title_id"]
			if !ok || tid.Str == "" {
				continue
			}
			count++
			c.Emit(f.Name, b.Raw, "cosmetic",
				[]config.Value{tid},
				map[string]config.Value{
					"cosmetic_id":   tid,
					"kind":          config.VStr("TITLE_SEASONAL"),
					"atlas_page_id": config.VStr(rec.Fields["atlas_page_id"].Str),
					"scope":         config.VStr("CHARACTER"),
				}, 0)
		}
	}
	if want, ok := bindingDecl(b, reDeclSeasonalNxN); ok && int64(count) != want {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, sec.Line,
			"seasonal atlas titles %d != declared %d", count, want)
	}
	c.consumed(f, b)
}

func cosmoIAP(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("IAP STORE COSMETICS — 13")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			pid := cellAt(row, 0).Scalar()
			cid := cellAt(row, 1).Scalar()
			c.Emit(f.Name, b.Raw, "product",
				[]config.Value{config.VStr(pid)},
				map[string]config.Value{
					"product_id":  config.VStr(pid),
					"cosmetic_id": config.VStr(cid),
					"display":     config.VStr(norm.NFC.String(cellAt(row, 2).Scalar())),
					"exclusivity": config.VStr(cellAt(row, 3).Scalar()),
				}, row[0].Line)
			c.Emit(f.Name, b.Raw, "cosmetic",
				[]config.Value{config.VStr(cid)},
				map[string]config.Value{
					"cosmetic_id": config.VStr(cid),
					"kind":        config.VStr("IAP"),
					"display":     config.VStr(norm.NFC.String(cellAt(row, 2).Scalar())),
					"scope":       config.VStr("ACCOUNT"),
				}, row[0].Line)
		}
	}
	c.EmitParam(f.Name, b.Raw, "iap_bundle",
		[]config.Value{config.VStr("product.cosmetic.bundle.nguoi_hung_lang_da")},
		map[string]config.Value{
			"contents": config.VSet(
				config.VStr("cosmetic.iap.appearance.co_tam_truyen"),
				config.VStr("cosmetic.iap.frame.thien_long"),
				config.VStr("cosmetic.iap.emote.bai_chao_lang"),
			),
		}, sec.Line)
	c.consumed(f, b)
}

func cosmoGuild(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("GUILD-SCOPED COSMETICS — 5")
	if sec == nil {
		return
	}
	for _, ch := range sec.Children {
		m := cosmoHeadRe.FindStringSubmatch(strings.TrimSpace(ch.Title))
		if m == nil {
			continue
		}
		cid := m[1]
		fields := map[string]config.Value{
			"cosmetic_id": config.VStr(cid),
			"kind":        config.VStr("GUILD"),
			"scope":       config.VStr("GUILD"),
		}
		for _, bl := range ch.Content {
			for j, l := range bl.Prose {
				l = strings.TrimSpace(l)
				if m2 := displayLineRe.FindStringSubmatch(l); m2 != nil {
					fields["display"] = config.VStr(norm.NFC.String(m2[1]))
				}
				if strings.HasPrefix(l, "Category:") {
					fields["category"] = config.VStr(strings.Trim(strings.TrimSpace(strings.TrimPrefix(l, "Category:")), "`"))
				}
				if strings.HasPrefix(l, "Unlock:") {
					fields["unlock"] = config.VStr(strings.TrimSpace(strings.TrimPrefix(l, "Unlock:")))
				}
				_ = j
			}
		}
		c.Emit(f.Name, b.Raw, "cosmetic",
			[]config.Value{config.VStr(cid)}, fields, ch.Line)
	}
	c.consumed(f, b)
}

func cosmoFencesToParams(c *Ctx, f *File, b *SourceBinding, secTitle, fam string) {
	sec := f.Root.SectionAt(secTitle)
	if sec == nil {
		c.consumed(f, b)
		return
	}
	ord := 0
	for _, bl := range sec.Content {
		var lines []config.Value
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l != "" {
				lines = append(lines, config.VStr(l))
			}
			_ = j
		}
		if len(lines) > 0 {
			ord++
			c.EmitParam(f.Name, b.Raw, fam,
				[]config.Value{config.VStr(secTitle), config.VInt(int64(ord))},
				map[string]config.Value{"lines": config.VSet(lines...)}, bl.Line)
		}
	}
	c.consumed(f, b)
}
