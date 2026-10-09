using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Guild
{
    /// <summary>
    /// Client mirror of GuildProgressionView (628 field 11): guild EXP,
    /// streak, the open cycle's five element vessels, candidates and
    /// vote state, plus the receiver's contribution rollups.
    /// </summary>
    public sealed class GuildProgressionModel
    {
        private readonly List<ElementPoint> _points =
            new List<ElementPoint>(5);
        private readonly List<string> _candidates = new List<string>();
        private readonly List<uint> _voteCounts = new List<uint>();

        public sealed class ElementPoint
        {
            public ElementPoint(Element element, uint current)
            {
                Element = element;
                Current = current;
            }

            public Element Element
            {
                get;
            }

            public uint Current
            {
                get;
            }
        }

        public ulong GuildExp
        {
            get;
            private set;
        }

        public uint RitualStreak
        {
            get;
            private set;
        }

        private string _cycleId = string.Empty;
        public string CycleId
        {
            get
            {
                return _cycleId;
            }
            private set
            {
                _cycleId = value;
            }
        }

        public uint MEffective
        {
            get;
            private set;
        }

        public uint RequiredPerElement
        {
            get;
            private set;
        }

        public IReadOnlyList<ElementPoint> Points
        {
            get
            {
                return _points;
            }
        }

        /// <summary>Cycle completed_at; 0 when open.</summary>
        public long CompletedAtMs
        {
            get;
            private set;
        }

        /// <summary>0 or 3 blessing candidates for the open draft.</summary>
        public IReadOnlyList<string> Candidates
        {
            get
            {
                return _candidates;
            }
        }

        public long VoteClosesAtMs
        {
            get;
            private set;
        }

        /// <summary>Vote counts aligned with <see cref="Candidates"/>.</summary>
        public IReadOnlyList<uint> VoteCounts
        {
            get
            {
                return _voteCounts;
            }
        }

        /// <summary>Receiver's cast blessing id; empty = none.</summary>
        private string _receiverVote = string.Empty;
        public string ReceiverVote
        {
            get
            {
                return _receiverVote;
            }
            private set
            {
                _receiverVote = value;
            }
        }

        private string _activeBlessingId = string.Empty;
        public string ActiveBlessingId
        {
            get
            {
                return _activeBlessingId;
            }
            private set
            {
                _activeBlessingId = value;
            }
        }

        public long BlessingExpiresAtMs
        {
            get;
            private set;
        }

        public ulong ReceiverLifetimeContribution
        {
            get;
            private set;
        }

        public ulong ReceiverCycleContribution
        {
            get;
            private set;
        }

        public void Replace(GuildProgressionView v)
        {
            GuildExp = v.GuildExp;
            RitualStreak = v.RitualStreak;
            CycleId = v.CycleId;
            MEffective = v.MEffective;
            RequiredPerElement = v.RequiredPointsPerElement;
            _points.Clear();
            foreach (GuildProgressionPoint p in v.Points)
            {
                _points.Add(new ElementPoint(p.Element, p.Current));
            }
            CompletedAtMs = v.CompletedAtMs;
            _candidates.Clear();
            _candidates.AddRange(v.CandidateBlessingIds);
            VoteClosesAtMs = v.VoteClosesAtMs;
            _voteCounts.Clear();
            _voteCounts.AddRange(v.VoteCounts);
            ReceiverVote = v.ReceiverVote;
            ActiveBlessingId = v.ActiveBlessingId;
            BlessingExpiresAtMs = v.BlessingExpiresAtMs;
            ReceiverLifetimeContribution = v.ReceiverLifetimeContribution;
            ReceiverCycleContribution = v.ReceiverCycleContribution;
        }
    }
}
