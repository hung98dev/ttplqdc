package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileEncounter — encounter_catalog.md driver (11 bindings): budget
// (counted), progression route, per-act fields/enemies/dungeons/bosses,
// finale phases, canonical bosses, endgame reuse, guardrails, culture gate.
func compileEncounter(c *Ctx, f *File, r *Registry) {
	st := &encState{}
	c.Data["encounter.state"] = st
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Launch Budget"):
			st.want, st.hasWant = bindingDecl4(b, reDeclEncounter)
			st.budget = b // emitted after counting
		case strings.HasPrefix(path, "Progression Route"):
			encProgression(c, f, b, st)
		case strings.Contains(path, "ACT"):
			encActs(c, f, b, st)
		case strings.Contains(path, "Final Boss"):
			encFinale(c, f, b, st)
		case strings.HasPrefix(path, "Canonical Eight Major Bosses"):
			encCanonicalBosses(c, f, b, st)
		case strings.HasPrefix(path, "Lv60 Endgame Reuse"):
			encEndgame(c, f, b)
		case strings.HasPrefix(path, "Readability Guardrails"):
			encGuardrails(c, f, b)
		case strings.HasPrefix(path, "Cultural Pillars"):
			encCulture(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"encounter binding %q has no driver", b.Raw)
		}
	}
	// Launch Budget: emit counted domain sizes; declared 6/18/5/8 checked.
	c.EmitParam(f.Name, "Launch Budget", "encounter_budget",
		[]config.Value{config.VStr("launch")},
		map[string]config.Value{
			"regions":      config.VInt(int64(st.regions)),
			"field_maps":   config.VInt(int64(len(st.fieldMaps))),
			"dungeons":     config.VInt(int64(len(st.dungeons))),
			"major_bosses": config.VInt(int64(len(st.bosses))),
		}, 0)
	if st.budget != nil {
		c.consumed(f, st.budget)
	}
	if st.hasWant && (int64(st.regions) != st.want[0] ||
		int64(len(st.fieldMaps)) != st.want[1] ||
		int64(len(st.dungeons)) != st.want[2] ||
		int64(len(st.bosses)) != st.want[3]) {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"encounter budget regions=%d fields=%d dungeons=%d bosses=%d, declared %d/%d/%d/%d",
			st.regions, len(st.fieldMaps), len(st.dungeons), len(st.bosses),
			st.want[0], st.want[1], st.want[2], st.want[3])
	}
}

type encState struct {
	budget    *SourceBinding
	regions   int
	fieldMaps []string
	dungeons  []string
	bosses    []string
	anchors   []string

	want    [4]int64
	hasWant bool
}

var levelBandRe = regexp.MustCompile(`^(\d+)-(\d+)$`)
var fieldLineRe = regexp.MustCompile(`^(map\.[a-z0-9_.]+)\s+Lv(\d+)-(\d+)\s*—\s*(.+)$`)
var enemyLineRe = regexp.MustCompile(`^(monster\.[a-z0-9_.]+)\s+(NORMAL|ELITE)\s*—\s*(.+)$`)
var canonBossRe = regexp.MustCompile(`^(\d+)\s+(boss\.[a-z0-9_.]+)\s+(.+)$`)

func encProgression(c *Ctx, f *File, b *SourceBinding, st *encState) {
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "Act,Levels,zone_id,Display region,Folk identity,Dungeon"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				lv := cellAt(row, 1).Scalar()
				m := levelBandRe.FindStringSubmatch(lv)
				if m == nil {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
						"Levels %q want a-b", lv)
					continue
				}
				lo, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				hi, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
				st.regions++
				c.Emit(f.Name, b.Raw, "progression_region",
					[]config.Value{config.VStr(cellAt(row, 2).Scalar())},
					map[string]config.Value{
						"act":            config.VStr(cellAt(row, 0).Scalar()),
						"level_lo":       config.VInt(lo.Int),
						"level_hi":       config.VInt(hi.Int),
						"zone_id":        config.VStr(cellAt(row, 2).Scalar()),
						"display_region": config.VStr(cellAt(row, 3).Scalar()),
						"folk_identity":  config.VStr(cellAt(row, 4).Scalar()),
						"dungeon":        config.VStr(cellAt(row, 5).Scalar()),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

// actSections returns the six `# ACT N — <name>` top-level sections.
func actSections(f *File) []*Section {
	var out []*Section
	for _, ch := range f.Root.Children {
		if strings.HasPrefix(ch.Title, "ACT ") {
			out = append(out, ch)
		}
	}
	return out
}

// encActs — the ACT bindings carry template paths (`# ACT I..VI` /
// `## Fields` etc.); walk the ACT sections directly and dispatch each
// level-2 child by keyword of this binding's declared subsection.
func encActs(c *Ctx, f *File, b *SourceBinding, st *encState) {
	var want string
	raw := b.Raw
	switch {
	case strings.Contains(raw, "Fields"):
		want = "fields"
	case strings.Contains(raw, "Enemy Family"):
		want = "enemies"
	case strings.Contains(raw, "Dungeon —"):
		want = "dungeon"
	case strings.Contains(raw, "Boss —"):
		want = "boss"
	}
	for _, sec := range actSections(f) {
		act := actFromTitle(sec.Title)
		for _, ch := range sec.Children {
			switch want {
			case "fields":
				if strings.HasPrefix(ch.Title, "Fields") {
					encFields(c, f, b, ch, act, st)
				}
			case "enemies":
				if strings.HasPrefix(ch.Title, "Enemy Family") {
					encEnemies(c, f, b, ch, act)
				}
			case "dungeon":
				if strings.HasPrefix(ch.Title, "Dungeon") {
					encDungeon(c, f, b, ch, act, st)
				}
			case "boss":
				if strings.HasPrefix(ch.Title, "Boss") || strings.HasPrefix(ch.Title, "Public Boss") {
					encBoss(c, f, b, ch, act, st)
				}
			}
		}
	}
	c.consumed(f, b)
}

var anchorRe = regexp.MustCompile("`?(map\\.[a-z0-9_.]+)`?")

func encFields(c *Ctx, f *File, b *SourceBinding, sec *Section, act string, st *encState) {
	for _, fb := range allFences(sec, "text") {
		for j, l := range fb.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			m := fieldLineRe.FindStringSubmatch(l)
			if m == nil {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, fb.Line+1+j,
					"field line %q", l)
				continue
			}
			lo, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
			hi, _ := (TypeSpec{Name: "int"}).ParseValue(m[3])
			fields := map[string]config.Value{
				"map_id":     config.VStr(m[1]),
				"level_lo":   config.VInt(lo.Int),
				"level_hi":   config.VInt(hi.Int),
				"zone":       config.VStr(zoneOf(m[1])),
				"act":        config.VStr(act),
				"descriptor": config.VStr(m[4]),
			}
			if strings.Contains(l, "has_water") {
				fields["has_water"] = config.VBool(true)
			}
			if strings.Contains(l, "fishing_spot") {
				fields["fishing_spot"] = config.VBool(true)
			}
			st.fieldMaps = append(st.fieldMaps, m[1])
			c.Emit(f.Name, b.Raw, "field_map",
				[]config.Value{config.VStr(m[1])}, fields, fb.Line+1+j)
		}
	}
	// safe-anchor line: `Safe/social anchor: `map.<zone>.<key>``
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			if strings.Contains(l, "anchor") {
				if m := anchorRe.FindStringSubmatch(l); m != nil {
					st.anchors = append(st.anchors, m[1])
					c.Emit(f.Name, b.Raw, "safe_anchor",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{
							"map_id": config.VStr(m[1]),
							"zone":   config.VStr(zoneOf(m[1])),
							"act":    config.VStr(act),
						}, bl.Line)
				}
			}
		}
	}
}

func zoneOf(mapID string) string {
	p := strings.Split(mapID, ".")
	if len(p) >= 2 {
		return "zone." + p[1]
	}
	return ""
}

func encEnemies(c *Ctx, f *File, b *SourceBinding, sec *Section, act string) {
	for _, fb := range allFences(sec, "text") {
		for j, l := range fb.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			m := enemyLineRe.FindStringSubmatch(l)
			if m == nil {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, fb.Line+1+j,
					"enemy line %q", l)
				continue
			}
			fields := map[string]config.Value{
				"monster_id": config.VStr(m[1]),
				"rank":       config.VStr(m[2]),
				"zone":       config.VStr(zoneOf(m[1])),
				"act":        config.VStr(act),
				"descriptor": config.VStr(m[3]),
				"season_0":   config.VBool(strings.Contains(m[3], "season-0")),
			}
			c.Emit(f.Name, b.Raw, "encounter_monster",
				[]config.Value{config.VStr(m[1])}, fields, fb.Line+1+j)
		}
	}
}

func encDungeon(c *Ctx, f *File, b *SourceBinding, sec *Section, act string, st *encState) {
	var dID, target string
	var stages []string
	name := strings.TrimPrefix(sec.Title, "Dungeon — ")
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if m := anchorRe.FindStringSubmatch(strings.ReplaceAll(l, "dungeon.", "map.PROXY.")); m != nil {
				_ = m
			}
			if i := strings.Index(l, "dungeon."); i >= 0 {
				if dID == "" {
					j := i + len("dungeon.")
					end := j
					for end < len(l) && (l[end] >= 'a' && l[end] <= 'z' || l[end] >= '0' && l[end] <= '9' || l[end] == '_') {
						end++
					}
					dID = l[i:end]
				}
			}
			if m := regexp.MustCompile("target\\s*`?([0-9]+)-([0-9]+)\\s*min").FindStringSubmatch(l); m != nil {
				target = m[1] + "-" + m[2]
			}
			if m := regexp.MustCompile(`^([0-9]+)\.\s+(.+)$`).FindStringSubmatch(l); m != nil {
				stages = append(stages, strings.TrimSpace(m[2]))
			}
		}
	}
	if dID == "" {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
			"dungeon section %q has no dungeon.<id>", sec.Title)
		return
	}
	st.dungeons = append(st.dungeons, dID)
	var lo, hi config.Value
	if target != "" {
		parts := strings.Split(target, "-")
		loV, _ := (TypeSpec{Name: "int"}).ParseValue(parts[0])
		hiV, _ := (TypeSpec{Name: "int"}).ParseValue(parts[1])
		lo, hi = loV, hiV
	}
	sv := make([]config.Value, len(stages))
	for i, s := range stages {
		sv[i] = config.VStr(s)
	}
	c.Emit(f.Name, b.Raw, "encounter_dungeon",
		[]config.Value{config.VStr(dID)},
		map[string]config.Value{
			"dungeon_id":     config.VStr(dID),
			"name":           config.VStr(name),
			"act":            config.VStr(act),
			"target_minutes": config.VRec(map[string]config.Value{"lo": lo, "hi": hi}),
			"stages":         config.VList(sv...),
		}, sec.Line)
}

var mechLineRe2 = regexp.MustCompile(`^([A-Z][A-Z0-9_]*)\s*->\s*(.+)$`)

func encBoss(c *Ctx, f *File, b *SourceBinding, sec *Section, act string, st *encState) {
	kind := "dungeon"
	if strings.HasPrefix(sec.Title, "Public Boss") {
		kind = "public"
	}
	var bossID, tests string
	var recLv int64 = -1
	var mechs []config.Value
	for _, bl := range sec.Content {
		for _, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if i := strings.Index(l, "boss."); i >= 0 && bossID == "" {
				j := i + len("boss.")
				end := j
				for end < len(l) && (l[end] >= 'a' && l[end] <= 'z' || l[end] >= '0' && l[end] <= '9' || l[end] == '_') {
					end++
				}
				bossID = l[i:end]
			}
			if m := regexp.MustCompile(`Lv\s*([0-9]+)`).FindStringSubmatch(l); m != nil && recLv < 0 && strings.Contains(l, "recommended") {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				recLv = v.Int
			}
			if strings.HasPrefix(l, "Tests:") {
				tests = strings.TrimSpace(strings.TrimPrefix(l, "Tests:"))
			}
		}
		if bl.Kind != BlockFence {
			continue
		}
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := mechLineRe2.FindStringSubmatch(l); m != nil {
				mechs = append(mechs, config.VRec(map[string]config.Value{
					"mechanic_id": config.VStr(m[1]),
					"description": config.VStr(m[2]),
				}))
			} else if m := regexp.MustCompile(`^([0-9]+)%\s*HP\s*->\s*(.+)$`).FindStringSubmatch(l); m != nil {
				mechs = append(mechs, config.VRec(map[string]config.Value{
					"mechanic_id": config.VStr("PHASE_THRESHOLD_" + m[1]),
					"description": config.VStr(m[2]),
				}))
			} else if l != "" {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, bl.Line+1+j,
					"boss mechanic line %q", l)
			}
		}
	}
	if bossID == "" {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, sec.Line,
			"boss section %q has no boss.<id>", sec.Title)
		return
	}
	st.bosses = append(st.bosses, bossID)
	fields := map[string]config.Value{
		"boss_id":   config.VStr(bossID),
		"kind":      config.VStr(kind),
		"act":       config.VStr(act),
		"mechanics": config.VList(mechs...),
	}
	if tests != "" {
		fields["tests"] = config.VStr(tests)
	}
	if recLv >= 0 {
		fields["recommended_level"] = config.VInt(recLv)
	}
	c.Emit(f.Name, b.Raw, "boss_encounter",
		[]config.Value{config.VStr(bossID)}, fields, sec.Line)
}

var finalePhaseRe = regexp.MustCompile("Phase\\s*(\\d+)\\s*—\\s*(.+?)\\s*\\(`?([0-9]+-[0-9]+%)`?\\)")
var motifsRe = regexp.MustCompile(`Motifs:\s*([A-Z_]+)\s*∥\s*([A-Z_]+)`)
var coeffRe = regexp.MustCompile(`([0-9]+\.[0-9]+)\s*ATTACK`)

func encFinale(c *Ctx, f *File, b *SourceBinding, st *encState) {
	for _, sec := range bindingSections(c, f, b) {
		var bossID string
		var recLv int64 = -1
		for _, bl := range sec.Content {
			for _, l := range bl.Prose {
				if i := strings.Index(l, "boss.than_trung"); i >= 0 && bossID == "" {
					bossID = "boss.than_trung"
				}
				if m := regexp.MustCompile(`Lv\s*([0-9]+)`).FindStringSubmatch(l); m != nil && recLv < 0 {
					v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
					recLv = v.Int
				}
			}
		}
		st.bosses = append(st.bosses, "boss.than_trung")
		for _, ph := range sec.Children {
			pm := finalePhaseRe.FindStringSubmatch(ph.Title)
			if pm == nil {
				continue
			}
			phaseN := pm[1]
			var motifs []string
			var patterns []config.Value
			for _, bl := range ph.Content {
				if bl.Kind == BlockFence {
					for _, l := range bl.FLines {
						if m := mechLineRe2.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
							motifs = append(motifs, m[1])
						}
					}
					continue
				}
				for _, l := range bl.Prose {
					_ = l
				}
			}
			for _, g := range ph.Children {
				var m1, m2 string
				var body []string
				var coeffs []config.Value
				for _, bl := range g.Content {
					for _, l := range bl.Prose {
						l = strings.TrimSpace(l)
						if m := motifsRe.FindStringSubmatch(l); m != nil {
							m1, m2 = m[1], m[2]
							continue
						}
						body = append(body, l)
						for _, cm := range coeffRe.FindAllStringSubmatch(l, -1) {
							d, _ := parseDecimal(cm[1])
							coeffs = append(coeffs, mustRat(d))
						}
					}
				}
				patterns = append(patterns, config.VRec(map[string]config.Value{
					"name":         config.VStr(g.Title),
					"motif_a":      config.VStr(m1),
					"motif_b":      config.VStr(m2),
					"coefficients": config.VList(coeffs...),
					"description":  config.VStr(strings.Join(body, "\n")),
				}))
			}
			mv := make([]config.Value, len(motifs))
			for i, m := range motifs {
				mv[i] = config.VStr(m)
			}
			c.Emit(f.Name, b.Raw, "finale_phase",
				[]config.Value{config.VStr("boss.than_trung"), config.VStr(phaseN)},
				map[string]config.Value{
					"boss_id":  config.VStr("boss.than_trung"),
					"phase":    config.VStr(phaseN),
					"name":     config.VStr(pm[2]),
					"hp_range": config.VStr(pm[3]),
					"motifs":   config.VList(mv...),
					"patterns": config.VList(patterns...),
				}, ph.Line)
		}
		c.Emit(f.Name, b.Raw, "boss_encounter",
			[]config.Value{config.VStr("boss.than_trung")},
			map[string]config.Value{
				"boss_id":           config.VStr("boss.than_trung"),
				"kind":              config.VStr("finale"),
				"act":               config.VStr("VI"),
				"recommended_level": config.VInt(recLv),
			}, sec.Line)
	}
	c.consumed(f, b)
}

func encCanonicalBosses(c *Ctx, f *File, b *SourceBinding, st *encState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := canonBossRe.FindStringSubmatch(l); m != nil {
					ord, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
					c.Emit(f.Name, b.Raw, "canonical_boss",
						[]config.Value{config.VStr(m[2])},
						map[string]config.Value{
							"boss_id":   config.VStr(m[2]),
							"order":     config.VInt(ord.Int),
							"placement": config.VStr(m[3]),
						}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

func encEndgame(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if i := strings.IndexByte(l, '='); i > 0 {
					name := strings.TrimSpace(l[:i])
					raw := strings.TrimSpace(l[i+1:])
					if ex, err := ParseExpr(raw); err == nil {
						c.EmitParam(f.Name, b.Raw, "endgame_rule",
							[]config.Value{config.VStr(name)},
							map[string]config.Value{"expr": config.VExpr(ex)}, fb.Line+1)
						continue
					}
				}
				c.EmitParam(f.Name, b.Raw, "endgame_rule",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"rule": config.VStr(l)}, fb.Line+1)
			}
		}
		if tbl := findTable(sec, "highlight_index,featured_dungeon_id"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				idx, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 0).Scalar())
				c.EmitParam(f.Name, b.Raw, "weekly_highlight",
					[]config.Value{config.VInt(idx.Int)},
					map[string]config.Value{
						"highlight_index": config.VInt(idx.Int),
						"dungeon_id":      config.VStr(cellAt(row, 1).Scalar()),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func encGuardrails(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				c.EmitParam(f.Name, b.Raw, "readability_guardrail",
					[]config.Value{config.VInt(int64(j))},
					map[string]config.Value{"rule": config.VStr(l)}, fb.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func encCulture(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		var rules []config.Value
		for _, bl := range sec.Content {
			if bl.Kind == BlockFence {
				for _, l := range bl.FLines {
					l = strings.TrimSpace(l)
					if l != "" {
						rules = append(rules, config.VStr(l))
					}
				}
			}
		}
		c.EmitParam(f.Name, b.Raw, "cultural_rule",
			[]config.Value{config.VStr(sec.Title)},
			map[string]config.Value{
				"section": config.VStr(sec.Title),
				"rules":   config.VList(rules...),
			}, sec.Line)
	}
	c.consumed(f, b)
}
