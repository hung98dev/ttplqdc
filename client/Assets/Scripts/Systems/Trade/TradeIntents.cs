using System;
using System.Collections.Generic;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Trade
{
    /// <summary>Production intents: mint + send over the bound sender.</summary>
    public sealed class TradeIntents : ITradeIntents
    {
        private readonly ITradeSender _sender;
        private readonly Func<byte[]> _mint;

        public TradeIntents(ITradeSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> RequestInvite(
            byte[] targetCharacterId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2STradeInvite
            {
                OperationId = ByteString.CopyFrom(operationId),
                TargetCharacterId = ByteString.CopyFrom(
                    targetCharacterId ?? new byte[0]),
            };
            await _sender.SendAsync(
                TradeApplier.C2STradeInvite, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestAccept(byte[] tradeId,
            CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2STradeAccept
            {
                OperationId = ByteString.CopyFrom(operationId),
                TradeId = ByteString.CopyFrom(tradeId ?? new byte[0]),
            };
            await _sender.SendAsync(
                TradeApplier.C2STradeAccept, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestCancel(byte[] tradeId,
            CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2STradeCancel
            {
                OperationId = ByteString.CopyFrom(operationId),
                TradeId = ByteString.CopyFrom(tradeId ?? new byte[0]),
            };
            await _sender.SendAsync(
                TradeApplier.C2STradeCancel, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestOfferUpdate(byte[] tradeId,
            ulong expectedRevision, IReadOnlyList<TradeOfferItemModel> items,
            long commonAmount, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2STradeOfferUpdate
            {
                OperationId = ByteString.CopyFrom(operationId),
                TradeId = ByteString.CopyFrom(tradeId ?? new byte[0]),
                ExpectedRevision = expectedRevision,
                CommonAmount = commonAmount,
            };
            if (items != null)
            {
                foreach (TradeOfferItemModel m in items)
                {
                    req.Items.Add(new TradeOfferItem
                    {
                        ItemInstanceId = ByteString.CopyFrom(
                            m.ItemInstanceId ?? new byte[0]),
                        Quantity = m.Quantity,
                    });
                }
            }
            await _sender.SendAsync(
                TradeApplier.C2STradeOfferUpdate, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestConfirm(byte[] tradeId,
            ulong expectedRevision, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2STradeConfirm
            {
                OperationId = ByteString.CopyFrom(operationId),
                TradeId = ByteString.CopyFrom(tradeId ?? new byte[0]),
                ExpectedRevision = expectedRevision,
            };
            await _sender.SendAsync(
                TradeApplier.C2STradeConfirm, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestFinalise(byte[] tradeId,
            ulong expectedRevision, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2STradeFinalise
            {
                OperationId = ByteString.CopyFrom(operationId),
                TradeId = ByteString.CopyFrom(tradeId ?? new byte[0]),
                ExpectedRevision = expectedRevision,
            };
            await _sender.SendAsync(
                TradeApplier.C2STradeFinalise, req, cancel);
            return operationId;
        }

        /// <summary>
        /// UUIDv7 mint (ids.md) — same shape as Systems/Auction's
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
