using System;
using System.Collections.Generic;

namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// Bounded per-frame work queue (client_performance.md § Smoothness by
    /// Construction): gameplay mode drains at most
    /// <see cref="GameplayBudgetSeconds"/> (2 ms) of work per tick, loading
    /// mode <see cref="LoadingBudgetSeconds"/> (12 ms). Work that does not fit
    /// the current frame is carried, not dropped — the queue keeps the
    /// remainder for the next tick. Time comes from an injected
    /// <see cref="IClock"/> so tests are deterministic.
    /// </summary>
    public sealed class FrameBudget : IFrameSystem
    {
        public const double GameplayBudgetSeconds = 0.002;

        public const double LoadingBudgetSeconds = 0.012;

        private readonly Queue<Func<bool>> _pending = new Queue<Func<bool>>();
        private readonly IClock _clock;
        private double _budgetSeconds = GameplayBudgetSeconds;

        public FrameBudget(IClock clock)
        {
            _clock = clock ?? throw new ArgumentNullException(nameof(clock));
        }

        /// <summary>
        /// When true the loading-screen budget (12 ms) applies; otherwise the
        /// gameplay budget (2 ms).
        /// </summary>
        public bool Loading
        {
            get
            {
                return _budgetSeconds == LoadingBudgetSeconds;
            }
            set
            {
                _budgetSeconds = value ? LoadingBudgetSeconds : GameplayBudgetSeconds;
            }
        }

        public int PendingCount
        {
            get
            {
                return _pending.Count;
            }
        }

        /// <summary>
        /// Queues a work item. The item returns true when finished; returning
        /// false keeps it at the head of the queue for the next tick.
        /// </summary>
        public void Enqueue(Func<bool> workItem)
        {
            if (workItem == null)
            {
                throw new ArgumentNullException(nameof(workItem));
            }

            _pending.Enqueue(workItem);
        }

        public void Tick(in FrameTime time)
        {
            double deadline = _clock.NowSeconds + _budgetSeconds;
            while (_pending.Count > 0)
            {
                Func<bool> item = _pending.Peek();
                if (item())
                {
                    _pending.Dequeue();
                }
                else
                {
                    break;
                }

                if (_clock.NowSeconds >= deadline)
                {
                    break;
                }
            }
        }
    }
}
