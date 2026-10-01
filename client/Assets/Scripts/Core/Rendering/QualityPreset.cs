namespace ThinhThan.Core.Rendering
{
    /// <summary>
    /// Rendering quality presets (client_performance.md § Platforms and Device
    /// Tiers). The active point Light2D budget per preset is LOW 4, MEDIUM 8,
    /// HIGH 16; see <see cref="PointLightBudget.ActivePointLightLimit"/>.
    /// </summary>
    public enum QualityPreset
    {
        Low,
        Medium,
        High,
    }
}
