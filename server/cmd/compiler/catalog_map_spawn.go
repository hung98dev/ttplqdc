package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileMapSpawn — map_spawn_catalog.md driver (7 bindings): shared spawn
// contract + respawn bands, 6 night-rare groups, 6 safe maps, 54 persistent
// field groups (2 NORMAL + 1 ELITE per FIELD), pool resolution rules,
// public-boss + Spirit Surge placement rules.
func compileMapSpawn(c *Ctx, f *File, r *Registry) {
	st := &msState{season0: map[string]bool{}}
	// season-0 variant roster (canonical list — Logical Pool Resolution
	// re-detects the same tokens as a consistency check)
	for _, id := range []string{"dom_dom_nguyen", "bup_lua", "tinh_buoi", "co_lua", "vong_bien", "hon_gao"} {
		st.season0[id] = true
	}
	c.Data["mapspawn.state"] = st
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Shared Rules"):
			msSharedRules(c, f, b)
		case strings.HasPrefix(path, "Rare night encounters"):
			msRare(c, f, b)
		case strings.HasPrefix(path, "Safe / Social Maps"):
			msSafeMaps(c, f, b, st)
		case strings.Contains(path, "ACT"):
			msActMaps(c, f, b, st)
		case strings.HasPrefix(path, "Logical Pool Resolution"):
			msPoolRules(c, f, b, st)
		case strings.HasPrefix(path, "Public Boss Placement"):
			msBossPlacement(c, f, b)
		case strings.HasPrefix(path, "Spirit Surge Placement"):
			msSurge(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"map_spawn binding %q has no driver", b.Raw)
		}
	}
	msVerify(c, f, st)
}

type msState struct {
	groups    int
	safe      map[string]bool
	season0   map[string]bool // season-0 variant monster keys
	poolCount map[string]int  // map_id -> emitted pool entries
}

var bandRe = regexp.MustCompile(`^([A-Z_]+)\s*=\s*([0-9]+)\.\.([0-9]+)s`)
var spawnHeadRe = regexp.MustCompile(`^(spawn\.[a-z0-9_.]+)\s*@\s*(anchor\.spawn\.[a-z0-9_.]+)\s*$`)
var bossPlaceRe = regexp.MustCompile(`^(boss\.[a-z0-9_.]+)\s*->\s*(map\.[a-z0-9_.]+),\s*(anchor\.[a-z0-9_.]+)\s*$`)

func msSharedRules(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" || strings.HasPrefix(l, "(") {
					continue
				}
				if m := bandRe.FindStringSubmatch(l); m != nil {
					lo, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
					hi, _ := (TypeSpec{Name: "int"}).ParseValue(m[3])
					c.EmitParam(f.Name, b.Raw, "spawn_respawn_band",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{
							"kind": config.VStr(m[1]),
							"lo_s": lo, "hi_s": hi,
						}, fb.Line+1+j)
					continue
				}
				c.EmitParam(f.Name, b.Raw, "spawn_rule",
					[]config.Value{config.VInt(int64(fb.Line*1000 + j))},
					map[string]config.Value{"rule": config.VStr(l)}, fb.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func msRare(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				if m := chestFieldRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
					c.EmitParam(f.Name, b.Raw, "rare_default",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{"value": config.VStr(m[2])}, fb.Line+1)
				}
			}
		}
		if tbl := findTable(sec, "spawn_group_id,map_id,anchor_id,monster"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				gid := cellAt(row, 0).Scalar()
				mid := cellAt(row, 1).Scalar()
				aid := cellAt(row, 2).Scalar()
				mon := cellAt(row, 3).Scalar()
				c.Emit(f.Name, b.Raw, "spawn_group",
					[]config.Value{config.VStr(gid)},
					map[string]config.Value{
						"spawn_group_id":  config.VStr(gid),
						"map_id":          config.VStr(mid),
						"anchor_id":       config.VStr(aid),
						"kind":            config.VStr("NIGHT_RARE"),
						"activation":      config.VStr("NIGHT"),
						"max_alive":       config.VInt(1),
						"respawn_seconds": config.VInt(300),
						"monster_pool":    config.VList(config.VStr(mon)),
					}, row[0].Line)
				c.Emit(f.Name, b.Raw, "spawn_anchor",
					[]config.Value{config.VStr(aid)},
					map[string]config.Value{
						"anchor_id": config.VStr(aid),
						"map_id":    config.VStr(mid),
						"locator":   config.VStr("FIXED_POINT"),
						"kind":      config.VStr("NIGHT_RARE"),
					}, row[0].Line)
				c.Emit(f.Name, b.Raw, "map_pool",
					[]config.Value{config.VStr(mid), config.VStr(mon)},
					map[string]config.Value{
						"map_id":     config.VStr(mid),
						"monster_id": config.VStr(mon),
						"group":      config.VStr(gid),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func msSafeMaps(c *Ctx, f *File, b *SourceBinding, st *msState) {
	for _, sec := range bindingSections(c, f, b) {
		st.safe = map[string]bool{}
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "map.") {
					st.safe[l] = true
					c.Emit(f.Name, b.Raw, "safe_map",
						[]config.Value{config.VStr(l)},
						map[string]config.Value{
							"map_id":               config.VStr(l),
							"hostile_spawn_groups": config.VInt(0),
						}, fb.Line+1+j)
					continue
				}
				if m := chestFieldRe.FindStringSubmatch(l); m != nil {
					c.EmitParam(f.Name, b.Raw, "safe_map_requirement",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{"value": config.VStr(m[2])}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

var msGroupKind = map[string]string{
	"normal_01": "NORMAL", "normal_02": "NORMAL", "elite_01": "ELITE",
}
var msMaxAlive = map[string]int64{"NORMAL": 20, "ELITE": 2}
var msRespawnBand = map[string][2]int64{"NORMAL": {10, 14}, "ELITE": {35, 60}, "NIGHT_RARE": {240, 360}}

// msActMaps walks `# ACT N` → `## map.*` → spawn-group blocks.
func msActMaps(c *Ctx, f *File, b *SourceBinding, st *msState) {
	if st.poolCount == nil {
		st.poolCount = map[string]int{}
	}
	for _, act := range actSections(f) {
		for _, msec := range act.Children {
			mapID := msec.StableID()
			if mapID == "" {
				continue
			}
			parts := strings.Split(mapID, ".")
			regTok := ""
			if len(parts) >= 2 {
				regTok = parts[1]
			}
			for _, fb := range allFences(msec, "text") {
				var gid, aid string
				var pool []string
				var maxA, resp int64 = -1, -1
				var line int
				flush := func() {
					if gid == "" {
						return
					}
					suffix := gid[strings.LastIndex(gid, ".")+1:]
					kind := msGroupKind[suffix]
					if kind == "" {
						c.Diags.Addf(config.DiagTableSyntaxError, f.Path, line,
							"spawn group %q suffix not normal_01|normal_02|elite_01", gid)
						return
					}
					if maxA > msMaxAlive[kind] {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, line,
							"%s max_alive=%d over cap %d", gid, maxA, msMaxAlive[kind])
					}
					band := msRespawnBand[kind]
					if resp < band[0] || resp > band[1] {
						c.Diags.Addf(config.DiagValueOutOfBounds, f.Path, line,
							"%s respawn=%ds outside %d..%ds", gid, resp, band[0], band[1])
					}
					pv := make([]config.Value, 0, len(pool))
					fields := map[string]config.Value{
						"spawn_group_id":  config.VStr(gid),
						"map_id":          config.VStr(mapID),
						"anchor_id":       config.VStr(aid),
						"kind":            config.VStr(kind),
						"activation":      config.VStr("ALWAYS"),
						"max_alive":       config.VInt(maxA),
						"respawn_seconds": config.VInt(resp),
					}
					for _, tok := range pool {
						full := "monster." + regTok + "." + tok
						fields2 := config.VStr(full)
						pv = append(pv, fields2)
						st.poolCount[mapID]++
						rec := map[string]config.Value{
							"map_id":     config.VStr(mapID),
							"monster_id": fields2,
							"group":      config.VStr(gid),
						}
						if st.season0[tok] {
							rec["season_region_index"] = config.VInt(0)
						}
						c.Emit(f.Name, b.Raw, "map_pool",
							[]config.Value{config.VStr(mapID), config.VStr(gid), fields2}, rec, line)
					}
					if kind == "ELITE" && len(pool) == 2 {
						fields["selection"] = config.VStr("uniform_one_on_respawn")
					}
					fields["monster_pool"] = config.VList(pv...)
					st.groups++
					c.Emit(f.Name, b.Raw, "spawn_group",
						[]config.Value{config.VStr(gid)}, fields, line)
					c.Emit(f.Name, b.Raw, "spawn_anchor",
						[]config.Value{config.VStr(aid)},
						map[string]config.Value{
							"anchor_id": config.VStr(aid),
							"map_id":    config.VStr(mapID),
							"locator":   config.VStr("FIXED_POINT"),
							"kind":      config.VStr(kind),
						}, line)
					gid, aid, pool, maxA, resp = "", "", nil, -1, -1
				}
				for j, l := range fb.FLines {
					if m := spawnHeadRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
						flush()
						gid, aid, line = m[1], m[2], fb.Line+1+j
						continue
					}
					if m := chestFieldRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
						switch m[1] {
						case "pool":
							for _, tok := range strings.Split(m[2], ",") {
								if t := strings.TrimSpace(tok); t != "" {
									pool = append(pool, t)
								}
							}
						case "max_alive":
							v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
							maxA = v.Int
						case "respawn":
							r := strings.TrimSuffix(strings.TrimSpace(m[2]), "s")
							v, _ := (TypeSpec{Name: "int"}).ParseValue(r)
							resp = v.Int
						}
					}
				}
				flush()
			}
		}
	}
	c.consumed(f, b)
}

func msPoolRules(c *Ctx, f *File, b *SourceBinding, st *msState) {
	for _, sec := range bindingSections(c, f, b) {
		for _, bl := range sec.Content {
			for _, l := range bl.Prose {
				for _, id := range []string{"dom_dom_nguyen", "bup_lua", "tinh_buoi", "co_lua", "vong_bien", "hon_gao"} {
					if strings.Contains(l, id) {
						st.season0[id] = true
					}
				}
			}
			if bl.Kind == BlockFence {
				for j, l := range bl.FLines {
					l = strings.TrimSpace(l)
					if l != "" {
						c.EmitParam(f.Name, b.Raw, "pool_rule",
							[]config.Value{config.VInt(int64(bl.Line*1000 + j))},
							map[string]config.Value{"rule": config.VStr(l)}, bl.Line+1+j)
					}
				}
			}
		}
	}
	c.consumed(f, b)
}

func msBossPlacement(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := bossPlaceRe.FindStringSubmatch(l); m != nil {
					c.Emit(f.Name, b.Raw, "boss_anchor",
						[]config.Value{config.VStr(m[3])},
						map[string]config.Value{
							"anchor_id": config.VStr(m[3]),
							"boss_id":   config.VStr(m[1]),
							"map_id":    config.VStr(m[2]),
						}, fb.Line+1+j)
				}
			}
		}
	}
	c.consumed(f, b)
}

func msSurge(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for j, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if m := chestFieldRe.FindStringSubmatch(l); m != nil {
					c.EmitParam(f.Name, b.Raw, "surge_spawn_rule",
						[]config.Value{config.VStr(m[1])},
						map[string]config.Value{"value": config.VStr(m[2])}, fb.Line+1+j)
					continue
				}
				c.EmitParam(f.Name, b.Raw, "surge_spawn_rule",
					[]config.Value{config.VInt(int64(fb.Line*1000 + j))},
					map[string]config.Value{"rule": config.VStr(l)}, fb.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func msVerify(c *Ctx, f *File, st *msState) {
	if st.groups != 54 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"persistent spawn groups %d != 54", st.groups)
	}
	if len(st.safe) != 6 {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"safe maps %d != 6", len(st.safe))
	}
}
