using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.GuildStorage
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Guild `IGuildSender`): production wraps NetSession,
    /// which stamps client_seq privately.
    /// </summary>
    public interface IGuildStorageSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
