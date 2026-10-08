using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Trade;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Trade
{
    /// <summary>
    /// Direct-trade panel: two pooled offer columns (own / partner),
    /// a fee/status line, and the confirm / finalise / cancel
    /// actions. Rows come from a Pool so rebuilds never allocate per
    /// apply. Intents are expected at the owning screen's revision —
    /// the panel tracks the last applied revision for the caller.
    /// </summary>
    public sealed class TradePanel : MonoBehaviour
    {
        public RectTransform? OwnRoot;
        public RectTransform? PartnerRoot;
        public TradeRowView? RowPrefab;
        public TMP_Text? StatusText;
        public TMP_Text? FeeText;
        public Button? ConfirmButton;
        public Button? FinaliseButton;
        public Button? CancelButton;

        public ITradeIntents? Intents
        {
            get;
            set;
        }

        private Pool<TradeRowView>? _pool;
        private readonly List<TradeRowView> _live =
            new List<TradeRowView>();
        private byte[] _tradeId = System.Array.Empty<byte>();
        private ulong _revision;

        /// <summary>Apply-call count (once-per-frame proof for tests).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Live row count across both columns (test introspection).</summary>
        public int LiveCount
        {
            get
            {
                return _live.Count;
            }
        }

        /// <summary>Row i across columns (test introspection).</summary>
        public TradeRowView RowAt(int i)
        {
            return _live[i];
        }

        /// <summary>Revision of the last applied 706 (test introspection).</summary>
        public ulong LastRevision
        {
            get
            {
                return _revision;
            }
        }

        /// <summary>Trade id of the last applied session (test introspection).</summary>
        public byte[] LastTradeId
        {
            get
            {
                return _tradeId;
            }
        }

        private void Awake()
        {
            if (StatusText != null)
            {
                StatusText.raycastTarget = false;
            }
            if (FeeText != null)
            {
                FeeText.raycastTarget = false;
            }
            _pool = new Pool<TradeRowView>(CreateRow,
                r => r.gameObject.SetActive(false));
            _pool.Prewarm(8);
            if (ConfirmButton != null)
            {
                ConfirmButton.onClick.AddListener(OnConfirmClicked);
            }
            if (FinaliseButton != null)
            {
                FinaliseButton.onClick.AddListener(OnFinaliseClicked);
            }
            if (CancelButton != null)
            {
                CancelButton.onClick.AddListener(OnCancelClicked);
            }
        }

        private TradeRowView CreateRow()
        {
            TradeRowView r;
            if (RowPrefab != null)
            {
                r = Instantiate(RowPrefab, transform, false);
            }
            else
            {
                var go = new GameObject("row", typeof(RectTransform),
                    typeof(TradeRowView));
                r = go.GetComponent<TradeRowView>();
            }
            return r;
        }

        /// <summary>
        /// One apply per UI phase: rebuild both columns from the 706
        /// projection and drive the action buttons by phase.
        /// </summary>
        public void Apply(TradeState s)
        {
            ApplyCount++;
            if (_pool == null)
            {
                return;
            }
            _tradeId = s.TradeId;
            _revision = s.Revision;
            foreach (TradeRowView r in _live)
            {
                _pool.Return(r);
            }
            _live.Clear();
            if (s.OwnSide.Items != null)
            {
                foreach (TradeOfferItemModel m in s.OwnSide.Items)
                {
                    Spawn(OwnRoot).Apply(m);
                }
            }
            if (s.PartnerSide.Items != null)
            {
                foreach (TradeOfferItemModel m in s.PartnerSide.Items)
                {
                    Spawn(PartnerRoot).Apply(m);
                }
            }
            for (int i = 0; i < _live.Count; i++)
            {
                _live[i].gameObject.SetActive(true);
            }
            Set(FeeText, "{0}", s.FeePreview);
            Set(StatusText, s.Phase.ToString());
            bool open = s.Phase == TradePhase.Open;
            bool locked = s.Phase == TradePhase.Locked;
            if (ConfirmButton != null)
            {
                ConfirmButton.interactable = Intents != null && open;
            }
            if (FinaliseButton != null)
            {
                FinaliseButton.interactable = Intents != null && locked;
            }
            if (CancelButton != null)
            {
                CancelButton.interactable = Intents != null &&
                    (open || locked);
            }
        }

        private TradeRowView Spawn(RectTransform? root)
        {
            TradeRowView r = _pool!.Rent();
            if (root != null)
            {
                r.transform.SetParent(root, false);
            }
            _live.Add(r);
            return r;
        }

        private void OnConfirmClicked()
        {
            if (Intents != null)
            {
                _ = Intents.RequestConfirm(_tradeId, _revision,
                    System.Threading.CancellationToken.None);
            }
        }

        private void OnFinaliseClicked()
        {
            if (Intents != null)
            {
                _ = Intents.RequestFinalise(_tradeId, _revision,
                    System.Threading.CancellationToken.None);
            }
        }

        private void OnCancelClicked()
        {
            if (Intents != null)
            {
                _ = Intents.RequestCancel(_tradeId,
                    System.Threading.CancellationToken.None);
            }
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
