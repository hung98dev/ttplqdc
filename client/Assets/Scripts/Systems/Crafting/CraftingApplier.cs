using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Crafting
{
    /// <summary>
    /// `ICraftingSink` applier: receives the 405/407 frames the
    /// orchestrator forwards under lease and folds them into
    /// <see cref="State"/>. `Version` bumps once per applied frame
    /// (PERF-022 dirty flag).
    /// </summary>
    public sealed class CraftingApplier : ICraftingSink
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SCraftRequest = 404;
        public const uint C2SEnhanceRequest = 406;

        private readonly CraftingState _state = new CraftingState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public CraftingState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Latest 405 verdict.</summary>
        public S2CCraftResult? LastCraftResult
        {
            get
            {
                return _state.LastCraftResult;
            }
        }

        /// <summary>Latest 407 verdict.</summary>
        public S2CEnhanceResult? LastEnhanceResult
        {
            get
            {
                return _state.LastEnhanceResult;
            }
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case WireIds.S2CCraftResult:
                    _state.ApplyCraft((S2CCraftResult)frame.Payload!);
                    break;
                case WireIds.S2CEnhanceResult:
                    _state.ApplyEnhance((S2CEnhanceResult)frame.Payload!);
                    break;
                default:
                    return;
            }
            Version++;
        }
    }
}
