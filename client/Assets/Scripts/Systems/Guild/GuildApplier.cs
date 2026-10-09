using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Guild
{
    /// <summary>
    /// Guild frame consumer (same shape as Systems/Party
    /// `PartyApplier`): the session driver forwards 609/628/641/649
    /// under lease and the applier folds them into <see cref="State"/>,
    /// pending invites, applications, and the last verdict.
    /// `Version` bumps once per applied frame.
    /// </summary>
    public sealed class GuildApplier
    {
        /// <summary>Inbound message ids this applier handles.</summary>
        public const uint S2CGuildInvite = 609;
        public const uint S2CGuildState = 628;
        public const uint S2CGuildApplications = 641;
        public const uint S2CGuildResult = 649;

        private readonly GuildState _state = new GuildState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public GuildState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Latest 609 invite delivered to this client.</summary>
        public S2CGuildInvite? LastInvite
        {
            get;
            private set;
        }

        /// <summary>Latest 641 applications list (officer+ surface).</summary>
        public S2CGuildApplications? LastApplications
        {
            get;
            private set;
        }

        /// <summary>Latest 649 verdict for any guild request.</summary>
        public S2CGuildResult? LastResult
        {
            get;
            private set;
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case S2CGuildInvite:
                    LastInvite = (S2CGuildInvite)frame.Payload!;
                    break;
                case S2CGuildState:
                    _state.Replace((S2CGuildState)frame.Payload!);
                    break;
                case S2CGuildApplications:
                    LastApplications = (S2CGuildApplications)frame.Payload!;
                    break;
                case S2CGuildResult:
                    LastResult = (S2CGuildResult)frame.Payload!;
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
