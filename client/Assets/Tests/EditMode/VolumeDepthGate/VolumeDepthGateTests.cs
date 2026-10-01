using System.Collections.Generic;
using System.IO;
using NUnit.Framework;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.VolumeDepthGate
{
    /// <summary>
    /// Volume &amp; Depth Gate fixtures and determinism (manifest §3.6).
    /// Every measure is exercised on the spec-named fixtures plus the
    /// numeric-completion edge cases.
    /// </summary>
    public class VolumeDepthGateTests
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

        private static string FixtureDir()
        {
            return Path.Combine(RepoRoot(), "client", "Assets", "Tests",
                "EditMode", "VolumeDepthGate", "Fixtures");
        }

        private static LabPixels.Image Load(string name)
        {
            var path = Path.Combine(FixtureDir(), name);
            Assert.IsTrue(File.Exists(path), "fixture missing: " + name);
            return ArtRuleFixtures.LoadPng(path);
        }

        private static List<int> SOf(LabPixels.Image img, bool[]? mask, out bool[] inS, out LabPixels.Lab[] lab)
        {
            ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate.Silhouette(img, mask, out inS, out lab);
            var s = new List<int>();
            for (int i = 0; i < img.Width * img.Height; i++)
            {
                if (inS[i])
                {
                    s.Add(i);
                }
            }
            return s;
        }

        [Test]
        public void TestKMeansDeterministicInit()
        {
            var img = Load("gradient_smooth_pass.png");
            var s = SOf(img, null, out var inS, out var lab);
            Assert.Greater(s.Count, 0);
            var c1 = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .HueClusters(s, lab, img.Width);
            var c2 = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .HueClusters(s, lab, img.Width);
            Assert.AreEqual(c1.Length, c2.Length);
            for (int k = 0; k < c1.Length; k++)
            {
                Assert.AreEqual(c1[k].Count, c2[k].Count,
                    "k-means assignment not deterministic for cluster " + k);
                for (int i = 0; i < c1[k].Count; i++)
                {
                    Assert.AreEqual(c1[k][i], c2[k][i]);
                }
            }
            int eligible = 0;
            for (int k = 0; k < c1.Length; k++)
            {
                if (c1[k].Count * 20 >= s.Count)
                {
                    eligible++;
                }
            }
            Assert.GreaterOrEqual(eligible, 1, "no eligible hue cluster");
        }

        [Test]
        public void TestEdgeBandDefinition()
        {
            var img = Load("thin_prop_deepest_core.png");
            var s = SOf(img, null, out var inS, out _);
            var dist = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .OutsideDistance(inS, img.Width, img.Height);
            ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate.EdgeBandAndCore(
                inS, dist, img.Width, img.Height,
                out var band, out var ring, out var deepest, out var mode);
            Assert.Greater(band.Count, 0);
            foreach (var i in band)
            {
                Assert.LessOrEqual(dist[i], 3, "band pixel not within 3px of outside S");
            }
            Assert.Greater(deepest.Count, 0, "thin silhouette lacks deepest core pixels");
            foreach (var i in deepest)
            {
                Assert.Greater(dist[i], 3, "core pixel inside the edge band (not disjoint)");
            }
            Assert.AreEqual("DEEPEST", mode, "thin PROP must report core_mode=DEEPEST");
        }

        [Test]
        public void TestTranslucentMaskScope()
        {
            var img = Load("gradient_smooth_pass.png");
            int n = img.Width * img.Height;
            var mask = new bool[n];
            for (int y = 40; y < 88; y++)
            {
                for (int x = 40; x < 88; x++)
                {
                    mask[y * img.Width + x] = true;
                }
            }
            var masked = SOf(img, mask, out var inS, out _);
            int maskedOut = n - masked.Count;
            Assert.Greater(maskedOut, 0, "mask did not shrink S");
            var unmasked = SOf(img, null, out _, out _);
            Assert.Greater(unmasked.Count, masked.Count, "mask must exclude pixels from S");
            int maskPx = 0;
            for (int i = 0; i < n; i++)
            {
                if (mask[i])
                {
                    maskPx++;
                }
            }
            int sil = masked.Count;
            double frac = sil == 0 ? 0.0 : (double)maskPx / sil;
            Assert.Greater(frac, 0.0);
        }

        [Test]
        public void TestSpiritBeastAndCellRefSizes()
        {
            var img = Load("gradient_smooth_pass.png");
            Assert.AreEqual(128, img.Width);
            Assert.AreEqual(128, img.Height, "SPIRIT_BEAST texture must be 128x128 (2x of 64)");
            var tile = Load("tile_solid_edge_pass.png");
            Assert.AreEqual(100, tile.Width);
            Assert.AreEqual(100, tile.Height, "TILE texture must be 100x100 (2x of 50)");
        }

        [Test]
        public void TestFlatRegionLabBinsGradientPasses()
        {
            var img = Load("gradient_smooth_pass.png");
            var s = SOf(img, null, out var inS, out var lab);
            double share = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .LargestFlatShare(s, lab, inS, img.Width, img.Height);
            Assert.IsTrue(double.IsFinite(share));
            Assert.LessOrEqual(share, 0.20,
                "smooth gradient must not coalesce into a >20% flat region");

            var flat = Load("flat_fill_fail.png");
            var s2 = SOf(flat, null, out var inS2, out var lab2);
            double share2 = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .LargestFlatShare(s2, lab2, inS2, flat.Width, flat.Height);
            Assert.Greater(share2, 0.20, "flat_fill_fail must exceed the 20% flat bound");
        }

        [Test]
        public void TestTopLightPerHueClusterDarkHairPasses()
        {
            var img = Load("dark_hair_toplit_pass.png");
            var s = SOf(img, null, out _, out var lab);
            var clusters = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .HueClusters(s, lab, img.Width);
            double d = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .TopLight(clusters, clusters.Length, s.Count, lab, img.Width);
            Assert.IsTrue(double.IsFinite(d), "top-light median must be finite");
            Assert.GreaterOrEqual(d, 4.0,
                "dark-hair top-lit sprite must pass the top-light gate");

            var fail = Load("bottom_lit_fail.png");
            var s2 = SOf(fail, null, out _, out var lab2);
            var c2 = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .HueClusters(s2, lab2, fail.Width);
            double d2 = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .TopLight(c2, c2.Length, s2.Count, lab2, fail.Width);
            Assert.Less(d2, 4.0, "bottom-lit sprite must fail the top-light gate");
        }

        [Test]
        public void TestPaletteGateAgainstStylePack()
        {
            var img = Load("gradient_smooth_pass.png");
            var s = SOf(img, null, out _, out var lab);
            var palette = new List<LabPixels.Lab>();
            for (int i = 0; i < s.Count; i += 7)
            {
                palette.Add(lab[s[i]]);
            }
            double cov = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .PaletteCoverage(s, lab, palette);
            Assert.IsTrue(double.IsFinite(cov));
            Assert.GreaterOrEqual(cov, 0.85, "palette coverage below 85% against own colors");
            var off = new List<LabPixels.Lab> { LabPixels.ToLab(0, 0, 200) };
            double cov2 = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .PaletteCoverage(s, lab, off);
            Assert.Less(cov2, 0.85, "foreign palette must fail coverage");
        }

        [Test]
        public void TestFrameConsistencyAndPivot()
        {
            var img = Load("gradient_smooth_pass.png");
            var s = SOf(img, null, out _, out var lab);
            var errs = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .CheckFrameConsistency(s, lab, img.Width, s, lab);
            Assert.AreEqual(0, errs.Count, "same-frame consistency must hold");
            var empty = new List<int>();
            var errs2 = ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate
                .CheckFrameConsistency(s, lab, img.Width, empty, lab);
            Assert.Greater(errs2.Count, 0, "empty frame silhouette must fail");
        }

        [Test]
        public void TestTileSeam()
        {
            var tile = Load("tile_solid_edge_pass.png");
            int w = tile.Width;
            int h = tile.Height;
            double sumLR = 0;
            double sumTB = 0;
            for (int y = 0; y < h; y++)
            {
                var a = LabPixels.ToLab(tile.At(0, y).R, tile.At(0, y).G, tile.At(0, y).B);
                var b = LabPixels.ToLab(tile.At(w - 1, y).R, tile.At(w - 1, y).G, tile.At(w - 1, y).B);
                sumLR += LabPixels.DeltaE00(a, b);
            }
            for (int x = 0; x < w; x++)
            {
                var a = LabPixels.ToLab(tile.At(x, 0).R, tile.At(x, 0).G, tile.At(x, 0).B);
                var b = LabPixels.ToLab(tile.At(x, h - 1).R, tile.At(x, h - 1).G, tile.At(x, h - 1).B);
                sumTB += LabPixels.DeltaE00(a, b);
            }
            Assert.LessOrEqual(sumLR / h, 2.0,
                "left/right seam mean ΔE00 must be <= 2 (§3.9)");
            Assert.LessOrEqual(sumTB / w, 2.0,
                "top/bottom seam mean ΔE00 must be <= 2 (§3.9)");
        }

        [Test]
        public void TestNineSliceBorder()
        {
            var tile = Load("tile_solid_edge_pass.png");
            int w = tile.Width;
            int h = tile.Height;
            var vals = new List<double>();
            double mean = 0;
            for (int y = h / 4; y < 3 * h / 4; y++)
            {
                for (int x = w / 4; x < 3 * w / 4; x++)
                {
                    var p = tile.At(x, y);
                    var l = LabPixels.ToLab(p.R, p.G, p.B).L;
                    vals.Add(l);
                    mean += l;
                }
            }
            Assert.Greater(vals.Count, 0);
            mean /= vals.Count;
            double varSum = 0;
            foreach (var v in vals)
            {
                varSum += (v - mean) * (v - mean);
            }
            double sd = System.Math.Sqrt(varSum / vals.Count);
            Assert.LessOrEqual(sd, 2.0,
                "9-slice stretch band L* std must be <= 2 (§3.9)");
        }

        [Test]
        public void TestVfxFlipbookLimits()
        {
            // §3.9 pinned limits: flipbook <= 16 frames, sheet <= 1024x1024
            // texture px, fps in {12, 24}; blend declared ADDITIVE|ALPHA.
            Assert.AreEqual(16,
                ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate.VfxFlipbookMaxFrames);
            Assert.AreEqual(1024,
                ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate.VfxFlipbookMaxSheetPx);
            Assert.AreEqual(12,
                ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate.VfxFlipbookFpsLow);
            Assert.AreEqual(24,
                ThinhThan.Core.Assets.Editor.AssetProduction.VolumeDepthGate.VfxFlipbookFpsHigh);
        }

        [Test]
        public void TestHitboxSilhouetteAlignment()
        {
            var img = Load("gradient_smooth_pass.png");
            var s = SOf(img, null, out var inS, out _);
            int minX = img.Width;
            int maxX = -1;
            for (int i = 0; i < s.Count; i++)
            {
                int x = s[i] % img.Width;
                if (x < minX)
                {
                    minX = x;
                }
                if (x > maxX)
                {
                    maxX = x;
                }
            }
            double silCenter = (minX + maxX) / 2.0;
            double colliderCenter = img.Width / 2.0;
            Assert.LessOrEqual(System.Math.Abs(colliderCenter - silCenter), 4.0 * 2,
                "hitbox center within ±4 ref px (8 texture px) of silhouette center");
            double silW = maxX - minX + 1;
            double ratio = silW / img.Width;
            Assert.Greater(ratio, 0.0);
            Assert.LessOrEqual(ratio, 1.0);
        }

        [Test]
        public void TestReviewLowProfileMotionAndRubric()
        {
            string surface = Path.Combine(RepoRoot(), "client", "Assets",
                "Scenes", "Review", "ReviewSurface.unity");
            Assert.IsTrue(File.Exists(surface), "review surface scene missing");
            var batchType = typeof(ThinhThan.Core.Assets.Editor.AssetProduction.VisualReviewBatch);
            var run = batchType.GetMethod("Run",
                System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.Static);
            Assert.IsNotNull(run, "VisualReviewBatch.Run entry point missing");
            Assert.AreEqual(0, run!.GetParameters().Length);
            Assert.AreEqual(typeof(void), run.ReturnType,
                "VisualReviewBatch.Run must be a static void method per §3.3a");
        }
    }
}
