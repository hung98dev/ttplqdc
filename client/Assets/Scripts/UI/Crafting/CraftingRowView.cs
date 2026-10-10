using ThinhThan.Systems.Crafting;
using TMPro;
using UnityEngine;

namespace ThinhThan.UI.Crafting
{
    /// <summary>
    /// One recipe row in the consult list: recipe id, the tiered
    /// material quantity and the common cost for one craft.
    /// </summary>
    public sealed class CraftingRowView : MonoBehaviour
    {
        public TMP_Text? RecipeText;
        public TMP_Text? InputText;
        public TMP_Text? CostText;

        public void Apply(CraftingRecipes.Row row)
        {
            Set(RecipeText, row.RecipeId);
            Set(InputText, "{0} x{1}", row.Tier.MaterialId,
                row.InputQuantity);
            Set(CostText, "{0}", row.CommonCost);
        }

        private static void Set(TMP_Text? t, string s)
        {
            if (t != null)
            {
                t.text = s;
            }
        }

        private static void Set(TMP_Text? t, string fmt, object a0, object a1)
        {
            if (t != null)
            {
                t.text = string.Format(fmt, a0, a1);
            }
        }

        private static void Set(TMP_Text? t, string fmt, object a0)
        {
            if (t != null)
            {
                t.text = string.Format(fmt, a0);
            }
        }
    }
}
