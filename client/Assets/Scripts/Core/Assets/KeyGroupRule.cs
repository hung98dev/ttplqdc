namespace ThinhThan.Core.Assets
{
    /// <summary>
    /// key → canonical-group assignment (client_assets.md § Grouping + § Stable
    /// Asset Keys): catalog-backed keys route by their catalog kind segment and
    /// embedded zone/dungeon/space identity; non-catalog keys route by kind and
    /// first name segment; `*.icon` facets of item/equipment/skill/cosmetic/
    /// status catalogs live in icons.shared and `*.bgm` facets route to the
    /// owning audio.bgm.* group. An unmapped key returns null and fails
    /// validation (§ Build/CI Validation).
    /// </summary>
    public static class KeyGroupRule
    {
        // Catalog kinds whose icon facet is atlased into icons.shared
        // ("item, equipment, skill, status and cosmetic icons").
        private static readonly string[] _iconCatalogKinds = { "item", "equipment", "skill", "status", "cosmetic" };

        // ui name segments that are boot-critical (bootstrap.local residency).
        private static readonly string[] _bootstrapUiNames = { "boot", "login", "update", "error", "splash" };

        /// <summary>The canonical group for a parsed key, or null when unmapped.</summary>
        public static string? Assign(AssetKey key)
        {
            return key.CatalogBacked ? AssignCatalog(key) : AssignNonCatalog(key);
        }

        private static string? AssignCatalog(AssetKey key)
        {
            var seg = key.CatalogId.Split('.');
            var kind = seg[0];

            if (key.Facet == AssetKey.KeyFacet.Bgm)
            {
                var spaceId = kind == "boss" ? AddressableGroups.BossSpace(key.CatalogId) : key.CatalogId;
                if (spaceId != null)
                {
                    var bgm = AddressableGroups.SpaceBgmGroup(spaceId);
                    if (bgm != null)
                    {
                        return bgm;
                    }
                }
                return AddressableGroups.AudioBgmShared;
            }

            if (key.Facet == AssetKey.KeyFacet.Icon && Contains(_iconCatalogKinds, kind))
            {
                return AddressableGroups.IconsShared;
            }

            switch (kind)
            {
                case "monster":
                case "npc":
                case "zone":
                    return seg.Length >= 2 && AddressableGroups.IsZoneKey(seg[1]) ? "region." + seg[1] : null;
                case "map":
                    return AddressableGroups.SpaceGroup(key.CatalogId);
                case "dungeon":
                    return seg.Length >= 2 && seg[1] == "finale"
                        ? AddressableGroups.DungeonFinale
                        : seg.Length >= 2 && AddressableGroups.IsDungeonKey(seg[1]) ? "dungeon." + seg[1] : null;
                case "instance":
                    return seg.Length >= 2 && seg[1] == "finale" ? AddressableGroups.DungeonFinale : null;
                case "boss":
                    var space = AddressableGroups.BossSpace(key.CatalogId);
                    return space != null ? AddressableGroups.SpaceGroup(space) : null;
                case "beast":
                    return AddressableGroups.BeastShared;
                case "cosmetic":
                    return AddressableGroups.CosmeticShared;
                case "class":
                case "skill":
                case "item":
                case "equipment":
                case "status":
                    return AddressableGroups.SharedLocal;
                default:
                    return null;
            }
        }

        private static string? AssignNonCatalog(AssetKey key)
        {
            var nameFirst = key.Name.Split('.')[0];
            switch (key.Kind)
            {
                case AssetKey.KeyKind.Ui:
                    return Contains(_bootstrapUiNames, nameFirst)
                        ? AddressableGroups.BootstrapLocal
                        : AddressableGroups.SharedLocal;
                case AssetKey.KeyKind.Font:
                    return AddressableGroups.BootstrapLocal;
                case AssetKey.KeyKind.Sfx:
                    return AddressableGroups.SharedLocal;
                case AssetKey.KeyKind.Bgm:
                    return AddressableGroups.IsZoneKey(nameFirst)
                        ? "audio.bgm." + nameFirst
                        : AddressableGroups.AudioBgmShared;
                case AssetKey.KeyKind.Prop:
                case AssetKey.KeyKind.Tile:
                case AssetKey.KeyKind.Parallax:
                    return RegionOrShared(nameFirst, AddressableGroups.SharedLocal);
                case AssetKey.KeyKind.Vfx:
                    if (nameFirst == "beast")
                    {
                        return AddressableGroups.BeastShared;
                    }
                    if (nameFirst == "cosmetic")
                    {
                        return AddressableGroups.CosmeticShared;
                    }
                    if (nameFirst == "pvp" || nameFirst == "guild_war")
                    {
                        return AddressableGroups.PvpShared;
                    }
                    return RegionOrShared(nameFirst, AddressableGroups.SharedLocal);
                default:
                    return null;
            }
        }

        private static string? RegionOrShared(string firstSegment, string fallback)
        {
            if (AddressableGroups.IsZoneKey(firstSegment))
            {
                return "region." + firstSegment;
            }
            if (AddressableGroups.IsDungeonKey(firstSegment))
            {
                return "dungeon." + firstSegment;
            }
            if (firstSegment == "finale")
            {
                return AddressableGroups.DungeonFinale;
            }
            return fallback;
        }

        private static bool Contains(string[] values, string value)
        {
            for (var i = 0; i < values.Length; i++)
            {
                if (values[i] == value)
                {
                    return true;
                }
            }
            return false;
        }
    }
}
