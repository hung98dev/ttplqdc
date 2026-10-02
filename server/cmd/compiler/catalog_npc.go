package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileNPC — npc_shop_catalog.md driver (8 bindings): service shape,
// 18 regional + 24 ambient NPCs, guide/craft/shop services, two shops.
func compileNPC(c *Ctx, f *File, r *Registry) {
	st := &npcState{profiles: map[string][]string{}, guides: map[string][]string{}, mapNPCs: map[string][]string{}}
	c.Data["npc.state"] = st
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Shared Service Shape"):
			npcServiceShape(c, f, b, st)
		case strings.HasPrefix(path, "Regional NPCs"):
			st.wantRegional, st.hasRegionalDecl = bindingDecl(b, reDeclDashN)
			npcRegional(c, f, b, st)
		case strings.HasPrefix(path, "Ambient NPCs"):
			st.wantAmbient, st.hasAmbientDecl = bindingDecl(b, reDeclDashN)
			npcAmbient(c, f, b, st)
		case strings.HasPrefix(path, "Guide Service Contract"):
			npcGuideServices(c, f, b, st)
		case strings.HasPrefix(path, "Craft / Enhancement Service"):
			npcCraftService(c, f, b, st)
		case strings.HasPrefix(path, "Bound Utility Shop"):
			npcBoundShop(c, f, b)
		case strings.HasPrefix(path, "Shop / Account / Auction Service"):
			npcAuctionService(c, f, b, st)
		case strings.HasPrefix(path, "Shop — shop.recovery.common") || strings.HasPrefix(path, "Shop — `shop.recovery.common`"):
			npcRecoveryShop(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"npc binding %q has no driver", b.Raw)
		}
	}
	if st.hasRegionalDecl && int64(len(st.regional)) != st.wantRegional {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"regional NPCs = %d, declared %d", len(st.regional), st.wantRegional)
	}
	if want, ok := fileDeclN(c, "world_route_catalog.md", reDeclCheckpoints); ok && int64(len(st.guides)) != want {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"guide NPCs = %d, declared checkpoints %d", len(st.guides), want)
	}
	if st.hasAmbientDecl && int64(len(st.ambient)) != st.wantAmbient {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"ambient NPCs = %d, declared %d", len(st.ambient), st.wantAmbient)
	}
}

type npcState struct {
	profiles map[string][]string // role -> capability tokens
	defaults map[string]config.Value
	regional []string            // npc ids
	guides   map[string][]string // map_id -> [npc ids] by role? actually map -> 3 npcs
	mapNPCs  map[string][]string
	ambient  []string
	byRole   map[string][]string // role token -> npc ids

	wantRegional    int64
	hasRegionalDecl bool
	wantAmbient     int64
	hasAmbientDecl  bool
}

var npcArrowRe = regexp.MustCompile(`^([a-z_]+)\s*->\s*(.+)$`)
var npcCellRe = regexp.MustCompile("^`?(npc\\.[a-z0-9_.]+)`?\\s*—\\s*(.+)$")

func npcServiceShape(c *Ctx, f *File, b *SourceBinding, st *npcState) {
	st.defaults = map[string]config.Value{}
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			isRole := false
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if m := npcArrowRe.FindStringSubmatch(l); m != nil {
					isRole = true
					role := m[1]
					var caps []string
					for _, t := range strings.Split(m[2], ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							caps = append(caps, t)
						}
					}
					st.profiles[role] = caps
					c.EmitParam(f.Name, b.Raw, "npc_role_profile",
						[]config.Value{config.VStr(role)},
						map[string]config.Value{
							"role": config.VStr(role), "capabilities": config.VStrs(caps...),
						}, fb.Line+1)
					continue
				}
				if i := strings.IndexByte(l, '='); !isRole && i > 0 {
					name := strings.TrimSpace(l[:i])
					raw := strings.TrimSpace(l[i+1:])
					st.defaults[fieldName(name)] = config.VStr(raw)
					continue
				}
			}
		}
	}
	if len(st.defaults) > 0 {
		c.EmitParam(f.Name, b.Raw, "npc_defaults",
			[]config.Value{config.VStr("shared")}, st.defaults, b.Line)
	}
	c.consumed(f, b)
}

func npcRegional(c *Ctx, f *File, b *SourceBinding, st *npcState) {
	roles := []string{"nguoi_dan_duong", "tho_nghe", "hang_quan"}
	st.byRole = map[string][]string{}
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "map_id,Guide / checkpoint,Craft / enhance,Shop / account / auction"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				mapID := cellAt(row, 0).Scalar()
				for col := 1; col <= 3; col++ {
					m := npcCellRe.FindStringSubmatch(cellAt(row, col).Scalar())
					if m == nil {
						c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
							"regional NPC cell %q", cellAt(row, col).Scalar())
						continue
					}
					id, display, role := m[1], m[2], roles[col-1]
					st.regional = append(st.regional, id)
					st.mapNPCs[mapID] = append(st.mapNPCs[mapID], id)
					st.byRole[role] = append(st.byRole[role], id)
					if role == "nguoi_dan_duong" {
						st.guides[mapID] = []string{id}
					}
					emitNPC(c, f, b, st, id, mapID, role, "ALWAYS", display, row[0].Line)
				}
			}
		}
	}
	c.consumed(f, b)
}

func npcAmbient(c *Ctx, f *File, b *SourceBinding, st *npcState) {
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "map_id,DAY_ONLY,NIGHT_ONLY,PATROL"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				mapID := cellAt(row, 0).Scalar()
				for col := 1; col <= 3; col++ {
					schedule := tbl.Headers[col]
					cell := cellAt(row, col).Scalar()
					for _, idm := range regexp.MustCompile("npc\\.[a-z0-9_.]+").FindAllString(cell, -1) {
						st.ambient = append(st.ambient, idm)
						st.mapNPCs[mapID] = append(st.mapNPCs[mapID], idm)
						emitNPC(c, f, b, st, idm, mapID, "ambient", schedule, "", row[0].Line)
					}
				}
			}
		}
	}
	c.consumed(f, b)
}

func emitNPC(c *Ctx, f *File, b *SourceBinding, st *npcState, id, mapID, role, schedule, display string, line int) {
	caps := st.profiles[role]
	if role == "ambient" {
		caps = []string{"DIALOGUE", "QUEST", "DECORATIVE"}
	}
	fields := map[string]config.Value{
		"npc_id":            config.VStr(id),
		"map_id":            config.VStr(mapID),
		"role":              config.VStr(role),
		"schedule":          config.VStr(schedule),
		"capabilities":      config.VStrs(caps...),
		"movement_mode":     config.VStr("STATIC"),
		"interaction_range": config.VStr("2.5m equivalent"),
	}
	if display != "" {
		fields["display"] = config.VStr(display)
	}
	c.Emit(f.Name, b.Raw, "npc", []config.Value{config.VStr(id)}, fields, line)
}

func npcGuideServices(c *Ctx, f *File, b *SourceBinding, st *npcState) {
	var services, checkpoints []string
	travel := map[string]int64{}
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				switch {
				case strings.HasPrefix(l, "service."):
					services = append(services, l)
				case strings.HasPrefix(l, "checkpoint."):
					checkpoints = append(checkpoints, l)
				default:
					if m := regexp.MustCompile(`^T([0-9]+)\s+([0-9]+)`).FindStringSubmatch(l); m != nil {
						v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
						travel[m[1]] = v.Int
					}
				}
			}
		}
	}
	for _, s := range services {
		c.EmitParam(f.Name, b.Raw, "npc_service",
			[]config.Value{config.VStr("nguoi_dan_duong"), config.VStr(s)},
			map[string]config.Value{
				"role":       config.VStr("nguoi_dan_duong"),
				"service_id": config.VStr(s),
			}, b.Line)
	}
	cv := make([]config.Value, len(checkpoints))
	for i, cp := range checkpoints {
		cv[i] = config.VStr(cp)
	}
	c.EmitParam(f.Name, b.Raw, "npc_checkpoints",
		[]config.Value{config.VStr("list")},
		map[string]config.Value{"checkpoints": config.VList(cv...)}, b.Line)
	var tv []config.Value
	for _, k := range sortedKeys(travel) {
		tv = append(tv, config.VRec(map[string]config.Value{
			"tier": config.VStr("T" + k), "cost": config.VInt(travel[k]),
		}))
	}
	c.EmitParam(f.Name, b.Raw, "travel_cost",
		[]config.Value{config.VStr("tiers")},
		map[string]config.Value{"tiers": config.VList(tv...)}, b.Line)
	// per-guide service records (6 guides × set_checkpoint/travel/respec)
	for mapID, ids := range st.guides {
		for _, id := range ids {
			for _, s := range services {
				c.Emit(f.Name, b.Raw, "npc_service_route",
					[]config.Value{config.VStr(id), config.VStr(s)},
					map[string]config.Value{
						"npc_id":     config.VStr(id),
						"service_id": config.VStr(s),
						"map_id":     config.VStr(mapID),
					}, b.Line)
			}
		}
	}
	c.consumed(f, b)
}

func npcCraftService(c *Ctx, f *File, b *SourceBinding, st *npcState) {
	var services []string
	shop := ""
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "service.") {
					services = append(services, l)
				}
				if strings.HasPrefix(l, "shop_id") {
					if i := strings.IndexByte(l, '='); i > 0 {
						shop = strings.TrimSpace(l[i+1:])
					}
				}
			}
		}
	}
	for _, id := range st.byRole["tho_nghe"] {
		for _, s := range services {
			c.Emit(f.Name, b.Raw, "npc_service_route",
				[]config.Value{config.VStr(id), config.VStr(s)},
				map[string]config.Value{
					"npc_id":     config.VStr(id),
					"service_id": config.VStr(s),
					"shop_id":    config.VStr(shop),
				}, b.Line)
		}
	}
	c.consumed(f, b)
}

func npcBoundShop(c *Ctx, f *File, b *SourceBinding) {
	shopID := "shop.utility.bound"
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "offer_id,item_id,currency,price,source binding override"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				price, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 3).Scalar())
				c.Emit(f.Name, b.Raw, "shop_offer",
					[]config.Value{config.VStr(shopID), config.VStr(cellAt(row, 0).Scalar())},
					map[string]config.Value{
						"shop_id":         config.VStr(shopID),
						"offer_id":        config.VStr(cellAt(row, 0).Scalar()),
						"item_id":         config.VStr(cellAt(row, 1).Scalar()),
						"currency":        config.VStr(cellAt(row, 2).Scalar()),
						"price":           config.VInt(price.Int),
						"binding":         config.VStr("CHARACTER_BOUND"),
						"binding_trigger": config.VStr("ON_ACQUIRE"),
						"unlimited_stock": config.VBool(true),
					}, row[0].Line)
			}
		}
		for _, fb := range allFences(sec, "text") {
			recs := map[string]config.Value{}
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if i := strings.IndexByte(l, '='); i > 0 {
					recs[fieldName(l[:i])] = config.VStr(strings.TrimSpace(l[i+1:]))
				}
			}
			if len(recs) > 0 {
				c.EmitParam(f.Name, b.Raw, "binding_override",
					[]config.Value{config.VStr(shopID)}, recs, fb.Line+1)
			}
		}
	}
	c.consumed(f, b)
}

func npcAuctionService(c *Ctx, f *File, b *SourceBinding, st *npcState) {
	var services []string
	shop := ""
	for _, sec := range bindingSections(c, f, b) {
		for _, fb := range allFences(sec, "text") {
			for _, l := range fb.FLines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "service.") {
					services = append(services, l)
				}
				if strings.HasPrefix(l, "shop_id") {
					if i := strings.IndexByte(l, '='); i > 0 {
						shop = strings.TrimSpace(l[i+1:])
					}
				}
			}
		}
	}
	for _, id := range st.byRole["hang_quan"] {
		for _, s := range services {
			c.Emit(f.Name, b.Raw, "npc_service_route",
				[]config.Value{config.VStr(id), config.VStr(s)},
				map[string]config.Value{
					"npc_id":     config.VStr(id),
					"service_id": config.VStr(s),
					"shop_id":    config.VStr(shop),
				}, b.Line)
		}
	}
	c.EmitParam(f.Name, b.Raw, "auction_gate",
		[]config.Value{config.VStr("level")},
		map[string]config.Value{"min_level": config.VInt(15)}, b.Line)
	c.consumed(f, b)
}

func npcRecoveryShop(c *Ctx, f *File, b *SourceBinding) {
	shopID := "shop.recovery.common"
	for _, sec := range bindingSections(c, f, b) {
		if tbl := findTable(sec, "offer_id,item_id,currency,buy_price,sell_back_price"); tbl != nil {
			for ri := range tbl.Cells {
				row := tbl.Cells[ri]
				bp, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 3).Scalar())
				sp, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 4).Scalar())
				fields := map[string]config.Value{
					"shop_id":         config.VStr(shopID),
					"offer_id":        config.VStr(cellAt(row, 0).Scalar()),
					"item_id":         config.VStr(cellAt(row, 1).Scalar()),
					"currency":        config.VStr(cellAt(row, 2).Scalar()),
					"buy_price":       config.VInt(bp.Int),
					"sell_back_price": config.VInt(sp.Int),
					"unlimited_stock": config.VBool(true),
				}
				if cellAt(row, 1).Scalar() == "item.tool.can_cau_tre" {
					fields["binding"] = config.VStr("CHARACTER_BOUND")
					fields["binding_trigger"] = config.VStr("ON_ACQUIRE")
					fields["single_ownership"] = config.VBool(true)
				}
				c.Emit(f.Name, b.Raw, "shop_offer",
					[]config.Value{config.VStr(shopID), config.VStr(cellAt(row, 0).Scalar())},
					fields, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func sortedKeys(m map[string]int64) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
