package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileCrafting — crafting_catalog.md driver: 168 equipment recipes
// expanded from equipment items + tier material/slot cost tables + utility
// charm recipes + hearth cooking.
func compileCrafting(c *Ctx, f *File, r *Registry) {
	st := &craftState{
		tierMat:  map[string]map[string]config.Value{},
		slotWt:   map[string]int64{},
		expanded: false,
	}
	for _, b := range r.Bindings {
		path := firstPath(b)
		switch {
		case strings.HasPrefix(path, "Equipment Recipe Expansion"):
			st.wantRecipes, st.hasRecipesDecl = bindingDecl(b, reDeclRecipes)
			st.expanded = true
			c.consumed(f, b) // expansion runs after tier/slot tables
		case strings.HasPrefix(path, "Tier Material Mapping"):
			craftTierMaterial(c, f, b, st)
		case strings.HasPrefix(path, "Slot Cost Weight"):
			st.wantWeightSum, st.hasWeightDecl = bindingDecl(b, reDeclSumTo)
			craftSlotWeights(c, f, b, st)
		case strings.HasPrefix(path, "Lucky Charm Recipes"):
			craftUtilityRecipes(c, f, b, st, "bua_may", 6, 2,
				map[string]string{
					"t1": "item.consumable.bua_may.so_cap", "t2": "item.consumable.bua_may.so_cap",
					"t3": "item.consumable.bua_may.trung_cap", "t4": "item.consumable.bua_may.trung_cap",
					"t5": "item.consumable.bua_may.cao_cap", "t6": "item.consumable.bua_may.sieu_cap",
				})
		case strings.HasPrefix(path, "Insurance Recipes"):
			craftUtilityRecipes(c, f, b, st, "bua_giu_bac", 10, 4,
				map[string]string{
					"t1": "item.consumable.bua_giu_bac.so_cap", "t2": "item.consumable.bua_giu_bac.so_cap",
					"t3": "item.consumable.bua_giu_bac.trung_cap", "t4": "item.consumable.bua_giu_bac.trung_cap",
					"t5": "item.consumable.bua_giu_bac.cao_cap", "t6": "item.consumable.bua_giu_bac.cao_cap",
				})
		case strings.HasPrefix(path, "Hearth Cooking"):
			craftHearth(c, f, b, st)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"crafting binding %q has no driver", b.Raw)
		}
	}
	if st.expanded {
		craftExpandEquipment(c, f, st)
	}
}

type craftState struct {
	tierMat  map[string]map[string]config.Value // tier -> {material_id, material_base, common_base, min_level}
	slotWt   map[string]int64
	expanded bool

	wantRecipes    int64
	hasRecipesDecl bool
	wantWeightSum  int64
	hasWeightDecl  bool
}

func craftTierMaterial(c *Ctx, f *File, b *SourceBinding, st *craftState) {
	sec := f.Root.SectionAt("Tier Material Mapping")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			tier := cellAt(row, 0).Scalar()
			num := func(i int) config.Value {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, i).Scalar(), ",", ""))
				return v
			}
			fields := map[string]config.Value{
				"tier":          config.VStr(tier),
				"material_id":   config.VStr(cellAt(row, 1).Scalar()),
				"minimum_level": num(2),
				"material_base": num(3),
				"common_base":   num(4),
			}
			st.tierMat[strings.ToLower(tier)] = fields
			c.Emit(f.Name, b.Raw, "tier_material",
				[]config.Value{config.VStr(tier)}, fields, row[0].Line)
		}
	}
	c.consumed(f, b)
}

func craftSlotWeights(c *Ctx, f *File, b *SourceBinding, st *craftState) {
	sec := f.Root.SectionAt("Slot Cost Weight")
	if sec == nil {
		return
	}
	var sum int64
	for _, bl := range sec.Content {
		switch bl.Kind {
		case BlockTable:
			if hasHeaders(bl, "slot") {
				for ri := range bl.Cells {
					row := bl.Cells[ri]
					slot := cellAt(row, 0).Scalar()
					w, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 1).Scalar())
					st.slotWt[slot] = w.Int
					sum += w.Int
					c.Emit(f.Name, b.Raw, "slot_weight",
						[]config.Value{config.VStr(slot)},
						map[string]config.Value{
							"slot":   config.VStr(slot),
							"weight": w,
						}, row[0].Line)
				}
			} else if hasHeaders(bl, "Tier") {
				// full-set reference totals -> params
				for ri := range bl.Cells {
					row := bl.Cells[ri]
					c.EmitParam(f.Name, b.Raw, "full_set_cost",
						[]config.Value{config.VStr(cellAt(row, 0).Scalar())},
						map[string]config.Value{
							"tier": config.VStr(cellAt(row, 0).Scalar()),
							"regional_material": func() config.Value {
								v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, 1).Scalar(), ",", ""))
								return v
							}(),
							"common": func() config.Value {
								v, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(cellAt(row, 2).Scalar(), ",", ""))
								return v
							}(),
						}, row[0].Line)
				}
			}
		case BlockFence:
			for j, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if strings.Contains(l, "=") && !strings.Contains(l, "recipe.") && !strings.Contains(l, "->") {
					c.EmitParam(f.Name, b.Raw, "craft_formula",
						[]config.Value{config.VStr(l)},
						map[string]config.Value{"formula": config.VStr(l)}, bl.Line+1+j)
				}
			}
		}
	}
	if st.hasWeightDecl && int64(sum) != st.wantWeightSum {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, sec.Line,
			"slot weights sum %d != declared %d", sum, st.wantWeightSum)
	}
	c.consumed(f, b)
}

func craftExpandEquipment(c *Ctx, f *File, st *craftState) {
	eq := c.Defs.Get("equipment")
	if eq == nil || len(eq.Records) == 0 {
		c.Diags.Addf(config.DiagUnresolvedReference, f.Path, 1,
			"equipment expansion produced no records")
		return
	}
	count := 0
	for _, ks := range eq.SortedKeys() {
		rec := eq.Records[config.KeyString(ks)]
		iid := ks[0].Str
		tier := strings.ToLower(rec.Fields["tier"].Str)
		slot := rec.Fields["slot"].Str
		tm, ok := st.tierMat[tier]
		if !ok {
			c.Diags.Addf(config.DiagUnresolvedReference, f.Path, 1,
				"tier %s has no material mapping", tier)
			continue
		}
		wt, ok := st.slotWt[slot]
		if !ok {
			c.Diags.Addf(config.DiagUnresolvedReference, f.Path, 1,
				"slot %s has no weight", slot)
			continue
		}
		rid := "recipe.eq." + tier + "." + rec.Fields["set_key"].Str + "." + slot
		matQty := tm["material_base"].Int * wt
		commonCost := tm["common_base"].Int * wt
		c.Emit(f.Name, "Equipment Recipe Expansion", "recipe",
			[]config.Value{config.VStr(rid)},
			map[string]config.Value{
				"recipe_id":         config.VStr(rid),
				"output_item_id":    config.VStr(iid),
				"success_mode":      config.VStr("GUARANTEED"),
				"output_quantity":   config.VInt(1),
				"enhancement_level": config.VInt(0),
				"tier":              config.VStr(strings.ToUpper(tier)),
				"common_cost":       config.VInt(commonCost),
				"minimum_level":     tm["minimum_level"],
			}, 0)
		c.Emit(f.Name, "Equipment Recipe Expansion", "recipe_input",
			[]config.Value{config.VStr(rid), tm["material_id"]},
			map[string]config.Value{
				"recipe_id": config.VStr(rid),
				"item_id":   tm["material_id"],
				"quantity":  config.VInt(matQty),
			}, 0)
		count++
	}
	if st.hasRecipesDecl && int64(count) != st.wantRecipes {
		c.Diags.Addf(config.DiagBalanceGuardrail, f.Path, 1,
			"recipe expansion %d != declared %d", count, st.wantRecipes)
	}
}

func craftUtilityRecipes(c *Ctx, f *File, b *SourceBinding, st *craftState,
	name string, matCount, commonMult int64, tierOut map[string]string) {
	for tier := 1; tier <= 6; tier++ {
		tk := "t" + strings.Repeat("", 0) + string(rune('0'+tier))
		out := tierOut[tk]
		tm := st.tierMat[tk]
		if tm == nil {
			continue
		}
		rid := "recipe.utility." + name + "." + tk
		common := tm["common_base"].Int * commonMult
		c.Emit(f.Name, b.Raw, "recipe",
			[]config.Value{config.VStr(rid)},
			map[string]config.Value{
				"recipe_id":       config.VStr(rid),
				"output_item_id":  config.VStr(out),
				"success_mode":    config.VStr("GUARANTEED"),
				"output_quantity": config.VInt(1),
				"tier":            config.VStr("T" + tk[1:]),
				"common_cost":     config.VInt(common),
				"minimum_level":   tm["minimum_level"],
			}, b.Line)
		c.Emit(f.Name, b.Raw, "recipe_input",
			[]config.Value{config.VStr(rid), tm["material_id"]},
			map[string]config.Value{
				"recipe_id": config.VStr(rid),
				"item_id":   tm["material_id"],
				"quantity":  config.VInt(matCount),
			}, b.Line)
	}
	c.consumed(f, b)
}

var cookInputRe = regexp.MustCompile("([0-9]+)\\s*`?([a-z0-9_.]+)`?")

func craftHearth(c *Ctx, f *File, b *SourceBinding, st *craftState) {
	sec := f.Root.SectionAt("Hearth Cooking")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			rid := cellAt(row, 0).Scalar()
			out := cellAt(row, 2).Scalar()
			var extraQty config.Value
			var extraID string
			if m := cookInputRe.FindStringSubmatch(strings.TrimSpace(cellAt(row, 3).Text)); m != nil {
				extraQty, _ = (TypeSpec{Name: "int"}).ParseValue(m[1])
				extraID = m[2]
			}
			c.Emit(f.Name, b.Raw, "recipe",
				[]config.Value{config.VStr(rid)},
				map[string]config.Value{
					"recipe_id":        config.VStr(rid),
					"output_item_id":   config.VStr(out),
					"success_mode":     config.VStr("GUARANTEED"),
					"output_quantity":  config.VInt(1),
					"station_kind":     config.VStr("cooking_hearth"),
					"extra_output_id":  config.VStr(extraID),
					"extra_output_qty": extraQty,
					"life_skill":       config.VBool(true),
				}, row[0].Line)
			// inputs: `N item + M item`
			for _, m := range cookInputRe.FindAllStringSubmatch(cellAt(row, 1).Text, -1) {
				qty, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				c.Emit(f.Name, b.Raw, "recipe_input",
					[]config.Value{config.VStr(rid), config.VStr(m[2])},
					map[string]config.Value{
						"recipe_id": config.VStr(rid),
						"item_id":   config.VStr(m[2]),
						"quantity":  qty,
					}, row[0].Line)
			}
		}
	}
	// LIFE_SKILL per-act values
	acts := []int64{6417, 8283, 6332, 7047, 9448, 10494}
	var vals []config.Value
	for i, v := range acts {
		vals = append(vals, config.VRec(map[string]config.Value{
			"act":        config.VInt(int64(i + 1)),
			"life_skill": config.VInt(v),
		}))
	}
	c.EmitParam(f.Name, b.Raw, "life_skill_cook",
		[]config.Value{config.VStr("DISH_COOKED")},
		map[string]config.Value{
			"act_values":   config.VList(vals...),
			"key_template": config.VStr("life_skill.cook.<recipe_id>.<character_id>.<operation_id>"),
		}, sec.Line)
	c.consumed(f, b)
}
