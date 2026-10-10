using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.GuildStorage
{
    /// <summary>
    /// Guild-storage frame consumer (same shape as Systems/Guild
    /// `GuildApplier`): the session driver forwards 631/647/649 under
    /// lease and the applier folds them into <see cref="State"/>.
    /// `Version` bumps once per applied frame.
    /// </summary>
    public sealed class GuildStorageApplier
    {
        /// <summary>Inbound message ids this applier handles.</summary>
        public const uint S2CGuildStorageState = 631;
        public const uint S2CGuildStorageClaims = 647;
        public const uint S2CGuildResult = 649;

        private readonly GuildStorageState _state = new GuildStorageState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public GuildStorageState State
        {
            get
            {
                return _state;
            }
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
                case S2CGuildStorageState:
                    _state.ReplaceItems((S2CGuildStorageState)frame.Payload!);
                    break;
                case S2CGuildStorageClaims:
                    _state.ReplaceClaims((S2CGuildStorageClaims)frame.Payload!);
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
