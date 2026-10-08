using System.Threading;
using UnityEngine;

namespace ThinhThan.Systems.Combat
{
    /// <summary>Outbound combat intents (200/201/202).</summary>
    public interface ICombatIntents
    {
        Awaitable<ulong> UseSkillAsync(
            string skillId, Protocol.V1.Facing facing, ulong targetEntityId,
            int areaCenterXMm, int areaCenterYMm, CancellationToken cancel);

        Awaitable<ulong> BasicAttackAsync(
            Protocol.V1.Facing facing, ulong targetEntityId,
            CancellationToken cancel);

        Awaitable<ulong> SetTargetAsync(
            ulong targetEntityId, CancellationToken cancel);
    }
}
