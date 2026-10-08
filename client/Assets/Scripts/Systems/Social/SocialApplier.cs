using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Social
{
    /// <summary>
    /// `ISocialSink` applier: folds the social frames the orchestrator
    /// forwards under lease (601, 612, 616, 619, 633, 654, 655) into
    /// <see cref="State"/>. `Version` bumps once per applied frame
    /// (PERF-022 dirty flag).
    /// </summary>
    public sealed class SocialApplier : ISocialSink
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SChatSendId = 600;
        public const uint C2SFriendRequestId = 611;
        public const uint C2SFriendAcceptId = 613;
        public const uint C2SFriendDeclineId = 614;
        public const uint C2SFriendRemoveId = 615;
        public const uint C2SBlockAddId = 617;
        public const uint C2SBlockRemoveId = 618;
        public const uint C2SReportPlayerId = 632;

        /// <summary>S2C ids consumed by Apply.</summary>
        public const uint S2CChatMessageId = 601;
        public const uint S2CFriendRequestId = 612;
        public const uint S2CFriendStateId = 616;
        public const uint S2CBlockStateId = 619;
        public const uint S2CReportPlayerResultId = 633;
        public const uint S2CSocialResultId = 654;
        public const uint S2CChatSendResultId = 655;

        private readonly SocialState _state = new SocialState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public SocialState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Route one decoded frame into the projection.</summary>
        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case S2CChatMessageId:
                    _state.AppendChat(frame.Payload as S2CChatMessage);
                    break;
                case S2CFriendRequestId:
                    _state.ApplyRequestPush(
                        frame.Payload as S2CFriendRequest);
                    break;
                case S2CFriendStateId:
                    _state.ApplyFriendState(
                        frame.Payload as S2CFriendState);
                    break;
                case S2CBlockStateId:
                    _state.ApplyBlockState(
                        frame.Payload as S2CBlockState);
                    break;
                case S2CReportPlayerResultId:
                    _state.LastReportResult =
                        frame.Payload as S2CReportPlayerResult;
                    break;
                case S2CSocialResultId:
                    _state.LastSocialResult =
                        frame.Payload as S2CSocialResult;
                    break;
                case S2CChatSendResultId:
                    _state.LastChatResult =
                        frame.Payload as S2CChatSendResult;
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
