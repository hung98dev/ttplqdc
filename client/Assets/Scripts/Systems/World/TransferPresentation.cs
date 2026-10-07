using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;

namespace ThinhThan.Systems.World
{
    /// <summary>
    /// TRANSFERRING_MAP presentation state (client_experience_contract.md):
    /// loading progress, the PLACEMENT_PENDING sub-state and timeout
    /// accounting. The budget clock (30 s world / 120 s instance per
    /// concurrency.md) excludes time spent in placement-pending — the
    /// server is still working on placement, not on the client's load.
    /// </summary>
    public sealed class TransferPresentation
    {
        /// <summary>Notice state key raised when the transfer fails
        /// ("Chuyển vùng thất bại…" — the localized text lives in the
        /// localization tables; UI resolves it).</summary>
        public const string FailureNoticeKey = "loc.world_map.transfer_failed";

        /// <summary>Presentation sub-state.</summary>
        public enum State
        {
            /// <summary>No transfer in flight.</summary>
            None = 0,

            /// <summary>105 received; client loading map content.</summary>
            Loading = 1,

            /// <summary>Map load succeeded; baseline settle in flight.</summary>
            Settling = 2,

            /// <summary>Transfer failed / timed out (notice raised).</summary>
            Failed = 3,
        }

        private readonly IClock _clock;
        private State _state = State.None;
        private bool _placementPending;
        private double _beganAt;
        private double _budgetSeconds;
        private double _pendingSince;
        private double _pendingAccumulated;

        public TransferPresentation(IClock clock)
        {
            _clock = clock;
        }

        /// <summary>Destination of the in-flight transfer.</summary>
        public TransferDestination Destination
        {
            get;
            private set;
        }

        /// <summary>Current sub-state.</summary>
        public State Current
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Whether the placement-pending overlay is up.</summary>
        public bool PlacementPending
        {
            get
            {
                return _placementPending;
            }
        }

        /// <summary>Active transfer time, excluding placement-pending.</summary>
        public double ElapsedSeconds
        {
            get
            {
                if (_state == State.None)
                {
                    return 0.0;
                }
                var pending = _pendingAccumulated;
                if (_placementPending)
                {
                    pending += _clock.NowSeconds - _pendingSince;
                }
                return _clock.NowSeconds - _beganAt - pending;
            }
        }

        /// <summary>Whether the active (non-pending) budget is spent.</summary>
        public bool Expired
        {
            get
            {
                return _state == State.Loading && ElapsedSeconds >= _budgetSeconds;
            }
        }

        /// <summary>105 received: starts the transfer clock.</summary>
        public void Begin(in TransferDestination destination, double budgetSeconds)
        {
            Destination = destination;
            _state = State.Loading;
            _placementPending = false;
            _beganAt = _clock.NowSeconds;
            _budgetSeconds = budgetSeconds;
            _pendingSince = 0.0;
            _pendingAccumulated = 0.0;
        }

        /// <summary>Map load + ready accepted; baseline settle pending.</summary>
        public void BeginSettling()
        {
            if (_state == State.Loading)
            {
                _state = State.Settling;
            }
        }

        /// <summary>
        /// 15-driven placement-pending sub-state: pauses the budget clock.
        /// </summary>
        public void SetPlacementPending(bool pending)
        {
            if (pending == _placementPending || _state == State.None)
            {
                return;
            }
            if (pending)
            {
                _placementPending = true;
                _pendingSince = _clock.NowSeconds;
            }
            else
            {
                _placementPending = false;
                _pendingAccumulated += _clock.NowSeconds - _pendingSince;
            }
        }

        /// <summary>Transfer failed or timed out — raises the notice state.</summary>
        public void Fail()
        {
            if (_state != State.None)
            {
                _state = State.Failed;
                _placementPending = false;
            }
        }

        /// <summary>Baseline arrived / context cleared: closes the state.</summary>
        public void Clear()
        {
            _state = State.None;
            _placementPending = false;
            _pendingAccumulated = 0.0;
            Destination = default;
        }
    }
}
