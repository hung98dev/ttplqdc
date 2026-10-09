using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.GuildStorage
{
    /// <summary>
    /// Guild-storage intent surface the UI calls (send-only).
    /// </summary>
    public interface IGuildStorageIntents
    {
        Awaitable<ulong> RequestDeposit(
            ByteString itemInstanceId, uint quantity,
            GuildStorageSection section, CancellationToken cancel);

        Awaitable<ulong> RequestWithdraw(
            ByteString itemInstanceId, uint quantity,
            GuildStorageSection section, CancellationToken cancel);

        Awaitable<ulong> RequestMove(
            ByteString itemInstanceId, GuildStorageSection toSection,
            CancellationToken cancel);

        Awaitable<ulong> RequestClaim(
            ByteString itemInstanceId, uint quantity, CancellationToken cancel);

        Awaitable<ulong> RequestClaimDecide(
            ByteString claimId, GuildStorageClaimDecision decision,
            CancellationToken cancel);
    }
}
