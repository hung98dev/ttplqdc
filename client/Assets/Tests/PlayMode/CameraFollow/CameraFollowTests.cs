using NUnit.Framework;
using ThinhThan.Core.Geometry;
using ThinhThan.Core.Runtime;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Camera;
using ThinhThan.Systems.Movement;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.CameraFollow
{
    /// <summary>
    /// PERF-023 + physics.md §6.2: one camera move per frame, critically
    /// damped follow without overshoot, region clamp with smoothed region
    /// transitions, snap only on transfers / hard reconciles.
    /// </summary>
    public sealed class CameraFollowTests
    {
        private sealed class Rig
        {
            public readonly MovementPredictionSystem Prediction;
            public readonly CameraRegionResolver Resolver;
            public readonly CameraViewModel View = new CameraViewModel();
            public readonly CameraFollowService Service;

            public Rig(GeometryData geometry)
            {
                Prediction = new MovementPredictionSystem(
                    new GeometryMath.GeometryWorld(geometry));
                Resolver = new CameraRegionResolver(geometry);
                Service = new CameraFollowService(
                    Prediction, Resolver, View, aspect: () => 16f / 9f);
            }

            public void Teleport(int xMm, int yMm)
            {
                Prediction.RestoreFrom(new MovementCheckpoint
                {
                    XMm = xMm,
                    YMm = yMm,
                    Facing = Facing.Right,
                    IsGrounded = true,
                });
            }
        }

        private static GeometryData FlatGeometry(
            GeometryData.CameraRegion[]? regions = null)
        {
            return new GeometryData(
                "map_test_flat", "map", "flat", "test-revision",
                200000, 50000,
                new[]
                {
                    new GeometryData.Segment(
                        1, GeometryData.SegmentKind.SolidGround,
                        -100000, 0, 200000, 0),
                },
                regions ?? new GeometryData.CameraRegion[0],
                new GeometryData.Anchor[0]);
        }

        private static FrameTime Frame(double nowSec, float delta = 0.016f)
        {
            return new FrameTime(delta, nowSec, 0);
        }

        [Test]
        public void TestCriticallyDampedNoOvershoot()
        {
            var rig = new Rig(FlatGeometry());
            rig.Teleport(0, 0);
            var t = Frame(0.0);
            rig.Service.Tick(in t); // first pose = snap to anchor
            Assert.AreEqual(0f, rig.Service.Pose.x, 0.001f);

            // Anchor steps +2 m (< snap threshold): the spring must chase
            // monotonically and never cross the target.
            rig.Teleport(2000, 0);
            double now = 0.016;
            float prev = rig.Service.Pose.x;
            for (int i = 0; i < 200; i++)
            {
                t = Frame(now, 0.016f);
                rig.Service.Tick(in t);
                now += 0.016;
                float x = rig.Service.Pose.x;
                Assert.LessOrEqual(x, 2.0001f,
                    "overshoot at step " + i + ": " + x);
                Assert.GreaterOrEqual(x, prev - 0.0001f,
                    "non-monotonic approach at step " + i);
                prev = x;
            }

            Assert.AreEqual(2f, rig.Service.Pose.x, 0.01f);

            // Settle, then step backwards — symmetric: never below target.
            rig.Teleport(1000, 0);
            prev = rig.Service.Pose.x;
            for (int i = 0; i < 200; i++)
            {
                t = Frame(now, 0.016f);
                rig.Service.Tick(in t);
                now += 0.016;
                float x = rig.Service.Pose.x;
                Assert.GreaterOrEqual(x, 0.9999f,
                    "undershoot at step " + i + ": " + x);
                Assert.LessOrEqual(x, prev + 0.0001f,
                    "non-monotonic approach at step " + i);
                prev = x;
            }

            Assert.AreEqual(1f, rig.Service.Pose.x, 0.01f);
        }

        [Test]
        public void TestSingleMovePerFrame()
        {
            var rig = new Rig(FlatGeometry());
            rig.Teleport(0, 0);
            var t = Frame(0.0);
            rig.Service.Tick(in t);
            Assert.AreEqual(1, rig.Service.MoveCount);

            // Ten more frames: exactly ten moves total, one per tick.
            for (int i = 1; i <= 10; i++)
            {
                t = Frame(i * 0.016, 0.016f);
                rig.Service.Tick(in t);
            }

            Assert.AreEqual(11, rig.Service.MoveCount);
        }

        [Test]
        public void TestCameraRegionClamp()
        {
            // Region 10 m × 4 m — the 16:9 view (25.6 × 14.4 m) exceeds it
            // on both axes → the centre must sit at the region centre.
            var rig = new Rig(FlatGeometry(new[]
            {
                new GeometryData.CameraRegion(1, 0, 0, 10000, 4000),
            }));
            rig.Teleport(9000, 3500);
            var t = Frame(0.0);
            rig.Service.Tick(in t);
            Assert.AreEqual(5f, rig.Service.Pose.x, 0.001f);
            Assert.AreEqual(2f, rig.Service.Pose.y, 0.001f);

            // A wide region clamps only the axis the view exceeds.
            var tall = new GeometryData.CameraRegion(
                2, 0, 0, 60000, 8000);
            var wide = new Rig(FlatGeometry(new[] { tall }));
            wide.Teleport(50000, 7500);
            wide.Service.Tick(in t);
            // X: view half = 12.8 m < 60 m region → anchor clamped inside.
            Assert.AreEqual(47.2f, wide.Service.Pose.x, 0.001f);
            // Y: view half = 7.2 m > 8 m region → axis-centred at 4 m.
            Assert.AreEqual(4f, wide.Service.Pose.y, 0.001f);
        }

        [Test]
        public void TestCameraRegionChangeSmoothed()
        {
            // Two adjacent 60 m regions; crossing keeps the move under the
            // snap threshold so the spring glides to the new clamp.
            var rig = new Rig(FlatGeometry(new[]
            {
                new GeometryData.CameraRegion(1, 0, 0, 60000, 20000),
                new GeometryData.CameraRegion(
                    2, 60000, 0, 120000, 20000),
            }));
            rig.Teleport(57000, 10000);
            var t = Frame(0.0);
            rig.Service.Tick(in t);
            Assert.AreEqual(1L, rig.Resolver.ActiveRegion!.Value.Id);
            // desired = clamp of anchor 57 m: region max 60 − half 12.8.
            Assert.AreEqual(47.2f, rig.Service.Pose.x, 0.001f);

            // Cross 1.5 m into region 2: the clamped target jumps
            // 47.2 → 72.8 m but the anchor moved a legal amount — the
            // region change must be smoothed, not a teleport.
            rig.Teleport(60500, 10000);
            t = Frame(0.016, 0.016f);
            rig.Service.Tick(in t);
            Assert.AreEqual(2L, rig.Resolver.ActiveRegion!.Value.Id);
            float step = rig.Service.Pose.x - 47.2f;
            Assert.Greater(step, 0f);
            Assert.Less(
                step, CameraFollowService.SnapThresholdMeters,
                "region change must be smoothed, not a teleport");
            Assert.Less(rig.Service.Pose.x, 72.8f);

            // Converges onto the new region's clamped centre over time.
            for (int i = 0; i < 200; i++)
            {
                t = Frame(0.032 + i * 0.016, 0.016f);
                rig.Service.Tick(in t);
            }

            Assert.AreEqual(72.8f, rig.Service.Pose.x, 0.05f);
        }

        [Test]
        public void TestSnapOnTransferAndHardReconcile()
        {
            var rig = new Rig(FlatGeometry());
            rig.Teleport(0, 0);
            var t = Frame(0.0);
            rig.Service.Tick(in t);

            // Transfer-sized jump (> 4 m single frame): instant snap, the
            // spring must not trail through the void.
            rig.Teleport(50000, 0);
            t = Frame(0.016, 0.016f);
            rig.Service.Tick(in t);
            Assert.AreEqual(50f, rig.Service.Pose.x, 0.001f);

            // Small drift then an explicit reconcile snap (forced-snap
            // correction seam — composition calls RequestSnap).
            rig.Teleport(51000, 0);
            t = Frame(0.032, 0.016f);
            rig.Service.Tick(in t);
            float trailing = rig.Service.Pose.x;
            Assert.Greater(trailing, 50f);
            Assert.Less(trailing, 51f);

            rig.Service.RequestSnap();
            t = Frame(0.048, 0.016f);
            rig.Service.Tick(in t);
            Assert.AreEqual(51f, rig.Service.Pose.x, 0.001f);
        }
    }
}
