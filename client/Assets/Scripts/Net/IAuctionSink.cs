namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of auction frames (731, 733, 735, 736, 739, 741, 743,
    /// 744) the session driver forwards under lease. Implemented by
    /// Systems/Auction (IMP-030).
    /// </summary>
    public interface IAuctionSink
    {
        void Apply(DecodedFrame frame);
    }
}
