using System;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.UI.StateMachine;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Hotbar cooldown slots → <see cref="HudDataModel"/>: a 203
    /// ACTION_STARTED for the self entity arms its slot's
    /// cooldown_ends_at_tick; a 204 rejection on the same skill clears it;
    /// expired rows are purged against the authoritative server tick.
    /// Slot order follows the <see cref="ISkillLoadout"/> mapping.
    /// </summary>
    public sealed class CooldownTracker : IDisposable
    {
        private readonly ReplicationApplier _replication;
        private readonly ISkillLoadout _loadout;
        private readonly HudDataModel _model;
        private readonly ICombatEventFeed _feed;
        private readonly HudCooldownSlot[] _rows =
            new HudCooldownSlot[HudDataModel.CooldownCapacity];
        private int _count;

        public CooldownTracker(
            ReplicationApplier replication,
            ISkillLoadout loadout,
            HudDataModel model,
            ICombatEventFeed feed)
        {
            _replication = replication ??
                throw new ArgumentNullException(nameof(replication));
            _loadout = loadout ??
                throw new ArgumentNullException(nameof(loadout));
            _model = model ?? throw new ArgumentNullException(nameof(model));
            _feed = feed ?? throw new ArgumentNullException(nameof(feed));
            _feed.ActionStarted += OnActionStarted;
            _feed.ActionRejected += OnActionRejected;
        }

        public void Dispose()
        {
            _feed.ActionStarted -= OnActionStarted;
            _feed.ActionRejected -= OnActionRejected;
        }

        /// <summary>Purges expired slots against the server tick.</summary>
        public void Refresh(ulong serverTick)
        {
            bool changed = false;
            for (int i = 0; i < _count; i++)
            {
                if (_rows[i].CooldownEndsAtTick <= serverTick)
                {
                    _rows[i] = _rows[--_count];
                    _rows[_count] = default;
                    i--;
                    changed = true;
                }
            }

            if (changed)
            {
                Publish();
            }
        }

        private void OnActionStarted(S2CActionStarted e)
        {
            EntityState? self = _replication.SelfState;
            if (self == null || e.SourceEntityId != self.EntityId)
            {
                return;
            }

            int slot = SlotOf(e.SkillId);
            if (slot <= 0)
            {
                return;
            }

            for (int i = 0; i < _count; i++)
            {
                if (_rows[i].Slot == slot)
                {
                    _rows[i] = new HudCooldownSlot(
                        slot, e.SkillId, e.CooldownEndsAtTick,
                        e.ServerTick);
                    Publish();
                    return;
                }
            }

            if (_count < _rows.Length)
            {
                _rows[_count++] = new HudCooldownSlot(
                    slot, e.SkillId, e.CooldownEndsAtTick, e.ServerTick);
                Publish();
            }
        }

        private void OnActionRejected(S2CActionRejected e)
        {
            for (int i = 0; i < _count; i++)
            {
                if (_rows[i].SkillId == e.SkillId)
                {
                    _rows[i] = _rows[--_count];
                    _rows[_count] = default;
                    Publish();
                    return;
                }
            }
        }

        private int SlotOf(string skillId)
        {
            for (int slot = 1; slot <= 5; slot++)
            {
                if (_loadout.SkillIdAt(slot) == skillId)
                {
                    return slot;
                }
            }

            return -1;
        }

        private void Publish()
        {
            // Slots already sit in loadout order (insertion order follows
            // arm order; the hotbar renders by slot index regardless).
            _model.SetCooldowns(
                new ReadOnlySpan<HudCooldownSlot>(_rows, 0, _count));
        }
    }
}
