using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.LifeSkills.Fishing
{
    /// <summary>
    /// Fishing intents (world_rules.md § Folk Fishing): each request
    /// mints a fresh operation_id (ids.md UUIDv7), sends the matching
    /// C2S_INTERACT (103) with CAST or HOOK and resolves to the minted
    /// id so the caller can correlate the recorded 116 result. The
    /// server owns every gate — rod/bait/range/cap, window timing and
    /// the keyed roll; the client only sends intent.
    /// </summary>
    public sealed class FishingIntents
    {
        /// <summary>C2S_INTERACT.</summary>
        public const uint MessageC2SInteract = 103;

        private readonly IFishingSender _sender;
        private readonly Func<byte[]> _mint;

        public FishingIntents(IFishingSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        /// <summary>103 CAST — cast the rod at the fishing spot.</summary>
        public async Awaitable<byte[]> RequestCast(
            string targetId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SInteract
            {
                InteractKind = InteractKind.Cast,
                TargetId = targetId,
                OperationId = ByteString.CopyFrom(operationId),
            };
            await _sender.SendAsync(MessageC2SInteract, req, cancel);
            return operationId;
        }

        /// <summary>103 HOOK — the one hook inside the open window.</summary>
        public async Awaitable<byte[]> RequestHook(
            string targetId, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SInteract
            {
                InteractKind = InteractKind.Hook,
                TargetId = targetId,
                OperationId = ByteString.CopyFrom(operationId),
            };
            await _sender.SendAsync(MessageC2SInteract, req, cancel);
            return operationId;
        }

        /// <summary>
        /// UUIDv7 mint (ids.md): 48-bit unix millis, version 7,
        /// process-counter mid section, random tail — same shape
        /// Systems/LifeSkills/Cooking produces on its own assembly.
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
