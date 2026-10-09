using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Atlas
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Rewards `IRewardClaimSender`): production wraps
    /// NetSession, which stamps client_seq privately.
    /// </summary>
    public interface IAtlasSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
