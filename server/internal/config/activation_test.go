package config

// Activation gate tests (IMP-004). Fixtures are API-built minimal
// candidates — the compiler producing real bundles is cmd/compiler
// package main and is not importable here (wave-4 plan §5.3).

import (
	"fmt"
	"strings"
	"testing"
)

var testClasses = []string{"kim", "moc", "thuy", "hoa", "tho"}

func mustRat(t *testing.T, num, den int64) Value {
	t.Helper()
	v, err := VRat(num, den)
	if err != nil {
		t.Fatalf("rat: %v", err)
	}
	return v
}

func put(t *testing.T, f *Family, key []Value, fields map[string]Value) {
	t.Helper()
	if dup, _ := f.Put(key, fields); dup {
		t.Fatalf("duplicate key in %s", f.Name)
	}
}

func rec(fields map[string]Value) map[string]Value { return fields }

// validCandidate builds a minimal complete candidate that satisfies the
// whole activation check suite.
func validCandidate(t *testing.T) *CandidateSnapshot {
	t.Helper()
	defs := map[string]*Family{}
	fam := func(name string, keys ...string) *Family {
		f := NewFamily(name, keys...)
		defs[name] = f
		return f
	}

	// progression persisted-formula families
	lv := fam("level", "level")
	var cum int64
	for l := int64(1); l <= 2; l++ {
		req := 10000 * l * l
		cum += req
		put(t, lv, []Value{VInt(l)}, rec(map[string]Value{
			"level": VInt(l), "exp_required": VInt(req), "exp_cumulative": VInt(cum),
		}))
	}
	budget := fam("act_exp_budget", "act")
	for _, act := range []string{"I", "II"} {
		put(t, budget, []Value{VStr(act)}, rec(map[string]Value{
			"act": VStr(act), "level_band": VStr("1-10"), "act_exp_total": VInt(100),
		}))
	}
	portfolio := fam("portfolio", "act", "channel")
	for _, act := range []string{"I", "II"} {
		for ch, exp := range map[string]int64{"FIELD_COMBAT": 60, "STORY_ONCE": 40} {
			put(t, portfolio, []Value{VStr(act), VStr(ch)}, rec(map[string]Value{
				"act": VStr(act), "channel": VStr(ch), "exp": VInt(exp),
			}))
		}
	}
	chans := fam("exp_channel", "channel")
	for ch, pct := range map[string]int64{"FIELD_COMBAT": 60, "STORY_ONCE": 40} {
		put(t, chans, []Value{VStr(ch)}, rec(map[string]Value{
			"channel": VStr(ch), "pct_of_act_budget": VInt(pct), "target_hours": VInt(10),
		}))
	}

	// monsters: 15 NORMAL + 7 ELITE (soul sources); 3 bosses
	mon := fam("monster", "monster_id")
	drop := fam("drop_table", "drop_table_id")
	put(t, drop, []Value{VStr("drop.test.mob")}, rec(map[string]Value{"drop_table_id": VStr("drop.test.mob")}))
	put(t, drop, []Value{VStr("drop.test.boss")}, rec(map[string]Value{"drop_table_id": VStr("drop.test.boss")}))
	monsterIDs := map[string][]string{"NORMAL": {}, "ELITE": {}}
	for i := 0; i < 15; i++ {
		id := fmt.Sprintf("monster.test.n%d", i)
		monsterIDs["NORMAL"] = append(monsterIDs["NORMAL"], id)
		put(t, mon, []Value{VStr(id)}, rec(map[string]Value{
			"monster_id": VStr(id), "rank": VStr("NORMAL"), "level": VInt(1),
			"element": VStr("KIM"), "size_profile": VStr("MONSTER_SMALL"),
			"base_exp": VInt(10), "drop_table_id": VStr("drop.test.mob"),
			"act": VStr("I"), "in_launch": VBool(true), "movement": VStr("WALK"),
			"combat": VStr("MELEE"), "special": VStr("NONE"),
			"aggro_range_mm": VInt(3000), "leash_rule": VStr("configured_min_leash"),
			"min_leash_mm": VInt(5000), "attacks": VList(),
		}))
	}
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("monster.test.e%d", i)
		monsterIDs["ELITE"] = append(monsterIDs["ELITE"], id)
		put(t, mon, []Value{VStr(id)}, rec(map[string]Value{
			"monster_id": VStr(id), "rank": VStr("ELITE"), "level": VInt(20),
			"element": VStr("MOC"), "size_profile": VStr("MONSTER_ELITE"),
			"base_exp": VInt(50), "drop_table_id": VStr("drop.test.mob"),
			"act": VStr("II"), "in_launch": VBool(true), "movement": VStr("WALK"),
			"combat": VStr("MELEE"), "special": VStr("NONE"),
			"aggro_range_mm": VInt(4000), "leash_rule": VStr("configured_min_leash"),
			"min_leash_mm": VInt(6000), "attacks": VList(),
		}))
	}
	geom := NewFamily("spaces", "space_id")
	pair := func(a, b int64) Value {
		return VRec(map[string]Value{"x": mustRat(t, a, 1), "y": mustRat(t, b, 1)})
	}
	ints := func(a, b int64) Value {
		return VRec(map[string]Value{"x": VInt(a), "y": VInt(b)})
	}
	put(t, geom, []Value{VStr("space.test.field1")}, rec(map[string]Value{
		"space_id": VStr("space.test.field1"), "kind": VStr("FIELD_OR_TOWN"),
		"span_screens": pair(3, 2), "bounds_max_m": pair(50, 40),
		"reference_extent_px": ints(1280, 720),
		"layout_profile":      VStr("profile.field1"), "required_topology": VStr("MAIN_ROUTE"),
	}))
	put(t, geom, []Value{VStr("space.test.dungeon")}, rec(map[string]Value{
		"space_id": VStr("space.test.dungeon"), "kind": VStr("DUNGEON"),
		"span_screens": pair(4, 3), "bounds_max_m": pair(60, 45),
		"reference_extent_px": ints(1280, 720),
		"layout_profile":      VStr("profile.dungeon"), "required_topology": VStr("STAGES"),
	}))
	boss := fam("boss", "boss_id")
	bossIDs := []string{}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("boss.test.b%d", i)
		bossIDs = append(bossIDs, id)
		put(t, boss, []Value{VStr(id)}, rec(map[string]Value{
			"boss_id": VStr(id), "lv": VInt(60), "mode": VStr("PUBLIC"),
			"space_id": VStr("space.test.dungeon"), "size_profile": VStr("WORLD_BOSS"),
			"element": VStr("THO"), "base_hp": VInt(85600), "attack": VInt(335),
			"defense": VInt(170), "base_exp": VInt(1000),
			"scaling": VStr("PUBLIC_DEFAULT"), "drop_table_id": VStr("drop.test.boss"),
		}))
	}

	// souls: declared rank-by-element distribution (soul_catalog.md
	// § Element Count Validation)
	souls := fam("soul", "soul_id")
	soulEffs := fam("soul_effect", "soul_id", "effect_id")
	type split struct{ n, e, b int }
	dist := map[string]split{"KIM": {3, 2, 0}, "MOC": {3, 2, 0}, "THUY": {3, 1, 1}, "HOA": {3, 1, 1}, "THO": {3, 1, 1}}
	mi, ei, bi := 0, 0, 0
	for _, el := range []string{"KIM", "MOC", "THUY", "HOA", "THO"} {
		want := dist[el]
		addSoul := func(rank, src string, idx int) {
			prefix := "monster"
			if rank == "BOSS" {
				prefix = "boss"
			}
			sid := fmt.Sprintf("soul.%s.%s.%s%d", prefix, el, strings.ToLower(rank), idx)
			eid := fmt.Sprintf("effect.soul.%s%s%d", el, strings.ToLower(rank), idx)
			put(t, souls, []Value{VStr(sid)}, rec(map[string]Value{
				"soul_id": VStr(sid), "display": VStr("Hồn " + el),
				"rank": VStr(rank), "element": VStr(el),
				"source_id": VStr(src), "effect_id": VStr(eid),
				"values":    VList(mustRat(t, 1, 10), mustRat(t, 2, 10), mustRat(t, 3, 10)),
				"level_map": VList(VInt(1), VInt(1), VInt(3), VInt(3), VInt(5)),
				"icd_ms":    VInt(1000), "icd_scope": VStr("OWNER"),
			}))
			put(t, soulEffs, []Value{VStr(sid), VStr(eid)}, rec(map[string]Value{
				"soul_id": VStr(sid), "effect_id": VStr(eid), "triggers": VList(),
			}))
		}
		for k := 0; k < want.n; k++ {
			addSoul("NORMAL", monsterIDs["NORMAL"][mi], mi)
			mi++
		}
		for k := 0; k < want.e; k++ {
			addSoul("ELITE", monsterIDs["ELITE"][ei], ei)
			ei++
		}
		for k := 0; k < want.b; k++ {
			addSoul("BOSS", bossIDs[bi], bi)
			bi++
		}
	}
	fcg := fam("first_clear_grant", "boss_id")
	put(t, fcg, []Value{VStr(bossIDs[0])}, rec(map[string]Value{
		"boss_id":  VStr(bossIDs[0]),
		"soul_id":  VStr("soul.boss.THUY.boss0"),
		"key_kind": VStr("reward.first_boss_soul"),
		"key_template": VStr(
			"reward.first_boss_soul.<boss_id>.<character_id>"),
	}))

	// items + economy refs
	items := fam("item", "item_id")
	put(t, items, []Value{VStr("item.test.potion")}, rec(map[string]Value{
		"item_id": VStr("item.test.potion"), "display": VStr("Potion"),
		"identity_note": VStr("test potion"), "type": VStr("CONSUMABLE"),
		"rarity": VStr("COMMON"), "binding": VStr("NONE"),
		"binding_trigger": VStr("NEVER"), "stack_limit": VInt(99),
	}))
	put(t, items, []Value{VStr("item.test.mat")}, rec(map[string]Value{
		"item_id": VStr("item.test.mat"), "display": VStr("Material"),
		"identity_note": VNull(), "type": VStr("MATERIAL"),
		"rarity": VStr("COMMON"), "binding": VStr("NONE"),
		"binding_trigger": VStr("NEVER"), "stack_limit": VInt(999),
	}))
	equip := fam("equipment", "item_id")
	put(t, equip, []Value{VStr("equipment.test.sword")}, rec(map[string]Value{
		"item_id": VStr("equipment.test.sword"), "display": VStr("Sword"),
	}))
	recipe := fam("recipe", "recipe_id")
	put(t, recipe, []Value{VStr("recipe.test.r1")}, rec(map[string]Value{
		"recipe_id": VStr("recipe.test.r1"),
		"output_item_id": VStr(
			"equipment.test.sword"), "success_mode": VStr("GUARANTEED"),
		"output_quantity": VInt(1), "tier": VStr("T1"),
		"common_cost": VInt(10), "minimum_level": VInt(1),
	}))
	ri := fam("recipe_input", "recipe_id", "item_id")
	put(t, ri, []Value{VStr("recipe.test.r1"), VStr("item.test.mat")}, rec(map[string]Value{
		"recipe_id": VStr("recipe.test.r1"), "item_id": VStr("item.test.mat"),
		"quantity": VInt(2),
	}))
	shop := fam("shop_offer", "shop_id", "offer_id")
	put(t, shop, []Value{VStr("shop.test.s1"), VStr("offer.test.o1")}, rec(map[string]Value{
		"shop_id": VStr("shop.test.s1"), "offer_id": VStr("offer.test.o1"),
		"item_id": VStr("item.test.potion"), "currency": VStr("common"),
		"price": VInt(10), "binding": VStr("CHARACTER_BOUND"),
		"binding_trigger": VStr("ON_ACQUIRE"), "unlimited_stock": VBool(true),
	}))

	// world graph
	wm := fam("world_map", "map_id")
	put(t, wm, []Value{VStr("map.test.field")}, rec(map[string]Value{
		"map_id": VStr("map.test.field"), "type": VStr("FIELD"),
		"rec_level_lo": VInt(1), "rec_level_hi": VInt(10),
		"entry_spawn": VStr("spawn.test.entry"), "first_discovery_exp": VInt(100),
	}))
	sa := fam("spawn_anchor", "anchor_id")
	put(t, sa, []Value{VStr("anchor.test.a1")}, rec(map[string]Value{
		"anchor_id": VStr("anchor.test.a1"), "map_id": VStr("map.test.field"),
		"locator": VStr("FIXED_POINT"), "kind": VStr("FIELD"),
	}))
	sg := fam("spawn_group", "spawn_group_id")
	put(t, sg, []Value{VStr("spawn.test.g1")}, rec(map[string]Value{
		"spawn_group_id": VStr("spawn.test.g1"), "map_id": VStr("map.test.field"),
		"anchor_id": VStr("anchor.test.a1"), "kind": VStr("FIELD_NORMAL"),
		"activation": VStr("ALWAYS"), "max_alive": VInt(5),
		"respawn_seconds": VInt(60),
		"monster_pool":    VList(VStr(monsterIDs["NORMAL"][0])),
	}))
	cp := fam("checkpoint", "checkpoint_id")
	put(t, cp, []Value{VStr("checkpoint.test.c1")}, rec(map[string]Value{
		"checkpoint_id": VStr("checkpoint.test.c1"), "map_id": VStr("map.test.field"),
	}))
	portal := fam("portal", "portal_id")
	put(t, portal, []Value{VStr("portal.test.p1")}, rec(map[string]Value{
		"portal_id": VStr("portal.test.p1"), "source_map": VStr("map.test.field"),
		"destination": VStr("space.test.dungeon"),
		"destination_spawn": VStr(
			"spawn.test.entry"),
	}))
	npc := fam("npc", "npc_id")
	put(t, npc, []Value{VStr("npc.test.n1")}, rec(map[string]Value{
		"npc_id": VStr("npc.test.n1"), "map_id": VStr("map.test.field"),
	}))
	quest := fam("quest", "quest_id")
	put(t, quest, []Value{VStr("quest.test.q1")}, rec(map[string]Value{
		"quest_id": VStr("quest.test.q1"), "type": VStr("MAIN"),
		"display": VStr("Q1"), "quest_giver": VStr("npc.test.n1"),
	}))
	qo := fam("quest_object", "object_id")
	put(t, qo, []Value{VStr("quest_object.test.o1")}, rec(map[string]Value{
		"object_id": VStr("quest_object.test.o1"), "quest_id": VStr("quest.test.q1"),
		"map_id": VStr("map.test.field"),
	}))

	// skills: 4 basics + 5 actives per class (20 + 25 primary rows)
	skills := fam("skill", "skill_id")
	actions := fam("skill_action", "skill_id")
	procs := fam("basic_proc", "skill_id")
	effects := fam("skill_effect", "skill_id", "ordinal")
	templates := fam("effect_template", "effect_id")
	for _, eid := range []string{"effect.test.slow", "effect.test.shield"} {
		put(t, templates, []Value{VStr(eid)}, rec(map[string]Value{
			"effect_id": VStr(eid), "source": VStr("Canonical Basic Effect Templates"),
			"duration": VStr("3,000ms"), "reapply": VStr("REFRESH_DURATION"),
		}))
	}
	spatial := fam("spatial_effect", "spatial_effect_id")
	put(t, spatial, []Value{VStr("spatial.test.knock")}, rec(map[string]Value{
		"spatial_effect_id": VStr("spatial.test.knock"),
		"source":            VStr("skill.thuy.basic.t0"),
		"origin_shape":      VStr("connected projectile target"),
		"exact_resolution":  VStr("push 1.0m knockback in facing direction"),
		"target_cap_interaction": VStr(
			"same primary target only"),
	}))

	cdBands := map[string][2]int64{
		"kim": {520, 220}, "thuy": {650, 310}, "moc": {700, 350},
		"hoa": {800, 400}, "tho": {980, 490},
	}
	elemOf := map[string]string{
		"kim": "KIM", "moc": "MOC", "thuy": "THUY", "hoa": "HOA", "tho": "THO",
	}
	putSkill := func(id, cat, exec, tgt string, tags []string, air, mb string) {
		tv := make([]Value, len(tags))
		for i, s := range tags {
			tv[i] = VStr(s)
		}
		put(t, skills, []Value{VStr(id)}, rec(map[string]Value{
			"skill_id": VStr(id), "display": VStr(id), "unlock_level": VInt(1),
			"class_element": VStr(elemOf[skillIDRe.FindStringSubmatch(id)[1]]),
			"category":      VStr(cat), "execution_type": VStr(exec),
			"targeting_mode": VStr(tgt), "tags": VSet(tv...),
			"air_profile": VStr(air), "movement_behavior": VStr(mb),
			"damage_element": VStr(elemOf[skillIDRe.FindStringSubmatch(id)[1]]),
		}))
	}
	addAction := func(id, speed string, su, ac, re, cd int64, geo map[string]Value) {
		put(t, actions, []Value{VStr(id)}, rec(map[string]Value{
			"skill_id": VStr(id), "speed_stat": VStr(speed),
			"startup_ms": VInt(su), "active_ms": VInt(ac), "recovery_ms": VInt(re),
			"geometry": VRec(geo), "base_cd_s": mustRat(t, cd, 1000),
		}))
	}
	geomRat := func(mm int64) Value { return mustRat(t, mm, 1) }
	basicGeos := map[string]map[string]Value{
		"kim":  {"kind": VStr("MELEE_BOX"), "reach": geomRat(2000), "half_height": geomRat(900)},
		"thuy": {"kind": VStr("PROJECTILE"), "range": geomRat(8000), "speed": geomRat(11000), "radius": geomRat(250)},
		"moc":  {"kind": VStr("PROJECTILE"), "range": geomRat(7800), "speed": geomRat(10500), "radius": geomRat(280)},
		"hoa":  {"kind": VStr("PROJECTILE"), "range": geomRat(8200), "speed": geomRat(12000), "radius": geomRat(240)},
		"tho":  {"kind": VStr("MELEE_BOX"), "reach": geomRat(2500), "half_height": geomRat(900)},
	}
	activeGeos := map[string]map[string]Value{
		"kim":  {"kind": VStr("DIRECTION_BOX"), "length": geomRat(5300), "half_height": geomRat(900)},
		"thuy": {"kind": VStr("AREA_POSITION"), "cast": geomRat(7000), "radius": geomRat(3000)},
		"moc":  {"kind": VStr("AREA_SELF"), "radius": geomRat(3000)},
		"hoa":  {"kind": VStr("AREA_POSITION"), "cast": geomRat(7000), "radius": geomRat(2800)},
		"tho":  {"kind": VStr("BARRIER_POSITION"), "cast": geomRat(6500), "thickness": geomRat(800), "height": geomRat(4000), "duration": geomRat(5000)},
	}
	for _, cls := range testClasses {
		base := cdBands[cls][0]
		maxcd := cdBands[cls][1]
		for i := 0; i < 4; i++ {
			id := fmt.Sprintf("skill.%s.basic.b%d", cls, i)
			tags := []string{"BASIC_ATTACK", "DAMAGING", "STATUS_APPLY"}
			mb := "ALLOW"
			geo := map[string]Value{}
			for k, v := range basicGeos[cls] {
				geo[k] = v
			}
			if cls == "kim" && i == 1 {
				tags = append(tags, "MOVEMENT")
				mb = "FORCED"
				geo = map[string]Value{
					"kind": VStr("DASH_LINE"), "distance": geomRat(2500),
					"duration": geomRat(180), "hit_half_height": geomRat(900),
				}
			}
			if cls == "thuy" && i == 0 {
				tags = append(tags, "DISPLACEMENT")
			}
			putSkill(id, "basic", "INSTANT", "DIRECTION", tags, "ALL", mb)
			su := base * 4 / 10
			addAction(id, "ATTACK_SPEED", su, base*2/10, base-su-base*2/10, base, geo)
			procsID := id
			se := []Value{VStr("effect.test.slow")}
			if cls == "thuy" && i == 0 {
				se = append(se, VStr("spatial.test.knock"))
			}
			put(t, procs, []Value{VStr(procsID)}, rec(map[string]Value{
				"skill_id": VStr(procsID), "base_coefficient": mustRat(t, 1, 1),
				"base_cd_s": mustRat(t, base, 1000), "max_cd_s": mustRat(t, maxcd, 1000),
				"cd_step_s": mustRat(t, (base-maxcd)/11, 1000),
				"base_proc": mustRat(t, 8, 100), "max_proc": mustRat(t, 20, 100),
				"status_effects": VList(se...), "note": VStr("n"),
			}))
		}
		for i := 0; i < 5; i++ {
			id := fmt.Sprintf("skill.%s.active.a%d", cls, i)
			tags := []string{"DAMAGING"}
			mb := "ALLOW"
			var geo map[string]Value
			var payload map[string]Value
			switch {
			case cls == "tho" && i == 0:
				id = "skill.tho.active.son_bich"
				tags = []string{"DEFENSIVE"}
				geo = activeGeos["tho"]
				payload = map[string]Value{
					"kind": VStr("BARRIER"), "geometry_ref": VStr("PRIMARY"),
					"block_enemy_movement": VBool(true), "block_projectiles": VBool(true),
				}
			case i == 0:
				tags = append(tags, "SIGNATURE")
				geo = activeGeos[cls]
				payload = map[string]Value{
					"kind": VStr("DAMAGE"), "coefficient": mustRat(t, 3, 1),
				}
			case i == 1:
				if cls == "tho" {
					tags = append(tags, "SIGNATURE")
				}
				geo = map[string]Value{
					"kind": VStr("AREA_SELF"), "radius": geomRat(3000),
				}
				payload = map[string]Value{
					"kind": VStr("DAMAGE"), "coefficient": mustRat(t, 2, 1),
				}
			case i == 2:
				geo = map[string]Value{
					"kind": VStr("SINGLE_TARGET_RANGE"), "range": geomRat(2000),
				}
				payload = map[string]Value{
					"kind": VStr("EXECUTE"), "hp_threshold": mustRat(t, 15, 100),
					"multiplier": mustRat(t, 3, 1),
				}
			case i == 3:
				geo = map[string]Value{
					"kind": VStr("AREA_POSITION"), "cast": geomRat(7000),
					"radius": geomRat(2600),
				}
				payload = map[string]Value{
					"kind": VStr("HEAL"), "target_max_hp_ratio": mustRat(t, 1, 10),
					"attack_coefficient": mustRat(t, 1, 2),
				}
				tags = []string{"HEAL"}
			default:
				geo = map[string]Value{
					"kind": VStr("DASH_LINE"), "distance": geomRat(4200),
					"duration": geomRat(300), "hit_half_height": geomRat(900),
				}
				payload = map[string]Value{
					"kind": VStr("DAMAGE"), "coefficient": mustRat(t, 25, 10),
				}
				tags = append(tags, "MOVEMENT")
				mb = "FORCED"
			}
			putSkill(id, "active", "INSTANT", "AREA", tags, "ALL", mb)
			addAction(id, "CAST_SPEED", 300, 200, 300, 8000, geo)
			put(t, effects, []Value{VStr(id), VInt(0)}, rec(map[string]Value{
				"skill_id": VStr(id), "ordinal": VInt(0), "payload": VRec(payload),
			}))
		}
		// one STATUS payload active per class for assertion-19 coverage
		id := fmt.Sprintf("skill.%s.active.a1", cls)
		put(t, effects, []Value{VStr(id), VInt(1)}, rec(map[string]Value{
			"skill_id": VStr(id), "ordinal": VInt(1),
			"payload": VRec(map[string]Value{
				"kind": VStr("STATUS"), "effect_id": VStr("effect.test.slow"),
			}),
		}))
		// tags must carry STATUS_APPLY for that payload
		sk := skills.Records[KeyString([]Value{VStr(id)})]
		tl := []Value{VStr("DAMAGING"), VStr("STATUS_APPLY")}
		if cls == "tho" {
			tl = append(tl, VStr("SIGNATURE"))
		}
		sk.Fields["tags"] = VSet(tl...)
		skills.Records[KeyString([]Value{VStr(id)})] = sk
	}
	ts := fam("target_scaling", "group")
	for gi, cls := range testClasses {
		members := make([]Value, 0, 5)
		for i := 0; i < 5; i++ {
			id := fmt.Sprintf("skill.%s.active.a%d", cls, i)
			if cls == "tho" && i == 0 {
				id = "skill.tho.active.son_bich"
			}
			members = append(members, VStr(id))
		}
		put(t, ts, []Value{VStr(fmt.Sprintf("group.test.%d", gi))}, rec(map[string]Value{
			"group": VStr(fmt.Sprintf("group.test.%d", gi)), "members": VList(members...),
			"tiers": VList(VRec(map[string]Value{
				"band": VStr("1..5"), "monster_targets": VInt(2),
				"player_targets": VInt(1),
			})),
		}))
	}

	params := NewFamily("validation_parameters", "family")
	put(t, params, []Value{VStr("act_exp_total")}, rec(map[string]Value{
		"family": VStr("act_exp_total"), "key_columns": VStrs("act"),
		"records": VList(VRec(map[string]Value{
			"key":    VList(VStr("TOTAL")),
			"fields": VRec(map[string]Value{"exp": VInt(200)}),
		})),
	}))

	c := &CandidateSnapshot{
		AuthoringSchemaVersion: AuthoringSchemaVersion,
		ContentSchemaVersion:   ContentSchemaVersion,
		RuleVersions: map[string]int{
			"item_definition_strings": 1,
			"soul_element_bindings":   1,
			"skill_barrier_payload":   1,
		},
		Definitions:          defs,
		ValidationParameters: params,
		Geometry:             geom,
	}
	recompute(t, c)
	return c
}

func recompute(t *testing.T, c *CandidateSnapshot) {
	t.Helper()
	payload, err := CanonicalBytes(CanonicalPayload(c))
	if err != nil {
		t.Fatalf("canonical payload: %v", err)
	}
	rev, err := ComputeContentRevision(payload)
	if err != nil {
		t.Fatalf("revision: %v", err)
	}
	c.ContentRevision = rev
}

func mustActivate(t *testing.T, g *Gate, c *CandidateSnapshot, m ActivationMeta) {
	t.Helper()
	if d := g.Activate(c, m); d.HasErrors() {
		t.Fatalf("activate rejected: %v", d)
	}
}

func mustReject(t *testing.T, g *Gate, c *CandidateSnapshot, m ActivationMeta) Diagnostics {
	t.Helper()
	d := g.Activate(c, m)
	if !d.HasErrors() {
		t.Fatalf("candidate must reject")
	}
	return d
}

// variantCandidate returns a candidate differing from validCandidate only
// by an item display string — a distinct, valid, non-schema-coupled
// revision (same persisted-formula outputs).
func variantCandidate(t *testing.T, tag string) *CandidateSnapshot {
	t.Helper()
	c := validCandidate(t)
	r := c.Definitions["item"].Records[KeyString([]Value{VStr("item.test.mat")})]
	r.Fields["display"] = VStr("Material " + tag)
	c.Definitions["item"].Records[KeyString([]Value{VStr("item.test.mat")})] = r
	recompute(t, c)
	return c
}

// cappedVariant returns a candidate with the level cap raised by one valid
// row — a persisted-formula output change (level cap / EXP table) that
// meets the schema-coupled criteria vs validCandidate.
func cappedVariant(t *testing.T, tag string) *CandidateSnapshot {
	t.Helper()
	c := variantCandidate(t, tag)
	f := c.Definitions["level"]
	put(t, f, []Value{VInt(3)}, rec(map[string]Value{
		"level": VInt(3), "exp_required": VInt(90000), "exp_cumulative": VInt(140000),
	}))
	recompute(t, c)
	return c
}

// ---- retention lifecycle --------------------------------------------------

func TestActivationRetainsNonPreviousPinnedRevision(t *testing.T) {
	g := NewGate()
	a := validCandidate(t)
	mustActivate(t, g, a, ActivationMeta{})
	revA := a.ContentRevision
	if d := g.Pin(revA, PinRef{Kind: PinCommand, ID: "cmd-1"}); d.HasErrors() {
		t.Fatalf("pin: %v", d)
	}
	b := cappedVariant(t, "v2")
	mustActivate(t, g, b, ActivationMeta{SchemaCoupled: true})
	// same persisted-formula outputs as B -> non-coupled, no tag needed
	c := cappedVariant(t, "v3")
	mustActivate(t, g, c, ActivationMeta{})
	// A is neither active nor previous but must be retained via the pin.
	if g.revisions[revA] == nil {
		t.Fatalf("pinned non-previous revision %s released", revA)
	}
	if g.ActiveRevision() != c.ContentRevision {
		t.Fatalf("active = %s, want %s", g.ActiveRevision(), c.ContentRevision)
	}
}

func TestRollbackRetainsOriginalBossRewardRevision(t *testing.T) {
	g := NewGate()
	a := validCandidate(t)
	mustActivate(t, g, a, ActivationMeta{})
	revA := a.ContentRevision
	if d := g.Pin(revA, PinRef{Kind: PinBossReward, ID: "reward-slot-1"}); d.HasErrors() {
		t.Fatalf("pin: %v", d)
	}
	b := cappedVariant(t, "v2")
	mustActivate(t, g, b, ActivationMeta{SchemaCoupled: true})
	// B is schema-coupled: binary rollback is refused and A stays pinned.
	if d := g.Rollback(); !d.HasErrors() {
		t.Fatalf("schema-coupled rollback must refuse")
	}
	if g.revisions[revA] == nil {
		t.Fatalf("pinned boss-reward revision %s released", revA)
	}
	if g.ActiveRevision() != b.ContentRevision {
		t.Fatalf("active = %s after refused rollback", g.ActiveRevision())
	}
	// A fresh non-coupled activation lets rollback proceed; the original
	// reward revision remains retained through its pin.
	c := cappedVariant(t, "v3")
	mustActivate(t, g, c, ActivationMeta{})
	if d := g.Rollback(); d.HasErrors() {
		t.Fatalf("rollback: %v", d)
	}
	if g.revisions[revA] == nil {
		t.Fatalf("pinned boss-reward revision %s released after rollback", revA)
	}
}

func TestRevisionReleasedOnlyAfterLastRecoveryReferenceDisposed(t *testing.T) {
	g := NewGate()
	a := validCandidate(t)
	mustActivate(t, g, a, ActivationMeta{})
	revA := a.ContentRevision
	g.Pin(revA, PinRef{Kind: PinCommand, ID: "cmd-1"})
	g.Pin(revA, PinRef{Kind: PinCommand, ID: "cmd-2"})
	mustActivate(t, g, cappedVariant(t, "v2"), ActivationMeta{SchemaCoupled: true})
	mustActivate(t, g, cappedVariant(t, "v3"), ActivationMeta{})
	g.Dispose("cmd-1")
	if g.revisions[revA] == nil {
		t.Fatalf("revision %s released before last disposition", revA)
	}
	g.Dispose("cmd-2")
	if g.revisions[revA] != nil {
		t.Fatalf("revision %s retained after last disposition", revA)
	}
}

// ---- atomic activation ----------------------------------------------------

func TestAtomicActivationLifecycle(t *testing.T) {
	g := NewGate()
	a := validCandidate(t)
	mustActivate(t, g, a, ActivationMeta{})
	if g.Active() != a || g.ActiveRevision() != a.ContentRevision {
		t.Fatalf("first activation did not install candidate")
	}
	b := cappedVariant(t, "v2")
	mustActivate(t, g, b, ActivationMeta{SchemaCoupled: true})
	if g.ActiveRevision() != b.ContentRevision {
		t.Fatalf("second activation did not swap active")
	}
	// B is schema-coupled — rollback must refuse.
	if d := g.Rollback(); !d.HasErrors() {
		t.Fatalf("schema-coupled active must refuse rollback")
	}
	// plain rollback restores the previous revision
	g3 := NewGate()
	x := validCandidate(t)
	mustActivate(t, g3, x, ActivationMeta{})
	y := variantCandidate(t, "v2")
	mustActivate(t, g3, y, ActivationMeta{})
	if d := g3.Rollback(); d.HasErrors() {
		t.Fatalf("rollback: %v", d)
	}
	if g3.ActiveRevision() != x.ContentRevision {
		t.Fatalf("rollback did not restore previous revision")
	}
}

func TestInvalidRevisionPreservesPrevious(t *testing.T) {
	g := NewGate()
	a := validCandidate(t)
	mustActivate(t, g, a, ActivationMeta{})
	bad := validCandidate(t)
	delete(bad.Definitions, "level")
	recompute(t, bad)
	if d := g.Activate(bad, ActivationMeta{}); !d.HasErrors() {
		t.Fatalf("invalid candidate must reject")
	}
	if g.ActiveRevision() != a.ContentRevision || g.Active() != a {
		t.Fatalf("rejected candidate disturbed active revision")
	}
}

// ---- activation rejection probes ------------------------------------------

func TestSpatialGeometryIntegrationRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	r := c.Geometry.Records[KeyString([]Value{VStr("space.test.field1")})]
	delete(r.Fields, "layout_profile")
	c.Geometry.Records[KeyString([]Value{VStr("space.test.field1")})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
	if g.Active() != nil {
		t.Fatalf("rejected spatial candidate installed")
	}
}

func TestSkillReachActivationRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	// thuy basic projectile range out of the 7.5..8.5m band
	ar := c.Definitions["skill_action"].Records[KeyString([]Value{VStr("skill.thuy.basic.b0")})]
	ar.Fields["geometry"] = VRec(map[string]Value{
		"kind": VStr("PROJECTILE"), "range": mustRat(t, 9500, 1),
		"speed": mustRat(t, 11000, 1), "radius": mustRat(t, 250, 1),
	})
	c.Definitions["skill_action"].Records[KeyString([]Value{VStr("skill.thuy.basic.b0")})] = ar
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

func TestSkillSecondaryGeometryRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	r := c.Definitions["spatial_effect"].Records[KeyString([]Value{VStr("spatial.test.knock")})]
	r.Fields["exact_resolution"] = VStr("")
	c.Definitions["spatial_effect"].Records[KeyString([]Value{VStr("spatial.test.knock")})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

func TestSkillDisplacementTagRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	// tag DISPLACEMENT without a forced-position/AIRBORNE result
	r := c.Definitions["skill"].Records[KeyString([]Value{VStr("skill.kim.basic.b0")})]
	r.Fields["tags"] = VSet(VStr("BASIC_ATTACK"), VStr("DAMAGING"), VStr("STATUS_APPLY"), VStr("DISPLACEMENT"))
	c.Definitions["skill"].Records[KeyString([]Value{VStr("skill.kim.basic.b0")})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

func TestSkillCooldownBandAndPhaseSumRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	// kim basic Lv12 cooldown outside the 0.20..0.24 band
	r := c.Definitions["basic_proc"].Records[KeyString([]Value{VStr("skill.kim.basic.b0")})]
	r.Fields["max_cd_s"] = mustRat(t, 400, 1000)
	c.Definitions["basic_proc"].Records[KeyString([]Value{VStr("skill.kim.basic.b0")})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

func TestSkillStatusTemplateCompletenessRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	// STATUS payload effect_id not present in effect_template
	r := c.Definitions["skill_effect"].Records[KeyString([]Value{VStr("skill.kim.active.a1"), VInt(1)})]
	r.Fields["payload"] = VRec(map[string]Value{
		"kind": VStr("STATUS"), "effect_id": VStr("effect.test.missing"),
	})
	c.Definitions["skill_effect"].Records[KeyString([]Value{VStr("skill.kim.active.a1"), VInt(1)})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

func TestSkillDamageElementRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	r := c.Definitions["skill"].Records[KeyString([]Value{VStr("skill.hoa.active.a2")})]
	r.Fields["damage_element"] = VStr("THUY")
	c.Definitions["skill"].Records[KeyString([]Value{VStr("skill.hoa.active.a2")})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

func TestSkillTagTargetGroupRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	// drop one active from its target_scaling group
	r := c.Definitions["target_scaling"].Records[KeyString([]Value{VStr("group.test.0")})]
	r.Fields["members"] = VList(VStr("skill.kim.active.a0"))
	c.Definitions["target_scaling"].Records[KeyString([]Value{VStr("group.test.0")})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

func TestHardBalanceFailureRejection(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	// exp_required(L=2) != 10000*L*L
	r := c.Definitions["level"].Records[KeyString([]Value{VInt(2)})]
	r.Fields["exp_required"] = VInt(41000)
	c.Definitions["level"].Records[KeyString([]Value{VInt(2)})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}

// ---- CAT-001/002/003 --------------------------------------------------------

func TestSoulElementCandidateRejectionPreservesPrevious(t *testing.T) {
	g := NewGate()
	a := validCandidate(t)
	mustActivate(t, g, a, ActivationMeta{})
	bad := validCandidate(t)
	// CAT-001: explicit non-NONE element required; mutating one soul's
	// element also breaks the declared rank-by-element distribution.
	for k, r := range bad.Definitions["soul"].Records {
		if fieldStr(r, "element") == "KIM" {
			r.Fields["element"] = VStr("NONE")
			bad.Definitions["soul"].Records[k] = r
			break
		}
	}
	recompute(t, bad)
	if d := g.Activate(bad, ActivationMeta{}); !d.HasErrors() {
		t.Fatalf("CAT-001 violation must reject")
	}
	if g.ActiveRevision() != a.ContentRevision {
		t.Fatalf("rejected soul candidate disturbed previous revision")
	}
}

func TestBarrierPayloadCandidateRejectionPreservesPrevious(t *testing.T) {
	g := NewGate()
	a := validCandidate(t)
	mustActivate(t, g, a, ActivationMeta{})
	bad := validCandidate(t)
	// CAT-002/assertion 22-23: BARRIER must be the sole closed
	// primary_geometry variant paired with a BARRIER_POSITION geometry.
	key := KeyString([]Value{VStr("skill.tho.active.son_bich"), VInt(0)})
	r := bad.Definitions["skill_effect"].Records[key]
	r.Fields["payload"] = VRec(map[string]Value{
		"kind": VStr("BARRIER"), "geometry_ref": VStr("SECONDARY"),
		"block_enemy_movement": VBool(true), "block_projectiles": VBool(true),
	})
	bad.Definitions["skill_effect"].Records[key] = r
	recompute(t, bad)
	if d := g.Activate(bad, ActivationMeta{}); !d.HasErrors() {
		t.Fatalf("CAT-002 violation must reject")
	}
	if g.ActiveRevision() != a.ContentRevision {
		t.Fatalf("rejected barrier candidate disturbed previous revision")
	}
}

func TestIncompleteRegisteredItemStringPayloadRejectsActivation(t *testing.T) {
	g := NewGate()
	c := validCandidate(t)
	// CAT-003: registered item strings must emit the complete payload
	r := c.Definitions["item"].Records[KeyString([]Value{VStr("item.test.potion")})]
	delete(r.Fields, "identity_note")
	c.Definitions["item"].Records[KeyString([]Value{VStr("item.test.potion")})] = r
	recompute(t, c)
	mustReject(t, g, c, ActivationMeta{})
}
