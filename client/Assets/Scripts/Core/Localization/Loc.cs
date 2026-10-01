using System.Collections.Generic;
using UnityEngine;
using UnityEngine.Localization;
using UnityEngine.Localization.Settings;

namespace ThinhThan.Core.Localization
{
    /// <summary>
    /// Synchronous table lookups and runtime locale switching
    /// (client_localization.md § Keys, § Server/Client Boundary, § Persistence).
    /// Keys use the canonical grammar <c>loc.&lt;domain&gt;.&lt;stable_segments&gt;</c>;
    /// a missing entry resolves to the developer display <c>[MISSING:&lt;key&gt;]</c>
    /// instead of a silent empty string. Formatted values take named Smart
    /// String arguments per call, so currency/entity variables are caller-owned
    /// and survive locale switches without retained state.
    /// </summary>
    public static class Loc
    {
        private const string CoreTableName = "Core";

        private const string MissingPrefix = "[MISSING:";

        private const string MissingSuffix = "]";

        /// <summary>Looks up <paramref name="key"/> in the Core string table for the active locale.</summary>
        public static string Get(string key)
        {
            return Resolve(key, null);
        }

        /// <summary>
        /// Looks up <paramref name="key"/> and formats with named Smart String
        /// arguments (e.g. <c>{player}</c> resolves <c>args["player"]</c>).
        /// </summary>
        public static string Get(string key, IReadOnlyDictionary<string, object> args)
        {
            return Resolve(key, args);
        }

        /// <summary>Active locale code, e.g. <c>vi-VN</c>.</summary>
        public static string ActiveCode
        {
            get
            {
                var locale = LocalizationSettings.SelectedLocale;
                return locale == null ? ThinhThanLocale.VietnameseCode : locale.Identifier.Code;
            }
        }

        /// <summary>
        /// Switches the active locale and persists the choice under
        /// <see cref="ThinhThanLocale.PreferenceKey"/> for the next launch.
        /// Codes outside {vi-VN, en-US} are ignored.
        /// </summary>
        public static void SetLocale(string code)
        {
            if (code != ThinhThanLocale.VietnameseCode && code != ThinhThanLocale.EnglishCode)
            {
                return;
            }
            var locale = LocalizationSettings.AvailableLocales.GetLocale(new LocaleIdentifier(code));
            if (locale == null)
            {
                return;
            }
            if (LocalizationSettings.SelectedLocale != locale)
            {
                LocalizationSettings.SelectedLocale = locale;
            }
            PlayerPrefs.SetString(ThinhThanLocale.PreferenceKey, code);
            PlayerPrefs.Save();
        }

        private static string Resolve(string key, IReadOnlyDictionary<string, object>? args)
        {
            if (string.IsNullOrEmpty(key))
            {
                return MissingPrefix + MissingSuffix;
            }
            string value;
            if (args == null || args.Count == 0)
            {
                value = LocalizationSettings.StringDatabase.GetLocalizedString(CoreTableName, key);
            }
            else
            {
                var namedArgs = new Dictionary<string, object>(args.Count);
                foreach (var pair in args)
                {
                    namedArgs[pair.Key] = pair.Value;
                }
                value = LocalizationSettings.StringDatabase.GetLocalizedString(CoreTableName, key, new object[] { namedArgs });
            }
            return string.IsNullOrEmpty(value) ? MissingPrefix + key + MissingSuffix : value;
        }
    }
}
