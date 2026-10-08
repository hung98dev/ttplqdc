using System;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// PERF-008 progress surface (client_performance.md § PERF-008): a wait
    /// longer than 0.5 s must show progress, and loading work must never
    /// freeze a frame longer than 100 ms. This reporter decides visibility
    /// off the injected clock and carries the 0..1 progress the view binds;
    /// the async loader feeding it yields work through FrameBudget so no
    /// single frame blocks.
    /// </summary>
    public sealed class LoadingProgressReporter
    {
        /// <summary>Wait threshold before progress UI becomes visible.</summary>
        public const double ShowAfterSeconds = 0.5;

        private readonly Func<double> _nowSeconds;
        private double _startedAtSeconds = -1.0;
        private float _progress;

        public LoadingProgressReporter(Func<double> nowSeconds)
        {
            _nowSeconds = nowSeconds ??
                throw new ArgumentNullException(nameof(nowSeconds));
        }

        /// <summary>Progress UI may show (elapsed &gt; 0.5 s).</summary>
        public bool Visible
        {
            get;
            private set;
        }

        /// <summary>Latest reported progress in [0, 1].</summary>
        public float Progress
        {
            get
            {
                return _progress;
            }
        }

        /// <summary>Marks the start of the wait being reported.</summary>
        public void Begin()
        {
            _startedAtSeconds = _nowSeconds();
            Visible = false;
            _progress = 0f;
        }

        /// <summary>Per-tick poll: flips <see cref="Visible"/> past the threshold.</summary>
        public void Poll()
        {
            if (_startedAtSeconds < 0.0 || Visible)
            {
                return;
            }

            if (_nowSeconds() - _startedAtSeconds > ShowAfterSeconds)
            {
                Visible = true;
            }
        }

        /// <summary>Records completion progress (clamped to [0, 1]).</summary>
        public void SetProgress(float progress)
        {
            _progress = progress < 0f ? 0f : progress > 1f ? 1f : progress;
        }

        /// <summary>Ends the wait; visibility and progress reset.</summary>
        public void Complete()
        {
            _startedAtSeconds = -1.0;
            Visible = false;
            _progress = 1f;
        }
    }
}
