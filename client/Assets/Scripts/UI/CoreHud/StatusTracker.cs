using System;
using System.Collections.Generic;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.UI.StateMachine;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Buff/debuff rows → <see cref="HudDataModel"/>: authoritative rows
    /// are self + accepted-target <c>EntityState.statuses</c>; the 205
    /// feed carries the status_kind the wire puts on icons
    /// (buff = rounded frame / debuff = triangle per §6 — shape, never
    /// colour alone).
    /// </summary>
    public sealed class StatusTracker : IDisposable
    {
        private const string DefaultKind = "buff";

        private readonly ReplicationApplier _replication;
        private readonly HudDataModel _model;
        private readonly ICombatEventFeed? _feed;
        private readonly Dictionary<string, string> _kinds =
            new Dictionary<string, string>(StringComparer.Ordinal);
        private readonly HudStatusSlot[] _rows =
            new HudStatusSlot[HudDataModel.StatusCapacity];
        private readonly HudStatusSlot[] _applied =
            new HudStatusSlot[HudDataModel.StatusCapacity];
        private int _appliedCount;

        public StatusTracker(
            ReplicationApplier replication,
            HudDataModel model,
            ICombatEventFeed? feed = null)
        {
            _replication = replication ??
                throw new ArgumentNullException(nameof(replication));
            _model = model ?? throw new ArgumentNullException(nameof(model));
            _feed = feed;
            if (_feed != null)
            {
                _feed.StatusEvent += OnStatusEvent;
            }
        }

        public void Dispose()
        {
            if (_feed != null)
            {
                _feed.StatusEvent -= OnStatusEvent;
            }
        }

        /// <summary>Rebuilds status rows from authoritative state.</summary>
        public void Refresh()
        {
            int n = 0;
            EntityState? self = _replication.SelfState;
            if (self != null)
            {
                n = Collect(self.Statuses, n);
            }

            if (_model.HasTarget &&
                _replication.Store.TryGetIndex(
                    _model.TargetEntityId, out int ti))
            {
                EntityState? target = _replication.Store.States[ti];
                if (target != null)
                {
                    n = Collect(target.Statuses, n);
                }
            }

            if (n == _appliedCount && Same(n))
            {
                return;
            }

            for (int i = 0; i < n; i++)
            {
                _applied[i] = _rows[i];
            }

            _appliedCount = n;
            _model.SetStatuses(
                new ReadOnlySpan<HudStatusSlot>(_rows, 0, n));
        }

        private bool Same(int n)
        {
            for (int i = 0; i < n; i++)
            {
                HudStatusSlot a = _applied[i];
                HudStatusSlot b = _rows[i];
                if (a.EffectId != b.EffectId || a.StatusKind != b.StatusKind ||
                    a.Stacks != b.Stacks || a.ExpiresAtTick != b.ExpiresAtTick)
                {
                    return false;
                }
            }

            return true;
        }

        private int Collect(
            Google.Protobuf.Collections.RepeatedField<EntityStatus> statuses,
            int n)
        {
            for (int i = 0; i < statuses.Count &&
                n < _rows.Length; i++)
            {
                EntityStatus s = statuses[i];
                _kinds.TryGetValue(s.EffectId, out string? kind);
                _rows[n++] = new HudStatusSlot(
                    s.EffectId, kind ?? DefaultKind, s.Stacks,
                    s.ExpiresAtTick);
            }

            return n;
        }

        /// <summary>
        /// 205 events teach the kind map (effect_id → status_kind) and mark
        /// the section — the next <see cref="Update"/> rebuilds rows.
        /// </summary>
        private void OnStatusEvent(S2CStatusEvent e)
        {
            _kinds[e.EffectId] = e.StatusKind;
            _model.Mark(HudDirty.Statuses);
        }
    }
}
