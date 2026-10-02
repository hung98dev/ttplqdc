package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileMonster — monster_catalog.md driver (11 bindings): stat formulas,
// movement/combat profiles, mechanic registry, size resolution, launch
// roster (46 NORMAL + 12 ELITE) + 6 season-0 rows, typed attack expansion,
// mechanic ops, roster overrides, static expansion requirements.
func compileMonster(c *Ctx, f *File, r *Registry) {
	st := &monsterState{
		attackRows:   map[string][]attackProfileRow{},
		mechanicOps:  map[string][]config.Value{},
		overrides:    map[string][]config.Value{},
		smallRoster:  map[string]bool{},
		moveProfiles: map[string]map[string]config.Value{},
	}
	c.Data["monster.state"] = st

	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Deterministic Stat Expansion"):
			monStatRules(c, f, b, st)
		case strings.HasPrefix(path, "Shared Movement Profiles"):
			monMovement(c, f, b, st)
		case strings.HasPrefix(path, "Shared Combat Profiles"):
			monCombatProfiles(c, f, b, st)
		case strings.HasPrefix(path, "Named Mechanic Extensions"):
			emitTable(c, f, b, "mechanic_id,type,payload", "mechanic",
				[]string{"mechanic_id"}, nil)
		case strings.HasPrefix(path, "Canonical Entity Size Resolution"):
			monSizeResolution(c, f, b, st)
		case strings.HasPrefix(path, "Launch Roster"):
			st.wantN, st.wantE, st.hasLaunchDecl = bindingDecl2(b, reDeclNE)
			monRoster(c, f, b, st, true)
		case strings.HasPrefix(path, "Season 0"):
			st.wantS0, st.hasS0Decl = bindingDecl(b, reDeclExactlyNorm)
			monRoster(c, f, b, st, false)
		case strings.HasPrefix(path, "Typed Attack Expansion"):
			monAttackProfiles(c, f, b, st)
		case strings.HasPrefix(path, "Typed Mechanic Operations"):
			monMechanicOps(c, f, b, st)
		case strings.HasPrefix(path, "Typed Roster Overrides"):
			monOverrides(c, f, b, st)
		case strings.HasPrefix(path, "Static Expansion Requirements"):
			monRequiredFields(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"monster binding %q has no driver", b.Raw)
		}
	}
	monEmit(c, f, st)
}

type monsterRow struct {
	id        string
	rank      string
	level     int64
	element   string
	movement  string
	combat    string
	special   string
	mechanic  string
	dropTable string
	baseExp   int64
	act       string
	zone      string
	inLaunch  bool
	line      int
}

type attackProfileRow struct {
	profile     string
	suffix      string
	shape       string
	lengthMm    int64
	widthMm     int64
	radiusMm    int64
	startupMs   int64
	activeMs    int64
	recoveryMs  int64
	cooldownMs  int64
	normalCoeff config.Rat
	eliteCoeff  config.Rat
	hitCap      int64
	line        int
}

type monsterState struct {
	roster       []*monsterRow
	stat         map[string]*config.ExprNode // normal_hp/normal_attack/normal_defense
	eliteExpr    map[string]*config.ExprNode // MAX_HP/ATTACK/DEFENSE/DEFENSE multiplier exprs
	eliteCDM     *config.ExprNode            // ELITE control_duration_multiplier
	sizeRule     string
	attackRows   map[string][]attackProfileRow
	mechanicOps  map[string][]config.Value // key mechanic_id+"|"+suffix
	overrides    map[string][]config.Value // key monster_key
	smallRoster  map[string]bool
	moveProfiles map[string]map[string]config.Value
	combatProse  map[string]string

	wantN         int64
	wantE         int64
	hasLaunchDecl bool
	wantS0        int64
	hasS0Decl     bool
}

var rosterSig = "monster_id,rank,Lv,element,movement,combat,special,base_exp,drop_table_id"

// monStatRules — both fences: normal exprs + ELITE multipliers, stored for
// expansion and emitted as `stat_rule` params.
func monStatRules(c *Ctx, f *File, b *SourceBinding, st *monsterState) {
	st.stat = map[string]*config.ExprNode{}
	st.eliteExpr = map[string]*config.ExprNode{}
	var notes []string
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				idx := strings.IndexByte(l, '=')
				if idx < 0 {
					notes = append(notes, l)
					continue
				}
				name := strings.TrimSpace(l[:idx])
				raw := strings.TrimSpace(l[idx+1:])
				ex, err := ParseExpr(raw)
				if err != nil {
					// prose assignment (e.g. base_exp authored note): keep as
					// text param, not an error — the fence is declared input.
					c.EmitParam(f.Name, b.Raw, "stat_rule",
						[]config.Value{config.VStr(name)},
						map[string]config.Value{"text": config.VStr(raw)},
						fb.Line+1)
					continue
				}
				st.stat[name] = ex
				if name == "control_duration_multiplier" {
					st.eliteCDM = ex
				}
				c.EmitParam(f.Name, b.Raw, "stat_rule",
					[]config.Value{config.VStr(name)},
					map[string]config.Value{
						"expr": config.VExpr(ex),
					}, fb.Line+1)
			}
		}
	}
	if len(notes) > 0 {
		nv := make([]config.Value, len(notes))
		for i, n := range notes {
			nv[i] = config.VStr(n)
		}
		c.EmitParam(f.Name, b.Raw, "stat_rule_note",
			[]config.Value{config.VStr("fence_commentary")},
			map[string]config.Value{"lines": config.VList(nv...)}, b.Line)
	}
	c.consumed(f, b)
}

var profileFieldRe = regexp.MustCompile(`^\s*([a-z_]+)\s*=\s*(.+)$`)

// monMovement — `Shared Movement Profiles` fence: `NAME:` blocks with
// indented field=value lines.
func monMovement(c *Ctx, f *File, b *SourceBinding, st *monsterState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			cur := ""
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if strings.HasSuffix(l, ":") {
					cur = strings.TrimSuffix(l, ":")
					if st.moveProfiles[cur] == nil {
						st.moveProfiles[cur] = map[string]config.Value{}
					}
					continue
				}
				if cur == "" {
					continue
				}
				for _, kv := range strings.Split(l, ",") {
					m := profileFieldRe.FindStringSubmatch(kv)
					if m == nil {
						continue // prose line ("authored horizontal/vertical ...")
					}
					name, raw := m[1], strings.TrimSpace(m[2])
					ts := TypeSpec{Name: "string"}
					if strings.HasSuffix(raw, "m") {
						ts = TypeSpec{Name: "int", IntSuffix: "m", IntScale: 1000}
					} else if raw == "true" || raw == "false" {
						ts = TypeSpec{Name: "bool"}
					}
					v, err := ts.ParseValue(raw)
					if err != nil {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, fb.Line+1+j,
							"movement %s.%s: %v", cur, name, err)
						continue
					}
					st.moveProfiles[cur][name] = v
				}
			}
		}
	}
	for name, fields := range st.moveProfiles {
		fields["profile"] = config.VStr(name)
		c.Emit(f.Name, b.Raw, "movement_profile",
			[]config.Value{config.VStr(name)}, fields, 0)
	}
	c.consumed(f, b)
}

// monCombatProfiles — each `## PROFILE` heading + its fence is declared
// intent; emit prose record (the typed expansion table is authoritative).
func monCombatProfiles(c *Ctx, f *File, b *SourceBinding, st *monsterState) {
	st.combatProse = map[string]string{}
	for _, sec := range bindingSections(c, f, b) {
		for _, ch := range sec.Children {
			var prose []string
			for _, bl := range ch.Content {
				prose = append(prose, bl.FLines...)
				prose = append(prose, bl.Prose...)
			}
			name := strings.Trim(ch.Title, "` ")
			st.combatProse[name] = strings.Join(prose, "\n")
			c.Emit(f.Name, b.Raw, "combat_profile",
				[]config.Value{config.VStr(name)},
				map[string]config.Value{
					"profile": config.VStr(name),
					"prose":   config.VStr(strings.Join(prose, "\n")),
				}, ch.Line)
		}
	}
	c.consumed(f, b)
}

// monSizeResolution — rank dispatch fence + SMALL_ROSTER list.
func monSizeResolution(c *Ctx, f *File, b *SourceBinding, st *monsterState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			var ids []string
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "monster.") {
					ids = append(ids, l)
				}
				if strings.Contains(l, "->") {
					st.sizeRule = l
				}
			}
			if len(ids) > 0 {
				for _, id := range ids {
					st.smallRoster[id] = true
				}
				vals := make([]config.Value, len(ids))
				for i, id := range ids {
					vals[i] = config.VStr(id)
				}
				c.EmitParam(f.Name, b.Raw, "small_roster",
					[]config.Value{config.VStr("members")},
					map[string]config.Value{"members": config.VList(vals...)},
					fb.Line+1)
			}
		}
	}
	c.EmitParam(f.Name, b.Raw, "size_resolution",
		[]config.Value{config.VStr("rank_dispatch")},
		map[string]config.Value{
			"rule": config.VStr("rank = ELITE -> MONSTER_ELITE; rank = NORMAL and monster_id in SMALL_ROSTER -> MONSTER_SMALL; all other rank = NORMAL -> MONSTER_MEDIUM"),
		}, b.Line)
	c.consumed(f, b)
}

// monRoster — launch + season-0 roster tables under `## Act N` headings.
func monRoster(c *Ctx, f *File, b *SourceBinding, st *monsterState, launch bool) {
	band := map[string][2]int64{
		"I": {1, 10}, "II": {11, 20}, "III": {21, 30},
		"IV": {31, 40}, "V": {41, 50}, "VI": {51, 60},
	}
	for _, sec := range bindingSections(c, f, b) {
		for _, ch := range sec.Children {
			act := actFromTitle(ch.Title)
			zone := zoneFromTitle(ch.Title)
			for _, tbl := range allTables(ch) {
				if headerSig(tbl) != rosterSig {
					continue
				}
				for ri := range tbl.Cells {
					row := tbl.Cells[ri]
					m := parseMonsterRow(c, f, b, row, launch, act, zone)
					if m == nil {
						continue
					}
					if bb, ok := band[m.act]; ok && (m.level < bb[0] || m.level > bb[1]) {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, m.line,
							"%s Lv%d outside act %s band %d-%d", m.id, m.level, m.act, bb[0], bb[1])
					}
					st.roster = append(st.roster, m)
				}
			}
		}
		// season-0 section has no Act children — table sits at top level
		for _, tbl := range allTables(sec) {
			if headerSig(tbl) != rosterSig {
				continue
			}
			if len(sec.Children) > 0 {
				continue // handled per-child above
			}
			for ri := range tbl.Cells {
				m := parseMonsterRow(c, f, b, tbl.Cells[ri], launch, "I", "")
				if m != nil {
					st.roster = append(st.roster, m)
				}
			}
		}
	}
	c.consumed(f, b)
}

var actTitleRe = regexp.MustCompile(`Act\s+([IVX]+)\s*—\s*(.+)`)

func actFromTitle(t string) string {
	if m := actTitleRe.FindStringSubmatch(t); m != nil {
		return m[1]
	}
	return ""
}

func zoneFromTitle(t string) string {
	if m := actTitleRe.FindStringSubmatch(t); m != nil {
		return strings.TrimSpace(m[2])
	}
	return ""
}

var mechanicTokenRe = regexp.MustCompile(`^([A-Z][A-Z_]+):`)

func parseMonsterRow(c *Ctx, f *File, b *SourceBinding, row []Cell, launch bool, act, zone string) *monsterRow {
	get := func(i int) string { return cellAt(row, i).Scalar() }
	id := get(0)
	if !strings.HasPrefix(id, "monster.") {
		c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
			"roster id %q not monster.*", id)
		return nil
	}
	rank := get(1)
	lv, err := (TypeSpec{Name: "int"}).ParseValue(get(2))
	if err != nil {
		c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line, "%v", err)
		return nil
	}
	exp, err := (TypeSpec{Name: "int"}).ParseValue(get(7))
	if err != nil {
		c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line, "%v", err)
		return nil
	}
	special := get(6)
	mech := ""
	if m := mechanicTokenRe.FindStringSubmatch(special); m != nil {
		mech = m[1]
	}
	return &monsterRow{
		id: id, rank: rank, level: lv.Int,
		element: get(3), movement: get(4), combat: get(5),
		special: special, mechanic: mech, dropTable: get(8),
		baseExp: exp.Int, act: act, zone: zone, inLaunch: launch,
		line: row[0].Line,
	}
}

// monAttackProfiles — Typed Attack Expansion table → (profile,suffix) rows.
func monAttackProfiles(c *Ctx, f *File, b *SourceBinding, st *monsterState) {
	sig := "profile,suffix,shape,length_m,width_m,radius_m,startup_ms,active_ms,recovery_ms,cooldown_ms,normal_coeff,elite_coeff,hit_cap"
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, sig); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"Typed Attack Expansion table not found")
		return
	}
	dm := TypeSpec{Name: "decimal", IntSuffix: "m", IntScale: 1000}
	meters := func(raw string) int64 {
		v, err := dm.ParseValue(raw)
		if err != nil {
			iv, err2 := (TypeSpec{Name: "int"}).ParseValue(raw)
			if err2 != nil {
				return -1
			}
			return iv.Int * 1000
		}
		if v.Kind == config.KindRational {
			if i, ok := v.Rat.Int(); ok {
				return i
			}
			// rational meters → mm
			num := v.Rat.Num * 1000
			r, err := config.ReduceRat(num, v.Rat.Den)
			if err == nil {
				if i, ok := r.Int(); ok {
					return i
				}
			}
		}
		return -1
	}
	rat := func(raw string) config.Rat {
		d, err := parseDecimal(raw)
		if err != nil {
			if iv, e2 := (TypeSpec{Name: "int"}).ParseValue(raw); e2 == nil {
				return config.Rat{Num: iv.Int, Den: 1}
			}
			return config.Rat{Num: 0, Den: 1}
		}
		return d
	}
	intv := func(raw string) int64 {
		v, err := (TypeSpec{Name: "int"}).ParseValue(raw)
		if err != nil {
			return -1
		}
		return v.Int
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		g := func(i int) string { return cellAt(row, i).Scalar() }
		ar := attackProfileRow{
			profile: g(0), suffix: g(1), shape: g(2),
			lengthMm: meters(g(3)), widthMm: meters(g(4)), radiusMm: meters(g(5)),
			startupMs: intv(g(6)), activeMs: intv(g(7)), recoveryMs: intv(g(8)), cooldownMs: intv(g(9)),
			normalCoeff: rat(g(10)), eliteCoeff: rat(g(11)), hitCap: intv(g(12)),
			line: row[0].Line,
		}
		st.attackRows[ar.profile] = append(st.attackRows[ar.profile], ar)
		c.EmitParam(f.Name, b.Raw, "attack_profile",
			[]config.Value{config.VStr(ar.profile), config.VStr(ar.suffix)},
			map[string]config.Value{
				"profile":      config.VStr(ar.profile),
				"suffix":       config.VStr(ar.suffix),
				"shape":        config.VStr(ar.shape),
				"length_mm":    config.VInt(ar.lengthMm),
				"width_mm":     config.VInt(ar.widthMm),
				"radius_mm":    config.VInt(ar.radiusMm),
				"startup_ms":   config.VInt(ar.startupMs),
				"active_ms":    config.VInt(ar.activeMs),
				"recovery_ms":  config.VInt(ar.recoveryMs),
				"cooldown_ms":  config.VInt(ar.cooldownMs),
				"normal_coeff": mustRat(ar.normalCoeff),
				"elite_coeff":  mustRat(ar.eliteCoeff),
				"hit_cap":      config.VInt(ar.hitCap),
			}, ar.line)
	}
	c.consumed(f, b)
}

// monMechanicOps — Typed Mechanic Operations table: parse `ops` grammar.
func monMechanicOps(c *Ctx, f *File, b *SourceBinding, st *monsterState) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "mechanic_id,attack_suffix,ops"); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"Typed Mechanic Operations table not found")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		mid := cellAt(row, 0).Scalar()
		suffix := cellAt(row, 1).Scalar()
		ops, err := ParseOps(cellAt(row, 2).Scalar(), mechanicOpCtors)
		if err != nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
				"ops %s/%s: %v", mid, suffix, err)
			continue
		}
		st.mechanicOps[mid+"|"+suffix] = ops
		c.Emit(f.Name, b.Raw, "mechanic_op",
			[]config.Value{config.VStr(mid), config.VStr(suffix)},
			map[string]config.Value{
				"mechanic_id":   config.VStr(mid),
				"attack_suffix": config.VStr(suffix),
				"ops":           config.VList(ops...),
			}, row[0].Line)
	}
	c.consumed(f, b)
}

// monOverrides — Typed Roster Overrides table.
func monOverrides(c *Ctx, f *File, b *SourceBinding, st *monsterState) {
	var tbl *Block
	for _, sec := range bindingSections(c, f, b) {
		if t := findTable(sec, "monster_key,attack_suffix,override_field,value"); t != nil {
			tbl = t
		}
	}
	if tbl == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
			"Typed Roster Overrides table not found")
		return
	}
	for ri := range tbl.Cells {
		row := tbl.Cells[ri]
		key := cellAt(row, 0).Scalar()
		suffix := cellAt(row, 1).Scalar()
		field := cellAt(row, 2).Scalar()
		raw := cellAt(row, 3).Scalar()
		var v config.Value
		var err error
		switch field {
		case "ops":
			var ops []config.Value
			ops, err = ParseOps(raw, mechanicOpCtors)
			v = config.VList(ops...)
		case "jump":
			v, err = (TypeSpec{Name: "bool"}).ParseValue(raw)
		default:
			v, err = (TypeSpec{Name: "int"}).ParseValue(raw)
		}
		if err != nil {
			c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, row[0].Line,
				"override %s %s %s: %v", key, suffix, field, err)
			continue
		}
		mkey := "monster." + key
		st.overrides[mkey] = append(st.overrides[mkey], config.VRec(map[string]config.Value{
			"monster_id":    config.VStr(mkey),
			"attack_suffix": config.VStr(suffix),
			"field":         config.VStr(field),
			"value":         v,
		}))
		c.Emit(f.Name, b.Raw, "roster_override",
			[]config.Value{config.VStr(mkey), config.VStr(suffix), config.VStr(field)},
			map[string]config.Value{
				"monster_id":    config.VStr(mkey),
				"attack_suffix": config.VStr(suffix),
				"field":         config.VStr(field),
				"value":         v,
			}, row[0].Line)
	}
	c.consumed(f, b)
}

// monRequiredFields — Static Expansion Requirements fence → checklist.
func monRequiredFields(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			var fields []config.Value
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l != "" {
					fields = append(fields, config.VStr(l))
				}
			}
			c.EmitParam(f.Name, b.Raw, "required_monster_field",
				[]config.Value{config.VStr("checklist")},
				map[string]config.Value{"fields": config.VList(fields...)},
				fb.Line+1)
		}
	}
	c.consumed(f, b)
}

// monEmit realizes the roster into `monster` records and expands
// `attack.<key>.<suffix>` rows.
func monEmit(c *Ctx, f *File, st *monsterState) {
	if st.stat == nil {
		return
	}
	for _, m := range st.roster {
		env := map[string]config.Rat{"L": {Num: m.level, Den: 1}}
		stats := map[string]config.Value{}
		for _, fn := range []string{"normal_hp", "normal_attack", "normal_defense"} {
			ex := st.stat[fn]
			if ex == nil {
				c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, m.line,
					"stat formula %s missing", fn)
				continue
			}
			r, err := EvalExpr(ex, env)
			if err != nil {
				c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, m.line,
					"%s %s: %v", m.id, fn, err)
				continue
			}
			v, _ := r.Int()
			stats[strings.TrimPrefix(fn, "normal_")] = config.VInt(v)
		}
		if m.rank == "ELITE" {
			// ELITE fence: MAX_HP = floor(normal_hp * 4.00), ATTACK
			// floor(normal_attack * 1.25), DEFENSE floor(normal_defense*1.20)
			eenv := map[string]config.Rat{
				"L":              {Num: m.level, Den: 1},
				"normal_hp":      {Num: stats["hp"].Int, Den: 1},
				"normal_attack":  {Num: stats["attack"].Int, Den: 1},
				"normal_defense": {Num: stats["defense"].Int, Den: 1},
			}
			for _, pair := range [][2]string{
				{"MAX_HP", "hp"}, {"ATTACK", "attack"}, {"DEFENSE", "defense"},
			} {
				ex := st.stat[pair[0]]
				if ex == nil {
					continue
				}
				r, err := EvalExpr(ex, eenv)
				if err != nil {
					c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, m.line,
						"%s %s: %v", m.id, pair[0], err)
					continue
				}
				v, _ := r.Int()
				stats[pair[1]] = config.VInt(v)
			}
		}
		stats["max_mp"] = config.VInt(0)
		cdm := config.Rat{Num: 1, Den: 1}
		if m.rank == "ELITE" && st.eliteCDM != nil {
			if r, err := EvalExpr(st.eliteCDM, env); err == nil {
				cdm = r
			}
		}
		stats["control_duration_multiplier"] = mustRat(cdm)

		size := "MONSTER_MEDIUM"
		if m.rank == "ELITE" {
			size = "MONSTER_ELITE"
		} else if st.smallRoster[m.id] {
			size = "MONSTER_SMALL"
		}
		mp := st.moveProfiles[m.movement]
		if mp == nil {
			c.Diags.Addf(config.DiagUnresolvedReference, f.Path, m.line,
				"%s movement profile %q undeclared", m.id, m.movement)
		}
		fields := map[string]config.Value{
			"monster_id":     config.VStr(m.id),
			"rank":           config.VStr(m.rank),
			"level":          config.VInt(m.level),
			"element":        config.VStr(m.element),
			"stats":          config.VRec(stats),
			"movement":       config.VStr(m.movement),
			"combat":         config.VStr(m.combat),
			"special":        config.VStr(m.special),
			"size_profile":   config.VStr(size),
			"base_exp":       config.VInt(m.baseExp),
			"drop_table_id":  config.VStr(m.dropTable),
			"act":            config.VStr(m.act),
			"in_launch":      config.VBool(m.inLaunch),
			"aggro_range_mm": mpValue(mp, "aggro_range"),
			"leash_rule":     config.VStr("configured_min_leash"),
			"min_leash_mm":   mpValue(mp, "configured_min_leash"),
		}
		if m.mechanic != "" {
			fields["mechanic_id"] = config.VStr(m.mechanic)
		}
		if m.zone != "" {
			fields["zone_title"] = config.VStr(m.zone)
		}
		ids := st.emitAttacks(c, f, m)
		fields["attacks"] = config.VStrs(ids...)
		c.Emit(f.Name, "Launch Roster", "monster",
			[]config.Value{config.VStr(m.id)}, fields, m.line)
	}
	// roster count gate
	var ln, le, s0 int
	for _, m := range st.roster {
		if !m.inLaunch {
			s0++
		} else if m.rank == "NORMAL" {
			ln++
		} else {
			le++
		}
	}
	if st.hasLaunchDecl && (int64(ln) != st.wantN || int64(le) != st.wantE) {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"monster roster NORMAL=%d ELITE=%d, declared %d/%d", ln, le, st.wantN, st.wantE)
	}
	if st.hasS0Decl && int64(s0) != st.wantS0 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"monster season-0 rows %d, declared %d", s0, st.wantS0)
	}
}

func mpValue(mp map[string]config.Value, k string) config.Value {
	if mp == nil {
		return config.VNull()
	}
	if v, ok := mp[k]; ok {
		return v
	}
	return config.VNull()
}

// emitAttacks emits `attack.<key>.<suffix>` records for a roster member and
// returns the emitted IDs (sorted deterministically by profile row order).
func (st *monsterState) emitAttacks(c *Ctx, f *File, m *monsterRow) []string {
	rows := st.attackRows[m.combat]
	if len(rows) == 0 {
		c.Diags.Addf(config.DiagUnresolvedReference, f.Path, m.line,
			"%s combat profile %q has no typed attack rows", m.id, m.combat)
		return nil
	}
	if (m.combat == "GUARD" || m.combat == "SUMMON_ECHO") && m.rank != "ELITE" {
		c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, m.line,
			"%s combat %s legal only on rank=ELITE", m.id, m.combat)
	}
	key := strings.TrimPrefix(m.id, "monster.")
	var rows2 []attackProfileRow
	if m.combat == "RUSH" {
		// RUSH additionally inherits MELEE/basic
		rows2 = append(rows2, st.attackRows["MELEE"]...)
		rows2 = append(rows2, rows...)
	} else {
		rows2 = rows
	}
	var ids []string
	for _, ar := range rows2 {
		aid := "attack." + key + "." + ar.suffix
		ids = append(ids, aid)
		coeff := ar.normalCoeff
		if m.rank == "ELITE" {
			coeff = ar.eliteCoeff
		}
		geom := config.VNull()
		switch ar.shape {
		case "RECT", "PROJECTILE":
			geom = config.VRec(map[string]config.Value{
				"shape":     config.VStr(ar.shape),
				"length_mm": config.VInt(ar.lengthMm),
				"width_mm":  config.VInt(ar.widthMm),
			})
		case "CIRCLE":
			geom = config.VRec(map[string]config.Value{
				"shape":     config.VStr(ar.shape),
				"radius_mm": config.VInt(ar.radiusMm),
			})
		case "NONE":
			geom = config.VNull()
		}
		fields := map[string]config.Value{
			"attack_id":   config.VStr(aid),
			"monster_id":  config.VStr(m.id),
			"profile":     config.VStr(ar.profile),
			"suffix":      config.VStr(ar.suffix),
			"shape":       config.VStr(ar.shape),
			"geometry":    geom,
			"startup_ms":  config.VInt(ar.startupMs),
			"active_ms":   config.VInt(ar.activeMs),
			"recovery_ms": config.VInt(ar.recoveryMs),
			"cooldown_ms": config.VInt(ar.cooldownMs),
			"coefficient": mustRat(coeff),
			"hit_cap":     config.VInt(ar.hitCap),
		}
		if ar.shape == "NONE" && ar.profile == "GUARD" && ar.suffix == "guard" {
			fields["frontal_incoming_multiplier"] = mustRat(config.Rat{Num: 60, Den: 100})
		}
		// mechanic ops bound via `special` token
		for _, sfx := range []string{m.mechanic + "|ALL", m.mechanic + "|" + ar.suffix} {
			if ops, ok := st.mechanicOps[sfx]; ok && m.mechanic != "" {
				fields["ops"] = config.VList(ops...)
				break
			}
		}
		// roster overrides
		for _, ov := range st.overrides[m.id] {
			ovr := ov.Rec
			sfx := ovr["attack_suffix"].Str
			if sfx != ar.suffix && sfx != "ALL" {
				continue
			}
			fld := ovr["field"].Str
			fields[fld] = ovr["value"]
		}
		c.Emit(f.Name, "Typed Attack Expansion", "attack",
			[]config.Value{config.VStr(m.id), config.VStr(ar.suffix)},
			fields, ar.line)
	}
	// FOLLOWUP ops on overrides emit attack.<key>.slam
	for _, ov := range st.overrides[m.id] {
		ovr := ov.Rec
		if ovr["field"].Str != "ops" {
			continue
		}
		for _, op := range ovr["value"].Elems {
			if op.Rec["kind"].Str != "FOLLOWUP" {
				continue
			}
			args := op.Rec["args"].Elems
			slamID := "attack." + key + "." + args[0].Str
			ids = append(ids, slamID)
			c.Emit(f.Name, "Typed Roster Overrides", "attack",
				[]config.Value{config.VStr(m.id), config.VStr(args[0].Str)},
				map[string]config.Value{
					"attack_id":   config.VStr(slamID),
					"monster_id":  config.VStr(m.id),
					"profile":     config.VStr("CONTROL"),
					"suffix":      config.VStr(args[0].Str),
					"shape":       config.VStr("RECT"),
					"geometry":    config.VRec(map[string]config.Value{"shape": config.VStr("RECT"), "length_mm": config.VInt(3000), "width_mm": config.VInt(1200)}),
					"startup_ms":  config.VInt(800),
					"active_ms":   config.VInt(150),
					"recovery_ms": config.VInt(650),
					"cooldown_ms": config.VInt(5500),
					"coefficient": args[1],
					"hit_cap":     config.VInt(1),
				}, m.line)
		}
	}
	return ids
}

func mustRat(r config.Rat) config.Value {
	v, err := config.VRat(r.Num, r.Den)
	if err != nil {
		return config.VNull()
	}
	return v
}

func firstPath(b *SourceBinding) string {
	if len(b.SectionPaths) == 0 {
		return ""
	}
	return b.SectionPaths[0]
}
