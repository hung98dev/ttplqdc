namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// PERF-015 sink for loading-screen frame policy
    /// (client_performance.md § PERF-015). Composition binds the real Unity
    /// surface (<see cref="UnityLoadingEnvironment"/>); tests fake it.
    /// </summary>
    public interface ILoadingEnvironment
    {
        /// <summary>FrameBudget loading mode (12 ms) vs gameplay (2 ms).</summary>
        void SetLoading(bool loading);

        /// <summary>Application.backgroundLoadingPriority High vs Low.</summary>
        void SetBackgroundLoadingPriorityHigh(bool high);

        /// <summary>The single GC.Collect allowed before a loading screen closes.</summary>
        void CollectGarbageOnce();
    }
}
