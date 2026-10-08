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
}
