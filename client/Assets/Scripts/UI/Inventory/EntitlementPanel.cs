using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Inventory;
using UnityEngine;

namespace ThinhThan.UI.Inventory
{
    /// <summary>
    /// Entitlement panel (435): pooled rows per entitlement. Rows the
    /// server marks refund-revoked still display — they are excluded
    /// from the equippable set by state, not hidden.
    /// </summary>
    public sealed class EntitlementPanel : MonoBehaviour
    {
        public RectTransform? RowsRoot;
        public EntitlementRowView? RowPrefab;
        public IInventoryIntents? Intents;

        /// <summary>419 in-flight: claim buttons idle until verdict.</summary>
        public bool ClaimInFlight;

        private Pool<EntitlementRowView>? _pool;
        private readonly List<EntitlementRowView> _live =
            new List<EntitlementRowView>();

        public int ApplyCount
        {
            get;
            private set;
        }

        public int LiveCount
        {
            get
            {
                return _live.Count;
            }
        }

        public EntitlementRowView RowAt(int i)
        {
            return _live[i];
        }

        private void Awake()
        {
            _pool = new Pool<EntitlementRowView>(CreateRow,
                r => r.gameObject.SetActive(false));
        }

        private EntitlementRowView CreateRow()
        {
            if (RowPrefab != null)
            {
                return Instantiate(RowPrefab, RowsRoot, false);
            }
            var go = new GameObject("row", typeof(RectTransform),
                typeof(EntitlementRowView));
            if (RowsRoot != null)
            {
                go.transform.SetParent(RowsRoot, false);
            }
            return go.GetComponent<EntitlementRowView>();
        }

        public void Apply(EntitlementPanelState s)
        {
            ApplyCount++;
            if (_pool == null)
            {
                return;
            }
            while (_live.Count < s.Rows.Count)
            {
                _live.Add(_pool.Rent());
            }
            for (int i = 0; i < s.Rows.Count; i++)
            {
                EntitlementRowModel m = s.Rows[i];
                EntitlementRowView row = _live[i];
                row.Apply(m, ClaimInFlight);
                row.gameObject.SetActive(true);
                if (row.ClaimButton != null && Intents != null &&
                    m.ClaimableTierIds.Count > 0)
                {
                    byte[] entitlementId = m.EntitlementId;
                    string tierId = m.ClaimableTierIds[0];
                    row.ClaimButton.onClick.RemoveAllListeners();
                    row.ClaimButton.onClick.AddListener(() =>
                    {
                        ClaimInFlight = true;
                        _ = Intents.RequestClaim(
                            entitlementId, tierId,
                            System.Threading.CancellationToken.None);
                    });
                }
            }
            for (int i = s.Rows.Count; i < _live.Count; i++)
            {
                _live[i].gameObject.SetActive(false);
            }
        }
    }
}
