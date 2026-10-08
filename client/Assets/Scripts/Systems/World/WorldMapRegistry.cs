using System.Collections.Generic;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Geometry;

namespace ThinhThan.Systems.World
{
    /// <summary>
    /// Static registry of the 24 normal-world maps
    /// (07_content/world_route_catalog.md): map_id -> zone key, the
    /// Addressables <c>region.&lt;zone&gt;</c> group that owns the map's
    /// content, the display-name localization key and the geometry asset
    /// path. PvP/Guild-War and dungeon spaces are out of scope — their
    /// packets own their own registries.
    /// </summary>
    public static class WorldMapRegistry
    {
        /// <summary>One registered world map.</summary>
        public readonly struct MapRecord
        {
            public MapRecord(string mapId, string zoneKey, string groupKey, string displayNameKey, string geometryAssetPath)
            {
                MapId = mapId;
                ZoneKey = zoneKey;
                GroupKey = groupKey;
                DisplayNameKey = displayNameKey;
                GeometryAssetPath = geometryAssetPath;
            }

            /// <summary>Canonical <c>map.&lt;zone&gt;.&lt;name&gt;</c> id.</summary>
            public string MapId
            {
                get;
            }

            /// <summary>Zone segment of the map_id (e.g. <c>lang_da</c>).</summary>
            public string ZoneKey
            {
                get;
            }

            /// <summary>Owning Addressables group (<c>region.&lt;zone&gt;</c>).</summary>
            public string GroupKey
            {
                get;
            }

            /// <summary>Localization key for the map's display name.</summary>
            public string DisplayNameKey
            {
                get;
            }

            /// <summary>
            /// Canonical geometry asset path
            /// (<c>Data/Geometry/&lt;map_id&gt;.geom.json</c> — the exported
            /// collision document produced from the map's Collision scene).
            /// </summary>
            public string GeometryAssetPath
            {
                get;
            }
        }

        private static readonly string[] MapIds =
        {
            "map.lang_da.dinh_lang",
            "map.lang_da.bo_ruong",
            "map.lang_da.ben_da",
            "map.lang_da.go_ma",
            "map.rung_u_minh.xom_rung",
            "map.rung_u_minh.loi_tram",
            "map.rung_u_minh.rung_sau",
            "map.rung_u_minh.mieu_bo_hoang",
            "map.ben_nuoc_den.cho_ben",
            "map.ben_nuoc_den.bai_lau",
            "map.ben_nuoc_den.duong_ngap",
            "map.ben_nuoc_den.ben_do_cu",
            "map.deo_may.ban_chan_deo",
            "map.deo_may.duong_rung",
            "map.deo_may.khe_da",
            "map.deo_may.rung_cam",
            "map.thanh_co.cong_ngoai",
            "map.thanh_co.duong_da",
            "map.thanh_co.hao_can",
            "map.thanh_co.den_tran",
            "map.nui_thieng.chan_nui",
            "map.nui_thieng.rung_may",
            "map.nui_thieng.suon_da",
            "map.nui_thieng.cong_co",
        };

        private static readonly Dictionary<string, MapRecord> _records = Build();

        private static Dictionary<string, MapRecord> Build()
        {
            var records = new Dictionary<string, MapRecord>(MapIds.Length);
            foreach (var mapId in MapIds)
            {
                var seg = mapId.Split('.');
                var zoneKey = seg[1];
                var groupKey = AddressableGroups.SpaceGroup(mapId) ?? "region." + zoneKey;
                records[mapId] = new MapRecord(
                    mapId,
                    zoneKey,
                    groupKey,
                    "loc.world_map." + zoneKey + "." + seg[2],
                    "Data/Geometry/" + mapId + ".geom.json");
            }
            return records;
        }

        /// <summary>All 24 map records in catalog order.</summary>
        public static IReadOnlyCollection<MapRecord> All
        {
            get
            {
                return _records.Values;
            }
        }

        /// <summary>Number of registered world maps (24).</summary>
        public static int Count
        {
            get
            {
                return _records.Count;
            }
        }

        /// <summary>Looks up a map_id; false for unknown/dungeon/pvp ids.</summary>
        public static bool TryGet(string mapId, out MapRecord record)
        {
            if (mapId != null && _records.TryGetValue(mapId, out var r))
            {
                record = r;
                return true;
            }
            record = default;
            return false;
        }

        /// <summary>
        /// The scroll region the camera may cover on this map: the union of
        /// the geometry's camera regions, clamped to the map bounds
        /// (geometry units; BoundsMaxX/Y when no regions are declared).
        /// </summary>
        public static void CameraBounds(GeometryData geometry, out long minX, out long minY, out long maxX, out long maxY)
        {
            if (geometry.CameraRegions.Length == 0)
            {
                minX = 0;
                minY = 0;
                maxX = geometry.BoundsMaxX;
                maxY = geometry.BoundsMaxY;
                return;
            }
            minX = long.MaxValue;
            minY = long.MaxValue;
            maxX = long.MinValue;
            maxY = long.MinValue;
            for (var i = 0; i < geometry.CameraRegions.Length; i++)
            {
                var r = geometry.CameraRegions[i];
                if (r.MinX < minX)
                {
                    minX = r.MinX;
                }
                if (r.MinY < minY)
                {
                    minY = r.MinY;
                }
                if (r.MaxX > maxX)
                {
                    maxX = r.MaxX;
                }
                if (r.MaxY > maxY)
                {
                    maxY = r.MaxY;
                }
            }
            if (minX < 0)
            {
                minX = 0;
            }
            if (minY < 0)
            {
                minY = 0;
            }
            if (maxX > geometry.BoundsMaxX)
            {
                maxX = geometry.BoundsMaxX;
            }
            if (maxY > geometry.BoundsMaxY)
            {
                maxY = geometry.BoundsMaxY;
            }
        }
    }
}
