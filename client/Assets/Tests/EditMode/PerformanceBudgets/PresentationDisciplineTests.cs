using System.Collections.Generic;
using System.IO;
using System.Text;
using NUnit.Framework;
using UnityEditor;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.PerformanceBudgets
{
    /// <summary>
    /// PERF-021 rendering discipline (client_performance.md item 6): SRP-batcher
    /// materials, custom transparency sort axis (0,1,0) on the 2D renderer,
    /// Tight mesh on large sprites, Animator CullCompletely, and no runtime
    /// material instances (no `.material` getter, no `new Material`).
    /// </summary>
    [Category("Performance")]
    public sealed class PresentationDisciplineTests
    {
        private const string ArtRoot = "Assets/Art";
        private const string ScriptsRoot = "Assets/Scripts";
        private const string RendererDataPath =
            "Assets/Settings/Rendering/ThinhThan2DRenderer.asset";

        private static readonly string[] SrpShaderPrefixes =
        {
            "Universal Render Pipeline/",
            "Universal ",
            "Sprites/",
            "Particles/",
            "UI/",
            "GUI/",
            "Skybox/",
            "ThinhThan/",
        };

        [Test]
        public void TestSrpBatcherCompatibleMaterials()
        {
            var violations = new List<string>();
            foreach (string path in EnumerateFiles(ArtRoot, "*.mat"))
            {
                var material = AssetDatabase.LoadAssetAtPath<Material>(path);
                if (material == null || material.shader == null)
                {
                    violations.Add(path + " -> material or shader unresolved");
                    continue;
                }

                bool ok = false;
                foreach (string prefix in SrpShaderPrefixes)
                {
                    if (material.shader.name.StartsWith(prefix))
                    {
                        ok = true;
                        break;
                    }
                }

                if (!ok)
                {
                    violations.Add(
                        path + " -> shader '" + material.shader.name + "'");
                }
            }

            foreach (string path in EnumerateFiles("Assets/Settings", "*.mat"))
            {
                var material = AssetDatabase.LoadAssetAtPath<Material>(path);
                if (material == null || material.shader == null)
                {
                    violations.Add(path + " -> material or shader unresolved");
                    continue;
                }

                bool ok = false;
                foreach (string prefix in SrpShaderPrefixes)
                {
                    if (material.shader.name.StartsWith(prefix))
                    {
                        ok = true;
                        break;
                    }
                }

                if (!ok)
                {
                    violations.Add(
                        path + " -> shader '" + material.shader.name + "'");
                }
            }

            Assert.IsEmpty(
                violations,
                "materials must use SRP-batcher-compatible shaders:\n" +
                    string.Join("\n", violations));
        }

        [Test]
        public void TestTransparencySortAxis()
        {
            string text = File.ReadAllText(RendererDataPath);
            StringAssert.Contains(
                "m_TransparencySortMode: 3",
                text,
                "the 2D renderer must use a custom transparency sort axis");
            StringAssert.Contains(
                "m_TransparencySortAxis: {x: 0, y: 1, z: 0}",
                text,
                "PERF-021 fixes the sort axis at (0, 1, 0) for Y-sorting");
        }

        [Test]
        public void TestTightMeshForLargeSprites()
        {
            var violations = new List<string>();
            int checkedCount = 0;
            foreach (string path in EnumerateFiles(ArtRoot, "*.png.meta"))
            {
                string meta = File.ReadAllText(path);
                if (!meta.Contains("textureType: 8"))
                {
                    continue;
                }

                string texturePath = path.Substring(
                    0, path.Length - ".meta".Length);
                var importer = AssetImporter.GetAtPath(texturePath) as TextureImporter;
                if (importer == null)
                {
                    continue;
                }

                importer.GetSourceTextureWidthAndHeight(
                    out int width, out int height);
                if (Mathf.Max(width, height) < 256)
                {
                    continue;
                }

                checkedCount++;
                if (!HasTransparentOuterMargin(texturePath))
                {
                    continue;
                }

                if (!meta.Contains("spriteMeshType: 1"))
                {
                    violations.Add(
                        texturePath + " (" + width + "x" + height +
                        ", transparent margins) -> spriteMeshType not Tight");
                }
            }

            Assert.IsEmpty(
                violations,
                "sprites >= 256 texture px with transparent margins use " +
                "Tight mesh (checked " + checkedCount + "):\n" +
                string.Join("\n", violations));
        }

        private static bool HasTransparentOuterMargin(string texturePath)
        {
            var texture = new Texture2D(4, 4);
            try
            {
                if (!ImageConversion.LoadImage(
                    texture, File.ReadAllBytes(texturePath)))
                {
                    return true;
                }

                Color32[] pixels = texture.GetPixels32();
                int width = texture.width;
                int height = texture.height;
                bool RowVisible(int y)
                {
                    for (int x = 0; x < width; x++)
                    {
                        if (pixels[y * width + x].a > 0)
                        {
                            return true;
                        }
                    }

                    return false;
                }

                bool ColumnVisible(int x)
                {
                    for (int y = 0; y < height; y++)
                    {
                        if (pixels[y * width + x].a > 0)
                        {
                            return true;
                        }
                    }

                    return false;
                }

                return !RowVisible(0) || !RowVisible(height - 1) ||
                    !ColumnVisible(0) || !ColumnVisible(width - 1);
            }
            finally
            {
                Object.DestroyImmediate(texture);
            }
        }

        [Test]
        public void TestAnimatorCulling()
        {
            var violations = new List<string>();
            foreach (string path in EnumerateFiles(ArtRoot, "*.prefab"))
            {
                string text = File.ReadAllText(path);
                int cursor = 0;
                while (true)
                {
                    int at = text.IndexOf(
                        "Animator:", cursor, System.StringComparison.Ordinal);
                    if (at < 0)
                    {
                        break;
                    }

                    cursor = at + 1;
                    int end = text.IndexOf("--- !u!", at, System.StringComparison.Ordinal);
                    if (end < 0)
                    {
                        end = text.Length;
                    }

                    string block = text.Substring(at, end - at);
                    if (!block.Contains("m_CullingMode: 2"))
                    {
                        violations.Add(
                            path + " -> Animator not CullCompletely");
                    }
                }
            }

            Assert.IsEmpty(
                violations,
                "PERF-021 requires Animator CullCompletely:\n" +
                string.Join("\n", violations));
        }

        [Test]
        public void TestNoRuntimeMaterialInstances()
        {
            var violations = new List<string>();
            foreach (string path in EnumerateFiles(ScriptsRoot, "*.cs"))
            {
                if (path.Replace('\\', '/').Contains("/Editor/"))
                {
                    continue;
                }

                string text = File.ReadAllText(path);
                if (text.Contains("new Material(") || text.Contains(".material"))
                {
                    violations.Add(path);
                }
            }

            Assert.IsEmpty(
                violations,
                "runtime code must not instance materials " +
                "(no .material getter, no new Material):\n" +
                string.Join("\n", violations));
        }

        private static IEnumerable<string> EnumerateFiles(
            string root,
            string pattern)
        {
            if (!Directory.Exists(root))
            {
                yield break;
            }

            foreach (string path in Directory.EnumerateFiles(
                root, pattern, SearchOption.AllDirectories))
            {
                yield return path;
            }
        }
    }
}
