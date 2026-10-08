using System;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// LOGIN_QUEUED presenter (client_experience_contract.md §1,
    /// session.md § Login Queue): shows the live queue_position and a
    /// localized "retrying automatically" note — the retry itself runs
    /// inside SessionOrchestrator's ticket loop after retry_after_ms.
    /// Cancel raises <see cref="CancelRequested"/> so the composition cancels
    /// the connect token → the FSM returns to AUTH_TITLE.
    /// </summary>
    public sealed class LoginQueuePresenter
    {
        private readonly Func<int> _queuePosition;
        private readonly Action _cancel;

        /// <param name="queuePosition">Live position source (session snapshot).</param>
        /// <param name="cancel">Cancels the in-flight connect/ticket loop.</param>
        public LoginQueuePresenter(Func<int> queuePosition, Action cancel)
        {
            _queuePosition = queuePosition ??
                throw new ArgumentNullException(nameof(queuePosition));
            _cancel = cancel ?? throw new ArgumentNullException(nameof(cancel));
        }

        /// <summary>Current queue position, 0 when not yet reported.</summary>
        public int Position
        {
            get
            {
                return _queuePosition();
            }
        }

        /// <summary>Localization key for the position line (smart arg {position}).</summary>
        public string PositionKey
        {
            get
            {
                return ScreensLoc.QueuePosition;
            }
        }

        /// <summary>Localization key for the auto-retry note.</summary>
        public string RetryingKey
        {
            get
            {
                return ScreensLoc.QueueRetrying;
            }
        }

        /// <summary>Hủy: back to AUTH_TITLE.</summary>
        public void Cancel()
        {
            _cancel();
        }
    }
}
