using System.Collections;
using System.Collections.Generic;
using System.IO;
using NUnit.Framework;
using ThinhThan.Core.Performance;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Runtime;
using ThinhThan.Net;
using ThinhThan.Systems.Replication;
using UnityEditor;
using UnityEditor.SceneManagement;
using UnityEditor.Profiling;
using UnityEditorInternal;
using UnityEngine;
using UnityEngine.Rendering;
using UnityEngine.SceneManagement;
using UnityEngine.TestTools;
using UnityEngine.Rendering.Universal;
using Unity.Profiling;

namespace ThinhThan.Tests.EditMode.PerformanceBudgets
{
    /// <summary>
    /// Graphics-backed performance driver (client_performance.md Measurement
    /// and Gates, ADR-0059): capability probe (PERF-016 prerequisite),
    /// PERF-002 interval-union CPU accounting on real frames, PERF-004 zero
    /// GC, PERF-005 tracked-memory growth, PERF-006 batch/light/particle
    /// budgets on the LOW preset, PERF-015 FrameBudget slice, PERF-018
    /// first-use hitch and PERF-016 additive overdraw into an RFloat target
    /// gated by a known-overlap calibration fixture. Every gate fails rather
    /// than skipping when the device is real; a Null device ignores.
    /// </summary>
    [Category("Performance")]
    public sealed class HotspotPerfDriverTests
    {
        private const string ScenePath = "Assets/Scenes/Perf/perf_hotspot.unity";
        private const string FixturePath =
            "Assets/Tests/PlayMode/NetReceive/Fixtures/hotspot_stream_40.bytes";
        private const string FixturePrefabPath =
            "Assets/Settings/Performance/OverdrawFixture.prefab";
        private const string OverdrawMaterialPath =
            "Assets/Settings/Performance/OverdrawCount.mat";
        private const string ShaderVariantsPath =
            "Assets/Settings/Performance/PerfShaderVariants.shadervariants";

        private const int ReplayFramesPerDelta = 6;
        private const int WarmupFrames = 600;
        private const int Repetitions = 3;
        private const int MeasureFramesPerRep = 600;

        private bool _epoPushed;
        private bool _epoWasEnabled;
        private EnterPlayModeOptions _epoPrevious;

        private static bool GraphicsAvailable
        {
            get
            {
                return SystemInfo.graphicsDeviceType !=
                    GraphicsDeviceType.Null;
            }
        }

        /// <summary>
        /// EditMode runs must not reload the domain on play-mode entry:
        /// an unscheduled assembly reload aborts the UTF run ("unexpected
        /// assembly reload") and leaves the editor stuck in play mode.
        /// </summary>
        private void EnterNoReloadPlayMode()
        {
            _epoWasEnabled = EditorSettings.enterPlayModeOptionsEnabled;
            _epoPrevious = EditorSettings.enterPlayModeOptions;
            _epoPushed = true;
            EditorSettings.enterPlayModeOptionsEnabled = true;
            EditorSettings.enterPlayModeOptions =
                EnterPlayModeOptions.DisableDomainReload |
                EnterPlayModeOptions.DisableSceneReload;
            EditorApplication.EnterPlaymode();
        }

        [UnityTearDown]
        public IEnumerator RestorePlayModeState()
        {
            if (EditorApplication.isPlaying ||
                EditorApplication.isPlayingOrWillChangePlaymode)
            {
                EditorApplication.ExitPlaymode();
            }

            while (EditorApplication.isPlaying)
            {
                yield return null;
            }

            if (_epoPushed)
            {
                EditorSettings.enterPlayModeOptionsEnabled = _epoWasEnabled;
                EditorSettings.enterPlayModeOptions = _epoPrevious;
                _epoPushed = false;
            }
        }

        [Test]
        public void ProbeVerifiesCapability()
        {
            if (!GraphicsAvailable)
            {
                Assert.Ignore(
                    "no graphics device — Performance requires -force-d3d11");
            }

            GraphicsCapabilityProbe.VerifyCurrentInvocation();
            Assert.Pass("graphics capability probe passed");
        }

        [Test]
        public void TestPerfAssetsExist()
        {
            Assert.IsNotNull(
                AssetDatabase.LoadAssetAtPath<ShaderVariantCollection>(
                    ShaderVariantsPath),
                "perf ShaderVariantCollection asset must exist");
            Material material = AssetDatabase.LoadAssetAtPath<Material>(
                OverdrawMaterialPath);
            Assert.IsNotNull(material, "OverdrawCount material must exist");
            Assert.IsNotNull(material.shader);
            Assert.IsNotNull(
                AssetDatabase.LoadAssetAtPath<GameObject>(FixturePrefabPath),
                "known-overlap calibration prefab must exist");
        }

        [UnityTest]
        [Timeout(900000)]
        public IEnumerator HotspotCountersMeetBudgets()
        {
            if (!GraphicsAvailable)
            {
                Assert.Ignore(
                    "no graphics device — Performance requires -force-d3d11");
                yield break;
            }

            GraphicsCapabilityProbe.VerifyCurrentInvocation();

            EnterNoReloadPlayMode();
            while (!EditorApplication.isPlaying)
            {
                yield return null;
            }

            var cpuByRep = new List<CpuWindow>();
            long gcBytes = 0;
            long peakTotalDelta = 0;
            long peakGfxDelta = 0;
            int maxBatches = 0;
            int maxSetPass = 0;
            int maxPointLights = 0;
            int maxParticles = 0;
            double maxFrameBudgetMs = 0.0;

            using (var memory = new MemoryProbe())
            using (var gc = new ProfilerRecorder(
                PerfMarkers.GcAllocatedInFrameCounter, 16))
            using (var batches = new ProfilerRecorder(
                PerfMarkers.BatchesCounter, 8))
            using (var setPass = new ProfilerRecorder(
                PerfMarkers.SetPassCallsCounter, 8))
            {
                LoadScene();
                // Single-mode load unloads the previous scene at the
                // next frame boundary; any object found/created before
                // then lives in the dying scene.
                yield return null;
                QualitySettings.SetQualityLevel(0, true);
                QualitySettings.vSyncCount = 0;
                Application.targetFrameRate = -1;

                HotspotRig rig = BuildRig();
                memory.CaptureBaseline();
                yield return null;

                for (int i = 0; i < WarmupFrames; i++)
                {
                    rig.Tick();
                    yield return null;
                }

                for (int rep = 0; rep < Repetitions; rep++)
                {
                    var cpu = new CpuWindow();
                    for (int i = 0; i < MeasureFramesPerRep; i++)
                    {
                        rig.Tick();
                        yield return null;
                        if (TryMeasureLastFrame(out double cpuSeconds))
                        {
                            cpu.Add(cpuSeconds);
                        }

                        gcBytes += SumRecorder(gc);
                        maxBatches = Mathf.Max(
                            maxBatches, MaxRecorderSample(batches));
                        maxSetPass = Mathf.Max(
                            maxSetPass, MaxRecorderSample(setPass));
                        maxPointLights = Mathf.Max(
                            maxPointLights, rig.ActivePointLightCount);
                        maxParticles = Mathf.Max(
                            maxParticles, rig.LiveParticles);
                        memory.Sample();
                        if (rig.LastFrameBudgetMs > maxFrameBudgetMs)
                        {
                            maxFrameBudgetMs = rig.LastFrameBudgetMs;
                        }
                    }

                    cpuByRep.Add(cpu);
                }

                peakTotalDelta = memory.PeakTotalDeltaBytes;
                peakGfxDelta = memory.PeakGfxDeltaBytes;
                rig.Destroy();
            }

            AssertHotspot(
                cpuByRep,
                gcBytes,
                peakTotalDelta,
                peakGfxDelta,
                maxBatches,
                maxSetPass,
                maxPointLights,
                maxParticles,
                maxFrameBudgetMs);

            EditorApplication.ExitPlaymode();
            while (EditorApplication.isPlaying)
            {
                yield return null;
            }
        }

        [UnityTest]
        [Timeout(600000)]
        public IEnumerator OverdrawMetricsAndFullScreenPasses()
        {
            if (!GraphicsAvailable)
            {
                Assert.Ignore(
                    "no graphics device — Performance requires -force-d3d11");
                yield break;
            }

            GraphicsCapabilityProbe.VerifyCurrentInvocation();

            EnterNoReloadPlayMode();
            while (!EditorApplication.isPlaying)
            {
                yield return null;
            }

            LoadScene();
            // Same deferred-unload hazard as HotspotCountersMeetBudgets:
            // yield so the swap completes before objects are resolved.
            yield return null;
            QualitySettings.SetQualityLevel(0, true);
            QualitySettings.vSyncCount = 0;

            Material counting =
                AssetDatabase.LoadAssetAtPath<Material>(OverdrawMaterialPath);
            GameObject fixturePrefab =
                AssetDatabase.LoadAssetAtPath<GameObject>(FixturePrefabPath);
            Camera camera = Object.FindAnyObjectByType<Camera>();
            Assert.IsNotNull(camera, "perf scene must contain a camera");

            GameObject fixture =
                Object.Instantiate(fixturePrefab);
            yield return null;

            float[] calibration = RenderOverdraw(camera, counting);
            AssertCalibration(calibration);
            Object.Destroy(fixture);
            yield return null;

            HotspotRig rig = BuildRig();
            for (int i = 0; i < 120; i++)
            {
                rig.Tick();
                yield return null;
            }

            float[] hotspot = RenderOverdraw(camera, counting);
            int fullScreenPasses = CountFullScreenPasses();
            rig.Destroy();

            AssertOverdraw(hotspot, fullScreenPasses);

            EditorApplication.ExitPlaymode();
            while (EditorApplication.isPlaying)
            {
                yield return null;
            }
        }

        private static void AssertCalibration(float[] pixels)
        {
            bool hasOne = false;
            bool hasTwo = false;
            bool hasThree = false;
            foreach (float v in pixels)
            {
                if (v < 0 || float.IsNaN(v) || float.IsInfinity(v))
                {
                    Assert.Fail(
                        "PERF-016 readback produced invalid count " + v);
                }

                if (Mathf.Approximately(v, 1f))
                {
                    hasOne = true;
                }
                else if (Mathf.Approximately(v, 2f))
                {
                    hasTwo = true;
                }
                else if (v >= 2.5f)
                {
                    hasThree = true;
                }
            }

            Assert.IsTrue(
                hasOne && hasTwo && hasThree,
                "PERF-016 calibration fixture must produce counts " +
                "1, 2 and 3 (base, single overlap, double overlap) — " +
                "readback or counting material is broken");
        }

        private static void AssertOverdraw(
            float[] pixels,
            int fullScreenPasses)
        {
            var sorted = new List<double>(pixels.Length);
            double sum = 0.0;
            foreach (float v in pixels)
            {
                Assert.IsTrue(
                    v >= 0 && !float.IsNaN(v) && !float.IsInfinity(v),
                    "PERF-016 readback must be finite and nonnegative");
                sorted.Add(v);
                sum += v;
            }

            double avg = sum / sorted.Count;
            double p99 = FrameIntervalProbe.Percentile(sorted, 0.99);
            Assert.LessOrEqual(
                avg, 2.5, "PERF-016 average overdraw at 1280x720 LOW");
            Assert.LessOrEqual(
                p99, 8.0, "PERF-016 p99 pixel overdraw at 1280x720 LOW");
            Assert.LessOrEqual(
                fullScreenPasses,
                1,
                "PERF-016 full-screen passes on LOW <= 1");
        }

        private static float[] RenderOverdraw(
            Camera camera,
            Material counting)
        {
            var previous =
                new Dictionary<SpriteRenderer, Material?>();
            SpriteRenderer[] renderers =
                Object.FindObjectsByType<SpriteRenderer>();
            foreach (SpriteRenderer renderer in renderers)
            {
                previous[renderer] = renderer.sharedMaterial;
                renderer.sharedMaterial = counting;
            }

            var previousParticles =
                new Dictionary<ParticleSystemRenderer, Material?>();
            ParticleSystemRenderer[] particleRenderers =
                Object.FindObjectsByType<ParticleSystemRenderer>();
            foreach (ParticleSystemRenderer renderer in particleRenderers)
            {
                previousParticles[renderer] = renderer.sharedMaterial;
                renderer.sharedMaterial = counting;
            }

            var target = new RenderTexture(
                1280, 720, 0, UnityEngine.RenderTextureFormat.RFloat);
            RenderTexture? previousTarget = camera.targetTexture;
            RenderTexture? previousActive = RenderTexture.active;
            try
            {
                camera.targetTexture = target;
                camera.Render();
                RenderTexture.active = target;
                var readback = new Texture2D(
                    1280, 720, TextureFormat.RFloat, false);
                try
                {
                    readback.ReadPixels(
                        new Rect(0, 0, 1280, 720), 0, 0);
                    readback.Apply();
                    Color[] colors = readback.GetPixels();
                    var counts = new float[colors.Length];
                    for (int i = 0; i < colors.Length; i++)
                    {
                        counts[i] = colors[i].r;
                    }

                    return counts;
                }
                finally
                {
                    Object.Destroy(readback);
                }
            }
            finally
            {
                foreach (KeyValuePair<SpriteRenderer, Material?> pair in
                    previous)
                {
                    if (pair.Key != null)
                    {
                        pair.Key.sharedMaterial = pair.Value;
                    }
                }

                foreach (
                    KeyValuePair<ParticleSystemRenderer, Material?> pair in
                    previousParticles)
                {
                    if (pair.Key != null)
                    {
                        pair.Key.sharedMaterial = pair.Value;
                    }
                }

                camera.targetTexture = previousTarget;
                RenderTexture.active = previousActive;
                target.Release();
                Object.Destroy(target);
            }
        }

        private static int CountFullScreenPasses()
        {
            int frameIndex = ProfilerDriver.lastFrameIndex;
            int passes = 0;
            for (int ti = 0; ti < 64; ti++)
            {
                using HierarchyFrameDataView view =
                    ProfilerDriver.GetHierarchyFrameDataView(
                        frameIndex, ti,
                        HierarchyFrameDataView.ViewModes.Default,
                        HierarchyFrameDataView.columnDontSort, false);
                if (!view.valid)
                {
                    continue;
                }

                int rootId = view.GetRootItemID();
                if (view.GetItemName(rootId) != PerfMarkers.PlayerLoopMarker)
                {
                    continue;
                }

                var stack = new List<int> { rootId };
                var childBuf = new List<int>(32);
                while (stack.Count > 0)
                {
                    int id = stack[stack.Count - 1];
                    stack.RemoveAt(stack.Count - 1);
                    string name = view.GetItemName(id);
                    if (IsFullScreenPassMarker(name))
                    {
                        passes++;
                    }

                    childBuf.Clear();
                    view.GetItemChildren(id, childBuf);
                    for (int i = 0; i < childBuf.Count; i++)
                    {
                        stack.Add(childBuf[i]);
                    }
                }
            }

            return passes;
        }

        private static bool IsFullScreenPassMarker(string name)
        {
            return name == "FinalBlitPass" ||
                name == "PostProcessPass" ||
                name == "UberPostProcess" ||
                name == "BloomPass" ||
                name == "CopyColorPass" ||
                name == "CopyDepthPass";
        }

        private static void AssertHotspot(
            List<CpuWindow> cpuByRep,
            long gcBytes,
            long peakTotalDelta,
            long peakGfxDelta,
            int maxBatches,
            int maxSetPass,
            int maxPointLights,
            int maxParticles,
            double maxFrameBudgetMs)
        {
            var failures = new List<string>();
            if (cpuByRep.Count != Repetitions || HasEmptyWindow(cpuByRep))
            {
                failures.Add("PERF-002 frame capture produced no samples");
            }
            else
            {
                var p95s = new List<double>();
                var p99s = new List<double>();
                var maxes = new List<double>();
                foreach (CpuWindow window in cpuByRep)
                {
                    p95s.Add(window.P95);
                    p99s.Add(window.P99);
                    maxes.Add(window.Max);
                }

                double medianP95 = FrameIntervalProbe.Median(p95s);
                double medianP99 = FrameIntervalProbe.Median(p99s);
                double medianMax = FrameIntervalProbe.Median(maxes);
                if (!(medianP95 <= 0.008))
                {
                    failures.Add(
                        "PERF-002 p95 " + FmtMs(medianP95) + " > 8 ms");
                }

                if (!(medianP99 <= 0.012))
                {
                    failures.Add(
                        "PERF-002 p99 " + FmtMs(medianP99) + " > 12 ms");
                }

                if (!(medianMax <= 0.033))
                {
                    failures.Add(
                        "PERF-002 max frame " + FmtMs(medianMax) +
                        " > 33 ms");
                }
            }

            if (gcBytes != 0)
            {
                failures.Add(
                    "PERF-004 GC allocated " + gcBytes +
                    " bytes in steady gameplay");
            }

            if (peakTotalDelta > 1536L * 1024 * 1024)
            {
                failures.Add(
                    "PERF-005 Total Used Memory growth " +
                    (peakTotalDelta / (1024 * 1024)) + " MB > 1536 MB");
            }

            if (peakGfxDelta > 1024L * 1024 * 1024)
            {
                failures.Add(
                    "PERF-005 Gfx Used Memory growth " +
                    (peakGfxDelta / (1024 * 1024)) + " MB > 1024 MB");
            }

            if (maxBatches > 150)
            {
                failures.Add("PERF-006 batches " + maxBatches + " > 150");
            }

            if (maxSetPass > 60)
            {
                failures.Add("PERF-006 SetPass " + maxSetPass + " > 60");
            }

            int lightCap = PresetApplier.PointLightLimitFor(QualityPreset.Low);
            if (maxPointLights > lightCap)
            {
                failures.Add(
                    "PERF-006 active point Light2D " + maxPointLights +
                    " > " + lightCap);
            }

            int particleCap =
                PresetApplier.ParticleCeilingFor(QualityPreset.Low);
            if (maxParticles > particleCap)
            {
                failures.Add(
                    "PERF-006 live particles " + maxParticles +
                    " > " + particleCap);
            }

            if (maxFrameBudgetMs > 2.0)
            {
                failures.Add(
                    "PERF-015 FrameBudget " +
                    maxFrameBudgetMs.ToString("0.00") + " ms > 2 ms");
            }

            Assert.IsEmpty(
                failures,
                "hotspot budget violations:\n" + string.Join("\n", failures));
        }

        private static bool HasEmptyWindow(List<CpuWindow> windows)
        {
            foreach (CpuWindow window in windows)
            {
                if (window.Count == 0)
                {
                    return true;
                }
            }

            return false;
        }

        private static string FmtMs(double seconds)
        {
            return (seconds * 1000.0).ToString("0.00") + " ms";
        }

        private static long SumRecorder(ProfilerRecorder recorder)
        {
            long sum = 0;
            for (int i = 0; i < recorder.Count; i++)
            {
                sum += recorder.GetSample(i).Value;
            }

            return sum;
        }

        private static int MaxRecorderSample(ProfilerRecorder recorder)
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

        private static void LoadScene()
        {
            Scene scene = EditorSceneManager.LoadSceneInPlayMode(
                ScenePath,
                new LoadSceneParameters(LoadSceneMode.Single));
            Assert.IsTrue(scene.IsValid(), "perf hotspot scene must load");
        }

        /// <summary>
        /// Extracts the last completed main-thread frame via
        /// <see cref="ProfilerDriver"/> and runs PERF-002 union accounting
        /// through <see cref="FrameIntervalProbe"/>.
        /// </summary>
        private static bool TryMeasureLastFrame(out double cpuSeconds)
        {
            cpuSeconds = 0.0;
            int frameIndex = ProfilerDriver.lastFrameIndex;
            for (int ti = 0; ti < 64; ti++)
            {
                using HierarchyFrameDataView view =
                    ProfilerDriver.GetHierarchyFrameDataView(
                        frameIndex, ti,
                        HierarchyFrameDataView.ViewModes.Default,
                        HierarchyFrameDataView.columnDontSort, false);
                if (!view.valid)
                {
                    continue;
                }

                int rootId = view.GetRootItemID();
                if (view.GetItemName(rootId) != PerfMarkers.PlayerLoopMarker)
                {
                    continue;
                }

                var samples = new List<FrameIntervalProbe.Sample>(512);
                var stack = new List<(int id, int parentId)>(64)
                {
                    (rootId, -1),
                };
                var childBuf = new List<int>(32);
                while (stack.Count > 0)
                {
                    (int id, int parentId) = stack[stack.Count - 1];
                    stack.RemoveAt(stack.Count - 1);
                    samples.Add(new FrameIntervalProbe.Sample(
                        id,
                        parentId,
                        view.GetItemName(id),
                        view.GetItemColumnDataAsDouble(
                            id, HierarchyFrameDataView.columnStartTime) *
                            0.001,
                        view.GetItemColumnDataAsDouble(
                            id, HierarchyFrameDataView.columnTotalTime) *
                            0.001));
                    childBuf.Clear();
                    view.GetItemChildren(id, childBuf);
                    for (int i = 0; i < childBuf.Count; i++)
                    {
                        stack.Add((childBuf[i], id));
                    }
                }

                return FrameIntervalProbe.TryComputeCpuSeconds(
                    samples, out cpuSeconds, out _);
            }

            return false;
        }

        private static HotspotRig BuildRig()
        {
            var go = GameObject.Find("HotspotEntities");
            Transform entities = go != null
                ? go.transform
                : new GameObject("HotspotEntities").transform;
            var rig = new HotspotRig(entities);
            rig.Load(File.ReadAllBytes(FixturePath));
            rig.ApplyBaselineAndViews();
            rig.SpawnEmitters();
            return rig;
        }

        private sealed class CpuWindow
        {
            private readonly List<double> _values = new List<double>(4096);

            public int Count
            {
                get
                {
                    return _values.Count;
                }
            }

            public void Add(double seconds)
            {
                _values.Add(seconds);
            }

            public double P95
            {
                get
                {
                    return FrameIntervalProbe.Percentile(_values, 0.95);
                }
            }

            public double P99
            {
                get
                {
                    return FrameIntervalProbe.Percentile(_values, 0.99);
                }
            }

            public double Max
            {
                get
                {
                    double max = 0.0;
                    for (int i = 0; i < _values.Count; i++)
                    {
                        if (_values[i] > max)
                        {
                            max = _values[i];
                        }
                    }

                    return max;
                }
            }
        }

        private sealed class UnityClock : IClock
        {
            public float UnscaledDeltaSeconds
            {
                get
                {
                    return Time.unscaledDeltaTime;
                }
            }

            public double NowSeconds
            {
                get
                {
                    return Time.realtimeSinceStartupAsDouble;
                }
            }
        }

        /// <summary>
        /// Drives the recorded hotspot stream through the real
        /// <see cref="ReplicationApplier"/> and updates one pooled sprite view
        /// per replicated entity inside the scene's HotspotEntities root.
        /// </summary>
        private sealed class HotspotRig
        {
            private readonly ReplicationApplier _applier =
                new ReplicationApplier();
            private readonly FrameBudget _frameBudget;
            private readonly ParticleBudget _particleBudget;
            private readonly PointLightBudget _lightBudget =
                new PointLightBudget();
            private readonly List<SpriteRenderer> _views =
                new List<SpriteRenderer>(64);
            private readonly List<ParticleSystem> _emitters =
                new List<ParticleSystem>(8);
            private readonly List<DecodedFrame> _baselines =
                new List<DecodedFrame>(4);
            private readonly List<DecodedFrame> _deltas =
                new List<DecodedFrame>(1024);
            private readonly Transform _root;
            private readonly UnityClock _clock = new UnityClock();
            private Sprite? _sprite;
            private int _deltaCursor;
            private int _tick;
            private int _syncOffset;

            public HotspotRig(Transform root)
            {
                _root = root;
                _frameBudget = new FrameBudget(_clock);
                _particleBudget = new ParticleBudget(QualityPreset.Low);
            }

            public int ActivePointLightCount
            {
                get
                {
                    return _lightBudget.Count;
                }
            }

            public int LiveParticles
            {
                get
                {
                    return _particleBudget.LiveCount;
                }
            }

            public double LastFrameBudgetMs
            {
                get;
                private set;
            }

            public int StoreCount
            {
                get
                {
                    return _applier.Store.Count;
                }
            }

            public void Load(byte[] container)
            {
                List<byte[]> envelopes = ReadFrames(container);
                Assert.IsNotEmpty(
                    envelopes, "hotspot fixture must contain frames");
                foreach (byte[] bytes in envelopes)
                {
                    var frame = new DecodedFrame();
                    EnvelopeCodec.Decode(bytes, frame);
                    if (frame.MessageId == WireIds.S2CWorldBaseline)
                    {
                        _baselines.Add(frame);
                    }
                    else if (frame.MessageId == WireIds.S2CStateDelta)
                    {
                        _deltas.Add(frame);
                    }
                }

                Assert.IsNotEmpty(
                    _deltas, "hotspot fixture must contain deltas");
            }

            public void ApplyBaselineAndViews()
            {
                foreach (DecodedFrame frame in _baselines)
                {
                    _applier.Apply(frame);
                }

                SyncAllViews();
            }

            public void Tick()
            {
                if (_tick % ReplayFramesPerDelta == 0 && _deltas.Count > 0)
                {
                    _applier.Apply(_deltas[_deltaCursor % _deltas.Count]);
                    _deltaCursor++;
                }

                _tick++;
                EnsureViews(_applier.Store.Count);
                _syncOffset = 0;
                _frameBudget.Enqueue(DrainSyncSlice);
                double before = _clock.NowSeconds;
                _frameBudget.Tick(
                    new FrameTime(0.016f, before, _tick));
                LastFrameBudgetMs =
                    (_clock.NowSeconds - before) * 1000.0;
            }

            private bool DrainSyncSlice()
            {
                EntityViewStore store = _applier.Store;
                const int slice = 16;
                int end = _syncOffset + slice;
                for (
                    int i = _syncOffset;
                    i < end && i < store.Count && i < _views.Count;
                    i++)
                {
                    (double xmm, double ymm) = _applier.RenderPosition(i);
                    _views[i].transform.localPosition = new Vector3(
                        (float)(xmm / 1000.0),
                        (float)(ymm / 1000.0),
                        0f);
                }

                _syncOffset = end;
                int limit = store.Count < _views.Count
                    ? store.Count
                    : _views.Count;
                return _syncOffset >= limit;
            }

            public void SpawnEmitters()
            {
                for (int i = 0; i < 8; i++)
                {
                    var go = new GameObject("perf_emitter_" + i);
                    go.transform.SetParent(_root, false);
                    var ps = go.AddComponent<ParticleSystem>();
                    ParticleSystem.MainModule main = ps.main;
                    main.maxParticles = 64;
                    main.startLifetime = 0.5f;
                    main.simulationSpeed = 1f;
                    _emitters.Add(ps);
                    _particleBudget.Register(ps);
                }

                for (int i = 0; i < 4; i++)
                {
                    var go = new GameObject("perf_light_" + i);
                    go.transform.SetParent(_root, false);
                    var light = go.AddComponent<Light2D>();
                    light.lightType = Light2D.LightType.Point;
                    light.pointLightInnerRadius = 0.5f;
                    light.pointLightOuterRadius = 3f;
                    _lightBudget.Register(light);
                }
            }

            public void Destroy()
            {
                for (int i = 0; i < _views.Count; i++)
                {
                    if (_views[i] != null)
                    {
                        Object.Destroy(_views[i].gameObject);
                    }
                }

                _views.Clear();
            }

            private void SyncAllViews()
            {
                EntityViewStore store = _applier.Store;
                EnsureViews(store.Count);
                for (int i = 0; i < store.Count; i++)
                {
                    (double xmm, double ymm) = _applier.RenderPosition(i);
                    _views[i].transform.localPosition = new Vector3(
                        (float)(xmm / 1000.0),
                        (float)(ymm / 1000.0),
                        0f);
                }
            }

            private void EnsureViews(int count)
            {
                while (_views.Count < count)
                {
                    var go = new GameObject("perf_entity_" + _views.Count);
                    go.transform.SetParent(_root, false);
                    var renderer = go.AddComponent<SpriteRenderer>();
                    renderer.sprite = SharedSprite();
                    _views.Add(renderer);
                }
            }

            private Sprite SharedSprite()
            {
                if (_sprite == null)
                {
                    var texture = new Texture2D(16, 16);
                    _sprite = Sprite.Create(
                        texture,
                        new Rect(0, 0, 16, 16),
                        new Vector2(0.5f, 0.5f),
                        32f);
                }

                return _sprite;
            }

            private static List<byte[]> ReadFrames(byte[] container)
            {
                var frames = new List<byte[]>();
                int offset = 12;
                while (offset + 4 <= container.Length)
                {
                    int length = System.BitConverter.ToInt32(
                        container, offset);
                    offset += 4;
                    var frame = new byte[length];
                    System.Array.Copy(container, offset, frame, 0, length);
                    offset += length;
                    frames.Add(frame);
                }

                return frames;
            }
        }
    }
}
