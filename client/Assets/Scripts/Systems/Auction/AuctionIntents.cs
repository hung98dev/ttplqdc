using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Auction
{
    /// <summary>Production intents: mint + send over the bound sender.</summary>
    public sealed class AuctionIntents : IAuctionIntents
    {
        private readonly IAuctionSender _sender;
        private readonly Func<byte[]> _mint;

        public AuctionIntents(IAuctionSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> RequestList(byte[] itemInstanceId,
            uint quantity, long priceCommon, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SAuctionList
            {
                OperationId = ByteString.CopyFrom(operationId),
                ItemInstanceId = ByteString.CopyFrom(
                    itemInstanceId ?? new byte[0]),
                Quantity = quantity,
                PriceCommon = priceCommon,
            };
            await _sender.SendAsync(
                AuctionApplier.C2SAuctionList, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestBuy(byte[] listingId,
            long expectedPriceCommon, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SAuctionBuy
            {
                OperationId = ByteString.CopyFrom(operationId),
                ListingId = ByteString.CopyFrom(listingId ?? new byte[0]),
                ExpectedPriceCommon = expectedPriceCommon,
            };
            await _sender.SendAsync(
                AuctionApplier.C2SAuctionBuy, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestCancelListing(
            byte[] listingId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SAuctionCancelListing
            {
                OperationId = ByteString.CopyFrom(operationId),
                ListingId = ByteString.CopyFrom(listingId ?? new byte[0]),
            };
            await _sender.SendAsync(
                AuctionApplier.C2SAuctionCancelListing, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestSearch(string itemId,
            string category, uint tier, long minPrice, long maxPrice,
            AuctionSort sort, string pageCursor, uint pageSize,
            CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SAuctionSearch
            {
                OperationId = ByteString.CopyFrom(operationId),
                ItemId = itemId ?? "",
                Category = category ?? "",
                Tier = tier,
                MinPrice = minPrice,
                MaxPrice = maxPrice,
                Sort = sort,
                PageCursor = pageCursor ?? "",
                PageSize = pageSize,
            };
            await _sender.SendAsync(
                AuctionApplier.C2SAuctionSearch, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestReclaim(
            byte[] escrowAssetId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SAuctionReclaim
            {
                OperationId = ByteString.CopyFrom(operationId),
                EscrowAssetId = ByteString.CopyFrom(
                    escrowAssetId ?? new byte[0]),
            };
            await _sender.SendAsync(
                AuctionApplier.C2SAuctionReclaim, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestProceedsClaim(
            byte[] proceedsId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SAuctionProceedsClaim
            {
                OperationId = ByteString.CopyFrom(operationId),
                ProceedsId = ByteString.CopyFrom(
                    proceedsId ?? new byte[0]),
            };
            await _sender.SendAsync(
                AuctionApplier.C2SAuctionProceedsClaim, req, cancel);
            return operationId;
        }

        /// <summary>
        /// UUIDv7 mint (ids.md) — same shape as Systems/Inventory's
        /// minter (Systems may not reference UI or other Systems).
        /// </summary>
        public static byte[] MintOperationId()
        {
            long unixMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
            var bytes = new byte[16];
            bytes[0] = (byte)(unixMs >> 40);
            bytes[1] = (byte)(unixMs >> 32);
            bytes[2] = (byte)(unixMs >> 24);
            bytes[3] = (byte)(unixMs >> 16);
            bytes[4] = (byte)(unixMs >> 8);
            bytes[5] = (byte)unixMs;
            byte[] tail = Guid.NewGuid().ToByteArray();
            for (int i = 6; i < 16; i++)
            {
                bytes[i] = tail[i - 6];
            }
            bytes[6] = (byte)(bytes[6] & 0x0F | 0x70);
            bytes[8] = (byte)(bytes[8] & 0x3F | 0x80);
            return bytes;
        }
    }
}
