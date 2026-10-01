using System.Collections.Generic;

namespace ThinhThan.Core.Assets
{
    /// <summary>
    /// The canonical Addressables group set (client_assets.md § Grouping,
    /// ADR-0071/ADR-0074): exactly 29 names. Download/RAM budgets and residency
    /// are canonical in presentation_asset_manifest.md §1.
    /// </summary>
    public static class AddressableGroups
    {
        /// <summary>Residency class per presentation_asset_manifest.md §1.</summary>
        public enum GroupKind
        {
            /// <summary>Built into the player; local build/load profile paths.</summary>
            Local = 0,

            /// <summary>May be delivered remotely; remote build/load profile paths.</summary>
            Remote = 1,
        }

        public const int CanonicalGroupCount = 29;
        public const string BootstrapLocal = "bootstrap.local";
        public const string SharedLocal = "shared.local";
        public const string IconsShared = "icons.shared";
        public const string BeastShared = "beast.shared";
        public const string CosmeticShared = "cosmetic.shared";
        public const string PvpShared = "pvp.shared";
        public const string AudioBgmShared = "audio.bgm.shared";
        public const string DungeonFinale = "dungeon.finale";
        public const string LocalizationLocales = "localization.locales";
        public const string LocalizationShared = "localization.shared";
        public const string LocalizationStringsPrefix = "localization.strings.";
        public const string LocalizationStringsViVn = "localization.strings.vi_vn";
        public const string LocalizationStringsEnUs = "localization.strings.en_us";

        /// <summary>Zone keys from ../07_content/encounter_catalog.md (6).</summary>
        public static readonly string[] ZoneKeys =
        {
            "lang_da",
            "rung_u_minh",
            "ben_nuoc_den",
            "deo_may",
            "thanh_co",
            "nui_thieng",
        };

        /// <summary>Dungeon keys from ../07_content/dungeon_catalog.md (5).</summary>
        public static readonly string[] DungeonKeys =
        {
            "dinh_lang_bo_hoang",
            "mieu_ba_trong_rung",
            "xom_chim",
            "hang_ma_tranh",
            "den_tran",
        };

        // boss_id -> space_id, transcribed from ../07_content/boss_catalog.md
        // (every boss has one canonical space_id).
        private static readonly Dictionary<string, string> _bossSpaces = new Dictionary<string, string>
        {
            ["boss.quy_nhap_trang"] = "dungeon.dinh_lang_bo_hoang",
            ["boss.moc_tinh_da"] = "dungeon.mieu_ba_trong_rung",
            ["boss.thuong_luong"] = "dungeon.xom_chim",
            ["boss.ma_da_chua"] = "map.ben_nuoc_den.ben_do_cu",
            ["boss.ho_tinh"] = "dungeon.hang_ma_tranh",
            ["boss.ho_tinh_chin_duoi"] = "dungeon.den_tran",
            ["boss.ngu_tinh"] = "map.nui_thieng.suon_da",
            ["boss.than_trung"] = "instance.finale.than_trung",
        };

        // dungeon_key -> owning zone_key (dungeons live inside regions).
        private static readonly Dictionary<string, string> _dungeonZones = new Dictionary<string, string>
        {
            ["dinh_lang_bo_hoang"] = "lang_da",
            ["mieu_ba_trong_rung"] = "rung_u_minh",
            ["xom_chim"] = "ben_nuoc_den",
            ["hang_ma_tranh"] = "deo_may",
            ["den_tran"] = "thanh_co",
            ["finale"] = "nui_thieng",
        };

        private static readonly string[] _canonicalNames = BuildCanonicalNames();

        private static string[] BuildCanonicalNames()
        {
            var names = new List<string>(CanonicalGroupCount)
            {
                BootstrapLocal,
                SharedLocal,
                IconsShared,
                BeastShared,
                CosmeticShared,
                PvpShared,
                AudioBgmShared,
            };
            foreach (var zone in ZoneKeys)
            {
                names.Add("audio.bgm." + zone);
            }
            foreach (var zone in ZoneKeys)
            {
                names.Add("region." + zone);
            }
            foreach (var dungeon in DungeonKeys)
            {
                names.Add("dungeon." + dungeon);
            }
            names.Add(DungeonFinale);
            names.Add(LocalizationLocales);
            names.Add(LocalizationShared);
            names.Add(LocalizationStringsViVn);
            names.Add(LocalizationStringsEnUs);
            return names.ToArray();
        }

        /// <summary>The 29 canonical group names in canonical order.</summary>
        public static IReadOnlyList<string> CanonicalNames()
        {
            return _canonicalNames;
        }

        /// <summary>Whether name is one of the 29 canonical group names.</summary>
        public static bool IsCanonicalName(string name)
        {
            for (var i = 0; i < _canonicalNames.Length; i++)
            {
                if (_canonicalNames[i] == name)
                {
                    return true;
                }
            }
            return false;
        }

        /// <summary>
        /// Residency of a canonical group (manifest §1): bootstrap.local,
        /// shared.local and every localization.* group are built into the
        /// player; all other groups may be remote.
        /// </summary>
        public static GroupKind KindOf(string name)
        {
            if (name == BootstrapLocal || name == SharedLocal || name.StartsWith("localization.", System.StringComparison.Ordinal))
            {
                return GroupKind.Local;
            }
            return GroupKind.Remote;
        }

        /// <summary>Whether name is a package-managed localization.* group.</summary>
        public static bool IsLocalizationGroup(string name)
        {
            return name.StartsWith("localization.", System.StringComparison.Ordinal);
        }

        public static bool IsZoneKey(string key)
        {
            for (var i = 0; i < ZoneKeys.Length; i++)
            {
                if (ZoneKeys[i] == key)
                {
                    return true;
                }
            }
            return false;
        }

        public static bool IsDungeonKey(string key)
        {
            for (var i = 0; i < DungeonKeys.Length; i++)
            {
                if (DungeonKeys[i] == key)
                {
                    return true;
                }
            }
            return false;
        }

        /// <summary>
        /// The group that owns a playable space_id
        /// (physics_geometry_contract.md §6.1): zone maps to their region,
        /// dungeons to their dungeon group, the finale to dungeon.finale and
        /// PvP/Guild-War spaces to pvp.shared.
        /// </summary>
        public static string? SpaceGroup(string spaceId)
        {
            var seg = spaceId.Split('.');
            if (seg.Length < 2)
            {
                return null;
            }
            if (seg[0] == "map")
            {
                if (seg[1] == "pvp" || seg[1] == "guild_war")
                {
                    return PvpShared;
                }
                return IsZoneKey(seg[1]) ? "region." + seg[1] : null;
            }
            if (seg[0] == "dungeon")
            {
                if (seg[1] == "finale")
                {
                    return DungeonFinale;
                }
                return IsDungeonKey(seg[1]) ? "dungeon." + seg[1] : null;
            }
            if (seg[0] == "instance")
            {
                return seg[1] == "finale" ? DungeonFinale : null;
            }
            return null;
        }

        /// <summary>
        /// The audio.bgm.* group that streams a space's BGM: zone maps and
        /// dungeons use their owning region's group; competitive and shared
        /// music use audio.bgm.shared.
        /// </summary>
        public static string? SpaceBgmGroup(string spaceId)
        {
            var seg = spaceId.Split('.');
            if (seg.Length < 2)
            {
                return null;
            }
            if (seg[0] == "map")
            {
                if (seg[1] == "pvp" || seg[1] == "guild_war")
                {
                    return AudioBgmShared;
                }
                return IsZoneKey(seg[1]) ? "audio.bgm." + seg[1] : null;
            }
            if (seg[0] == "dungeon")
            {
                return _dungeonZones.TryGetValue(seg[1], out var zone) ? "audio.bgm." + zone : null;
            }
            if (seg[0] == "instance" && seg[1] == "finale")
            {
                return "audio.bgm." + _dungeonZones["finale"];
            }
            return null;
        }

        /// <summary>The canonical space_id of a boss (boss_catalog.md), or null.</summary>
        public static string? BossSpace(string bossId)
        {
            return _bossSpaces.TryGetValue(bossId, out var space) ? space : null;
        }

        /// <summary>The owning zone_key of a dungeon_key, or null.</summary>
        public static string? DungeonZone(string dungeonKey)
        {
            return _dungeonZones.TryGetValue(dungeonKey, out var zone) ? zone : null;
        }
    }
}
