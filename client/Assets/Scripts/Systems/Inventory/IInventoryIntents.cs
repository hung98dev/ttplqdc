using System.Threading;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>
    /// Intent surface the UI calls: each request mints a fresh
    /// operation_id (ids.md UUIDv7), sends the matching C2S payload and
    /// resolves to the minted id so the caller can correlate results.
    /// Claim (418) is intent-only here — execution is IMP-102's server
    /// path; the panel displays its 419 verdict.
    /// </summary>
    public interface IInventoryIntents
    {
        Awaitable<byte[]> RequestMutate(
            InventoryOp op, byte[] itemInstanceId, uint toSlot,
            uint quantity, CancellationToken cancel);

        Awaitable<byte[]> RequestExpand(
            uint expectedCapacity, CancellationToken cancel);

        Awaitable<byte[]> RequestClaim(
            byte[] entitlementId, string rewardTierId,
            CancellationToken cancel);
    }

}
