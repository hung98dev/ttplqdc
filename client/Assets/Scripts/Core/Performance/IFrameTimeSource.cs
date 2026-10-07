namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Frame-time sample source injected into <see cref="QualityGovernor"/>
    /// and <see cref="QualityBenchmark"/> (client_performance.md item 7 —
    /// the governor reads GPU frame time with a CPU fallback; tests inject
    /// deterministic sequences instead of real timing).
    /// </summary>
    public interface IFrameTimeSource
    {
        /// <summary>Duration of the most recent complete frame, seconds.</summary>
        double FrameSeconds { get; }
    }
}
