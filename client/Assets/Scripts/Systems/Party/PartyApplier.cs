using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Party
{
    /// <summary>
    /// `IPartySink` applier (seam landed #225): receives the
    /// 603/607/636/653 frames the orchestrator forwards under lease and
    /// folds them into <see cref="State"/> / <see cref="Board"/> /
    /// last-result fields. `Version` bumps once per applied frame.
    /// </summary>
    public sealed class PartyApplier : IPartySink
    {
        private readonly PartyState _state = new PartyState();
        private readonly PartyBoardState _board = new PartyBoardState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public PartyState State
        {
            get
            {
                return _state;
            }
        }

        public PartyBoardState Board
        {
            get
            {
                return _board;
            }
        }

        /// <summary>Latest 603 invite delivered to this client.</summary>
        public S2CPartyInvite? LastInvite
        {
            get;
            private set;
        }

        /// <summary>Latest 653 verdict for any party request.</summary>
        public S2CPartyResult? LastResult
        {
            get;
            private set;
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case WireIds.S2CPartyInvite:
                    LastInvite = (S2CPartyInvite)frame.Payload!;
                    break;
                case WireIds.S2CPartyState:
                    _state.Replace((S2CPartyState)frame.Payload!);
                    break;
                case WireIds.S2CPartyBoardState:
                    _board.Replace((S2CPartyBoardState)frame.Payload!);
                    break;
                case WireIds.S2CPartyResult:
                    LastResult = (S2CPartyResult)frame.Payload!;
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
