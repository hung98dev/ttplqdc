using ThinhThan.Systems.Equipment;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Equipment
{
    /// <summary>
    /// One equipment-loadout cell: slot id label + occupied badge +
    /// unequip button (display-only labels; only the button raycasts).
    /// </summary>
    public sealed class EquipmentSlotView : MonoBehaviour
    {
        public TMP_Text? SlotIdText;
        public TMP_Text? ItemText;
        public GameObject? EmptyHint;
        public Button? UnequipButton;

        /// <summary>Canonical slot this cell displays.</summary>
        public string SlotId = "";

        private void Awake()
        {
            if (SlotIdText != null)
            {
                SlotIdText.raycastTarget = false;
            }
            if (ItemText != null)
            {
                ItemText.raycastTarget = false;
            }
        }

        /// <summary>Apply display state for one slot.</summary>
        public void Apply(byte[]? itemInstanceId)
        {
            bool occupied = itemInstanceId != null;
            if (SlotIdText != null)
            {
                SlotIdText.SetText(SlotId);
            }
            if (ItemText != null)
            {
                ItemText.SetText(occupied ? "equipped" : "");
            }
            if (EmptyHint != null)
            {
                EmptyHint.SetActive(!occupied);
            }
            if (UnequipButton != null)
            {
                UnequipButton.gameObject.SetActive(occupied);
            }
        }
    }
}
