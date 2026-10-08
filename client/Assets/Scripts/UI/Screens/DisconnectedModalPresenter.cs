using System;
using System.Collections.Generic;
using ThinhThan.Core.Localization;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// DISCONNECTED retry modal presenter (client_experience_contract.md §5):
    /// "Mất kết nối tới máy chủ. Đang thử kết nối lại... (Lần n/5)" with
    /// Thử lại ngay + Thoát ra màn hình chính buttons. The attempt counter
    /// comes from the session driver's reconnect policy; buttons forward to
    /// injected intents so views never touch sockets.
    /// </summary>
    public sealed class DisconnectedModalPresenter
    {
        /// <summary>Max auto-retries per the §5 contract / ReconnectPolicy.</summary>
        public const int MaxAttempts = 5;

        private readonly Func<int> _attempt;
        private readonly Action _retryNow;
        private readonly Action _exitToTitle;

        /// <param name="attempt">Live reconnect-attempt source (1..5).</param>
        /// <param name="retryNow">Retry-now button intent.</param>
        /// <param name="exitToTitle">Exit-to-title button intent.</param>
        public DisconnectedModalPresenter(
            Func<int> attempt, Action retryNow, Action exitToTitle)
        {
            _attempt = attempt ?? throw new ArgumentNullException(nameof(attempt));
            _retryNow = retryNow ?? throw new ArgumentNullException(nameof(retryNow));
            _exitToTitle = exitToTitle ??
                throw new ArgumentNullException(nameof(exitToTitle));
        }

        /// <summary>Current reconnect attempt, clamped to 1..MaxAttempts.</summary>
        public int Attempt
        {
            get
            {
                int a = _attempt();
                if (a < 1)
                {
                    return 1;
                }

                return a > MaxAttempts ? MaxAttempts : a;
            }
        }

        /// <summary>Formatted "(Lần n/5)" line in the active locale.</summary>
        public string AttemptLine
        {
            get
            {
                return Loc.Get(
                    ScreensLoc.DisconnectedRetry,
                    new Dictionary<string, object>
                    {
                        { "attempt", Attempt },
                        { "max", MaxAttempts },
                    });
            }
        }

        /// <summary>Thử lại ngay button.</summary>
        public void RetryNow()
        {
            _retryNow();
        }

        /// <summary>Thoát ra màn hình chính button.</summary>
        public void ExitToTitle()
        {
            _exitToTitle();
        }
    }
}
