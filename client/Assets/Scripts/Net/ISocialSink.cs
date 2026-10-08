namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of social frames (601, 612, 616, 619, 633, 654, 655)
    /// the session driver forwards under lease. Implemented by
    /// Systems/Social (IMP-034).
    /// </summary>
    public interface ISocialSink
    {
        void Apply(DecodedFrame frame);
    }
}
