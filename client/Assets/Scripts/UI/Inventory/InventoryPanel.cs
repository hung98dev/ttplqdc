using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Inventory;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Inventory
{
    /// <summary>
    /// Inventory grid panel: pooled slot cells sized to the 433
    /// capacity, a capacity bar, and the expand button priced from the
    /// expansion ladder. SlotView rows come from a Pool so panel
    /// rebuilds never allocate per apply.
    /// </summary>
    public sealed class InventoryPanel : MonoBehaviour
    {
        public RectTransform? GridRoot;
        public InventorySlotView? SlotViewPrefab;
        public Image? CapacityFill;
        public TMP_Text? CapacityText;
        public Button? ExpandButton;
        public TMP_Text? ExpandPriceText;

        /// <summary>
        /// Expansion ladder (economy.md: 60→120 in +10 steps);
        /// index = (capacity - 60) / 10. Beyond the last rung the button
        /// reports full.
        /// </summary>
        public static readonly long[] ExpandPrices =
            { 10000, 25000, 50000, 100000, 200000, 400000 };

        public const int MinCapacity = 60;
        public const int MaxCapacity = 120;
        public const int CapacityStep = 10;

        private Pool<InventorySlotView>? _pool;
        private readonly List<InventorySlotView> _live =
            new List<InventorySlotView>();

        /// <summary>Apply-call count (once-per-frame proof for tests).</summary>
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
        public InventorySlotView CellAt(int i)
        {
            return _live[i];
        }

        private void Awake()
        {
            if (CapacityFill != null)
            {
                CapacityFill.raycastTarget = false;
            }
            if (CapacityText != null)
            {
                CapacityText.raycastTarget = false;
            }
            if (ExpandPriceText != null)
            {
                ExpandPriceText.raycastTarget = false;
            }
            _pool = new Pool<InventorySlotView>(CreateCell,
                c => c.gameObject.SetActive(false));
            _pool.Prewarm(MinCapacity);
        }

        private InventorySlotView CreateCell()
        {
            InventorySlotView c;
            if (SlotViewPrefab != null)
            {
                c = Instantiate(SlotViewPrefab, GridRoot, false);
            }
            else
            {
                var go = new GameObject("slot", typeof(RectTransform),
                    typeof(InventorySlotView));
                if (GridRoot != null)
                {
                    go.transform.SetParent(GridRoot, false);
                }
                c = go.GetComponent<InventorySlotView>();
            }
            return c;
        }

        /// <summary>
        /// One apply per UI phase: rebuilds the grid from the snapshot
        /// model — occupied slots fill front-to-back, empty capacity
        /// cells follow.
        /// </summary>
        public void Apply(InventoryState s)
        {
            ApplyCount++;
            int needed = s.Capacity > 0 ? s.Capacity : MinCapacity;
            EnsureCells(needed);

            int cell = 0;
            foreach (InventorySlotModel m in s.Slots)
            {
                _live[cell].Apply(m);
                cell++;
            }
            var emptyModel = new InventorySlotModel();
            for (; cell < needed; cell++)
            {
                emptyModel.Slot = (uint)cell;
                _live[cell].Apply(emptyModel);
            }
            // Park cells beyond current capacity in the pool.
            for (int i = needed; i < _live.Count; i++)
            {
                _live[i].gameObject.SetActive(false);
            }
            for (int i = 0; i < needed; i++)
            {
                _live[i].gameObject.SetActive(true);
            }

            SetCapacityBar(s);
            SetExpandAffordance(s);
        }

        private void EnsureCells(int needed)
        {
            if (_pool == null)
            {
                return;
            }
            while (_live.Count < needed)
            {
                _live.Add(_pool.Rent());
            }
        }

        private void SetCapacityBar(InventoryState s)
        {
            int used = s.Slots.Count;
            int cap = s.Capacity > 0 ? s.Capacity : MinCapacity;
            if (CapacityFill != null)
            {
                CapacityFill.fillAmount =
                    Mathf.Clamp01((float)used / cap);
            }
            if (CapacityText != null)
            {
                CapacityText.SetText("{0}/{1}", (float)used,
                    (float)cap);
            }
        }

        private void SetExpandAffordance(InventoryState s)
        {
            bool full = s.Capacity >= MaxCapacity;
            if (ExpandButton != null)
            {
                ExpandButton.interactable = !full;
            }
            if (ExpandPriceText != null)
            {
                if (full)
                {
                    ExpandPriceText.SetText("MAX");
                    return;
                }
                int step = (s.Capacity - MinCapacity) / CapacityStep;
                long price = step >= 0 && step < ExpandPrices.Length
                    ? ExpandPrices[step]
                    : 0L;
                ExpandPriceText.SetText("{0}", price);
            }
        }
    }
}
