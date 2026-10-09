using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Party
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Rewards `IRewardClaimSender`): production wraps
    /// NetSession, which stamps client_seq privately.
    /// </summary>
    public interface IPartySender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
