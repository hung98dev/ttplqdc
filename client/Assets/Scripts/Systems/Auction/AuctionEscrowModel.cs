using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Auction
{
    /// <summary>One escrowed-asset row (740 reclaim target).</summary>
    public struct AuctionEscrowModel
    {
        public byte[] EscrowAssetId;
        public string ItemId;
        public AuctionEscrowReason Reason;
        public long AutoClaimAtUnixMs;
    }
}
