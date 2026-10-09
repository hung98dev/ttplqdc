using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.LifeSkills.Fishing
{
    /// <summary>
    /// Fishing presentation state driven by recorded
    /// S2C_INTERACT_RESULT (116) frames (world_rules.md § Folk
    /// Fishing): a CAST success moves the session to CASTING; the hook
    /// window's timing UI is presented from the authored constants
    /// (opens 2 s after cast, one HOOK in [0.40 s, 1.20 s]) purely as a
    /// client hint — the server accepts or rejects on its own clock;
    /// a HOOK success records the granted catch and projects the daily
    /// counter; a rare catch raises the single PHAT_HIEN peak flag
    /// (merged with Atlas Seen — one peak per settlement); spectators
    /// raise <see cref="SpectatorSplash"/> for the same-map splash.
    /// </summary>
    public sealed class FishingPresentation
    {
        /// <summary>Per-action result state.</summary>
        public enum ActionState
        {
            /// <summary>No request in flight.</summary>
            Idle = 0,

            /// <summary>103 sent; waiting on the recorded 116.</summary>
            Pending = 1,

            /// <summary>116 SUCCESS.</summary>
            Succeeded = 2,

            /// <summary>116 ERROR — <see cref="FailureCode"/> set.</summary>
            Failed = 3,
        }

        /// <summary>FSM of the live cast.</summary>
        public enum CastState
        {
            /// <summary>No cast in flight.</summary>
            Idle = 0,

            /// <summary>CAST committed; waiting for the window.</summary>
            Casting = 1,

            /// <summary>HOOK_WINDOW open (presentation only).</summary>
            HookWindow = 2,

            /// <summary>The cast resolved (catch, miss or failure).</summary>
            Resolved = 3,
        }

        /// <summary>Row of one granted item from a HOOK result.</summary>
        public readonly struct GrantedItem
        {
            public GrantedItem(string itemId, uint quantity)
            {
                ItemId = itemId;
                Quantity = quantity;
            }

            /// <summary>item_id of the grant.</summary>
            public string ItemId
            {
                get;
            }

            /// <summary>Granted quantity.</summary>
            public uint Quantity
            {
                get;
            }
        }

        /// <summary>Window timing constants (world_rules.md).</summary>
        public const float CastToWindowSeconds = 2.0f;

        /// <summary>Hook accept interval start (0.40 s).</summary>
        public const float HookAcceptMinSeconds = 0.40f;

        /// <summary>Hook accept interval end (1.20 s).</summary>
        public const float HookAcceptMaxSeconds = 1.20f;

        /// <summary>Daily success cap projected onto the counter.</summary>
        public const int DailyCatchCap = 50;

        /// <summary>item_id of the rare catch raising the peak.</summary>
        public const string RareCatchItemId =
            "item." + "material" + ".ca_chep_hoa_rong";

        private readonly List<GrantedItem> _granted =
            new List<GrantedItem>();

        /// <summary>CAST result state.</summary>
        public ActionState Cast
        {
            get;
            private set;
        }

        /// <summary>HOOK result state.</summary>
        public ActionState Hook
        {
            get;
            private set;
        }

        /// <summary>FSM of the live cast.</summary>
        public CastState CastFlow
        {
            get;
            private set;
        }

        /// <summary>error_code from the latest failed result.</summary>
        public ErrorCode FailureCode
        {
            get;
            private set;
        }

        /// <summary>Successes caught today (0..50).</summary>
        public int DailyCatchCount
        {
            get;
            private set;
        }

        /// <summary>Granted rows of the latest HOOK result.</summary>
        public IReadOnlyList<GrantedItem> Granted
        {
            get
            {
                return _granted;
            }
        }

        /// <summary>
        /// Raised once when a rare catch lands — the one PHAT_HIEN
        /// peak for the settlement (consumers clear it after showing).
        /// </summary>
        public bool RarePeak
        {
            get;
            private set;
        }

        /// <summary>
        /// Raised for spectators within 15 m on the same map_instance
        /// when a rare catch lands — the splash presentation.
        /// </summary>
        public bool SpectatorSplash
        {
            get;
            private set;
        }

        /// <summary>103 submitted: the matching slot goes pending.</summary>
        public void BeginRequest(InteractKind kind)
        {
            Set(kind, ActionState.Pending);
            FailureCode = ErrorCode.Unspecified;
        }

        /// <summary>
        /// The window hint opens 2 s after a committed cast — purely a
        /// presentation marker; the server owns the timing.
        /// </summary>
        public void OpenHookWindow()
        {
            if (CastFlow == CastState.Casting)
            {
                CastFlow = CastState.HookWindow;
            }
        }

        /// <summary>
        /// Ends the live cast — miss, disconnect, movement or map
        /// transfer (world_rules.md § Folk Fishing failure paths).
        /// </summary>
        public void EndCast()
        {
            if (CastFlow != CastState.Idle)
            {
                CastFlow = CastState.Resolved;
            }
        }

        /// <summary>Applies the recorded 116 for one fishing kind.</summary>
        public void Apply(S2CInteractResult result)
        {
            switch (result.InteractKind)
            {
                case InteractKind.Cast:
                case InteractKind.Hook:
                    break;
                default:
                    return;
            }
            if (result.Result.Status == ResultStatus.Success)
            {
                Set(result.InteractKind, ActionState.Succeeded);
                FailureCode = ErrorCode.Unspecified;
                switch (result.InteractKind)
                {
                    case InteractKind.Cast:
                        CastFlow = CastState.Casting;
                        break;
                    case InteractKind.Hook:
                        _granted.Clear();
                        foreach (ItemQuantity q in result.Granted)
                        {
                            _granted.Add(
                                new GrantedItem(q.ItemId, q.Quantity));
                            if (q.ItemId == RareCatchItemId)
                            {
                                RarePeak = true;
                            }
                        }
                        CastFlow = CastState.Resolved;
                        if (_granted.Count > 0 &&
                            DailyCatchCount < DailyCatchCap)
                        {
                            DailyCatchCount++;
                        }
                        break;
                }
            }
            else
            {
                Set(result.InteractKind, ActionState.Failed);
                FailureCode = result.Result.ErrorCode;
                CastFlow = CastState.Resolved;
            }
        }

        /// <summary>
        /// Spectator-side splash: another character landed a rare
        /// catch within 15 m on this map instance.
        /// </summary>
        public void RaiseSpectatorSplash()
        {
            SpectatorSplash = true;
        }

        /// <summary>Consumes the rare peak after presentation.</summary>
        public void ConsumeRarePeak()
        {
            RarePeak = false;
        }

        /// <summary>Consumes the spectator splash after presentation.</summary>
        public void ConsumeSpectatorSplash()
        {
            SpectatorSplash = false;
        }

        /// <summary>Clears session projection on channel/day change.</summary>
        public void ClearWorldState()
        {
            CastFlow = CastState.Idle;
            RarePeak = false;
            SpectatorSplash = false;
        }

        private void Set(InteractKind kind, ActionState state)
        {
            switch (kind)
            {
                case InteractKind.Cast:
                    Cast = state;
                    break;
                case InteractKind.Hook:
                    Hook = state;
                    break;
            }
        }
    }
}
