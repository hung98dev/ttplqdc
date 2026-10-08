using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Auction
{
    /// <summary>
    /// `IAuctionSink` applier (seam landed #225): receives the
    /// 731/733/735/736/739/741/743/744 frames the orchestrator forwards
    /// under lease and folds them into <see cref="State"/>. State
    /// messages replace wholesale; `Version` bumps once per applied
    /// frame (PERF-022 dirty flag).
    /// </summary>
    public sealed class AuctionApplier : IAuctionSink
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SAuctionList = 730;
        public const uint C2SAuctionBuy = 732;
        public const uint C2SAuctionCancelListing = 734;
        public const uint C2SAuctionSearch = 738;
        public const uint C2SAuctionReclaim = 740;
        public const uint C2SAuctionProceedsClaim = 742;

        private readonly AuctionState _state = new AuctionState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public AuctionState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Latest 731 result (list verdict).</summary>
        public S2CAuctionListResult? LastListResult
        {
            get;
            private set;
        }

        /// <summary>Latest 733 result (buy verdict).</summary>
        public S2CAuctionBuyResult? LastBuyResult
        {
            get;
            private set;
        }

        /// <summary>Latest 735 result (cancel verdict).</summary>
        public S2CAuctionCancelResult? LastCancelResult
        {
            get;
            private set;
        }

        /// <summary>Latest 741 result (reclaim verdict).</summary>
        public S2CAuctionReclaimResult? LastReclaimResult
        {
            get;
            private set;
        }

        /// <summary>Latest 743 result (proceeds verdict).</summary>
        public S2CAuctionProceedsResult? LastProceedsResult
        {
            get;
            private set;
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case WireIds.S2CAuctionListResult:
                    LastListResult =
                        (S2CAuctionListResult)frame.Payload!;
                    break;
                case WireIds.S2CAuctionBuyResult:
                    LastBuyResult = (S2CAuctionBuyResult)frame.Payload!;
                    break;
                case WireIds.S2CAuctionCancelResult:
                    LastCancelResult =
                        (S2CAuctionCancelResult)frame.Payload!;
                    break;
                case WireIds.S2CAuctionSold:
                    _state.ApplySold((S2CAuctionSold)frame.Payload!);
                    break;
                case WireIds.S2CAuctionSearchResult:
                    _state.ReplaceSearch(
                        (S2CAuctionSearchResult)frame.Payload!);
                    break;
                case WireIds.S2CAuctionReclaimResult:
                    LastReclaimResult =
                        (S2CAuctionReclaimResult)frame.Payload!;
                    break;
                case WireIds.S2CAuctionProceedsResult:
                    LastProceedsResult =
                        (S2CAuctionProceedsResult)frame.Payload!;
                    break;
                case WireIds.S2CAuctionMyState:
                    _state.Replace((S2CAuctionMyState)frame.Payload!);
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
