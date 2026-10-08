using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Rewards
{
    /// <summary>
    /// `IRewardClaimSink` applier (seam landed #225): receives the
    /// 409/434/440/441 frames the orchestrator forwards under lease
    /// and folds them into <see cref="State"/> / last-result fields.
    /// `Version` bumps once per applied frame; the presenter applies
    /// at most once per UI phase (PERF-022).
    /// </summary>
    public sealed class RewardClaimApplier : IRewardClaimSink
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SRewardClaim = 408;
        public const uint C2SRewardClaimListRequest = 439;

        private readonly RewardClaimsState _state =
            new RewardClaimsState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public RewardClaimsState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Latest 409 claim verdict (+ granted lines).</summary>
        public S2CRewardClaimResult? LastClaimResult
        {
            get;
            private set;
        }

        /// <summary>Latest 440 list page (paging display).</summary>
        public S2CRewardClaimListResult? LastListResult
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
                case WireIds.S2CRewardClaimResult:
                    LastClaimResult =
                        (S2CRewardClaimResult)frame.Payload!;
                    break;
                case WireIds.S2CRewardClaimsState:
                    _state.Replace((S2CRewardClaimsState)frame.Payload!);
                    break;
                case WireIds.S2CRewardClaimListResult:
                    LastListResult =
                        (S2CRewardClaimListResult)frame.Payload!;
                    break;
                case WireIds.S2CRewardClaimDelta:
                    _state.ApplyDelta(
                        (S2CRewardClaimDelta)frame.Payload!);
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
