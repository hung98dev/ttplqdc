using ThinhThan.Core.Runtime;

namespace ThinhThan.Net
{
    /// <summary>
    /// Reconnect backoff per reconnect.md: bounded exponential 1..5 s with
    /// jitter, up to <see cref="MaxAttempts"/> attempts before the UI falls
    /// back to AUTH_TITLE (client_experience_contract.md DISCONNECTED).
    /// </summary>
    public sealed class ReconnectPolicy
    {
        public const int MaxAttempts = 5;

        public const int MinDelayMs = 1000;

        public const int MaxDelayMs = 5000;

        private readonly PresentationRandom _jitter;
        private int _attempt;

        public ReconnectPolicy(PresentationRandom? jitter = null)
        {
            _jitter = jitter ?? new PresentationRandom(0x74697461UL);
        }

        public int Attempt
        {
            get
            {
                return _attempt;
            }
        }

        /// <summary>True while another attempt remains.</summary>
        public bool CanRetry
        {
            get
            {
                return _attempt < MaxAttempts;
            }
        }

        /// <summary>
        /// Delay before the next attempt: 1 s * 2^attempt, capped at 5 s, with
        /// up to 25% jitter. Returns -1 when the budget is exhausted.
        /// </summary>
        public int NextDelayMs()
        {
            if (_attempt >= MaxAttempts)
            {
                return -1;
            }

            int exponential = MinDelayMs << _attempt;
            int bounded = exponential > MaxDelayMs ? MaxDelayMs : exponential;
            int jitter = _jitter.NextInt(bounded / 4 + 1);
            _attempt++;
            return bounded + jitter;
        }

        public void Reset()
        {
            _attempt = 0;
        }
    }
}
