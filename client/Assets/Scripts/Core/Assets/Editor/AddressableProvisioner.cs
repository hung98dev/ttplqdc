using System.Collections.Generic;
using System.IO;
using System.Text.RegularExpressions;
using UnityEditor;
using UnityEditor.AddressableAssets.Settings;
using UnityEditor.AddressableAssets.Settings.GroupSchemas;
using UnityEngine;

namespace ThinhThan.Core.Assets.Editor
{
    /// <summary>
    /// Converges <c>Assets/AddressableAssetsData</c> to the canonical contract
    /// on every project open: the CI materialization step only opens the
    /// project (-batchmode -quit), so the provisioner self-runs via
    /// InitializeOnLoadMethod after package import hooks have settled. The
    /// pass is idempotent — a converged project produces zero diff.
    /// Contract: client_assets.md § Grouping, ADR-0074.
    /// </summary>
    public static class AddressableProvisioner
    {
        public const string SettingsFolder = "Assets/AddressableAssetsData";
        public const string SettingsPath = "Assets/AddressableAssetsData/AddressableAssetSettings.asset";
        public const string DefaultObjectPath = "Assets/AddressableAssetsData/DefaultObject.asset";
        public const string EbsConfigName = "com.unity.addressableassets";
        public const string BaselineGuid = "03d05df79b43898f254cff5dad608436";

        private const string PackageGroupPrefix = "Localization-";
        private const string PackageLocalesGroup = "Localization-Locales";
        private const string PackageStringTablesPrefix = "Localization-String-Tables-";

        [InitializeOnLoadMethod]
        private static void ProvisionOnEditorLoad()
        {
            Provision();
        }

        /// <summary>
        /// Idempotent convergence: the settings asset exists at the baseline
        /// path and GUID, all 29 canonical groups exist with their bundle
        /// schemas and profile paths, package-created <c>Localization-*</c>
        /// entries are re-homed into <c>localization.*</c>, the
        /// com.unity.addressableassets EditorBuildSettings slot points at the
        /// canonical settings object and DefaultObject.asset is gone.
        /// </summary>
        public static AddressableAssetSettings Provision()
        {
            var settings = EnsureSettings();
            EnsureBaselineGuid();
            settings = AssetDatabase.LoadAssetAtPath<AddressableAssetSettings>(SettingsPath);
            EnsureGroups(settings);
            AddressableBuildConfig.EnsureProfiles(settings);
            RehomePackageGroups(settings);
            RestoreEditorBuildSettingsSlot(settings);
            DeleteDefaultObject();
            EditorUtility.SetDirty(settings);
            AssetDatabase.SaveAssets();
            return settings;
        }

        private static AddressableAssetSettings EnsureSettings()
        {
            var settings = AssetDatabase.LoadAssetAtPath<AddressableAssetSettings>(SettingsPath);
            if (settings == null)
            {
                settings = AddressableAssetSettings.Create(SettingsFolder, "AddressableAssetSettings", false, true);
                AssetDatabase.SaveAssets();
            }
            if (settings == null)
            {
                throw new System.InvalidOperationException("AddressableProvisioner: failed to create " + SettingsPath);
            }
            return settings;
        }

        /// <summary>
        /// The committed .meta pins the path-derived baseline GUID
        /// (repository_layout.md § ProjectSettings Baseline). Unity may assign
        /// a fresh GUID when the settings asset is (re)created and only flush
        /// that GUID to the .meta on a later save, so converge in a loop:
        /// save+refresh until the database GUID and the on-disk meta both say
        /// baseline.
        /// </summary>
        private static void EnsureBaselineGuid()
        {
            var metaPath = Path.Combine(ProjectRoot(), SettingsPath + ".meta");
            for (var attempt = 0; attempt < 3; attempt++)
            {
                AssetDatabase.SaveAssets();
                AssetDatabase.Refresh(ImportAssetOptions.ForceSynchronousImport);
                if (AssetDatabase.AssetPathToGUID(SettingsPath) == BaselineGuid)
                {
                    return;
                }
                if (!File.Exists(metaPath))
                {
                    continue;
                }
                var text = File.ReadAllText(metaPath);
                var updated = Regex.Replace(text, @"^guid:\s*[0-9a-fA-F]+", "guid: " + BaselineGuid, RegexOptions.Multiline);
                if (updated != text)
                {
                    File.WriteAllText(metaPath, updated);
                }
            }
            if (AssetDatabase.AssetPathToGUID(SettingsPath) != BaselineGuid)
            {
                Debug.LogError("AddressableProvisioner: settings GUID != baseline " + BaselineGuid);
            }
        }

        private static string ProjectRoot()
        {
            return Path.GetFullPath(Path.Combine(Application.dataPath, ".."));
        }

        /// <summary>
        /// Every canonical group exists with BundledAssetGroupSchema +
        /// ContentUpdateGroupSchema configured per manifest §1; any
        /// non-canonical, non-package-managed group is removed.
        /// </summary>
        private static void EnsureGroups(AddressableAssetSettings settings)
        {
            var stale = new List<AddressableAssetGroup>();
            foreach (var group in settings.groups)
            {
                if (!AddressableGroups.IsCanonicalName(group.Name)
                    && !group.Name.StartsWith(PackageGroupPrefix, System.StringComparison.Ordinal)
                    && !group.ReadOnly)
                {
                    stale.Add(group);
                }
            }
            foreach (var group in stale)
            {
                settings.RemoveGroup(group);
            }
            foreach (var name in AddressableGroups.CanonicalNames())
            {
                var group = settings.FindGroup(name);
                if (group == null)
                {
                    group = settings.CreateGroup(
                        name, false, false, true, null,
                        typeof(BundledAssetGroupSchema),
                        typeof(ContentUpdateGroupSchema));
                }
                AddressableBuildConfig.ConfigureGroup(settings, group);
            }
            var defaultGroup = settings.FindGroup(AddressableGroups.SharedLocal);
            if (settings.DefaultGroup != defaultGroup)
            {
                settings.DefaultGroup = defaultGroup;
            }
        }

        /// <summary>
        /// ADR-0074: move entries out of every package-created
        /// <c>Localization-*</c> group into the mapped canonical group,
        /// preserving package addresses/labels (the runtime resolution
        /// contract), then remove the emptied package groups.
        /// </summary>
        public static void RehomePackageGroups(AddressableAssetSettings settings)
        {
            var packageGroups = new List<AddressableAssetGroup>();
            foreach (var group in settings.groups)
            {
                if (group.Name.StartsWith(PackageGroupPrefix, System.StringComparison.Ordinal))
                {
                    packageGroups.Add(group);
                }
            }
            foreach (var group in packageGroups)
            {
                var dest = settings.FindGroup(CanonicalForPackageGroup(group.Name));
                if (dest == null)
                {
                    Debug.LogError("AddressableProvisioner: no canonical group for " + group.Name);
                    continue;
                }
                var entries = new List<AddressableAssetEntry>(group.entries);
                foreach (var entry in entries)
                {
                    settings.CreateOrMoveEntry(entry.guid, dest, false, true);
                }
                settings.RemoveGroup(group);
            }
        }

        /// <summary>The canonical localization.* group for a package group name.</summary>
        public static string CanonicalForPackageGroup(string packageGroupName)
        {
            if (packageGroupName == PackageLocalesGroup)
            {
                return AddressableGroups.LocalizationLocales;
            }
            if (packageGroupName.StartsWith(PackageStringTablesPrefix, System.StringComparison.Ordinal))
            {
                var code = packageGroupName.Substring(PackageStringTablesPrefix.Length);
                return AddressableGroups.LocalizationStringsPrefix + code.ToLowerInvariant().Replace('-', '_');
            }
            return AddressableGroups.LocalizationShared;
        }

        /// <summary>
        /// Restore the com.unity.addressableassets EditorBuildSettings slot to
        /// the canonical settings object (ADR-0074).
        /// </summary>
        public static void RestoreEditorBuildSettingsSlot(AddressableAssetSettings settings)
        {
            if (EditorBuildSettings.TryGetConfigObject<AddressableAssetSettings>(EbsConfigName, out var current)
                && current == settings)
            {
                return;
            }
            EditorBuildSettings.AddConfigObject(EbsConfigName, settings, true);
        }

        /// <summary>Delete the package-created DefaultObject.asset (ADR-0074).</summary>
        public static void DeleteDefaultObject()
        {
            AssetDatabase.DeleteAsset(DefaultObjectPath);
        }
    }
}
