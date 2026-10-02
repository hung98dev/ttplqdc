using System.IO;
using NUnit.Framework;
using ThinhThan.Core.Assets.Editor.AssetProduction;
using UnityEngine;
using UnityEngine.Rendering;

namespace ThinhThan.Tests.EditMode.VolumeDepthGate
{
    /// <summary>
    /// Visual Review batch pass (manifest §3.3/§3.3a). The
    /// -testCategory VisualReview run executes this under -force-d3d11 on
    /// the hosted Windows runner; the plain -nographics EditMode pass also
    /// discovers it, where the null device makes it a clean Ignore instead
    /// of a fake failure.
    /// </summary>
    public class VisualReviewBatchTests
    {
        [Test]
        [Category("VisualReview")]
        public void RunProducesArtifactsUnderD3D11Warp()
        {
            if (SystemInfo.graphicsDeviceType == GraphicsDeviceType.Null)
            {
                Assert.Ignore("requires a real graphics device (-force-d3d11 pass)");
            }
            ThinhThan.Core.Assets.Editor.AssetProduction.VisualReviewBatch.Run();
            var dir = Path.GetFullPath(Path.Combine(
                Application.dataPath, "..", "..", "artifacts", "visual-review"));
            Assert.IsTrue(Directory.Exists(dir), "visual-review artifact dir missing");
            var report = Path.Combine(dir, VisualReviewBatch.ReportFile);
            var fixtures = Path.Combine(dir, VisualReviewBatch.FixturesFile);
            Assert.IsTrue(File.Exists(report), "capture-report.json missing");
            Assert.IsTrue(File.Exists(fixtures), "visual-review-fixtures.xml missing");
            Assert.IsTrue(File.ReadAllText(report).Contains("\"result\":\"PASS\""),
                "capture-report.json must carry result=PASS");
            Assert.Greater(Directory.GetFiles(dir, "*.png").Length, 0,
                "no PNG captures produced");
            Assert.IsTrue(File.ReadAllText(fixtures).Contains("GraphicsFixtures"),
                "fixtures XML must carry the GraphicsFixtures category");
        }
    }
}
