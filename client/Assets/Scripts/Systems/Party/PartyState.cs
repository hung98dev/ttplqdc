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

        public byte[] PartyId
        {
            get;
            private set;
        } = new byte[0];

        public ulong PartyRevision
        {
            get;
            private set;
        }

        public byte[] LeaderCharacterId
        {
            get;
            private set;
        } = new byte[0];

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
            PartyId = s.PartyId.ToByteArray();
            PartyRevision = s.PartyRevision;
            LeaderCharacterId = s.LeaderCharacterId.ToByteArray();
            InParty = PartyId.Length == 16;
            _members.Clear();
            foreach (PartyMemberView v in s.Members)
            {
                _members.Add(new PartyMemberModel(v));
            }
        }
    }
}
