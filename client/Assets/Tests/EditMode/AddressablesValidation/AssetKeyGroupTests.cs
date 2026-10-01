using System.Collections.Generic;
using NUnit.Framework;
using ThinhThan.Core.Assets;

namespace ThinhThan.Tests.EditMode.AddressablesValidation
{
    /// <summary>
    /// Key-grammar and key-to-group rule invariants (client_assets.md § Stable
    /// Asset Keys + § Grouping; presentation_asset_manifest.md §1 totals).
    /// </summary>
    public class AssetKeyGroupTests
    {
        private const long Mib = 1024L * 1024L;

        [Test]
        public void TestKeyDerivationRule()
        {
            var cases = new (string key, string group)[]
            {
                ("asset.ui.boot.prefab", AddressableGroups.BootstrapLocal),
                ("asset.ui.hud.prefab", AddressableGroups.SharedLocal),
                ("asset.font.main.font", AddressableGroups.BootstrapLocal),
                ("asset.sfx.just_guard_success.clip", AddressableGroups.SharedLocal),
                ("asset.bgm.lang_da.forest.bgm", "audio.bgm.lang_da"),
                ("asset.bgm.menu_main.bgm", AddressableGroups.AudioBgmShared),
                ("asset.prop.lang_da.crate.prefab", "region.lang_da"),
                ("asset.tile.rung_u_minh.base.sprite", "region.rung_u_minh"),
                ("asset.parallax.deo_may.layer_far.sprite", "region.deo_may"),
                ("asset.vfx.den_tran.seal_burst.vfx", "region.den_tran"),
                ("asset.vfx.beast.contract_burst.vfx", AddressableGroups.BeastShared),
                ("asset.vfx.pvp.duel_countdown.vfx", AddressableGroups.PvpShared),
                ("asset.dungeon.hang_ma_tranh.scene", "dungeon.hang_ma_tranh"),
                ("asset.dungeon.finale.scene", AddressableGroups.DungeonFinale),
                ("asset.boss.thuong_luong.portrait", "dungeon.xom_chim"),
                ("asset.boss.ma_da_chua.portrait", "region.ben_nuoc_den"),
                ("asset.boss.than_trung.portrait", AddressableGroups.DungeonFinale),
                ("asset.beast.ho_ly.prefab", AddressableGroups.BeastShared),
                ("asset.cosmetic.armor_linhsu.prefab", AddressableGroups.CosmeticShared),
                ("asset.cosmetic.armor_linhsu.icon", AddressableGroups.IconsShared),
                ("asset.skill.tiep_tam_chuong.icon", AddressableGroups.IconsShared),
                ("asset.skill.tiep_tam_chuong.sprite", AddressableGroups.SharedLocal),
                ("asset.status.just_guard.icon", AddressableGroups.IconsShared),
                ("asset.map.ben_nuoc_den.ben_do_cu.bgm", "audio.bgm.ben_nuoc_den"),
                ("asset.dungeon.mieu_ba_trong_rung.bgm", "audio.bgm.rung_u_minh"),
                ("asset.instance.finale.than_trung.bgm", "audio.bgm.nui_thieng"),
            };
            foreach (var (key, expectedGroup) in cases)
            {
                Assert.IsTrue(AssetKey.TryParse(key, out var parsed), "parse failed: " + key);
                Assert.AreEqual(expectedGroup, KeyGroupRule.Assign(parsed), "group for " + key);
            }
            Assert.IsNull(KeyGroupRule.Assign(Parse("asset.quest.daily_sprite_hunt.prefab")),
                "unmapped catalog kind must return null");
        }

        [Test]
        public void TestCanonicalGroupSetAndSingleMembership()
        {
            var names = AddressableGroups.CanonicalNames();
            Assert.AreEqual(29, names.Count);
            var unique = new HashSet<string>(names);
            Assert.AreEqual(29, unique.Count, "canonical group names must be unique");
            foreach (var name in names)
            {
                Assert.IsTrue(AddressableGroups.IsCanonicalName(name), name);
                Assert.IsTrue(GroupBudgets.Budgets.ContainsKey(name), name + " budget");
            }
            Assert.IsTrue(AddressableGroups.IsCanonicalName("audio.bgm.thanh_co"));
            Assert.IsTrue(AddressableGroups.IsCanonicalName("dungeon.den_tran"));
            Assert.IsFalse(AddressableGroups.IsCanonicalName("Localization-Locales"));
            Assert.IsFalse(AddressableGroups.IsCanonicalName("region.sai_gon"));

            // A parseable key resolves to exactly one canonical group.
            Assert.IsTrue(AssetKey.TryParse("asset.npc.lang_da.elder.prefab", out var key));
            var assigned = KeyGroupRule.Assign(key);
            Assert.AreEqual("region.lang_da", assigned);
            Assert.IsTrue(unique.Contains(assigned!));
        }

        [Test]
        public void TestPresentationAliasSingleHop()
        {
            var aliases = new Dictionary<string, string>
            {
                ["asset.equipment.starter_sword.sprite"] = "asset.prop.shared.starter_sword.sprite",
                ["asset.prop.shared.cursed_blade.sprite"] = "asset.prop.shared.real_blade.sprite",
            };
            string? AliasOf(string key)
            {
                return aliases.TryGetValue(key, out var target) ? target : null;
            }

            Assert.IsTrue(PresentationAlias.TryResolve("asset.equipment.starter_sword.sprite", AliasOf, out var resolved));
            Assert.AreEqual("asset.prop.shared.starter_sword.sprite", resolved);

            Assert.IsTrue(PresentationAlias.TryResolve("asset.ui.hud.sprite", AliasOf, out var direct));
            Assert.AreEqual("asset.ui.hud.sprite", direct, "non-alias keys resolve to themselves");

            aliases["asset.prop.shared.starter_sword.sprite"] = "asset.prop.shared.cursed_blade.sprite";
            Assert.IsFalse(PresentationAlias.TryResolve("asset.equipment.starter_sword.sprite", AliasOf, out _),
                "alias -> alias must fail validation");
        }

        [Test]
        public void TestDeterministicGroupRamBudgets()
        {
            // texture format x size x mips (256x256 RGBA32, no mips).
            Assert.AreEqual(256L * 256 * 4, GroupBudgets.EstimateTextureBytes(256, 256, GroupBudgets.TexFormat.Rgba32, 1));
            // ASTC 4x4 = 16 bytes per 4x4 block; 256x256 -> 64x64 blocks.
            Assert.AreEqual(64L * 64 * 16, GroupBudgets.EstimateTextureBytes(256, 256, GroupBudgets.TexFormat.Astc4X4, 1));
            // BC7 4x4 blocks; non-multiple sizes round up: 5x5 -> 2x2 blocks.
            Assert.AreEqual(2L * 2 * 16, GroupBudgets.EstimateTextureBytes(5, 5, GroupBudgets.TexFormat.Bc7, 1));
            // mips accumulate: 4x4 RGBA32 + 2x2 + 1x1.
            Assert.AreEqual(4L * 4 * 4 + 2 * 2 * 4 + 1 * 1 * 4, GroupBudgets.EstimateTextureBytes(4, 4, GroupBudgets.TexFormat.Rgba32, 3));
            // mesh = vertices + indices.
            Assert.AreEqual(100L * 32 + 300 * 2, GroupBudgets.EstimateMeshBytes(100, 32, 300, 2));
            // audio = seconds x rate x channels x 16-bit PCM.
            Assert.AreEqual(60L * 44100 * 2 * 2, GroupBudgets.EstimateAudioBytes(60.0, 2, 44100));
        }

        [Test]
        public void TestResidentSteadyAndTransferPeak()
        {
            var steady = GroupBudgets.ResidentSteadyWorstCaseBytes();
            var peak = GroupBudgets.TransferPeakWorstCaseBytes();
            var baseInstall = GroupBudgets.BaseInstallCompressedBytes();

            Assert.AreEqual(400 * Mib, steady, "resident steady worst case");
            Assert.AreEqual(524 * Mib, peak, "transfer peak worst case");
            Assert.AreEqual(92 * Mib, baseInstall, "base install payload");
            Assert.LessOrEqual(steady, GroupBudgets.ResidentSteadyMaxBytes);
            Assert.LessOrEqual(peak, GroupBudgets.TransferPeakMaxBytes);
            Assert.LessOrEqual(baseInstall, GroupBudgets.BaseInstallMaxBytes);
            Assert.Greater(peak, steady);
        }

        [Test]
        public void TestMeshTypeRule()
        {
            // Tight iff long side >= 256 texture px AND transparent margins.
            Assert.IsTrue(SpriteImportRules.RequiresTightMesh(256, true));
            Assert.IsTrue(SpriteImportRules.RequiresTightMesh(512, true));
            Assert.IsFalse(SpriteImportRules.RequiresTightMesh(255, true), "below the px threshold");
            Assert.IsFalse(SpriteImportRules.RequiresTightMesh(512, false), "opaque margins -> Full Rect");
            // Every size_profile with collider art is Tight; spirit beasts/NPCs may be Full Rect.
            Assert.IsTrue(SpriteImportRules.RequiresTightMesh(
                SpriteImportRules.InfoFor(SpriteImportRules.SizeProfile.WorldBoss).TextureW, true));
            Assert.IsFalse(SpriteImportRules.RequiresTightMesh(
                SpriteImportRules.InfoFor(SpriteImportRules.SizeProfile.SpiritBeast).TextureW, false));
        }

        [Test]
        public void TestParallaxFarPpu50()
        {
            Assert.AreEqual(50, SpriteImportRules.PpuForClass(false, true), "PARALLAX_FAR imports at 50 PPU (1x)");
            Assert.AreEqual(200, SpriteImportRules.PpuForClass(true, false), "UI imports at 200 PPU");
            Assert.AreEqual(100, SpriteImportRules.PpuForClass(false, false), "gameplay sprites at 100 PPU");
            Assert.AreEqual(128, SpriteImportRules.TexturePixels(SpriteImportRules.IconCellRef), "icon 64x64 ref -> 128x128");
            Assert.AreEqual(100, SpriteImportRules.TexturePixels(SpriteImportRules.TileCellRef), "tile 50x50 ref -> 100x100");
        }

        private static AssetKey Parse(string key)
        {
            Assert.IsTrue(AssetKey.TryParse(key, out var parsed), "parse failed: " + key);
            return parsed;
        }
    }
}
