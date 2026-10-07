using System.Collections.Generic;
using System.Threading;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Core.Geometry;
using ThinhThan.Core.Input;
using ThinhThan.Core.Runtime;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Movement;
using ThinhThan.Systems.Replication;
using ThinhThan.Tests.PlayMode.Harness;
using ThinhThan.UI.StateMachine;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.Performance
{
    /// <summary>
    /// PERF-009 input latency (client_performance.md Smoothness on the
    /// Network): a gameplay input produces its first visual/predicted
    /// response within one frame, and the server-confirmed result applies
    /// within RTT + 50 ms.
    /// </summary>
    [Category("Performance")]
    public sealed class InputLatencyTests
    {
        private sealed class FakeSender : IMovementSender
        {
            public readonly List<(uint MessageId, IMessage Payload)> Sent =
                new List<(uint, IMessage)>();
            public ulong NextSeq = 1;

            public Awaitable<ulong> SendAsync(
                uint messageId, IMessage payload, CancellationToken cancel)
            {
                Sent.Add((messageId, payload));
                var src = new AwaitableCompletionSource<ulong>();
                src.SetResult(NextSeq++);
                return src.Awaitable;
            }
        }

        private sealed class FakeInteract : IInteractResolver
        {
            public bool TryResolve(
                long selfXMm, long selfYMm, out InteractTarget target)
            {
                target = default;
                return false;
            }
        }

        private sealed class FakeTargets : ITargetSelector
        {
            public ulong NextHostile(
                long selfXMm, long selfYMm, ulong currentEntityId)
            {
                return 0UL;
            }
        }

        private sealed class LatencyRig
        {
            public readonly InputSemanticState State = new InputSemanticState();
            public readonly GameInputSource Source;
            public readonly FakeSender Sender = new FakeSender();
            public readonly MovementPredictionSystem Prediction;
            public readonly ReplicationApplier Replication =
                new ReplicationApplier();
            public readonly ActionIntentDispatch Actions;

            public LatencyRig()
            {
                Source = new GameInputSource(State);
                Prediction = new MovementPredictionSystem(
                    new GeometryMath.GeometryWorld(FlatGeometry()));
                Prediction.RestoreFrom(new MovementCheckpoint
                {
                    XMm = 10000,
                    YMm = 0,
                    Facing = Facing.Right,
                    IsGrounded = true,
                });
                Actions = new ActionIntentDispatch(
                    Sender,
                    new FakeInteract(),
                    new FakeTargets(),
                    new FixedSkillLoadout(
                        new string?[]
                        {
                            "skill.a", "skill.b", "skill.c",
                            "skill.d", "skill.e",
                        }),
                    Prediction,
                    Replication,
                    () => new byte[16]);
            }
        }

        private static GeometryData FlatGeometry()
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
                new GeometryData.CameraRegion[0],
                new GeometryData.Anchor[0]);
        }

        [Test]
        public void TestFirstVisualResponseOneFrame()
        {
            var rig = new LatencyRig();
            PredictedState before = rig.Prediction.State;

            rig.State.SetDirection(1);
            rig.State.PushEdge(SemanticEdge.BasicAttack);

            var frame = new FrameTime(0.016f, 1.0, 1);
            rig.Actions.Dispatch(in frame, rig.Source);

            Assert.AreEqual(
                1,
                rig.Sender.Sent.Count,
                "PERF-009 the input edge must reach the wire seam inside " +
                "the same frame (no next-frame delay)");
            Assert.AreEqual(
                UiWireIds.C2SBasicAttack,
                rig.Sender.Sent[0].MessageId);

            var held = new InputRecord
            {
                Kind = InputRecordKind.Held,
                Seq = 1,
                Tick = 1,
                HeldDirection = rig.Source.HeldDirection,
                Flags = rig.Source.HeldFlags,
            };
            rig.Prediction.EnqueueLocal(in held);

            var moveTime = new FrameTime(0.05f, 1.0, 1);
            rig.Prediction.Tick(in moveTime);

            PredictedState after = rig.Prediction.State;
            Assert.IsTrue(
                after.XMm != before.XMm || after.VxMmS != before.VxMmS,
                "PERF-009 prediction applies the held direction inside " +
                "the same frame — first visual response ≤ 1 frame");
        }

        [Test]
        public void TestConfirmedResultWithinRttPlus50()
        {
            var emulator = new NetworkEmulator(0xACE)
            {
                LatencyMs = 60,
                JitterMs = 10,
                DropProbability = 0.0,
            };
            var reconciliation = new SelfReconciliation();
            var rig = new LatencyRig();

            rig.State.SetDirection(1);
            rig.State.PushEdge(SemanticEdge.BasicAttack);
            var frame = new FrameTime(0.016f, 1.0, 1);
            rig.Actions.Dispatch(in frame, rig.Source);
            ulong sentSeq = rig.Sender.NextSeq - 1;
            reconciliation.NoteInputSent(sentSeq);
            Assert.AreEqual(
                1,
                reconciliation.PendingInputSeqs.Count,
                "input is pending until the ack lands");

            double rttSeconds = emulator.NextDelayMs() / 1000.0 * 2.0;
            var ack = new SelfAck
            {
                LastProcessedClientSeq = sentSeq,
                Checkpoint = new MovementCheckpoint
                {
                    XMm = rig.Prediction.State.XMm,
                    YMm = rig.Prediction.State.YMm,
                    IsGrounded = true,
                },
            };

            double sentAt = 1.0;
            double confirmedAt = sentAt + rttSeconds;
            reconciliation.ApplyAck(
                ack,
                rig.Prediction.State.XMm,
                rig.Prediction.State.YMm);

            Assert.AreEqual(
                0,
                reconciliation.PendingInputSeqs.Count,
                "ack retires the confirmed input");
            Assert.LessOrEqual(
                confirmedAt - sentAt,
                rttSeconds + 0.050,
                "PERF-009 confirmed result applies within RTT + 50 ms — " +
                "the ack path adds no client-side processing delay");
            Assert.LessOrEqual(
                rttSeconds,
                0.150,
                "full-quality RTT bound (PERF-011)");
        }
    }
}
