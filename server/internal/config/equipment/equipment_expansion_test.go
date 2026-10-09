package equipment

import (
	"fmt"
	"strings"
	"testing"

	"thinhthan/internal/config"
)

// fixture mirrors the emitted equipment domain: 12 sets x 14 slots =
// 168 item.eq.* records plus their set/effect/support, roll, layout,
// budget, enhancement and guaranteed-recipe families, built the way the
// compiler emits them (record shape pinned by catalog_equipment.go).
type setRow struct {
	key, tier, layout, display, source string
}

var fixtureSets = []setRow{
	{"dinh_lang", "T1", "A", "Trầm Lăng", "Act-I dungeon/elite/crafting mix"},
	{"xom_chim", "T1", "B", "Xóm Chìm", "`dungeon.xom_chim`"},
	{"hang_ma_tranh", "T2", "A", "Hang Ma Tranh", "`dungeon.hang_ma_tranh`"},
	{"song_cong", "T2", "B", "Sông Cồng", "regional elites"},
	{"mieu_ba_trong_rung", "T2", "A", "Miếu Bà trong Rừng", "`dungeon.mieu_ba_trong_rung`"},
	{"thanh_nghiem", "T2", "B", "Thánh Nghiệm", "elite/regional/boss"},
	{"than_tai", "T3", "B", "Thần Tài", "dungeon progression"},
	{"than_phach", "T3", "A", "Thần Phách", "elite/regional/boss"},
	{"dinh_hon", "T4", "B", "Đỉnh Hồn", "dungeon progression"},
	{"than_tam", "T4", "A", "Thần Tâm", "elite/regional/boss"},
	{"vinh_hang", "T5", "A", "Vĩnh Hằng", "endgame boss"},
	{"dau_cu", "T6", "B", "Đấu Cự", "`boss.than_trung`"},
}

var fixtureBudgets = map[string][6]int64{
	// tier -> unit_A, unit_D, unit_H, unit_M, secondary_rolls
	"T1": {4, 5, 8, 20, 1},
	"T2": {6, 8, 12, 30, 1},
	"T3": {9, 11, 16, 40, 1},
	"T4": {16, 11, 16, 60, 2},
	"T5": {22, 14, 20, 80, 2},
	"T6": {29, 17, 24, 100, 2},
}

var fixtureRarity = map[string]string{
	"T1": "COMMON", "T2": "MAGIC", "T3": "RARE",
	"T4": "EPIC", "T5": "RELIC", "T6": "MYTHIC",
}

var fixtureLevels = map[string][2]int64{
	"T1": {1, 14}, "T2": {15, 24}, "T3": {25, 34},
	"T4": {35, 44}, "T5": {45, 54}, "T6": {55, 60},
}

var fixtureLayouts = map[string]map[string]string{
	"A": {
		"weapon": "KIM", "head": "MOC", "body": "MOC", "hands": "THUY",
		"legs": "THUY", "feet": "THO", "necklace": "HOA", "ring": "HOA",
		"costume": "KIM", "talisman": "THO", "jade": "KIM", "seal": "HOA",
		"relic": "MOC", "charm": "THUY",
	},
	"B": {
		"weapon": "THO", "head": "HOA", "body": "HOA", "hands": "KIM",
		"legs": "KIM", "feet": "MOC", "necklace": "THUY", "ring": "THUY",
		"costume": "THO", "talisman": "MOC", "jade": "THO", "seal": "THUY",
		"relic": "HOA", "charm": "KIM",
	},
}

func vrat(num, den int64) config.Value {
	return config.Value{Kind: config.KindRational, Rat: config.Rat{Num: num, Den: den}}
}

func fam(name string, keyCols ...string) *config.Family {
	return config.NewFamily(name, keyCols...)
}

// buildSnapshot assembles the emitted equipment domain.
func buildSnapshot(t *testing.T) *config.CandidateSnapshot {
	t.Helper()
	defs := map[string]*config.Family{}

	items := fam("equipment", "item_id")
	sets := fam("equipment_set", "set_key")
	effects := fam("set_effect", "set_key", "threshold")
	supports := fam("set_support", "set_key")
	recipes := fam("recipe", "recipe_id")
	for _, s := range fixtureSets {
		b := fixtureBudgets[s.tier]
		lv := fixtureLevels[s.tier]
		sets.Put([]config.Value{config.VStr(s.key)}, map[string]config.Value{
			"set_id":          config.VStr("set." + strings.ToLower(s.tier) + "." + s.key),
			"set_key":         config.VStr(s.key),
			"tier":            config.VStr(s.tier),
			"layout":          config.VStr(s.layout),
			"display":         config.VStr(s.display),
			"source_identity": config.VStr(s.source),
		})
		for _, thr := range []int64{2, 4, 6} {
			eid := fmt.Sprintf("effect.set.%s.%s.%d", strings.ToLower(s.tier), s.key, thr)
			effects.Put([]config.Value{config.VStr(s.key), config.VInt(thr)}, map[string]config.Value{
				"set_key":   config.VStr(s.key),
				"threshold": config.VInt(thr),
				"effect_id": config.VStr(eid),
				"payload":   config.VStr(fmt.Sprintf("MAX_HP +0.0%d PERCENT_ADD", thr)),
			})
		}
		supports.Put([]config.Value{config.VStr(s.key)}, map[string]config.Value{
			"set_key":    config.VStr(s.key),
			"support_id": config.VStr("support.set." + s.key),
			"payload":    config.VStr("ATTACK +0.01 PERCENT_ADD"),
		})
		for si, slot := range CanonicalSlotOrder {
			iid := ItemID(s.tier, s.key, slot)
			fixed, enh := fixtureFixed(slot, b)
			items.Put([]config.Value{config.VStr(iid)}, map[string]config.Value{
				"item_id":           config.VStr(iid),
				"kind":              config.VStr("EQUIPMENT"),
				"tier":              config.VStr(s.tier),
				"set_key":           config.VStr(s.key),
				"slot":              config.VStr(slot),
				"slot_ordinal":      config.VInt(int64(si)),
				"element":           config.VStr(fixtureLayouts[s.layout][slot]),
				"level_min":         config.VInt(lv[0]),
				"level_max":         config.VInt(lv[1]),
				"rarity":            config.VStr(fixtureRarity[s.tier]),
				"binding":           config.VStr("UNBOUND"),
				"binding_trigger":   config.VStr("ON_EQUIP"),
				"stack_limit":       config.VInt(1),
				"fixed_stats":       fixed,
				"enhanceable_stats": enh,
				"secondary_rolls":   config.VInt(b[4]),
			})
			rid := "recipe.eq." + strings.ToLower(s.tier) + "." + s.key + "." + slot
			recipes.Put([]config.Value{config.VStr(rid)}, map[string]config.Value{
				"recipe_id":      config.VStr(rid),
				"output_item_id": config.VStr(iid),
				"success_mode":   config.VStr("GUARANTEED"),
			})
		}
	}
	defs["equipment"] = items
	defs["equipment_set"] = sets
	defs["set_effect"] = effects
	defs["set_support"] = supports
	defs["recipe"] = recipes

	rolls := fam("roll_def", "stat_id")
	flatStats := map[string]string{
		"roll.attack_flat": "ATTACK", "roll.defense_flat": "DEFENSE",
		"roll.max_hp_flat": "MAX_HP", "roll.max_mp_flat": "MAX_MP",
	}
	utilStats := map[string]string{
		"roll.crit_chance": "CRIT_CHANCE", "roll.attack_speed": "ATTACK_SPEED",
		"roll.cast_speed": "CAST_SPEED", "roll.cooldown_reduction": "COOLDOWN_REDUCTION",
		"roll.lifesteal": "LIFESTEAL", "roll.reflect": "REFLECT",
		"roll.absorb": "ABSORB", "roll.heal_reduction": "HEAL_REDUCTION",
	}
	for _, id := range RollPool {
		if stat, ok := flatStats[id]; ok {
			rolls.Put([]config.Value{config.VStr(id)}, map[string]config.Value{
				"stat_id":     config.VStr(id),
				"stat":        config.VStr(stat),
				"kind":        config.VStr("FLAT"),
				"stage":       config.VStr(""),
				"tier_ranges": config.VList(flatRanges()...),
			})
			continue
		}
		rolls.Put([]config.Value{config.VStr(id)}, map[string]config.Value{
			"stat_id":     config.VStr(id),
			"stat":        config.VStr(utilStats[id]),
			"kind":        config.VStr("UTILITY"),
			"stage":       config.VStr("FLAT_ADD"),
			"tier_ranges": config.VList(utilRanges()...),
		})
	}
	defs["roll_def"] = rolls

	budgets := fam("tier_budget", "tier")
	for _, tier := range TierNames {
		b := fixtureBudgets[tier]
		lv := fixtureLevels[tier]
		budgets.Put([]config.Value{config.VStr(tier)}, map[string]config.Value{
			"tier":            config.VStr(tier),
			"levels":          config.VStr(fmt.Sprintf("%d-%d", lv[0], lv[1])),
			"rarity":          config.VStr(fixtureRarity[tier]),
			"unit_A":          config.VInt(b[0]),
			"unit_D":          config.VInt(b[1]),
			"unit_H":          config.VInt(b[2]),
			"unit_M":          config.VInt(b[3]),
			"secondary_rolls": config.VInt(b[4]),
		})
	}
	defs["tier_budget"] = budgets

	layouts := fam("element_layout", "layout", "slot")
	for l, m := range fixtureLayouts {
		for slot, elem := range m {
			layouts.Put([]config.Value{config.VStr(l), config.VStr(slot)}, map[string]config.Value{
				"layout":  config.VStr(l),
				"slot":    config.VStr(slot),
				"element": config.VStr(elem),
			})
		}
	}
	defs["element_layout"] = layouts

	enh := fam("enhancement_base", "tier")
	for _, tier := range TierNames {
		enh.Put([]config.Value{config.VStr(tier)}, map[string]config.Value{
			"tier":            config.VStr(tier),
			"material_units":  config.VInt(1),
			"common_currency": config.VInt(100),
		})
	}
	defs["enhancement_base"] = enh

	dungeons := fam("dungeon", "dungeon_id")
	for _, id := range []string{"dungeon.xom_chim", "dungeon.hang_ma_tranh", "dungeon.mieu_ba_trong_rung"} {
		dungeons.Put([]config.Value{config.VStr(id)}, map[string]config.Value{"dungeon_id": config.VStr(id)})
	}
	defs["dungeon"] = dungeons
	bosses := fam("boss", "boss_id")
	bosses.Put([]config.Value{config.VStr("boss.than_trung")}, map[string]config.Value{
		"boss_id": config.VStr("boss.than_trung"),
	})
	defs["boss"] = bosses

	params := config.NewFamily("validation_parameters", "family")
	slotOrder := config.NewFamily("slot_order", "slot")
	for i, s := range CanonicalSlotOrder {
		slotOrder.Put([]config.Value{config.VStr(s)}, map[string]config.Value{
			"slot": config.VStr(s), "ordinal": config.VInt(int64(i)),
		})
	}
	putParamFamily(params, slotOrder)
	prio := config.NewFamily("support_priority", "set_key")
	for _, s := range fixtureSets {
		prio.Put([]config.Value{config.VStr(s.key)}, map[string]config.Value{
			"set_key": config.VStr(s.key), "priority": config.VInt(20),
		})
	}
	putParamFamily(params, prio)

	return &config.CandidateSnapshot{
		Definitions:          defs,
		ValidationParameters: params,
	}
}

func putParamFamily(params, f *config.Family) {
	recs := make([]config.Value, 0, len(f.Records))
	for _, k := range f.SortedKeys() {
		rec := f.Records[config.KeyString(k)]
		recs = append(recs, config.VRec(map[string]config.Value{
			"fields": config.VRec(rec.Fields),
			"key":    config.VList(rec.Key...),
		}))
	}
	params.Put([]config.Value{config.VStr(f.Name)}, map[string]config.Value{
		"family":      config.VStr(f.Name),
		"key_columns": config.VStrs(f.KeyColumns...),
		"records":     config.VList(recs...),
	})
}

func flatRanges() []config.Value {
	out := make([]config.Value, 0, len(TierNames))
	for _, tier := range TierNames {
		b := fixtureBudgets[tier]
		out = append(out, config.VRec(map[string]config.Value{
			"tier":  config.VStr(tier),
			"range": config.VList(vrat(b[0]/2, 1), vrat(b[0], 1)),
		}))
	}
	return out
}

func utilRanges() []config.Value {
	out := make([]config.Value, 0, len(TierNames))
	for i, tier := range TierNames {
		out = append(out, config.VRec(map[string]config.Value{
			"tier":  config.VStr(tier),
			"range": config.VList(vrat(int64(i+1), 1000), vrat(int64(i+1)*2, 1000)),
		}))
	}
	return out
}

// fixtureFixed resolves one slot's fixed stats like the compiler's
// expansion: flat terms enhanceable, ring/charm utility terms not.
func fixtureFixed(slot string, b [6]int64) (config.Value, config.Value) {
	term := func(stat string, v config.Value) config.Value {
		return config.VRec(map[string]config.Value{"stat": config.VStr(stat), "value": v})
	}
	switch slot {
	case "weapon":
		return config.VList(term("ATTACK", config.VInt(2*b[0]))),
			config.VSet(config.VStr("ATTACK"))
	case "ring":
		return config.VList(term("ATTACK", config.VInt(b[0])), term("CRIT_CHANCE", vrat(5, 1000))),
			config.VSet(config.VStr("ATTACK"))
	case "charm":
		return config.VList(term("MAX_HP", config.VInt(2*b[2])), term("COOLDOWN_REDUCTION", vrat(1, 100))),
			config.VSet(config.VStr("MAX_HP"))
	default:
		return config.VList(term("DEFENSE", config.VInt(b[1])), term("MAX_HP", config.VInt(b[2]))),
			config.VSet(config.VStr("DEFENSE"), config.VStr("MAX_HP"))
	}
}

func checkOK(t *testing.T, c *config.CandidateSnapshot) {
	t.Helper()
	d := Check(c)
	for _, e := range d {
		t.Errorf("unexpected diagnostic: %s", e.String())
	}
}

// TestTwelveSetsExpansion: exactly 12 set defs, each with tier, A/B
// layout, the three 2/4/6 fences and exactly one support signature.
func TestTwelveSetsExpansion(t *testing.T) {
	c := buildSnapshot(t)
	cat, d := Load(c)
	if len(d) != 0 {
		t.Fatalf("load diagnostics: %v", d)
	}
	if len(cat.Sets) != 12 {
		t.Fatalf("sets = %d, want 12", len(cat.Sets))
	}
	wantTier := map[string]string{}
	for _, s := range fixtureSets {
		wantTier[s.key] = s.tier
	}
	for key, s := range cat.Sets {
		if s.Tier != wantTier[key] {
			t.Errorf("set %s tier %s, want %s", key, s.Tier, wantTier[key])
		}
		if s.Layout != "A" && s.Layout != "B" {
			t.Errorf("set %s layout %q", key, s.Layout)
		}
		for _, thr := range []int64{2, 4, 6} {
			e, ok := s.Effects[thr]
			if !ok {
				t.Errorf("set %s missing %dpc effect", key, thr)
				continue
			}
			want := fmt.Sprintf("effect.set.%s.%s.%d", strings.ToLower(s.Tier), key, thr)
			if e.ID != want {
				t.Errorf("set %s %dpc id %s, want %s", key, thr, e.ID, want)
			}
		}
		if len(s.Effects) != 3 {
			t.Errorf("set %s has %d effects, want exactly 3", key, len(s.Effects))
		}
		if s.Support == nil {
			t.Errorf("set %s missing support signature", key)
		} else if s.Support.ID != "support.set."+key || s.Support.Priority != 20 {
			t.Errorf("set %s support %s@%d", key, s.Support.ID, s.Support.Priority)
		}
	}
	checkOK(t, c)
}

// TestOneHundredSixtyEightDefinitions: the finite expansion emits the
// complete 12 x 14 ID matrix; each item carries element per layout,
// canonical slot ordinal, binding defaults and resolved fixed stats.
func TestOneHundredSixtyEightDefinitions(t *testing.T) {
	c := buildSnapshot(t)
	cat, d := Load(c)
	if len(d) != 0 {
		t.Fatalf("load diagnostics: %v", d)
	}
	if len(cat.ItemIDs) != 168 {
		t.Fatalf("items = %d, want 168", len(cat.ItemIDs))
	}
	seen := map[string]bool{}
	for _, id := range cat.ItemIDs {
		if seen[id] {
			t.Fatalf("duplicate item id %s", id)
		}
		seen[id] = true
		it := cat.Items[id]
		if it.Binding != "UNBOUND" || it.BindingTrigger != "ON_EQUIP" {
			t.Errorf("%s binding %s/%s", id, it.Binding, it.BindingTrigger)
		}
		if it.Element != fixtureLayouts[cat.Sets[it.SetKey].Layout][it.Slot] {
			t.Errorf("%s element %s mismatch layout", id, it.Element)
		}
		if len(it.FixedStats) == 0 {
			t.Errorf("%s no fixed stats", id)
		}
	}
	// spot-check resolved fixed-stat shape per slot kind
	w := cat.Items[ItemID("T6", "dau_cu", "weapon")]
	if len(w.FixedStats) != 1 || w.FixedStats[0].Stat != "ATTACK" ||
		w.FixedStats[0].Flat != 2*fixtureBudgets["T6"][0] {
		t.Errorf("T6 weapon fixed %v", w.FixedStats)
	}
	r := cat.Items[ItemID("T1", "dinh_lang", "ring")]
	var util *StatTerm
	for i := range r.FixedStats {
		if r.FixedStats[i].IsUtility() {
			util = &r.FixedStats[i]
		}
	}
	if util == nil || util.Stat != "CRIT_CHANCE" {
		t.Fatalf("ring utility term missing: %v", r.FixedStats)
	}
	for _, s := range r.Enhanceable {
		if s == "CRIT_CHANCE" {
			t.Errorf("ring utility stat must not be enhanceable")
		}
	}
	checkOK(t, c)
}

// TestSecondaryRollThresholds: the closed 12-ID pool, per-tier roll
// counts (1 for T1-T3, 2 for T4-T6) and floored inclusive bounds.
func TestSecondaryRollThresholds(t *testing.T) {
	c := buildSnapshot(t)
	cat, d := Load(c)
	if len(d) != 0 {
		t.Fatalf("load diagnostics: %v", d)
	}
	if len(cat.Rolls) != 12 {
		t.Fatalf("roll pool = %d, want 12", len(cat.Rolls))
	}
	for _, id := range RollPool {
		rd := cat.Rolls[id]
		if rd == nil {
			t.Fatalf("missing roll %s", id)
		}
		if len(rd.TierRanges) != 6 {
			t.Errorf("roll %s covers %d tiers, want 6", id, len(rd.TierRanges))
		}
	}
	for _, id := range cat.ItemIDs {
		it := cat.Items[id]
		want := fixtureBudgets[it.Tier][4]
		if it.SecondaryRolls != want {
			t.Errorf("%s secondary_rolls %d, want %d", id, it.SecondaryRolls, want)
		}
	}
	checkOK(t, c)

	// negatives: missing pool member, no-recipe item, untyped payload,
	// wrong support priority each surface a diagnostic.
	t.Run("missing roll member rejected", func(t *testing.T) {
		c2 := buildSnapshot(t)
		delete(c2.Definitions["roll_def"].Records,
			config.KeyString([]config.Value{config.VStr("roll.heal_reduction")}))
		assertDiag(t, Check(c2), "roll.heal_reduction")
	})
	t.Run("no guaranteed recipe rejected", func(t *testing.T) {
		c2 := buildSnapshot(t)
		for k, rec := range c2.Definitions["recipe"].Records {
			if strField(rec, "output_item_id") == ItemID("T1", "dinh_lang", "weapon") {
				delete(c2.Definitions["recipe"].Records, k)
			}
		}
		assertDiag(t, Check(c2), "guaranteed recipe")
	})
	t.Run("untyped modifier rejected", func(t *testing.T) {
		c2 := buildSnapshot(t)
		rec := c2.Definitions["set_effect"].Records[config.KeyString([]config.Value{config.VStr("dinh_lang"), config.VInt(2)})]
		rec.Fields["payload"] = config.VStr("MAX_HP +0.03")
		assertDiag(t, Check(c2), "typed stage")
	})
	t.Run("support priority drift rejected", func(t *testing.T) {
		c2 := buildSnapshot(t)
		vp := c2.ValidationParameters.Records[config.KeyString([]config.Value{config.VStr("support_priority")})]
		recs := vp.Fields["records"].Elems
		for i, r := range recs {
			f := r.Rec["fields"]
			f.Rec["priority"] = config.VInt(7)
			recs[i] = r
		}
		assertDiag(t, Check(c2), "priority")
	})
}

func assertDiag(t *testing.T, d config.Diagnostics, want string) {
	t.Helper()
	if len(d) == 0 {
		t.Fatalf("expected diagnostics containing %q, got none", want)
	}
	for _, e := range d {
		if strings.Contains(e.String(), want) {
			return
		}
	}
	t.Fatalf("expected diagnostic containing %q; got %v", want, d)
}
