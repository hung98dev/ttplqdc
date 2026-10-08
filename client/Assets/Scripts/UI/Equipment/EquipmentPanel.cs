using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Equipment;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Equipment
{
    /// <summary>
    /// Equipment loadout panel: the 14 canonical slots of the visible
    /// loadout, loadout tabs (primary/secondary_1/secondary_2), the
    /// ACTIVE badge and the latest 403 verdict line. Unequip / switch
    /// buttons fire intents only — the server adjudicates (402); the
    /// panel re-renders only on a new authoritative result.
    /// </summary>
    public sealed class EquipmentPanel : MonoBehaviour
    {
        /// <summary>The 14 canonical slots in catalog order.</summary>
        public static readonly string[] SlotIds =
        {
            "weapon", "head", "body", "hands", "legs", "feet",
            "necklace", "ring", "costume", "talisman", "jade",
            "seal", "relic", "charm",
        };

        public RectTransform? GridRoot;
        public EquipmentSlotView? SlotViewPrefab;
        public TMP_Text? ActiveBadgeText;
        public TMP_Text? ResultText;
        public Button? SwitchButton;
        public IEquipmentIntents? Intents
        {
            get;
            set;
        }

        /// <summary>Loadout currently displayed.</summary>
        public string VisibleLoadoutId = EquipmentState.LoadoutIds[0];

        private Pool<EquipmentSlotView>? _pool;
        private readonly List<EquipmentSlotView> _live =
            new List<EquipmentSlotView>();
        private int _lastRequestedSlot = -1;

        /// <summary>Apply-call count (once-per-version proof).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Live cell count (test introspection).</summary>
        public int LiveCount
        {
            get
            {
                return _live.Count;
            }
        }

        /// <summary>Cell i (test introspection).</summary>
        public EquipmentSlotView CellAt(int i)
        {
            return _live[i];
        }

        private void Awake()
        {
            if (ActiveBadgeText != null)
            {
                ActiveBadgeText.raycastTarget = false;
            }
            if (ResultText != null)
            {
                ResultText.raycastTarget = false;
            }
            _pool = new Pool<EquipmentSlotView>(CreateCell,
                c => c.gameObject.SetActive(false));
            _pool.Prewarm(SlotIds.Length);
            if (SwitchButton != null)
            {
                SwitchButton.onClick.AddListener(OnSwitchClicked);
            }
        }

        private EquipmentSlotView CreateCell()
        {
            EquipmentSlotView c;
            if (SlotViewPrefab != null)
            {
                c = Instantiate(SlotViewPrefab, GridRoot, false);
            }
            else
            {
                var go = new GameObject("slot", typeof(RectTransform),
                    typeof(EquipmentSlotView));
                if (GridRoot != null)
                {
                    go.transform.SetParent(GridRoot, false);
                }
                c = go.GetComponent<EquipmentSlotView>();
            }
            return c;
        }

        /// <summary>Rebuild rows once per applied authoritative
        /// state — never per frame.</summary>
        public void Apply(EquipmentState s)
        {
            ApplyCount++;
            EnsureCells(SlotIds.Length);
            for (int i = 0; i < SlotIds.Length; i++)
            {
                _live[i].SlotId = SlotIds[i];
                _live[i].Apply(s.ItemAt(VisibleLoadoutId, SlotIds[i]));
                int captured = i;
                if (_live[i].UnequipButton != null)
                {
                    _live[i].UnequipButton.onClick.RemoveAllListeners();
                    _live[i].UnequipButton.onClick.AddListener(
                        () => OnUnequipClicked(captured));
                }
            }
            if (ActiveBadgeText != null)
            {
                ActiveBadgeText.SetText(
                    s.ActiveLoadoutId == VisibleLoadoutId
                        ? "ACTIVE" : "SUPPORT");
            }
        }

        /// <summary>Displays the latest 403 verdict (error or ok).</summary>
        public void ApplyResult(ThinhThan.Protocol.V1.S2CLoadoutResult r)
        {
            if (ResultText == null || r.Result == null)
            {
                return;
            }
            ResultText.SetText(r.Result.ErrorCode ==
                ThinhThan.Protocol.V1.ErrorCode.Unspecified
                    ? "ok" : r.Result.ErrorCode.ToString());
        }

        private void EnsureCells(int needed)
        {
            while (_live.Count < needed)
            {
                EquipmentSlotView c = _pool!.Get();
                c.gameObject.SetActive(true);
                _live.Add(c);
            }
        }

        private void OnUnequipClicked(int slotIndex)
        {
            if (Intents == null || slotIndex < 0)
            {
                return;
            }
            _lastRequestedSlot = slotIndex;
            _ = Intents.RequestUnequip(VisibleLoadoutId,
                SlotIds[slotIndex], gameObject.GetCancellationTokenOnDestroy());
        }

        private void OnSwitchClicked()
        {
            if (Intents == null)
            {
                return;
            }
            _ = Intents.RequestSwitchActive(VisibleLoadoutId,
                gameObject.GetCancellationTokenOnDestroy());
        }

        /// <summary>Last unequip request slot (test introspection).</summary>
        public int LastRequestedSlot
        {
            get
            {
                return _lastRequestedSlot;
            }
        }
    }
}
