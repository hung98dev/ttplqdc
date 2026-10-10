package crafting

import (
	"context"
	"fmt"

	"thinhthan/internal/config/equipment"
)

// Recipe is the runtime view of one compiled recipe the executors need
// (crafting_catalog.md § recipes; GUARANTEED x1 +0, binding default).
type Recipe struct {
	RecipeID     string
	OutputItemID string
	// Input is the single regional material input and quantity for one
	// craft; both are multiplied by batch_quantity.
	InputItemID  string
	InputQty     int64
	CommonCost   int64
	MinimumLevel int32
}

// Recipes resolves a recipe_id to its compiled row; nil is fail-closed.
type Recipes func(ctx context.Context, recipeID string) (Recipe, error)

// luckyOut maps a recipe tier to the produced lucky-charm grade.
func luckyOut(tier string) (string, bool) {
	switch tier {
	case "t1", "t2":
		return luckyGradeLow, true
	case "t3", "t4":
		return luckyGradeMid, true
	case "t5":
		return luckyGradeHigh, true
	case "t6":
		return luckyGradeSuper, true
	}
	return "", false
}

// insureOut maps a recipe tier to the produced insurance-charm grade.
func insureOut(tier string) (string, bool) {
	switch tier {
	case "t1", "t2":
		return luckyGradeLow, true
	case "t3", "t4":
		return luckyGradeMid, true
	case "t5", "t6":
		return luckyGradeHigh, true
	}
	return "", false
}

// RecipesFromCatalog derives the recipe resolver from the compiled
// equipment catalog: 168 `recipe.eq.<tier>.<set_key>.<slot>` rows plus
// the 6+6 utility charm recipes (crafting_catalog.md).
func RecipesFromCatalog(cat *equipment.Catalog) Recipes {
	eq := map[string]Recipe{}
	for _, itemID := range cat.ItemIDs {
		it := cat.Items[itemID]
		if it == nil {
			continue
		}
		row, ok := tierTable[it.Tier]
		if !ok {
			continue
		}
		w, ok := slotWeights[it.Slot]
		if !ok {
			continue
		}
		recipeID := fmt.Sprintf("recipe.eq.%s.%s.%s", it.Tier, it.SetKey, it.Slot)
		eq[recipeID] = Recipe{
			RecipeID:     recipeID,
			OutputItemID: itemID,
			InputItemID:  row.MaterialID,
			InputQty:     row.Material * w,
			CommonCost:   row.Common * w,
			MinimumLevel: row.MinLevel,
		}
	}
	util := map[string]Recipe{}
	for tier, row := range tierTable {
		if g, ok := luckyOut(tier); ok {
			id := fmt.Sprintf("recipe.utility.bua_may.%s", tier)
			util[id] = Recipe{
				RecipeID:     id,
				OutputItemID: charmKindLucky + "." + g,
				InputItemID:  row.MaterialID,
				InputQty:     6 * row.Material,
				CommonCost:   2 * row.Common,
				MinimumLevel: row.MinLevel,
			}
		}
		if g, ok := insureOut(tier); ok {
			id := fmt.Sprintf("recipe.utility.bua_giu_bac.%s", tier)
			util[id] = Recipe{
				RecipeID:     id,
				OutputItemID: charmKindInsure + "." + g,
				InputItemID:  row.MaterialID,
				InputQty:     10 * row.Material,
				CommonCost:   4 * row.Common,
				MinimumLevel: row.MinLevel,
			}
		}
	}
	return func(ctx context.Context, recipeID string) (Recipe, error) {
		if r, ok := eq[recipeID]; ok {
			return r, nil
		}
		if r, ok := util[recipeID]; ok {
			return r, nil
		}
		return Recipe{}, fmt.Errorf("%w: recipe %s", ErrNotFound, recipeID)
	}
}

// EquipmentRecipeCount is the authored equipment-recipe cardinality —
// 168 = 6 tiers x 28 set.slot pairs (asserted by tests).
const EquipmentRecipeCount = 168
