using System.Collections.Generic;
using NUnit.Framework;
using ThinhThan.Core.Performance;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.Performance
{
    /// <summary>
    /// PERF-001 device tiers + PERF-013 battery saver (client_performance.md
    /// Device Tiers, item 3): the 5 s first-launch benchmark selects
    /// LOW/MEDIUM/HIGH, persisted presets apply presentation-only values
    /// (render scale, Light2D budget, particle budget, parallax L3, bloom),
    /// and the battery-saver toggle caps every tier at 30 FPS.
    /// </summary>
    [Category("Performance")]
    public sealed class QualityPresetTests
    {
        private const double Target = 1.0 / 60.0;

        private sealed class FakeClock : IClock
        {
            public double Now;
            public float UnscaledDeltaSeconds
            {
                get { return 0.016f; }
            }

            public double NowSeconds
            {
                get { return Now; }
            }
        }

        private sealed class FakeFrameSource : IFrameTimeSource
        {
            public double FrameSeconds { get; set; }
        }

        private int _savedTargetFrameRate;

        [SetUp]
        public void SaveGlobals()
        {
            _savedTargetFrameRate = Application.targetFrameRate;
        }

        [TearDown]
        public void RestoreGlobals()
        {
            Application.targetFrameRate = _savedTargetFrameRate;
        }

        [Test]
        public void TestBenchmarkSelectsPreset()
        {
            AssertPreset(
                frameSeconds: 1.0 / 240.0,
                expected: QualityPreset.High);
            AssertPreset(
                frameSeconds: 1.0 / 90.0,
                expected: QualityPreset.Medium);
            AssertPreset(
                frameSeconds: 1.0 / 30.0,
                expected: QualityPreset.Low);

            var storage = new InMemorySessionStorage();
            QualityBenchmark.Persist(storage, QualityPreset.Medium);
            Assert.IsTrue(
                QualityBenchmark.TryGetPersisted(storage, out QualityPreset persisted));
            Assert.AreEqual(QualityPreset.Medium, persisted);
            Assert.IsFalse(
                QualityBenchmark.TryGetPersisted(
                    new InMemorySessionStorage(), out _));
        }

        private static void AssertPreset(
            double frameSeconds,
            QualityPreset expected)
        {
            var clock = new FakeClock();
            var source = new FakeFrameSource
            {
                FrameSeconds = frameSeconds,
            };
            var benchmark = new QualityBenchmark(clock, source);
            QualityPreset? result = null;
            int frames =
                (int)(QualityBenchmark.DurationSeconds / frameSeconds) + 8;
            for (int i = 0; i < frames && !result.HasValue; i++)
            {
                result = benchmark.ObserveFrame();
                clock.Now += frameSeconds;
            }

            Assert.AreEqual(
                expected,
                result,
                "p95 " + (frameSeconds * 1000.0) + " ms must select " +
                expected);
        }

        [Test]
        public void TestPresetsPresentationOnly()
        {
            int qualityLevel = -1;
            float renderScale = -1f;
            int lightLimit = -1;
            Vector2 lightCenter = default;
            int particleCeiling = -1;
            bool? parallax = null;
            bool? bloom = null;
            var sinks = new PresetApplier.Sinks
            {
                SetQualityLevel = v => qualityLevel = v,
                SetRenderScale = v => renderScale = v,
                ApplyLightBudget = (n, c) =>
                {
                    lightLimit = n;
                    lightCenter = c;
                },
                SetParticleCeiling = v => particleCeiling = v,
                SetParallaxL3Visible = v => parallax = v,
                SetBloomEnabled = v => bloom = v,
            };

            PresetApplier.Apply(QualityPreset.Low, sinks, Vector2.zero);
            Assert.AreEqual(0, qualityLevel);
            Assert.AreEqual(0.75f, renderScale, 1e-4);
            Assert.AreEqual(4, lightLimit);
            Assert.AreEqual(512, particleCeiling);
            Assert.AreEqual(false, parallax, "L3 hidden on LOW");
            Assert.AreEqual(false, bloom, "bloom is HIGH-only");

            PresetApplier.Apply(QualityPreset.Medium, sinks, Vector2.one);
            Assert.AreEqual(1, qualityLevel);
            Assert.AreEqual(1.0f, renderScale, 1e-4);
            Assert.AreEqual(8, lightLimit);
            Assert.AreEqual(1024, particleCeiling);
            Assert.AreEqual(true, parallax);
            Assert.AreEqual(false, bloom);

            PresetApplier.Apply(QualityPreset.High, sinks, Vector2.one);
            Assert.AreEqual(2, qualityLevel);
            Assert.AreEqual(1.0f, renderScale, 1e-4);
            Assert.AreEqual(16, lightLimit);
            Assert.AreEqual(2048, particleCeiling);
            Assert.AreEqual(true, parallax);
            Assert.AreEqual(true, bloom);

            Assert.AreEqual(
                Vector2.one,
                lightCenter,
                "the light budget sink receives the view center");
        }

        [Test]
        public void TestBatterySaverCaps30()
        {
            var storage = new InMemorySessionStorage();
            var saver = new BatterySaver(storage);

            foreach (
                QualityPreset preset in
                new[]
                {
                    QualityPreset.Low,
                    QualityPreset.Medium,
                    QualityPreset.High,
                })
            {
                saver.SetBaselineFrameRate(BaselineFor(preset));
                saver.Enabled = true;
                Assert.AreEqual(
                    BatterySaver.CapFrameRate,
                    saver.EffectiveFrameRate,
                    preset + " must cap at 30 with battery saver on");
                Assert.AreEqual(
                    BatterySaver.CapFrameRate,
                    Application.targetFrameRate);
                saver.Enabled = false;
                Assert.AreEqual(
                    BaselineFor(preset),
                    saver.EffectiveFrameRate,
                    preset + " restores the tier baseline when disabled");
            }
        }

        private static int BaselineFor(QualityPreset preset)
        {
            switch (preset)
            {
                case QualityPreset.Low: return 30;
                case QualityPreset.Medium: return 60;
                default: return 120;
            }
        }
    }
}
