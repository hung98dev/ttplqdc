namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of interact-result frames (116) the session driver
    /// forwards under lease. Implemented by Systems/Discovery
    /// (IMP-020). 116 also reaches <see cref="IWorldSink"/> — sinks
    /// filter by service/interact fields.
    /// </summary>
    public interface IDiscoverySink
    {
        void Apply(DecodedFrame frame);
    }
}
