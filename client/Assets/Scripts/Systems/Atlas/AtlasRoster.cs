using System.Collections.Generic;

namespace ThinhThan.Systems.Atlas
{
    /// <summary>
    /// Compiled Atlas roster mirror for display: page id, family,
    /// tier thresholds and the authored T3 title cosmetic id.
    /// Mirrors server/internal/durable/atlas/catalog.go — regenerate
    /// when docs/07_content/atlas_catalog.md changes. Seasonal pages
    /// share the id space; a 518 snapshot lists every authored page,
    /// so entries not in this table still render (thresholds zero).
    /// </summary>
    public static class AtlasRoster
    {
        public sealed class Entry
        {
            public string Id = "";
            public string Family = "";
            public uint[] Thresholds = {0, 0, 0};
            public string Title = "";
        }

        private static readonly Entry[] Entries =
        {
            new Entry { Id = "atlas.page.quai_dam.lang_da.dom_dom_ma", Family = "quai_dam", Thresholds = new uint[] {1, 10, 100}, Title = "cosmetic.title.atlas.dom_dom_ma" },
            new Entry { Id = "atlas.page.quai_dam.lang_da.hon_do_trang", Family = "quai_dam", Thresholds = new uint[] {1, 5, 30}, Title = "cosmetic.title.atlas.hon_do_trang" },
            new Entry { Id = "atlas.page.co_vat.ruong_co_01", Family = "co_vat", Thresholds = new uint[] {1, 10, 36}, Title = "cosmetic.title.atlas.kho_bau" },
            new Entry { Id = "atlas.page.co_vat.ca_bong", Family = "co_vat", Thresholds = new uint[] {1, 10, 50}, Title = "cosmetic.title.atlas.ca_bong2" },
            new Entry { Id = "atlas.page.co_vat.ca_bong_kho", Family = "co_vat", Thresholds = new uint[] {1, 10, 50}, Title = "cosmetic.title.atlas.bep_bong" },
            new Entry { Id = "atlas.page.di_tich.quy_nhap_trang", Family = "di_tich", Thresholds = new uint[] {1, 3, 10}, Title = "cosmetic.title.atlas.di_quy" },
            new Entry { Id = "atlas.page.hon_giam.dom_dom_ma", Family = "hon_giam", Thresholds = new uint[] {1, 3, 5}, Title = "cosmetic.title.atlas.hon_dom_dom" },
        };

        private static readonly Dictionary<string, Entry> ById =
            BuildIndex();

        private static Dictionary<string, Entry> BuildIndex()
        {
            var d = new Dictionary<string, Entry>();
            foreach (Entry e in Entries)
            {
                d[e.Id] = e;
            }
            return d;
        }

        /// <summary>Threshold lookup; unknown ids get a zero row.</summary>
        public static Entry For(string pageId)
        {
            if (ById.TryGetValue(pageId, out Entry e))
            {
                return e;
            }
            return new Entry { Id = pageId, Family = FamilyOf(pageId) };
        }

        /// <summary>Family segment of a page id (atlas.page.FAMILY.*).</summary>
        public static string FamilyOf(string pageId)
        {
            string[] parts = pageId.Split('.');
            return parts.Length >= 3 ? parts[2] : "";
        }
    }
}
