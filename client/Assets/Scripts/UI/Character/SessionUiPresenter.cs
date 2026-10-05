using System;
using ThinhThan.Core.Session;
using ThinhThan.Net;

namespace ThinhThan.UI.Character
{
    /// <summary>
    /// Maps session state to the UI surfaces of client_experience_contract.md:
    /// screen per ClientUiState, LOGIN_QUEUED counter, the non-dismissible
    /// SESSION_REPLACED modal ("Tài khoản của bạn đã được đăng nhập từ một
    /// thiết bị khác." + Đồng ý), dead overlay, and PLACEMENT_PENDING inside
    /// TRANSFERRING_MAP. View layers read the snapshot; they never talk to
    /// sockets.
    /// </summary>
    public sealed class SessionUiPresenter
    {
        private readonly SessionOrchestrator _orchestrator;
        private readonly SessionStateMachine _fsm;

        public SessionUiPresenter(
            SessionOrchestrator orchestrator, SessionStateMachine fsm)
        {
            _orchestrator = orchestrator ??
                throw new ArgumentNullException(nameof(orchestrator));
            _fsm = fsm ?? throw new ArgumentNullException(nameof(fsm));
        }

        /// <summary>Active modal kind.</summary>
        public SessionModal Modal
        {
            get
            {
                if (_fsm.SessionReplaced)
                {
                    return SessionModal.SessionReplaced;
                }

                if (_fsm.ClientUpdateRequired)
                {
                    return SessionModal.ClientUpdateRequired;
                }

                if (_fsm.ServerDraining)
                {
                    return SessionModal.ServerDraining;
                }

                if (_fsm.UiState == ClientUiState.Disconnected)
                {
                    return SessionModal.Disconnected;
                }

                return SessionModal.None;
            }
        }

        /// <summary>The screen the shell should be showing.</summary>
        public ClientUiState Screen
        {
            get
            {
                return _fsm.UiState;
            }
        }

        /// <summary>Login-queue position shown while LOGIN_QUEUED.</summary>
        public int QueuePosition
        {
            get
            {
                return _fsm.QueuePosition;
            }
        }

        /// <summary>TRANSFERRING_MAP substate (no client timeout).</summary>
        public bool PlacementPending
        {
            get
            {
                return _fsm.PlacementPending;
            }
        }

        /// <summary>DEAD overlay while a respawn is pending.</summary>
        public bool DeadOverlay
        {
            get
            {
                return _fsm.DeadOverlay;
            }
        }

        /// <summary>Account pending deletion: UI offers only
        /// cancel-deletion/logout.</summary>
        public bool PendingDeletion
        {
            get
            {
                return _orchestrator.PendingDeletion;
            }
        }

        /// <summary>Đồng ý on the SESSION_REPLACED modal.</summary>
        public void AcknowledgeSessionReplaced()
        {
            _orchestrator.AcknowledgeSessionReplaced();
        }
    }
}
