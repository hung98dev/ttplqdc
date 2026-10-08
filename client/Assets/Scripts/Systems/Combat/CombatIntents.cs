using System;
using System.Threading;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Combat
{
    /// <summary>
    /// Production combat intents: stamp client_mono_ms on the injected
    /// monotonic clock and send over the bound sender. The server runs
    /// validation; the client never predicts acceptance.
    /// </summary>
    public sealed class CombatIntents : ICombatIntents
    {
        private readonly ICombatSender _sender;
        private readonly Func<long> _monoMs;

        public CombatIntents(ICombatSender sender, Func<long> monoMs)
        {
            _sender = sender ?? throw new ArgumentNullException(
                nameof(sender));
            _monoMs = monoMs ?? throw new ArgumentNullException(
                nameof(monoMs));
        }

        public async Awaitable<ulong> UseSkillAsync(
            string skillId, Facing facing, ulong targetEntityId,
            int areaCenterXMm, int areaCenterYMm, CancellationToken cancel)
        {
            var req = new C2SSkillUse
            {
                ClientMonoMs = (ulong)_monoMs(),
                SkillId = skillId,
                Facing = facing,
                TargetEntityId = targetEntityId,
                AreaCenterXMm = areaCenterXMm,
                AreaCenterYMm = areaCenterYMm,
            };
            return await _sender.SendAsync(
                CombatApplier.C2SSkillUse, req, cancel);
        }

        public async Awaitable<ulong> BasicAttackAsync(
            Facing facing, ulong targetEntityId, CancellationToken cancel)
        {
            var req = new C2SBasicAttack
            {
                ClientMonoMs = (ulong)_monoMs(),
                Facing = facing,
                TargetEntityId = targetEntityId,
            };
            return await _sender.SendAsync(
                CombatApplier.C2SBasicAttack, req, cancel);
        }

        public async Awaitable<ulong> SetTargetAsync(
            ulong targetEntityId, CancellationToken cancel)
        {
            var req = new C2STargetIntent
            {
                TargetEntityId = targetEntityId,
            };
            return await _sender.SendAsync(
                CombatApplier.C2STargetIntent, req, cancel);
        }
    }
}
