using System;
using System.Collections.Generic;
using System.Diagnostics;
using NUnit.Framework;
using ThinhThan.Core.Runtime;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.FrameRuntime
{
    /// <summary>PERF-014/PERF-015 frame-runtime gates (packet IMP-065).</summary>
    public sealed class FrameRuntimeTests
    {
        private sealed class ManualClock : IClock
        {
            public double NowSeconds
            {
                get;
                set;
            }

            public float UnscaledDeltaSeconds
            {
                get;
                set;
            }
        }

        private sealed class Recorder : IFrameSystem
        {
            public List<FramePhase>? Sink
            {
                get;
                set;
            }

            public FramePhase Self
            {
                get;
                set;
            }

            public void Tick(in FrameTime time)
            {
                Sink?.Add(Self);
            }
        }

        [Test]
        public void TestPhaseOrderFixed()
        {
            var order = new List<FramePhase>();
            var go = new GameObject("loop");
            FrameLoop loop = go.AddComponent<FrameLoop>();
            foreach (FramePhase phase in Enum.GetValues(typeof(FramePhase)))
            {
                loop.Register(phase, new Recorder
                {
                    Sink = order,
                    Self = phase,
                });
            }

            loop.TickOnce();
            UnityEngine.Object.DestroyImmediate(go);
            var expected = new List<FramePhase>
            {
                FramePhase.Input, FramePhase.NetReceive,
                FramePhase.Prediction, FramePhase.Interpolation,
                FramePhase.Presentation, FramePhase.UI,
                FramePhase.Camera,
            };
            Assert.AreEqual(expected, order);
        }

        [Test]
        public void TestFrameTimeClamp100ms()
        {
            var clamped = new FrameTime(0.25f, 1.0, 0);
            Assert.AreEqual(0.1f, clamped.Delta, 1e-6f);
            var negative = new FrameTime(-0.5f, 1.0, 0);
            Assert.AreEqual(0f, negative.Delta);
            var normal = new FrameTime(0.016f, 1.0, 0);
            Assert.AreEqual(0.016f, normal.Delta, 1e-6f);
        }

        [Test]
        public void TestRegistrationOutsideTickOnly()
        {
            var go = new GameObject("loop");
            FrameLoop loop = go.AddComponent<FrameLoop>();
            var go2 = new GameObject("loop2");
            FrameLoop other = go2.AddComponent<FrameLoop>();
            loop.Register(FramePhase.Input, new Recorder());
            loop.Register(FramePhase.Camera, new Recorder());
            var blocker = new RegisterDuringTick(other);
            loop.Register(FramePhase.NetReceive, blocker);
            var time = new FrameTime(0.016f, 0.0, 0);
            Assert.Throws<InvalidOperationException>(
                () => loop.TickUpdatePhases(in time));
            UnityEngine.Object.DestroyImmediate(go);
            UnityEngine.Object.DestroyImmediate(go2);
        }

        private sealed class RegisterDuringTick : IFrameSystem
        {
            private readonly FrameLoop _loop;

            public RegisterDuringTick(FrameLoop loop)
            {
                _loop = loop;
            }

            public void Tick(in FrameTime time)
            {
                _loop.Register(FramePhase.UI, new Recorder());
            }
        }

        [Test]
        public void TestEntityViewsIndexBased()
        {
            var store = new EntityViewStore();
            for (int i = 0; i < 5; i++)
            {
                store.Upsert(new EntityState
                {
                    EntityId = (ulong)(10 + i),
                    XMm = i,
                });
            }

            Assert.AreEqual(5, store.Count);
            Assert.AreEqual(10UL, store.Ids[0]);
            Assert.AreEqual(14UL, store.Ids[4]);
            Assert.IsTrue(store.TryGetIndex(12UL, out int index));
            Assert.AreEqual(2, index);
            Assert.IsTrue(store.Remove(11UL));
            Assert.AreEqual(4, store.Count);
            Assert.AreEqual(4, store.Ids.Length);
            Assert.IsTrue(store.TryGetIndex(14UL, out int moved));
            Assert.AreEqual(14UL, store.Ids[moved]);
        }

        [Test]
        public void TestFrameBudgetGameplay2ms()
        {
            var clock = new ManualClock();
            var budget = new FrameBudget(clock);
            int ran = 0;
            for (int i = 0; i < 10; i++)
            {
                budget.Enqueue(() =>
                {
                    ran++;
                    clock.NowSeconds += 0.001;
                    return true;
                });
            }

            var time = new FrameTime(0.016f, 0.0, 0);
            budget.Tick(in time);
            Assert.LessOrEqual(ran, 2);
            Assert.Greater(budget.PendingCount, 0);
        }

        [Test]
        public void TestFrameBudgetLoading12ms()
        {
            var clock = new ManualClock();
            var budget = new FrameBudget(clock)
            {
                Loading = true,
            };
            int ran = 0;
            for (int i = 0; i < 20; i++)
            {
                budget.Enqueue(() =>
                {
                    ran++;
                    clock.NowSeconds += 0.001;
                    return true;
                });
            }

            var time = new FrameTime(0.016f, 0.0, 0);
            budget.Tick(in time);
            Assert.AreEqual(12, ran);
        }

        [Test]
        public void TestPoolPrewarmReuse()
        {
            var created = new List<object>();
            var pool = new Pool<object>(
                () =>
                {
                    var item = new object();
                    created.Add(item);
                    return item;
                },
                maxRetained: 8);
            pool.Prewarm(4);
            Assert.AreEqual(4, pool.IdleCount);
            object a = pool.Rent();
            object b = pool.Rent();
            Assert.AreSame(a, created[3]);
            Assert.AreSame(b, created[2]);
            Assert.AreEqual(2, pool.LiveCount);
            pool.Return(a);
            object c = pool.Rent();
            Assert.AreSame(a, c);
            pool.Return(b);
            pool.Return(c);
        }

        [Test]
        public void TestLogDevConditional()
        {
            var method = typeof(Log).GetMethod("Dev");
            Assert.IsNotNull(method);
            // method non-null: asserted immediately above.
            object[] attributes = method!.GetCustomAttributes(
                typeof(ConditionalAttribute), false);
            Assert.IsNotEmpty(attributes);
            var conditional = (ConditionalAttribute)attributes[0];
            Assert.AreEqual("THINHTHAN_DEV", conditional.ConditionString);
        }
    }
}
