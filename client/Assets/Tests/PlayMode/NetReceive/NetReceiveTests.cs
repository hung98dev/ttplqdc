using System;
using System.Collections.Generic;
using System.IO;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.Tests.PlayMode.NetReceive.Fixtures;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.NetReceive
{
    /// <summary>
    /// PERF-024 decode/framing/apply budgets + the bounded receive queue +
    /// the byte-identical hotspot fixture (packet IMP-065).
    /// </summary>
    public sealed class NetReceiveTests
    {
        private const string FixturePath =
            "Tests/PlayMode/NetReceive/Fixtures/hotspot_stream_40.bytes";

        private static byte[] LoadFixture()
        {
            return File.ReadAllBytes(
                Path.Combine(Application.dataPath, FixturePath));
        }

        private static DecodedFrame NewListFrame(int accountedBytes)
        {
            return new DecodedFrame
            {
                MessageId = WireIds.S2CCharacterList,
                Payload = new S2CCharacterList(),
                AccountedBytes = accountedBytes,
            };
        }

        [Test]
        public void TestBoundedReceiveQueue()
        {
            var queue = new ReceiveQueue();
            int exhausted = 0;
            queue.Exhausted += () => exhausted++;
            for (int i = 0; i < ReceiveQueue.MaxFrames; i++)
            {
                Assert.IsTrue(queue.TryEnqueue(NewListFrame(16)));
            }

            Assert.IsFalse(queue.TryEnqueue(NewListFrame(16)));
            Assert.AreEqual(1, exhausted);

            var bytesQueue = new ReceiveQueue();
            int each = ReceiveQueue.MaxBytes / 8;
            for (int i = 0; i < 8; i++)
            {
                Assert.IsTrue(bytesQueue.TryEnqueue(NewListFrame(each)));
            }

            Assert.IsFalse(bytesQueue.TryEnqueue(NewListFrame(1)));
        }

        [Test]
        public void TestDecodeAllocationBudget()
        {
            byte[] container = LoadFixture();
            List<byte[]> frames = HotspotStreamGenerator.ReadFrames(container);
            Assert.AreEqual(601, frames.Count);

            var frame = new DecodedFrame();
            foreach (byte[] bytes in frames)
            {
                EnvelopeCodec.Decode(bytes, frame);
            }

            long before = GC.GetAllocatedBytesForCurrentThread();
            foreach (byte[] bytes in frames)
            {
                EnvelopeCodec.Decode(bytes, frame);
            }

            long allocated = GC.GetAllocatedBytesForCurrentThread() - before;
            double perSecond = allocated / 60.0;
            Assert.LessOrEqual(
                perSecond, 64.0 * 1024.0,
                "PERF-024 decode allocation over the 60 s hotspot stream");
        }

        [Test]
        public void TestPooledFramingZeroAlloc()
        {
            var queue = new ReceiveQueue();
            var reusable = new DecodedFrame();
            void Cycle()
            {
                reusable.ConnectionGeneration = 0;
                reusable.MessageId = WireIds.S2CHeartbeat;
                reusable.Payload = null;
                reusable.AccountedBytes = 16;
                Assert.IsTrue(queue.TryEnqueue(reusable));
                Assert.IsTrue(queue.TryDequeue(out ReceiveLease lease));
                lease.Dispose();
            }

            for (int i = 0; i < 128; i++)
            {
                Cycle();
            }

            long before = GC.GetAllocatedBytesForCurrentThread();
            for (int i = 0; i < 4096; i++)
            {
                Cycle();
            }

            long allocated = GC.GetAllocatedBytesForCurrentThread() - before;
            Assert.AreEqual(0, allocated,
                "PERF-024 framing/buffer path must allocate zero bytes");
        }

        [Test]
        public void TestMainThreadApplyZeroAlloc()
        {
            var applier = new ReplicationApplier();
            List<byte[]> frames =
                HotspotStreamGenerator.ReadFrames(LoadFixture());
            var decoded = new List<DecodedFrame>(frames.Count);
            foreach (byte[] bytes in frames)
            {
                var frame = new DecodedFrame();
                EnvelopeCodec.Decode(bytes, frame);
                if (frame.MessageId == WireIds.S2CWorldBaseline ||
                    frame.MessageId == WireIds.S2CStateDelta)
                {
                    decoded.Add(frame);
                }
            }

            void ApplyAll()
            {
                foreach (DecodedFrame frame in decoded)
                {
                    applier.Apply(frame);
                }
            }

            ApplyAll();
            ApplyAll();
            long before = GC.GetAllocatedBytesForCurrentThread();
            ApplyAll();
            long allocated = GC.GetAllocatedBytesForCurrentThread() - before;
            Assert.AreEqual(0, allocated,
                "PERF-024 main-thread apply must allocate zero bytes");
        }

        [Test]
        public void TestHotspotStreamFixtureRegeneratesIdentically()
        {
            byte[] fixture = LoadFixture();
            byte[] regenerated = HotspotStreamGenerator.Generate();
            Assert.AreEqual(
                fixture.Length, regenerated.Length,
                "hotspot_stream_40.bytes length");
            for (int i = 0; i < fixture.Length; i++)
            {
                if (fixture[i] != regenerated[i])
                {
                    Assert.Fail(
                        "fixture byte mismatch at offset " + i);
                }
            }
        }
    }
}
