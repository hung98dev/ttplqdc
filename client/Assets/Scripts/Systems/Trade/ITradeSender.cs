using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Trade
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/Auction `IAuctionSender`): production wraps NetSession,
    /// which stamps client_seq privately.
    /// </summary>
    public interface ITradeSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }

}
