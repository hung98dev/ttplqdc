namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// HUD top-right world-context seam (F-8): map id / channel / display
    /// name sourced from baseline + transfer-prepare fields; production
    /// routing lands in composition, tests fake.
    /// </summary>
    public interface IWorldContextFeed
    {
        /// <summary>Current map_id ("" when unknown).</summary>
        string MapId
        {
            get;
        }

        /// <summary>Current channel_index (0 = instances).</summary>
        uint ChannelIndex
        {
            get;
        }

        /// <summary>content_revision of the active world.</summary>
        string ContentRevision
        {
            get;
        }

        /// <summary>Localized display name (falls back to MapId).</summary>
        string DisplayName
        {
            get;
        }
    }
}
