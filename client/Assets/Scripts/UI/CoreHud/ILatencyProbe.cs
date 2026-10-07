namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Ping seam (F-8): round-trip ms for the HUD top-right readout;
    /// production source lands in composition, tests fake.
    /// </summary>
    public interface ILatencyProbe
    {
        /// <summary>Latest round-trip estimate in milliseconds; -1 unknown.</summary>
        int RttMs
        {
            get;
        }
    }
}
