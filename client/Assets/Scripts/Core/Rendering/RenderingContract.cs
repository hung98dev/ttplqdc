using UnityEngine;

namespace ThinhThan.Core.Rendering
{
    /// <summary>
    /// Rendering contract constants (client.md § Rendering, ADR-0056,
    /// presentation_asset_manifest.md §3.1): pixel densities per asset class
    /// and the URP 2D renderer transparency sort axis.
    /// </summary>
    public static class RenderingContract
    {
        /// <summary>Gameplay sprites are authored 2x and imported at 100 PPU (ADR-0055).</summary>
        public const int GameplaySpritePixelsPerUnit = 100;

        /// <summary>UI sprites are authored 2x and imported at 200 PPU (Canvas Reference PPU 100).</summary>
        public const int UiSpritePixelsPerUnit = 200;

        /// <summary>Parallax-far layers may author at TEXTURE_SCALE 1 and import at 50 PPU.</summary>
        public const int ParallaxFarSpritePixelsPerUnit = 50;

        /// <summary>Contact-shadow children render one order band under their actor sprite.</summary>
        public const int ContactShadowOrderInLayer = -1;

        /// <summary>Transparency sort axis (0,1,0): sprites sort back-to-front as Y rises.</summary>
        public static readonly Vector3 TransparencySortAxis = new Vector3(0f, 1f, 0f);
    }
}
