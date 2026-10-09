namespace ThinhThan.Systems.LifeSkills.Cooking
{
    /// <summary>
    /// Static mirror of the five hearth recipes in
    /// 07_content/crafting_catalog.md (recipe.food.*): input
    /// requirements, guaranteed food output, guaranteed kindling extra
    /// and the authored LIFE_SKILL act EXP. Presentation data only —
    /// the server is authoritative; a recipe the server rejects still
    /// resolves through the recorded 116 result.
    /// </summary>
    public static class CookingRecipes
    {
        /// <summary>Guaranteed extra output of every hearth recipe.</summary>
        public const string ExtraKindlingItemId = "item.material.cui_lua_trai";

        /// <summary>One hearth recipe row.</summary>
        public readonly struct Row
        {
            public Row(string recipeId, string outputItemId,
                (string itemId, uint quantity)[] inputs, ulong lifeSkillExp)
            {
                RecipeId = recipeId;
                OutputItemId = outputItemId;
                Inputs = inputs;
                LifeSkillExp = lifeSkillExp;
            }

            /// <summary>recipe.food.* catalog id.</summary>
            public string RecipeId
            {
                get;
            }

            /// <summary>Guaranteed food/consumable output item id.</summary>
            public string OutputItemId
            {
                get;
            }

            /// <summary>Required input items (item_id, quantity).</summary>
            public (string ItemId, uint Quantity)[] Inputs
            {
                get;
            }

            /// <summary>Authored LIFE_SKILL EXP for one act.</summary>
            public ulong LifeSkillExp
            {
                get;
            }
        }

        /// <summary>All five hearth recipes, catalog order.</summary>
        public static readonly Row[] All =
        {
            new Row(
                "recipe.food.ca_bong_kho",
                "item.consumable.food.ca_bong_kho",
                new (string, uint)[]
                {
                    ("item.material.ca_bong", 1u),
                    ("item.material.rau_ram", 1u),
                },
                6417UL),
            new Row(
                "recipe.food.ca_chep_nuong",
                "item.consumable.food.ca_chep_nuong",
                new (string, uint)[]
                {
                    ("item.material.ca_chep", 1u),
                    ("item.material.gung_lang", 1u),
                },
                8283UL),
            new Row(
                "recipe.food.tom_nuong",
                "item.consumable.food.tom_nuong",
                new (string, uint)[]
                {
                    ("item.material.tom_song", 1u),
                    ("item.material.rau_ram", 1u),
                },
                6332UL),
            new Row(
                "recipe.food.ca_ro_kho",
                "item.consumable.food.ca_bong_kho",
                new (string, uint)[]
                {
                    ("item.material.ca_ro_dong", 1u),
                    ("item.material.gung_lang", 1u),
                },
                7047UL),
            new Row(
                "recipe.food.ruou_nep",
                "item.consumable.ruou_nep",
                new (string, uint)[]
                {
                    ("item.material.ca_chep", 2u),
                    ("item.material.gung_lang", 1u),
                },
                10494UL),
        };
    }
}
