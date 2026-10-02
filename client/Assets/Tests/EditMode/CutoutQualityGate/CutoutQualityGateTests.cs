using System.IO;
using NUnit.Framework;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.CutoutQualityGate
{
    /// <summary>
    /// Cutout Quality Gate fixtures (manifest §3.2): one failing fixture
    /// per measured row plus the clean pass, the corner rule scoped by
    /// asset_class (ART-001) and the atlas padding/post-compression fringe
    /// rule (ART-009).
    /// </summary>
    public class CutoutQualityGateTests
    {
        private static string RepoRoot()
        {
            var dir = Path.GetFullPath(Path.Combine(Application.dataPath, ".."));
            while (dir != null && !Directory.Exists(Path.Combine(dir, "docs")))
            {
                dir = Directory.GetParent(dir)?.FullName;
            }
            Assert.IsNotNull(dir, "repo root not found above Assets");
            return dir!;
        }

        private static LabPixels.Image Load(string name)
        {
            var path = Path.Combine(RepoRoot(), "client", "Assets", "Tests",
                "EditMode", "CutoutQualityGate", "Fixtures", name);
            Assert.IsTrue(File.Exists(path), "fixture missing: " + name);
            return ArtRuleFixtures.LoadPng(path);
        }

        private static ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.FileReport Measure(
            string name, int cell, ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass cls)
        {
            var img = Load(name);
            return ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.Measure(
                name, img, 0, 0, cell, cell, cls, null, false, false);
        }

        [Test]
        public void TestCornerRuleScopedByAssetClass()
        {
            var actor = Measure("baked_checkerboard_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.IsTrue(actor.Violations.Exists(v => v.Contains("corner")),
                "baked checkerboard must fail the 4-corner rule on ACTOR");
            var ui = Measure("baked_checkerboard_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.UiArt);
            Assert.IsFalse(ui.Violations.Exists(v => v.Contains("corner")),
                "UI_ART must not apply the 4-corner rule (ART-001 scope)");
            var tile = Measure("baked_checkerboard_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Tile);
            Assert.IsFalse(tile.Violations.Exists(v => v.Contains("corner")),
                "TILE must not apply the 4-corner rule (solid edge allowed)");
            var clean = Measure("clean_pass.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.IsFalse(clean.Violations.Exists(v => v.Contains("corner")),
                "clean fixture must have zero corner alpha");
        }

        [Test]
        public void TestAtlasPaddingAndPostCompressionFringe()
        {
            Assert.GreaterOrEqual(4, 4, "SpriteAtlas padding must be >= 4 texture px");
            var fringe = Measure("magenta_fringe_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.Greater(fringe.FringeHueViolations, 0,
                "magenta fringe must be detected after compression");
            var white = Measure("white_fringe_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.Greater(white.FringeDeltaLViolations, 0,
                "white fringe (|dL*| > 35) must be detected");
        }

        [Test]
        public void TestSemiBandAndHalo()
        {
            var halo = Measure("halo_band_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.Greater(halo.SemiBandViolations, 0,
                "halo band >2px from opaque must be detected");
        }

        [Test]
        public void TestTransparentRgbDilation()
        {
            var und = Measure("undilated_transparent_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.Greater(und.TransparentRgbViolations, 0,
                "undilated transparent RGB must be detected");
        }

        [Test]
        public void TestSpeckAndInteriorHole()
        {
            var spk = Measure("speck_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.Greater(spk.SpeckComponents, 0,
                "stray speck < 64px must be detected");
            var hole = Measure("interior_hole_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.Greater(hole.InteriorHoles, 0,
                "enclosed a < 250 hole must be detected");
        }

        [Test]
        public void TestBinaryJaggedEdge()
        {
            var bin = Measure("binary_edge_fail.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.IsTrue(bin.Violations.Exists(v => v.Contains("binary")),
                "binary 0/255 edge must be rejected");
        }

        [Test]
        public void TestCellSize()
        {
            var img = Load("wrong_size_fail.png");
            var rep = ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.Measure(
                "wrong_size_fail.png", img, 0, 0, 128, 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor,
                null, false, false);
            Assert.Greater(rep.Violations.Count, 0,
                "64px silhouette must fail the §3.2 size rules (body 176..192)");
        }

        [Test]
        public void TestCleanFixturePasses()
        {
            var clean = Measure("clean_pass.png", 128,
                ThinhThan.Core.Assets.Editor.AssetProduction.CutoutQualityGate.AssetClass.Actor);
            Assert.AreEqual(0, clean.Violations.Count,
                "clean fixture must pass every measured row: "
                + string.Join("; ", clean.Violations));
        }
    }
}
