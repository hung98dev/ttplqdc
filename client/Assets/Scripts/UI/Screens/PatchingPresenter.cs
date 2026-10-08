using System;
using System.Threading;
using UnityEngine;

namespace ThinhThan.UI.Screens
{

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
