using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Equipment
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Inventory `IInventorySender`): production wraps
    /// NetSession, which stamps client_seq privately.
    /// </summary>
    public interface IEquipmentSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }

}
