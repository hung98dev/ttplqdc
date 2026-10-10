package balance

import (
	"strings"
	"testing"

	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
)

// ---------------------------------------------------------------------
// synthetic compiled candidate
// ---------------------------------------------------------------------

// The fixture mirrors the compiled catalog shapes with magnitudes chosen
// so every guardrail passes deterministically (fixture A), while fixture
// B marginal/max-stress draws stay inside the same windows.

var tierFlats = map[string][4]int64{ // ATTACK, DEFENSE, MAX_HP, MAX_MP flat sums per tier
	"T1": {21, 30, 290, 55},
	"T2": {35, 50, 480, 110},
	"T3": {40, 80, 800, 200},
	"T4": {115, 110, 1200, 240},
	"T5": {120, 150, 1400, 270},
	"T6": {186, 143, 800, 287},
}

var slotLayout = []struct {
	slot string
	stat string
	pct  int64
}{
	{"weapon", "ATTACK", 60}, {"offhand", "ATTACK", 40},
	{"head", "DEFENSE", 20}, {"shoulder", "DEFENSE", 20},
	{"chest", "DEFENSE", 25}, {"legs", "DEFENSE", 20},
	{"boots", "DEFENSE", 15},
	{"belt", "MAX_HP", 25}, {"ring1", "MAX_HP", 25},
	{"ring2", "MAX_HP", 25}, {"amulet", "MAX_HP", 25},
	{"charm", "MAX_MP", 40}, {"token", "MAX_MP", 30},
	{"relic", "MAX_MP", 30},
}

var slotEnhanceable = map[string][]string{
	"weapon":  {"ATTACK", "ATTACK_SPEED", "CRIT_CHANCE", "LIFESTEAL"},
	"offhand": {"ATTACK", "ATTACK_SPEED", "CRIT_CHANCE", "LIFESTEAL"},
	"head":    {"DEFENSE", "MAX_HP", "REFLECT"}, "shoulder": {"DEFENSE", "MAX_HP", "REFLECT"},
	"chest": {"DEFENSE", "MAX_HP", "REFLECT"}, "legs": {"DEFENSE", "MAX_HP", "REFLECT"},
	"boots": {"DEFENSE", "ABSORB"}, "belt": {"DEFENSE", "ABSORB"},
	"ring1": {"MAX_HP", "HEAL_REDUCTION"}, "ring2": {"MAX_HP", "HEAL_REDUCTION"},
	"amulet": {"MAX_HP", "HEAL_REDUCTION"},
	"charm":  {"MAX_MP", "CAST_SPEED", "COOLDOWN_REDUCTION"},
	"token":  {"MAX_MP", "CAST_SPEED", "COOLDOWN_REDUCTION"},
	"relic":  {"MAX_MP", "CAST_SPEED", "COOLDOWN_REDUCTION"},
}

var tierOrder = []string{"T1", "T2", "T3", "T4", "T5", "T6"}

func mustRat(t *testing.T, n, d int64) config.Value {
	t.Helper()
	v, err := config.VRat(n, d)
	if err != nil {
		t.Fatalf("VRat(%d,%d): %v", n, d, err)
	}
	return v
}

func put(t *testing.T, f *config.Family, key []config.Value, fields map[string]config.Value) {
	t.Helper()
	if dup, _ := f.Put(key, fields); dup {
		t.Fatalf("duplicate key in %s", f.Name)
	}
}

func fam(defs map[string]*config.Family, name string, keys ...string) *config.Family {
	f := config.NewFamily(name, keys...)
	defs[name] = f
	return f
}

// rollRanges: per roll type, per tier {{loN,loD},{hiN,hiD}} rational pairs.
var rollRanges = map[string]map[string][2][2]int64{
	"attack_flat":             {"T1": {{1, 1}, {1, 1}}, "T2": {{1, 1}, {1, 1}}, "T3": {{2, 1}, {3, 1}}, "T4": {{3, 1}, {4, 1}}, "T5": {{4, 1}, {5, 1}}, "T6": {{6, 1}, {8, 1}}},
	"defense_flat":            {"T1": {{1, 1}, {2, 1}}, "T2": {{1, 1}, {3, 1}}, "T3": {{2, 1}, {4, 1}}, "T4": {{3, 1}, {5, 1}}, "T5": {{3, 1}, {5, 1}}, "T6": {{4, 1}, {6, 1}}},
	"max_hp_flat":             {"T1": {{5, 1}, {10, 1}}, "T2": {{10, 1}, {20, 1}}, "T3": {{15, 1}, {25, 1}}, "T4": {{20, 1}, {30, 1}}, "T5": {{25, 1}, {35, 1}}, "T6": {{30, 1}, {40, 1}}},
	"max_mp_flat":             {"T1": {{5, 1}, {10, 1}}, "T2": {{10, 1}, {20, 1}}, "T3": {{15, 1}, {25, 1}}, "T4": {{20, 1}, {30, 1}}, "T5": {{25, 1}, {35, 1}}, "T6": {{30, 1}, {40, 1}}},
	"crit_chance_roll":        {"T1": {{1, 1000}, {4, 1000}}, "T2": {{1, 1000}, {4, 1000}}, "T3": {{1, 1000}, {4, 1000}}, "T4": {{1, 1000}, {4, 1000}}, "T5": {{1, 1000}, {4, 1000}}, "T6": {{1, 1000}, {4, 1000}}},
	"attack_speed_roll":       {"T1": {{2, 1000}, {10, 1000}}, "T2": {{2, 1000}, {10, 1000}}, "T3": {{2, 1000}, {10, 1000}}, "T4": {{2, 1000}, {10, 1000}}, "T5": {{2, 1000}, {10, 1000}}, "T6": {{2, 1000}, {10, 1000}}},
	"cast_speed_roll":         {"T1": {{2, 1000}, {10, 1000}}, "T2": {{2, 1000}, {10, 1000}}, "T3": {{2, 1000}, {10, 1000}}, "T4": {{2, 1000}, {10, 1000}}, "T5": {{2, 1000}, {10, 1000}}, "T6": {{2, 1000}, {10, 1000}}},
	"cooldown_reduction_roll": {"T1": {{2, 1000}, {10, 1000}}, "T2": {{2, 1000}, {10, 1000}}, "T3": {{2, 1000}, {10, 1000}}, "T4": {{2, 1000}, {10, 1000}}, "T5": {{2, 1000}, {10, 1000}}, "T6": {{2, 1000}, {10, 1000}}},
	"lifesteal_roll":          {"T1": {{1, 10000}, {9, 10000}}, "T2": {{1, 10000}, {9, 10000}}, "T3": {{1, 10000}, {9, 10000}}, "T4": {{1, 10000}, {9, 10000}}, "T5": {{1, 10000}, {9, 10000}}, "T6": {{1, 10000}, {9, 10000}}},
	"reflect_roll":            {"T1": {{1, 10000}, {17, 10000}}, "T2": {{1, 10000}, {17, 10000}}, "T3": {{1, 10000}, {17, 10000}}, "T4": {{1, 10000}, {17, 10000}}, "T5": {{1, 10000}, {17, 10000}}, "T6": {{1, 10000}, {17, 10000}}},
	"absorb_roll":             {"T1": {{1, 10000}, {11, 10000}}, "T2": {{1, 10000}, {11, 10000}}, "T3": {{1, 10000}, {11, 10000}}, "T4": {{1, 10000}, {11, 10000}}, "T5": {{1, 10000}, {11, 10000}}, "T6": {{1, 10000}, {11, 10000}}},
	"heal_reduction_roll":     {"T1": {{1, 10000}, {35, 10000}}, "T2": {{1, 10000}, {35, 10000}}, "T3": {{1, 10000}, {35, 10000}}, "T4": {{1, 10000}, {35, 10000}}, "T5": {{1, 10000}, {35, 10000}}, "T6": {{1, 10000}, {35, 10000}}},
}

func rollRange(rid, tier string) (lo, hi config.Rat) {
	r := rollRanges[rid][tier]
	return config.Rat{Num: r[0][0], Den: r[0][1]}, config.Rat{Num: r[1][0], Den: r[1][1]}
}

var rollStats = map[string]string{
	"attack_flat": "ATTACK", "defense_flat": "DEFENSE", "max_hp_flat": "MAX_HP",
	"max_mp_flat": "MAX_MP", "crit_chance_roll": "CRIT_CHANCE",
	"attack_speed_roll": "ATTACK_SPEED", "cast_speed_roll": "CAST_SPEED",
	"cooldown_reduction_roll": "COOLDOWN_REDUCTION",
	"lifesteal_roll":          "LIFESTEAL", "reflect_roll": "REFLECT",
	"absorb_roll": "ABSORB", "heal_reduction_roll": "HEAL_REDUCTION",
}

// skill geometry matrix: id -> (kind, category, geometry fields).
type skillSpec struct {
	kind, cat string
	fields    map[string]config.Value
	cost      int64
	cd        int64 // ms
	cdRat     [2]int64
	coef      int64 // damage coefficient num/100
	tags      []string
}

func skillTable() []skillSpec {
	g := func(m map[string]int64) map[string]config.Value {
		out := map[string]config.Value{}
		for k, v := range m {
			out[k] = config.VInt(v)
		}
		return out
	}
	specs := []skillSpec{
		// KIM basics
		{"MELEE_BOX", "basic", g(map[string]int64{"kind": -1, "reach": 1900}), 0, 0, [2]int64{500, 1000}, 100, nil},
		{"MELEE_BOX", "basic", g(map[string]int64{"reach": 2400}), 0, 0, [2]int64{550, 1000}, 100, nil},
		{"DIRECTION_BOX", "basic", g(map[string]int64{"length": 3200, "width": 600}), 0, 0, [2]int64{600, 1000}, 100, nil},
		{"DASH_LINE", "basic", g(map[string]int64{"distance": 2400, "width": 500}), 0, 0, [2]int64{650, 1000}, 100, nil},
		// MOC basics
		{"PROJECTILE", "basic", g(map[string]int64{"range": 7500, "radius": 250}), 0, 0, [2]int64{700, 1000}, 90, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8000, "radius": 300}), 0, 0, [2]int64{700, 1000}, 90, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8500, "radius": 250}), 0, 0, [2]int64{750, 1000}, 90, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8000, "radius": 220}), 0, 0, [2]int64{800, 1000}, 90, nil},
		// THUY basics
		{"PROJECTILE", "basic", g(map[string]int64{"range": 7500, "radius": 250}), 0, 0, [2]int64{650, 1000}, 92, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8000, "radius": 300}), 0, 0, [2]int64{700, 1000}, 92, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8200, "radius": 280}), 0, 0, [2]int64{750, 1000}, 92, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8500, "radius": 300}), 0, 0, [2]int64{800, 1000}, 92, nil},
		// HOA basics
		{"PROJECTILE", "basic", g(map[string]int64{"range": 7500, "radius": 300}), 0, 0, [2]int64{800, 1000}, 95, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 7800, "radius": 280}), 0, 0, [2]int64{850, 1000}, 95, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8000, "radius": 250}), 0, 0, [2]int64{900, 1000}, 95, nil},
		{"PROJECTILE", "basic", g(map[string]int64{"range": 8200, "radius": 300}), 0, 0, [2]int64{950, 1000}, 95, nil},
		// THO basics
		{"MELEE_BOX", "basic", g(map[string]int64{"reach": 2000}), 0, 0, [2]int64{950, 1000}, 105, nil},
		{"MELEE_BOX", "basic", g(map[string]int64{"reach": 2200}), 0, 0, [2]int64{1000, 1000}, 105, nil},
		{"MELEE_BOX", "basic", g(map[string]int64{"reach": 2300}), 0, 0, [2]int64{1050, 1000}, 105, nil},
		{"DIRECTION_BOX", "basic", g(map[string]int64{"length": 3000, "width": 800}), 0, 0, [2]int64{1100, 1000}, 105, nil},
		// KIM actives
		{"SINGLE_TARGET_RANGE", "active", g(map[string]int64{"range": 2600}), 12, 8000, [2]int64{}, 220, nil},
		{"AREA_SELF", "active", g(map[string]int64{"radius": 2800}), 16, 12000, [2]int64{}, 160, nil},
		{"MELEE_BOX", "active", g(map[string]int64{"reach": 2500}), 18, 15000, [2]int64{}, 180, nil},
		{"AREA_SELF", "active", g(map[string]int64{"radius": 3200}), 24, 20000, [2]int64{}, 0, []string{"DEFENSIVE"}},
		{"DASH_LINE", "active", g(map[string]int64{"distance": 4200, "width": 600}), 35, 9000, [2]int64{}, 140, nil},
		// MOC actives
		{"SINGLE_TARGET_RANGE", "active", g(map[string]int64{"range": 7500}), 36, 25000, [2]int64{}, 0, []string{"HEAL"}},
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7000, "radius": 2500}), 20, 14000, [2]int64{}, 150, nil},
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7500, "radius": 2800}), 22, 18000, [2]int64{}, 170, nil},
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7000, "radius": 3000}), 16, 12000, [2]int64{}, 140, nil},
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7000, "radius": 3200}), 30, 22000, [2]int64{}, 190, nil},
		// THUY actives
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7500, "radius": 3500}), 38, 28000, [2]int64{}, 260, nil},
		{"DIRECTION_BOX", "active", g(map[string]int64{"length": 5500, "width": 800}), 24, 16000, [2]int64{}, 190, nil},
		{"MOVE_CONTACT_LINE", "active", g(map[string]int64{"distance": 4500, "duration_ms": 1200, "width": 600}), 22, 18000, [2]int64{}, 180, []string{"DISPLACEMENT"}},
		{"SELF", "active", g(map[string]int64{}), 20, 20000, [2]int64{}, 0, nil},
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7000, "radius": 2600}), 18, 14000, [2]int64{}, 150, nil},
		// HOA actives
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7500, "radius": 3000}), 40, 26000, [2]int64{}, 240, nil},
		{"AREA_SELF", "active", g(map[string]int64{"radius": 3500}), 28, 20000, [2]int64{}, 180, nil},
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7000, "radius": 2800}), 18, 14000, [2]int64{}, 150, nil},
		{"DASH_LINE", "active", g(map[string]int64{"distance": 4200, "width": 600}), 35, 9000, [2]int64{}, 130, nil},
		{"SELF", "active", g(map[string]int64{}), 20, 18000, [2]int64{}, 0, nil},
		// THO actives
		{"MELEE_BOX", "active", g(map[string]int64{"reach": 2400}), 14, 10000, [2]int64{}, 160, nil},
		{"AREA_SELF", "active", g(map[string]int64{"radius": 3000}), 30, 24000, [2]int64{}, 0, []string{"DEFENSIVE"}},
		{"SINGLE_TARGET_RANGE", "active", g(map[string]int64{"range": 2600}), 20, 15000, [2]int64{}, 200, []string{"DISPLACEMENT"}},
		{"AREA_POSITION", "active", g(map[string]int64{"cast": 7000, "radius": 3000}), 26, 22000, [2]int64{}, 200, nil},
		{"MELEE_BOX", "active", g(map[string]int64{"reach": 2500}), 18, 12000, [2]int64{}, 170, nil},
	}
	return specs
}

var skillIDs = []string{
	// basics (20)
	"skill.kim.basic.kiem_thuc", "skill.kim.basic.kiem_tam",
	"skill.kim.basic.xuyen_ha", "skill.kim.basic.phong_loi",
	"skill.moc.basic.linh_diep", "skill.moc.basic.diep_thu",
	"skill.moc.basic.linh_tam", "skill.moc.basic.moc_tiem",
	"skill.thuy.basic.thuy_tien", "skill.thuy.basic.thuy_lan",
	"skill.thuy.basic.thuy_dao", "skill.thuy.basic.thuy_quyen",
	"skill.hoa.basic.hoa_phu", "skill.hoa.basic.hoa_tiem",
	"skill.hoa.basic.hoa_dan", "skill.hoa.basic.hoa_tien",
	"skill.tho.basic.tran_quyen", "skill.tho.basic.tho_quyen",
	"skill.tho.basic.tho_dap", "skill.tho.basic.tho_ngach",
	// actives (25)
	"skill.kim.active.nhat_kiem_dinh_hon", "skill.kim.active.kiem_tran",
	"skill.kim.active.pha_giap", "skill.kim.active.hoi_kiem",
	"skill.kim.active.xuyen_phong",
	"skill.moc.active.van_moc_hoi_sinh", "skill.moc.active.van_doc",
	"skill.moc.active.thanh_dang", "skill.moc.active.moc_bo",
	"skill.moc.active.moc_de",
	"skill.thuy.active.thien_ha", "skill.thuy.active.han_trieu",
	"skill.thuy.active.trieu_quyen", "skill.thuy.active.luu_bo",
	"skill.thuy.active.song_anh",
	"skill.hoa.active.cuu_hoa_lien", "skill.hoa.active.hoa_vuc",
	"skill.hoa.active.lien_bao", "skill.hoa.active.boc_bo",
	"skill.hoa.active.hoa_tam",
	"skill.tho.active.thach_kich", "skill.tho.active.tho_giap",
	"skill.tho.active.dia_chan", "skill.tho.active.thien_son_tran",
	"skill.tho.active.son_lap",
}

// balanceCandidate builds the complete synthetic candidate.
func balanceCandidate(t *testing.T) *config.CandidateSnapshot {
	t.Helper()
	defs := map[string]*config.Family{}
	eq := fam(defs, "equipment", "item_id")
	sets := fam(defs, "equipment_set", "set_id")
	rolls := fam(defs, "roll_def", "stat_id")
	budgets := fam(defs, "tier_budget", "tier")
	monster := fam(defs, "monster", "monster_id")
	boss := fam(defs, "boss", "boss_id")
	attack := fam(defs, "attack", "attack_id")
	skill := fam(defs, "skill", "skill_id")
	action := fam(defs, "skill_action", "skill_id")
	proc := fam(defs, "basic_proc", "skill_id")
	effect := fam(defs, "skill_effect", "skill_id")
	spatial := fam(defs, "spatial_effect", "spatial_effect_id")

	// equipment: items split across three set keys per tier so the
	// alphabetically-first set (used by fixture-B max-everywhere counts)
	// carries only the first four slots.
	for _, tier := range tierOrder {
		fl := tierFlats[tier]
		stats := map[string]int64{"ATTACK": fl[0], "DEFENSE": fl[1], "MAX_HP": fl[2], "MAX_MP": fl[3]}
		for _, sk := range []string{"aaa", "alpha", "beta"} {
			for i, sl := range slotLayout {
				if sk == "aaa" && i >= 4 {
					continue
				}
				if sk != "aaa" && i < 4 {
					continue
				}
				v := stats[sl.stat] * sl.pct / 100
				if v < 1 {
					v = 1
				}
				put(t, eq, []config.Value{config.VStr("item.eq." + tier + "." + sk + "." + sl.slot)},
					map[string]config.Value{
						"item_id":           config.VStr("item.eq." + tier + "." + sk + "." + sl.slot),
						"kind":              config.VStr("EQ"),
						"tier":              config.VStr(tier),
						"set_key":           config.VStr("set." + tier + "." + sk),
						"slot":              config.VStr(sl.slot),
						"slot_ordinal":      config.VInt(int64(i)),
						"element":           config.VStr("KIM"),
						"rarity":            config.VStr("RARE"),
						"binding":           config.VStr("CHARACTER_BOUND"),
						"level_min":         config.VInt(int64((i/14 + 1) * 10)),
						"fixed_stats":       config.VList(config.VRec(map[string]config.Value{"stat": config.VStr(sl.stat), "value": config.VInt(v)})),
						"enhanceable_stats": config.VStrs(slotEnhanceable[sl.slot]...),
						"secondary_rolls":   config.VInt(2),
					})
			}
		}
	}
	// equipment sets (6 tiers x 3 keys)
	for _, tier := range tierOrder {
		for _, sk := range []string{"aaa", "alpha", "beta"} {
			put(t, sets, []config.Value{config.VStr("set." + tier + "." + sk)}, map[string]config.Value{
				"set_id":          config.VStr("set." + tier + "." + sk),
				"set_key":         config.VStr("set." + tier + "." + sk),
				"tier":            config.VStr(tier),
				"layout":          config.VStr("FOURTEEN"),
				"display":         config.VStr(sk),
				"source_identity": config.VStr("content"),
			})
		}
	}
	// roll pool (12)
	for rid, stat := range rollStats {
		kind := "FLAT"
		if stat == "CRIT_CHANCE" || stat == "ATTACK_SPEED" || stat == "CAST_SPEED" ||
			stat == "COOLDOWN_REDUCTION" || stat == "LIFESTEAL" || stat == "REFLECT" ||
			stat == "ABSORB" || stat == "HEAL_REDUCTION" {
			kind = "UTILITY"
		}
		fields := map[string]config.Value{
			"stat_id":    config.VStr(rid),
			"stat":       config.VStr(stat),
			"kind":       config.VStr(kind),
			"stage":      config.VStr("SECONDARY"),
			"range_kind": config.VStr("FLAT_UNIT"),
		}
		if kind == "UTILITY" {
			var trs []config.Value
			for _, tier := range tierOrder {
				lo, hi := rollRange(rid, tier)
				trs = append(trs, config.VRec(map[string]config.Value{
					"tier":  config.VStr(tier),
					"range": config.VList(mustRat(t, lo.Num, lo.Den), mustRat(t, hi.Num, hi.Den)),
				}))
			}
			fields["tier_ranges"] = config.VList(trs...)
		}
		put(t, rolls, []config.Value{config.VStr(rid)}, fields)
	}
	tierUnits := map[string][6]int64{ // T1..T6 flat magnitudes per stat
		"A": {1, 1, 1, 1, 1, 1},
		"D": {1, 1, 1, 1, 1, 1},
		"H": {1, 1, 1, 1, 1, 1},
		"M": {1, 1, 1, 1, 1, 1},
	}
	for ti, tier := range tierOrder {
		put(t, budgets, []config.Value{config.VStr(tier)}, map[string]config.Value{
			"tier":   config.VStr(tier),
			"levels": config.VStr("x10"),
			"rarity": config.VStr("RARE"),
			"unit_A": config.VInt(tierUnits["A"][ti]), "unit_D": config.VInt(tierUnits["D"][ti]),
			"unit_H": config.VInt(tierUnits["H"][ti]), "unit_M": config.VInt(tierUnits["M"][ti]),
			"secondary_rolls": config.VInt(2),
		})
	}
	// flat_roll_ranges parameter: one coefficient bound per flat stat,
	// resolved against tier units; degenerate [1,1] coefficients keep the
	// authored magnitude equal to the tier unit.
	flatUnits := map[string]string{"ATTACK": "A", "DEFENSE": "D", "MAX_HP": "H", "MAX_MP": "M"}
	var rangeElems []config.Value
	for _, stat := range []string{"ATTACK", "DEFENSE", "MAX_HP", "MAX_MP"} {
		rangeElems = append(rangeElems, config.VRec(map[string]config.Value{
			"stat":       config.VStr(stat),
			"unit":       config.VStr(flatUnits[stat]),
			"range_kind": config.VStr("FLAT_UNIT"),
			"lo_coef":    mustRat(t, 1, 1),
			"hi_coef":    mustRat(t, 1, 1),
		}))
	}
	vp := config.NewFamily("validation_parameters", "name")
	put(t, vp, []config.Value{config.VStr("flat_roll_ranges")}, map[string]config.Value{
		"records": config.VList(config.VRec(map[string]config.Value{
			"key": config.VList(config.VStr("secondary_pool")),
			"fields": config.VRec(map[string]config.Value{
				"ranges": config.VList(rangeElems...),
			}),
		})),
	})
	// monsters + attacks
	put(t, monster, []config.Value{config.VStr("monster.fld.quy_thu")}, map[string]config.Value{
		"monster_id":   config.VStr("monster.fld.quy_thu"),
		"lv":           config.VInt(10),
		"rank":         config.VStr("NORMAL"),
		"size_profile": config.VStr("MONSTER_MEDIUM"),
	})
	put(t, attack, []config.Value{config.VStr("attack.quy_thu.bite")}, map[string]config.Value{
		"attack_id": config.VStr("attack.quy_thu.bite"),
		"shape":     config.VStr("RECT"),
		"profile":   config.VStr("MELEE"),
		"suffix":    config.VStr("basic"),
		"length_mm": config.VInt(1500),
	})
	// authored bosses
	for _, b := range []bossRow{
		{"quy_nhap_trang", 10, "INSTANCED", "BOSS_LARGE", 14600, 85, 45},
		{"moc_tinh_da", 20, "INSTANCED", "BOSS_LARGE", 22400, 135, 70},
		{"thuong_luong", 30, "INSTANCED", "BOSS_LARGE", 33400, 185, 95},
		{"ma_da_chua", 30, "PUBLIC", "WORLD_BOSS", 33400, 185, 95},
		{"ho_tinh", 40, "INSTANCED", "BOSS_LARGE", 47600, 235, 120},
		{"ho_tinh_chin_duoi", 50, "INSTANCED", "BOSS_LARGE", 65000, 285, 145},
		{"ngu_tinh", 55, "PUBLIC", "WORLD_BOSS", 74900, 310, 157},
		{"than_trung", 60, "INSTANCED", "WORLD_BOSS", 85600, 335, 170},
	} {
		put(t, boss, []config.Value{config.VStr(b.ID)}, map[string]config.Value{
			"boss_id":      config.VStr(b.ID),
			"lv":           config.VInt(b.Level),
			"mode":         config.VStr(b.Mode),
			"size_profile": config.VStr(b.Profile),
			"base_hp":      config.VInt(b.BaseHP),
			"attack":       config.VInt(b.Attack),
			"defense":      config.VInt(b.Defense),
		})
	}
	// skills + actions
	specs := skillTable()
	for i, sid := range skillIDs {
		s := specs[i]
		var tags []config.Value
		for _, tg := range s.tags {
			tags = append(tags, config.VStr(tg))
		}
		put(t, skill, []config.Value{config.VStr(sid)}, map[string]config.Value{
			"skill_id": config.VStr(sid),
			"category": config.VStr(s.cat),
			"tags":     config.VSet(tags...),
		})
		geo := map[string]config.Value{"kind": config.VStr(s.kind)}
		for k, v := range s.fields {
			geo[k] = v
		}
		fields := map[string]config.Value{
			"skill_id":     config.VStr(sid),
			"speed_stat":   config.VStr("ATTACK_SPEED"),
			"startup_ms":   config.VInt(120),
			"active_ms":    config.VInt(140),
			"recovery_ms":  config.VInt(200),
			"geometry":     config.VRec(geo),
			"base_cd_s":    mustRat(t, s.cdRat[0], max64(1, s.cdRat[1])),
			"air_geometry": config.VNull(),
		}
		if s.cat == "active" {
			fields["cost_mp"] = config.VInt(s.cost)
			fields["base_cd_s"] = mustRat(t, s.cd, 1000)
		}
		if sid == "skill.kim.active.pha_giap" {
			fields["air_geometry"] = config.VRec(map[string]config.Value{
				"kind": config.VStr("MELEE_BOX"), "reach": config.VInt(2500)})
		}
		put(t, action, []config.Value{config.VStr(sid)}, fields)
		if s.cat == "active" && s.coef > 0 {
			put(t, effect, []config.Value{config.VStr(sid), config.VInt(0)}, map[string]config.Value{
				"skill_id": config.VStr(sid),
				"ordinal":  config.VInt(0),
				"payload": config.VRec(map[string]config.Value{
					"kind":        config.VStr("DAMAGE"),
					"coefficient": mustRat(t, s.coef, 100),
				}),
			})
		}
	}
	// SPATIAL effects for the two DISPLACEMENT-tagged actives
	put(t, effect, []config.Value{config.VStr("skill.thuy.active.trieu_quyen"), config.VInt(1)}, map[string]config.Value{
		"skill_id": config.VStr("skill.thuy.active.trieu_quyen"),
		"ordinal":  config.VInt(1),
		"payload": config.VRec(map[string]config.Value{
			"kind":   config.VStr("SPATIAL"),
			"ref_id": config.VStr("spatial.thuy.trieu_quyen"),
		}),
	})
	put(t, effect, []config.Value{config.VStr("skill.tho.active.dia_chan"), config.VInt(1)}, map[string]config.Value{
		"skill_id": config.VStr("skill.tho.active.dia_chan"),
		"ordinal":  config.VInt(1),
		"payload": config.VRec(map[string]config.Value{
			"kind":   config.VStr("SPATIAL"),
			"ref_id": config.VStr("spatial.tho.dia_chan"),
		}),
	})
	put(t, spatial, []config.Value{config.VStr("spatial.thuy.trieu_quyen")}, map[string]config.Value{
		"spatial_effect_id":      config.VStr("spatial.thuy.trieu_quyen"),
		"source":                 config.VStr("skill.thuy.active.trieu_quyen"),
		"origin_shape":           config.VStr("LINE"),
		"exact_resolution":       config.VStr("forced position slide along path"),
		"target_cap_interaction": config.VStr("per-target once"),
	})
	put(t, spatial, []config.Value{config.VStr("spatial.tho.dia_chan")}, map[string]config.Value{
		"spatial_effect_id":      config.VStr("spatial.tho.dia_chan"),
		"source":                 config.VStr("skill.tho.active.dia_chan"),
		"origin_shape":           config.VStr("SINGLE"),
		"exact_resolution":       config.VStr("AIRBORNE apex 1.2m"),
		"target_cap_interaction": config.VStr("immunity window"),
	})
	// basic_proc pins
	for cl, suf := range basic1 {
		coef, cd := map[string]int64{"KIM": 100, "MOC": 90, "THUY": 92, "HOA": 95, "THO": 110}[cl],
			map[string]int64{"KIM": 500, "MOC": 700, "THUY": 650, "HOA": 800, "THO": 950}[cl]
		put(t, proc, []config.Value{config.VStr("skill." + lower(cl) + ".basic." + suf)},
			map[string]config.Value{
				"skill_id":         config.VStr("skill." + lower(cl) + ".basic." + suf),
				"base_coefficient": mustRat(t, coef, 100),
				"base_cd_s":        mustRat(t, cd, 1000),
				"status_effects":   config.VList(),
			})
	}
	return &config.CandidateSnapshot{
		AuthoringSchemaVersion: config.AuthoringSchemaVersion,
		ContentSchemaVersion:   config.ContentSchemaVersion,
		RuleVersions:           map[string]int{},
		Definitions:            defs,
		ValidationParameters:   vp,
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// guardrailDiags returns only the balance-guardrail diagnostics.
func guardrailDiags(d config.Diagnostics) []string {
	var out []string
	for _, diag := range d {
		if diag.Code == config.DiagBalanceGuardrail {
			out = append(out, diag.String())
		}
	}
	return out
}

// ---------------------------------------------------------------------
// packet-named tests
// ---------------------------------------------------------------------

// TestTTKWindowsAgainstFourNewStats: fixture A keeps every TTK window
// inside guardrails with the 12-stat roll pool present.
func TestTTKWindowsAgainstFourNewStats(t *testing.T) {
	c := balanceCandidate(t)
	if g := guardrailDiags(Check(c)); len(g) > 0 {
		t.Fatalf("guardrail violations: %v", g)
	}
}

// TestReferenceBasicAttackPin: the canonical basic_1 pin resolves to the
// authored coefficient/cooldown for every class.
func TestReferenceBasicAttackPin(t *testing.T) {
	c := balanceCandidate(t)
	want := map[string][2]float64{
		"KIM": {1.00, 0.50}, "MOC": {0.90, 0.70}, "THUY": {0.92, 0.65},
		"HOA": {0.95, 0.80}, "THO": {1.10, 0.95},
	}
	for _, cl := range classIDs {
		coef, cd, _, _, ok := basicPin(c, cl)
		if !ok {
			t.Fatalf("no basic_1 pin for %s", cl)
		}
		if coef != want[cl][0] || cd != want[cl][1] {
			t.Fatalf("%s basic_1 pin %.2f/%.3fs, want %.2f/%.3fs",
				cl, coef, cd, want[cl][0], want[cl][1])
		}
	}
}

// TestHeavyHitSurvivabilityGuardrail: heavy-hit stays inside 6-18% of
// MAX_HP; an inflated boss attack trips the guardrail.
func TestHeavyHitSurvivabilityGuardrail(t *testing.T) {
	c := balanceCandidate(t)
	if g := guardrailDiags(Check(c)); len(g) > 0 {
		t.Fatalf("unexpected violations: %v", g)
	}
	mut := balanceCandidate(t)
	for k, r := range mut.Definitions["boss"].Records {
		r.Fields["attack"] = config.VInt(r.Fields["attack"].Int * 3)
		mut.Definitions["boss"].Records[k] = r
	}
	found := false
	for _, s := range guardrailDiags(Check(mut)) {
		if strings.Contains(s, "heavy_hit") {
			found = true
		}
	}
	if !found {
		t.Fatalf("inflated boss attack did not trip heavy_hit guardrail")
	}
}

// TestSecondaryRollBounds: the 12-type pool passes the +1% budget rule;
// a one-unit perturbation of every endpoint moves the metric.
func TestSecondaryRollBounds(t *testing.T) {
	c := balanceCandidate(t)
	cat, ld := equipment.Load(c)
	if len(ld) > 0 {
		t.Fatalf("equipment load diags: %v", ld)
	}
	if len(cat.Rolls) != 12 {
		t.Fatalf("roll pool = %d, want 12", len(cat.Rolls))
	}
	if g := guardrailDiags(Check(c)); len(g) > 0 {
		t.Fatalf("guardrail violations: %v", g)
	}
	// one legal unit perturbation must move the marginal output
	m0, ok := expectedMagnitude(cat, "ATTACK", "T6")
	if !ok {
		t.Fatalf("no attack_flat T6 magnitude")
	}
	p0, _ := poolExpectedPower(cat, "T6", oldPoolStats)
	loA, hiA := rollRange("attack_flat", "T6")
	cat2 := mutatedRoll(t, "attack_flat", "T6", loA,
		config.Rat{Num: hiA.Num + 1, Den: hiA.Den})
	m1, _ := expectedMagnitude(cat2, "ATTACK", "T6")
	p1, _ := poolExpectedPower(cat2, "T6", oldPoolStats)
	if cmpRat(m0, m1) == 0 || cmpRat(p0, p1) == 0 {
		t.Fatalf("one-unit attack_flat perturbation did not move output")
	}
	for _, rid := range []string{"lifesteal_roll", "reflect_roll", "absorb_roll", "heal_reduction_roll"} {
		n0, _ := expectedMagnitude(cat, rollStats[rid], "T6")
		loR, hiR := rollRange(rid, "T6")
		cat3 := mutatedRoll(t, rid, "T6", loR,
			config.Rat{Num: hiR.Num + 1, Den: hiR.Den})
		n1, _ := expectedMagnitude(cat3, rollStats[rid], "T6")
		if cmpRat(n0, n1) == 0 {
			t.Fatalf("one-unit %s perturbation did not move output", rid)
		}
	}
}

// TestSkillReachMatrix45: exactly 45 primary-geometry rows, each inside
// its role band; a 46th row trips the count gate.
func TestSkillReachMatrix45(t *testing.T) {
	c := balanceCandidate(t)
	rows, d := enumerateReach(c)
	if len(rows) != 45 {
		t.Fatalf("reach rows = %d, want 45", len(rows))
	}
	if g := guardrailDiags(d); len(g) > 0 {
		t.Fatalf("reach violations: %v", g)
	}
	mut := balanceCandidate(t)
	f := mut.Definitions["skill_action"]
	put(t, f, []config.Value{config.VStr("skill.kim.active.extra")}, map[string]config.Value{
		"skill_id": config.VStr("skill.kim.active.extra"),
		"geometry": config.VRec(map[string]config.Value{
			"kind": config.VStr("MELEE_BOX"), "reach": config.VInt(2000)}),
	})
	mut.Definitions["skill"].Put([]config.Value{config.VStr("skill.kim.active.extra")},
		map[string]config.Value{"skill_id": config.VStr("skill.kim.active.extra"),
			"category": config.VStr("active")})
	_, d2 := enumerateReach(mut)
	found := false
	for _, s := range guardrailDiags(d2) {
		if strings.Contains(s, "rows") {
			found = true
		}
	}
	if !found {
		t.Fatalf("46th geometry row did not trip the row-count gate")
	}
}

// TestSkillReachSeparationRatio: min ranged-basic / max hostile-melee
// reach ratio >= 2.50 (launch 7.5/2.6 = 2.8846).
func TestSkillReachSeparationRatio(t *testing.T) {
	c := balanceCandidate(t)
	n, dd, _, _ := separationRatioMM(c)
	if n == 0 || dd == 0 {
		t.Fatalf("no separation ratio")
	}
	r := float64(n) / float64(dd)
	if r < 2.50 {
		t.Fatalf("separation %.4f < 2.50", r)
	}
	if n != 7500 || dd != 2600 {
		t.Fatalf("separation inputs %d/%d, want 7500/2600", n, dd)
	}
	// a hostile melee reach above the ratio trips the gate
	mut := balanceCandidate(t)
	f := mut.Definitions["skill_action"]
	put(t, f, []config.Value{config.VStr("skill.kim.basic.long")}, map[string]config.Value{
		"skill_id": config.VStr("skill.kim.basic.long"),
		"geometry": config.VRec(map[string]config.Value{
			"kind": config.VStr("MELEE_BOX"), "reach": config.VInt(3100)}),
	})
	if g := guardrailDiags(mustEnumerate(mut, t)); len(g) == 0 {
		t.Fatalf("hostile melee 3.1m did not trip separation gate")
	}
}

// TestSkillCameraReadabilityMargin: 12.8m - max outer reach >= 1.8m.
func TestSkillCameraReadabilityMargin(t *testing.T) {
	c := balanceCandidate(t)
	rows, d := enumerateReach(c)
	if g := guardrailDiags(d); len(g) > 0 {
		t.Fatalf("violations: %v", g)
	}
	var maxOuter int64
	for _, r := range rows {
		if r.OuterMM > maxOuter {
			maxOuter = r.OuterMM
		}
	}
	if cameraHalfMM-maxOuter < marginMinMM {
		t.Fatalf("margin %dmm", cameraHalfMM-maxOuter)
	}
	mut := balanceCandidate(t)
	f := mut.Definitions["skill_action"]
	put(t, f, []config.Value{config.VStr("skill.thuy.active.too_far")}, map[string]config.Value{
		"skill_id": config.VStr("skill.thuy.active.too_far"),
		"geometry": config.VRec(map[string]config.Value{
			"kind": config.VStr("AREA_POSITION"), "cast": config.VInt(9000),
			"radius": config.VInt(3000)}),
	})
	mut.Definitions["skill"].Put([]config.Value{config.VStr("skill.thuy.active.too_far")},
		map[string]config.Value{"skill_id": config.VStr("skill.thuy.active.too_far"),
			"category": config.VStr("active")})
	if g := guardrailDiags(mustEnumerate(mut, t)); len(g) == 0 {
		t.Fatalf("outer reach 12000mm did not trip camera margin gate")
	}
}

// TestSkillColliderBoundaryProfiles: every monster/boss size_profile is
// an ADR-0046 profile; 0.001m gap hits, 0.002m misses per profile.
func TestSkillColliderBoundaryProfiles(t *testing.T) {
	c := balanceCandidate(t)
	if g := guardrailDiags(Check(c)); len(g) > 0 {
		t.Fatalf("violations: %v", g)
	}
	for _, p := range hurtboxProfiles {
		if !surfaceHit(1) || surfaceHit(2) {
			t.Fatalf("%s boundary wrong", p.ID)
		}
	}
	mut := balanceCandidate(t)
	f := mut.Definitions["monster"]
	put(t, f, []config.Value{config.VStr("monster.fld.bad")}, map[string]config.Value{
		"monster_id":   config.VStr("monster.fld.bad"),
		"size_profile": config.VStr("MONSTER_HUGE"),
	})
	found := false
	for _, s := range guardrailDiags(Check(mut)) {
		if strings.Contains(s, "colliders") {
			found = true
		}
	}
	if !found {
		t.Fatalf("undeclared size_profile did not trip collider gate")
	}
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

// mutatedRoll rebuilds a candidate with one roll's magnitude bound
// raised by one legal unit and returns the loaded equipment catalog.
// Utility rolls mutate tier_ranges; flat rolls mutate flat_roll_ranges
// coefficients.
func mutatedRoll(t *testing.T, rid, tier string, lo, hi config.Rat) *equipment.Catalog {
	t.Helper()
	c := balanceCandidate(t)
	f := c.Definitions["roll_def"]
	key := config.KeyString([]config.Value{config.VStr(rid)})
	r, ok := f.Records[key]
	if !ok {
		t.Fatalf("no roll_def %s", rid)
	}
	if trs, has := r.Fields["tier_ranges"]; has {
		for i, tr := range trs.Elems {
			if tr.Rec["tier"].Str == tier {
				trs.Elems[i].Rec["range"] = config.VList(
					mustRat(t, lo.Num, lo.Den), mustRat(t, hi.Num, hi.Den))
			}
		}
		f.Records[key] = r
	} else {
		// flat: bump hi_coef in flat_roll_ranges
		vp := c.ValidationParameters
		rec := vp.Records[config.KeyString([]config.Value{config.VStr("flat_roll_ranges")})]
		stat := rollStats[rid]
		for _, pr := range rec.Fields["records"].Elems {
			for _, rv := range pr.Rec["fields"].Rec["ranges"].Elems {
				if rv.Rec["stat"].Str == stat {
					rv.Rec["hi_coef"] = mustRat(t, hi.Num, hi.Den)
				}
			}
		}
	}
	cat, ld := equipment.Load(c)
	if len(ld) > 0 {
		t.Fatalf("mutated equipment load diags: %v", ld)
	}
	return cat
}

func mustEnumerate(c *config.CandidateSnapshot, t *testing.T) config.Diagnostics {
	t.Helper()
	_, d := enumerateReach(c)
	return d
}
