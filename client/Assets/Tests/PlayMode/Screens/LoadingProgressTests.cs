using NUnit.Framework;
using ThinhThan.Core.Runtime;
using ThinhThan.UI.Screens;

namespace ThinhThan.Tests.PlayMode.Screens
{
    /// <summary>
    /// PERF-008 / PERF-015 loading-screen behaviour (client_performance.md):
    /// progress surfaces only after a 0.5 s wait, queued work drains inside
    /// the 12 ms loading budget so no frame freezes past 100 ms, and the
    /// loading policy raises High priority + one GC before restoring
    /// gameplay budgets.
    /// </summary>
    public sealed class LoadingProgressTests
    {
        private sealed class FakeClock : IClock
        {
            public float UnscaledDeltaSeconds
            {
                get;
                set;
            }

            public double NowSeconds
            {
                get;
                set;
            }
        }

        private sealed class FakeEnvironment : ILoadingEnvironment
        {
            public bool Loading
            {
                get;
                private set;
            }

            public bool HighPriority
            {
                get;
                private set;
            }

            public int Collections
            {
                get;
                private set;
            }

            public void SetLoading(bool loading)
            {
                Loading = loading;
            }

            public void SetBackgroundLoadingPriorityHigh(bool high)
            {
                HighPriority = high;
            }

            public void CollectGarbageOnce()
            {
                Collections++;
            }
        }

        /// <summary>
        /// PERF-008: the bar stays hidden while the wait is under 0.5 s and
        /// appears once it passes — progress is always visible for slow
        /// operations.
        /// </summary>
        [Test]
        public void TestProgressShownAfterHalfSecond()
        {
            var clock = new FakeClock();
            var reporter = new LoadingProgressReporter(() => clock.NowSeconds);

            reporter.Begin();
            reporter.Poll();
            Assert.IsFalse(reporter.Visible);

            clock.NowSeconds = 0.49;
            reporter.Poll();
            Assert.IsFalse(reporter.Visible);

            clock.NowSeconds = 0.51;
            reporter.Poll();
            Assert.IsTrue(reporter.Visible);

            reporter.SetProgress(0.5f);
            Assert.AreEqual(0.5f, reporter.Progress);

            reporter.Complete();
            Assert.AreEqual(1f, reporter.Progress);
            Assert.IsFalse(reporter.Visible);
        }

        /// <summary>
        /// PERF-008: incremental work drains through FrameBudget — each tick
        /// stops at the 12 ms loading deadline, so no single frame does more
        /// than a budget's worth of work (no frozen frame &gt; 100 ms).
        /// </summary>
        [Test]
        public void TestNoFrozenFrameOver100ms()
        {
            var clock = new FakeClock();
            var budget = new FrameBudget(clock) { Loading = true };
            var loader = new IncrementalLoader(budget);

            const int items = 10;
            // 6.5 ms per step: the second item's end (13 ms) clears the 12 ms
            // deadline by a wide FP-safe margin — exactly 2 items per tick.
            const double stepSeconds = 0.0065;
            var executed = 0;
            for (var i = 0; i < items; i++)
            {
                loader.Enqueue(delegate()
                {
                    executed++;
                    clock.NowSeconds += stepSeconds;
                    return true;
                });
            }

            var frameIndex = 0L;
            clock.NowSeconds = 0.0;
            TickBudget(budget, clock, frameIndex++);
            // 6.5 ms per step under a 12 ms deadline => at most 2 items per tick.
            Assert.LessOrEqual(executed, 2);

            var beforeTick = executed;
            TickBudget(budget, clock, frameIndex++);
            Assert.LessOrEqual(executed - beforeTick, 2);

            while (loader.Remaining > 0)
            {
                var before = executed;
                TickBudget(budget, clock, frameIndex++);
                Assert.LessOrEqual(executed - before, 2);
            }

            Assert.AreEqual(0, loader.Remaining);
            Assert.AreEqual(1f, loader.Progress);
        }

        /// <summary>
        /// PERF-015: Enter raises the 12 ms loading budget + High background
        /// priority; Exit collects once and restores gameplay mode —
        /// idempotent across repeated enter/exit.
        /// </summary>
        [Test]
        public void TestLoadingPriorityAndBudgetMode()
        {
            var env = new FakeEnvironment();
            var mode = new LoadingPriorityMode(env);

            mode.Enter();
            Assert.IsTrue(env.Loading);
            Assert.IsTrue(env.HighPriority);
            Assert.AreEqual(0, env.Collections);

            mode.Enter();
            Assert.AreEqual(0, env.Collections);

            mode.Exit();
            Assert.AreEqual(1, env.Collections);
            Assert.IsFalse(env.Loading);
            Assert.IsFalse(env.HighPriority);

            mode.Exit();
            Assert.AreEqual(1, env.Collections);

            mode.Enter();
            mode.Exit();
            Assert.AreEqual(2, env.Collections);
        }

        private static void TickBudget(
            FrameBudget budget, FakeClock clock, long frameIndex)
        {
            var time = new FrameTime(0f, clock.NowSeconds, frameIndex);
            budget.Tick(in time);
        }
    }
}
