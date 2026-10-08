namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of interact-result frames (116) the session driver
    /// forwards under lease. Implemented by Systems/Cooking
    /// (IMP-059). 116 also reaches <see cref="IWorldSink"/> — sinks
    /// filter by interact_kind.
    /// </summary>
    public interface ICookingSink
    {
        void Apply(DecodedFrame frame);
    }
}
