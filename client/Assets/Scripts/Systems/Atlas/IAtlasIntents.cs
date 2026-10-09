using System.Threading;
using UnityEngine;

namespace ThinhThan.Systems.Atlas
{
    /// <summary>Outbound Atlas intents the UI invokes.</summary>
    public interface IAtlasIntents
    {
        /// <summary>
        /// Sends C2S_ATLAS_CLAIM (504) acknowledging one reached tier.
        /// Returns the minted operation id.
        /// </summary>
        Awaitable<byte[]> Acknowledge(
            string atlasPageId, uint tier, CancellationToken cancel);
    }
}
