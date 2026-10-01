using System.Collections.Generic;
using UnityEditor;
using UnityEditor.AddressableAssets.Settings;
using UnityEditor.AddressableAssets.Settings.GroupSchemas;
using UnityEngine;

namespace ThinhThan.Core.Assets.Editor
{
    /// <summary>
    /// Build/delivery profile configuration for the canonical groups
    /// (client_assets.md § Remote URL + § Build/CI Validation,
    /// presentation_asset_manifest.md §1): dev/staging/production remote base
    /// URL variables, local-vs-remote build/load path pairs, LZ4 for
    /// player-resident groups, LZMA for remote groups and PackSeparately only
    /// for cosmetic.shared. Also owns <see cref="VerifyCatalog"/> — the
    /// deterministic pre-build validation of group set, keys and single
    /// membership.
    /// </summary>
    public static class AddressableBuildConfig
    {
        public const string LocalBuildPathVar = "LocalBuildPath";
        public const string LocalLoadPathVar = "LocalLoadPath";
        public const string RemoteBuildPathVar = "RemoteBuildPath";
        public const string RemoteLoadPathVar = "RemoteLoadPath";
        public const string RemoteBaseUrlVar = "RemoteBaseUrl";

        /// <summary>Profile names, one per deploy environment.</summary>
        public static readonly string[] EnvironmentProfiles = { "Dev", "Staging", "Production" };

        // Placeholder environment origins (client_assets.md § Remote URL:
        // remote base URL is environment configuration; no CDN vendor SDK).
        private const string DevRemoteBaseUrl = "https://assets.dev.thinhthan.internal";
        private const string StagingRemoteBaseUrl = "https://assets.staging.thinhthan.internal";
        private const string ProductionRemoteBaseUrl = "https://assets.thinhthan.internal";

        /// <summary>
        /// Ensure the path variables and the Dev/Staging/Production profiles
        /// exist. RemoteLoadPath resolves through RemoteBaseUrl so a build
        /// only switches profiles.
        /// </summary>
        public static void EnsureProfiles(AddressableAssetSettings settings)
        {
            var profileSettings = settings.profileSettings;
            if (profileSettings == null)
            {
                Debug.LogError("AddressableBuildConfig: settings.profileSettings is null");
                return;
            }
            EnsureVariable(profileSettings, LocalBuildPathVar,
                "[UnityEngine.AddressableAssets.PlatformGroupsService.BuildPath]/[BuildTarget]");
            EnsureVariable(profileSettings, LocalLoadPathVar,
                "{UnityEngine.AddressableAssets.Addressables.RuntimePath}/[BuildTarget]");
            EnsureVariable(profileSettings, RemoteBuildPathVar, "ServerData/[BuildTarget]");
            EnsureVariable(profileSettings, RemoteLoadPathVar, "[" + RemoteBaseUrlVar + "]/[BuildTarget]");
            EnsureVariable(profileSettings, RemoteBaseUrlVar, DevRemoteBaseUrl);

            var names = profileSettings.GetAllProfileNames();
            foreach (var env in EnvironmentProfiles)
            {
                string profileId;
                if (names.Contains(env))
                {
                    profileId = profileSettings.GetProfileId(env);
                }
                else
                {
                    profileId = profileSettings.AddProfile(env, settings.activeProfileId);
                }
                profileSettings.SetValue(profileId, RemoteBaseUrlVar, BaseUrlFor(env));
            }
        }

        private static string BaseUrlFor(string environment)
        {
            switch (environment)
            {
                case "Staging":
                    return StagingRemoteBaseUrl;
                case "Production":
                    return ProductionRemoteBaseUrl;
                default:
                    return DevRemoteBaseUrl;
            }
        }

        private static void EnsureVariable(AddressableAssetProfileSettings profileSettings, string name, string value)
        {
            if (!profileSettings.GetVariableNames().Contains(name))
            {
                profileSettings.CreateValue(name, value);
            }
        }

        /// <summary>
        /// Configure a group's BundledAssetGroupSchema/ContentUpdateGroupSchema
        /// for its residency and bundle mode (manifest §1): player-resident
        /// groups use local paths + LZ4 + StaticContent; remote groups use
        /// remote paths + LZMA + asset-bundle cache; cosmetic.shared packs
        /// separately, every other group packs together.
        /// </summary>
        public static void ConfigureGroup(AddressableAssetSettings settings, AddressableAssetGroup group)
        {
            var bundled = group.GetSchema<BundledAssetGroupSchema>();
            if (bundled == null)
            {
                bundled = (BundledAssetGroupSchema)group.AddSchema(typeof(BundledAssetGroupSchema), true);
            }
            var update = group.GetSchema<ContentUpdateGroupSchema>();
            if (update == null)
            {
                update = (ContentUpdateGroupSchema)group.AddSchema(typeof(ContentUpdateGroupSchema), true);
            }
            var local = AddressableGroups.KindOf(group.Name) == AddressableGroups.GroupKind.Local;
            bundled.BuildPath.SetVariableByName(settings, local ? LocalBuildPathVar : RemoteBuildPathVar);
            bundled.LoadPath.SetVariableByName(settings, local ? LocalLoadPathVar : RemoteLoadPathVar);
            bundled.Compression = local
                ? BundledAssetGroupSchema.BundleCompressionMode.LZ4
                : BundledAssetGroupSchema.BundleCompressionMode.LZMA;
            bundled.BundleMode = group.Name == AddressableGroups.CosmeticShared
                ? BundledAssetGroupSchema.BundlePackingMode.PackSeparately
                : BundledAssetGroupSchema.BundlePackingMode.PackTogether;
            bundled.BundleNaming = BundledAssetGroupSchema.BundleNamingStyle.AppendHash;
            bundled.UseAssetBundleCache = !local;
            bundled.IncludeAddressInCatalog = true;
            bundled.IncludeGUIDInCatalog = true;
            bundled.IncludeLabelsInCatalog = true;
            bundled.IncludeInBuild = true;
            update.StaticContent = local;
        }

        /// <summary>
        /// Pre-build validation per client_assets.md § Build/CI Validation:
        /// the group set is exactly the canonical 29, every non-localization
        /// entry address parses as a stable asset key and belongs to exactly
        /// its canonical group, no duplicate keys or bundle copies, and no
        /// alias resolves to an alias. Returns all violations; empty = PASS.
        /// </summary>
        public static List<string> VerifyCatalog(AddressableAssetSettings settings)
        {
            var errors = new List<string>();
            var seenGroups = new HashSet<string>();
            foreach (var group in settings.groups)
            {
                if (!AddressableGroups.IsCanonicalName(group.Name))
                {
                    errors.Add("non-canonical group '" + group.Name + "'");
                }
                if (!seenGroups.Add(group.Name))
                {
                    errors.Add("duplicate group '" + group.Name + "'");
                }
            }
            foreach (var name in AddressableGroups.CanonicalNames())
            {
                if (settings.FindGroup(name) == null)
                {
                    errors.Add("missing canonical group '" + name + "'");
                }
            }

            var addressToEntry = new Dictionary<string, AddressableAssetEntry>();
            var guidToGroup = new Dictionary<string, AddressableAssetGroup>();
            foreach (var group in settings.groups)
            {
                var exempt = AddressableGroups.IsLocalizationGroup(group.Name);
                foreach (var entry in group.entries)
                {
                    if (guidToGroup.TryGetValue(entry.guid, out var firstGroup) && firstGroup != group)
                    {
                        errors.Add("asset " + entry.guid + " present in two groups (" + firstGroup.Name + ", " + group.Name + ")");
                    }
                    guidToGroup[entry.guid] = group;
                    if (addressToEntry.TryGetValue(entry.address, out _))
                    {
                        errors.Add("duplicate address '" + entry.address + "'");
                    }
                    else
                    {
                        addressToEntry[entry.address] = entry;
                    }
                    if (exempt)
                    {
                        continue;
                    }
                    if (!AssetKey.TryParse(entry.address, out var key, out var parseError))
                    {
                        errors.Add("key '" + entry.address + "': " + parseError);
                        continue;
                    }
                    var assigned = KeyGroupRule.Assign(key);
                    if (assigned == null)
                    {
                        errors.Add("unmapped key '" + entry.address + "'");
                    }
                    else if (assigned != group.Name)
                    {
                        errors.Add("key '" + entry.address + "' in group '" + group.Name + "', canonical group is '" + assigned + "'");
                    }
                }
            }

            foreach (var kv in addressToEntry)
            {
                var path = AssetDatabase.GUIDToAssetPath(kv.Value.guid);
                var obj = AssetDatabase.LoadMainAssetAtPath(path);
                if (obj is PresentationAlias && !PresentationAlias.TryResolve(kv.Key, AliasTargetOf(addressToEntry), out _))
                {
                    errors.Add("alias chain longer than one hop at '" + kv.Key + "'");
                }
            }

            if (AssetDatabase.LoadAssetAtPath<UnityEngine.Object>(AddressableProvisioner.DefaultObjectPath) != null)
            {
                errors.Add("package DefaultObject.asset must not exist (ADR-0074)");
            }
            return errors;
        }

        private static System.Func<string, string?> AliasTargetOf(Dictionary<string, AddressableAssetEntry> addressToEntry)
        {
            return key =>
            {
                if (!addressToEntry.TryGetValue(key, out var entry))
                {
                    return null;
                }
                var obj = AssetDatabase.LoadMainAssetAtPath(AssetDatabase.GUIDToAssetPath(entry.guid));
                return obj is PresentationAlias alias ? alias.target_key : null;
            };
        }
    }
}
