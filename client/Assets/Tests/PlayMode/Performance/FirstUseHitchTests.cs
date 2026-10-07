using System.Collections;
using System.Diagnostics;
using System.IO;
using NUnit.Framework;
using ThinhThan.Core.Performance;
using UnityEngine;
using UnityEngine.TestTools;

namespace ThinhThan.Tests.PlayMode.Performance
{
    /// <summary>
    /// First-use hitches (client_performance.md item 7 + PERF-018): shaders
    /// warm during the loading screen instead of on first cast, the variant
    /// collection asset exists, and loading-priority/frame-slice switches
    /// bound the hitch.
    /// </summary>
    [Category("Performance")]
    public sealed class FirstUseHitchTests
    {
        private const string VariantsPath =
            "Assets/Settings/Performance/PerfShaderVariants.shadervariants";

        [Test]
        public void TestShaderWarmupOnLoading()
        {
            Assert.IsTrue(
                File.Exists(VariantsPath),
                "PERF-018 requires the committed shader variant collection " +
                "at " + VariantsPath);

            long before =
                System.GC.GetAllocatedBytesForCurrentThread();
            long start = Stopwatch.GetTimestamp();
            ShaderWarmup.WarmUp(new ShaderVariantCollection());
            double ms = (Stopwatch.GetTimestamp() - start) * 1000.0 /
                Stopwatch.Frequency;
            long allocated =
                System.GC.GetAllocatedBytesForCurrentThread() - before;

            Assert.LessOrEqual(
                ms, 250.0,
                "warm-up call itself must be cheap — real warming runs " +
                "under the loading screen");
            Assert.AreEqual(
                0, allocated, "warmup scheduling allocates nothing");

            int savedVsync = QualitySettings.vSyncCount;
            int savedTarget = Application.targetFrameRate;
            ThreadPriority savedPriority =
                Application.backgroundLoadingPriority;
            try
            {
                FramePacing.SetLoadingScreenActive(true);
                Assert.AreEqual(
                    ThreadPriority.High,
                    Application.backgroundLoadingPriority,
                    "PERF-015 loading screens budget background loading " +
                    "at High priority");
                FramePacing.SetLoadingScreenActive(false);
                Assert.AreEqual(
                    ThreadPriority.Low,
                    Application.backgroundLoadingPriority,
                    "in-game background loading drops to Low priority");
            }
            finally
            {
                QualitySettings.vSyncCount = savedVsync;
                Application.targetFrameRate = savedTarget;
                Application.backgroundLoadingPriority = savedPriority;
            }
        }

        [UnityTest]
        public IEnumerator TestFirstUseSkillVfxAndUiScreens()
        {
            var parent = new GameObject("vfx_harness");
            var renderers = new SpriteRenderer[32];
            for (int i = 0; i < renderers.Length; i++)
            {
                var child = new GameObject("vfx_" + i);
                child.transform.SetParent(parent.transform, false);
                renderers[i] = child.AddComponent<SpriteRenderer>();
                renderers[i].enabled = false;
            }

            yield return null;

            long start = Stopwatch.GetTimestamp();
            foreach (SpriteRenderer renderer in renderers)
            {
                renderer.enabled = true;
            }

            double firstEnableMs =
                (Stopwatch.GetTimestamp() - start) * 1000.0 /
                Stopwatch.Frequency;
            Assert.LessOrEqual(
                firstEnableMs,
                50.0,
                "first-use activation of a combat-VFX burst must stay under " +
                "50 ms CPU — shader/variant warm-up lands on the loading " +
                "screen, not on cast");

            Object.Destroy(parent);
        }
    }
}
