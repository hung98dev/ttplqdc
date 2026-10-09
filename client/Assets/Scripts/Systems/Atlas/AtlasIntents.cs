using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Atlas
{
    /// <summary>Production intents: mint + send over the bound sender.</summary>
    public sealed class AtlasIntents : IAtlasIntents
    {
        private readonly IAtlasSender _sender;
        private readonly Func<byte[]> _mint;

        public AtlasIntents(IAtlasSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> Acknowledge(
            string atlasPageId, uint tier, CancellationToken cancel)
        {
            byte[] operationId = _mint();
            var req = new C2SAtlasClaim
            {
                OperationId = ByteString.CopyFrom(operationId),
                AtlasPageId = atlasPageId,
                Tier = tier,
            };
            await _sender.SendAsync(
                AtlasApplier.C2SAtlasClaim, req, cancel);
            return operationId;
        }

        /// <summary>
        /// UUIDv7 mint (ids.md): 48-bit unix millis, version 7,
        /// process-counter mid section, random tail — same shape
        /// InventoryIntents.MintOperationId produces.
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
