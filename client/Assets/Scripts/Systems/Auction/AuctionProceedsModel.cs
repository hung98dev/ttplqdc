using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Auction
{
    /// <summary>One seller-proceeds row (742 claim target).</summary>
    public struct AuctionProceedsModel
    {
        public byte[] ProceedsId;
        public long AmountCommon;
        public AuctionProceedsState State;
    }
}
