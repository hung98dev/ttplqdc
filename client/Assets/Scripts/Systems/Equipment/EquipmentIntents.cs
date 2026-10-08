using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Equipment
{
    /// <summary>Production intents: mint + send over the bound sender.
    /// Only the three registered kinds are emittable — SKILL_SET and
    /// SOUL_CONTRACT belong to IMP-017/IMP-031 surfaces.</summary>
    public sealed class EquipmentIntents : IEquipmentIntents
    {
        private readonly IEquipmentSender _sender;
        private readonly Func<byte[]> _mint;

        public EquipmentIntents(IEquipmentSender sender,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _mint = mintOperationId ?? MintOperationId;
        }

        public async Awaitable<byte[]> RequestEquip(string loadoutId,
            string slotId, byte[] itemInstanceId,
            CancellationToken cancel)
        {
            var req = new C2SLoadoutChange
            {
                OperationId = ByteString.CopyFrom(_mint()),
                Kind = LoadoutChangeKind.Equip,
                Equip = new LoadoutEquip
                {
                    LoadoutId = loadoutId,
                    SlotId = slotId,
                    ItemInstanceId = ByteString.CopyFrom(itemInstanceId),
                },
            };
            await _sender.SendAsync(
                EquipmentApplier.C2SLoadoutChange, req, cancel);
            return req.OperationId.ToByteArray();
        }

        public async Awaitable<byte[]> RequestUnequip(string loadoutId,
            string slotId, CancellationToken cancel)
        {
            var req = new C2SLoadoutChange
            {
                OperationId = ByteString.CopyFrom(_mint()),
                Kind = LoadoutChangeKind.Unequip,
                Unequip = new LoadoutUnequip
                {
                    LoadoutId = loadoutId,
                    SlotId = slotId,
                },
            };
            await _sender.SendAsync(
                EquipmentApplier.C2SLoadoutChange, req, cancel);
            return req.OperationId.ToByteArray();
        }

        public async Awaitable<byte[]> RequestSwitchActive(
            string loadoutId, CancellationToken cancel)
        {
            var req = new C2SLoadoutChange
            {
                OperationId = ByteString.CopyFrom(_mint()),
                Kind = LoadoutChangeKind.SwitchActive,
                SwitchActive = new LoadoutSwitchActive
                {
                    LoadoutId = loadoutId,
                },
            };
            await _sender.SendAsync(
                EquipmentApplier.C2SLoadoutChange, req, cancel);
            return req.OperationId.ToByteArray();
        }

        /// <summary>UUIDv7 mint (wire_delivery § Operation IDs) —
        /// millisecond clock in the high bits, random tail.</summary>
        public static byte[] MintOperationId()
        {
            long unixMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
            var b = new byte[16];
            b[0] = (byte)(unixMs >> 40);
            b[1] = (byte)(unixMs >> 32);
            b[2] = (byte)(unixMs >> 24);
            b[3] = (byte)(unixMs >> 16);
            b[4] = (byte)(unixMs >> 8);
            b[5] = (byte)unixMs;
            byte[] tail = Guid.NewGuid().ToByteArray();
            for (int i = 6; i < 16; i++)
            {
                b[i] = tail[i - 6];
            }
            b[6] = (byte)(b[6] & 0x0F | 0x70);
            b[8] = (byte)(b[8] & 0x3F | 0x80);
            return b;
        }
    }
}
