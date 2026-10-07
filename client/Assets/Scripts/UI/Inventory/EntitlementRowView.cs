using ThinhThan.Systems.Inventory;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Inventory
{
    /// <summary>
    /// One entitlement row of the 435 panel: product label, grant-state
    /// badge, claimable tier count. Claim clicks fire intents — the
    /// 419 verdict lands back through the applier.
    /// </summary>
    public sealed class EntitlementRowView : MonoBehaviour
    {
        public TMP_Text? ProductText;
        public TMP_Text? StateText;
        public TMP_Text? TiersText;
        public Button? ClaimButton;

        private void Awake()
        {
            foreach (TMP_Text t in
                GetComponentsInChildren<TMP_Text>(true))
            {
                t.raycastTarget = false;
            }
        }

        public void Apply(EntitlementRowModel m, bool claimInFlight)
        {
            SetText(ProductText, m.ProductId);
            SetText(StateText, m.GrantState.ToString());
            if (TiersText != null)
            {
                int claimable = m.ClaimableTierIds.Count;
                int claimed = m.ClaimedTierIds.Count;
                TiersText.SetText("{0}/{1}", (float)claimed,
                    (float)(claimed + claimable));
            }
            if (ClaimButton != null)
            {
                ClaimButton.interactable = !claimInFlight &&
                    m.PanelEquippable && m.ClaimableTierIds.Count > 0;
            }
        }

        private static void SetText(TMP_Text? t, string s)
        {
            if (t != null)
            {
                t.SetText(s);
            }
        }
    }
}
