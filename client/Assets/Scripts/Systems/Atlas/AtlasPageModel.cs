namespace ThinhThan.Systems.Atlas
{
    /// <summary>
    /// One Atlas page's client view: counter progress against the
    /// authored thresholds plus reached/acknowledged tier state.
    /// </summary>
    public sealed class AtlasPageModel
    {
        public string PageId = "";
        public string Family = "";
        public ulong Counter;
        public uint ReachedTier;
        public readonly uint[] Thresholds = new uint[3];
        public string TitleCosmeticId = "";
        public readonly bool[] TierAcknowledged = new bool[3];

        /// <summary>Display name of the reached tier.</summary>
        public AtlasTierName TierName
        {
            get
            {
                if (ReachedTier >= 3)
                {
                    return AtlasTierName.Mastered;
                }
                if (ReachedTier == 2)
                {
                    return AtlasTierName.Studied;
                }
                if (ReachedTier == 1)
                {
                    return AtlasTierName.Seen;
                }
                return AtlasTierName.Locked;
            }
        }

        /// <summary>
        /// Progress text for the current frontier ("7 / 10"), or
        /// "Mastered" once the top tier is reached.
        /// </summary>
        public string ProgressText()
        {
            if (ReachedTier >= 3)
            {
                return "Mastered";
            }
            ulong next = Thresholds[ReachedTier];
            return Counter + " / " + next;
        }

        /// <summary>
        /// Journal "new" marker: a reached tier not yet acknowledged
        /// (cleared when the page-level acknowledgement lands).
        /// </summary>
        public bool HasUnacknowledgedTier()
        {
            for (int t = 0; t < ReachedTier && t < 3; t++)
            {
                if (!TierAcknowledged[t])
                {
                    return true;
                }
            }
            return false;
        }
    }
}
