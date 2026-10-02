package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileBoss — boss_catalog.md driver (5 bindings): base stat formula,
// 8-row roster, numeric mechanic payloads, add rules, reward semantics.
func compileBoss(c *Ctx, f *File, r *Registry) {
	st := &bossState{formula: map[string]*config.ExprNode{}}
	c.Data["boss.state"] = st
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Base Stat Formula"):
			bossFormula(c, f, b, st)
		case strings.HasPrefix(path, "Roster"):
			st.wantRoster, st.hasRosterDecl = bindingDecl(b, reDeclRows)
			bossRoster(c, f, b, st)
		case strings.HasPrefix(path, "Numeric Mechanic Payloads"):
			bossMechanics(c, f, b)
		case strings.HasPrefix(path, "Boss Add Rules"):
			bossAddRules(c, f, b)
		case strings.HasPrefix(path, "Reward Semantics"):
			bossRewards(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"boss binding %q has no driver", b.Raw)
		}
	}
	bossVerify(c, f, st)
}

type bossState struct {
	formula map[string]*config.ExprNode
	roster  []*bossRow

	wantRoster    int64
	hasRosterDecl bool
}

type bossRow struct {
	id        string
	level     int64
	mode      string
	spaceID   string
	size      string
	element   string
	hp        int64
	attack    int64
	defense   int64
	baseExp   int64
	scaling   string
	dropTable string
	line      int
}

func bossFormula(c *Ctx, f *File, b *SourceBinding, st *bossState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				idx := strings.IndexByte(l, '=')
				if idx < 0 {
					continue
				}
				name := strings.TrimSpace(l[:idx])
				raw := strings.TrimSpace(l[idx+1:])
				ex, err := ParseExpr(raw)
				if err != nil {
					c.EmitParam(f.Name, b.Raw, "boss_stat_rule",
						[]config.Value{config.VStr(name)},
						map[string]config.Value{"text": config.VStr(raw)}, fb.Line+1)
					continue
				}
				st.formula[name] = ex
				c.EmitParam(f.Name, b.Raw, "boss_stat_rule",
					[]config.Value{config.VStr(name)},
					map[string]config.Value{"expr": config.VExpr(ex)}, fb.Line+1)
			}
		}
	}
	c.consumed(f, b)
}

func bossRoster(c *Ctx, f *File, b *SourceBinding, st *bossState) {
	types := inputTypes(b)
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "boss_id,Lv,mode,space_id,size_profile,element,base HP,ATTACK,DEFENSE,base_exp,scaling,drop_table_id"); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "boss roster table not found")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		vals := rowValues(c, f, tbl, ri, types)
		{
			br := &bossRow{
				id:        vals["boss_id"].Str,
				level:     vals["lv"].Int,
				mode:      vals["mode"].Str,
				spaceID:   vals["space_id"].Str,
				size:      vals["size_profile"].Str,
				element:   vals["element"].Str,
				hp:        vals["base_hp"].Int,
				attack:    vals["attack"].Int,
				defense:   vals["defense"].Int,
				baseExp:   vals["base_exp"].Int,
				scaling:   vals["scaling"].Str,
				dropTable: vals["drop_table_id"].Str,
				line:      row[0].Line,
			}
			st.roster = append(st.roster, br)
			c.Emit(f.Name, b.Raw, "boss",
				[]config.Value{config.VStr(br.id)}, vals, br.line)
			switch br.mode {
			case "INSTANCED":
				if br.scaling != "PARTY_DEFAULT" {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, br.line,
						"%s INSTANCED must use PARTY_DEFAULT, got %s", br.id, br.scaling)
				}
			case "PUBLIC":
				if br.scaling != "PUBLIC_DEFAULT" {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, br.line,
						"%s PUBLIC must use PUBLIC_DEFAULT, got %s", br.id, br.scaling)
				}
				if br.size != "WORLD_BOSS" {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, br.line,
						"%s PUBLIC must use WORLD_BOSS, got %s", br.id, br.size)
				}
			}
			// base HP must equal floor(10000 + 300L + 16L²)
			if ex := st.formula["MAX_HP"]; ex != nil {
				r, err := EvalExpr(ex, map[string]config.Rat{"L": {Num: br.level, Den: 1}})
				if err == nil {
					want, _ := r.Int()
					if want != br.hp {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, br.line,
							"%s base HP %d != formula %d", br.id, br.hp, want)
					}
				}
			}
		}
	}
	c.consumed(f, b)
}

var mechLineRe = regexp.MustCompile(`^([A-Z][A-Z0-9_]*)\s*->\s*(.*)$`)
var phaseHeadRe = regexp.MustCompile(`^Phase\s+(\d+)\s*(?:\(([^)]*)\))?`)

// bossMechanics — `## boss.<key>` sections, `Phase N:` fences with
// `NAME -> payload` mechanic grammar.
func bossMechanics(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, ch := range sec.Children {
			m := regexp.MustCompile("^`?(boss[.][a-z0-9_.]+)`?").FindStringSubmatch(ch.Title)
			if m == nil {
				continue
			}
			bossID := m[1]
			curPhase := ""
			threshold := ""
			var pendingHead *Block
			_ = pendingHead
			for _, bl := range ch.Content {
				if bl.Kind == BlockProse {
					for _, l := range bl.Prose {
						if m := phaseHeadRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
							curPhase = m[1]
							threshold = strings.TrimSpace(m[2])
						}
					}
					continue
				}
				if bl.Kind != BlockFence {
					continue
				}
				phaseN := curPhase
				thr := threshold
				if phaseN == "" && bossID == "boss.than_trung" {
					phaseN = "?"
				}
				mechName := ""
				fencePhase := phaseN
				if m := phaseHeadRe.FindStringSubmatch(strings.TrimSpace(bl.FLines[0])); m != nil {
					fencePhase = m[1]
					thr = strings.TrimSpace(m[2])
				}
				for j, l := range bl.FLines {
					l = strings.TrimRight(l, " ")
					if m := phaseHeadRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
						// `Phase N:` line inside fence
						continue
					}
					if strings.TrimSpace(l) == "" {
						continue
					}
					trim := strings.TrimSpace(l)
					if m := mechLineRe.FindStringSubmatch(trim); m != nil {
						mechName = m[1]
						payload := m[2]
						emitBossMech(c, f, b, bossID, fencePhase, thr, mechName, payload, bl.Line+1+j)
						continue
					}
					if mechName != "" && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) {
						// continuation line of previous mechanic
						emitBossMechCont(c, f, b, bossID, fencePhase, mechName, trim, bl.Line+1+j)
						continue
					}
					// Fragments: fence etc — treat whole block separately
					if strings.HasSuffix(trim, ":") {
						mechName = strings.TrimSuffix(trim, ":")
						emitBossMech(c, f, b, bossID, fencePhase, thr, mechName, "", bl.Line+1+j)
					}
				}
			}
		}
	}
	c.consumed(f, b)
}

func emitBossMech(c *Ctx, f *File, b *SourceBinding, bossID, phase, thr, name, payload string, line int) {
	fields := map[string]config.Value{
		"boss_id":     config.VStr(bossID),
		"mechanic_id": config.VStr(name),
		"phase":       config.VStr(phase),
		"parts":       config.VList(bossPayloadParts(payload)...),
	}
	if thr != "" {
		fields["phase_threshold"] = config.VStr(thr)
	}
	c.Emit(f.Name, b.Raw, "mechanic_payload",
		[]config.Value{config.VStr(bossID), config.VStr(name), config.VStr(phase)},
		fields, line)
}

func emitBossMechCont(c *Ctx, f *File, b *SourceBinding, bossID, phase, name, cont string, line int) {
	c.Emit(f.Name, b.Raw, "mechanic_payload_part",
		[]config.Value{config.VStr(bossID), config.VStr(name), config.VStr(phase), config.VInt(int64(line))},
		map[string]config.Value{
			"boss_id":     config.VStr(bossID),
			"mechanic_id": config.VStr(name),
			"phase":       config.VStr(phase),
			"text":        config.VStr(cont),
		}, line)
}

var bossCoeffRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*ATTACK\s*([A-Za-z]*)`)
var bossTimeRe = regexp.MustCompile(`(tell|startup|duration|for|over|gap|after|within|cooldown)\s*(?:>=|<=|=)?\s*([0-9]+(?:\.[0-9]+)?)s`)
var bossPullRe = regexp.MustCompile(`PULL\s*([0-9]+(?:\.[0-9]+)?)m`)
var bossStatusRe = regexp.MustCompile(`\b(WEAKEN|ROOT|STUN|SLOW|BURN|VULNERABLE|UNSTAGGERABLE|MA_AM)\b`)

func bossPayloadParts(payload string) []config.Value {
	var out []config.Value
	for _, m := range bossCoeffRe.FindAllStringSubmatch(payload, -1) {
		d, _ := parseDecimal(m[1])
		el := m[2]
		if el == "" {
			el = "physical"
		}
		out = append(out, config.VRec(map[string]config.Value{
			"type":        config.VStr("attack_coeff"),
			"coefficient": mustRat(d),
			"element":     config.VStr(strings.ToUpper(el)),
		}))
	}
	for _, m := range bossPullRe.FindAllStringSubmatch(payload, -1) {
		d, _ := parseDecimal(m[1])
		out = append(out, config.VRec(map[string]config.Value{
			"type": config.VStr("pull"), "meters": mustRat(d),
		}))
	}
	for _, m := range bossTimeRe.FindAllStringSubmatch(payload, -1) {
		d, _ := parseDecimal(m[2])
		out = append(out, config.VRec(map[string]config.Value{
			"type": config.VStr("time_" + m[1]), "seconds": mustRat(d),
		}))
	}
	for _, m := range bossStatusRe.FindAllStringSubmatch(payload, -1) {
		out = append(out, config.VRec(map[string]config.Value{
			"type": config.VStr("status"), "status": config.VStr(m[1]),
		}))
	}
	out = append(out, config.VRec(map[string]config.Value{
		"type": config.VStr("text"), "text": config.VStr(payload),
	}))
	return out
}

func bossAddRules(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		var rules []config.Value
		for _, bl := range sec.Content {
			for _, l := range bl.Prose {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "-") {
					rules = append(rules, config.VStr(strings.TrimSpace(l[1:])))
				}
			}
		}
		c.EmitParam(f.Name, b.Raw, "boss_add_rule",
			[]config.Value{config.VStr("defaults")},
			map[string]config.Value{
				"rules":            config.VList(rules...),
				"boss_slots":       config.VInt(7),
				"grant_exp":        config.VBool(false),
				"soul_drop_table":  config.VBool(false),
				"despawn_on_reset": config.VBool(true),
				"field_population": config.VBool(false),
			}, sec.Line)
	}
	c.consumed(f, b)
}

func bossRewards(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		var rules []config.Value
		for _, bl := range sec.Content {
			for _, l := range bl.Prose {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "-") {
					rules = append(rules, config.VStr(strings.TrimSpace(l[1:])))
				}
			}
		}
		if len(rules) > 0 {
			c.EmitParam(f.Name, b.Raw, "boss_reward_rule",
				[]config.Value{config.VStr("semantics")},
				map[string]config.Value{"rules": config.VList(rules...)}, sec.Line)
		}
		for _, ch := range sec.Children {
			m := regexp.MustCompile("^`?(boss[.][a-z0-9_.]+)`?").FindStringSubmatch(ch.Title)
			if m == nil {
				continue
			}
			bossID := m[1]
			var key, exp string
			for _, bl := range ch.Content {
				if bl.Kind != BlockFence {
					continue
				}
				for _, l := range bl.FLines {
					l = strings.TrimSpace(l)
					switch {
					case strings.HasPrefix(l, "reward."):
						key = l
					case strings.HasSuffix(l, " character EXP"):
						exp = strings.TrimSuffix(l, " character EXP")
					}
				}
			}
			expV := config.VNull()
			if exp != "" {
				if v, err := (TypeSpec{Name: "grouped_int"}).ParseValue(exp); err == nil {
					expV = v
				} else {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, ch.Line, "%v", err)
				}
			}
			c.Emit(f.Name, b.Raw, "first_clear_reward",
				[]config.Value{config.VStr(bossID)},
				map[string]config.Value{
					"boss_id":    config.VStr(bossID),
					"reward_key": config.VStr(key),
					"exp":        expV,
				}, ch.Line)
		}
	}
	c.consumed(f, b)
}

// bossVerify — exactly 8 roster rows.
func bossVerify(c *Ctx, f *File, st *bossState) {
	if st.hasRosterDecl && int64(len(st.roster)) != st.wantRoster {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"boss roster = %d, declared %d", len(st.roster), st.wantRoster)
	}
}
