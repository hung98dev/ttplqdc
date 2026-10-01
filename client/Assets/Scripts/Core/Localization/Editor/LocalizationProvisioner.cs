using System.Collections.Generic;
using System.Reflection;
using UnityEditor;
using UnityEditor.Localization;
using UnityEngine;
using UnityEngine.Localization;
using UnityEngine.Localization.Settings;
using UnityEngine.Localization.Tables;

namespace ThinhThan.Core.Localization.Editor
{
    /// <summary>
    /// Materializes the bilingual localization pipeline (IMP-064,
    /// client_localization.md): the pinned <c>LocalizationSettings.asset</c>,
    /// the vi-VN/en-US locale assets, and the <c>Core</c> string table
    /// collection seeded with the launch keys.
    /// Addressable registration goes through the package's editor API
    /// (<see cref="LocalizationEditorSettings"/>), which lands entries in the
    /// package's <c>Localization-*</c> groups; the pass then re-runs the
    /// addressables provisioner (IMP-063), which rehomes those entries into
    /// the canonical <c>localization.*</c> groups and deletes the empty
    /// package groups (ADR-0074). The bridge is reflective because the frozen
    /// asmdef does not reference <c>ThinhThan.Core.Assets.Editor</c>.
    /// </summary>
    public static class LocalizationProvisioner
    {
        private const string SettingsDir = "Assets/Localization/Settings";

        private const string TablesDir = "Assets/Localization/Tables/Core";

        private const string SettingsAssetPath = SettingsDir + "/LocalizationSettings.asset";

        private const string EbsConfigName = "com.unity.localization.settings";

        private const string CollectionName = "Core";

        private const string AssetsProvisionerTypeName =
            "ThinhThan.Core.Assets.Editor.AddressableProvisioner, ThinhThan.Core.Assets.Editor";

        private static readonly string[] _supportedLocaleCodes =
        {
            ThinhThanLocale.VietnameseCode,
            ThinhThanLocale.EnglishCode,
        };

        private static readonly (string Key, string ViVn, string EnUs)[] _seedEntries =
        {
            ("loc.combat.just_guard_hint", "Bước đúng lúc", "Step on time"),
            ("loc.combat.just_guard", "Chặn Chuẩn", "Just Guard"),
            ("loc.combat.linh_thu_clutch", "Linh Thú Cứu Trận", "Linh Thu Clutch"),
            ("loc.peak.phat_hien.chest_spotted", "Phát Hiện: Rương Ẩn", "Discovery: Hidden Chest"),
            ("loc.peak.phat_hien.chest", "Phát Hiện: Mở Rương", "Discovery: Chest Opened"),
            ("loc.peak.phat_hien.rare_fish", "Phát Hiện: Cá Chép Hóa Rồng", "Discovery: Dragon Carp"),
            ("loc.notice.enhancement_plus_16_broadcast", "Chúc mừng {player} đã cường hóa {item} lên +16!", "Congratulations {player} for enhancing {item} to +16!"),
            ("loc.system.rewards_paused", "Phần thưởng tạm ngưng", "Rewards paused"),
        };

        [InitializeOnLoadMethod]
        private static void Initialize()
        {
            EditorApplication.delayCall += Provision;
        }

        /// <summary>Idempotent provisioning pass, safe to run on every editor launch.</summary>
        public static void Provision()
        {
            var changed = EnsureSettingsAsset(out var settings);
            if (settings == null)
            {
                Debug.LogError("LocalizationProvisioner: could not load " + SettingsAssetPath);
                return;
            }
            changed |= NormalizeSettings(settings);
            changed |= EnsureLocales(out var locales);
            changed |= EnsureCoreCollection(locales, out var collection);
            if (collection != null)
            {
                changed |= SeedKeys(collection);
            }
            if (changed)
            {
                EditorUtility.SetDirty(settings);
                AssetDatabase.SaveAssets();
            }
            RunAssetsProvisioner();
        }

        private static bool EnsureSettingsAsset(out LocalizationSettings? settings)
        {
            var changed = false;
            settings = LoadAsset<LocalizationSettings>(SettingsAssetPath);
            if (settings == null)
            {
                EnsureFolder("Assets/Localization");
                EnsureFolder(SettingsDir);
                settings = ScriptableObject.CreateInstance<LocalizationSettings>();
                AssetDatabase.CreateAsset(settings, SettingsAssetPath);
                settings = AssetDatabase.LoadAssetAtPath<LocalizationSettings>(SettingsAssetPath);
                changed = true;
            }
            if (!EditorBuildSettings.TryGetConfigObject<LocalizationSettings>(EbsConfigName, out var slot)
                || slot != settings)
            {
                EditorBuildSettings.AddConfigObject(EbsConfigName, settings, true);
                changed = true;
            }
            return changed;
        }

        /// <summary>
        /// Rewrite the startup selector chain to saved-preference → en-* OS →
        /// vi-VN fallback, pin the project locale and synchronous init
        /// (client_localization.md § Locale Selection).
        /// </summary>
        private static bool NormalizeSettings(LocalizationSettings settings)
        {
            var changed = false;
            var selectors = settings.GetStartupLocaleSelectors();
            var chainOk = selectors.Count == 3
                && selectors[0] is PlayerPrefLocaleSelector pref
                && pref.PlayerPreferenceKey == ThinhThanLocale.PreferenceKey
                && selectors[1] is ThinhThanOsLocaleSelector
                && selectors[2] is SpecificLocaleSelector specific
                && specific.LocaleId.Code == ThinhThanLocale.VietnameseCode;
            if (!chainOk)
            {
                selectors.Clear();
                selectors.Add(new PlayerPrefLocaleSelector());
                selectors.Add(new ThinhThanOsLocaleSelector());
                selectors.Add(new SpecificLocaleSelector
                {
                    LocaleId = new LocaleIdentifier(ThinhThanLocale.VietnameseCode),
                });
                changed = true;
            }

            var serialized = new SerializedObject(settings);
            var projectCode = serialized.FindProperty("m_ProjectLocaleIdentifier.m_Code");
            if (projectCode != null && projectCode.stringValue != ThinhThanLocale.VietnameseCode)
            {
                projectCode.stringValue = ThinhThanLocale.VietnameseCode;
                changed = true;
            }
            var syncInit = serialized.FindProperty("m_InitializeSynchronously");
            if (syncInit != null && !syncInit.boolValue)
            {
                syncInit.boolValue = true;
                changed = true;
            }
            if (changed)
            {
                serialized.ApplyModifiedPropertiesWithoutUndo();
            }
            return changed;
        }

        private static bool EnsureLocales(out List<Locale> locales)
        {
            locales = new List<Locale>(_supportedLocaleCodes.Length);
            var changed = false;
            foreach (var code in _supportedLocaleCodes)
            {
                var path = SettingsDir + "/Locale " + code + ".asset";
                var locale = LoadAsset<Locale>(path);
                if (locale == null)
                {
                    // LocaleName drives the package's group-name pattern
                    // (Localization-String-Tables-{LocaleName}) and the
                    // addressable entry address; pinning it to the code makes
                    // the ADR-0074 canonical mapping deterministic.
                    locale = Locale.CreateLocale(code);
                    locale.name = "Locale " + code;
                    locale.LocaleName = code;
                    AssetDatabase.CreateAsset(locale, path);
                    changed = true;
                }
                else if (locale.LocaleName != code)
                {
                    locale.LocaleName = code;
                    EditorUtility.SetDirty(locale);
                    changed = true;
                }
                if (LocalizationEditorSettings.GetLocale(new LocaleIdentifier(code)) == null)
                {
                    LocalizationEditorSettings.AddLocale(locale);
                    changed = true;
                }
                locales.Add(locale);
            }
            return changed;
        }

        private static bool EnsureCoreCollection(
            IReadOnlyList<Locale> locales,
            out StringTableCollection? collection)
        {
            EnsureFolder("Assets/Localization");
            EnsureFolder("Assets/Localization/Tables");
            EnsureFolder(TablesDir);
            var changed = false;
            collection = LoadAsset<StringTableCollection>(TablesDir + "/" + CollectionName + ".asset");
            if (collection == null)
            {
                collection = LocalizationEditorSettings.CreateStringTableCollection(
                    CollectionName, TablesDir, new List<Locale>(locales));
                changed = true;
            }
            return changed;
        }

        /// <summary>Seed the launch keys in both locales (idempotent upserts).</summary>
        private static bool SeedKeys(StringTableCollection collection)
        {
            var viTable = collection.GetTable(new LocaleIdentifier(ThinhThanLocale.VietnameseCode)) as StringTable;
            var enTable = collection.GetTable(new LocaleIdentifier(ThinhThanLocale.EnglishCode)) as StringTable;
            if (viTable == null || enTable == null)
            {
                Debug.LogError("LocalizationProvisioner: Core tables missing for vi-VN/en-US");
                return false;
            }

            var changed = false;
            foreach (var (key, viVn, enUs) in _seedEntries)
            {
                changed |= Upsert(viTable, key, viVn);
                changed |= Upsert(enTable, key, enUs);
            }
            if (changed)
            {
                EditorUtility.SetDirty(viTable);
                EditorUtility.SetDirty(enTable);
                if (viTable.SharedData != null)
                {
                    EditorUtility.SetDirty(viTable.SharedData);
                }
            }
            return changed;
        }

        private static bool Upsert(StringTable table, string key, string value)
        {
            var entry = table.GetEntry(key);
            var changed = false;
            if (entry == null || entry.Value != value)
            {
                entry = table.AddEntry(key, value);
                changed = true;
            }
            var smart = value != null && value.Contains("{") && value.Contains("}");
            if (entry != null && entry.IsSmart != smart)
            {
                entry.IsSmart = smart;
                changed = true;
            }
            return changed;
        }

        /// <summary>
        /// Re-runs the IMP-063 addressables provisioner so the package-created
        /// <c>Localization-*</c> groups are rehomed into the canonical
        /// <c>localization.*</c> groups in the same launch (ADR-0074). The
        /// frozen asmdef does not reference <c>ThinhThan.Core.Assets.Editor</c>,
        /// so the entry point is invoked by reflection; both provisioners are
        /// idempotent, which makes the call ordering-independent.
        /// </summary>
        private static void RunAssetsProvisioner()
        {
            var type = System.Type.GetType(AssetsProvisionerTypeName);
            var method = type?.GetMethod("Provision", BindingFlags.Public | BindingFlags.Static);
            if (method == null)
            {
                Debug.LogError("LocalizationProvisioner: addressables provisioner not found");
                return;
            }
            method.Invoke(null, null);
        }

        /// <summary>
        /// Load an asset; when it is committed on disk but not yet imported
        /// (delayCall can beat the first import pass) force a synchronous
        /// refresh instead of recreating (IMP-063 lesson).
        /// </summary>
        private static T? LoadAsset<T>(string assetPath) where T : Object
        {
            var asset = AssetDatabase.LoadAssetAtPath<T>(assetPath);
            if (asset == null
                && System.IO.File.Exists(System.IO.Path.Combine(ProjectRoot(), assetPath)))
            {
                AssetDatabase.Refresh(ImportAssetOptions.ForceSynchronousImport);
                asset = AssetDatabase.LoadAssetAtPath<T>(assetPath);
            }
            return asset;
        }

        private static string ProjectRoot()
        {
            return System.IO.Path.GetFullPath(System.IO.Path.Combine(Application.dataPath, ".."));
        }

        private static void EnsureFolder(string path)
        {
            if (AssetDatabase.IsValidFolder(path))
            {
                return;
            }
            var parent = System.IO.Path.GetDirectoryName(path)!.Replace('\\', '/');
            var leaf = System.IO.Path.GetFileName(path);
            AssetDatabase.CreateFolder(parent, leaf);
        }
    }
}
