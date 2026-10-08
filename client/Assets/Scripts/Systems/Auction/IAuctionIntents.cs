using System.Threading;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Auction
{
    /// <summary>Client intents for the auction C2S set (messages.md).</summary>
    public interface IAuctionIntents
    {
        Awaitable<byte[]> RequestList(byte[] itemInstanceId, uint quantity,
            long priceCommon, CancellationToken cancel);
        Awaitable<byte[]> RequestBuy(byte[] listingId,
            long expectedPriceCommon, CancellationToken cancel);
        Awaitable<byte[]> RequestCancelListing(byte[] listingId,
            CancellationToken cancel);
        Awaitable<byte[]> RequestSearch(string itemId, string category,
            uint tier, long minPrice, long maxPrice, AuctionSort sort,
            string pageCursor, uint pageSize, CancellationToken cancel);
        Awaitable<byte[]> RequestReclaim(byte[] escrowAssetId,
            CancellationToken cancel);
        Awaitable<byte[]> RequestProceedsClaim(byte[] proceedsId,
            CancellationToken cancel);
    }

}
