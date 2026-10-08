using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Progression
{
    /// <summary>
    /// Authoritative progression projection (messages.md 515): the
    /// client never computes level, points or allocations locally — every
    /// S2C_PROGRESSION_STATE replaces the whole snapshot and bumps
    /// <see cref="Revision"/>. Presentation reads the snapshot; intents
    /// go through <see cref="IProgressionIntents"/>.
    /// </summary>
    public sealed class ProgressionState
    {
        /// <summary>Skill row in wire order (catalog document order).</summary>
        public readonly struct SkillRow
        {
            public readonly string SkillId;
            public readonly int Level;

            public SkillRow(string skillId, int level)
            {
                SkillId = skillId;
                Level = level;
            }
        }

        private readonly List<SkillRow> _skills = new List<SkillRow>();
        private readonly string?[] _activeSlots = new string?[5];

        public int Level
        {
            get;
            private set;
        }

        public long CurrentExp
        {
            get;
            private set;
        }

        public int UnspentSkillPoints
        {
            get;
            private set;
        }

        public int UnspentPotentialPoints
        {
            get;
            private set;
        }

        /// <summary>Allocated potential points {STR, VIT, INT, AGI}.</summary>
        public int PotentialStr
        {
            get;
            private set;
        }

        public int PotentialVit
        {
            get;
            private set;
        }

        public int PotentialInt
        {
            get;
            private set;
        }

        public int PotentialAgi
        {
            get;
            private set;
        }

        /// <summary>Total potential points ever earned (derivation feed).</summary>
        public int PotentialEarnedTotal
        {
            get;
            private set;
        }

        /// <summary>Wire's characters.progression_revision echo.</summary>
        public ulong ProgressionRevision
        {
            get;
            private set;
        }

        /// <summary>Local monotonic apply counter (presentation staleness).</summary>
        public ulong Revision
        {
            get;
            private set;
        }

        /// <summary>Attached character's class_id — set by composition
        /// (attach path); the 515 snapshot carries no class field.</summary>
        public string? ClassId
        {
            get;
            set;
        }

        public IReadOnlyList<SkillRow> Skills
        {
            get
            {
                return _skills;
            }
        }

        /// <summary>Default loadout basic skill (skills.md § Loadout).</summary>
        public string? BasicSkillId
        {
            get;
            private set;
        }

        /// <summary>Active slots 1..5 (null = empty).</summary>
        public IReadOnlyList<string?> ActiveSlots
        {
            get
            {
                return _activeSlots;
            }
        }

        /// <summary>
        /// Replaces the snapshot with a received 515 (REPLACEABLE_STATE —
        /// the newest push wins unconditionally).
        /// </summary>
        public void Apply(S2CProgressionState s)
        {
            Level = (int)s.Level;
            CurrentExp = s.CurrentExp;
            UnspentSkillPoints = (int)s.UnspentSkillPoints;
            UnspentPotentialPoints = (int)s.UnspentPotentialPoints;
            PotentialDelta alloc = s.PotentialAllocated;
            PotentialStr = alloc != null ? (int)alloc.Str : 0;
            PotentialVit = alloc != null ? (int)alloc.Vit : 0;
            PotentialInt = alloc != null ? (int)alloc.Int : 0;
            PotentialAgi = alloc != null ? (int)alloc.Agi : 0;
            PotentialEarnedTotal = (int)s.PotentialEarnedTotal;
            ProgressionRevision = s.ProgressionRevision;

            _skills.Clear();
            foreach (SkillView v in s.Skills)
            {
                _skills.Add(new SkillRow(v.SkillId, (int)v.Level));
            }

            SkillLoadoutView loadout = s.SkillLoadout;
            BasicSkillId = loadout != null && loadout.BasicSkillId.Length != 0
                ? loadout.BasicSkillId
                : null;
            for (int i = 0; i < _activeSlots.Length; i++)
            {
                _activeSlots[i] = null;
            }
            if (loadout != null)
            {
                for (int i = 0; i < _activeSlots.Length && i < loadout.ActiveSlots.Count; i++)
                {
                    string slot = loadout.ActiveSlots[i];
                    _activeSlots[i] = slot.Length != 0 ? slot : null;
                }
            }

            Revision++;
        }

        /// <summary>Reset on detach/session end — the next attach pushes
        /// a fresh snapshot.</summary>
        public void Clear()
        {
            Apply(new S2CProgressionState());
            ClassId = null;
        }
    }
}
