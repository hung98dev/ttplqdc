namespace ThinhThan.Core.Localization
{
    /// <summary>
    /// Locale constants shared by the settings asset, startup selectors and
    /// runtime helpers (client_localization.md § Required Locales).
    /// </summary>
    public static class ThinhThanLocale
    {
        public const string VietnameseCode = "vi-VN";

        public const string EnglishCode = "en-US";

        /// <summary>PlayerPrefs key the player-choice selector reads (ADR-0015).</summary>
        public const string PreferenceKey = "selected-locale";

        /// <summary>Keys seeded into the Core string table at launch (client_localization.md launch keys).</summary>
        public static readonly string[] LaunchKeys =
        {
            "loc.combat.just_guard_hint",
            "loc.combat.just_guard",
            "loc.combat.linh_thu_clutch",
            "loc.peak.phat_hien.chest_spotted",
            "loc.peak.phat_hien.chest",
            "loc.peak.phat_hien.rare_fish",
            "loc.notice.enhancement_plus_16_broadcast",
            "loc.system.rewards_paused",
        };
    }
}
