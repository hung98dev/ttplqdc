using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Effects
{
    /// <summary>
    /// Status-effect presentation driven inside FrameLoop: watches
    /// <see cref="EffectState.Version"/> and reconciles pooled icon +
    /// duration-bar presenters per entity; S2C_STATUS_EVENT deltas and
    /// baseline snapshots drive everything — nothing is predicted.
    /// No Update/coroutines/LINQ (ADR-0059).
    /// </summary>
    public sealed class EffectPresentation
    {
        private readonly EffectState _state;
        private readonly EffectPresenters _pools;
        private readonly Dictionary<ulong, List<EffectPresenters.IconPresenter>> _liveIcons =
            new Dictionary<ulong, List<EffectPresenters.IconPresenter>>();
        private readonly Dictionary<ulong, List<EffectPresenters.BarPresenter>> _liveBars =
            new Dictionary<ulong, List<EffectPresenters.BarPresenter>>();
        private ulong _seenVersion;

        public EffectPresentation(EffectState state, EffectPresenters pools)
        {
            _state = state;
            _pools = pools;
        }

        /// <summary>Presentation cue kinds surfaced to the UI layer.</summary>
        public enum CueKind
        {
            /// <summary>A status icon appeared.</summary>
            IconShown = 0,

            /// <summary>A status icon refreshed duration/stacks.</summary>
            IconRefreshed = 1,

            /// <summary>A status icon expired/was dispelled/consumed.</summary>
            IconRemoved = 2,
        }

        /// <summary>One presentation cue.</summary>
        public readonly struct Cue
        {
            /// <summary>Cue kind.</summary>
            public CueKind Kind
            {
                get;
            }

            /// <summary>Entity id.</summary>
            public ulong EntityId
            {
                get;
            }

            /// <summary>Template id.</summary>
            public string EffectId
            {
                get;
            }

            public Cue(CueKind kind, ulong entityId, string effectId)
            {
                Kind = kind;
                EntityId = entityId;
                EffectId = effectId;
            }
        }

        private readonly List<Cue> _cues = new List<Cue>();

        /// <summary>Cues produced by the last Evaluate (reused list).</summary>
        public List<Cue> Cues
        {
            get
            {
                return _cues;
            }
        }

        /// <summary>Live icon presenters for an entity (test seam).</summary>
        public int IconCount(ulong entityId)
        {
            return _liveIcons.TryGetValue(entityId, out var l) ? l.Count : 0;
        }

        /// <summary>Live bar presenters for an entity (test seam).</summary>
        public int BarCount(ulong entityId)
        {
            return _liveBars.TryGetValue(entityId, out var l) ? l.Count : 0;
        }

        /// <summary>FrameLoop Presentation-phase step: reconcile
        /// presenters against the current status model and update bar
        /// remaining times from the authoritative tick.</summary>
        public void Evaluate(ulong serverTick)
        {
            _cues.Clear();
            if (_seenVersion != _state.Version)
            {
                _seenVersion = _state.Version;
                foreach (var kv in _liveIcons)
                {
                    Reconcile(kv.Key, serverTick);
                }
            }
            UpdateBars(serverTick);
        }

        /// <summary>Notifies presentation that an entity's model
        /// changed (called by the sink wiring for entities whose
        /// versions moved). Each notify batch replaces the cue list.</summary>
        public void NotifyEntity(ulong entityId, ulong serverTick)
        {
            _cues.Clear();
            EnsureLists(entityId);
            Reconcile(entityId, serverTick);
        }

        /// <summary>Drops presenters for an entity that left the view.</summary>
        public void RemoveEntity(ulong entityId)
        {
            if (_liveIcons.TryGetValue(entityId, out var icons))
            {
                foreach (var p in icons)
                {
                    _pools.ReturnIcon(p);
                }
                _liveIcons.Remove(entityId);
            }
            if (_liveBars.TryGetValue(entityId, out var bars))
            {
                foreach (var p in bars)
                {
                    _pools.ReturnBar(p);
                }
                _liveBars.Remove(entityId);
            }
            _state.RemoveEntity(entityId);
        }

        private void EnsureLists(ulong entityId)
        {
            if (!_liveIcons.ContainsKey(entityId))
            {
                _liveIcons[entityId] = new List<EffectPresenters.IconPresenter>();
            }
            if (!_liveBars.ContainsKey(entityId))
            {
                _liveBars[entityId] = new List<EffectPresenters.BarPresenter>();
            }
        }

        private void Reconcile(ulong entityId, ulong serverTick)
        {
            var icons = _liveIcons[entityId];
            var bars = _liveBars[entityId];
            var entries = _state.Entries(entityId);

            // Remove presenters whose entry vanished.
            for (int i = icons.Count - 1; i >= 0; i--)
            {
                var p = icons[i];
                if (!Find(entries, p.EffectId))
                {
                    _cues.Add(new Cue(CueKind.IconRemoved, entityId, p.EffectId));
                    _pools.ReturnIcon(p);
                    icons.RemoveAt(i);
                }
            }
            for (int i = bars.Count - 1; i >= 0; i--)
            {
                var p = bars[i];
                if (!Find(entries, p.EffectId))
                {
                    _pools.ReturnBar(p);
                    bars.RemoveAt(i);
                }
            }

            // Upsert presenters per live entry.
            foreach (var e in entries)
            {
                var icon = FindIcon(icons, e.EffectId);
                if (icon == null)
                {
                    icon = _pools.RentIcon();
                    icon.EntityId = entityId;
                    icon.EffectId = e.EffectId;
                    icon.StatusKind = e.StatusKind;
                    icon.Stacks = e.Stacks;
                    icon.ExpiresAtTick = e.ExpiresAtTick;
                    icons.Add(icon);
                    _cues.Add(new Cue(CueKind.IconShown, entityId, e.EffectId));
                }
                else if (icon.Stacks != e.Stacks || icon.ExpiresAtTick != e.ExpiresAtTick)
                {
                    icon.Stacks = e.Stacks;
                    icon.ExpiresAtTick = e.ExpiresAtTick;
                    _cues.Add(new Cue(CueKind.IconRefreshed, entityId, e.EffectId));
                }
                if (FindBar(bars, e.EffectId) == null)
                {
                    var bar = _pools.RentBar();
                    bar.EntityId = entityId;
                    bar.EffectId = e.EffectId;
                    bar.ExpiresAtTick = e.ExpiresAtTick;
                    bars.Add(bar);
                }
            }
        }

        private static bool Find(List<EffectState.Entry> entries, string effectId)
        {
            foreach (var e in entries)
            {
                if (e.EffectId == effectId)
                {
                    return true;
                }
            }
            return false;
        }

        private static EffectPresenters.IconPresenter? FindIcon(
            List<EffectPresenters.IconPresenter> icons, string effectId)
        {
            foreach (var p in icons)
            {
                if (p.EffectId == effectId)
                {
                    return p;
                }
            }
            return null;
        }

        private static EffectPresenters.BarPresenter? FindBar(
            List<EffectPresenters.BarPresenter> bars, string effectId)
        {
            foreach (var p in bars)
            {
                if (p.EffectId == effectId)
                {
                    return p;
                }
            }
            return null;
        }

        private void UpdateBars(ulong serverTick)
        {
            foreach (var kv in _liveBars)
            {
                foreach (var p in kv.Value)
                {
                    p.RemainingTicks = p.ExpiresAtTick > serverTick
                        ? p.ExpiresAtTick - serverTick
                        : 0;
                }
            }
        }
    }
}
