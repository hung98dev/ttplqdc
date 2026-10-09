using System.Collections.Generic;
using System.Threading;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Crafting;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Crafting
{
    /// <summary>
    /// Crafting consult panel: a pooled recipe list, the selected
    /// recipe's input/cost preview, affordability flags, and the
    /// enhancement block (target, at most one lucky-charm and one
    /// insurance slot with eligibility, server-provided rate
    /// display). Rows come from a Pool so rebuilds never allocate per
    /// apply. Sends go through the bound intents; outcomes are the
    /// recorded 405/407 results in the applied state.
    /// </summary>
    public sealed class CraftingPanel : MonoBehaviour
    {
        public RectTransform? ListRoot;
        public CraftingRowView? RowPrefab;
        public TMP_Text? DetailText;
        public TMP_Text? StatusText;
        public TMP_Text? RateText;
        public Button? CraftButton;
        public Button? EnhanceButton;

        public ICraftingIntents? Intents
        {
            get;
            set;
        }

        private Pool<CraftingRowView>? _pool;
        private readonly List<CraftingRowView> _live =
            new List<CraftingRowView>();
        private CraftingState? _state;

        /// <summary>Apply-call count (once-per-frame proof for tests).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Live recipe row count (test introspection).</summary>
        public int LiveCount
        {
            get
            {
                return _live.Count;
            }
        }

        /// <summary>Row i (test introspection).</summary>
        public CraftingRowView RowAt(int i)
        {
            return _live[i];
        }

        private void Awake()
        {
            _pool = new Pool<CraftingRowView>(CreateRow,
                r => r.gameObject.SetActive(false));
            _pool.Prewarm(8);
            if (CraftButton != null)
            {
                CraftButton.onClick.AddListener(SendCraft);
            }
            if (EnhanceButton != null)
            {
                EnhanceButton.onClick.AddListener(SendEnhance);
            }
        }

        private CraftingRowView CreateRow()
        {
            CraftingRowView r;
            if (RowPrefab != null)
            {
                r = Instantiate(RowPrefab, transform, false);
            }
            else
            {
                var go = new GameObject("row", typeof(RectTransform),
                    typeof(CraftingRowView));
                r = go.GetComponent<CraftingRowView>();
            }
            return r;
        }

        /// <summary>
        /// Rebuild the recipe list + consult status from the applied
        /// projection. Row content is presentation-only; outcomes are
        /// the recorded results in <paramref name="state"/>.
        /// </summary>
        public void Apply(CraftingState state)
        {
            _state = state;
            ApplyCount++;
            if (_pool != null)
            {
                foreach (CraftingRowView row in _live)
                {
                    _pool.Return(row);
                }
                _live.Clear();
                foreach (CraftingRecipes.Row row in CraftingRecipes.Equipment)
                {
                    CraftingRowView view = _pool.Rent();
                    view.Apply(row);
                    _live.Add(view);
                }
                for (int i = 0; i < _live.Count; i++)
                {
                    _live[i].gameObject.SetActive(true);
                }
            }

            CraftingRecipes.TryFind(state.SelectedRecipeId,
                out CraftingRecipes.Row sel);
            Set(DetailText, sel.RecipeId != null
                ? string.Format("{0} -> {1} (+0)  mat x{2}  common {3}",
                    sel.RecipeId, sel.OutputItemId, sel.InputQuantity,
                    sel.CommonCost * state.Batch)
                : "");

            string status = "";
            if (state.LastCraftResult != null)
            {
                status = string.Format("craft {0} x{1}: {2}",
                    state.LastCraftResult.RecipeId,
                    state.LastCraftResult.BatchQuantity,
                    state.LastCraftResult.Result);
            }
            if (state.LastEnhanceResult != null)
            {
                status = string.Format("enhance +{0} -> +{1} {2}",
                    state.LastEnhanceResult.LevelBefore,
                    state.LastEnhanceResult.LevelAfter,
                    state.LastEnhanceResult.Success ? "success" : "fail");
            }
            Set(StatusText, status);

            if (state.LastEnhanceResult != null)
            {
                Set(RateText, "{0}bp pity {1}",
                    state.LastEnhanceResult.FinalRateBp,
                    state.LastEnhanceResult.PityFailCount);
            }
            else
            {
                Set(RateText, "");
            }
        }

        /// <summary>Send the bound craft consult (404).</summary>
        public void SendCraft()
        {
            CraftingState? s = _state;
            ICraftingIntents? i = Intents;
            if (s == null || i == null || s.NpcId.Length == 0 ||
                s.SelectedRecipeId.Length == 0)
            {
                return;
            }
            uint batch = s.Batch < CraftingRecipes.BatchMin
                ? CraftingRecipes.BatchMin
                : s.Batch;
            _ = i.RequestCraft(s.NpcId, s.SelectedRecipeId, batch,
                CancellationToken.None);
        }

        /// <summary>Send the bound enhance consult (406).</summary>
        public void SendEnhance()
        {
            CraftingState? s = _state;
            ICraftingIntents? i = Intents;
            if (s == null || i == null || s.NpcId.Length == 0 ||
                s.EnhanceItemInstanceId.Length != 16)
            {
                return;
            }
            _ = i.RequestEnhance(s.NpcId, s.EnhanceItemInstanceId,
                s.EnhanceTargetLevel, s.LuckyCharmInstanceId,
                s.InsuranceInstanceId, CancellationToken.None);
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
    }
}
