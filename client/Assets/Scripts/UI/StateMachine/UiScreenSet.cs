using System;
using ThinhThan.Core.Session;
using ThinhThan.UI.Character;
using UnityEngine;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// The eight §1 screens plus the overlay/modal surfaces they carry
    /// (client_experience_contract.md §1, §5): LOGIN_QUEUED position,
    /// TRANSFERRING_MAP progress/PLACEMENT_PENDING label, DEAD wait
    /// overlay, DISCONNECTED retry modal, non-dismissible
    /// SESSION_REPLACED.
    /// <para>
    /// Screen roots are bound by composition; unbound states still track
    /// active state so the driver and tests work headless. A bound
    /// <see cref="GameObject"/> is activated only on a real state change —
    /// PERF-022 never rebuilds static layout for a HUD value change.
    /// </para>
    /// </summary>
    public sealed class UiScreenSet
    {
        private static readonly int ScreenCount =
            Enum.GetValues(typeof(ClientUiState)).Length;
        private static readonly int ModalCount =
            Enum.GetValues(typeof(SessionModal)).Length;

        private readonly GameObject?[] _screens = new GameObject?[ScreenCount];
        private readonly GameObject?[] _modals = new GameObject?[ModalCount];
        private ClientUiState _active = ClientUiState.Boot;
        private SessionModal _modal = SessionModal.None;

        /// <summary>Raised when the active screen or modal changes.</summary>
        public event Action? Changed;

        /// <summary>Currently shown §1 screen.</summary>
        public ClientUiState ActiveScreen
        {
            get
            {
                return _active;
            }
        }

        /// <summary>Currently shown modal kind.</summary>
        public SessionModal Modal
        {
            get
            {
                return _modal;
            }
        }

        /// <summary>LOGIN_QUEUED display position.</summary>
        public int QueuePosition
        {
            get;
            private set;
        }

        /// <summary>TRANSFERRING_MAP: server holds placement.</summary>
        public bool PlacementPending
        {
            get;
            private set;
        }

        /// <summary>TRANSFERRING_MAP: a transfer is in flight.</summary>
        public bool TransferPending
        {
            get;
            private set;
        }

        /// <summary>IN_WORLD: dead + respawn placement pending.</summary>
        public bool DeadOverlay
        {
            get;
            private set;
        }

        /// <summary>Account pending deletion (title offers only cancel/logout).</summary>
        public bool PendingDeletion
        {
            get;
            private set;
        }

        /// <summary>Bound root activated for a §1 screen.</summary>
        public void BindScreen(ClientUiState state, GameObject root)
        {
            _screens[(int)state] = root ??
                throw new ArgumentNullException(nameof(root));
            RefreshScreen(state);
        }

        /// <summary>Bound root activated while a modal kind shows.</summary>
        public void BindModal(SessionModal kind, GameObject root)
        {
            _modals[(int)kind] = root ??
                throw new ArgumentNullException(nameof(root));
            RefreshModal(kind);
        }

        /// <summary>
        /// Shows a §1 screen: activates its bound root, deactivates the
        /// previous one. Same-state calls are no-ops.
        /// </summary>
        public void Show(ClientUiState state)
        {
            if (state == _active)
            {
                return;
            }

            GameObject? previous = _screens[(int)_active];
            _active = state;
            if (previous != null)
            {
                previous.SetActive(false);
            }

            RefreshScreen(state);
            Emit();
        }

        /// <summary>Applies one session snapshot's UI-facing state.</summary>
        public void Apply(
            ClientUiState screen,
            int queuePosition,
            bool placementPending,
            bool deadOverlay,
            bool transferPending,
            bool pendingDeletion,
            SessionModal modal)
        {
            bool changed =
                queuePosition != QueuePosition ||
                placementPending != PlacementPending ||
                deadOverlay != DeadOverlay ||
                transferPending != TransferPending ||
                pendingDeletion != PendingDeletion;
            QueuePosition = queuePosition;
            PlacementPending = placementPending;
            DeadOverlay = deadOverlay;
            TransferPending = transferPending;
            PendingDeletion = pendingDeletion;

            if (modal != _modal)
            {
                GameObject? previous =
                    _modal != SessionModal.None ? _modals[(int)_modal] : null;
                _modal = modal;
                if (previous != null)
                {
                    previous.SetActive(false);
                }

                if (modal != SessionModal.None)
                {
                    RefreshModal(modal);
                }

                changed = true;
            }

            ClientUiState before = _active;
            Show(screen);
            if (changed && before == _active)
            {
                Emit();
            }
        }

        private void RefreshScreen(ClientUiState state)
        {
            GameObject? root = _screens[(int)state];
            if (root != null)
            {
                root.SetActive(state == _active);
            }
        }

        private void RefreshModal(SessionModal kind)
        {
            GameObject? root = _modals[(int)kind];
            if (root != null)
            {
                root.SetActive(kind == _modal);
            }
        }

        private void Emit()
        {
            if (Changed != null)
            {
                Changed();
            }
        }
    }
}
