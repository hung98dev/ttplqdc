using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>Production intents: mint + send over the bound sender.</summary>
    public sealed class InventoryIntents : IInventoryIntents
    {
        private readonly IInventorySender _sender;
        private readonly Func<byte[]> _mint;

        public InventoryIntents(IInventorySender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> RequestMutate(
            InventoryOp op, byte[] itemInstanceId, uint toSlot,
            uint quantity, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SInventoryMutate
            {
                OperationId = ByteString.CopyFrom(operationId),
                Op = op,
                ToSlot = toSlot,
                Quantity = quantity,
            };
            if (itemInstanceId != null)
            {
                req.ItemInstanceId = ByteString.CopyFrom(itemInstanceId);
            }
            await _sender.SendAsync(
                InventoryApplier.C2SInventoryMutate, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestExpand(
            uint expectedCapacity, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SInventoryExpand
            {
                OperationId = ByteString.CopyFrom(operationId),
                ExpectedCapacity = expectedCapacity,
            };
            await _sender.SendAsync(
                InventoryApplier.C2SInventoryExpand, req, cancel);
            return operationId;
        }

        public async Awaitable<byte[]> RequestClaim(
            byte[] entitlementId, string rewardTierId,
            CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SEntitlementClaim
            {
                OperationId = ByteString.CopyFrom(operationId),
                EntitlementId = ByteString.CopyFrom(
                    entitlementId ?? new byte[0]),
                RewardTierId = rewardTierId ?? "",
            };
            await _sender.SendAsync(
                InventoryApplier.C2SEntitlementClaim, req, cancel);
            return operationId;
        }

        /// <summary>
        /// UUIDv7 mint (ids.md): 48-bit unix millis, version 7,
        /// process-counter mid section, random tail — same shape
        /// UI/StateMachine's OperationIdMinter produces on its own
        /// assembly (kept local: Systems may not reference UI).
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
