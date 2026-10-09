using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.LifeSkills.Cooking
{
    /// <summary>
    /// Cooking/bonfire presentation state driven by recorded
    /// S2C_INTERACT_RESULT (116) frames: each interact kind has an
    /// idle/pending/succeeded/failed slot; KINDLE success marks the
    /// channel bonfire active; COOK success records the granted rows
    /// (food + kindling); BONFIRE_REST toggles the local rest session —
    /// it also ends on a movement intent or leaving the map, which the
    /// composition hooks call through <see cref="EndRest"/>.
    /// </summary>
    public sealed class CookingPresentation
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

        /// <summary>Row of one granted item from a COOK result.</summary>
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

        private readonly List<GrantedItem> _granted = new List<GrantedItem>();

        /// <summary>KINDLE result state.</summary>
        public ActionState Kindle
        {
            get;
            private set;
        }

        /// <summary>COOK result state.</summary>
        public ActionState Cook
        {
            get;
            private set;
        }

        /// <summary>BONFIRE_REST result state.</summary>
        public ActionState Rest
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

        /// <summary>Channel bonfire live flag (KINDLE success).</summary>
        public bool BonfireActive
        {
            get;
            private set;
        }

        /// <summary>Local rest-session flag (BONFIRE_REST toggles).</summary>
        public bool Resting
        {
            get;
            private set;
        }

        /// <summary>Granted rows of the latest COOK result.</summary>
        public IReadOnlyList<GrantedItem> Granted
        {
            get
            {
                return _granted;
            }
        }

        /// <summary>103 submitted: the matching slot goes pending.</summary>
        public void BeginRequest(InteractKind kind)
        {
            Set(kind, ActionState.Pending);
            FailureCode = ErrorCode.Unspecified;
        }

        /// <summary>Applies the recorded 116 for one cooking kind.</summary>
        public void Apply(S2CInteractResult result)
        {
            switch (result.InteractKind)
            {
                case InteractKind.Kindle:
                case InteractKind.Cook:
                case InteractKind.BonfireRest:
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
                    case InteractKind.Kindle:
                        BonfireActive = true;
                        break;
                    case InteractKind.Cook:
                        _granted.Clear();
                        foreach (ItemQuantity q in result.Granted)
                        {
                            _granted.Add(new GrantedItem(q.ItemId, q.Quantity));
                        }
                        break;
                    case InteractKind.BonfireRest:
                        Resting = !Resting;
                        break;
                }
            }
            else
            {
                Set(result.InteractKind, ActionState.Failed);
                FailureCode = result.Result.ErrorCode;
            }
        }

        /// <summary>
        /// Ends the local rest session without a wire result: movement,
        /// combat, map transfer or disconnect (world_rules.md § Rest
        /// ends). Composition hooks the movement/combat surfaces here.
        /// </summary>
        public void EndRest()
        {
            Resting = false;
        }

        /// <summary>Clears bonfire/rest presentation on channel change.</summary>
        public void ClearWorldState()
        {
            BonfireActive = false;
            Resting = false;
        }

        private void Set(InteractKind kind, ActionState state)
        {
            switch (kind)
            {
                case InteractKind.Kindle:
                    Kindle = state;
                    break;
                case InteractKind.Cook:
                    Cook = state;
                    break;
                case InteractKind.BonfireRest:
                    Rest = state;
                    break;
            }
        }
    }
}
