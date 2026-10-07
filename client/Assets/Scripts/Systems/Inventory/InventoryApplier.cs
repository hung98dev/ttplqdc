using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>
    /// `IInventorySink` applier (seam landed #194): receives the
    /// 401/419/429/432/433/435 frames the orchestrator forwards under
    /// lease and folds them into <see cref="State"/> /
    /// <see cref="Panel"/>. State messages are REPLACEABLE_STATE —
    /// each one swaps wholesale, no coalescing needed. `Version`
    /// bumps once per applied frame; the presenter applies at most
    /// once per UI phase (PERF-022).
    /// </summary>
    public sealed class InventoryApplier : IInventorySink
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SInventoryMutate = 400;
        public const uint C2SEntitlementClaim = 418;
        public const uint C2SInventoryExpand = 428;

        private readonly InventoryState _state = new InventoryState();
        private readonly EntitlementPanelState _panel =
            new EntitlementPanelState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public InventoryState State
        {
            get
            {
                return _state;
            }
        }

        public EntitlementPanelState Panel
        {
            get
            {
                return _panel;
            }
        }

        /// <summary>Latest 401 result (mutate verdict + changed set).</summary>
        public S2CInventoryResult? LastMutateResult
        {
            get;
            private set;
        }

        /// <summary>Latest 419 result (claim verdict display row).</summary>
        public S2CEntitlementClaimResult? LastClaimResult
        {
            get;
            private set;
        }

        /// <summary>Latest 429 result (capacity_after + currency_delta).</summary>
        public S2CInventoryExpandResult? LastExpandResult
        {
            get;
            private set;
        }

        public void Apply(DecodedFrame frame)
        {
            // Payload!: the decoder fills Payload for every mapped id in
            // this switch.
            switch (frame.MessageId)
            {
                case WireIds.S2CInventoryResult:
                    LastMutateResult = (S2CInventoryResult)frame.Payload!;
                    if (LastMutateResult.Result != null &&
                        LastMutateResult.Result.Status ==
                            ResultStatus.Success)
                    {
                        _state.ApplyResult(LastMutateResult);
                    }
                    break;
                case WireIds.S2CEntitlementClaimResult:
                    LastClaimResult =
                        (S2CEntitlementClaimResult)frame.Payload!;
                    _panel.ApplyClaimResult(LastClaimResult);
                    break;
                case WireIds.S2CInventoryExpandResult:
                    LastExpandResult =
                        (S2CInventoryExpandResult)frame.Payload!;
                    if (LastExpandResult.Result != null &&
                        LastExpandResult.Result.Status ==
                            ResultStatus.Success)
                    {
                        _state.ApplyExpandResult(LastExpandResult);
                    }
                    break;
                case WireIds.S2CWalletState:
                    _state.ReplaceWallet((S2CWalletState)frame.Payload!);
                    break;
                case WireIds.S2CInventoryState:
                    _state.ReplaceInventory(
                        (S2CInventoryState)frame.Payload!);
                    break;
                case WireIds.S2CEntitlementPanelState:
                    _panel.Replace(
                        (S2CEntitlementPanelState)frame.Payload!);
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
