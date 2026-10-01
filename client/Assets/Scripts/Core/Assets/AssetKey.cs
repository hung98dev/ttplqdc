using System;

namespace ThinhThan.Core.Assets
{
    /// <summary>
    /// Stable asset key parser (client_assets.md § Stable Asset Keys, ADR-0071):
    /// catalog-backed keys are `asset.&lt;catalog_id&gt;.&lt;facet&gt;` with the
    /// canonical content ID verbatim (PvP/Guild-War spaces use their space_id),
    /// non-catalog keys are `asset.&lt;kind&gt;.&lt;name&gt;.&lt;facet&gt;` with
    /// kind in {ui,sfx,bgm,font,prop,tile,parallax,vfx}, name = [a-z0-9_]+
    /// dot-joined and no variant segment.
    /// </summary>
    public readonly struct AssetKey
    {
        /// <summary>The kind segment of a non-catalog key; Catalog otherwise.</summary>
        public enum KeyKind
        {
            Ui = 0,
            Sfx = 1,
            Bgm = 2,
            Font = 3,
            Prop = 4,
            Tile = 5,
            Parallax = 6,
            Vfx = 7,
            Catalog = 8,
        }

        /// <summary>The terminal facet segment.</summary>
        public enum KeyFacet
        {
            Prefab = 0,
            Sprite = 1,
            Anim = 2,
            Scene = 3,
            Vfx = 4,
            Icon = 5,
            Portrait = 6,
            Bgm = 7,
            Clip = 8,
            Font = 9,
            Text = 10,
        }

        /// <summary>The Unity asset type a facet must resolve to.</summary>
        public enum ExpectedAssetType
        {
            GameObject = 0,
            SceneAsset = 1,
            Sprite = 2,
            AnimationClip = 3,
            AudioClip = 4,
            FontAsset = 5,
            TextAsset = 6,
            Any = 7,
        }

        public const string Prefix = "asset.";
        public const string ObsoleteCreditsKey = "asset.ui.credits.third_party_assets";

        private static readonly string[] _kindNames =
        {
            "ui", "sfx", "bgm", "font", "prop", "tile", "parallax", "vfx",
        };

        private static readonly string[] _facetNames =
        {
            "prefab", "sprite", "anim", "scene", "vfx", "icon", "portrait", "bgm", "clip", "font", "text",
        };

        /// <summary>The full key text.</summary>
        public string Raw
        {
            get;
        }

        /// <summary>True for `asset.&lt;catalog_id&gt;.&lt;facet&gt;` keys.</summary>
        public bool CatalogBacked
        {
            get;
        }

        /// <summary>The kind segment (non-catalog kind or Catalog).</summary>
        public KeyKind Kind
        {
            get;
        }

        /// <summary>The facet segment.</summary>
        public KeyFacet Facet
        {
            get;
        }

        /// <summary>The catalog_id verbatim; empty for non-catalog keys.</summary>
        public string CatalogId
        {
            get;
        }

        /// <summary>The kind segment of the catalog_id; empty for non-catalog keys.</summary>
        public string CatalogKind
        {
            get;
        }

        /// <summary>The dot-joined name for non-catalog keys; empty otherwise.</summary>
        public string Name
        {
            get;
        }

        private AssetKey(string raw, KeyKind kind, KeyFacet facet, string catalogId, string name)
        {
            Raw = raw;
            CatalogBacked = kind == KeyKind.Catalog;
            Kind = kind;
            Facet = facet;
            CatalogId = catalogId;
            CatalogKind = CatalogBacked ? catalogId.Substring(0, catalogId.IndexOf('.')) : string.Empty;
            Name = name;
        }

        public static bool TryParse(string? key, out AssetKey parsed)
        {
            return TryParse(key, out parsed, out _);
        }

        /// <summary>
        /// Parses a stable asset key. Rejects: wrong prefix, uppercase or
        /// non-ASCII characters, colons, empty/leading/trailing dot segments,
        /// too few segments, an unknown or missing facet (a variant segment is
        /// its own catalog ID or its own facet, never a suffix), and the
        /// obsolete `asset.ui.credits.third_party_assets` form.
        /// </summary>
        public static bool TryParse(string? key, out AssetKey parsed, out string error)
        {
            parsed = default;
            error = string.Empty;
            if (string.IsNullOrEmpty(key))
            {
                error = "empty key";
                return false;
            }
            if (key == ObsoleteCreditsKey)
            {
                error = "obsolete credits key; use asset.ui.credits.text";
                return false;
            }
            if (!key.StartsWith(Prefix, StringComparison.Ordinal))
            {
                error = "key must start with 'asset.'";
                return false;
            }
            foreach (var c in key)
            {
                if (!((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '.'))
                {
                    error = "invalid character '" + c + "' (keys are ASCII lowercase, digits, '_' and '.')";
                    return false;
                }
            }
            var segments = key.Split('.');
            if (segments.Length < 4)
            {
                error = "key needs at least asset.<id>.<facet> segments";
                return false;
            }
            for (var i = 0; i < segments.Length; i++)
            {
                if (segments[i].Length == 0)
                {
                    error = "empty segment";
                    return false;
                }
            }
            var facet = IndexOf(_facetNames, segments[segments.Length - 1]);
            if (facet < 0)
            {
                error = "last segment '" + segments[segments.Length - 1] + "' is not a facet (no variant segment allowed)";
                return false;
            }
            var midLength = segments.Length - 2;
            var kindIndex = IndexOf(_kindNames, segments[1]);
            if (kindIndex >= 0)
            {
                var name = string.Join(".", segments, 2, midLength - 1);
                parsed = new AssetKey(key, (KeyKind)kindIndex, (KeyFacet)facet, string.Empty, name);
                return true;
            }
            if (midLength < 2)
            {
                error = "catalog_id needs at least a kind and name segment";
                return false;
            }
            var catalogId = string.Join(".", segments, 1, midLength);
            parsed = new AssetKey(key, KeyKind.Catalog, (KeyFacet)facet, catalogId, string.Empty);
            return true;
        }

        /// <summary>The Unity asset type an entry under this facet must be.</summary>
        public ExpectedAssetType ExpectedType()
        {
            switch (Facet)
            {
                case KeyFacet.Prefab:
                case KeyFacet.Vfx:
                    return ExpectedAssetType.GameObject;
                case KeyFacet.Scene:
                    return ExpectedAssetType.SceneAsset;
                case KeyFacet.Sprite:
                case KeyFacet.Icon:
                case KeyFacet.Portrait:
                    return ExpectedAssetType.Sprite;
                case KeyFacet.Anim:
                    return ExpectedAssetType.AnimationClip;
                case KeyFacet.Bgm:
                case KeyFacet.Clip:
                    return ExpectedAssetType.AudioClip;
                case KeyFacet.Font:
                    return ExpectedAssetType.FontAsset;
                case KeyFacet.Text:
                    return ExpectedAssetType.TextAsset;
                default:
                    return ExpectedAssetType.Any;
            }
        }

        private static int IndexOf(string[] names, string value)
        {
            for (var i = 0; i < names.Length; i++)
            {
                if (names[i] == value)
                {
                    return i;
                }
            }
            return -1;
        }

        public override string ToString()
        {
            return Raw;
        }
    }
}
