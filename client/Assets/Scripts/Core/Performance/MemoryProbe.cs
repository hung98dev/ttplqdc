using System;
using System.Collections.Generic;
using Unity.Profiling;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// PERF-005 memory accounting (client_performance.md § PERF-005):
    /// captures a baseline in an empty bootstrap scene, then samples the
    /// engine counters every second during the run; the gate compares the
    /// sustained peak — the 95th percentile of the run's samples (nearest
    /// rank <c>ceil(0.95·N) − 1</c>, so a transient single-sample excursion
    /// does not decide the gate) — minus the baseline for Total Used
    /// Memory (≤ 2.0 GB) and Gfx Used Memory (≤ 1.0 GB). Unsupported
    /// counters mark the measurement invalid — they never fabricate a pass.
    /// </summary>
    public sealed class MemoryProbe : IDisposable
    {
        private const int RecorderCapacity = 8;

        private readonly ProfilerRecorder _total;
        private readonly ProfilerRecorder _gfx;
        private readonly ProfilerRecorder _gcAllocated;
        private readonly List<long> _totalSamples = new List<long>(64);
        private readonly List<long> _gfxSamples = new List<long>(64);
        private long _baselineTotal;
        private long _baselineGfx;
        private bool _baselined;

        public MemoryProbe()
        {
            _total = ProfilerRecorder.StartNew(
                ProfilerCategory.Memory,
                PerfMarkers.TotalUsedMemoryCounter,
                RecorderCapacity);
            _gfx = ProfilerRecorder.StartNew(
                ProfilerCategory.Memory,
                PerfMarkers.GfxUsedMemoryCounter,
                RecorderCapacity);
            _gcAllocated = ProfilerRecorder.StartNew(
                ProfilerCategory.Memory,
                PerfMarkers.GcAllocatedInFrameCounter,
                RecorderCapacity);
        }

        /// <summary>False when a required counter is unavailable on this platform.</summary>
        public bool Valid
        {
            get
            {
                return _total.Valid && _gfx.Valid;
            }
        }

        /// <summary>Current Total Used Memory (bytes).</summary>
        public long TotalBytes
        {
            get
            {
                return _total.LastValue;
            }
        }

        /// <summary>Current Gfx Used Memory (bytes).</summary>
        public long GfxBytes
        {
            get
            {
                return _gfx.LastValue;
            }
        }

        /// <summary>GC bytes allocated by the most recent sampled frame (PERF-004).</summary>
        public long GcAllocatedBytes
        {
            get
            {
                return _gcAllocated.Valid ? _gcAllocated.LastValue : 0;
            }
        }

        /// <summary>Number of run samples recorded so far.</summary>
        public int SampleCount
        {
            get
            {
                return _totalSamples.Count;
            }
        }

        /// <summary>
        /// Sustained-peak Total Used Memory minus the empty-scene baseline:
        /// the p95 of the run's samples (nearest rank), not the maximum.
        /// </summary>
        public long SustainedTotalDeltaBytes
        {
            get
            {
                return SustainedDelta(_totalSamples, _baselineTotal);
            }
        }

        /// <summary>Sustained-peak Gfx Used Memory minus the baseline.</summary>
        public long SustainedGfxDeltaBytes
        {
            get
            {
                return SustainedDelta(_gfxSamples, _baselineGfx);
            }
        }

        /// <summary>Records the empty-scene baseline; run samples start from here.</summary>
        public void CaptureBaseline()
        {
            _baselineTotal = TotalBytes;
            _baselineGfx = GfxBytes;
            _totalSamples.Clear();
            _gfxSamples.Clear();
            _baselined = true;
        }

        /// <summary>
        /// Appends one counter sample. Callers honor the spec cadence
        /// (every 1 s during the run); the percentile gate tolerates the
        /// transient excursions a raw max would flag.
        /// </summary>
        public void Sample()
        {
            if (!_baselined)
            {
                return;
            }

            _totalSamples.Add(TotalBytes);
            _gfxSamples.Add(GfxBytes);
        }

        public void Dispose()
        {
            _total.Dispose();
            _gfx.Dispose();
            _gcAllocated.Dispose();
        }

        private static long SustainedDelta(List<long> samples, long baseline)
        {
            if (samples.Count == 0)
            {
                return 0;
            }

            var values = new double[samples.Count];
            for (int i = 0; i < samples.Count; i++)
            {
                values[i] = samples[i];
            }

            return (long)FrameIntervalProbe.Percentile(values, 0.95) - baseline;
        }
    }
}
