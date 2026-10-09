using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Guild
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Party `IPartySender`): production wraps NetSession,
    /// which stamps client_seq privately.
    /// </summary>
    public interface IGuildSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
