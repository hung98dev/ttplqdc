namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of trade frames (701, 704, 706, 709, 710) the session
    /// driver forwards under lease. Implemented by Systems/Trade
    /// (IMP-029).
    /// </summary>
    public interface ITradeSink
    {
        void Apply(DecodedFrame frame);
    }
}
