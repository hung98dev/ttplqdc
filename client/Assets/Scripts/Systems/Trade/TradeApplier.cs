using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Trade
{
    /// <summary>
    /// `ITradeSink` applier: receives the 701/704/706/709/710 frames
    /// the orchestrator forwards under lease and folds them into
    /// <see cref="State"/>. `Version` bumps once per applied frame
    /// (PERF-022 dirty flag).
    /// </summary>
    public sealed class TradeApplier : ITradeSink
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2STradeInvite = 700;
        public const uint C2STradeAccept = 702;
        public const uint C2STradeCancel = 703;
        public const uint C2STradeOfferUpdate = 705;
        public const uint C2STradeConfirm = 707;
        public const uint C2STradeFinalise = 708;

        private readonly TradeState _state = new TradeState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public TradeState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Latest 710 request verdict.</summary>
        public S2CTradeRequestResult? LastRequestResult
        {
            get;
            private set;
        }

        /// <summary>
        /// Own character id — must be set for own/partner side
        /// resolution in the 706 projection.
        /// </summary>
        public void SetLocalCharacterId(byte[] characterId)
        {
            _state.LocalCharacterId = characterId ?? new byte[0];
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case WireIds.S2CTradeInvite:
                    _state.ApplyInvite((S2CTradeInvite)frame.Payload!);
                    break;
                case WireIds.S2CTradeCancelled:
                    _state.ApplyCancelled(
                        (S2CTradeCancelled)frame.Payload!);
                    break;
                case WireIds.S2CTradeOfferState:
                    _state.ReplaceOfferState(
                        (S2CTradeOfferState)frame.Payload!);
                    break;
                case WireIds.S2CTradeResult:
                    _state.ApplyResult((S2CTradeResult)frame.Payload!);
                    break;
                case WireIds.S2CTradeRequestResult:
                    LastRequestResult =
                        (S2CTradeRequestResult)frame.Payload!;
                    _state.LastRequestResult = LastRequestResult;
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
