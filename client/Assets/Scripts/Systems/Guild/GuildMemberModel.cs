using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Guild
{
    /// <summary>
    /// Client view of one roster row from S2C_GUILD_STATE(628):
    /// character id, display name, class, level, role content-id and
    /// the online projection (online flag + last_online_at offline-only).
    /// </summary>
    public sealed class GuildMemberModel
    {
        public GuildMemberModel(GuildMemberView v)
        {
            CharacterId = v.CharacterId.ToByteArray();
            DisplayName = v.DisplayName;
            ClassId = v.ClassId;
            Level = v.Level;
            Role = v.Role;
            Online = v.OnlineState == OnlineState.Online;
            LastOnlineAt = v.LastOnlineAt;
        }

        public byte[] CharacterId
        {
            get;
        }

        public string DisplayName
        {
            get;
        }

        public string ClassId
        {
            get;
        }

        public uint Level
        {
            get;
        }

        /// <summary>guild.role.* content id.</summary>
        public string Role
        {
            get;
        }

        public bool Online
        {
            get;
        }

        /// <summary>Unix ms; meaningful only when <see cref="Online"/>
        /// is false.</summary>
        public long LastOnlineAt
        {
            get;
        }
    }
}
