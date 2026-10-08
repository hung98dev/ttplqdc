using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Auction
{
    /// <summary>One listing/search row projection for the UI.</summary>
    public struct AuctionListingModel
    {
        public byte[] ListingId;
        public string ItemId;
        public long PriceCommon;
        public AuctionListingState State;
        public long ExpiresAtUnixMs;
    }

    /// <summary>One escrowed-asset row (740 reclaim target).</summary>
    public struct AuctionEscrowModel
    {
        public byte[] EscrowAssetId;
        public string ItemId;
        public AuctionEscrowReason Reason;
        public long AutoClaimAtUnixMs;
    }

    /// <summary>One seller-proceeds row (742 claim target).</summary>
    public struct AuctionProceedsModel
    {
        public byte[] ProceedsId;
        public long AmountCommon;
        public AuctionProceedsState State;
    }
}
