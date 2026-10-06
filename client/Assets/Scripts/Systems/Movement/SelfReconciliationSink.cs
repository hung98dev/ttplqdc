namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Narrow seam so the dispatch can report each stamped client_seq
    /// without depending on the replication assembly shape.
    /// </summary>
    public interface SelfReconciliationSink
    {
        /// <summary>Marks <paramref name="seq"/> as awaiting its self_ack.</summary>
        void NoteInputSent(ulong seq);
    }
}
