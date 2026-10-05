using System;
using System.Collections.Generic;
using Google.Protobuf.Collections;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Replication
{
    /// <summary>
    /// Index-based entity view storage (client_performance.md § Smoothness by
    /// Construction): entity state lives in contiguous arrays, looked up
    /// through a dense id -> index map; no per-entity objects and no
    /// per-frame GetComponent. The local player is never stored — self state
    /// comes from baseline/self-ack channels (ADR-0071).
    /// </summary>
    public sealed class EntityViewStore
    {
        public const int InitialCapacity = 64;

        private readonly Dictionary<ulong, int> _indexById =
            new Dictionary<ulong, int>();

        private ulong[] _ids = new ulong[InitialCapacity];
        private EntityState?[] _states = new EntityState?[InitialCapacity];
        private int _count;

        public int Count
        {
            get
            {
                return _count;
            }
        }

        /// <summary>Contiguous id array; only the first Count entries live.</summary>
        public ReadOnlySpan<ulong> Ids
        {
            get
            {
                return new ReadOnlySpan<ulong>(_ids, 0, _count);
            }
        }

        /// <summary>Contiguous state array; only the first Count entries live.</summary>
        public ReadOnlySpan<EntityState?> States
        {
            get
            {
                return new ReadOnlySpan<EntityState?>(_states, 0, _count);
            }
        }

        public bool TryGetIndex(ulong entityId, out int index)
        {
            return _indexById.TryGetValue(entityId, out index);
        }

        /// <summary>Inserts or replaces an entity snapshot.</summary>
        public void Upsert(EntityState entity)
        {
            if (entity == null)
            {
                throw new ArgumentNullException(nameof(entity));
            }

            if (_indexById.TryGetValue(entity.EntityId, out int index))
            {
                _states[index] = entity;
                return;
            }

            if (_count == _ids.Length)
            {
                Array.Resize(ref _ids, _ids.Length * 2);
                Array.Resize(ref _states, _states.Length * 2);
            }

            _ids[_count] = entity.EntityId;
            _states[_count] = entity;
            _indexById[entity.EntityId] = _count;
            _count++;
        }

        /// <summary>Swap-remove keeps the arrays contiguous.</summary>
        public bool Remove(ulong entityId)
        {
            if (!_indexById.TryGetValue(entityId, out int index))
            {
                return false;
            }

            int last = _count - 1;
            if (index != last)
            {
                _ids[index] = _ids[last];
                _states[index] = _states[last];
                _indexById[_ids[index]] = index;
            }

            _states[last] = null;
            _ids[last] = 0UL;
            _indexById.Remove(entityId);
            _count--;
            return true;
        }

        /// <summary>
        /// Field-wise merge of an <see cref="EntityDelta"/> onto the stored
        /// snapshot (synchronization.md § Delta Semantics): absent fields are
        /// unchanged; list fields replace wholesale when present.
        /// </summary>
        public bool ApplyDelta(EntityDelta delta)
        {
            if (!_indexById.TryGetValue(delta.EntityId, out int index))
            {
                return false;
            }

            EntityState? state = _states[index];
            if (state == null)
            {
                return false;
            }

            if (delta.HasDisplayName)
            {
                state.DisplayName = delta.DisplayName;
            }

            if (delta.HasLevel)
            {
                state.Level = delta.Level;
            }

            if (delta.HasOwnerEntityId)
            {
                state.OwnerEntityId = delta.OwnerEntityId;
            }

            if (delta.HasXMm)
            {
                state.XMm = delta.XMm;
            }

            if (delta.HasYMm)
            {
                state.YMm = delta.YMm;
            }

            if (delta.HasVxMmS)
            {
                state.VxMmS = delta.VxMmS;
            }

            if (delta.HasVyMmS)
            {
                state.VyMmS = delta.VyMmS;
            }

            if (delta.HasFacing)
            {
                state.Facing = delta.Facing;
            }

            if (delta.HasMovementState)
            {
                state.MovementState = delta.MovementState;
            }

            if (delta.HasHp)
            {
                state.Hp = delta.Hp;
            }

            if (delta.HasMaxHp)
            {
                state.MaxHp = delta.MaxHp;
            }

            if (delta.HasShield)
            {
                state.Shield = delta.Shield;
            }

            if (delta.HasFlags)
            {
                state.Flags = delta.Flags;
            }

            if (delta.HasEncounterId)
            {
                state.EncounterId = delta.EncounterId;
            }

            if (delta.HasStatLifesteal)
            {
                state.StatLifesteal = delta.StatLifesteal;
            }

            if (delta.HasStatReflect)
            {
                state.StatReflect = delta.StatReflect;
            }

            if (delta.HasStatAbsorb)
            {
                state.StatAbsorb = delta.StatAbsorb;
            }

            if (delta.HasStatHealReduction)
            {
                state.StatHealReduction = delta.StatHealReduction;
            }

            if (delta.HasStatHealingReceived)
            {
                state.StatHealingReceived = delta.StatHealingReceived;
            }

            if (delta.Statuses != null)
            {
                state.Statuses.Clear();
                for (int e = 0; e < delta.Statuses.Entries.Count; e++)
                {
                    state.Statuses.Add(delta.Statuses.Entries[e]);
                }
            }

            if (delta.EquippedCosmetics != null)
            {
                state.EquippedCosmetics.Clear();
                for (int e = 0; e < delta.EquippedCosmetics.Entries.Count;
                    e++)
                {
                    state.EquippedCosmetics.Add(
                        delta.EquippedCosmetics.Entries[e]);
                }
            }

            return true;
        }

        public void Clear()
        {
            Array.Clear(_states, 0, _count);
            Array.Clear(_ids, 0, _count);
            _indexById.Clear();
            _count = 0;
        }
    }
}
