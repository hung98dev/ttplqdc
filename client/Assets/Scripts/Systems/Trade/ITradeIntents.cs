using System.Collections.Generic;
using System.Threading;
using UnityEngine;

namespace ThinhThan.Systems.Trade
{
    /// <summary>Client intents for the direct-trade C2S set (700-708).</summary>
    public interface ITradeIntents
    {
        Awaitable<byte[]> RequestInvite(byte[] targetCharacterId,
            CancellationToken cancel);
        Awaitable<byte[]> RequestAccept(byte[] tradeId,
            CancellationToken cancel);
        Awaitable<byte[]> RequestCancel(byte[] tradeId,
            CancellationToken cancel);
        Awaitable<byte[]> RequestOfferUpdate(byte[] tradeId,
            ulong expectedRevision, IReadOnlyList<TradeOfferItemModel> items,
            long commonAmount, CancellationToken cancel);
        Awaitable<byte[]> RequestConfirm(byte[] tradeId,
            ulong expectedRevision, CancellationToken cancel);
        Awaitable<byte[]> RequestFinalise(byte[] tradeId,
            ulong expectedRevision, CancellationToken cancel);
    }

}
