namespace ThinhThan.Core.Session
{
    /// <summary>
    /// Immutable view of <see cref="SessionStateMachine"/> delivered to
    /// listeners on every transition or flag change.
    /// </summary>
    public readonly struct SessionStateSnapshot
    {
        public SessionStateSnapshot(
            SessionPhase phase,
            ClientUiState uiState,
            bool placementPending,
            bool deadOverlay,
            bool sessionReplaced,
            bool clientUpdateRequired,
            bool serverDraining,
            ulong sessionEpoch,
            int queuePosition)
        {
            Phase = phase;
            UiState = uiState;
            PlacementPending = placementPending;
            DeadOverlay = deadOverlay;
            SessionReplaced = sessionReplaced;
            ClientUpdateRequired = clientUpdateRequired;
            ServerDraining = serverDraining;
            SessionEpoch = sessionEpoch;
            QueuePosition = queuePosition;
        }

        public SessionPhase Phase
        {
            get;
        }

        public ClientUiState UiState
        {
            get;
        }

        /// <summary>TRANSFERRING_MAP substate: server is holding placement.</summary>
        public bool PlacementPending
        {
            get;
        }

        /// <summary>DEAD overlay while RESPAWN is pending.</summary>
        public bool DeadOverlay
        {
            get;
        }

        /// <summary>Non-dismissible SESSION_REPLACED modal is showing.</summary>
        public bool SessionReplaced
        {
            get;
        }

        /// <summary>CLIENT_UPDATE_REQUIRED / PROTOCOL_UNSUPPORTED received.</summary>
        public bool ClientUpdateRequired
        {
            get;
        }

        /// <summary>SERVER_DRAINING announced: reconnect after the deadline.</summary>
        public bool ServerDraining
        {
            get;
        }

        public ulong SessionEpoch
        {
            get;
        }

        /// <summary>Last advertised login-queue position; 0 when not queued.</summary>
        public int QueuePosition
        {
            get;
        }
    }
}
