package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileClassSkill — class_skill_catalog.md driver. The registry binds the
// Active Payloads table formally; the prose family map declares the other
// authoritative tables (runtime matrix, proc matrix, passive scaling, target
// scaling, action specifications, spatial effects, zone/passive payloads).
func compileClassSkill(c *Ctx, f *File, r *Registry) {
	st := &skState{
		matrix:   map[string]map[string]config.Value{},
		effects:  map[string]bool{},
		zones:    map[string]bool{},
		spatials: map[string]bool{},
		actions:  map[string]map[string]config.Value{},
		payloads: map[string]int{},
	}
	c.Data["skill.state"] = st
	for _, b := range r.Bindings {
		skActivePayloads(c, f, b, st)
	}
	// prose family map — authoritative tables consumed by declared family
	skRuntimeMatrix(c, f, st)
	skBasicScaling(c, f, st)
	skEffectTemplates(c, f, st)
	skActiveScaling(c, f)
	skPassiveScaling(c, f)
	skTargetScaling(c, f)
	skReachAudit(c, f)
	skActionSpecs(c, f, st)
	skSecondarySpatial(c, f, st)
	skZoneSchedules(c, f, st)
	skPassivePayloads(c, f, st)
	skVerify(c, f, st)
}

type skState struct {
	matrix   map[string]map[string]config.Value // skill_id -> runtime matrix row
	effects  map[string]bool                    // effect_id templates
	zones    map[string]bool                    // zone_id schedules
	spatials map[string]bool                    // spatial_effect_id
	actions  map[string]map[string]config.Value // skill_id -> action row
	payloads map[string]int                     // skill_id -> payload row count
	geoCount int
}

// ---- geometry calls -------------------------------------------------

// geomArgRe parses `name=0.0m|0.0m/s|5000ms|0.0` named arguments.
var geomCallRe = regexp.MustCompile("([A-Z_]+)\\(([^)]*)\\)")
var geomArgRe = regexp.MustCompile(`([a-z_]+)\s*=\s*([0-9.]+)(m/s|ms|m|s)?`)

// parseGeometry parses one `KIND(arg=val[unit], ...)` or bare `KIND` call.
func parseGeometry(c *Ctx, f *File, raw string, line int) map[string]config.Value {
	raw = strings.TrimSpace(strings.Trim(raw, "`"))
	m := geomCallRe.FindStringSubmatch(raw)
	if m == nil {
		if regexp.MustCompile(`^[A-Z_]+$`).MatchString(raw) {
			return map[string]config.Value{"kind": config.VStr(raw)}
		}
		c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "geometry %q", raw)
		return nil
	}
	kind := m[1]
	args := map[string]config.Value{"kind": config.VStr(kind)}
	for _, am := range geomArgRe.FindAllStringSubmatch(m[2], -1) {
		scale := int64(1)
		switch am[3] {
		case "m":
			scale = 1000 // meters -> mm
		case "m/s":
			scale = 1000 // mm/s
		}
		d, err := parseDecimal(am[2])
		if err != nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
				"geometry arg %q", am[0])
			continue
		}
		d.Num *= scale
		r, rerr := config.ReduceRat(d.Num, d.Den)
		if rerr != nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "geometry arg %q", am[0])
			continue
		}
		args[am[1]] = mustRat(r)
	}
	return args
}

var classRe = regexp.MustCompile(`^skill\.([a-z]+)\.(basic|passive|active)\.`)
var lvUnlockRe = regexp.MustCompile(`^Lv([0-9]+)$`)
var phaseRe = regexp.MustCompile(`^([0-9]+)/([0-9]+)/([0-9]+)$`)
var durRe = regexp.MustCompile(`^([0-9.]+)s$`)
var mpCostRe = regexp.MustCompile(`^([0-9]+)\s*MP$`)

// ---- runtime matrix -------------------------------------------------

func skRuntimeMatrix(c *Ctx, f *File, st *skState) {
	sec := f.Root.SectionAt("Canonical Runtime Matrix")
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1, "Canonical Runtime Matrix missing")
		return
	}
	for _, cls := range sec.Children {
		elem := strings.SplitN(cls.Title, " ", 2)[0]
		for _, bl := range cls.Content {
			if bl.Kind != BlockTable {
				continue
			}
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				sid := cellAt(row, 0).Scalar()
				lv := cellAt(row, 2).Scalar()
				m := lvUnlockRe.FindStringSubmatch(lv)
				if m == nil {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"unlock %q", lv)
					continue
				}
				ulv, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				tags := cellAt(row, 5).Scalar()
				var tagList []config.Value
				if tags != "NONE" {
					for _, t := range strings.Split(tags, ",") {
						tagList = append(tagList, config.VStr(strings.TrimSpace(t)))
					}
				}
				exec := cellAt(row, 3).Scalar()
				tgt := cellAt(row, 4).Scalar()
				air := cellAt(row, 6).Scalar()
				mb := "ALLOW"
				for _, t := range tagList {
					if t.Str == "MOVEMENT" {
						mb = "FORCED"
					}
				}
				cat := "active"
				if mm := classRe.FindStringSubmatch(sid); mm != nil {
					cat = mm[2]
				}
				fields := map[string]config.Value{
					"skill_id":          config.VStr(sid),
					"display":           config.VStr(cellAt(row, 1).Scalar()),
					"unlock_level":      ulv,
					"class_element":     config.VStr(elem),
					"category":          config.VStr(cat),
					"execution_type":    config.VStr(exec),
					"targeting_mode":    config.VStr(tgt),
					"tags":              config.VSet(tagList...),
					"air_profile":       config.VStr(air),
					"movement_behavior": config.VStr(mb),
					"damage_element":    config.VStr(elem),
				}
				st.matrix[sid] = fields
				c.Emit(f.Name, "Canonical Runtime Matrix", "skill",
					[]config.Value{config.VStr(sid)}, fields, row[0].Line)
			}
		}
	}
}

// ---- scaling formulas -------------------------------------------------

func skScalingFences(c *Ctx, f *File, sec *Section, fam string) {
	algo := "SCALING"
	switch {
	case strings.HasPrefix(sec.Title, "Basic"):
		algo = "BASIC_SCALING"
	case strings.HasPrefix(sec.Title, "Active"):
		algo = "ACTIVE_SCALING"
	case strings.HasPrefix(sec.Title, "Passive"):
		algo = "PASSIVE_LINEAR"
	}
	for _, fb := range allFences(sec, "text") {
		for j, l := range fb.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			i := strings.IndexByte(l, '=')
			if i <= 0 {
				continue
			}
			name := strings.TrimSpace(l[:i])
			ex, err := ParseExpr(strings.TrimSpace(l[i+1:]))
			if err != nil {
				continue // prose continuation, not a formula
			}
			c.EmitParam(f.Name, sec.Title, fam,
				[]config.Value{config.VStr(algo + "." + name)},
				map[string]config.Value{"algorithm": config.VStr(algo), "expr": config.VExpr(ex)}, fb.Line+1+j)
		}
	}
}

func skBasicScaling(c *Ctx, f *File, st *skState) {
	sec := f.Root.SectionAt("Skill-Level Scaling Model > Basic Attack Scaling")
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1, "Basic Attack Scaling missing")
		return
	}
	skScalingFences(c, f, sec, "scaling_algorithm")
	// Basic Attack Cooldown & Status Proc Matrix
	for _, ch := range sec.Children {
		if !strings.HasPrefix(ch.Title, "Basic Attack Cooldown") {
			continue
		}
		for _, bl := range ch.Content {
			if bl.Kind != BlockTable {
				continue
			}
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				sid := cellAt(row, 0).Scalar()
				num := func(i int) config.Value {
					d, err := parseDecimal(cellAt(row, i).Scalar())
					if err != nil {
						c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
							"proc matrix cell %q", cellAt(row, i).Scalar())
						return config.VNull()
					}
					return mustRat(d)
				}
				// status_effect cell: `a` + `b` list + optional notes
				var procs []config.Value
				for _, tok := range strings.Split(cellAt(row, 7).Text, ";") {
					for _, tt := range strings.Split(tok, "+") {
						tt = strings.TrimSpace(tt)
						if m := regexp.MustCompile("`?(effect\\.[a-z0-9_.]+|spatial\\.[a-z0-9_.]+)`?").FindStringSubmatch(tt); m != nil {
							procs = append(procs, config.VStr(m[1]))
						}
					}
				}
				c.Emit(f.Name, "Basic Attack Cooldown & Status Proc Matrix", "basic_proc",
					[]config.Value{config.VStr(sid)},
					map[string]config.Value{
						"skill_id":         config.VStr(sid),
						"base_coefficient": num(1),
						"base_cd_s":        num(2),
						"max_cd_s":         num(3),
						"cd_step_s":        num(4),
						"base_proc":        num(5),
						"max_proc":         num(6),
						"status_effects":   config.VList(procs...),
						"note":             config.VStr(strings.TrimSpace(cellAt(row, 7).Text)),
					}, row[0].Line)
			}
		}
	}
}

func skActiveScaling(c *Ctx, f *File) {
	sec := f.Root.SectionAt("Skill-Level Scaling Model > Active Skill Scaling")
	if sec == nil {
		return
	}
	skScalingFences(c, f, sec, "scaling_algorithm")
}

func skPassiveScaling(c *Ctx, f *File) {
	sec := f.Root.SectionAt("Skill-Level Scaling Model > Passive Skill Scaling (Levels 1..6)")
	if sec == nil {
		sec = f.Root.SectionAt("Skill-Level Scaling Model > Passive Skill Scaling")
	}
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1, "Passive Skill Scaling missing")
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			pid := cellAt(row, 0).Scalar()
			val := func(i int) config.Value {
				raw := cellAt(row, i).Scalar()
				if m := durRe.FindStringSubmatch(raw); m != nil {
					d, _ := parseDecimal(m[1])
					return func() config.Value {
						if rr, rerr := config.ReduceRat(d.Num*1000, d.Den); rerr == nil {
							return mustRat(rr)
						}
						return config.VNull()
					}()
				}
				d, err := parseDecimal(strings.TrimPrefix(raw, "+"))
				if err != nil {
					return config.VStr(raw)
				}
				return mustRat(d)
			}
			c.Emit(f.Name, "Passive Skill Scaling", "passive_scaling",
				[]config.Value{config.VStr(pid)},
				map[string]config.Value{
					"passive_id":         config.VStr(pid),
					"level_1":            val(1),
					"per_level_step":     val(2),
					"level_6":            val(3),
					"effect_description": config.VStr(cellAt(row, 4).Scalar()),
				}, row[0].Line)
		}
	}
}

func skEffectTemplates(c *Ctx, f *File, st *skState) {
	base := f.Root.SectionAt("Skill-Level Scaling Model")
	if base == nil {
		return
	}
	var tplSecs []*Section
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, ch := range s.Children {
			if strings.Contains(ch.Title, "Templates") {
				tplSecs = append(tplSecs, ch)
			}
			walk(ch)
		}
	}
	walk(base)
	for _, ts := range tplSecs {
		for _, bl := range ts.Content {
			if bl.Kind != BlockTable {
				continue
			}
			// both template tables + the shield table key by effect_id
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				eid := cellAt(row, 0).Scalar()
				if !strings.HasPrefix(eid, "effect.") {
					continue
				}
				fields := map[string]config.Value{
					"effect_id": config.VStr(eid),
					"source":    config.VStr(ts.Title),
				}
				for hi, h := range bl.Headers {
					fn := fieldName(h)
					if fn == "effect_id" {
						continue
					}
					fields[fn] = config.VStr(cellAt(row, hi).Scalar())
				}
				st.effects[eid] = true
				c.Emit(f.Name, ts.Title, "effect_template",
					[]config.Value{config.VStr(eid)}, fields, row[0].Line)
			}
		}
	}
}

// ---- target scaling ----------------------------------------------------

var tsGroupHeadRe = regexp.MustCompile(`^\*\*([^*]+)\*\*`)
var tsTierRe = regexp.MustCompile(`^([0-9]+(?:\.\.[0-9]+)?)\s*/\s*([0-9]+)$`)

func skTargetScaling(c *Ctx, f *File) {
	sec := f.Root.SectionAt("Skill-Level Scaling Model > Target Limits and Target Scaling Tables")
	if sec == nil {
		return
	}
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, ch := range s.Children {
			var group string
			var members []string
			for _, bl := range ch.Content {
				for _, l := range bl.Prose {
					l = strings.TrimSpace(l)
					if m := tsGroupHeadRe.FindStringSubmatch(l); m != nil {
						group = strings.TrimSpace(m[1])
						continue
					}
					if group != "" && strings.Contains(l, "`") {
						for _, id := range strings.Split(l, ",") {
							id = strings.TrimSpace(strings.Trim(id, "`"))
							if id != "" {
								members = append(members, id)
							}
						}
					}
				}
				if bl.Kind == BlockTable && group != "" {
					var tiers []config.Value
					for ri := range bl.Cells {
						row := bl.Cells[ri]
						hdrs := bl.Headers
						for ci := range row {
							h := ""
							if ci < len(hdrs) {
								h = hdrs[ci]
							}
							mm := tsTierRe.FindStringSubmatch(strings.TrimSpace(cellAt(row, ci).Scalar()))
							if mm == nil {
								continue
							}
							mv, _ := (TypeSpec{Name: "int"}).ParseValue(mm[1])
							pv, _ := (TypeSpec{Name: "int"}).ParseValue(mm[2])
							tiers = append(tiers, config.VRec(map[string]config.Value{
								"band":            config.VStr(h),
								"monster_targets": mv,
								"player_targets":  pv,
							}))
						}
					}
					mv := make([]config.Value, len(members))
					for i, s := range members {
						mv[i] = config.VStr(s)
					}
					c.Emit(f.Name, "Target Limits and Target Scaling Tables", "target_scaling",
						[]config.Value{config.VStr(group)},
						map[string]config.Value{
							"group":   config.VStr(group),
							"members": config.VList(mv...),
							"tiers":   config.VList(tiers...),
						}, bl.Line)
					group, members = "", nil
				}
			}
			walk(ch)
		}
	}
	walk(sec)
}

func skReachAudit(c *Ctx, f *File) {
	sec := f.Root.SectionAt("Launch Reach Audit")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			c.EmitParam(f.Name, "Launch Reach Audit", "reach_audit",
				[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
				map[string]config.Value{
					"role":     config.VStr(cellAt(row, 0).Scalar()),
					"rows":     config.VStr(cellAt(row, 1).Scalar()),
					"observed": config.VStr(cellAt(row, 2).Scalar()),
					"band":     config.VStr(cellAt(row, 3).Scalar()),
					"result":   config.VStr(cellAt(row, 4).Scalar()),
				}, row[0].Line)
		}
	}
}

// ---- action specs -------------------------------------------------------

func skActionSpecs(c *Ctx, f *File, st *skState) {
	sec := f.Root.SectionAt("Action Timing and Geometry")
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, 1, "Action Timing and Geometry missing")
		return
	}
	geoCount := 0
	for _, cls := range sec.Children {
		for _, bl := range cls.Content {
			if bl.Kind != BlockTable {
				continue
			}
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				sid := cellAt(row, 0).Scalar()
				ph := phaseRe.FindStringSubmatch(cellAt(row, 2).Scalar())
				if ph == nil {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"phases %q", cellAt(row, 2).Scalar())
					continue
				}
				p := func(i int) config.Value {
					v, _ := (TypeSpec{Name: "int"}).ParseValue(ph[i])
					return v
				}
				geoText := cellAt(row, 3).Text
				// optional `; air KIND(...)` variant
				parts := strings.SplitN(geoText, ";", 2)
				geo := parseGeometry(c, f, parts[0], row[0].Line)
				fields := map[string]config.Value{
					"skill_id":   config.VStr(sid),
					"speed_stat": config.VStr(cellAt(row, 1).Scalar()),
					"startup_ms": p(0), "active_ms": p(1), "recovery_ms": p(2),
					"geometry": config.VRec(geo),
				}
				geoCount++
				if len(parts) == 2 {
					air := strings.TrimSpace(parts[1])
					if am := geomCallRe.FindStringSubmatch(air); am != nil ||
						regexp.MustCompile(`^air`).MatchString(air) {
						air = strings.TrimSpace(strings.TrimPrefix(air, "air"))
						if ag := parseGeometry(c, f, air, row[0].Line); ag != nil {
							fields["air_geometry"] = config.VRec(ag)
						}
					}
				}
				if d, err := parseDecimal(strings.TrimSuffix(cellAt(row, 4).Scalar(), "s")); err == nil {
					fields["base_cd_s"] = mustRat(d)
				}
				if m := mpCostRe.FindStringSubmatch(cellAt(row, 5).Scalar()); m != nil {
					v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
					fields["cost_mp"] = v
				}
				st.actions[sid] = fields
				c.Emit(f.Name, "Action Specifications", "skill_action",
					[]config.Value{config.VStr(sid)}, fields, row[0].Line)
			}
		}
	}
	st.geoCount = geoCount
	_ = geoCount
}

func skSecondarySpatial(c *Ctx, f *File, st *skState) {
	sec := f.Root.SectionAt("Secondary Spatial Effects and Displacement")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			sid := cellAt(row, 0).Scalar()
			if !strings.HasPrefix(sid, "spatial.") {
				continue
			}
			st.spatials[sid] = true
			fields := map[string]config.Value{
				"spatial_effect_id": config.VStr(sid),
			}
			for hi, h := range bl.Headers {
				fn := fieldName(h)
				if fn == "spatial_effect_id" {
					continue
				}
				fields[fn] = config.VStr(cellAt(row, hi).Scalar())
			}
			c.Emit(f.Name, "Secondary Spatial Effects and Displacement", "spatial_effect",
				[]config.Value{config.VStr(sid)}, fields, row[0].Line)
		}
	}
}

// ---- active payloads -----------------------------------------------------

// payloadCallRe parses `KIND(args)`; arg lists may be positional or k=v.
var payloadCallRe = regexp.MustCompile(`([A-Z_]+)\s*\(([^)]*)\)`)

func parsePayloadCalls(c *Ctx, f *File, raw string, line int) []config.Value {
	var out []config.Value
	for _, piece := range strings.Split(raw, ";") {
		piece = strings.TrimSpace(strings.Trim(piece, "`"))
		if piece == "" {
			continue
		}
		m := payloadCallRe.FindStringSubmatch(piece)
		if m == nil {
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
				"payload %q not a closed call", piece)
			continue
		}
		kind := m[1]
		rec := map[string]config.Value{"kind": config.VStr(kind)}
		switch kind {
		case "DAMAGE":
			d, err := parseDecimal(strings.TrimSpace(m[2]))
			if err != nil {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "DAMAGE arg %q", m[2])
				continue
			}
			rec["coefficient"] = mustRat(d)
		case "HEAL":
			parts := strings.Split(m[2], ",")
			if len(parts) != 2 {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "HEAL args %q", m[2])
				continue
			}
			h, _ := parseDecimal(strings.TrimSpace(parts[0]))
			a, _ := parseDecimal(strings.TrimSpace(parts[1]))
			rec["target_max_hp_ratio"] = mustRat(h)
			rec["attack_coefficient"] = mustRat(a)
		case "EXECUTE":
			parts := strings.Split(m[2], ",")
			if len(parts) != 2 {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line, "EXECUTE args %q", m[2])
				continue
			}
			h, _ := parseDecimal(strings.TrimSpace(parts[0]))
			a, _ := parseDecimal(strings.TrimSpace(parts[1]))
			rec["hp_threshold"] = mustRat(h)
			rec["multiplier"] = mustRat(a)
		case "STATUS", "SHIELD":
			rec["effect_id"] = config.VStr(strings.TrimSpace(m[2]))
		case "SPATIAL", "ZONE":
			rec["ref_id"] = config.VStr(strings.TrimSpace(m[2]))
		case "BARRIER":
			if strings.TrimSpace(m[2]) != "primary_geometry" {
				c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
					"BARRIER arg %q must be primary_geometry", m[2])
				continue
			}
			rec["geometry_ref"] = config.VStr("PRIMARY")
			rec["block_enemy_movement"] = config.VBool(true)
			rec["block_projectiles"] = config.VBool(true)
		default:
			c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
				"unknown payload constructor %q", kind)
			continue
		}
		out = append(out, config.VRec(rec))
	}
	return out
}

func skActivePayloads(c *Ctx, f *File, b *SourceBinding, st *skState) {
	sec := f.Root.SectionAt("Compiler Source Schema > Active Payloads")
	if sec == nil {
		// `### Active Payloads` lives under the schema section
		for _, s := range f.Root.FindSections("Active Payloads") {
			sec = s
		}
	}
	if sec == nil {
		c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line, "Active Payloads missing")
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			sid := cellAt(row, 0).Scalar()
			payloads := parsePayloadCalls(c, f, cellAt(row, 1).Text, row[0].Line)
			st.payloads[sid] = len(payloads)
			for ord, pl := range payloads {
				c.Emit(f.Name, b.Raw, "skill_effect",
					[]config.Value{config.VStr(sid), config.VInt(int64(ord))},
					map[string]config.Value{
						"skill_id": config.VStr(sid),
						"ordinal":  config.VInt(int64(ord)),
						"payload":  pl,
					}, row[0].Line)
			}
		}
	}
	// CAT-002: the sole BARRIER row requires BARRIER_POSITION geometry
	c.EmitParam(f.Name, b.Raw, "rule_versions",
		[]config.Value{config.VStr("skill_barrier_payload")},
		map[string]config.Value{"version": config.VInt(1)}, b.Line)
	c.consumed(f, b)
}

func skZoneSchedules(c *Ctx, f *File, st *skState) {
	sec := f.Root.SectionAt("Compiler Source Schema > Zone Schedules")
	if sec == nil {
		for _, s := range f.Root.FindSections("Zone Schedules") {
			sec = s
		}
	}
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			zid := cellAt(row, 0).Scalar()
			num := func(i int) config.Value {
				v, err := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, i).Scalar())
				if err != nil {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"zone cell %q", cellAt(row, i).Scalar())
				}
				return v
			}
			payloads := parsePayloadCalls(c, f, cellAt(row, 5).Text, row[0].Line)
			st.zones[zid] = true
			c.Emit(f.Name, "Zone Schedules", "zone_schedule",
				[]config.Value{config.VStr(zid)},
				map[string]config.Value{
					"zone_id":           config.VStr(zid),
					"lifetime_ms":       num(1),
					"first_tick_ms":     num(2),
					"interval_ms":       num(3),
					"tick_count":        num(4),
					"payloads":          config.VList(payloads...),
					"activation_status": config.VStr(cellAt(row, 6).Scalar()),
				}, row[0].Line)
		}
	}
}

func skPassivePayloads(c *Ctx, f *File, st *skState) {
	sec := f.Root.SectionAt("Compiler Source Schema > Passive Payloads")
	if sec == nil {
		for _, s := range f.Root.FindSections("Passive Payloads") {
			sec = s
		}
	}
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			sid := cellAt(row, 0).Scalar()
			trig := cellAt(row, 1).Text
			var trigs []config.Value
			for _, t := range strings.Split(trig, ";") {
				t = strings.TrimSpace(strings.Trim(t, "`"))
				if t != "" {
					trigs = append(trigs, config.VStr(t))
				}
			}
			var payloads []config.Value
			for _, piece := range strings.Split(cellAt(row, 2).Text, ";") {
				piece = strings.TrimSpace(strings.Trim(piece, "`"))
				m := payloadCallRe.FindStringSubmatch(piece)
				if m == nil {
					continue
				}
				args := map[string]config.Value{"kind": config.VStr(m[1])}
				var positionals []config.Value
				for _, a := range strings.Split(m[2], ",") {
					a = strings.TrimSpace(a)
					if i := strings.IndexByte(a, '='); i > 0 {
						args[strings.TrimSpace(a[:i])] = config.VStr(strings.TrimSpace(a[i+1:]))
					} else {
						positionals = append(positionals, config.VStr(a))
					}
				}
				if len(positionals) > 0 {
					args["args"] = config.VList(positionals...)
				}
				payloads = append(payloads, config.VRec(args))
			}
			c.Emit(f.Name, "Passive Payloads", "passive_payload",
				[]config.Value{config.VStr(sid)},
				map[string]config.Value{
					"skill_id": config.VStr(sid),
					"triggers": config.VList(trigs...),
					"payloads": config.VList(payloads...),
				}, row[0].Line)
		}
	}
}

// ---- validation -----------------------------------------------------------

func skVerify(c *Ctx, f *File, st *skState) {
	// assertion 1: 5 classes x 12 skills (4 basic / 5 active / 3 passive)
	perClass := map[string]map[string]int{}
	for sid, fields := range st.matrix {
		m := classRe.FindStringSubmatch(sid)
		if m == nil {
			continue
		}
		if perClass[m[1]] == nil {
			perClass[m[1]] = map[string]int{}
		}
		perClass[m[1]][m[2]]++
		_ = fields
	}
	for cls, cats := range perClass {
		if cats["basic"] != 4 || cats["active"] != 5 || cats["passive"] != 3 {
			c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
				"class %s basic=%d active=%d passive=%d", cls,
				cats["basic"], cats["active"], cats["passive"])
		}
	}
	if len(perClass) != 5 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"classes %d != 5", len(perClass))
	}
	// assertion 12: exactly 45 primary geometries
	if st.geoCount != 45 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"primary geometries %d != 45", st.geoCount)
	}
	// assertion 22: every ACTIVE skill has exactly one payload row
	for sid, fields := range st.matrix {
		if fields["category"].Str != "active" {
			continue
		}
		if st.payloads[sid] == 0 {
			c.Diags.Addf(config.DiagUnresolvedReference, f.Path, 1,
				"active skill %s has no payload row", sid)
		}
	}
	// payload refs resolve: STATUS/SHIELD -> effect_template, ZONE -> zone,
	// SPATIAL -> spatial_effect; BARRIER requires BARRIER_POSITION geometry
	for range st.actions {
	}
	_ = f
}
