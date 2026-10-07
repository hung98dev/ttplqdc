using System;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.UI.StateMachine;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Self vitals → <see cref="HudDataModel"/>: HP/shield + DEAD flag from
    /// the replicated self state, MP from the self-private projection.
    /// Marks <see cref="HudDirty.Vitals"/> only on an actual change.
    /// </summary>
    public sealed class VitalsTracker
    {
        private readonly ReplicationApplier _replication;
        private readonly HudDataModel _model;

        public VitalsTracker(ReplicationApplier replication, HudDataModel model)
        {
            _replication = replication ??
                throw new ArgumentNullException(nameof(replication));
            _model = model ?? throw new ArgumentNullException(nameof(model));
        }

        /// <summary>Polls once per frame; marks Vitals when values moved.</summary>
        public void Refresh()
        {
            EntityState? self = _replication.SelfState;
            SelfPrivateState? selfPrivate = _replication.SelfPrivate;

            long hp = self?.Hp ?? 0L;
            long maxHp = self?.MaxHp ?? 0L;
            long shield = self?.Shield ?? 0L;
            bool dead = self != null &&
                (self.Flags & UiEntityFlags.Dead) != 0U;
            long mp = selfPrivate?.CurrentMp ?? 0L;
            long maxMp = selfPrivate?.MaxMp ?? 0L;

            if (hp == _model.Hp && maxHp == _model.MaxHp &&
                shield == _model.Shield && dead == _model.Dead &&
                mp == _model.Mp && maxMp == _model.MaxMp)
            {
                return;
            }

            _model.Hp = hp;
            _model.MaxHp = maxHp;
            _model.Shield = shield;
            _model.Dead = dead;
            _model.Mp = mp;
            _model.MaxMp = maxMp;
            _model.Mark(HudDirty.Vitals);
        }
    }
}
