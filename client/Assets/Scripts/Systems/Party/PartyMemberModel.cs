using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Party
{
    /// <summary>One roster row of the authoritative party mirror.</summary>
    public sealed class PartyMemberModel
    {
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

        public OnlineState OnlineState
        {
            get;
        }

        public string ZoneId
        {
            get;
        }

        public PartyMemberModel(PartyMemberView v)
        {
            CharacterId = v.CharacterId.ToByteArray();
            DisplayName = v.DisplayName;
            ClassId = v.ClassId;
            Level = v.Level;
            OnlineState = v.OnlineState;
            ZoneId = v.ZoneId;
        }
    }
}
