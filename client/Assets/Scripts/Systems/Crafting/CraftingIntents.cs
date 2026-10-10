using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Crafting
{
    /// <summary>Production intents: mint + send over the bound sender.</summary>
    public sealed class CraftingIntents : ICraftingIntents
    {
        private readonly ICraftingSender _sender;
        private readonly Func<byte[]> _mint;

        public CraftingIntents(ICraftingSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> RequestCraft(string npcId,
            string recipeId, uint batchQuantity, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SCraft
            {
                OperationId = ByteString.CopyFrom(operationId),
                NpcId = npcId ?? "",
                RecipeId = recipeId ?? "",
                BatchQuantity = batchQuantity,
            };
            await _sender.SendAsync(
                CraftingApplier.C2SCraftRequest, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestEnhance(string npcId,
            byte[] itemInstanceId, uint targetLevel,
            byte[] luckyCharmItemInstanceId, byte[] insuranceItemInstanceId,
            CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SEnhance
            {
                OperationId = ByteString.CopyFrom(operationId),
                NpcId = npcId ?? "",
                ItemInstanceId = ByteString.CopyFrom(
                    itemInstanceId ?? new byte[0]),
                TargetLevel = targetLevel,
            };
            if (luckyCharmItemInstanceId != null &&
                luckyCharmItemInstanceId.Length == 16)
            {
                req.LuckyCharmItemInstanceId = ByteString.CopyFrom(
                    luckyCharmItemInstanceId);
            }
            if (insuranceItemInstanceId != null &&
                insuranceItemInstanceId.Length == 16)
            {
                req.InsuranceItemInstanceId = ByteString.CopyFrom(
                    insuranceItemInstanceId);
            }
            await _sender.SendAsync(
                CraftingApplier.C2SEnhanceRequest, req, cancel);
            return operationId;
        }

        /// <summary>
        /// UUIDv7 mint (ids.md) — same shape as Systems/Trade's
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
