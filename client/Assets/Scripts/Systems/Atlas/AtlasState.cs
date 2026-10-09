using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Atlas
{
    /// <summary>
    /// Client mirror of the 518 snapshot: page models keyed by page id
    /// plus the latest atlas_revision. Apply-of-snapshot replaces the
    /// whole map — the server pushes the full self-snapshot after
    /// attach/reconnect and every committed change.
    /// </summary>
    public sealed class AtlasState
    {
        private readonly Dictionary<string, AtlasPageModel> _pages =
            new Dictionary<string, AtlasPageModel>();

        /// <summary>Latest atlas_revision carried by 518.</summary>
        public ulong Revision;

        /// <summary>Snapshot pages keyed by page id.</summary>
        public IReadOnlyDictionary<string, AtlasPageModel> Pages
        {
            get
            {
                return _pages;
            }
        }

        /// <summary>Replaces state from one 518 payload.</summary>
        public void ApplySnapshot(S2CAtlasState s)
        {
            Revision = s.AtlasRevision;
            _pages.Clear();
            foreach (AtlasPageView v in s.Pages)
            {
                var m = new AtlasPageModel
                {
                    PageId = v.AtlasPageId,
                    Family = AtlasRoster.FamilyOf(v.AtlasPageId),
                    Counter = v.Counter,
                    ReachedTier = v.ReachedTier,
                };
                AtlasRoster.Entry e = AtlasRoster.For(v.AtlasPageId);
                for (int i = 0; i < 3 && i < e.Thresholds.Length; i++)
                {
                    m.Thresholds[i] = e.Thresholds[i];
                }
                m.TitleCosmeticId = e.Title;
                foreach (AtlasTierView t in v.Tiers)
                {
                    int idx = (int)t.Tier - 1;
                    if (idx >= 0 && idx < 3)
                    {
                        m.TierAcknowledged[idx] =
                            t.AcknowledgedAtMs > 0;
                    }
                }
                _pages[m.PageId] = m;
            }
        }

        /// <summary>Marks one tier acknowledged locally (505 echo).</summary>
        public void MarkAcknowledged(string pageId, uint tier)
        {
            if (!_pages.TryGetValue(pageId, out AtlasPageModel m))
            {
                return;
            }
            for (int t = 0; t < ReachedTierOf(m) && t < 3; t++)
            {
                m.TierAcknowledged[t] = true;
            }
        }

        private static int ReachedTierOf(AtlasPageModel m)
        {
            return (int)m.ReachedTier;
        }
    }
}
