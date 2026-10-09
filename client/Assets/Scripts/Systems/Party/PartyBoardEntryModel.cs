using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Party
{
    /// <summary>One safe-anchor board entry of the 636 mirror.</summary>
    public sealed class PartyBoardEntryModel
    {
        public byte[] PostId
        {
            get;
        }

        public byte[] PosterCharacterId
        {
            get;
        }

        public string DisplayName
        {
            get;
        }

        public string DungeonId
        {
            get;
        }

        public uint DesiredSize
        {
            get;
        }

        public uint ExpiresInSeconds
        {
            get;
        }

        public PartyBoardEntryModel(PartyBoardEntry v)
        {
            PostId = v.PostId.ToByteArray();
            PosterCharacterId = v.PosterCharacterId.ToByteArray();
            DisplayName = v.DisplayName;
            DungeonId = v.DungeonId;
            DesiredSize = v.DesiredSize;
            ExpiresInSeconds = v.ExpiresInSeconds;
        }
    }

