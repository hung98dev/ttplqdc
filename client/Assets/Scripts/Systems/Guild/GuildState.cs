using System.Collections.Generic;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Guild
{
    /// <summary>
    /// Authoritative guild mirror (628 REPLACEABLE_STATE): full snapshot
    /// replace per frame — identity, receiver role, roster, progression,
    /// owned cosmetics.
    /// </summary>
    public sealed class GuildState
    {
        private readonly List<GuildMemberModel> _members =
            new List<GuildMemberModel>();
        private readonly List<string> _ownedCosmeticIds =
            new List<string>();
        private readonly GuildProgressionModel _progression =
            new GuildProgressionModel();
        private byte[] _guildId = new byte[0];

        public byte[] GuildId
        {
            get
            {
                return _guildId;
            }
        }

        public ulong GuildRevision
        {
            get;
            private set;
        }

        private string _guildName = string.Empty;
        public string GuildName
        {
            get
            {
                return _guildName;
            }
            private set
            {
                _guildName = value;
            }
        }

        /// <summary>Receiver's role (guild.role.* content id).</summary>
        private string _role = string.Empty;
        public string Role
        {
            get
            {
                return _role;
            }
            private set
            {
                _role = value;
            }
        }

        public uint Level
        {
            get;
            private set;
        }

        public uint MembersCount
        {
            get;
            private set;
        }

        public uint MaxMembers
        {
            get;
            private set;
        }

        private string _motd = string.Empty;
        public string Motd
        {
            get
            {
                return _motd;
            }
            private set
            {
                _motd = value;
            }
        }

        public GuildRecruitmentMode RecruitmentMode
        {
            get;
            private set;
        }

        public IReadOnlyList<GuildMemberModel> Members
        {
            get
            {
                return _members;
            }
        }

        public GuildProgressionModel Progression
        {
            get
            {
                return _progression;
            }
        }

        public IReadOnlyList<string> OwnedCosmeticIds
        {
            get
            {
                return _ownedCosmeticIds;
            }
        }

        public ulong CosmeticRevision
        {
            get;
            private set;
        }

        public bool InGuild
        {
            get
            {
                return _guildId.Length == 16;
            }
        }

        public void Replace(S2CGuildState s)
        {
            _guildId = s.GuildId.ToByteArray();
            GuildRevision = s.GuildRevision;
            GuildName = s.GuildName;
            Role = s.Role;
            Level = s.Level;
            MembersCount = s.MembersCount;
            MaxMembers = s.MaxMembers;
            Motd = s.Motd;
            RecruitmentMode = s.RecruitmentMode;
            _members.Clear();
            foreach (GuildMemberView v in s.Members)
            {
                _members.Add(new GuildMemberModel(v));
            }
            _progression.Replace(s.Progression);
            _ownedCosmeticIds.Clear();
            _ownedCosmeticIds.AddRange(s.OwnedGuildCosmeticIds);
            CosmeticRevision = s.CosmeticRevision;
        }
    }
}
