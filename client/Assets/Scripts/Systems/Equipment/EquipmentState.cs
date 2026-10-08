using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Equipment
{
    /// <summary>
    /// Authoritative loadout mirror: the three canonical loadout ids,
    /// each slot's bound item_instance_id, the ACTIVE id and the wire
    /// `loadout_revision` (403). Client-owned cache of server truth —
    /// the server is authoritative for every equip decision; this
    /// state only replays committed results for display.
    /// </summary>
    public sealed class EquipmentState
    {
        public static readonly string[] LoadoutIds =
        {
            "loadout.primary", "loadout.secondary_1",
            "loadout.secondary_2",
        };

        private readonly Dictionary<string, Dictionary<string, byte[]>> _slots =
            new Dictionary<string, Dictionary<string, byte[]>>();

        public EquipmentState()
        {
            foreach (string id in LoadoutIds)
            {
                _slots[id] = new Dictionary<string, byte[]>();
            }
        }

        public string ActiveLoadoutId = "loadout.primary";

        /// <summary>Server's SUM(character_loadouts.revision).</summary>
        public ulong LoadoutRevision;

        /// <summary>item_instance_id bound at (loadout_id, slot_id);
        /// null when empty.</summary>
        public byte[]? ItemAt(string loadoutId, string slotId)
        {
            if (!_slots.TryGetValue(loadoutId, out var slots))
            {
                return null;
            }
            return slots.TryGetValue(slotId, out var item) ? item : null;
        }

        public void ApplyResult(S2CLoadoutResult result)
        {
            LoadoutRevision = result.LoadoutRevision;
            if (!string.IsNullOrEmpty(result.ActiveLoadoutId))
            {
                ActiveLoadoutId = result.ActiveLoadoutId;
            }
            foreach (LoadoutChangedSlot change in result.ChangedSlots)
            {
                if (!_slots.TryGetValue(change.LoadoutId, out var slots))
                {
                    continue;
                }
                if (change.ItemInstanceId.Length == 0)
                {
                    slots.Remove(change.SlotId);
                }
                else
                {
                    slots[change.SlotId] =
                        change.ItemInstanceId.ToByteArray();
                }
            }
        }
    }
}
