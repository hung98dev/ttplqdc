using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Party
{
    /// <summary>
    /// Authoritative party mirror (607 REPLACEABLE_STATE): full snapshot
    /// replace per frame — party_id, revision, leader, roster.
    /// </summary>
    public sealed class PartyState
    {
        private readonly List<PartyMemberModel> _members =
            new List<PartyMemberModel>();
        private byte[] _partyId = new byte[0];
        private byte[] _leaderCharacterId = new byte[0];

        public byte[] PartyId
        {
            get
            {
                return _partyId;
            }
        }

        public ulong PartyRevision
        {
            get;
            private set;
        }

        public byte[] LeaderCharacterId
        {
            get
            {
                return _leaderCharacterId;
            }
        }

        public bool InParty
        {
            get;
            private set;
        }

        public IReadOnlyList<PartyMemberModel> Members
        {
            get
            {
                return _members;
            }
        }

        public void Replace(S2CPartyState s)
        {
            _partyId = s.PartyId.ToByteArray();
            PartyRevision = s.PartyRevision;
            _leaderCharacterId = s.LeaderCharacterId.ToByteArray();
            InParty = _partyId.Length == 16;
            _members.Clear();
            foreach (PartyMemberView v in s.Members)
            {
                _members.Add(new PartyMemberModel(v));
            }
        }
    }
}
