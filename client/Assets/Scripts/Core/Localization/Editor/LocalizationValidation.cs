using System.Collections.Generic;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using UnityEditor;
using UnityEditor.AddressableAssets.Settings;
using UnityEngine;
using UnityEngine.Localization;
using UnityEngine.Localization.Tables;

namespace ThinhThan.Core.Localization.Editor
{
    /// <summary>
    /// Bilingual locale validation (client_localization.md § Tests, packet
    /// IMP-064): canonical settings asset at the pinned path/GUID, both locale
    /// assets registered in <c>localization.locales</c>, the Core collection's
    /// tables registered in their canonical groups, bilingual key parity with
    /// no missing or empty translations, placeholder parity between locales,
    /// key grammar and launch-glyph coverage, and no leftover package
    /// <c>Localization-*</c> groups (ADR-0074). The missing-translation gate
    /// rejects any table set where a required entry is absent or empty.
    /// </summary>
    public static class LocalizationValidation
    {
        public const string SettingsAssetPath = "Assets/Localization/Settings/LocalizationSettings.asset";

        public const string SettingsBaselineGuid = "62784203d690f64ca75841bbefc963da";

        public const string SettingsRepoPath = "client/Assets/Localization/Settings/LocalizationSettings.asset";

        private const string TablesDir = "Assets/Localization/Tables/Core";

        private const string CollectionName = "Core";

        private const string PackageGroupPrefix = "Localization-";

        private static readonly Regex KeyGrammar = new Regex(
            "^loc\\.[a-z0-9_]+(\\.[a-z0-9_]+)+$",
            RegexOptions.Compiled);

        /// <summary>Structured result of a validation pass.</summary>
        public sealed class LocaleValidationReport
        {
            public readonly List<string> Errors = new List<string>();

            public readonly List<string> Checks = new List<string>();

            public bool Ok => Errors.Count == 0;

            public string ToReport()
            {
                var sb = new StringBuilder();
                sb.Append("locale validation report\n");
                sb.Append("status: ").Append(Ok ? "PASS" : "FAIL").Append('\n');
                sb.Append("checks:\n");
                foreach (var check in Checks)
                {
                    sb.Append("  - ").Append(check).Append('\n');
                }
                sb.Append("errors:\n");
                foreach (var error in Errors)
                {
                    sb.Append("  - ").Append(error).Append('\n');
                }
                return sb.ToString();
            }
        }

        /// <summary>Runs every locale invariant check and returns the report.</summary>
        public static LocaleValidationReport Validate()
        {
            var report = new LocaleValidationReport();
            CheckSettingsAsset(report);
            var viTable = LoadTable(report, ThinhThanLocale.VietnameseCode);
            var enTable = LoadTable(report, ThinhThanLocale.EnglishCode);
            if (viTable != null && enTable != null)
            {
                CheckKeyParityAndCoverage(report, viTable, enTable);
            }
            CheckAddressableMembership(report);
            return report;
        }

        private static void CheckSettingsAsset(LocaleValidationReport report)
        {
            var guid = AssetDatabase.AssetPathToGUID(SettingsAssetPath);
            if (string.IsNullOrEmpty(guid))
            {
                report.Errors.Add("missing LocalizationSettings.asset at " + SettingsAssetPath);
                return;
            }
            report.Checks.Add("settings asset present at " + SettingsAssetPath);
            if (!string.Equals(guid, SettingsBaselineGuid, System.StringComparison.Ordinal))
            {
                report.Errors.Add("settings GUID " + guid + " != baseline " + SettingsBaselineGuid);
                return;
            }
            var derived = DerivedGuid(SettingsRepoPath);
            if (!string.Equals(derived, SettingsBaselineGuid, System.StringComparison.Ordinal))
            {
                report.Errors.Add("path-derived GUID " + derived + " != baseline " + SettingsBaselineGuid);
            }
            if (!EditorBuildSettings.TryGetConfigObject<LocalizationSettings>("com.unity.localization.settings", out var slot)
                || AssetDatabase.GetAssetPath(slot) != SettingsAssetPath)
            {
                report.Errors.Add("EditorBuildSettings slot com.unity.localization.settings does not resolve to " + SettingsAssetPath);
            }
        }

        private static string DerivedGuid(string repoRelativePath)
        {
            using (var sha = SHA256.Create())
            {
                var bytes = sha.ComputeHash(Encoding.UTF8.GetBytes(repoRelativePath));
                var sb = new StringBuilder(32);
                for (int i = 0; i < 16; i++)
                {
                    sb.Append(bytes[i].ToString("x2"));
                }
                return sb.ToString();
            }
        }

        private static StringTable? LoadTable(LocaleValidationReport report, string code)
        {
            var path = TablesDir + "/" + CollectionName + "_" + code + ".asset";
            var table = AssetDatabase.LoadAssetAtPath<StringTable>(path);
            if (table == null)
            {
                report.Errors.Add("missing string table at " + path);
                return null;
            }
            report.Checks.Add("table " + CollectionName + "_" + code + " loaded");
            return table;
        }

        /// <summary>
        /// The missing-translation gate: every key in the shared data must
        /// resolve to a non-empty value in BOTH locale tables, with matching
        /// placeholder sets and launch-glyph coverage.
        /// </summary>
        private static void CheckKeyParityAndCoverage(
            LocaleValidationReport report,
            StringTable viTable,
            StringTable enTable)
        {
            var shared = viTable.SharedData;
            if (shared == null || enTable.SharedData != shared)
            {
                report.Errors.Add("tables do not share a SharedTableData");
                return;
            }

            var sharedKeys = new HashSet<string>();
            foreach (var entry in shared.Entries)
            {
                var key = entry.Key;
                if (string.IsNullOrEmpty(key))
                {
                    report.Errors.Add("shared entry " + entry.Id + " has an empty key");
                    continue;
                }
                if (!sharedKeys.Add(key))
                {
                    report.Errors.Add("duplicate key in shared data: " + key);
                }
                if (!KeyGrammar.IsMatch(key))
                {
                    report.Errors.Add("key violates loc.<domain>.<stable_segments> grammar: " + key);
                }
            }

            foreach (var key in ThinhThanLocale.LaunchKeys)
            {
                if (!sharedKeys.Contains(key))
                {
                    report.Errors.Add("launch key missing from shared data: " + key);
                }
            }

            int keyCount = 0;
            foreach (var key in sharedKeys)
            {
                keyCount++;
                CheckTableEntry(report, viTable, enTable, key);
            }
            report.Checks.Add("bilingual parity over " + keyCount + " keys");
        }

        /// <summary>
        /// The missing-translation gate for a single key: both locales must
        /// have a non-empty value with matching placeholders.
        /// </summary>
        public static void CheckTableEntry(
            LocaleValidationReport report,
            StringTable viTable,
            StringTable enTable,
            string key)
        {
            var viEntry = viTable.GetEntry(key);
            var enEntry = enTable.GetEntry(key);
            if (viEntry == null || string.IsNullOrEmpty(viEntry.Value))
            {
                report.Errors.Add("missing vi-VN translation: " + key);
            }
            if (enEntry == null || string.IsNullOrEmpty(enEntry.Value))
            {
                report.Errors.Add("missing en-US translation: " + key);
            }
            if (viEntry == null || enEntry == null || viEntry.Value == null || enEntry.Value == null)
            {
                return;
            }
            var viPlaceholders = Placeholders(viEntry.Value);
            var enPlaceholders = Placeholders(enEntry.Value);
            foreach (var name in viPlaceholders)
            {
                if (!enPlaceholders.Contains(name))
                {
                    report.Errors.Add("placeholder {" + name + "} missing in en-US for " + key);
                }
            }
            foreach (var name in enPlaceholders)
            {
                if (!viPlaceholders.Contains(name))
                {
                    report.Errors.Add("placeholder {" + name + "} missing in vi-VN for " + key);
                }
            }
            CheckGlyphCoverage(report, viEntry.Value, key, ThinhThanLocale.VietnameseCode);
            CheckGlyphCoverage(report, enEntry.Value, key, ThinhThanLocale.EnglishCode);
        }

        private static HashSet<string> Placeholders(string value)
        {
            var set = new HashSet<string>();
            int start = 0;
            while (true)
            {
                int open = value.IndexOf('{', start);
                if (open < 0)
                {
                    break;
                }
                int close = value.IndexOf('}', open + 1);
                if (close < 0)
                {
                    break;
                }
                var name = value.Substring(open + 1, close - open - 1);
                if (name.Length > 0)
                {
                    set.Add(name);
                }
                start = close + 1;
            }
            return set;
        }

        /// <summary>Every character must sit in the launch glyph set (client_localization.md § Fonts).</summary>
        private static void CheckGlyphCoverage(LocaleValidationReport report, string value, string key, string code)
        {
            foreach (var c in value)
            {
                if (!InLaunchGlyphSet(c))
                {
                    report.Errors.Add(
                        "glyph U+" + ((int)c).ToString("X4") + " outside launch set in " + code + " value for " + key);
                }
            }
        }

        private static bool InLaunchGlyphSet(char c)
        {
            int o = c;
            if (o >= 0x20 && o <= 0x7E) return true;
            if (o >= 0xA0 && o <= 0xFF) return true;
            if (o >= 0x1EA0 && o <= 0x1EF9) return true;
            switch (o)
            {
                case 0x102:
                case 0x103:
                case 0x110:
                case 0x111:
                case 0x128:
                case 0x129:
                case 0x168:
                case 0x169:
                case 0x1A0:
                case 0x1A1:
                case 0x1AF:
                case 0x1B0:
                case 0x20AB:
                    return true;
                default:
                    return false;
            }
        }

        /// <summary>
        /// Addressable membership: locale assets carry the <c>Locale</c> label in
        /// <c>localization.locales</c>; table assets carry <c>Locale-&lt;code&gt;</c>
        /// in <c>localization.strings.&lt;code&gt;</c>; shared data sits in
        /// <c>localization.shared</c>; no <c>Localization-*</c> package groups
        /// remain (ADR-0074).
        /// </summary>
        private static void CheckAddressableMembership(LocaleValidationReport report)
        {
            var settings = AddressableAssetSettingsDefaultObject.Settings;
            if (settings == null)
            {
                report.Errors.Add("no AddressableAssetSettings");
                return;
            }
            foreach (var group in settings.groups)
            {
                if (group.Name.StartsWith(PackageGroupPrefix, System.StringComparison.Ordinal))
                {
                    report.Errors.Add("unconverged package group present: " + group.Name);
                }
            }
            CheckAddressableEntry(report, settings, "Assets/Localization/Settings/Locale vi-VN.asset",
                "localization.locales", "vi-VN", "Locale");
            CheckAddressableEntry(report, settings, "Assets/Localization/Settings/Locale en-US.asset",
                "localization.locales", "en-US", "Locale");
            CheckAddressableEntry(report, settings, TablesDir + "/" + CollectionName + " Shared Data.asset",
                "localization.shared", CollectionName + " Shared Data", null);
            CheckAddressableEntry(report, settings, TablesDir + "/" + CollectionName + "_vi-VN.asset",
                "localization.strings.vi_vn", CollectionName + "_vi-VN", "Locale-vi-VN");
            CheckAddressableEntry(report, settings, TablesDir + "/" + CollectionName + "_en-US.asset",
                "localization.strings.en_us", CollectionName + "_en-US", "Locale-en-US");
        }

        private static void CheckAddressableEntry(
            LocaleValidationReport report,
            AddressableAssetSettings settings,
            string assetPath,
            string groupName,
            string address,
            string? label)
        {
            var guid = AssetDatabase.AssetPathToGUID(assetPath);
            if (string.IsNullOrEmpty(guid))
            {
                report.Errors.Add("asset missing for addressable check: " + assetPath);
                return;
            }
            var entry = settings.FindAssetEntry(guid);
            var tag = System.IO.Path.GetFileName(assetPath);
            if (entry == null)
            {
                report.Errors.Add(tag + " has no addressable entry");
                return;
            }
            if (entry.parentGroup == null || entry.parentGroup.Name != groupName)
            {
                report.Errors.Add(tag + " not in group " + groupName);
            }
            if (entry.address != address)
            {
                report.Errors.Add(tag + " address " + entry.address + " != " + address);
            }
            if (label != null && !entry.labels.Contains(label))
            {
                report.Errors.Add(tag + " missing label " + label);
            }
            report.Checks.Add(tag + " -> " + groupName + " [" + address + "]");
        }
    }
}
