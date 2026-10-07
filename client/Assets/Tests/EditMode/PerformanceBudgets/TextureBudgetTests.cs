using System.Collections.Generic;
using System.IO;
using NUnit.Framework;

namespace ThinhThan.Tests.EditMode.PerformanceBudgets
{
    /// <summary>
    /// PERF-006 texture memory (client_performance.md measurement row +
    /// presentation_asset_manifest.md §1): decoded gameplay textures must fit
    /// the resident-steady umbrella budget. The estimate uses ~1 byte/px
    /// (BC7/ASTC 4x4 in-memory) over every committed PNG that ships in the
    /// runtime art tree (authoring references under StyleRef are excluded).
    /// </summary>
    [Category("Performance")]
    public sealed class TextureBudgetTests
    {
        private const string ArtRoot = "Assets/Art";

        /// <summary>Resident steady cap from presentation_asset_manifest.md §1.</summary>
        private const long ResidentSteadyBytes = 450L * 1024 * 1024;

        private static readonly string[] ExcludedRoots =
        {
            "/StyleRef/",
        };

        [Test]
        public void TestTextureMemoryBudgets()
        {
            var perArea = new Dictionary<string, long>();
            long total = 0;
            int textures = 0;

            foreach (string path in EnumeratePngs(ArtRoot))
            {
                if (!TryReadPngSize(path, out int width, out int height))
                {
                    continue;
                }

                textures++;
                long bytes = (long)width * height;
                total += bytes;
                string key = AreaKey(path);
                perArea[key] = perArea.TryGetValue(key, out long v)
                    ? v + bytes
                    : bytes;
            }

            var lines = new List<string>();
            foreach (KeyValuePair<string, long> pair in perArea)
            {
                lines.Add(pair.Key + " " + (pair.Value / (1024 * 1024)) + " MB");
            }

            Assert.LessOrEqual(
                total,
                ResidentSteadyBytes,
                "runtime texture estimate " +
                (total / (1024 * 1024)) +
                " MB exceeds the 450 MB resident-steady budget " +
                "(§1) across " + textures + " textures:\n" +
                string.Join("\n", lines));
        }

        private static string AreaKey(string path)
        {
            string rel = path.Replace('\\', '/').Substring(ArtRoot.Length + 1);
            int slash = rel.IndexOf('/');
            if (slash < 0)
            {
                return rel;
            }

            int second = rel.IndexOf('/', slash + 1);
            return second < 0 ? rel : rel.Substring(0, second);
        }

        private static IEnumerable<string> EnumeratePngs(string root)
        {
            foreach (string path in Directory.EnumerateFiles(
                root, "*.png", SearchOption.AllDirectories))
            {
                string normalized = path.Replace('\\', '/');
                bool excluded = false;
                foreach (string marker in ExcludedRoots)
                {
                    if (normalized.Contains(marker))
                    {
                        excluded = true;
                        break;
                    }
                }

                if (!excluded)
                {
                    yield return normalized;
                }
            }
        }

        private static bool TryReadPngSize(
            string path,
            out int width,
            out int height)
        {
            width = 0;
            height = 0;
            byte[] header = new byte[26];
            using (FileStream stream = File.OpenRead(path))
            {
                if (stream.Read(header, 0, header.Length) < header.Length)
                {
                    return false;
                }
            }

            if (header[0] != 0x89 || header[1] != 'P' ||
                header[2] != 'N' || header[3] != 'G' ||
                header[12] != 'I' || header[13] != 'H' ||
                header[14] != 'D' || header[15] != 'R')
            {
                return false;
            }

            width = (header[16] << 24) | (header[17] << 16) |
                (header[18] << 8) | header[19];
            height = (header[20] << 24) | (header[21] << 16) |
                (header[22] << 8) | header[23];
            return width > 0 && height > 0;
        }
    }
}
