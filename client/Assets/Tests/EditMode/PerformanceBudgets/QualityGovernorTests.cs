using System.Collections.Generic;
using NUnit.Framework;
using ThinhThan.Core.Performance;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Runtime;

namespace ThinhThan.Tests.EditMode.PerformanceBudgets
{
    /// <summary>
    /// PERF-017 governor contract (client_performance.md item 7): 120-frame
    /// p95 window reset each step; step down render scale −0.05 to 0.6 then
    /// particle level −25pp (floors floor(B·n/4), n∈{4,3,2}); step up after
    /// p95 &lt; 75% of target for 10 continuous seconds (particles → B first,
    /// then scale → preset); ≥ 3 s between steps; never above preset;
    /// presentation-only; deterministic injected clock + frame source.
    /// </summary>
    [Category("Performance")]
    public sealed class QualityGovernorTests
    {
        private const double Target = 1.0 / 60.0;

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

        private sealed class FakeFrameSource : IFrameTimeSource
        {
            public double FrameSeconds
            {
                get;
                set;
            }
        }

        private static QualityGovernor NewGovernor(
            FakeClock clock,
            FakeFrameSource source,
            QualityPreset preset = QualityPreset.Medium)
        {
            return new QualityGovernor(preset, Target, clock, source);
        }

        private static void Feed(
            QualityGovernor governor,
            FakeClock clock,
            FakeFrameSource source,
            double frameSeconds,
            int frames,
            double dt = 0.02)
        {
            source.FrameSeconds = frameSeconds;
            for (int i = 0; i < frames; i++)
            {
                governor.ObserveFrame();
                clock.Now += dt;
            }
        }

        [Test]
        public void TestStepDownOnP95Over110()
        {
            var clock = new FakeClock();
            var source = new FakeFrameSource();
            QualityGovernor governor = NewGovernor(clock, source);
            float startScale = governor.Current.RenderScale;

            Feed(governor, clock, source, Target * 1.5, QualityGovernor.WindowFrames);
            Assert.AreEqual(
                startScale - QualityGovernor.RenderScaleStep,
                governor.Current.RenderScale,
                1e-4,
                "p95 > 110% of target must step render scale down by 0.05");
            Assert.AreEqual(
                QualityGovernor.ParticleLevelFull,
                governor.Current.ParticleLevel);

            clock.Now += QualityGovernor.HysteresisSeconds + 0.1;
            Feed(governor, clock, source, Target * 1.5, 10);
            Assert.AreEqual(
                startScale - 2 * QualityGovernor.RenderScaleStep,
                governor.Current.RenderScale,
                1e-4,
                "sustained overload steps again once hysteresis passed");

            for (int step = 0; step < 20; step++)
            {
                clock.Now += QualityGovernor.HysteresisSeconds + 0.1;
                Feed(governor, clock, source, Target * 1.5, 10);
            }

            Assert.AreEqual(
                QualityGovernor.RenderScaleFloor,
                governor.Current.RenderScale,
                1e-4,
                "render scale floors at 0.6 before particle levels drop");
            Assert.Less(
                governor.Current.ParticleLevel,
                QualityGovernor.ParticleLevelFull,
                "after the scale floor, steps down move to particle levels");
        }

        [Test]
        public void TestStepUpAfter10sBelow75()
        {
            var clock = new FakeClock();
            var source = new FakeFrameSource();
            QualityGovernor governor =
                NewGovernor(clock, source, QualityPreset.Low);

            for (int step = 0; step < 12; step++)
            {
                clock.Now += QualityGovernor.HysteresisSeconds + 0.1;
                Feed(governor, clock, source, Target * 1.5, 10);
            }

            Assert.AreEqual(
                QualityGovernor.RenderScaleFloor,
                governor.Current.RenderScale,
                1e-4);
            int degradedLevel = governor.Current.ParticleLevel;
            Assert.Less(degradedLevel, QualityGovernor.ParticleLevelFull);

            Feed(
                governor,
                clock,
                source,
                Target * 0.5,
                200,
                dt: QualityGovernor.StepUpHoldSeconds / 199.0 - 0.01);
            Assert.AreEqual(
                degradedLevel,
                governor.Current.ParticleLevel,
                "no step-up before 10 continuous seconds under 75%");

            clock.Now += QualityGovernor.StepUpHoldSeconds;
            Feed(governor, clock, source, Target * 0.5, 4);
            Assert.AreEqual(
                degradedLevel + 1,
                governor.Current.ParticleLevel,
                "step-up restores particles toward the preset first");
        }

        [Test]
        public void TestHysteresis3s()
        {
            var clock = new FakeClock();
            var source = new FakeFrameSource();
            QualityGovernor governor = NewGovernor(clock, source);
            float first = governor.Current.RenderScale;

            Feed(governor, clock, source, Target * 1.5, 20);
            Assert.AreEqual(
                first - QualityGovernor.RenderScaleStep,
                governor.Current.RenderScale,
                1e-4);

            Feed(governor, clock, source, Target * 1.5, 60);
            Assert.AreEqual(
                first - QualityGovernor.RenderScaleStep,
                governor.Current.RenderScale,
                1e-4,
                "a second step inside 3 s of the first is forbidden");

            clock.Now += QualityGovernor.HysteresisSeconds;
            Feed(governor, clock, source, Target * 1.5, 5);
            Assert.AreEqual(
                first - 2 * QualityGovernor.RenderScaleStep,
                governor.Current.RenderScale,
                1e-4,
                "after 3 s the next step is allowed");
        }

        [Test]
        public void TestFloorsAndPresetCeiling()
        {
            var clock = new FakeClock();
            var source = new FakeFrameSource();
            QualityGovernor governor =
                NewGovernor(clock, source, QualityPreset.Low);
            int baseCeiling = PresetApplier.ParticleCeilingFor(QualityPreset.Low);

            for (int step = 0; step < 24; step++)
            {
                clock.Now += QualityGovernor.HysteresisSeconds + 0.1;
                Feed(governor, clock, source, Target * 1.5, 10);
            }

            Assert.AreEqual(
                QualityGovernor.RenderScaleFloor,
                governor.Current.RenderScale,
                1e-4);
            Assert.AreEqual(
                QualityGovernor.ParticleLevelMin,
                governor.Current.ParticleLevel);
            Assert.AreEqual(
                baseCeiling * QualityGovernor.ParticleLevelMin /
                    QualityGovernor.ParticleLevelFull,
                governor.Current.ParticleCeiling,
                "floor is floor(B·2/4) = 50% of the preset ceiling");

            Feed(
                governor,
                clock,
                source,
                Target * 0.5,
                QualityGovernor.WindowFrames + 4);
            for (int step = 0; step < 12; step++)
            {
                clock.Now += QualityGovernor.StepUpHoldSeconds +
                    QualityGovernor.HysteresisSeconds;
                Feed(governor, clock, source, Target * 0.5, 5);
            }

            Assert.AreEqual(
                PresetApplier.RenderScaleFor(QualityPreset.Low),
                governor.Current.RenderScale,
                1e-4,
                "step-up never exceeds the selected preset");
            Assert.AreEqual(QualityGovernor.ParticleLevelFull, governor.Current.ParticleLevel);
            Assert.AreEqual(baseCeiling, governor.Current.ParticleCeiling);
        }

        [Test]
        public void TestGovernorPresentationOnly()
        {
            var clock = new FakeClock();
            var source = new FakeFrameSource();
            QualityGovernor governor = NewGovernor(clock, source);
            var seen = new List<QualityGovernor.State>();
            governor.Changed += state => seen.Add(state);

            Feed(governor, clock, source, Target * 1.5, 20);
            Assert.IsNotEmpty(seen, "a step publishes the presentation state");

            foreach (QualityGovernor.State state in seen)
            {
                Assert.IsTrue(
                    state.RenderScale >= QualityGovernor.RenderScaleFloor - 1e-4 &&
                    state.RenderScale <= 1.0f + 1e-4,
                    "render scale stays inside [0.6, preset]");
                Assert.IsTrue(
                    state.ParticleLevel >= QualityGovernor.ParticleLevelMin &&
                    state.ParticleLevel <= QualityGovernor.ParticleLevelFull);
                Assert.IsTrue(
                    state.ParticleCeiling >=
                        PresetApplier.ParticleCeilingFor(QualityPreset.Medium) / 2 &&
                    state.ParticleCeiling <=
                        PresetApplier.ParticleCeilingFor(QualityPreset.Medium));
            }

            governor.SetPreset(QualityPreset.High);
            Assert.AreEqual(
                PresetApplier.RenderScaleFor(QualityPreset.High),
                governor.Current.RenderScale,
                "a preset switch resets the level to 100% of the new preset");
            Assert.AreEqual(
                QualityGovernor.ParticleLevelFull,
                governor.Current.ParticleLevel,
                "a preset switch never preserves a higher old budget");
        }
    }
}
