package cooking

import (
	"context"
	"fmt"
)

// Recipe is the runtime view of one `recipe.food.*` hearth row the
// executors need. Composition binds Recipes from the activated content
// snapshot (compiler emits `recipe` + `recipe_input` records and the
// `life_skill_cook` param); a nil resolver is fail-closed.
type Recipe struct {
	RecipeID       string
	Inputs         []Input
	OutputItemID   string
	ExtraItemID    string // `item.material.cui_lua_trai` on every food row
	LifeSkillByAct [6]uint64
}

// Input is one consumed material of a hearth recipe.
type Input struct {
	ItemID   string
	Quantity uint64
}

// Recipes resolves a recipe_id to its runtime definition.
type Recipes func(ctx context.Context, recipeID string) (Recipe, error)

// LifeSkillExp returns the authored per-act award (progression_route.md
// DISH_COOKED act values; act is 1..6 by captured character level band).
func (r Recipe) LifeSkillExp(act int) uint64 {
	if act < 1 || act > len(r.LifeSkillByAct) {
		return 0
	}
	return r.LifeSkillByAct[act-1]
}

var errUnknownRecipe = fmt.Errorf("cooking: unknown recipe")

// staticRecipes is the compiled crafting_catalog.md § Hearth Cooking
// table verbatim — every `recipe.food.*` grants
// `extra_output = 1 item.material.cui_lua_trai`. Composition may bind
// this table directly until a content-bundle accessor lands.
func staticRecipes(_ context.Context, recipeID string) (Recipe, error) {
	if r, ok := staticRecipeTable[recipeID]; ok {
		return r, nil
	}
	return Recipe{}, fmt.Errorf("%w: %q", errUnknownRecipe, recipeID)
}

// StaticRecipes exposes the compiled table for composition/tests.
func StaticRecipes() Recipes { return staticRecipes }

// lifeSkillActs are the canonical DISH_COOKED per-act values
// (progression_route.md): I 6417, II 8283, III 6332, IV 7047, V 9448,
// VI 10494.
var lifeSkillActs = [6]uint64{6417, 8283, 6332, 7047, 9448, 10494}

// extraKindlingID is the guaranteed extra output of every hearth recipe
// (crafting_catalog.md § Hearth Cooking; cooking is the kindling faucet).
const extraKindlingID = "item.material.cui_lua_trai"

var staticRecipeTable = map[string]Recipe{
	"recipe.food.ca_bong_kho": {
		RecipeID: "recipe.food.ca_bong_kho",
		Inputs: []Input{
			{ItemID: "item.material.ca_bong", Quantity: 1},
			{ItemID: "item.material.rau_ram", Quantity: 1},
		},
		OutputItemID:   "item.consumable.food.ca_bong_kho",
		ExtraItemID:    extraKindlingID,
		LifeSkillByAct: lifeSkillActs,
	},
	"recipe.food.ca_chep_nuong": {
		RecipeID: "recipe.food.ca_chep_nuong",
		Inputs: []Input{
			{ItemID: "item.material.ca_chep", Quantity: 1},
			{ItemID: "item.material.gung_lang", Quantity: 1},
		},
		OutputItemID:   "item.consumable.food.ca_chep_nuong",
		ExtraItemID:    extraKindlingID,
		LifeSkillByAct: lifeSkillActs,
	},
	"recipe.food.tom_nuong": {
		RecipeID: "recipe.food.tom_nuong",
		Inputs: []Input{
			{ItemID: "item.material.tom_song", Quantity: 1},
			{ItemID: "item.material.rau_ram", Quantity: 1},
		},
		OutputItemID:   "item.consumable.food.tom_nuong",
		ExtraItemID:    extraKindlingID,
		LifeSkillByAct: lifeSkillActs,
	},
	"recipe.food.ca_ro_kho": {
		RecipeID: "recipe.food.ca_ro_kho",
		Inputs: []Input{
			{ItemID: "item.material.ca_ro_dong", Quantity: 1},
			{ItemID: "item.material.gung_lang", Quantity: 1},
		},
		OutputItemID:   "item.consumable.food.ca_bong_kho",
		ExtraItemID:    extraKindlingID,
		LifeSkillByAct: lifeSkillActs,
	},
	"recipe.food.ruou_nep": {
		RecipeID: "recipe.food.ruou_nep",
		Inputs: []Input{
			{ItemID: "item.material.ca_chep", Quantity: 2},
			{ItemID: "item.material.gung_lang", Quantity: 1},
		},
		OutputItemID:   "item.consumable.ruou_nep",
		ExtraItemID:    extraKindlingID,
		LifeSkillByAct: lifeSkillActs,
	},
}
