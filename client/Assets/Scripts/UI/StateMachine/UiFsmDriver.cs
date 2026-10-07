using System;
using System.Threading;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using ThinhThan.Net;
using ThinhThan.Systems.Replication;
using ThinhThan.UI.Character;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Drives <see cref="UiScreenSet"/> from the session FSM: subscribes to
    /// <see cref="SessionStateMachine.Changed"/> and re-evaluates the
    /// overlay inputs that live outside the snapshot once per UI-phase tick
    /// (DEAD self flag + placement pending → dead overlay per §5; account
    /// pending-deletion + transfer-in-flight → screen chrome).
    /// <para>
    /// Button intents the screens raise — detach to CHARACTER_SELECT,
    /// logout to AUTH_TITLE, Đồng ý on SESSION_REPLACED — are forwarded to
    /// the orchestrator; views never talk to sockets.
    /// </para>
    /// </summary>
    public sealed class UiFsmDriver : IFrameSystem, IDisposable
    {
        private readonly SessionStateMachine _fsm;
        private readonly SessionOrchestrator _orchestrator;
        private readonly ReplicationApplier _replication;
        private readonly SessionUiPresenter _presenter;
        private readonly UiScreenSet _screens;

        public UiFsmDriver(
            SessionStateMachine fsm,
            SessionOrchestrator orchestrator,
            ReplicationApplier replication,
            SessionUiPresenter presenter,
            UiScreenSet screens)
        {
            _fsm = fsm ?? throw new ArgumentNullException(nameof(fsm));
            _orchestrator = orchestrator ??
                throw new ArgumentNullException(nameof(orchestrator));
            _replication = replication ??
                throw new ArgumentNullException(nameof(replication));
            _presenter = presenter ??
                throw new ArgumentNullException(nameof(presenter));
            _screens = screens ??
                throw new ArgumentNullException(nameof(screens));
            _fsm.Changed += OnChanged;
            Apply();
        }

        /// <summary>Đổi nhân vật: C2S_CHARACTER_DETACH path (§1).</summary>
        public void RequestDetach(CancellationToken cancel)
        {
            _ = _orchestrator.DetachAsync(cancel);
        }

        /// <summary>Đăng xuất / exit-to-title from the DISCONNECTED modal.</summary>
        public void RequestLogout(CancellationToken cancel)
        {
            _ = _orchestrator.LogoutAsync(cancel);
        }

        /// <summary>Đồng ý on the non-dismissible SESSION_REPLACED modal.</summary>
        public void AcknowledgeSessionReplaced()
        {
            _presenter.AcknowledgeSessionReplaced();
        }

        /// <summary>
        /// UI-phase pass: snapshot-driven surfaces were applied on
        /// <see cref="SessionStateMachine.Changed"/>; replication-driven
        /// ones (self DEAD flag) re-evaluate here so the respawn overlay
        /// tracks the applier without a second event bus.
        /// </summary>
        public void Tick(in FrameTime time)
        {
            // §1/§5: IN_WORLD + nhân vật DEAD + S2C_PLACEMENT_PENDING(RESPAWN)
            // → lớp phủ chờ hồi sinh, không timeout.
            bool dead = _replication.SelfState != null &&
                (_replication.SelfState.Flags & UiEntityFlags.Dead) != 0U;
            bool overlay = dead && _fsm.PlacementPending &&
                _fsm.UiState == ClientUiState.InWorld;
            if (overlay != _fsm.DeadOverlay)
            {
                _fsm.SetDeadOverlay(overlay);
            }

            Apply();
        }

        public void Dispose()
        {
            _fsm.Changed -= OnChanged;
        }

        private void OnChanged(SessionStateSnapshot snapshot)
        {
            Apply();
        }

        private void Apply()
        {
            _screens.Apply(
                _presenter.Screen,
                _presenter.QueuePosition,
                _presenter.PlacementPending,
                _presenter.DeadOverlay,
                _fsm.UiState == ClientUiState.TransferringMap &&
                    !_fsm.PlacementPending,
                _presenter.PendingDeletion,
                _presenter.Modal);
        }
    }
}
