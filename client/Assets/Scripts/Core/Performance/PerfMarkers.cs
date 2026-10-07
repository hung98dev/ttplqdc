using System;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Profiler marker names, recorder counter names and marker classification
    /// shared by the performance budget types (client_performance.md —
    /// PERF-002 interval accounting, PERF-005 memory counters, PERF-006
    /// batch counters, PERF-004 GC counters).
    /// </summary>
    public static class PerfMarkers
    {
        /// <summary>Root profiler marker of one complete main-thread frame.</summary>
        public const string PlayerLoopMarker = "PlayerLoop";

        /// <summary>ProfilerRecorder counter: managed memory used (bytes).</summary>
        public const string TotalUsedMemoryCounter = "Total Used Memory";

        /// <summary>ProfilerRecorder counter: graphics driver memory used (bytes).</summary>
        public const string GfxUsedMemoryCounter = "Gfx Used Memory";

        /// <summary>ProfilerRecorder counter: managed GC bytes allocated this frame.</summary>
        public const string GcAllocatedInFrameCounter = "GC Allocated In Frame";

        /// <summary>ProfilerRecorder counter: draw batches submitted this frame.</summary>
        public const string BatchesCounter = "Batches";

        /// <summary>ProfilerRecorder counter: shader/property state switches this frame.</summary>
        public const string SetPassCallsCounter = "SetPass Calls";

        /// <summary>Exact marker name excluded as a vSync/idle wait (PERF-002).</summary>
        public const string WaitForTargetFpsMarker = "WaitForTargetFPS";

        /// <summary>Exact marker name conditionally excluded (PERF-002).</summary>
        public const string SemaphoreWaitMarker = "Semaphore.WaitForSignal";

        /// <summary>
        /// True for marker names that are always excluded from main-thread
        /// gameplay CPU accounting: <c>Gfx.*</c>, <c>Camera.Render</c>,
        /// <c>Render.*</c> and <c>WaitForTargetFPS</c>. Semaphore waits are
        /// excluded separately and only under a rendering ancestor.
        /// </summary>
        public static bool IsAlwaysExcludedMarker(string name)
        {
            if (name == null)
            {
                return false;
            }

            return name.StartsWith("Gfx.", StringComparison.Ordinal) ||
                name.Equals("Camera.Render", StringComparison.Ordinal) ||
                name.StartsWith("Render.", StringComparison.Ordinal) ||
                name.Equals(WaitForTargetFpsMarker, StringComparison.Ordinal);
        }

        /// <summary>
        /// True for marker names that identify rendering work on an ancestor
        /// chain — used to decide whether a <c>Semaphore.WaitForSignal</c>
        /// descendant is a rendering wait (PERF-002 exclusion rule).
        /// </summary>
        public static bool IsRenderAncestorMarker(string name)
        {
            if (name == null)
            {
                return false;
            }

            return name.StartsWith("Gfx.", StringComparison.Ordinal) ||
                name.Equals("Camera.Render", StringComparison.Ordinal) ||
                name.StartsWith("Render.", StringComparison.Ordinal);
        }
    }
}
