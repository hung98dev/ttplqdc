using System.Collections.Generic;
using System.IO;
using System.Security.Cryptography;
using System.Text;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.AddressablesValidation
{
    /// <summary>
    /// Catalog-level invariants (client_assets.md § Grouping/§ Stable Asset
    /// Keys, presentation_asset_manifest.md §1/§3, ADR-0071): group budgets
    /// and totals, size_profile rows, playable-space routing coverage and the
    /// path-derived baseline GUID of the settings asset.
    /// </summary>
    public class AddressablesValidationTests
    {
        private const long Mib = 1024L * 1024L;

        private static string RepoRoot()
        {
            var dir = Path.GetFullPath(Path.Combine(Application.dataPath, ".."));
            while (dir != null && !Directory.Exists(Path.Combine(dir, "docs")))
            {
                dir = Directory.GetParent(dir)?.FullName;
            }
            Assert.IsNotNull(dir, "repo root (containing docs/) not found above Assets");
            return dir!;
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

        [Test]
        public void TestCatalogAssetKeyResolution()
        {
            Assert.IsTrue(AssetKey.TryParse("asset.monster.lang_da.ghost_dog.prefab", out var monster));
            Assert.IsTrue(monster.CatalogBacked);
            Assert.AreEqual("monster.lang_da.ghost_dog", monster.CatalogId);
            Assert.AreEqual(AssetKey.KeyFacet.Prefab, monster.Facet);
            Assert.AreEqual("region.lang_da", KeyGroupRule.Assign(monster));

            Assert.IsTrue(AssetKey.TryParse("asset.map.pvp.duel_court.scene", out var pvp));
            Assert.AreEqual("map.pvp.duel_court", pvp.CatalogId);
            Assert.AreEqual(AddressableGroups.PvpShared, KeyGroupRule.Assign(pvp));

            Assert.IsTrue(AssetKey.TryParse("asset.instance.finale.than_trung.scene", out var finale));
            Assert.AreEqual(AddressableGroups.DungeonFinale, KeyGroupRule.Assign(finale));

            Assert.IsTrue(AssetKey.TryParse("asset.item.gong_ren.icon", out var icon));
            Assert.AreEqual(AssetKey.KeyFacet.Icon, icon.Facet);
            Assert.AreEqual(AddressableGroups.IconsShared, KeyGroupRule.Assign(icon));

            Assert.IsFalse(AssetKey.TryParse("asset.item.gong_ren.icon.hd", out _),
                "variant segment after the facet must fail");
            Assert.IsFalse(AssetKey.TryParse("asset.Item.gong_ren.icon", out _), "uppercase must fail");
            Assert.IsFalse(AssetKey.TryParse("asset.ui.credits.third_party_assets", out _), "obsolete key must fail");
        }

        [Test]
        public void TestAddressableGroupBudgets()
        {
            Assert.AreEqual(AddressableGroups.CanonicalGroupCount, GroupBudgets.Budgets.Count,
                "every canonical group must have a budget row");
            foreach (var name in AddressableGroups.CanonicalNames())
            {
                Assert.IsTrue(GroupBudgets.Budgets.ContainsKey(name), "missing budget for " + name);
                var budget = GroupBudgets.Budgets[name];
                Assert.Greater(budget.CompressedMaxBytes, 0, name);
                Assert.Greater(budget.RamMaxBytes, 0, name);
            }
            Assert.AreEqual(70 * Mib, GroupBudgets.Budgets[AddressableGroups.SharedLocal].CompressedMaxBytes);
            Assert.AreEqual(120 * Mib, GroupBudgets.Budgets["region.nui_thieng"].RamMaxBytes);
            Assert.AreEqual(AddressableGroups.GroupKind.Local,
                GroupBudgets.Budgets[AddressableGroups.LocalizationStringsViVn].Kind);
            Assert.AreEqual(AddressableGroups.GroupKind.Remote,
                GroupBudgets.Budgets["region.lang_da"].Kind);
        }

        [Test]
        public void TestCanonicalSpriteImportProfiles()
        {
            var profiles = SpriteImportRules.Profiles;
            Assert.AreEqual(8, profiles.Count, "manifest §3 declares 8 size_profile rows");

            var boss = profiles[SpriteImportRules.SizeProfile.BossLarge];
            Assert.AreEqual(200, boss.SilhouetteW);
            Assert.AreEqual(220, boss.SilhouetteH);
            Assert.AreEqual(256, boss.CellW);
            Assert.AreEqual(512, boss.TextureW, "texture_2x = 2 x cell");
            Assert.AreEqual(120, boss.ColliderW);

            var npc = profiles[SpriteImportRules.SizeProfile.NpcHumanoid];
            Assert.AreEqual(96, npc.CellW);
            Assert.AreEqual(192, npc.TextureW);
            Assert.AreEqual(0, npc.ColliderW, "NPC has no combat collider box");

            foreach (var kv in profiles)
            {
                var info = kv.Value;
                Assert.AreEqual(info.CellW * SpriteImportRules.TextureScale, info.TextureW, kv.Key + " texture width");
                Assert.AreEqual(info.CellH * SpriteImportRules.TextureScale, info.TextureH, kv.Key + " texture height");
                Assert.Greater(info.SilhouetteW, 0, kv.Key + " silhouette");
            }
            Assert.AreEqual(50, SpriteImportRules.ReferencePixelsPerMeter);
            Assert.AreEqual(0.5f, SpriteImportRules.PivotX);
            Assert.AreEqual(0.0f, SpriteImportRules.PivotY);
        }

        [Test]
        public void TestPlayableSceneKeyCoverage()
        {
            var spaces = new List<string>();
            foreach (var zone in AddressableGroups.ZoneKeys)
            {
                spaces.Add("map." + zone + ".entrance");
            }
            foreach (var dungeon in AddressableGroups.DungeonKeys)
            {
                spaces.Add("dungeon." + dungeon);
            }
            spaces.Add("instance.finale.than_trung");
            spaces.Add("map.pvp.duel_court");
            spaces.Add("map.pvp.five_element_arena");
            spaces.Add("map.guild_war.five_seal_conflict");

            foreach (var spaceId in spaces)
            {
                var group = AddressableGroups.SpaceGroup(spaceId);
                Assert.IsNotNull(group, "unmapped playable space " + spaceId);
                Assert.IsTrue(AddressableGroups.IsCanonicalName(group!), spaceId + " -> " + group);

                Assert.IsTrue(AssetKey.TryParse("asset." + spaceId + ".scene", out var key),
                    "scene key must parse: " + spaceId);
                Assert.AreEqual(group, KeyGroupRule.Assign(key), "scene key routing for " + spaceId);
            }
        }

        [Test]
        public void TestSettingsAssetMatchesBaselineGuid()
        {
            const string settingsRepoPath = "client/Assets/AddressableAssetsData/AddressableAssetSettings.asset";
            var expected = DerivedGuid(settingsRepoPath);
            var metaPath = Path.Combine(RepoRoot(), "client/Assets/AddressableAssetsData/AddressableAssetSettings.asset.meta");
            Assert.IsTrue(File.Exists(metaPath), "missing " + metaPath);
            StringAssert.Contains(
                "guid: " + expected,
                File.ReadAllText(metaPath),
                "settings meta must pin the path-derived baseline GUID");
            Assert.AreEqual("03d05df79b43898f254cff5dad608436", expected,
                "baseline GUID drifted — check repo-relative path");
        }
    }
}
