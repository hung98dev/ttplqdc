using NUnit.Framework;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.Tests.PlayMode.Harness;

namespace ThinhThan.Tests.PlayMode.Performance
{
    /// <summary>
    /// PERF-010/011 smoothness (client_performance.md Smoothness on the
    /// Network): interp delay = 2 snapshot intervals (200 ms at 10 Hz)
    /// adapted within 150–300 ms, extrapolation capped at 250 ms, error
    /// ≤0.5 m smoothed over 100 ms / snapped above, and correction rate
    /// ≤1/min at full quality.
    /// </summary>
    [Category("Performance")]
    public sealed class NetworkSmoothnessTests
    {
        [Test]
        public void TestInterpolationExtrapolationCorrection()
        {
            var buffer = new SnapshotBuffer();
            Assert.AreEqual(
                SnapshotBuffer.BaseDelaySeconds,
                buffer.DelaySeconds,
                1e-9,
                "interp delay starts at 2 snapshot intervals (200 ms)");

            double t = 0.0;
            for (int i = 0; i < 8; i++)
            {
                buffer.Push(
                    serverTick: (ulong)(i + 1),
                    xMm: 1000 * i,
                    yMm: 0,
                    vxMmS: 500,
                    vyMmS: 0,
                    nowSeconds: t);
                t += 0.100;
            }

            Assert.GreaterOrEqual(
                buffer.DelaySeconds,
                SnapshotBuffer.MinDelaySeconds,
                "adaptive delay never drops below 150 ms");
            Assert.LessOrEqual(
                buffer.DelaySeconds,
                SnapshotBuffer.MaxDelaySeconds,
                "adaptive delay never exceeds 300 ms");

            double newest = t - 0.100;
            (double xExtrap, double yExtrap) = buffer.Interpolate(
                newest + buffer.DelaySeconds + 10.0);
            double maxExtrapX =
                7000 + 500.0 * SnapshotBuffer.ExtrapolationLimitSeconds;
            Assert.LessOrEqual(
                xExtrap,
                maxExtrapX,
                "extrapolation freezes 250 ms past the newest sample");

            var reconciliation = new SelfReconciliation();
            var closeAck = new SelfAck
            {
                LastProcessedClientSeq = 1,
                Checkpoint = new MovementCheckpoint
                {
                    XMm = 10000, YMm = 0, IsGrounded = true,
                },
            };
            reconciliation.ApplyAck(closeAck, 10400, 0);
            long errorSq = reconciliation.SnapErrorSqMm;
            Assert.GreaterOrEqual(
                errorSq, 0L);
            Assert.Less(
                errorSq,
                SelfReconciliation.SnapThresholdSqMm,
                "a 400 mm error stays under the 0.5 m snap threshold and " +
                "smooths instead of teleporting");

            reconciliation.ApplyAck(closeAck, 11000, 0);
            Assert.GreaterOrEqual(
                reconciliation.SnapErrorSqMm,
                SelfReconciliation.SnapThresholdSqMm,
                "a 1000 mm error crosses the snap threshold");
        }

        [Test]
        public void TestFullQualityAndDegradedConditions()
        {
            var full = new NetworkEmulator(0xF00D)
            {
                LatencyMs = 60,
                JitterMs = 12,
                DropProbability = 0.005,
            };
            var degraded = new NetworkEmulator(0xDEAD)
            {
                LatencyMs = 130,
                JitterMs = 20,
                DropProbability = 0.045,
            };

            AssertEmulatorProfile(
                full,
                maxRttMs: 150,
                maxJitterMs: 30,
                maxLoss: 0.02);
            AssertEmulatorProfile(
                degraded,
                maxRttMs: 300,
                maxJitterMs: 60,
                maxLoss: 0.05);

            var reconciliation = new SelfReconciliation();
            int corrections = 0;
            int displayedX = 10000;
            var rng = new System.Random(7);
            for (int minute = 0; minute < 3; minute++)
            {
                for (int i = 0; i < 40; i++)
                {
                    int jitterMm = rng.Next(-80, 81);
                    var ack = new SelfAck
                    {
                        LastProcessedClientSeq = (ulong)(i + 1),
                        Checkpoint = new MovementCheckpoint
                        {
                            XMm = displayedX + jitterMm,
                            YMm = 0,
                            IsGrounded = true,
                        },
                    };
                    reconciliation.ApplyAck(ack, displayedX, 0);
                    if (reconciliation.SnapErrorSqMm >=
                        SelfReconciliation.SnapThresholdSqMm)
                    {
                        corrections++;
                    }

                    displayedX += 100;
                }
            }

            Assert.LessOrEqual(
                corrections,
                1,
                "PERF-011 full-quality conditions keep >0.5 m corrections " +
                "to at most one per minute");

            reconciliation.ForceSnap(new MovementCheckpoint
            {
                XMm = displayedX, YMm = 0, IsGrounded = true,
            });
            Assert.IsTrue(
                reconciliation.ForcedSnap != null,
                "S2C_MOVEMENT_CORRECTION forces a snap (no lerp)");
            reconciliation.ConsumeForcedSnap();
            Assert.IsFalse(
                reconciliation.ForcedSnap != null,
                "the forced snap clears once the presentation applies it");
        }

        private static void AssertEmulatorProfile(
            NetworkEmulator emulator,
            int maxRttMs,
            int maxJitterMs,
            double maxLoss)
        {
            int maxDelay = 0;
            int drops = 0;
            const int trials = 400;
            for (int i = 0; i < trials; i++)
            {
                int delay = emulator.NextDelayMs();
                if (delay > maxDelay)
                {
                    maxDelay = delay;
                }

                if (emulator.ShouldDrop())
                {
                    drops++;
                }
            }

            Assert.LessOrEqual(
                maxDelay,
                maxRttMs / 2,
                "one-way emulator latency stays inside its profile");
            Assert.LessOrEqual(
                emulator.JitterMs,
                maxJitterMs,
                "jitter inside the profile bound");
            Assert.LessOrEqual(
                drops / (double)trials,
                maxLoss + 0.02,
                "loss probability matches the profile");
        }
    }
}
