using System.Collections.Generic;

namespace ThinhThan.Core.Assets
{
    /// <summary>
    /// Canonical sprite import constants (presentation_asset_manifest.md §3,
    /// §3.1; ADR-0046/0055/0071): textures authored at 2x, imported at 100 PPU
    /// (UI 200, PARALLAX_FAR 50), pivot Bottom Center (0.5, 0), Filter
    /// Bilinear, mesh type Tight iff the texture long side is >= 256 texture
    /// px with transparent margins else Full Rect, Generate Physics Shape
    /// false, Mip Maps false for gameplay sprites, Transform scale (1,1,1).
    /// </summary>
    public static class SpriteImportRules
    {
        /// <summary>The manifest §3 size_profile enum.</summary>
        public enum SizeProfile
        {
            Character = 0,
            MonsterSmall = 1,
            MonsterMedium = 2,
            MonsterElite = 3,
            BossLarge = 4,
            WorldBoss = 5,
            SpiritBeast = 6,
            NpcHumanoid = 7,
        }

        /// <summary>One row of the manifest §3 size_profile table (px).</summary>
        public sealed class SizeProfileInfo
        {
            public SizeProfile Profile
            {
                get;
            }
            public int SilhouetteW
            {
                get;
            }
            public int SilhouetteH
            {
                get;
            }
            public int CellW
            {
                get;
            }
            public int CellH
            {
                get;
            }
            public int TextureW
            {
                get;
            }
            public int TextureH
            {
                get;
            }
            public int ColliderW
            {
                get;
            }
            public int ColliderH
            {
                get;
            }

            public SizeProfileInfo(
                SizeProfile profile,
                int silhouetteW, int silhouetteH,
                int cellW, int cellH,
                int textureW, int textureH,
                int colliderW, int colliderH)
            {
                Profile = profile;
                SilhouetteW = silhouetteW;
                SilhouetteH = silhouetteH;
                CellW = cellW;
                CellH = cellH;
                TextureW = textureW;
                TextureH = textureH;
                ColliderW = colliderW;
                ColliderH = colliderH;
            }
        }

        /// <summary>Reference resolution: ART_PIXELS_PER_METER (ADR-0046).</summary>
        public const int ReferencePixelsPerMeter = 50;

        /// <summary>TEXTURE_SCALE: texture px = 2 x reference px.</summary>
        public const int TextureScale = 2;

        /// <summary>Pixels Per Unit for gameplay sprites.</summary>
        public const int GameplayPpu = 100;

        /// <summary>Pixels Per Unit for UI sprites.</summary>
        public const int UiPpu = 200;

        /// <summary>Pixels Per Unit for PARALLAX_FAR (authored 1x).</summary>
        public const int ParallaxFarPpu = 50;

        /// <summary>Pivot Bottom Center.</summary>
        public const float PivotX = 0.5f;
        public const float PivotY = 0.0f;

        /// <summary>Manifest §3.1: icon cell is 64x64 reference px.</summary>
        public const int IconCellRef = 64;

        /// <summary>Manifest §3.1: tile cell is 50x50 reference px.</summary>
        public const int TileCellRef = 50;

        /// <summary>UI reference plane (1280x720 ref -> 2560x1440 texture).</summary>
        public const int UiPlaneRefWidth = 1280;
        public const int UiPlaneRefHeight = 720;

        /// <summary>Mesh type Tight requires a texture long side >= this many texture px.</summary>
        public const int TightMeshMinLongSide = 256;

        /// <summary>cell_ref ceiling for PROP/VFX (reference px, multiple of 16).</summary>
        public const int MaxCellRef = 512;

        private static readonly Dictionary<SizeProfile, SizeProfileInfo> _profiles = BuildProfiles();

        private static Dictionary<SizeProfile, SizeProfileInfo> BuildProfiles()
        {
            return new Dictionary<SizeProfile, SizeProfileInfo>
            {
                [SizeProfile.Character] = new SizeProfileInfo(SizeProfile.Character, 64, 96, 96, 128, 192, 256, 40, 90),
                [SizeProfile.MonsterSmall] = new SizeProfileInfo(SizeProfile.MonsterSmall, 50, 50, 64, 64, 128, 128, 30, 30),
                [SizeProfile.MonsterMedium] = new SizeProfileInfo(SizeProfile.MonsterMedium, 75, 100, 96, 128, 192, 256, 50, 70),
                [SizeProfile.MonsterElite] = new SizeProfileInfo(SizeProfile.MonsterElite, 125, 150, 160, 192, 320, 384, 80, 120),
                [SizeProfile.BossLarge] = new SizeProfileInfo(SizeProfile.BossLarge, 200, 220, 256, 256, 512, 512, 120, 160),
                [SizeProfile.WorldBoss] = new SizeProfileInfo(SizeProfile.WorldBoss, 250, 280, 320, 320, 640, 640, 150, 200),
                [SizeProfile.SpiritBeast] = new SizeProfileInfo(SizeProfile.SpiritBeast, 48, 48, 64, 64, 128, 128, 0, 0),
                [SizeProfile.NpcHumanoid] = new SizeProfileInfo(SizeProfile.NpcHumanoid, 64, 96, 96, 128, 192, 256, 0, 0),
            };
        }

        /// <summary>Every declared size_profile row.</summary>
        public static IReadOnlyDictionary<SizeProfile, SizeProfileInfo> Profiles
        {
            get
            {
                return _profiles;
            }
        }

        public static SizeProfileInfo InfoFor(SizeProfile profile)
        {
            return _profiles[profile];
        }

        /// <summary>
        /// Mesh type rule (manifest §3): Tight iff the texture long side is
        /// >= 256 texture px AND the texture has transparent margins (alpha 0
        /// at any edge); otherwise Full Rect.
        /// </summary>
        public static bool RequiresTightMesh(int textureLongSidePx, bool hasTransparentMargin)
        {
            return textureLongSidePx >= TightMeshMinLongSide && hasTransparentMargin;
        }

        /// <summary>texture px = 2 x reference px.</summary>
        public static int TexturePixels(int referencePx)
        {
            return referencePx * TextureScale;
        }

        /// <summary>PPU for the sprite class: gameplay 100, UI 200, PARALLAX_FAR 50.</summary>
        public static int PpuForClass(bool ui, bool parallaxFar)
        {
            if (ui)
            {
                return UiPpu;
            }
            if (parallaxFar)
            {
                return ParallaxFarPpu;
            }
            return GameplayPpu;
        }
    }
}
