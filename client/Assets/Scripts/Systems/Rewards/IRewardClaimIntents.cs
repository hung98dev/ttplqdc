using System.Threading;
using UnityEngine;

namespace ThinhThan.Systems.Rewards
{
    /// <summary>Outbound reward-claim intents (408 claim, 439 page).</summary>
    public interface IRewardClaimIntents
    {
        /// <summary>Claims one pending reward claim (408).</summary>
        Awaitable<byte[]> RequestClaim(
            byte[] rewardClaimId, CancellationToken cancel);

        /// <summary>Requests one 50-row claims page (439).</summary>
        Awaitable<byte[]> RequestList(
            uint offset, uint limit, CancellationToken cancel);
    }
}
