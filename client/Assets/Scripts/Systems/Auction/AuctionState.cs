using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Auction
{
    /// <summary>
    /// Client-side projection of the auction state: the 744 my-state
    /// snapshot (own listings + escrowed assets + proceeds) plus the
    /// latest search page. All state messages replace wholesale.
    /// </summary>
    public sealed class AuctionState
    {
        private readonly List<AuctionListingModel> _listings =
            new List<AuctionListingModel>();
        private readonly List<AuctionEscrowModel> _escrow =
            new List<AuctionEscrowModel>();
        private readonly List<AuctionProceedsModel> _proceeds =
            new List<AuctionProceedsModel>();
        private readonly List<AuctionListingModel> _searchRows =
            new List<AuctionListingModel>();

        public IReadOnlyList<AuctionListingModel> Listings
        {
            get
            {
                return _listings;
            }
        }

        public IReadOnlyList<AuctionEscrowModel> EscrowAssets
        {
            get
            {
                return _escrow;
            }
        }

        public IReadOnlyList<AuctionProceedsModel> Proceeds
        {
            get
            {
                return _proceeds;
            }
        }

        public IReadOnlyList<AuctionListingModel> SearchRows
        {
            get
            {
                return _searchRows;
            }
        }

        private string _nextSearchCursor = "";

        public string NextSearchCursor
        {
            get
            {
                return _nextSearchCursor;
            }
            private set
            {
                _nextSearchCursor = value;
            }
        }

        /// <summary>Replace the whole my-state projection (744).</summary>
        public void Replace(S2CAuctionMyState state)
        {
            _listings.Clear();
            _escrow.Clear();
            _proceeds.Clear();
            if (state == null)
            {
                return;
            }
            foreach (AuctionListingView v in state.Listings)
            {
                _listings.Add(ToModel(v));
            }
            foreach (AuctionEscrowAssetView v in state.EscrowAssets)
            {
                _escrow.Add(new AuctionEscrowModel
                {
                    EscrowAssetId = v.EscrowAssetId.ToByteArray(),
                    ItemId = v.Item != null ? v.Item.ItemId : "",
                    Reason = v.Reason,
                    AutoClaimAtUnixMs = v.AutoClaimAt,
                });
            }
            foreach (AuctionProceedsView v in state.Proceeds)
            {
                _proceeds.Add(new AuctionProceedsModel
                {
                    ProceedsId = v.ProceedsId.ToByteArray(),
                    AmountCommon = v.AmountCommon,
                    State = v.State,
                });
            }
        }

        /// <summary>Swap the search-page projection (739).</summary>
        public void ReplaceSearch(S2CAuctionSearchResult result)
        {
            _searchRows.Clear();
            NextSearchCursor = "";
            if (result == null)
            {
                return;
            }
            foreach (AuctionSearchRow v in result.Rows)
            {
                _searchRows.Add(ToModel(v));
            }
            NextSearchCursor = result.NextPageCursor ?? "";
        }

        /// <summary>
        /// Fold one S2C_AUCTION_SOLD (736) into the projection: the own
        /// listing flips SOLD and the proceeds row appears; the next 744
        /// replaces everything anyway.
        /// </summary>
        public void ApplySold(S2CAuctionSold sold)
        {
            if (sold == null)
            {
                return;
            }
            byte[] lid = sold.ListingId.ToByteArray();
            for (int i = 0; i < _listings.Count; i++)
            {
                if (Equal(_listings[i].ListingId, lid))
                {
                    AuctionListingModel m = _listings[i];
                    m.State = AuctionListingState.Sold;
                    _listings[i] = m;
                }
            }
            _proceeds.Add(new AuctionProceedsModel
            {
                ProceedsId = sold.ProceedsId.ToByteArray(),
                AmountCommon = sold.ProceedsAmount,
                State = AuctionProceedsState.Pending,
            });
        }

        private static AuctionListingModel ToModel(AuctionListingView v)
        {
            return new AuctionListingModel
            {
                ListingId = v.ListingId.ToByteArray(),
                ItemId = v.Item != null ? v.Item.ItemId : "",
                PriceCommon = v.PriceCommon,
                State = v.State,
                ExpiresAtUnixMs = v.ExpiresAt,
            };
        }

        private static AuctionListingModel ToModel(AuctionSearchRow v)
        {
            return new AuctionListingModel
            {
                ListingId = v.ListingId.ToByteArray(),
                ItemId = v.Item != null ? v.Item.ItemId : "",
                PriceCommon = v.PriceCommon,
                State = AuctionListingState.Active,
                ExpiresAtUnixMs = v.ExpiresAt,
            };
        }

        private static bool Equal(byte[] a, byte[] b)
        {
            if (a == null || b == null || a.Length != b.Length)
            {
                return false;
            }
            for (int i = 0; i < a.Length; i++)
            {
                if (a[i] != b[i])
                {
                    return false;
                }
            }
            return true;
        }
    }
}
