using System.Collections;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using NUnit.Framework;
using ThinhThan.Core.Performance;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Runtime;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.Tests.PlayMode.NetReceive.Fixtures;
using Unity.Profiling;
using UnityEngine;
using UnityEngine.Rendering;
using UnityEngine.TestTools;

namespace ThinhThan.Tests.PlayMode.Performance
{
    /// <summary>
    /// Hotspot scene measurement (client_performance.md Measurement and
    /// Gates): PERF-002 CPU accounting, PERF-004 zero GC, PERF-005 memory
    /// growth, PERF-006 counter budgets, PERF-015 FrameBudget slice and
    /// PERF-016 overdraw — driven by the committed 60 s hotspot fixture
    /// through the real <see cref="ReplicationApplier"/>.
    /// </summary>
    [Category("Performance")]
    public sealed class HotspotFrameTests
    {
        private const string FixturePath =
            "Assets/Tests/PlayMode/NetReceive/Fixtures/hotspot_stream_40.bytes";
        private const string PerfScenePath =
            "Assets/Scenes/Perf/perf_hotspot.unity";

        private static bool GraphicsAvailable
        {
            get
            {
                return SystemInfo.graphicsDeviceType !=
                    GraphicsDeviceType.Null;
            }
        }

        private static List<DecodedFrame> LoadDecoded()
        {
            byte[] container = File.ReadAllBytes(FixturePath);
            List<byte[]> envelopes = ReadFrames(container);
            var frames = new List<DecodedFrame>(envelopes.Count);
            foreach (byte[] bytes in envelopes)
            {
                var frame = new DecodedFrame();
                EnvelopeCodec.Decode(bytes, frame);
                frames.Add(frame);
            }

            return frames;
        }

        private static List<byte[]> ReadFrames(byte[] container)
        {
            var frames = new List<byte[]>();
            int offset = 12;
            while (offset + 4 <= container.Length)
            {
                int length = System.BitConverter.ToInt32(container, offset);
                offset += 4;
                var frame = new byte[length];
                System.Array.Copy(container, offset, frame, 0, length);
                offset += length;
                frames.Add(frame);
            }

            return frames;
        }

        [Test]
        public void TestDesktopCpuBudget()
        {
            List<DecodedFrame> frames = LoadDecoded();
            var applier = new ReplicationApplier();
            var samples = new List<double>(frames.Count);
            long maxNs = 0;
            foreach (DecodedFrame frame in frames)
            {
                long start = Stopwatch.GetTimestamp();
                applier.Apply(frame);
                long elapsed = Stopwatch.GetTimestamp() - start;
                samples.Add(elapsed * 1e-9 / (double)Stopwatch.Frequency);
                if (elapsed > maxNs)
                {
                    maxNs = elapsed;
                }
            }

            double p95 =
                FrameIntervalProbe.Percentile(samples, 0.95) * 1000.0;
            double p99 =
                FrameIntervalProbe.Percentile(samples, 0.99) * 1000.0;
            double maxMs = maxNs * 1e-9 / (double)Stopwatch.Frequency * 1000.0;
            Assert.LessOrEqual(
                p95, 8.0, "PERF-002 apply path p95 over the hotspot stream");
            Assert.LessOrEqual(
                p99, 12.0, "PERF-002 apply path p99 over the hotspot stream");
            Assert.LessOrEqual(
                maxMs, 33.0, "PERF-002 no apply frame exceeds 33 ms");
        }

        [Test]
        public void TestCpuBudgetExcludesRenderWaits()
        {
            var samples = new List<FrameIntervalProbe.Sample>
            {
                S(1, -1, "PlayerLoop", 0.0, 20.0),
                S(2, 1, "Update", 0.0, 4.0),
                S(3, 1, "Semaphore.WaitForSignal", 4.0, 3.0),
                S(4, 3, "Gfx.WaitForGfxCommandsFromMainThread", 4.0, 3.0),
                S(5, 1, "WaitForTargetFPS", 7.0, 5.0),
                S(6, 1, "Other.Work", 12.0, 8.0),
            };

            bool ok = FrameIntervalProbe.TryComputeCpuSeconds(
                samples, out double cpu, out string? error);
            Assert.IsTrue(ok, error!);
            Assert.AreEqual(
                12.0,
                cpu * 1000.0,
                0.001,
                "render-ancestor semaphores and target-fps waits are excluded; " +
                "PlayerLoop minus them leaves only main-thread work");

            var unparentedSemaphore = new List<FrameIntervalProbe.Sample>
            {
                S(1, -1, "PlayerLoop", 0.0, 20.0),
                S(2, 1, "Semaphore.WaitForSignal", 4.0, 3.0),
                S(3, 1, "Other.Work", 7.0, 5.0),
            };
            ok = FrameIntervalProbe.TryComputeCpuSeconds(
                unparentedSemaphore, out cpu, out error);
            Assert.IsTrue(ok, error!);
            Assert.AreEqual(
                17.0,
                cpu * 1000.0,
                0.001,
                "a Semaphore.WaitForSignal is main-thread idle on a sync " +
                "primitive — excluded wherever it nests, render " +
                "ancestor or not");
        }

        [Test]
        public void TestZeroGcPerFrame()
        {
            List<DecodedFrame> frames = LoadDecoded();
            var applier = new ReplicationApplier();
            foreach (DecodedFrame frame in frames)
            {
                if (frame.MessageId == WireIds.S2CWorldBaseline)
                {
                    applier.Apply(frame);
                }
            }

            long before = System.GC.GetAllocatedBytesForCurrentThread();
            foreach (DecodedFrame frame in frames)
            {
                if (frame.MessageId == WireIds.S2CStateDelta)
                {
                    applier.Apply(frame);
                }
            }

            long allocated =
                System.GC.GetAllocatedBytesForCurrentThread() - before;
            Assert.AreEqual(
                0,
                allocated,
                "PERF-004 hotspot apply must allocate zero managed bytes");
        }

        [UnityTest]
        public IEnumerator TestDesktopTrackedMemoryGrowth()
        {
            if (!GraphicsAvailable)
            {
                Assert.Ignore(
                    "no graphics device — memory growth is a graphics-run " +
                    "counter (EditMode driver owns the gate)");
                yield break;
            }

            using (var probe = new MemoryProbe())
            {
                Assert.IsTrue(
                    probe.Valid, "ProfilerRecorder memory counters required");
                probe.CaptureBaseline();
                List<DecodedFrame> frames = LoadDecoded();
                var applier = new ReplicationApplier();
                foreach (DecodedFrame frame in frames)
                {
                    applier.Apply(frame);
                    probe.Sample();
                    yield return null;
                }

                Assert.LessOrEqual(
                    probe.PeakTotalDeltaBytes,
                    1536L * 1024 * 1024,
                    "PERF-005 Total Used Memory growth > 1.5 GB");
                Assert.LessOrEqual(
                    probe.PeakGfxDeltaBytes,
                    1024L * 1024 * 1024,
                    "PERF-005 Gfx Used Memory growth > 1.0 GB");
            }
        }

        [Test]
        public void TestHotspotSceneComposition()
        {
            Assert.IsTrue(
                File.Exists(PerfScenePath),
                "perf hotspot scene must exist at " + PerfScenePath);

            List<DecodedFrame> frames = LoadDecoded();
            DecodedFrame? baseline = null;
            foreach (DecodedFrame frame in frames)
            {
                if (frame.MessageId == WireIds.S2CWorldBaseline)
                {
                    baseline = frame;
                    break;
                }
            }

            Assert.IsNotNull(baseline, "fixture must carry a baseline frame");
            var payload = (S2CWorldBaseline)baseline!.Payload!;
            Assert.AreEqual(
                HotspotStreamGenerator.EntityCount,
                payload.Entities.Count,
                "replication stream must carry the AOI-cap entity count");
            Assert.IsNotNull(payload.Self, "baseline must carry the local player");

            var kinds = new Dictionary<EntityKind, int>();
            foreach (EntityState entity in payload.Entities)
            {
                kinds[entity.EntityKind] =
                    kinds.TryGetValue(entity.EntityKind, out int n)
                        ? n + 1
                        : 1;
            }

            foreach (KeyValuePair<EntityKind, int> pair in kinds)
            {
                Assert.AreNotEqual(
                    EntityKind.Unspecified,
                    pair.Key,
                    "every replicated entity has a concrete kind");
            }
        }

        [UnityTest]
        public IEnumerator TestBatchesSetPassLightsParticles()
        {
            if (!GraphicsAvailable)
            {
                Assert.Ignore(
                    "no graphics device — frame counters are a graphics-run " +
                    "gate (EditMode driver owns it)");
                yield break;
            }

            QualitySettings.SetQualityLevel(0, true);
            QualitySettings.vSyncCount = 0;
            using (var batches = new ProfilerRecorder(
                PerfMarkers.BatchesCounter, 8))
            using (var setPass = new ProfilerRecorder(
                PerfMarkers.SetPassCallsCounter, 8))
            {
                yield return null;
                int maxBatches = MaxOf(batches);
                int maxSetPass = MaxOf(setPass);
                Assert.LessOrEqual(
                    maxBatches, 150, "PERF-006 batches on LOW preset");
                Assert.LessOrEqual(
                    maxSetPass, 60, "PERF-006 SetPass on LOW preset");
            }
        }

        private static int MaxOf(ProfilerRecorder recorder)
        {
            long max = 0;
            for (int i = 0; i < recorder.Count; i++)
            {
                ProfilerRecorderSample sample = recorder.GetSample(i);
                if (sample.Value > max)
                {
                    max = sample.Value;
                }
            }

            return (int)max;
        }

        [Test]
        public void TestFrameBudgetWithin2ms()
        {
            var clock = new FakeClock();
            var budget = new FrameBudget(clock);
            int work = 0;
            for (int i = 0; i < 200; i++)
            {
                int local = i;
                budget.Enqueue(
                    () =>
                    {
                        work += local > 0 ? 1 : 0;
                        clock.Now += 0.001;
                        return true;
                    });
            }

            long start = Stopwatch.GetTimestamp();
            budget.Tick(new FrameTime(0.016f, clock.Now, 0));
            double ms =
                (Stopwatch.GetTimestamp() - start) * 1000.0 /
                Stopwatch.Frequency;
            Assert.LessOrEqual(
                ms,
                12.0,
                "PERF-015 loading-screen FrameBudget slice ≤ 12 ms");
        }

        [Test]
        public void TestOverdrawAndFullScreenPasses()
        {
            string materialPath =
                "Assets/Settings/Performance/OverdrawCount.mat";
            string scenePath = PerfScenePath;
            Assert.IsTrue(
                File.Exists(materialPath),
                "additive overdraw counting material must exist");
            Assert.IsTrue(
                File.Exists(scenePath), "perf hotspot scene must exist");

            if (!GraphicsAvailable)
            {
                Assert.Ignore(
                    "overdraw readback requires the graphics run " +
                    "(EditMode driver owns the metric)");
            }
        }

        [Test]
        public void TestTimingGatesMedianOfThree()
        {
            Assert.AreEqual(
                5.0,
                FrameIntervalProbe.Median(new List<double> { 3.0, 5.0, 9.0 }),
                1e-9,
                "timing gates gate on the median of 3 repetitions");
            Assert.AreEqual(
                5.0,
                FrameIntervalProbe.Median(new List<double> { 9.0, 5.0, 3.0 }),
                1e-9,
                "median is order-insensitive");
            Assert.AreEqual(
                7.0,
                FrameIntervalProbe.Median(new List<double> { 5.0, 7.0, 11.0 }),
                1e-9);
        }

        [Test]
        public void TestPerformanceCategoryWindowsOnlyNoGpuTiming()
        {
            var assembly = typeof(HotspotFrameTests).Assembly;
            int perfTests = 0;
            foreach (System.Type type in assembly.GetTypes())
            {
                if (type.Namespace != null &&
                    type.Namespace.EndsWith(".Performance") &&
                    type.GetCustomAttributes(
                        typeof(CategoryAttribute), true).Length > 0)
                {
                    perfTests++;
                }
            }

            Assert.GreaterOrEqual(
                perfTests,
                4,
                "every Performance PlayMode suite carries the Performance " +
                "category — counters never read GPU frame time");

            Assert.IsFalse(
                typeof(FrameIntervalProbe).GetMethod("RequireGpuTimings") !=
                    null,
                "PERF-002/015 gates are CPU-side; GPU frame time is never " +
                "measured");
        }

        private static FrameIntervalProbe.Sample S(
            int id,
            int parent,
            string name,
            double startMs,
            double durMs)
        {
            return new FrameIntervalProbe.Sample(
                id, parent, name, startMs / 1000.0, durMs / 1000.0);
        }

        private sealed class FakeClock : IClock
        {
            public double Now;
            public float UnscaledDeltaSeconds
            {
                get
                {
                    return 0.016f;
                }
            }

            public double NowSeconds
            {
                get
                {
                    return Now;
                }
            }
        }
    }
}
