using System.Threading;
using UnityEngine;

namespace ThinhThan.Systems.Equipment
{
    /// <summary>C2S_LOADOUT_CHANGE intents (402), EQUIP/UNEQUIP/
    /// SWITCH_ACTIVE only.</summary>
    public interface IEquipmentIntents
    {
        Awaitable<byte[]> RequestEquip(string loadoutId, string slotId,
            byte[] itemInstanceId, CancellationToken cancel);

        Awaitable<byte[]> RequestUnequip(string loadoutId,
            string slotId, CancellationToken cancel);

        Awaitable<byte[]> RequestSwitchActive(string loadoutId,
            CancellationToken cancel);
    }

}
