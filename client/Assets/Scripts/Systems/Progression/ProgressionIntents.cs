using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Movement;
using UnityEngine;

namespace ThinhThan.Systems.Progression
{
    /// <summary>
    /// <see cref="IProgressionIntents"/> over the shared send seam —
    /// operation_id is minted per send (UUIDv7, ids.md) so a replayed
    /// submit stays idempotent server-side. Tests inject a deterministic
    /// minter to correlate the 514 verdict.
    /// </summary>
    public sealed class ProgressionIntents : IProgressionIntents
    {
        private readonly IMovementSender _sender;
        private readonly Func<byte[]> _mint;

        /// <param name="mintOperationId">Defaults to the shared UUIDv7
        /// shape (time millis + seed + random tail) when omitted.</param>
        public ProgressionIntents(
            IMovementSender sender, Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(nameof(sender));
            _mint = mintOperationId ?? MintV7;
        }

        public async Awaitable<bool> RequestUpgrade(
            string skillId, int expectedLevel, CancellationToken cancel)
        {
            var req = new C2SSkillUpgrade
            {
                OperationId = ByteString.CopyFrom(_mint()),
                SkillId = skillId,
                ExpectedLevel = expectedLevel < 0 ? 0u : (uint)expectedLevel,
            };
            await _sender.SendAsync(ProgressionWireIds.C2SSkillUpgrade, req, cancel);
            return true;
        }

        public async Awaitable<bool> RequestAllocate(
            PotentialDelta deltas, CancellationToken cancel)
        {
            var req = new C2SPotentialAllocate
            {
                OperationId = ByteString.CopyFrom(_mint()),
                Deltas = deltas,
            };
            await _sender.SendAsync(ProgressionWireIds.C2SPotentialAllocate, req, cancel);
            return true;
        }

        public async Awaitable<bool> RequestRespec(
            string npcId, RespecKind kind, CancellationToken cancel)
        {
            var req = new C2SRespec
            {
                OperationId = ByteString.CopyFrom(_mint()),
                NpcId = npcId,
                Kind = kind,
            };
            await _sender.SendAsync(ProgressionWireIds.C2SRespec, req, cancel);
            return true;
        }

        /// <summary>UUIDv7 (ids.md) — same shape the session layer mints.</summary>
        private static byte[] MintV7()
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
            bytes[6] = (byte)(0x70 | bytes[6] & 0x0F);
            bytes[8] = (byte)(bytes[8] & 0x3F | 0x80);
            return bytes;
        }
    }
}
