namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of combat action/status frames (203, 204, 205, 304) the
    /// session driver forwards under lease. Implemented by
    /// Systems/Combat (IMP-014). 204 also reaches
    /// <see cref="IWorldSink"/> — each sink filters by
    /// request_message_id.
    /// </summary>
    public interface ICombatSink
    {
        void Apply(DecodedFrame frame);
    }
}
