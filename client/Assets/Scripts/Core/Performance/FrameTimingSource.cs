using UnityEngine;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Production <see cref="IFrameTimeSource"/>: reads GPU frame time from
    /// <see cref="FrameTimingManager"/>, falling back to the CPU frame time
    /// where GPU timing is unsupported or reports zero, then to
    /// <see cref="Time.unscaledDeltaTime"/> when no timing is available
    /// (client_performance.md item 7 — GPU read with CPU fallback).
    /// </summary>
    public sealed class FrameTimingSource : IFrameTimeSource
    {
        private const int TimingCapacity = 2;

        private readonly FrameTiming[] _timings = new FrameTiming[TimingCapacity];

        private double _frameSeconds;

        /// <summary>
        /// Captures the newest frame timing. Call once per frame from the
        /// consuming system before reading <see cref="FrameSeconds"/>.
        /// </summary>
        public void Capture()
        {
            FrameTimingManager.CaptureFrameTimings();
            uint count = FrameTimingManager.GetLatestTimings(1, _timings);
            if (count == 0)
            {
                _frameSeconds = Time.unscaledDeltaTime;
                return;
            }

            double gpu = _timings[0].gpuFrameTime;
            double cpu = _timings[0].cpuFrameTime;
            _frameSeconds = gpu > 0.0 ? gpu : cpu > 0.0 ? cpu : Time.unscaledDeltaTime;
        }

        public double FrameSeconds
        {
            get
            {
                return _frameSeconds;
            }
        }
    }
}
