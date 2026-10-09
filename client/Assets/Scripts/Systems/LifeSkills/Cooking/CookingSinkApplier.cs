using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.LifeSkills.Cooking
{
    /// <summary>
    /// The <see cref="ICookingSink"/> the session driver forwards
    /// interact-result frames to (116): routes KINDLE / COOK /
    /// BONFIRE_REST results into <see cref="CookingPresentation"/> and
    /// ignores every other interact kind (116 is shared with World and
    /// Discovery sinks).
    /// </summary>
    public sealed class CookingSinkApplier : ICookingSink
    {
        /// <summary>S2C_INTERACT_RESULT.</summary>
        public const uint MessageInteractResult = 116;

        private readonly CookingPresentation _presentation;

        public CookingSinkApplier(CookingPresentation presentation)
        {
            _presentation = presentation;
        }

        /// <summary>Dispatches a decoded frame to the cooking model.</summary>
        public void Apply(DecodedFrame frame)
        {
            if (frame?.Payload == null)
            {
                return;
            }
            if (frame.MessageId != MessageInteractResult)
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
