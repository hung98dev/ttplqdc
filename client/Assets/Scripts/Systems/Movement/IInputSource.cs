namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Semantic input surface for <see cref="MovementInputDispatch"/>
    /// (real producer = IMP-066 input composition). One sample per frame:
    /// the current held flags plus every discrete edge pressed since the
    /// last sample.
    /// </summary>
    public interface IInputSource
    {
        /// <summary>Held direction now: -1 left, 0 none, +1 right.</summary>
        int HeldDirection
        {
            get;
        }

        /// <summary>
        /// Opaque input_flags bits now (F-04 — bit packing is owned by the
        /// input producer, never reinterpreted here). 0 when nothing held.
        /// </summary>
        uint HeldFlags
        {
            get;
        }

        /// <summary>
        /// Fills <paramref name="edges"/> with discrete edges queued since
        /// the last sample; returns the count written (may exceed capacity —
        /// the caller drains until 0).
        /// </summary>
        int SampleEdges(LocalEdge[] edges);
    }
}
