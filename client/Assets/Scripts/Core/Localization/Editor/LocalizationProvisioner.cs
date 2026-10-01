using UnityEditor;
using UnityEditor.AddressableAssets.Settings;
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
    /// collection seeded with the launch keys. Addressable entries are
    /// registered directly into the canonical <c>localization.*</c> groups with
    /// the package's address/label contract (ADR-0074), so the converged state
    /// is produced in the same editor launch and no <c>Localization-*</c>
    /// package group is ever created.
    /// </summary>
    public static class LocalizationProvisioner
    {
        private const string SettingsDir = "Assets/Localization/Settings";

        private const string TablesDir = "Assets/Localization/Tables/Core";

        private const string SettingsAssetPath = SettingsDir + "/LocalizationSettings.asset";

        private const string AddressableSettingsPath = "Assets/AddressableAssetsData/AddressableAssetSettings.asset";

        private const string EbsConfigName = "com.unity.localization.settings";

        private const string CollectionName = "Core";

        private const string GroupLocales = "localization.locales";

        private const string GroupShared = "localization.shared";

        private const string GroupStringsViVn = "localization.strings.vi_vn";

        private const string GroupStringsEnUs = "localization.strings.en_us";

        private const string LocaleLabel = "Locale";

        private const string LocaleLabelPrefix = "Locale-";

        private const string SharedDataSuffix = " Shared Data";

        private static readonly (string Code, string Group)[] _supportedLocales =
        {
            (ThinhThanLocale.VietnameseCode, GroupStringsViVn),
            (ThinhThanLocale.EnglishCode, GroupStringsEnUs),
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
            var addressables = LoadAsset<AddressableAssetSettings>(AddressableSettingsPath);
            if (addressables == null)
            {
                return;
            }

            var settingsCreated = EnsureSettingsAsset(out var settings);
            if (settings == null)
            {
                Debug.LogError("LocalizationProvisioner: could not load " + SettingsAssetPath);
                return;
            }
            var changed = settingsCreated;
            changed |= NormalizeSettings(settings);
            changed |= EnsureLocales(addressables);
            changed |= EnsureCoreCollection(addressables);
            if (changed)
            {
                EditorUtility.SetDirty(settings);
                EditorUtility.SetDirty(addressables);
                AssetDatabase.SaveAssets();
            }
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

        private static bool EnsureLocales(AddressableAssetSettings addressables)
        {
            var group = addressables.FindGroup(GroupLocales);
            if (group == null)
            {
                Debug.LogError("LocalizationProvisioner: canonical group missing: " + GroupLocales);
                return false;
            }

            var changed = false;
            foreach (var (code, _) in _supportedLocales)
            {
                var path = SettingsDir + "/Locale " + code + ".asset";
                var locale = LoadAsset<Locale>(path);
                if (locale == null)
                {
                    locale = Locale.CreateLocale(code);
                    locale.name = "Locale " + code;
                    AssetDatabase.CreateAsset(locale, path);
                    changed = true;
                }
                changed |= EnsureEntry(
                    addressables,
                    path,
                    group,
                    address: code,
                    label: LocaleLabel);
            }
            return changed;
        }

        private static bool EnsureCoreCollection(AddressableAssetSettings addressables)
        {
            EnsureFolder("Assets/Localization");
            EnsureFolder("Assets/Localization/Tables");
            EnsureFolder(TablesDir);

            var shared = LoadAsset<SharedTableData>(TablesDir + "/" + CollectionName + SharedDataSuffix + ".asset");
            var collection = LoadAsset<StringTableCollection>(TablesDir + "/" + CollectionName + ".asset");
            var changed = false;

            var sharedGroup = addressables.FindGroup(GroupShared);
            if (shared == null)
            {
                shared = ScriptableObject.CreateInstance<SharedTableData>();
                shared.TableCollectionName = CollectionName;
                var sharedPath = TablesDir + "/" + CollectionName + SharedDataSuffix + ".asset";
                AssetDatabase.CreateAsset(shared, sharedPath);
                shared.TableCollectionNameGuid = System.Guid.Parse(AssetDatabase.AssetPathToGUID(sharedPath));
                EditorUtility.SetDirty(shared);
                changed = true;
            }
            if (sharedGroup == null)
            {
                Debug.LogError("LocalizationProvisioner: canonical group missing: " + GroupShared);
            }
            else
            {
                changed |= EnsureEntry(
                    addressables,
                    TablesDir + "/" + CollectionName + SharedDataSuffix + ".asset",
                    sharedGroup,
                    address: CollectionName + SharedDataSuffix,
                    label: null);
            }

            StringTable? viTable = null;
            StringTable? enTable = null;
            foreach (var (code, groupName) in _supportedLocales)
            {
                var tablePath = TablesDir + "/" + CollectionName + "_" + code + ".asset";
                var table = LoadAsset<StringTable>(tablePath);
                if (table == null)
                {
                    table = ScriptableObject.CreateInstance<StringTable>();
                    table.SharedData = shared;
                    table.LocaleIdentifier = new LocaleIdentifier(code);
                    table.name = CollectionName + "_" + code;
                    AssetDatabase.CreateAsset(table, tablePath);
                    changed = true;
                }
                var group = addressables.FindGroup(groupName);
                if (group == null)
                {
                    Debug.LogError("LocalizationProvisioner: canonical group missing: " + groupName);
                }
                else
                {
                    changed |= EnsureEntry(
                        addressables,
                        tablePath,
                        group,
                        address: CollectionName + "_" + code,
                        label: LocaleLabelPrefix + code);
                }
                if (code == ThinhThanLocale.VietnameseCode)
                {
                    viTable = table;
                }
                else
                {
                    enTable = table;
                }
            }

            if (viTable != null && enTable != null)
            {
                changed |= SeedKeys(viTable, enTable);
            }

            if (collection == null)
            {
                collection = ScriptableObject.CreateInstance<StringTableCollection>();
                collection.name = CollectionName;
                var serialized = new SerializedObject(collection);
                serialized.FindProperty("m_SharedTableData").objectReferenceValue = shared;
                var tables = serialized.FindProperty("m_Tables");
                tables.arraySize = 2;
                tables.GetArrayElementAtIndex(0).objectReferenceValue = viTable;
                tables.GetArrayElementAtIndex(1).objectReferenceValue = enTable;
                serialized.ApplyModifiedPropertiesWithoutUndo();
                AssetDatabase.CreateAsset(collection, TablesDir + "/" + CollectionName + ".asset");
                changed = true;
            }
            return changed;
        }

        /// <summary>
        /// Register an asset into a canonical group with the package's
        /// address/label contract (ADR-0074). Idempotent: an existing entry with
        /// the right address and label is left alone.
        /// </summary>
        private static bool EnsureEntry(
            AddressableAssetSettings addressables,
            string assetPath,
            AddressableAssetGroup group,
            string address,
            string? label)
        {
            var guid = AssetDatabase.AssetPathToGUID(assetPath);
            if (string.IsNullOrEmpty(guid))
            {
                return false;
            }
            var entry = addressables.FindAssetEntry(guid);
            var changed = false;
            if (entry == null)
            {
                entry = addressables.CreateOrMoveEntry(guid, group, true, true);
                changed = true;
            }
            else if (entry.parentGroup != group)
            {
                addressables.MoveEntry(entry, group, true, true);
                changed = true;
            }
            if (entry.address != address)
            {
                entry.address = address;
                changed = true;
            }
            if (!string.IsNullOrEmpty(label) && !entry.labels.Contains(label))
            {
                entry.SetLabel(label, true, true);
                changed = true;
            }
            return changed;
        }

        /// <summary>Seed the launch keys in both locales (idempotent upserts).</summary>
        private static bool SeedKeys(StringTable viTable, StringTable enTable)
        {
            var changed = false;
            foreach (var (key, viVn, enUs) in _seedEntries)
            {
                var viEntry = viTable.GetEntry(key);
                if (viEntry == null || viEntry.Value != viVn)
                {
                    viEntry = viTable.AddEntry(key, viVn);
                    changed = true;
                }
                if (viEntry != null && viEntry.IsSmart != IsSmart(viVn))
                {
                    viEntry.IsSmart = IsSmart(viVn);
                    changed = true;
                }
                var enEntry = enTable.GetEntry(key);
                if (enEntry == null || enEntry.Value != enUs)
                {
                    enEntry = enTable.AddEntry(key, enUs);
                    changed = true;
                }
                if (enEntry != null && enEntry.IsSmart != IsSmart(enUs))
                {
                    enEntry.IsSmart = IsSmart(enUs);
                    changed = true;
                }
            }
            if (changed)
            {
                EditorUtility.SetDirty(viTable);
                EditorUtility.SetDirty(enTable);
                EditorUtility.SetDirty(viTable.SharedData);
            }
            return changed;
        }

        private static bool IsSmart(string value)
        {
            return value != null && value.Contains("{") && value.Contains("}");
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
