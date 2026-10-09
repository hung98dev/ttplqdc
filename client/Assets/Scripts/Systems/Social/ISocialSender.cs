using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Social
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Trade `ITradeSender`): production wraps NetSession,
    /// which stamps client_seq privately.
    /// </summary>
    public interface ISocialSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
