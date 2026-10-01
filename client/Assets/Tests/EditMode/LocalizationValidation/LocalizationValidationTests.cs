using NUnit.Framework;
using ThinhThan.Core.Localization;
using ThinhThan.Core.Localization.Editor;
using UnityEditor;
using UnityEngine;
using UnityEngine.Localization.Tables;

namespace ThinhThan.Tests.EditMode.LocalizationValidation
{
    /// <summary>
    /// Packet tests (IMP-064): bilingual key parity, the missing-translation
    /// gate, and the pinned LocalizationSettings baseline GUID
    /// (client_localization.md § Tests).
    /// </summary>
    public class LocalizationValidationTests
    {
        private static StringTable LoadTable(string code)
        {
            var path = "Assets/Localization/Tables/Core/Core_" + code + ".asset";
            var table = AssetDatabase.LoadAssetAtPath<StringTable>(path);
            Assert.IsNotNull(table, "missing string table " + path);
            return table!;
        }

        [Test]
        public void TestBilingualKeyParity()
        {
            var viTable = LoadTable(ThinhThanLocale.VietnameseCode);
            var enTable = LoadTable(ThinhThanLocale.EnglishCode);
            Assert.AreSame(viTable.SharedData, enTable.SharedData, "tables must share one SharedTableData");

            foreach (var shared in viTable.SharedData.Entries)
            {
                var viEntry = viTable.GetEntry(shared.Key);
                var enEntry = enTable.GetEntry(shared.Key);
                Assert.IsNotNull(viEntry, "vi-VN missing entry for " + shared.Key);
                Assert.IsNotNull(enEntry, "en-US missing entry for " + shared.Key);
                Assert.IsFalse(string.IsNullOrEmpty(viEntry!.Value), "vi-VN empty value for " + shared.Key);
                Assert.IsFalse(string.IsNullOrEmpty(enEntry!.Value), "en-US empty value for " + shared.Key);
            }
            foreach (var key in ThinhThanLocale.LaunchKeys)
            {
                Assert.IsNotNull(viTable.GetEntry(key), "vi-VN missing launch key " + key);
                Assert.IsNotNull(enTable.GetEntry(key), "en-US missing launch key " + key);
            }
        }

        [Test]
        public void TestNoMissingTranslations()
        {
            var report = ThinhThan.Core.Localization.Editor.LocalizationValidation.Validate();
            if (!report.Ok)
            {
                Assert.Fail(report.ToReport());
            }

            // The gate must reject a table set where a required entry is empty.
            var rejected = ScriptableObject.CreateInstance<StringTable>();
            var rejectedEn = ScriptableObject.CreateInstance<StringTable>();
            var sharedData = ScriptableObject.CreateInstance<SharedTableData>();
            try
            {
                sharedData.TableCollectionName = "Synthetic";
                rejected.SharedData = sharedData;
                rejectedEn.SharedData = sharedData;
                rejected.LocaleIdentifier = new UnityEngine.Localization.LocaleIdentifier(ThinhThanLocale.VietnameseCode);
                rejectedEn.LocaleIdentifier = new UnityEngine.Localization.LocaleIdentifier(ThinhThanLocale.EnglishCode);
                rejected.AddEntry("loc.test.synthetic_gate", "giá trị");
                rejectedEn.AddEntry("loc.test.synthetic_gate", string.Empty);

                var synthetic = new ThinhThan.Core.Localization.Editor.LocalizationValidation.LocaleValidationReport();
                ThinhThan.Core.Localization.Editor.LocalizationValidation.CheckTableEntry(
                    synthetic, rejected, rejectedEn, "loc.test.synthetic_gate");
                Assert.IsFalse(synthetic.Ok, "missing-translation gate must reject an empty en-US value");
                Assert.IsTrue(synthetic.Errors.Exists(e => e.Contains("en-US")), "expected an en-US error");
            }
            finally
            {
                Object.DestroyImmediate(rejected);
                Object.DestroyImmediate(rejectedEn);
                Object.DestroyImmediate(sharedData);
            }
        }

        [Test]
        public void TestSettingsAssetMatchesBaselineGuid()
        {
            var guid = AssetDatabase.AssetPathToGUID(
                ThinhThan.Core.Localization.Editor.LocalizationValidation.SettingsAssetPath);
            Assert.AreEqual(
                ThinhThan.Core.Localization.Editor.LocalizationValidation.SettingsBaselineGuid,
                guid,
                "LocalizationSettings.asset must carry the path-derived baseline GUID");

            var slotOk = EditorBuildSettings.TryGetConfigObject<UnityEngine.Localization.Settings.LocalizationSettings>(
                "com.unity.localization.settings", out var slot);
            Assert.IsTrue(slotOk, "EditorBuildSettings slot com.unity.localization.settings missing");
            Assert.AreEqual(
                ThinhThan.Core.Localization.Editor.LocalizationValidation.SettingsAssetPath,
                AssetDatabase.GetAssetPath(slot));
        }
    }
}
