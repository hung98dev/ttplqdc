using System.Threading;
using UnityEngine;

namespace ThinhThan.Systems.Crafting
{
    /// <summary>Client intents for the crafting C2S set (404, 406).</summary>
    public interface ICraftingIntents
    {
        Awaitable<byte[]> RequestCraft(string npcId, string recipeId,
            uint batchQuantity, CancellationToken cancel);
        Awaitable<byte[]> RequestEnhance(string npcId,
            byte[] itemInstanceId, uint targetLevel,
            byte[] luckyCharmItemInstanceId,
            byte[] insuranceItemInstanceId, CancellationToken cancel);
    }

}
