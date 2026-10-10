using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Cosmetics
{
    /// <summary>Production intents: mint + send over the bound sender.</summary>
    public sealed class CosmeticsIntents : ICosmeticsIntents
    {
        private readonly ICosmeticsSender _sender;
        private readonly Func<byte[]> _mint;

        public CosmeticsIntents(ICosmeticsSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> RequestRedeem(string cosmeticId,
            CosmeticRoute route, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SCosmeticRedeem
            {
                OperationId = ByteString.CopyFrom(operationId),
                CosmeticId = cosmeticId ?? "",
                Route = route,
            };
            await _sender.SendAsync(
                CosmeticsApplier.C2SCosmeticRedeem, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestEquip(CosmeticSlot slot,
            string cosmeticId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SCosmeticEquip
            {
                OperationId = ByteString.CopyFrom(operationId),
                Slot = slot,
                CosmeticId = cosmeticId ?? "",
            };
            await _sender.SendAsync(
                CosmeticsApplier.C2SCosmeticEquip, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestGuildEquip(byte[] guildId,
            GuildCosmeticSlot slot, string cosmeticId,
            ulong expectedRevision, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SGuildCosmeticEquip
            {
                OperationId = ByteString.CopyFrom(operationId),
                GuildId = ByteString.CopyFrom(guildId ?? new byte[0]),
                Slot = slot,
                CosmeticId = cosmeticId ?? "",
                ExpectedRevision = expectedRevision,
            };
            await _sender.SendAsync(
                CosmeticsApplier.C2SGuildCosmeticEquip, req, cancel);
            return operationId;
        }

        /// <summary>UUIDv7 mint for operation ids.</summary>
        public static byte[] MintOperationId()
        {
            byte[] b = Guid.NewGuid().ToByteArray();
            long ms = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
            b[0] = (byte)(ms >> 40);
            b[1] = (byte)(ms >> 32);
            b[2] = (byte)(ms >> 24);
            b[3] = (byte)(ms >> 16);
            b[4] = (byte)(ms >> 8);
            b[5] = (byte)ms;
            b[6] = (byte)((b[6] & 0x0F) | 0x70);
            b[8] = (byte)((b[8] & 0x3F) | 0x80);
            return b;
        }
    }
}
