using ThinhThan.Systems.Inventory;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Inventory
{
    /// <summary>
    /// One inventory grid cell: quantity + locked badge + enhancement
    /// tag. Apply is display-only (PERF-022: no layout work, no
    /// raycast targets).
    /// </summary>
    public sealed class InventorySlotView : MonoBehaviour
    {
        public TMP_Text? QuantityText;
        public TMP_Text? EnhancementText;
        public TMP_Text? ItemIdText;
        public GameObject? LockedBadge;
        public GameObject? EmptyHint;
        public Button? Hit;

        private readonly char[] _scratch = new char[24];

        /// <summary>Wire slot this cell displays.</summary>
        public uint Slot
        {
            get;
            private set;
        }

        private void Awake()
        {
            // The icon/labels are display-only; only the cell button raycasts.
            if (QuantityText != null)
            {
                QuantityText.raycastTarget = false;
            }
            if (EnhancementText != null)
            {
                EnhancementText.raycastTarget = false;
            }
            if (ItemIdText != null)
            {
                ItemIdText.raycastTarget = false;
            }
        }

        /// <summary>
        /// Applies one slot model: empty slot shows the hint; occupied
        /// shows quantity, lock badge when locked_quantity > 0 (ADR-0064).
        /// </summary>
        public void Apply(InventorySlotModel m)
        {
            Slot = m.Slot;
            bool empty = m.Item == null;
            if (EmptyHint != null)
            {
                EmptyHint.SetActive(empty);
            }
            if (Hit != null)
            {
                Hit.interactable = !empty;
            }
            if (empty)
            {
                SetText(QuantityText, "");
                SetText(EnhancementText, "");
                SetText(ItemIdText, "");
                if (LockedBadge != null)
                {
                    LockedBadge.SetActive(false);
                }
                return;
            }
            InventoryItemModel item = m.Item!;
            SetText(ItemIdText, item.ItemId);
            SetText(QuantityText,
                item.Quantity > 1 ? item.Quantity.ToString() : "");
            SetText(EnhancementText,
                item.EnhancementLevel > 0
                    ? "+" + item.EnhancementLevel.ToString()
                    : "");
            if (LockedBadge != null)
            {
                LockedBadge.SetActive(m.LockedQuantity > 0);
            }
        }

        private void SetText(TMP_Text? target, string s)
        {
            if (target != null)
            {
                target.SetText(s);
            }
        }
    }
}
