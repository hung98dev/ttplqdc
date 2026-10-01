using System;
using System.Globalization;
using UnityEngine.Localization;
using UnityEngine.Localization.Settings;

namespace ThinhThan.Core.Localization
{
    /// <summary>
    /// Maps the device OS culture onto the supported locales. The package
    /// SystemLocaleSelector walks CultureInfo parents and so cannot map a
    /// sibling culture such as en-GB onto en-US; this selector returns en-US
    /// for every en-* culture and null otherwise so the next selector in the
    /// chain (the vi-VN default) decides — an unsupported OS culture never
    /// forces an unsupported locale (client_localization.md § Locale Selection).
    /// </summary>
    [Serializable]
    public class ThinhThanOsLocaleSelector : IStartupLocaleSelector
    {
        public Locale? GetStartupLocale(ILocalesProvider availableLocales)
        {
            var culture = CultureInfo.CurrentUICulture;
            if (culture != null && culture.Name.StartsWith("en", StringComparison.OrdinalIgnoreCase))
            {
                return availableLocales.GetLocale(new LocaleIdentifier(ThinhThanLocale.EnglishCode));
            }
            return null;
        }
    }
}
