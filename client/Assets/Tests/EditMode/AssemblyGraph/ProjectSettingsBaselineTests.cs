using System.IO;
using System.Security.Cryptography;
using System.Text;
using NUnit.Framework;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.AssemblyGraph
{
    /// <summary>
    /// ProjectSettings baseline contract (repository_layout.md § Project
    /// Settings Baseline): path-derived GUIDs are the first 32 lowercase hex
    /// characters of SHA-256 over the asset's repository-relative path
    /// (UTF-8, '/' separators).
    /// </summary>
    public class ProjectSettingsBaselineTests
    {
        private static readonly string ProjectSettingsDir =
            Path.GetFullPath(Path.Combine(Application.dataPath, "..", "ProjectSettings"));

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

        private static string ReadAsset(string name)
        {
            var path = Path.Combine(ProjectSettingsDir, name);
            Assert.IsTrue(File.Exists(path), "missing " + path);
            return File.ReadAllText(path);
        }

        private static void AssertContains(string haystack, string needle, string context)
        {
            StringAssert.Contains(needle, haystack, context);
        }

        [Test]
        public void TestEditorBuildSettingsConfigObjects()
        {
            var text = ReadAsset("EditorBuildSettings.asset");
            var addrGuid = DerivedGuid("client/Assets/AddressableAssetsData/AddressableAssetSettings.asset");
            var locGuid = DerivedGuid("client/Assets/Localization/Settings/LocalizationSettings.asset");
            AssertContains(
                text,
                "com.unity.addressableassets: {fileID: 11400000, guid: " + addrGuid + ", type: 2}",
                "addressables config-object slot");
            AssertContains(
                text,
                "com.unity.localization.settings: {fileID: 11400000, guid: " + locGuid + ", type: 2}",
                "localization config-object slot");
        }

        [Test]
        public void TestGraphicsSettingsUrpSlot()
        {
            var text = ReadAsset("GraphicsSettings.asset");
            var urpGuid = DerivedGuid("client/Assets/Settings/Rendering/ThinhThanURP.asset");
            AssertContains(
                text,
                "m_CustomRenderPipeline: {fileID: 11400000, guid: " + urpGuid + ", type: 2}",
                "URP render pipeline slot");
        }

        [Test]
        public void TestServerGeometryTag()
        {
            var text = ReadAsset("TagManager.asset");
            AssertContains(text, "- ServerGeometry", "ServerGeometry tag");
        }

        [Test]
        public void TestPlayerAndPhysics2DBaseline()
        {
            var player = ReadAsset("ProjectSettings.asset");
            AssertContains(player, "gcIncremental: 1", "incremental GC");
            AssertContains(
                player,
                "androidUseSwappy: 1",
                "Android Optimized Frame Pacing (androidUseSwappy serialization)");
            var physics2d = ReadAsset("Physics2DSettings.asset");
            AssertContains(
                physics2d,
                "m_SimulationMode: 2",
                "Physics2D simulationMode = Script (SimulationMode2D.Script)");
        }
    }
}
