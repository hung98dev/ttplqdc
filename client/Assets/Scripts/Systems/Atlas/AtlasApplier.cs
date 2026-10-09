using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Atlas
{
    /// <summary>
    /// `IAtlasSink` applier: folds the 518 self-snapshot, the 505 claim
    /// verdict and 506 ATLAS_TIER progression events into
    /// <see cref="State"/>. `Version` bumps once per applied frame; the
    /// presenter applies at most once per UI phase (PERF-022).
    /// </summary>
    public sealed class AtlasApplier : IAtlasSink
    {
        /// <summary>Wire ids this system consumes (messages.md §7).</summary>
        public const uint C2SAtlasClaim = 504;
        public const uint S2CAtlasClaimResultId = 505;
        public const uint S2CProgressionEventId = 506;
        public const uint S2CAtlasStateId = 518;

        private readonly AtlasState _state = new AtlasState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public AtlasState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Latest 505 acknowledge verdict.</summary>
        public S2CAtlasClaimResult? LastClaimResult
        {
            get;
            private set;
        }

        /// <summary>Latest 506 ATLAS_TIER progression event.</summary>
        public S2CProgressionEvent? LastAtlasProgression
        {
            get;
            private set;
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case S2CAtlasStateId:
                    _state.ApplySnapshot(
                        (S2CAtlasState)frame.Payload!);
                    break;
                case S2CAtlasClaimResultId:
                    var res = (S2CAtlasClaimResult)frame.Payload!;
                    LastClaimResult = res;
                    if (res.Result != null && res.Result.Status ==
                        ResultStatus.Success)
                    {
                        _state.MarkAcknowledged(
                            res.AtlasPageId, res.Tier);
                    }
                    break;
                case S2CProgressionEventId:
                    var ev = (S2CProgressionEvent)frame.Payload!;
                    if (ev.EventKind ==
                        ProgressionEventKind.AtlasTier)
                    {
                        LastAtlasProgression = ev;
                    }
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
