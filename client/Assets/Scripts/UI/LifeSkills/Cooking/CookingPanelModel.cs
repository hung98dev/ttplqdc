using System.Collections.Generic;

namespace ThinhThan.UI.LifeSkills.Cooking
{
    /// <summary>
    /// View-state of the cooking panel: one row per hearth recipe plus
    /// the bonfire section (kindle prompt, rest toggle, wine buff).
    /// Pure data — headless PlayMode tests drive it without scene
    /// objects.
    /// </summary>
    public sealed class CookingPanelModel
    {
        /// <summary>One recipe row.</summary>
        public readonly struct RecipeRow
        {
            public RecipeRow(string recipeId, string outputItemId,
                uint missingCount, bool craftable)
            {
                RecipeId = recipeId;
                OutputItemId = outputItemId;
                MissingCount = missingCount;
                Craftable = craftable;
            }

            /// <summary>recipe.food.* catalog id.</summary>
            public string RecipeId
            {
                get;
            }

            /// <summary>Guaranteed output item id.</summary>
            public string OutputItemId
            {
                get;
            }

            /// <summary>Input units the inventory is short (0 = met).</summary>
            public uint MissingCount
            {
                get;
            }

            /// <summary>All inputs present — the row may fire COOK.</summary>
            public bool Craftable
            {
                get;
            }
        }

        /// <summary>Recipe rows in catalog order.</summary>
        public IReadOnlyList<RecipeRow> Rows = new List<RecipeRow>();

        /// <summary>Channel bonfire active (KINDLE state).</summary>
        public bool BonfireActive;

        /// <summary>Local rest-session flag.</summary>
        public bool Resting;

        /// <summary>Rượu Nếp buff (+5% ATTACK) active.</summary>
        public bool BuffActive;

        /// <summary>Buff ticks remaining at last apply.</summary>
        public ulong BuffRemainingTicks;

        /// <summary>Latest failed interact error (Unspecified = none).</summary>
        public string ErrorKey = "";
    }
}
