using NUnit.Framework;
using ThinhThan.Core.Performance;
using UnityEditor;
using UnityEngine;
using UnityEngine.Scripting;

namespace ThinhThan.Tests.EditMode.PerformanceBudgets
{
    /// <summary>
    /// PERF-019 frame-rate control (client_performance.md item 3): desktop
    /// vSync=1 with targetFrameRate unset, Android vSync=0 with tier target +
    /// Optimized Frame Pacing, incremental GC 1 ms slice, Physics2D Script.
    /// PERF-013 battery saver caps every tier at 30 FPS.
    /// </summary>
    [Category("Performance")]
    public sealed class FramePacingSettingsTests
    {
        private int _savedVSync;
        private int _savedTargetFrameRate;
        private ulong _savedGcSlice;
        private SimulationMode2D _savedSimulationMode;
        private ThreadPriority _savedLoadingPriority;

        [SetUp]
        public void SaveGlobals()
        {
            _savedVSync = QualitySettings.vSyncCount;
            _savedTargetFrameRate = Application.targetFrameRate;
            _savedGcSlice = GarbageCollector.incrementalTimeSliceNanoseconds;
            _savedSimulationMode = Physics2D.simulationMode;
            _savedLoadingPriority = Application.backgroundLoadingPriority;
        }

        [TearDown]
        public void RestoreGlobals()
        {
            QualitySettings.vSyncCount = _savedVSync;
            Application.targetFrameRate = _savedTargetFrameRate;
            GarbageCollector.incrementalTimeSliceNanoseconds = _savedGcSlice;
            Physics2D.simulationMode = _savedSimulationMode;
            Application.backgroundLoadingPriority = _savedLoadingPriority;
        }

        [Test]
        public void TestDesktopVsync()
        {
            FramePacing.ApplyDesktop();
            Assert.AreEqual(1, QualitySettings.vSyncCount);
            Assert.AreEqual(
                -1,
                Application.targetFrameRate,
                "desktop leaves targetFrameRate unset");
        }

        [Test]
        public void TestAndroidTargetFrameRateAndOptimizedPacing()
        {
            FramePacing.ApplyAndroid(60);
            Assert.AreEqual(0, QualitySettings.vSyncCount);
            Assert.AreEqual(60, Application.targetFrameRate);
            Assert.IsTrue(
                PlayerSettings.optimizedFramePacing,
                "PERF-019 requires Optimized Frame Pacing enabled for Android");
        }

        [Test]
        public void TestIncrementalGcSlice()
        {
            FramePacing.ApplyCommon();
            Assert.AreEqual(
                FramePacing.IncrementalGcSliceNanoseconds,
                GarbageCollector.incrementalTimeSliceNanoseconds);
        }

        [Test]
        public void TestPhysics2DScriptMode()
        {
            FramePacing.ApplyCommon();
            Assert.AreEqual(
                SimulationMode2D.Script,
                Physics2D.simulationMode,
                "the client has no authoritative physics; Physics2D is script-driven");
        }

        [Test]
        public void TestLoadingPriorityToggle()
        {
            FramePacing.ApplyCommon();
            FramePacing.SetLoadingScreenActive(true);
            Assert.AreEqual(
                ThreadPriority.High,
                Application.backgroundLoadingPriority);
            FramePacing.SetLoadingScreenActive(false);
            Assert.AreEqual(
                ThreadPriority.Low,
                Application.backgroundLoadingPriority);
        }
    }
}
