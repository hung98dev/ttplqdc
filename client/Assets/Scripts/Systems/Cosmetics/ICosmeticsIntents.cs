using System.Threading;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Cosmetics
{
    /// <summary>
    /// Client intents → wire messages. Cosmetic ids are catalog ids;
    /// the server decides ownership, routes and category fits
    /// (client authority boundary — cosmetics.md §2).
    /// </summary>
    public interface ICosmeticsIntents
    {
        /// <summary>C2S_COSMETIC_REDEEM (422) with one selected route.</summary>
        Awaitable<byte[]> RequestRedeem(string cosmeticId,
            CosmeticRoute route, CancellationToken cancel);

        /// <summary>C2S_COSMETIC_EQUIP (424); empty cosmeticId unequips.</summary>
        Awaitable<byte[]> RequestEquip(CosmeticSlot slot,
            string cosmeticId, CancellationToken cancel);

        /// <summary>C2S_GUILD_COSMETIC_EQUIP (656); empty cosmeticId
        /// clears the slot. expectedRevision comes from the 628 view.</summary>
        Awaitable<byte[]> RequestGuildEquip(byte[] guildId,
            GuildCosmeticSlot slot, string cosmeticId,
            ulong expectedRevision, CancellationToken cancel);
    }
}
