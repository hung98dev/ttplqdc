using System;
using ThinhThan.Core.Runtime;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// Incremental load pump for PERF-008: queued work items drain through
    /// <see cref="FrameBudget"/>, whose per-tick cap (12 ms in loading mode)
    /// keeps any single frame from freezing past the 100 ms bound. A step
    /// returns false to resume next frame; true marks it complete and
    /// advances the reported progress by one item.
    /// </summary>
    public sealed class IncrementalLoader
    {
        private readonly FrameBudget _budget;
        private int _completed;
        private int _total;

        public IncrementalLoader(FrameBudget budget)
        {
            _budget = budget ?? throw new ArgumentNullException(nameof(budget));
        }

        /// <summary>Completed/total progress in [0, 1] (1 when nothing queued).</summary>
        public float Progress
        {
            get
            {
                return _total == 0 ? 1f : (float)_completed / _total;
            }
        }

        /// <summary>Items not yet complete.</summary>
        public int Remaining
        {
            get
            {
                return _total - _completed;
            }
        }

        /// <summary>
        /// Enqueues a step: <paramref name="step"/> runs inside FrameBudget
        /// until it returns true (done). Long work splits itself across
        /// frames by returning false; the budget deadline bounds each frame.
        /// </summary>
        public void Enqueue(Func<bool> step)
        {
            if (step == null)
            {
                throw new ArgumentNullException(nameof(step));
            }

            _total++;
            _budget.Enqueue(delegate()
            {
                if (!step())
                {
                    return false;
                }

                _completed++;
                return true;
            });
        }
    }
}
