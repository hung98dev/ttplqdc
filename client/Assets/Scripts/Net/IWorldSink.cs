namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of world transfer/interact result frames (110, 116, 204)
    /// the session driver forwards under lease. Implemented by
    /// Systems/World (IMP-018).
    /// </summary>
    public interface IWorldSink
    {
        void Apply(DecodedFrame frame);
    }
}
