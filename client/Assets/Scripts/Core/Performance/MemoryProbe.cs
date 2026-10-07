using System;
using Unity.Profiling;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// PERF-005 memory accounting (client_performance.md § PERF-005):
    /// captures a baseline in an empty bootstrap scene, then samples the
    /// engine counters every second during the run; the gate compares
    /// peak-minus-baseline for Total Used Memory (≤ 1.5 GB) and Gfx Used
    /// Memory (≤ 1.0 GB). Unsupported counters mark the measurement
    /// invalid — they never fabricate a pass.
    /// </summary>
    public sealed class MemoryProbe : IDisposable
    {
        private const int RecorderCapacity = 8;

        private readonly ProfilerRecorder _total;
        private readonly ProfilerRecorder _gfx;
        private readonly ProfilerRecorder _gcAllocated;
        private long _baselineTotal;
        private long _baselineGfx;
        private long _peakTotal;
        private long _peakGfx;
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
            get { return _total.Valid && _gfx.Valid; }
        }

        /// <summary>Current Total Used Memory (bytes).</summary>
        public long TotalBytes
        {
            get { return _total.LastValue; }
        }

        /// <summary>Current Gfx Used Memory (bytes).</summary>
        public long GfxBytes
        {
            get { return _gfx.LastValue; }
        }

        /// <summary>GC bytes allocated by the most recent sampled frame (PERF-004).</summary>
        public long GcAllocatedBytes
        {
            get { return _gcAllocated.Valid ? _gcAllocated.LastValue : 0; }
        }

        /// <summary>Peak Total Used Memory minus the empty-scene baseline.</summary>
        public long PeakTotalDeltaBytes
        {
            get { return _peakTotal - _baselineTotal; }
        }

        /// <summary>Peak Gfx Used Memory minus the empty-scene baseline.</summary>
        public long PeakGfxDeltaBytes
        {
            get { return _peakGfx - _baselineGfx; }
        }

        /// <summary>Records the empty-scene baseline and resets peaks to it.</summary>
        public void CaptureBaseline()
        {
            _baselineTotal = TotalBytes;
            _baselineGfx = GfxBytes;
            _peakTotal = _baselineTotal;
            _peakGfx = _baselineGfx;
            _baselined = true;
        }

        /// <summary>
        /// Samples the counters once per second during the run; tracks the
        /// peak. Callers honor the spec cadence (every 1 s during the run).
        /// </summary>
        public void Sample()
        {
            if (!_baselined)
            {
                return;
            }

            long total = TotalBytes;
            if (total > _peakTotal)
            {
                _peakTotal = total;
            }

            long gfx = GfxBytes;
            if (gfx > _peakGfx)
            {
                _peakGfx = gfx;
            }
        }

        public void Dispose()
        {
            _total.Dispose();
            _gfx.Dispose();
            _gcAllocated.Dispose();
        }
    }
}
