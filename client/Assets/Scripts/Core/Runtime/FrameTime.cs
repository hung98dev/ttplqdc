namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// One frame of simulation time handed to <see cref="IFrameSystem.Tick"/>.
    /// <see cref="Delta"/> is the unscaled frame delta clamped to
    /// <see cref="MaxDeltaSeconds"/> (100 ms per client_performance.md §
    /// Smoothness by Construction); <see cref="NowSeconds"/> is the monotonic
    /// realtimeSinceStartupAsDouble-style clock.
    /// </summary>
    public readonly struct FrameTime
    {
        public const float MaxDeltaSeconds = 0.1f;

        public FrameTime(float unscaledDeltaSeconds, double nowSeconds, long frameIndex)
        {
            Delta = unscaledDeltaSeconds < 0f
                ? 0f
                : unscaledDeltaSeconds > MaxDeltaSeconds
                    ? MaxDeltaSeconds
                    : unscaledDeltaSeconds;
            NowSeconds = nowSeconds;
            FrameIndex = frameIndex;
        }

        public float Delta
        {
            get;
        }

        public double NowSeconds
        {
            get;
        }

        public long FrameIndex
        {
            get;
        }
    }
}
