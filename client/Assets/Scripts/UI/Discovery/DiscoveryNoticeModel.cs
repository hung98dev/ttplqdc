namespace ThinhThan.UI.Discovery
{
    /// <summary>
    /// One first-discovery notice: the authored map id, the committed
    /// EXP grant, and the level the server reported after applying it
    /// (messages.md §506 DISCOVERY).
    /// </summary>
    public readonly struct DiscoveryNoticeModel
    {
        public DiscoveryNoticeModel(string mapId, long exp, int levelAfter, long serverTimeMs)
        {
            MapId = mapId;
            Exp = exp;
            LevelAfter = levelAfter;
            ServerTimeMs = serverTimeMs;
        }

        public string MapId { get; }

        public long Exp { get; }

        public int LevelAfter { get; }

        public long ServerTimeMs { get; }
    }
}
