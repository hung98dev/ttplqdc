using System;
using ThinhThan.Core.Performance;
using ThinhThan.Core.Runtime;
using UnityEngine.Scripting;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// Unity-backed <see cref="ILoadingEnvironment"/>: maps the loading flag
    /// onto <see cref="FrameBudget.Loading"/> (12 ms loading / 2 ms gameplay
    /// per PERF-015); background-loading priority goes through
    /// <see cref="FramePacing.SetLoadingScreenActive"/>; one incremental
    /// garbage-collect step runs before the screen restores.
    /// </summary>
    public sealed class UnityLoadingEnvironment : ILoadingEnvironment
    {
        private readonly FrameBudget _budget;

        public UnityLoadingEnvironment(FrameBudget budget)
        {
            _budget = budget ?? throw new ArgumentNullException(nameof(budget));
        }

        public void SetLoading(bool loading)
        {
            _budget.Loading = loading;
        }

        public void SetBackgroundLoadingPriorityHigh(bool high)
        {
            FramePacing.SetLoadingScreenActive(high);
        }

        public void CollectGarbageOnce()
        {
            GarbageCollector.CollectIncremental(
                FramePacing.IncrementalGcSliceNanoseconds);
        }
    }
}
