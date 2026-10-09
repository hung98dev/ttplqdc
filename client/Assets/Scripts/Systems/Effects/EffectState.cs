using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Effects
{
    /// <summary>
    /// Client-side mirror of the authoritative status/shield state:
    /// the per-entity status model populated from
    /// <see cref="EntityState.Statuses"/> baseline snapshots and
    /// S2C_STATUS_EVENT (205) deltas. Display-only — the server owns
    /// every transition (status_effects.md; client.md authority
    /// boundary).
    /// </summary>
    public sealed class EffectState
    {
        /// <summary>One live status entry on an entity.</summary>
        public readonly struct Entry
        {
            /// <summary>Canonical template id.</summary>
            public string EffectId
            {
                get;
            }

            /// <summary>Applying source entity.</summary>
            public ulong SourceEntityId
            {
                get;
            }

            /// <summary>Current stack count.</summary>
            public uint Stacks
            {
                get;
            }

            /// <summary>Authoritative expiry tick.</summary>
            public ulong ExpiresAtTick
            {
                get;
            }

            /// <summary>Wire status_kind label (control/slow/dot/buff/debuff).</summary>
            public string StatusKind
            {
                get;
            }

            public Entry(string effectId, ulong source, uint stacks, ulong expires, string kind)
            {
                EffectId = effectId;
                SourceEntityId = source;
                Stacks = stacks;
                ExpiresAtTick = expires;
                StatusKind = kind;
            }
        }

        private sealed class Key
        {
            public readonly string EffectId;
            public readonly ulong Source;

            public Key(string effectId, ulong source)
            {
                EffectId = effectId;
                Source = source;
            }

            public override bool Equals(object? obj)
            {
                return obj is Key other &&
                    other.EffectId == EffectId && other.Source == Source;
            }

            public override int GetHashCode()
            {
                return (EffectId, Source).GetHashCode();
            }
        }

        private readonly Dictionary<ulong, Dictionary<Key, Entry>> _byEntity =
            new Dictionary<ulong, Dictionary<Key, Entry>>();
        private readonly List<Entry> _scratch = new List<Entry>(16);
        private ulong _version;

        /// <summary>Bumped on every consumed frame/snapshot so
        /// presenters re-present on change.</summary>
        public ulong Version
        {
            get
            {
                return _version;
            }
        }

        /// <summary>Number of live entries on one entity.</summary>
        public int Count(ulong entityId)
        {
            return _byEntity.TryGetValue(entityId, out var book) ? book.Count : 0;
        }

        /// <summary>Entries on one entity in deterministic order
        /// (source, then effect id). The returned list is reused —
        /// callers copy what they keep.</summary>
        public List<Entry> Entries(ulong entityId)
        {
            _scratch.Clear();
            if (_byEntity.TryGetValue(entityId, out var book))
            {
                foreach (var e in book.Values)
                {
                    _scratch.Add(e);
                }
            }
            _scratch.Sort(CompareEntries);
            return _scratch;
        }

        private static int CompareEntries(Entry a, Entry b)
        {
            if (a.SourceEntityId != b.SourceEntityId)
            {
                return a.SourceEntityId < b.SourceEntityId ? -1 : 1;
            }
            return string.CompareOrdinal(a.EffectId, b.EffectId);
        }

        /// <summary>Applies one S2C_STATUS_EVENT delta: APPLIED /
        /// REFRESHED / STACK_CHANGED upsert; EXPIRED / DISPELLED /
        /// CONSUMED remove.</summary>
        public void ApplyEvent(S2CStatusEvent ev)
        {
            var book = Book(ev.TargetEntityId);
            var key = new Key(ev.EffectId, ev.SourceEntityId);
            switch (ev.Event)
            {
                case StatusEventKind.Applied:
                case StatusEventKind.Refreshed:
                case StatusEventKind.StackChanged:
                    book[key] = new Entry(
                        ev.EffectId, ev.SourceEntityId, ev.Stacks,
                        ev.ExpiresAtTick, ev.StatusKind);
                    break;
                case StatusEventKind.Expired:
                case StatusEventKind.Dispelled:
                case StatusEventKind.Consumed:
                    book.Remove(key);
                    break;
                default:
                    return;
            }
            _version++;
        }

        /// <summary>Replaces an entity's model from an authoritative
        /// baseline snapshot's statuses.</summary>
        public void SyncSnapshot(EntityState snap)
        {
            var book = Book(snap.EntityId);
            book.Clear();
            foreach (var s in snap.Statuses)
            {
                var key = new Key(s.EffectId, s.SourceEntityId);
                book[key] = new Entry(s.EffectId, s.SourceEntityId, s.Stacks, s.ExpiresAtTick, string.Empty);
            }
            _version++;
        }

        /// <summary>Drops all state for an entity that left the view.</summary>
        public void RemoveEntity(ulong entityId)
        {
            if (_byEntity.Remove(entityId))
            {
                _version++;
            }
        }

        private Dictionary<Key, Entry> Book(ulong entityId)
        {
            if (!_byEntity.TryGetValue(entityId, out var book))
            {
                book = new Dictionary<Key, Entry>();
                _byEntity[entityId] = book;
            }
            return book;
        }
    }
}
