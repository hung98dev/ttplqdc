using System;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// TRANSFERRING_MAP presenter (client_experience_contract.md §5): dimmed
    /// screen with the region's folk art, an Addressables progress bar, and
    /// the PLACEMENT_PENDING waiting label. The 30 s (120 s instance) budget
    /// counts from S2C_TRANSFER_PREPARE; time in PLACEMENT_PENDING does not
    /// count. On budget expiry the screen shows the fail message and asks
    /// the composition to return to the safe point. While the screen is
    /// active it holds PERF-015's loading policy and PERF-008's progress
    /// surface.
    /// </summary>
    public sealed class TransferPresenter
    {
        /// <summary>Standard transfer budget (seconds, from TRANSFER_PREPARE).</summary>
        public const double StandardBudgetSeconds = 30.0;

        /// <summary>Instance transfer budget (seconds).</summary>
        public const double InstanceBudgetSeconds = 120.0;

        private readonly LoadingPriorityMode _loading;
        private readonly LoadingProgressReporter _progress;
        private readonly Func<double> _nowSeconds;
        private readonly Action _budgetExceeded;

        private double _prepareAtSeconds = -1.0;
        private double _budgetSeconds = StandardBudgetSeconds;
        private bool _placementPending;
        private bool _failed;

        /// <param name="budgetExceeded">Composition callback: cancel + safe point.</param>
        public TransferPresenter(
            LoadingPriorityMode loading,
            LoadingProgressReporter progress,
            Func<double> nowSeconds,
            Action budgetExceeded)
        {
            _loading = loading ??
                throw new ArgumentNullException(nameof(loading));
            _progress = progress ??
                throw new ArgumentNullException(nameof(progress));
            _nowSeconds = nowSeconds ??
                throw new ArgumentNullException(nameof(nowSeconds));
            _budgetExceeded = budgetExceeded ??
                throw new ArgumentNullException(nameof(budgetExceeded));
        }

        /// <summary>The load exceeded its budget (fail message shown).</summary>
        public bool Failed
        {
            get
            {
                return _failed;
            }
        }

        /// <summary>PLACEMENT_PENDING substate: waiting on the server, no timeout.</summary>
        public bool PlacementPending
        {
            get
            {
                return _placementPending;
            }
        }

        /// <summary>Progress reporter driving the bar + PERF-008 visibility.</summary>
        public LoadingProgressReporter Progress
        {
            get
            {
                return _progress;
            }
        }

        /// <summary>Localization key for the status line.</summary>
        public string StatusKey
        {
            get
            {
                if (_failed)
                {
                    return ScreensLoc.TransferFailed;
                }

                return _placementPending
                    ? ScreensLoc.TransferWaitingPlacement
                    : ScreensLoc.TransferTitle;
            }
        }

        /// <summary>
        /// Enter TRANSFERRING_MAP for this transfer. The PERF-015 policy
        /// raises now (12 ms FrameBudget + High loading priority); the budget
        /// clock starts at S2C_TRANSFER_PREPARE, not at placement wait.
        /// </summary>
        public void EnterTransfer(bool instance)
        {
            _loading.Enter();
            _progress.Begin();
            _budgetSeconds = instance ? InstanceBudgetSeconds : StandardBudgetSeconds;
            _prepareAtSeconds = -1.0;
            _placementPending = false;
            _failed = false;
        }

        /// <summary>S2C_TRANSFER_PREPARE arrived: budget clock starts.</summary>
        public void OnTransferPrepare()
        {
            _prepareAtSeconds = _nowSeconds();
        }

        /// <summary>S2C_PLACEMENT_PENDING: show the waiting line, hold the clock.</summary>
        public void SetPlacementPending(bool pending)
        {
            _placementPending = pending;
        }

        /// <summary>Addressables progress update (0..1).</summary>
        public void SetLoadProgress(float progress)
        {
            _progress.SetProgress(progress);
        }

        /// <summary>
        /// UI-phase poll: PERF-008 visibility + budget check. Placement
        /// pending time never counts toward the 30 s/120 s budget.
        /// </summary>
        public void Poll()
        {
            _progress.Poll();
            if (_failed || _placementPending || _prepareAtSeconds < 0.0)
            {
                return;
            }

            if (_nowSeconds() - _prepareAtSeconds > _budgetSeconds)
            {
                _failed = true;
                _budgetExceeded();
            }
        }

        /// <summary>Screen closes (IN_WORLD reached or aborted): restore budgets.</summary>
        public void LeaveTransfer()
        {
            _progress.Complete();
            _loading.Exit();
            _prepareAtSeconds = -1.0;
            _placementPending = false;
            _failed = false;
        }
    }
}
