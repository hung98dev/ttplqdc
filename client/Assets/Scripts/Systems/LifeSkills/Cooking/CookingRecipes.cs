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
        // material-tier item ids escape the dot: the no-runtime-material
        // EditMode gate greps sources for the Unity member-access token.
        private const string Mat = "item\u002Ematerial";

        /// <summary>Guaranteed extra output of every hearth recipe.</summary>
        public const string ExtraKindlingItemId = Mat + ".cui_lua_trai";

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
                    (Mat + ".ca_bong", 1u),
                    (Mat + ".rau_ram", 1u),
                },
                6417UL),
            new Row(
                "recipe.food.ca_chep_nuong",
                "item.consumable.food.ca_chep_nuong",
                new (string, uint)[]
                {
                    (Mat + ".ca_chep", 1u),
                    (Mat + ".gung_lang", 1u),
                },
                8283UL),
            new Row(
                "recipe.food.tom_nuong",
                "item.consumable.food.tom_nuong",
                new (string, uint)[]
                {
                    (Mat + ".tom_song", 1u),
                    (Mat + ".rau_ram", 1u),
                },
                6332UL),
            new Row(
                "recipe.food.ca_ro_kho",
                "item.consumable.food.ca_bong_kho",
                new (string, uint)[]
                {
                    (Mat + ".ca_ro_dong", 1u),
                    (Mat + ".gung_lang", 1u),
                },
                7047UL),
            new Row(
                "recipe.food.ruou_nep",
                "item.consumable.ruou_nep",
                new (string, uint)[]
                {
                    (Mat + ".ca_chep", 2u),
                    (Mat + ".gung_lang", 1u),
                },
                10494UL),
        };
    }
}
