using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.LifeSkills.Cooking
{
    /// <summary>
    /// Hearth/bonfire intents: each request mints a fresh operation_id
    /// (ids.md UUIDv7), sends the matching C2S_INTERACT (103) with the
    /// interact_kind of the action and resolves to the minted id so the
    /// caller can correlate the recorded 116 result.
    /// </summary>
    public sealed class CookingIntents
    {
        /// <summary>C2S_INTERACT.</summary>
        public const uint MessageC2SInteract = 103;

        /// <summary>Bonfire anchor target id segment used at admission.</summary>
        public const string BonfireTargetId = "bonfire";

        private readonly ICookingSender _sender;
        private readonly Func<byte[]> _mint;

        public CookingIntents(ICookingSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        /// <summary>103 KINDLE — one cui_lua_trai at the bonfire.</summary>
        public async Awaitable<byte[]> RequestKindle(
            string targetId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SInteract
            {
                InteractKind = InteractKind.Kindle,
                TargetId = targetId,
                OperationId = ByteString.CopyFrom(operationId),
            };
            await _sender.SendAsync(MessageC2SInteract, req, cancel);
            return operationId;
        }

        /// <summary>103 COOK — one hearth recipe act at the hearth.</summary>
        public async Awaitable<byte[]> RequestCook(
            string targetId, string recipeId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SInteract
            {
                InteractKind = InteractKind.Cook,
                TargetId = targetId,
                OperationId = ByteString.CopyFrom(operationId),
                RecipeId = recipeId,
            };
            await _sender.SendAsync(MessageC2SInteract, req, cancel);
            return operationId;
        }

        /// <summary>103 BONFIRE_REST — start rest / stand up toggle.</summary>
        public async Awaitable<byte[]> RequestBonfireRest(
            string targetId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SInteract
            {
                InteractKind = InteractKind.BonfireRest,
                TargetId = targetId,
                OperationId = ByteString.CopyFrom(operationId),
            };
            await _sender.SendAsync(MessageC2SInteract, req, cancel);
            return operationId;
        }

        /// <summary>
        /// UUIDv7 mint (ids.md): 48-bit unix millis, version 7,
        /// process-counter mid section, random tail — same shape
        /// Systems/Inventory produces on its own assembly.
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
