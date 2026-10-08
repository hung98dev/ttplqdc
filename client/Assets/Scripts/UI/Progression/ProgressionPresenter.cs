using System;
using System.Threading;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Progression;
using UnityEngine;

namespace ThinhThan.UI.Progression
{
    /// <summary>
    /// Bridge between the authoritative <see cref="ProgressionState"/>
    /// projection and the progression panel: reads the state (never
    /// mutates it), exposes a view model with dirty sections, and turns
    /// panel gestures into <see cref="IProgressionIntents"/> calls.
    /// Point budgeting shown here is presentation-only — the server
    /// re-validates every mutation.
    /// </summary>
    public sealed class ProgressionPresenter
    {
        /// <summary>Dirty sections for once-per-frame widget writes.</summary>
        [Flags]
        public enum Dirty : byte
        {
            None = 0,
            Vitals = 1 << 0,   // level + exp + revision line
            Points = 1 << 1,   // unspent pools + allocated potential
            Skills = 1 << 2,   // skill rows + loadout
            Result = 1 << 3,   // 514 verdict line
        }

        /// <summary>Panel-facing snapshot — filled by Sync().</summary>
        public struct Model
        {
            public int Level;
            public long CurrentExp;
            public long ExpToNext;
            public int UnspentSkillPoints;
            public int UnspentPotentialPoints;
            public int PotentialStr;
            public int PotentialVit;
            public int PotentialInt;
            public int PotentialAgi;
            public int PotentialEarnedTotal;
            public string? BasicSkillId;
            public string?[] ActiveSlots;
            public ProgressionState.SkillRow[] Skills;
            /// <summary>Latest 514 verdict text: ok → "Lưu thành công",
            /// else the error code name.</summary>
            public string ResultText;
            public Dirty Dirty;
        }

        private readonly ProgressionState _state;
        private readonly ProgressionApplier _applier;
        private readonly IProgressionIntents _intents;
        private readonly CancellationTokenSource _cancel =
            new CancellationTokenSource();

        private Model _model;
        private ulong _seenRevision;
        private ulong _seenResultRevision;

        public ProgressionPresenter(
            ProgressionState state,
            ProgressionApplier applier,
            IProgressionIntents intents)
        {
            _state = state ?? throw new ArgumentNullException(nameof(state));
            _applier = applier ??
                throw new ArgumentNullException(nameof(applier));
            _intents = intents ?? throw new ArgumentNullException(nameof(intents));
            _model.ActiveSlots = new string?[5];
            _model.Skills = new ProgressionState.SkillRow[0];
            _model.ResultText = string.Empty;
        }

        public Model Snapshot
        {
            get
            {
                return _model;
            }
        }

        /// <summary>UI cadence polls; presentations apply only dirty
        /// sections and then call <see cref="ClearDirty"/>.</summary>
        public void Sync()
        {
            if (_state.Revision != _seenRevision)
            {
                _seenRevision = _state.Revision;
                _model.Level = _state.Level;
                _model.CurrentExp = _state.CurrentExp;
                _model.ExpToNext = ExpToNext(_state.Level);
                _model.UnspentSkillPoints = _state.UnspentSkillPoints;
                _model.UnspentPotentialPoints = _state.UnspentPotentialPoints;
                _model.PotentialStr = _state.PotentialStr;
                _model.PotentialVit = _state.PotentialVit;
                _model.PotentialInt = _state.PotentialInt;
                _model.PotentialAgi = _state.PotentialAgi;
                _model.PotentialEarnedTotal = _state.PotentialEarnedTotal;
                _model.BasicSkillId = _state.BasicSkillId;
                Array.Clear(_model.ActiveSlots, 0, _model.ActiveSlots.Length);
                for (int i = 0; i < _state.ActiveSlots.Count; i++)
                {
                    _model.ActiveSlots[i] = _state.ActiveSlots[i];
                }
                _model.Skills = new ProgressionState.SkillRow[_state.Skills.Count];
                for (int i = 0; i < _state.Skills.Count; i++)
                {
                    _model.Skills[i] = _state.Skills[i];
                }
                _model.Dirty = Dirty.Vitals | Dirty.Points | Dirty.Skills;
            }
            if (_applier.ResultRevision != _seenResultRevision)
            {
                _seenResultRevision = _applier.ResultRevision;
                _model.ResultText = ResultText();
                _model.Dirty |= Dirty.Result;
            }
        }

        public void ClearDirty()
        {
            _model.Dirty = Dirty.None;
        }

        /// <summary>Panel gesture → C2S_SKILL_UPGRADE. Fire-and-forget:
        /// the 514 verdict lands on the result line.</summary>
        public Awaitable<bool> UpgradeSkill(string skillId, int expectedLevel)
        {
            return _intents.RequestUpgrade(
                skillId, expectedLevel, _cancel.Token);
        }

        /// <summary>Panel gesture → C2S_POTENTIAL_ALLOCATE (all-or-
        /// nothing set; presenter passes the composed deltas).</summary>
        public Awaitable<bool> AllocatePotential(PotentialDelta deltas)
        {
            return _intents.RequestAllocate(deltas, _cancel.Token);
        }

        /// <summary>Panel gesture → C2S_RESPEC via the open NPC service
        /// session.</summary>
        public Awaitable<bool> Respec(string npcId, RespecKind kind)
        {
            return _intents.RequestRespec(npcId, kind, _cancel.Token);
        }

        /// <summary>Dispose on panel teardown — cancels in-flight
        /// sends.</summary>
        public void Dispose()
        {
            _cancel.Cancel();
            _cancel.Dispose();
        }

        /// <summary>progression.md EXP curve mirror for the panel bar —
        /// exp_required(L) = 10000·L² in the ×100 display scale; 0 past
        /// the level-60 cap. Display only, never a send input.</summary>
        public static long ExpToNext(int level)
        {
            if (level < 1 || level >= 60)
            {
                return 0;
            }
            return 10000L * level * level;
        }

        private string ResultText()
        {
            if (_applier.Results.Count == 0)
            {
                return string.Empty;
            }
            ProgressionApplier.Result r = _applier.Results[0];
            return r.Status == ResultStatus.Success
                ? "Lưu thành công"
                : r.Error.ToString();
        }
    }
}
