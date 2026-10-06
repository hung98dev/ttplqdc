using System.Collections.Generic;
using System.Threading;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Core.Geometry;
using ThinhThan.Core.Runtime;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Movement;
using ThinhThan.Systems.Replication;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.MovementPrediction
{
    /// <summary>
    /// IMP-013 PlayMode: input dispatch cadence + self-ack
    /// smooth/snap/107-replay reconciliation (in-code GeometryData
    /// fixtures — no .geom.json delivery on this branch, F-05).
    /// </summary>
    public sealed class MovementPredictionTests
    {
        // --- Fixtures -----------------------------------------------------

        private static GeometryData FlatGeometry()
        {
            var segments = new[]
            {
                new GeometryData.Segment(
                    1, GeometryData.SegmentKind.SolidGround,
                    -100000, 0, 200000, 0),
                new GeometryData.Segment(
                    2, GeometryData.SegmentKind.Wall,
                    60000, 0, 60000, 400),
            };
            return new GeometryData(
                "map_test_flat",
                "map",
                "flat",
                "test-revision",
                200000,
                50000,
                segments,
                new GeometryData.CameraRegion[0],
                new GeometryData.Anchor[0]);
        }

        private sealed class FakeSender : IMovementSender
        {
            public readonly List<(uint Id, IMessage Msg)> Sent =
                new List<(uint, IMessage)>();

            public ulong NextSeq = 1;

            public Awaitable<ulong> SendAsync(
                uint messageId, IMessage payload, CancellationToken cancel)
            {
                Sent.Add((messageId, payload));
                ulong seq = NextSeq++;
                var src = new AwaitableCompletionSource<ulong>();
                src.SetResult(seq);
                return src.Awaitable;
            }
        }

        private sealed class FakeInput : IInputSource
        {
            public int HeldDirection;
            public uint HeldFlags;
            public readonly Queue<LocalEdge> Edges = new Queue<LocalEdge>();

            int IInputSource.HeldDirection
            {
                get
                {
                    return HeldDirection;
                }
            }

            uint IInputSource.HeldFlags
            {
                get
                {
                    return HeldFlags;
                }
            }

            public int SampleEdges(LocalEdge[] edges)
            {
                int n = 0;
                while (n < edges.Length && Edges.Count > 0)
                {
                    edges[n++] = Edges.Dequeue();
                }
                return n;
            }
        }

        private sealed class FakeSink : SelfReconciliationSink
        {
            public readonly List<ulong> Sent = new List<ulong>();

            public void NoteInputSent(ulong seq)
            {
                Sent.Add(seq);
            }
        }

        private static FrameTime Frame(double nowSec, float delta = 0.016f)
        {
            return new FrameTime(delta, nowSec, 0);
        }

        private static MovementCheckpoint Checkpoint(
            int x, int y, MovementState state = MovementState.Idle)
        {
            return new MovementCheckpoint
            {
                XMm = x,
                YMm = y,
                MovementState = state,
                IsGrounded = true,
                JumpCount = 0,
            };
        }

        // --- Dispatch cadence ---------------------------------------------

        [Test]
        public void TestEdgeSendsImmediatelyAndHeldStateResends()
        {
            var source = new FakeInput();
            var sender = new FakeSender();
            var history = new InputHistory();
            var sink = new FakeSink();
            var dispatch = new MovementInputDispatch(
                source, sender, history, sink);

            source.Edges.Enqueue(LocalEdge.PressRight);
            source.HeldDirection = 1;
            source.HeldFlags = 2;

            dispatch.Dispatch(Frame(0.0));

            Assert.AreEqual(2, sender.Sent.Count);
            Assert.AreEqual(
                MovementConstants.WireC2SMovementEdge, sender.Sent[0].Id);
            var edgeMsg = (C2SMovementEdge)sender.Sent[0].Msg;
            Assert.AreEqual(MovementEdgeType.Press, edgeMsg.EdgeType);
            Assert.AreEqual(Facing.Right, edgeMsg.Direction);
            Assert.AreEqual(
                MovementConstants.WireC2SInputState, sender.Sent[1].Id);

            // Second frame: nothing changed and <250 ms elapsed — no resend.
            dispatch.Dispatch(Frame(0.03));
            Assert.AreEqual(2, sender.Sent.Count);

            // Held resend cadence: >= 250 ms since the last held send.
            dispatch.Dispatch(Frame(0.30));
            Assert.AreEqual(3, sender.Sent.Count);
            Assert.AreEqual(
                MovementConstants.WireC2SInputState, sender.Sent[2].Id);

            // History recorded both sent seqs.
            Assert.AreEqual(3, history.Count);
            Assert.AreEqual(3, sink.Sent.Count);
        }

        [Test]
        public void TestJumpAndDropDispatchOnDedicatedIds()
        {
            var source = new FakeInput();
            var sender = new FakeSender();
            var dispatch = new MovementInputDispatch(
                source, sender, new InputHistory(), new FakeSink());

            source.Edges.Enqueue(LocalEdge.Jump);
            source.Edges.Enqueue(LocalEdge.Drop);
            dispatch.Dispatch(Frame(0.0));

            Assert.AreEqual(MovementConstants.WireC2SJump, sender.Sent[0].Id);
            Assert.AreEqual(
                MovementConstants.WireC2SDropThrough, sender.Sent[1].Id);
        }

        // --- Reconciliation ------------------------------------------------

        private static (MovementPredictionSystem, ReconciliationController, InputHistory, SelfReconciliation) Rig()
        {
            var world = new GeometryMath.GeometryWorld(FlatGeometry());
            var prediction = new MovementPredictionSystem(world);
            prediction.RestoreFrom(Checkpoint(10000, 0));
            var controller = new ReconciliationController(prediction);
            return (prediction, controller, new InputHistory(), new SelfReconciliation());
        }

        private static SelfAck Ack(
            ulong lastSeq, MovementCheckpoint checkpoint)
        {
            return new SelfAck
            {
                LastProcessedClientSeq = lastSeq,
                Checkpoint = checkpoint,
            };
        }

        [Test]
        public void TestSelfAckSmoothUnderThreshold()
        {
            var (prediction, controller, history, self) = Rig();

            // Local shadow drifted 300 mm right of the ack (< 500 mm).
            self.ApplyAck(
                Ack(10, Checkpoint(9700, 0)), 10000, 0);
            Assert.IsTrue(controller.Process(self, history));
            Assert.AreNotEqual((0L, 0L), controller.VisualOffsetMm);
        }

        [Test]
        public void TestSelfAckSnapOverThreshold()
        {
            var (prediction, controller, history, self) = Rig();

            // Local shadow drifted 900 mm — over the 500 mm band.
            self.ApplyAck(
                Ack(10, Checkpoint(9100, 0)), 10000, 0);
            controller.Process(self, history);
            Assert.AreEqual((0L, 0L), controller.VisualOffsetMm);
        }

        [Test]
        public void TestCorrection107AlwaysSnapsAndReplays()
        {
            var (prediction, controller, history, self) = Rig();

            history.Push(new InputRecord
            {
                Kind = InputRecordKind.Held,
                Seq = 8,
                Tick = 0,
                HeldDirection = 1,
                Flags = 2,
            });
            self.ApplyAck(Ack(7, Checkpoint(10000, 0)), 10000, 0);
            controller.Process(self, history);

            self.ForceSnap(Checkpoint(40000, 0, MovementState.Run));
            Assert.IsTrue(controller.Process(self, history));
            Assert.AreEqual((0L, 0L), controller.VisualOffsetMm);
            Assert.AreEqual(40000, prediction.State.XMm);
        }

        [Test]
        public void TestPredictionIntegratesHeldDirection()
        {
            var (prediction, _, _, _) = Rig();
            prediction.EnqueueLocal(new InputRecord
            {
                Kind = InputRecordKind.Held,
                HeldDirection = 1,
            });
            prediction.Tick(Frame(0.0, delta: 0.05f));
            Assert.AreEqual(
                MovementConstants.RunSpeedMmS *
                    (int)MovementConstants.TickMillis / 1000,
                prediction.State.XMm - 10000);
            Assert.AreEqual(MovementState.Run, prediction.State.MovementState);
        }
    }
}
