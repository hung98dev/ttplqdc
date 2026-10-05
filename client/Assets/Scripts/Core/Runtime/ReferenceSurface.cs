namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// The 1280x720 logical reference surface (client_experience_contract.md
    /// § Display, ADR-0046): the camera/UI design surface the bootstrap
    /// configures — Scale With Screen Size at Match 0.5 and Reference PPU
    /// 100. It is NOT a map size: world maps span 2..5 viewports and never
    /// read these constants.
    /// </summary>
    public static class ReferenceSurface
    {
        /// <summary>Reference-resolution width (px) for the canvas scaler.</summary>
        public const int Width = 1280;

        /// <summary>Reference-resolution height (px) for the canvas scaler.</summary>
        public const int Height = 720;

        /// <summary>Canvas Scaler matchWidthOrHeight (contract: 0.5).</summary>
        public const float MatchWidthOrHeight = 0.5f;

        /// <summary>Reference pixels per unit on the canvas (contract: 100).</summary>
        public const float ReferencePixelsPerUnit = 100f;

        /// <summary>UI sprites are authored 2x, imported at 200 PPU (ADR-0055).</summary>
        public const int UiSpritePixelsPerUnit = 200;

        /// <summary>Default desktop window equals the reference surface.</summary>
        public const int DefaultWindowWidth = Width;

        public const int DefaultWindowHeight = Height;
    }
}
