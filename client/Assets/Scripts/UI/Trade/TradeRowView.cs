using ThinhThan.Systems.Trade;
using TMPro;
using UnityEngine;

namespace ThinhThan.UI.Trade
{
    /// <summary>
    /// One offered-item row: item id and quantity offered by the side
    /// it sits under.
    /// </summary>
    public sealed class TradeRowView : MonoBehaviour
    {
        public TMP_Text? ItemText;
        public TMP_Text? QuantityText;

        public void Apply(TradeOfferItemModel m)
        {
            Set(ItemText, m.ItemId);
            Set(QuantityText, "{0}", m.Quantity);
        }

        private static void Set(TMP_Text? t, string fmt, long n)
        {
            if (t != null)
            {
                t.text = string.Format(fmt, n);
            }
        }

        private static void Set(TMP_Text? t, string s)
        {
            if (t != null)
            {
                t.text = s;
            }
        }
    }
}
