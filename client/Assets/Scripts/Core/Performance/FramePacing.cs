using UnityEngine;
using UnityEngine.Scripting;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Platform frame-pacing application (client_performance.md PERF-019,
    /// items 3–4): desktop runs vSync with no frame-rate cap; Android runs
    /// vSync off at the tier target with Optimized Frame Pacing (project
    /// setting); incremental GC time slice is one millisecond; 2D physics
    /// runs in Script mode so its step is driven by the frame loop;
    /// background loading stays Low in gameplay and only rises on loading
    /// screens.
    /// </summary>
    public static class FramePacing
    {
        /// <summary>Incremental GC slice in nanoseconds (PERF-019).</summary>
        public const ulong IncrementalGcSliceNanoseconds = 1000000;

        /// <summary>Desktop pacing: vSyncCount=1, targetFrameRate unset.</summary>
        public static void ApplyDesktop()
        {
            QualitySettings.vSyncCount = 1;
            Application.targetFrameRate = -1;
        }

        /// <summary>Android pacing: vSyncCount=0 + tier frame-rate target.</summary>
        public static void ApplyAndroid(int tierFrameRate)
        {
            QualitySettings.vSyncCount = 0;
            Application.targetFrameRate = tierFrameRate;
        }

        /// <summary>
        /// Shared runtime pacing: incremental GC 1 ms slice and Physics2D
        /// Script simulation mode.
        /// </summary>
        public static void ApplyCommon()
        {
            GarbageCollector.incrementalTimeSliceNanoseconds =
                IncrementalGcSliceNanoseconds;
            Physics2D.simulationMode = SimulationMode2D.Script;
        }

        /// <summary>
        /// Loading-screen scheduling (item 4): High priority while on a
        /// loading screen, Low everywhere else.
        /// </summary>
        public static void SetLoadingScreenActive(bool onLoadingScreen)
        {
            Application.backgroundLoadingPriority = onLoadingScreen
                ? ThreadPriority.High
                : ThreadPriority.Low;
        }
    }
}
