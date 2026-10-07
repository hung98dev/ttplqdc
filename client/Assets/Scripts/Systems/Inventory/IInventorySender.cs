using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Movement `IMovementSender`): production wraps NetSession,
    /// which stamps client_seq privately.
    /// </summary>
    public interface IInventorySender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }

}
