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
}
