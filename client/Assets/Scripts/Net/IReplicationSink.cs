namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of replication frames (300..308, 107, 206, 207) the session
    /// driver forwards under lease. Implemented by Systems/Replication.
    /// </summary>
    public interface IReplicationSink
    {
        void Apply(DecodedFrame frame);

        /// <summary>Baseline/lifecycle invalidated: drop all retained state.</summary>
        void Invalidate();
    }
}
