namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// Time source injected into <see cref="FrameLoop"/> and
    /// <see cref="FrameBudget"/> so tests can drive deterministic time.
    /// </summary>
    public interface IClock
    {
        /// <summary>Unscaled seconds since the previous frame.</summary>
        float UnscaledDeltaSeconds
        {
            get;
        }

        /// <summary>Monotonic seconds (realtimeSinceStartupAsDouble).</summary>
        double NowSeconds
        {
            get;
        }
    }
}
