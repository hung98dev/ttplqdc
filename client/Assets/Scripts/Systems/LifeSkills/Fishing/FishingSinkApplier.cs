using System;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.LifeSkills.Fishing
{
    /// <summary>
    /// Sink-side applier on the shared S2C_INTERACT_RESULT (116)
    /// frame: forwards CAST/HOOK results to
    /// <see cref="FishingPresentation"/> and ignores every other kind
    /// (the frame multiplexes all interact kinds).
    /// </summary>
    public sealed class FishingSinkApplier
    {
        /// <summary>S2C_INTERACT_RESULT.</summary>
        public const uint MessageResult = 116;

        private readonly FishingPresentation _presentation;

        public FishingSinkApplier(FishingPresentation presentation)
        {
            _presentation = presentation ??
                throw new ArgumentNullException(nameof(presentation));
        }

        /// <summary>Routes one decoded frame; foreign kinds are dropped.</summary>
        public void Apply(DecodedFrame frame)
        {
            if (frame.MessageId != MessageResult)
            {
                return;
            }
            if (frame.Payload is S2CInteractResult result)
            {
                _presentation.Apply(result);
            }
        }
    }
}
