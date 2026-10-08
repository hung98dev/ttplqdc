using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Auction;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Auction
{
    /// <summary>
    /// Auction house panel: three pooled row sections (my listings,
    /// escrow assets awaiting reclaim, proceeds awaiting claim) plus
    /// the search results section. Rows come from a Pool so rebuilds
    /// never allocate per apply.
    /// </summary>
    public sealed class AuctionPanel : MonoBehaviour
    {
        public RectTransform? ListingsRoot;
        public RectTransform? EscrowRoot;
        public RectTransform? ProceedsRoot;
        public RectTransform? SearchRoot;
        public AuctionRowView? RowPrefab;
        public TMP_Text? StatusText;
        public Button? SearchButton;
        public TMP_InputField? SearchItemId;
        public IAuctionIntents? Intents
        {
            get;
            set;
        }

        /// <summary>Search page size bound (messages.md 738: 1..50).</summary>
        public const int SearchPageSize = 50;

        private Pool<AuctionRowView>? _pool;
        private readonly List<AuctionRowView> _live =
            new List<AuctionRowView>();
        private bool _searchInFlight;

        /// <summary>Apply-call count (once-per-frame proof for tests).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Live row count across all sections (test introspection).</summary>
        public int LiveCount
        {
            get
            {
                return _live.Count;
            }
        }

        /// <summary>Row i across sections (test introspection).</summary>
        public AuctionRowView RowAt(int i)
        {
            return _live[i];
        }

        private void Awake()
        {
            if (StatusText != null)
            {
                StatusText.raycastTarget = false;
            }
            _pool = new Pool<AuctionRowView>(CreateRow,
                r => r.gameObject.SetActive(false));
            _pool.Prewarm(8);
            if (SearchButton != null)
            {
                SearchButton.onClick.AddListener(OnSearchClicked);
            }
        }

        private AuctionRowView CreateRow()
        {
            AuctionRowView r;
            if (RowPrefab != null)
            {
                r = Instantiate(RowPrefab, transform, false);
            }
            else
            {
                var go = new GameObject("row", typeof(RectTransform),
                    typeof(AuctionRowView));
                r = go.GetComponent<AuctionRowView>();
            }
            r.Intents = Intents;
            return r;
        }

        /// <summary>
        /// One apply per UI phase: rebuild all sections from the
        /// snapshot projection — ACTIVE/other own listings first, then
        /// escrow assets, then PENDING proceeds, then the last search
        /// page.
        /// </summary>
        public void Apply(AuctionState s)
        {
            ApplyCount++;
            _searchInFlight = false;
            if (_pool == null)
            {
                return;
            }
            foreach (AuctionRowView r in _live)
            {
                _pool.Return(r);
            }
            _live.Clear();

            foreach (AuctionListingModel m in s.Listings)
            {
                Spawn(ListingsRoot).Apply(m, true);
            }
            foreach (AuctionEscrowModel m in s.EscrowAssets)
            {
                Spawn(EscrowRoot).ApplyEscrow(m);
            }
            foreach (AuctionProceedsModel m in s.Proceeds)
            {
                Spawn(ProceedsRoot).ApplyProceeds(m);
            }
            foreach (AuctionListingModel m in s.SearchRows)
            {
                Spawn(SearchRoot).Apply(m, false);
            }
            for (int i = 0; i < _live.Count; i++)
            {
                _live[i].gameObject.SetActive(true);
            }
            if (SearchButton != null)
            {
                SearchButton.interactable = Intents != null;
            }
        }

        private AuctionRowView Spawn(RectTransform? root)
        {
            AuctionRowView r = _pool!.Rent();
            r.Intents = Intents;
            if (root != null)
            {
                r.transform.SetParent(root, false);
            }
            _live.Add(r);
            return r;
        }

        private void OnSearchClicked()
        {
            if (Intents == null || _searchInFlight)
            {
                return;
            }
            _searchInFlight = true;
            _ = Intents.RequestSearch(
                SearchItemId != null ? SearchItemId.text : "",
                "", 0, 0, 0,
                Protocol.V1.AuctionSort.PriceAsc, "",
                SearchPageSize,
                System.Threading.CancellationToken.None);
        }
    }
}
