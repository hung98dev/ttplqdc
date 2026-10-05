using System;
using System.Collections.Generic;

namespace ThinhThan.Core.Session
{
    /// <summary>
    /// Session-state FSM on top of the UI-state machine (IMP-065). Transition
    /// legality mirrors protocol.md § Phase Legality and
    /// client_experience_contract.md; illegal transitions throw.
    /// </summary>
    public sealed class SessionStateMachine
    {
        private static readonly HashSet<(SessionPhase, SessionPhase)> _legalPhases =
            new HashSet<(SessionPhase, SessionPhase)>
            {
                (SessionPhase.Disconnected, SessionPhase.Connecting),
                (SessionPhase.Disconnected, SessionPhase.Reconnecting),
                (SessionPhase.Connecting, SessionPhase.Authenticating),
                (SessionPhase.Connecting, SessionPhase.Disconnected),
                (SessionPhase.Authenticating, SessionPhase.CharacterSelect),
                (SessionPhase.Authenticating, SessionPhase.InWorld),
                (SessionPhase.Authenticating, SessionPhase.Disconnected),
                (SessionPhase.Authenticating, SessionPhase.Reconnecting),
                (SessionPhase.CharacterSelect, SessionPhase.InWorld),
                (SessionPhase.CharacterSelect, SessionPhase.Disconnected),
                (SessionPhase.CharacterSelect, SessionPhase.Reconnecting),
                (SessionPhase.InWorld, SessionPhase.TransferringMap),
                (SessionPhase.InWorld, SessionPhase.CharacterSelect),
                (SessionPhase.InWorld, SessionPhase.Disconnected),
                (SessionPhase.InWorld, SessionPhase.Reconnecting),
                (SessionPhase.TransferringMap, SessionPhase.InWorld),
                (SessionPhase.TransferringMap, SessionPhase.Disconnected),
                (SessionPhase.TransferringMap, SessionPhase.Reconnecting),
                (SessionPhase.Reconnecting, SessionPhase.Connecting),
                (SessionPhase.Reconnecting, SessionPhase.Authenticating),
                (SessionPhase.Reconnecting, SessionPhase.CharacterSelect),
                (SessionPhase.Reconnecting, SessionPhase.InWorld),
                (SessionPhase.Reconnecting, SessionPhase.Disconnected),
            };

        private static readonly HashSet<(ClientUiState, ClientUiState)> _legalUi =
            new HashSet<(ClientUiState, ClientUiState)>
            {
                (ClientUiState.Boot, ClientUiState.PatchingUpdate),
                (ClientUiState.Boot, ClientUiState.AuthTitle),
                (ClientUiState.PatchingUpdate, ClientUiState.AuthTitle),
                (ClientUiState.AuthTitle, ClientUiState.LoginQueued),
                (ClientUiState.AuthTitle, ClientUiState.CharacterSelect),
                (ClientUiState.LoginQueued, ClientUiState.CharacterSelect),
                (ClientUiState.LoginQueued, ClientUiState.AuthTitle),
                (ClientUiState.CharacterSelect, ClientUiState.InWorld),
                (ClientUiState.CharacterSelect, ClientUiState.AuthTitle),
                (ClientUiState.CharacterSelect, ClientUiState.Disconnected),
                (ClientUiState.InWorld, ClientUiState.TransferringMap),
                (ClientUiState.InWorld, ClientUiState.CharacterSelect),
                (ClientUiState.InWorld, ClientUiState.Disconnected),
                (ClientUiState.TransferringMap, ClientUiState.InWorld),
                (ClientUiState.TransferringMap, ClientUiState.Disconnected),
                (ClientUiState.Disconnected, ClientUiState.AuthTitle),
                (ClientUiState.Disconnected, ClientUiState.CharacterSelect),
                (ClientUiState.Disconnected, ClientUiState.InWorld),
            };

        private SessionPhase _phase = SessionPhase.Disconnected;
        private ClientUiState _uiState = ClientUiState.Boot;

        /// <summary>Raised after every committed state or flag change.</summary>
        public event Action<SessionStateSnapshot>? Changed;

        public SessionPhase Phase
        {
            get
            {
                return _phase;
            }
        }

        public ClientUiState UiState
        {
            get
            {
                return _uiState;
            }
        }

        public bool PlacementPending
        {
            get;
            private set;
        }

        public bool DeadOverlay
        {
            get;
            private set;
        }

        public bool SessionReplaced
        {
            get;
            private set;
        }

        public bool ClientUpdateRequired
        {
            get;
            private set;
        }

        public bool ServerDraining
        {
            get;
            private set;
        }

        public ulong SessionEpoch
        {
            get;
            private set;
        }

        public int QueuePosition
        {
            get;
            private set;
        }

        public SessionStateSnapshot Snapshot
        {
            get
            {
                return new SessionStateSnapshot(
                    _phase,
                    _uiState,
                    PlacementPending,
                    DeadOverlay,
                    SessionReplaced,
                    ClientUpdateRequired,
                    ServerDraining,
                    SessionEpoch,
                    QueuePosition);
            }
        }

        /// <summary>Any state may move to AUTH_TITLE on a forced-update error.</summary>
        public void ForceUpdateRequired()
        {
            ClientUpdateRequired = true;
            SetPhaseInternal(SessionPhase.Disconnected);
            _uiState = ClientUiState.AuthTitle;
            Emit();
        }

        public void SetSessionReplaced()
        {
            SessionReplaced = true;
            SetPhaseInternal(SessionPhase.Disconnected);
            _uiState = ClientUiState.AuthTitle;
            Emit();
        }

        public void SetServerDraining()
        {
            ServerDraining = true;
            Emit();
        }

        /// <summary>Dismiss of the SESSION_REPLACED modal -> title.</summary>
        public void AcknowledgeSessionReplaced()
        {
            if (!SessionReplaced)
            {
                return;
            }

            SessionReplaced = false;
            Emit();
        }

        public void SetPlacementPending(bool value)
        {
            if (PlacementPending == value)
            {
                return;
            }

            PlacementPending = value;
            Emit();
        }

        public void SetDeadOverlay(bool value)
        {
            if (DeadOverlay == value)
            {
                return;
            }

            DeadOverlay = value;
            Emit();
        }

        public void SetQueuePosition(int position)
        {
            if (QueuePosition == position)
            {
                return;
            }

            QueuePosition = position;
            Emit();
        }

        /// <summary>Validates and commits a session-phase transition.</summary>
        public void TransitionPhase(SessionPhase next)
        {
            if (next == _phase)
            {
                return;
            }

            SetPhaseInternal(next);
            Emit();
        }

        /// <summary>Validates and commits a UI-state transition.</summary>
        public void TransitionUi(ClientUiState next)
        {
            if (next == _uiState)
            {
                return;
            }

            if (!_legalUi.Contains((_uiState, next)))
            {
                throw new InvalidOperationException(
                    "Illegal UI-state transition " + _uiState + " -> " + next);
            }

            _uiState = next;
            Emit();
        }

        /// <summary>Full reset to DISCONNECTED/BOOT, clearing every flag.</summary>
        public void Reset()
        {
            _phase = SessionPhase.Disconnected;
            _uiState = ClientUiState.Boot;
            PlacementPending = false;
            DeadOverlay = false;
            SessionReplaced = false;
            ClientUpdateRequired = false;
            ServerDraining = false;
            SessionEpoch = 0UL;
            QueuePosition = 0;
            Emit();
        }

        /// <summary>Called by the session driver when HELLO_OK assigns epoch.</summary>
        public void AcceptSessionEpoch(ulong epoch)
        {
            SessionEpoch = epoch;
            Emit();
        }

        /// <summary>Disconnect path skips the legality table: every live
        /// phase may drop to Disconnected, and DISCONNECTED UI retries then
        /// falls back to AUTH_TITLE.</summary>
        public void DropToDisconnected()
        {
            SetPhaseInternal(SessionPhase.Disconnected);
            _uiState = ClientUiState.Disconnected;
            PlacementPending = false;
            DeadOverlay = false;
            Emit();
        }

        private void SetPhaseInternal(SessionPhase next)
        {
            if (next == _phase)
            {
                return;
            }

            if (!_legalPhases.Contains((_phase, next)))
            {
                throw new InvalidOperationException(
                    "Illegal session-phase transition " + _phase + " -> " + next);
            }

            _phase = next;
        }

        private void Emit()
        {
            if (Changed != null)
            {
                Changed(Snapshot);
            }
        }
    }
}
