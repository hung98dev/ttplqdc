using System;
using System.Threading;
using System.Threading.Tasks;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// BOOT presenter (client_experience_contract.md §1): version check +
    /// Addressables catalog load through an injected step; outcome selects
    /// PATCHING_UPDATE or AUTH_TITLE.
    /// </summary>
    public sealed class BootPresenter
    {
        /// <summary>Boot outcome driving the next §1 state.</summary>
        public enum Outcome
        {
            PatchingUpdate,
            AuthTitle,
        }

        private readonly Func<CancellationToken, Task<bool>> _catalogCheck;
        private readonly Action<Outcome> _done;
        private bool _busy;

        /// <param name="catalogCheck">Version/catalog step; true = update available.</param>
        /// <param name="done">Receives the chosen next state.</param>
        public BootPresenter(
            Func<CancellationToken, Task<bool>> catalogCheck,
            Action<Outcome> done)
        {
            _catalogCheck = catalogCheck ??
                throw new ArgumentNullException(nameof(catalogCheck));
            _done = done ?? throw new ArgumentNullException(nameof(done));
        }

        /// <summary>Runs the boot sequence once.</summary>
        public async Task RunAsync(CancellationToken cancel)
        {
            if (_busy)
            {
                return;
            }

            _busy = true;
            bool needsUpdate = await _catalogCheck(cancel).ConfigureAwait(false);
            _done(needsUpdate ? Outcome.PatchingUpdate : Outcome.AuthTitle);
        }
    }

    /// <summary>
    /// PATCHING_UPDATE presenter (client_experience_contract.md §1): percent
    /// progress bar for the resource update; completion moves to AUTH_TITLE.
    /// Progress comes through PERF-008's reporter so a slow update always
    /// shows a bar past 0.5 s.
    /// </summary>
    public sealed class PatchingPresenter
    {
        private readonly LoadingProgressReporter _progress;
        private readonly Action _done;
        private float _percent;

        public PatchingPresenter(LoadingProgressReporter progress, Action done)
        {
            _progress = progress ??
                throw new ArgumentNullException(nameof(progress));
            _done = done ?? throw new ArgumentNullException(nameof(done));
        }

        /// <summary>0..1 update fraction for the bar.</summary>
        public float Percent
        {
            get
            {
                return _percent;
            }
        }

        /// <summary>Starts the patching wait.</summary>
        public void Begin()
        {
            _progress.Begin();
            _percent = 0f;
        }

        /// <summary>Patches progress; completion reaches AUTH_TITLE.</summary>
        public void SetPercent(float percent)
        {
            _percent = percent < 0f ? 0f : percent > 1f ? 1f : percent;
            _progress.SetProgress(_percent);
            _progress.Poll();
            if (_percent >= 1f)
            {
                _progress.Complete();
                _done();
            }
        }
    }
}
