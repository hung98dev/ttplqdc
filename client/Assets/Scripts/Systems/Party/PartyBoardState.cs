using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Party
{
    /// <summary>
    /// Authoritative board mirror (636 REPLACEABLE_STATE): full snapshot
    /// replace per frame.
    /// </summary>
    public sealed class PartyBoardState
    {
        private readonly List<PartyBoardEntryModel> _entries =
            new List<PartyBoardEntryModel>();

        public IReadOnlyList<PartyBoardEntryModel> Entries
        {
            get
            {
                return _entries;
            }
        }

        public void Replace(S2CPartyBoardState s)
        {
            _entries.Clear();
            foreach (PartyBoardEntry v in s.Entries)
            {
                _entries.Add(new PartyBoardEntryModel(v));
            }
        }
    }
}
