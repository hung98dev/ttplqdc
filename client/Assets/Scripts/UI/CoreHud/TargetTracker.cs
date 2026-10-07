using System;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.UI.StateMachine;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Target frame → <see cref="HudDataModel"/>: displays ONLY the
    /// server-accepted target (SelfPrivate.accepted_target_entity_id →
    /// replicated entity). ADR-0071: the client never shows a locally
    /// chosen target.
    /// </summary>
    public sealed class TargetTracker
    {
        private readonly ReplicationApplier _replication;
        private readonly HudDataModel _model;

        public TargetTracker(ReplicationApplier replication, HudDataModel model)
        {
            _replication = replication ??
                throw new ArgumentNullException(nameof(replication));
            _model = model ?? throw new ArgumentNullException(nameof(model));
        }

        public void Refresh()
        {
            SelfPrivateState? selfPrivate = _replication.SelfPrivate;
            ulong id = selfPrivate?.AcceptedTargetEntityId ?? 0UL;

            EntityState? target = null;
            if (id != 0UL &&
                _replication.Store.TryGetIndex(id, out int index) &&
                index >= 0 && index < _replication.Store.States.Length)
            {
                target = _replication.Store.States[index];
            }

            bool has = target != null;
            string? name = target?.DisplayName;
            uint level = target?.Level ?? 0U;
            EntityKind kind = target?.EntityKind ?? EntityKind.Unspecified;
            long hp = target?.Hp ?? 0L;
            long maxHp = target?.MaxHp ?? 0L;
            bool dead = target != null &&
                (target.Flags & UiEntityFlags.Dead) != 0U;

            if (has == _model.HasTarget && id == _model.TargetEntityId &&
                name == _model.TargetName && level == _model.TargetLevel &&
                kind == _model.TargetKind && hp == _model.TargetHp &&
                maxHp == _model.TargetMaxHp && dead == _model.TargetDead)
            {
                return;
            }

            _model.HasTarget = has;
            _model.TargetEntityId = has ? id : 0UL;
            _model.TargetName = name;
            _model.TargetLevel = level;
            _model.TargetKind = kind;
            _model.TargetHp = hp;
            _model.TargetMaxHp = maxHp;
            _model.TargetDead = dead;
            _model.Mark(HudDirty.Target);
        }
    }
}
