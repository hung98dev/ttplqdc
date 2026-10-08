using System;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// PERF-015 loading-screen frame policy (client_performance.md § PERF-015):
    /// while a loading screen is active the FrameBudget runs in 12 ms mode and
    /// <c>backgroundLoadingPriority</c> is High; the close performs exactly one
    /// <c>GC.Collect</c> before restoring 2 ms / Low for gameplay. Enter/Exit
    /// are idempotent — duplicate calls never collect twice or flip state.
    /// </summary>
    public sealed class LoadingPriorityMode
    {
        private readonly ILoadingEnvironment _environment;
        private bool _active;
        private bool _collectPending;

        public LoadingPriorityMode(ILoadingEnvironment environment)
        {
            _environment = environment ??
                throw new ArgumentNullException(nameof(environment));
        }

        /// <summary>True while a loading screen holds the raised budget.</summary>
        public bool Active
        {
            get
            {
                return _active;
            }
        }

        /// <summary>
        /// Enter the loading policy: FrameBudget 12 ms + High loading
        /// priority. The GC.Collect is deferred to <see cref="Exit"/> so it
        /// runs once, right before gameplay resumes.
        /// </summary>
        public void Enter()
        {
            if (_active)
            {
                return;
            }

            _active = true;
            _collectPending = true;
            _environment.SetLoading(true);
            _environment.SetBackgroundLoadingPriorityHigh(true);
        }

        /// <summary>
        /// Exit the loading policy: one GC.Collect, then 2 ms / Low.
        /// </summary>
        public void Exit()
        {
            if (!_active)
            {
                return;
            }

            if (_collectPending)
            {
                _collectPending = false;
                _environment.CollectGarbageOnce();
            }

            _environment.SetLoading(false);
            _environment.SetBackgroundLoadingPriorityHigh(false);
            _active = false;
        }
    }
}
