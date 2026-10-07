using System;
using System.Collections.Generic;
using ThinhThan.Core.Rendering;
using UnityEngine;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Aggregate live-particle budget (client_performance.md item 2,
    /// PERF-006): the count of live particles across all registered systems
    /// never exceeds the per-level ceiling — 512/1024/2048 by preset,
    /// trimmed further by <see cref="QualityGovernor"/> levels. Mandatory
    /// dangerous-telegraph presentation reserves its required count first;
    /// optional emissions use the remainder and are refused (or trimmed
    /// before the next emission) when the ceiling would be exceeded.
    /// </summary>
    public sealed class ParticleBudget
    {
        private readonly List<ParticleSystem> _systems = new List<ParticleSystem>();
        private int _presetCeiling;
        private int _level = QualityGovernor.ParticleLevelFull;
        private int _mandatoryReserved;

        public ParticleBudget(QualityPreset preset)
        {
            _presetCeiling = PresetApplier.ParticleCeilingFor(preset);
        }

        /// <summary>Preset ceiling B before governor levels (512/1024/2048).</summary>
        public int PresetCeiling
        {
            get
            {
                return _presetCeiling;
            }
        }

        /// <summary>Governor level in [2, 4]; 4 = 100% of the preset ceiling.</summary>
        public int Level
        {
            get
            {
                return _level;
            }
        }

        /// <summary>Effective ceiling: floor(B · level / 4).</summary>
        public int EffectiveCeiling
        {
            get
            {
                return _presetCeiling * _level / QualityGovernor.ParticleLevelFull;
            }
        }

        /// <summary>Mandatory telegraph count currently reserved.</summary>
        public int MandatoryReserved
        {
            get
            {
                return _mandatoryReserved;
            }
        }

        /// <summary>
        /// Live particle count summed across registered systems; dead
        /// references are pruned lazily.
        /// </summary>
        public int LiveCount
        {
            get
            {
                int live = 0;
                for (int i = _systems.Count - 1; i >= 0; i--)
                {
                    ParticleSystem system = _systems[i];
                    if (system == null)
                    {
                        _systems.RemoveAt(i);
                        continue;
                    }

                    live += system.particleCount;
                }

                return live;
            }
        }

        /// <summary>
        /// Particle count still available to optional emissions: the
        /// remainder after mandatory telegraph reservations are charged.
        /// </summary>
        public int OptionalAvailable
        {
            get
            {
                int free = EffectiveCeiling - _mandatoryReserved - LiveCount;
                return free > 0 ? free : 0;
            }
        }

        /// <summary>Registers a system so its particles count against the budget.</summary>
        public void Register(ParticleSystem system)
        {
            if (system == null)
            {
                throw new ArgumentNullException(nameof(system));
            }

            if (!_systems.Contains(system))
            {
                _systems.Add(system);
            }
        }

        /// <summary>Removes a system from the budget accounting.</summary>
        public void Unregister(ParticleSystem system)
        {
            if (system != null)
            {
                _systems.Remove(system);
            }
        }

        /// <summary>Switches preset; governor level resets to full.</summary>
        public void SetPreset(QualityPreset preset)
        {
            _presetCeiling = PresetApplier.ParticleCeilingFor(preset);
            _level = QualityGovernor.ParticleLevelFull;
        }

        /// <summary>Applies a governor level — presentation-only trim.</summary>
        public void ApplyLevel(int level)
        {
            _level = Math.Max(
                QualityGovernor.ParticleLevelMin,
                Math.Min(QualityGovernor.ParticleLevelFull, level));
        }

        /// <summary>
        /// Reserves count for a mandatory dangerous-telegraph emission.
        /// Mandatory presentation is guaranteed: the reserve always applies
        /// and is charged before optional capacity — authoring must fit
        /// simultaneous mandatory telegraphs even at the governor floor.
        /// </summary>
        public void ReserveMandatory(int count)
        {
            if (count > 0)
            {
                _mandatoryReserved += count;
            }
        }

        /// <summary>Releases a telegraph reservation.</summary>
        public void ReleaseMandatory(int count)
        {
            _mandatoryReserved = Math.Max(0, _mandatoryReserved - Math.Max(0, count));
        }

        /// <summary>
        /// Requests an optional emission. Returns true when the requested
        /// count fits in the remainder after mandatory reservations; the
        /// caller trims (or skips) the emission when false.
        /// </summary>
        public bool TryEmitOptional(int count)
        {
            if (count < 0)
            {
                throw new ArgumentOutOfRangeException(nameof(count));
            }

            return LiveCount + _mandatoryReserved + count <= EffectiveCeiling;
        }
    }
}
